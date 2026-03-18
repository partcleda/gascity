package main

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/gastownhall/gascity/internal/beads"
	"github.com/gastownhall/gascity/internal/clock"
	"github.com/gastownhall/gascity/internal/config"
	"github.com/gastownhall/gascity/internal/events"
	"github.com/gastownhall/gascity/internal/runtime"
	sessionpkg "github.com/gastownhall/gascity/internal/session"
)

const (
	defaultMaxParallelStartsPerWave = 3
	defaultMaxParallelStopsPerWave  = 3
)

type startCandidate struct {
	session *beads.Bead
	tp      TemplateParams
	order   int
}

func (c startCandidate) name() string {
	return c.session.Metadata["session_name"]
}

func (c startCandidate) template() string {
	return c.session.Metadata["template"]
}

type preparedStart struct {
	candidate startCandidate
	cfg       runtime.Config
	coreHash  string
	liveHash  string
}

type startResult struct {
	prepared preparedStart
	err      error
	outcome  string
}

type stopTarget struct {
	name     string
	template string
	order    int
}

type stopResult struct {
	target stopTarget
	err    error
}

func dependencyTemplateWaveOrder(templatesInOrder []string, deps map[string][]string) (map[string]int, bool) {
	if len(templatesInOrder) == 0 {
		return map[string]int{}, true
	}
	present := make(map[string]bool, len(templatesInOrder))
	indegree := make(map[string]int, len(templatesInOrder))
	dependents := make(map[string][]string, len(templatesInOrder))
	emitted := make(map[string]bool, len(templatesInOrder))
	for _, template := range templatesInOrder {
		present[template] = true
	}
	for _, template := range templatesInOrder {
		for _, dep := range deps[template] {
			if !present[dep] {
				continue
			}
			indegree[template]++
			dependents[dep] = append(dependents[dep], template)
		}
	}
	waveByTemplate := make(map[string]int, len(templatesInOrder))
	emittedCount := 0
	wave := 0
	for emittedCount < len(templatesInOrder) {
		var ready []string
		for _, template := range templatesInOrder {
			if emitted[template] {
				continue
			}
			if indegree[template] == 0 {
				ready = append(ready, template)
			}
		}
		if len(ready) == 0 {
			return nil, false
		}
		for _, template := range ready {
			emitted[template] = true
			waveByTemplate[template] = wave
			emittedCount++
			for _, dependent := range dependents[template] {
				indegree[dependent]--
			}
		}
		wave++
	}
	return waveByTemplate, true
}

func strictSerialWaveOrder[T any](items []T) map[int]int {
	result := make(map[int]int, len(items))
	for i := range items {
		result[i] = i
	}
	return result
}

func dependencyTemplateAlive(
	template string,
	cfg *config.City,
	desiredState map[string]TemplateParams,
	sp runtime.Provider,
	cityName string,
	store beads.Store,
) bool {
	if cfg == nil || template == "" {
		return false
	}
	cfgAgent := findAgentByTemplate(cfg, template)
	if cfgAgent == nil {
		return false
	}
	if cfgAgent.Pool != nil {
		for name, tp := range desiredState {
			if tp.TemplateName != template {
				continue
			}
			if sp.IsRunning(name) && sp.ProcessAlive(name, tp.Hints.ProcessNames) {
				return true
			}
		}
		return false
	}
	sessionName := lookupSessionNameOrLegacy(store, cityName, template, cfg.Workspace.SessionTemplate)
	depTP := desiredState[sessionName]
	return sp.IsRunning(sessionName) && sp.ProcessAlive(sessionName, depTP.Hints.ProcessNames)
}

func candidateWaveOrder(
	candidates []startCandidate,
	cfg *config.City,
	desiredState map[string]TemplateParams,
	sp runtime.Provider,
	cityName string,
	store beads.Store,
) (map[int]int, bool) {
	if len(candidates) == 0 {
		return map[int]int{}, true
	}
	var templatesInOrder []string
	templateSeen := make(map[string]bool)
	candidateTemplates := make(map[string]bool)
	for _, candidate := range candidates {
		template := candidate.template()
		candidateTemplates[template] = true
		if !templateSeen[template] {
			templateSeen[template] = true
			templatesInOrder = append(templatesInOrder, template)
		}
	}
	filteredDeps := make(map[string][]string)
	eligibleTemplates := make(map[string]bool, len(candidateTemplates))
	for _, template := range templatesInOrder {
		cfgAgent := findAgentByTemplate(cfg, template)
		if cfgAgent == nil {
			continue
		}
		blocked := false
		for _, dep := range cfgAgent.DependsOn {
			if dependencyTemplateAlive(dep, cfg, desiredState, sp, cityName, store) {
				continue
			}
			if !candidateTemplates[dep] {
				blocked = true
				break
			}
			filteredDeps[template] = append(filteredDeps[template], dep)
		}
		if !blocked {
			eligibleTemplates[template] = true
		}
	}
	var eligibleTemplatesInOrder []string
	for _, template := range templatesInOrder {
		if eligibleTemplates[template] {
			eligibleTemplatesInOrder = append(eligibleTemplatesInOrder, template)
		}
	}
	templateWave, ok := dependencyTemplateWaveOrder(eligibleTemplatesInOrder, filteredDeps)
	if !ok {
		return strictSerialWaveOrder(candidates), false
	}
	candidateWave := make(map[int]int, len(candidates))
	for idx, candidate := range candidates {
		wave, ok := templateWave[candidate.template()]
		if !ok {
			continue
		}
		candidateWave[idx] = wave
	}
	return candidateWave, true
}

func prepareStartCandidate(
	candidate startCandidate,
	store beads.Store,
	clk clock.Clock,
) (*preparedStart, error) {
	session := candidate.session
	tp := candidate.tp
	if _, _, err := preWakeCommit(session, store, clk); err != nil {
		return nil, err
	}
	agentCfg := templateParamsToConfig(tp)
	coreHash := runtime.CoreFingerprint(agentCfg)
	liveHash := runtime.LiveFingerprint(agentCfg)
	if wd := resolveTaskWorkDir(store, session.Metadata["template"]); wd != "" {
		agentCfg.WorkDir = wd
	} else if wd := session.Metadata["work_dir"]; wd != "" {
		agentCfg.WorkDir = wd
	}
	if sk := session.Metadata["session_key"]; sk != "" && tp.ResolvedProvider != nil {
		firstStart := session.Metadata["started_config_hash"] == ""
		agentCfg.Command = resolveSessionCommand(agentCfg.Command, sk, tp.ResolvedProvider, firstStart)
	}
	generation, _ := strconv.Atoi(session.Metadata["generation"])
	if generation <= 0 {
		generation = sessionpkg.DefaultGeneration
	}
	continuationEpoch, _ := strconv.Atoi(session.Metadata["continuation_epoch"])
	if continuationEpoch <= 0 {
		continuationEpoch = sessionpkg.DefaultContinuationEpoch
	}
	instanceToken := session.Metadata["instance_token"]
	if instanceToken == "" {
		instanceToken = sessionpkg.NewInstanceToken()
		if err := store.SetMetadata(session.ID, "instance_token", instanceToken); err != nil {
			return nil, err
		}
		session.Metadata["instance_token"] = instanceToken
	}
	agentCfg.Env = mergeEnv(agentCfg.Env, sessionpkg.RuntimeEnv(
		session.ID,
		candidate.name(),
		generation,
		continuationEpoch,
		instanceToken,
	))
	agentCfg = runtime.SyncWorkDirEnv(agentCfg)
	return &preparedStart{
		candidate: candidate,
		cfg:       agentCfg,
		coreHash:  coreHash,
		liveHash:  liveHash,
	}, nil
}

func executePreparedStartWave(
	ctx context.Context,
	prepared []preparedStart,
	sp runtime.Provider,
	startupTimeout time.Duration,
	maxParallel int,
) []startResult {
	if len(prepared) == 0 {
		return nil
	}
	if maxParallel <= 0 {
		maxParallel = 1
	}
	results := make([]startResult, len(prepared))
	sem := make(chan struct{}, maxParallel)
	done := make(chan int, len(prepared))
	for i, item := range prepared {
		i, item := i, item
		sem <- struct{}{}
		go func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					results[i] = startResult{
						prepared: item,
						err:      fmt.Errorf("panic during start: %v", recovered),
						outcome:  "panic_recovered",
					}
				}
				<-sem
				done <- i
			}()
			startCtx := ctx
			cancel := func() {}
			if startupTimeout > 0 {
				startCtx, cancel = context.WithTimeout(ctx, startupTimeout)
			}
			defer cancel()
			err := sp.Start(startCtx, item.candidate.name(), item.cfg)
			outcome := "success"
			switch {
			case err == nil:
				outcome = "success"
			case startCtx.Err() == context.DeadlineExceeded:
				outcome = "deadline_exceeded"
			case startCtx.Err() == context.Canceled:
				outcome = "canceled"
			default:
				outcome = "provider_error"
			}
			results[i] = startResult{
				prepared: item,
				err:      err,
				outcome:  outcome,
			}
		}()
	}
	for range prepared {
		<-done
	}
	return results
}

func commitStartResult(
	result startResult,
	store beads.Store,
	clk clock.Clock,
	rec events.Recorder,
	stdout, stderr io.Writer,
) bool {
	session := result.prepared.candidate.session
	name := result.prepared.candidate.name()
	tp := result.prepared.candidate.tp
	if result.err != nil {
		fmt.Fprintf(stderr, "session reconciler: starting %s: %v\n", name, result.err) //nolint:errcheck
		_ = store.SetMetadata(session.ID, "last_woke_at", "")
		session.Metadata["last_woke_at"] = ""
		recordWakeFailure(session, store, clk)
		return false
	}
	fmt.Fprintf(stdout, "Woke session '%s'\n", tp.DisplayName()) //nolint:errcheck
	rec.Record(events.Event{
		Type:    events.SessionWoke,
		Actor:   "gc",
		Subject: tp.DisplayName(),
	})
	if err := store.SetMetadataBatch(session.ID, map[string]string{
		"config_hash":         result.prepared.coreHash,
		"started_config_hash": result.prepared.coreHash,
		"live_hash":           result.prepared.liveHash,
		"started_live_hash":   result.prepared.liveHash,
	}); err != nil {
		fmt.Fprintf(stderr, "session reconciler: storing hashes for %s: %v\n", name, err) //nolint:errcheck
	}
	return true
}

func executePlannedStarts(
	ctx context.Context,
	candidates []startCandidate,
	cfg *config.City,
	desiredState map[string]TemplateParams,
	sp runtime.Provider,
	store beads.Store,
	cityName string,
	clk clock.Clock,
	rec events.Recorder,
	startupTimeout time.Duration,
	stdout, stderr io.Writer,
) int {
	if len(candidates) == 0 {
		return 0
	}
	waveByCandidate, ok := candidateWaveOrder(candidates, cfg, desiredState, sp, cityName, store)
	if !ok {
		fmt.Fprintln(stderr, "session reconciler: dependency graph fallback to serial start order") //nolint:errcheck
	}
	maxWave := -1
	for _, wave := range waveByCandidate {
		if wave > maxWave {
			maxWave = wave
		}
	}
	wakeCount := 0
	for wave := 0; wave <= maxWave; wave++ {
		var waveCandidates []startCandidate
		for idx, candidate := range candidates {
			if waveByCandidate[idx] == wave {
				waveCandidates = append(waveCandidates, candidate)
			}
		}
		if len(waveCandidates) == 0 {
			continue
		}
		var prepared []preparedStart
		for _, candidate := range waveCandidates {
			if !allDependenciesAlive(*candidate.session, cfg, desiredState, sp, cityName, store) {
				continue
			}
			item, err := prepareStartCandidate(candidate, store, clk)
			if err != nil {
				fmt.Fprintf(stderr, "session reconciler: pre-wake %s: %v\n", candidate.name(), err) //nolint:errcheck
				continue
			}
			prepared = append(prepared, *item)
		}
		results := executePreparedStartWave(ctx, prepared, sp, startupTimeout, defaultMaxParallelStartsPerWave)
		for _, result := range results {
			if commitStartResult(result, store, clk, rec, stdout, stderr) {
				wakeCount++
			}
		}
	}
	return wakeCount
}

func stopWaveOrder(targets []stopTarget, cfg *config.City) (map[int]int, bool) {
	if len(targets) == 0 {
		return map[int]int{}, true
	}
	var templatesInOrder []string
	templateSeen := make(map[string]bool)
	for _, target := range targets {
		if templateSeen[target.template] {
			continue
		}
		templateSeen[target.template] = true
		templatesInOrder = append(templatesInOrder, target.template)
	}
	deps := buildDepsMap(cfg)
	templateWave, ok := dependencyTemplateWaveOrder(templatesInOrder, deps)
	if !ok {
		return strictSerialWaveOrder(targets), false
	}
	maxWave := 0
	for _, wave := range templateWave {
		if wave > maxWave {
			maxWave = wave
		}
	}
	targetWave := make(map[int]int, len(targets))
	for idx, target := range targets {
		targetWave[idx] = maxWave - templateWave[target.template]
	}
	return targetWave, true
}

func executeStopWave(targets []stopTarget, sp runtime.Provider, maxParallel int) []stopResult {
	if len(targets) == 0 {
		return nil
	}
	if maxParallel <= 0 {
		maxParallel = 1
	}
	results := make([]stopResult, len(targets))
	sem := make(chan struct{}, maxParallel)
	done := make(chan int, len(targets))
	for i, target := range targets {
		i, target := i, target
		sem <- struct{}{}
		go func() {
			defer func() {
				<-sem
				done <- i
			}()
			results[i] = stopResult{
				target: target,
				err:    sp.Stop(target.name),
			}
		}()
	}
	for range targets {
		<-done
	}
	return results
}

func stopTargetsForNames(names []string, cfg *config.City) []stopTarget {
	targets := make([]stopTarget, 0, len(names))
	for idx, name := range names {
		targets = append(targets, stopTarget{
			name:     name,
			template: resolveAgentTemplate(name, cfg),
			order:    idx,
		})
	}
	return targets
}

func stopSessionsBounded(
	names []string,
	cfg *config.City,
	sp runtime.Provider,
	rec events.Recorder,
	actor string,
	stdout, stderr io.Writer,
) int {
	targets := stopTargetsForNames(names, cfg)
	waveByTarget, ok := stopWaveOrder(targets, cfg)
	if !ok {
		fmt.Fprintln(stderr, "session lifecycle: dependency graph fallback to serial stop order") //nolint:errcheck
	}
	maxWave := -1
	for _, wave := range waveByTarget {
		if wave > maxWave {
			maxWave = wave
		}
	}
	stopped := 0
	for wave := 0; wave <= maxWave; wave++ {
		var waveTargets []stopTarget
		for idx, target := range targets {
			if waveByTarget[idx] == wave {
				waveTargets = append(waveTargets, target)
			}
		}
		results := executeStopWave(waveTargets, sp, defaultMaxParallelStopsPerWave)
		for _, result := range results {
			if result.err != nil {
				fmt.Fprintf(stderr, "gc stop: stopping %s: %v\n", result.target.name, result.err) //nolint:errcheck
				continue
			}
			fmt.Fprintf(stdout, "Stopped agent '%s'\n", result.target.name) //nolint:errcheck
			stopped++
			rec.Record(events.Event{
				Type: events.SessionStopped, Actor: actor, Subject: result.target.name,
			})
		}
	}
	return stopped
}
