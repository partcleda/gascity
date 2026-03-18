package main

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gastownhall/gascity/internal/beads"
	"github.com/gastownhall/gascity/internal/clock"
	"github.com/gastownhall/gascity/internal/config"
	"github.com/gastownhall/gascity/internal/events"
	"github.com/gastownhall/gascity/internal/runtime"
)

type gatedStartProvider struct {
	*runtime.Fake
	mu            sync.Mutex
	inFlight      int
	maxInFlight   int
	started       []string
	startSignals  chan string
	releaseByName map[string]chan struct{}
}

func newGatedStartProvider() *gatedStartProvider {
	return &gatedStartProvider{
		Fake:          runtime.NewFake(),
		startSignals:  make(chan string, 32),
		releaseByName: make(map[string]chan struct{}),
	}
}

func (p *gatedStartProvider) Start(ctx context.Context, name string, cfg runtime.Config) error {
	p.mu.Lock()
	p.inFlight++
	if p.inFlight > p.maxInFlight {
		p.maxInFlight = p.inFlight
	}
	p.started = append(p.started, name)
	ch := p.releaseByName[name]
	if ch == nil {
		ch = make(chan struct{})
		p.releaseByName[name] = ch
	}
	p.mu.Unlock()

	p.startSignals <- name

	select {
	case <-ch:
	case <-ctx.Done():
		p.mu.Lock()
		p.inFlight--
		p.mu.Unlock()
		return ctx.Err()
	}

	err := p.Fake.Start(ctx, name, cfg)
	p.mu.Lock()
	p.inFlight--
	p.mu.Unlock()
	return err
}

func (p *gatedStartProvider) release(name string) {
	p.mu.Lock()
	ch := p.releaseByName[name]
	p.mu.Unlock()
	if ch != nil {
		select {
		case <-ch:
		default:
			close(ch)
		}
	}
}

func (p *gatedStartProvider) waitForStarts(t *testing.T, n int) []string {
	t.Helper()
	var names []string
	timeout := time.After(3 * time.Second)
	for len(names) < n {
		select {
		case name := <-p.startSignals:
			names = append(names, name)
		case <-timeout:
			t.Fatalf("timed out waiting for %d starts, got %v", n, names)
		}
	}
	return names
}

func (p *gatedStartProvider) ensureNoFurtherStart(t *testing.T, wait time.Duration) {
	t.Helper()
	select {
	case name := <-p.startSignals:
		t.Fatalf("unexpected extra start signal: %s", name)
	case <-time.After(wait):
	}
}

type gatedStopProvider struct {
	*runtime.Fake
	mu            sync.Mutex
	inFlight      int
	maxInFlight   int
	stopSignals   chan string
	releaseByName map[string]chan struct{}
}

func newGatedStopProvider() *gatedStopProvider {
	return &gatedStopProvider{
		Fake:          runtime.NewFake(),
		stopSignals:   make(chan string, 32),
		releaseByName: make(map[string]chan struct{}),
	}
}

func (p *gatedStopProvider) Stop(name string) error {
	p.mu.Lock()
	p.inFlight++
	if p.inFlight > p.maxInFlight {
		p.maxInFlight = p.inFlight
	}
	ch := p.releaseByName[name]
	if ch == nil {
		ch = make(chan struct{})
		p.releaseByName[name] = ch
	}
	p.mu.Unlock()

	p.stopSignals <- name
	<-ch

	err := p.Fake.Stop(name)
	p.mu.Lock()
	p.inFlight--
	p.mu.Unlock()
	return err
}

func (p *gatedStopProvider) release(name string) {
	p.mu.Lock()
	ch := p.releaseByName[name]
	p.mu.Unlock()
	if ch != nil {
		select {
		case <-ch:
		default:
			close(ch)
		}
	}
}

func (p *gatedStopProvider) waitForStops(t *testing.T, n int) []string {
	t.Helper()
	var names []string
	timeout := time.After(3 * time.Second)
	for len(names) < n {
		select {
		case name := <-p.stopSignals:
			names = append(names, name)
		case <-timeout:
			t.Fatalf("timed out waiting for %d stops, got %v", n, names)
		}
	}
	return names
}

func (p *gatedStopProvider) ensureNoFurtherStop(t *testing.T, wait time.Duration) {
	t.Helper()
	select {
	case name := <-p.stopSignals:
		t.Fatalf("unexpected extra stop signal: %s", name)
	case <-time.After(wait):
	}
}

func containsAll(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[string]int)
	for _, name := range got {
		seen[name]++
	}
	for _, name := range want {
		if seen[name] == 0 {
			return false
		}
		seen[name]--
	}
	return true
}

func TestReconcileSessionBeads_StartsIndependentWaveInParallelBeforeDependentWave(t *testing.T) {
	cfg := &config.City{
		Agents: []config.Agent{
			{Name: "worker", DependsOn: []string{"db", "cache"}},
			{Name: "db"},
			{Name: "cache"},
		},
	}
	store := beads.NewMemStore()
	sp := newGatedStartProvider()
	rec := events.Discard
	clk := &clock.Fake{Time: time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)}
	desired := map[string]TemplateParams{
		"db":     {Command: "db", SessionName: "db", TemplateName: "db"},
		"cache":  {Command: "cache", SessionName: "cache", TemplateName: "cache"},
		"worker": {Command: "worker", SessionName: "worker", TemplateName: "worker"},
	}
	db := makeBead("db-id", map[string]string{
		"session_name": "db", "template": "db", "generation": "1", "instance_token": "tok-db", "state": "asleep",
	})
	cache := makeBead("cache-id", map[string]string{
		"session_name": "cache", "template": "cache", "generation": "1", "instance_token": "tok-cache", "state": "asleep",
	})
	worker := makeBead("worker-id", map[string]string{
		"session_name": "worker", "template": "worker", "generation": "1", "instance_token": "tok-worker", "state": "asleep",
	})
	for _, bead := range []beads.Bead{db, cache, worker} {
		if _, err := store.Create(beads.Bead{
			ID:       bead.ID,
			Title:    bead.Metadata["session_name"],
			Type:     sessionBeadType,
			Labels:   []string{sessionBeadLabel},
			Metadata: bead.Metadata,
		}); err != nil {
			t.Fatal(err)
		}
	}
	sessions, err := loadSessionBeads(store)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan int, 1)
	go func() {
		done <- reconcileSessionBeads(
			context.Background(), sessions, desired, configuredSessionNames(cfg, "", store),
			cfg, sp, store, nil, nil, nil, newDrainTracker(), map[string]int{}, "",
			nil, clk, rec, 5*time.Second, 0, ioDiscard{}, ioDiscard{},
		)
	}()

	firstWave := sp.waitForStarts(t, 2)
	if !containsAll(firstWave, "db", "cache") {
		t.Fatalf("first wave = %v, want db+cache", firstWave)
	}
	sp.ensureNoFurtherStart(t, 150*time.Millisecond)
	sp.release("db")
	sp.release("cache")

	secondWave := sp.waitForStarts(t, 1)
	if !containsAll(secondWave, "worker") {
		t.Fatalf("second wave = %v, want worker", secondWave)
	}
	sp.release("worker")

	select {
	case woken := <-done:
		if woken != 3 {
			t.Fatalf("woken = %d, want 3", woken)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("reconcile did not finish")
	}

	if sp.maxInFlight != 2 {
		t.Fatalf("max in-flight starts = %d, want 2", sp.maxInFlight)
	}
}

func TestReconcileSessionBeads_FailedDependencyBlocksDependentButNotSibling(t *testing.T) {
	env := newReconcilerTestEnv()
	env.cfg = &config.City{
		Agents: []config.Agent{
			{Name: "worker", DependsOn: []string{"db"}},
			{Name: "db"},
			{Name: "cache"},
		},
	}
	env.addDesired("worker", "worker", false)
	env.addDesired("db", "db", false)
	env.addDesired("cache", "cache", false)
	env.sp.StartErrors["db"] = context.DeadlineExceeded

	woken := env.reconcile([]beads.Bead{
		env.createSessionBead("worker", "worker"),
		env.createSessionBead("db", "db"),
		env.createSessionBead("cache", "cache"),
	})

	if woken != 1 {
		t.Fatalf("woken = %d, want 1", woken)
	}
	if env.sp.IsRunning("worker") {
		t.Fatal("worker should not be running when db failed to start")
	}
	if !env.sp.IsRunning("cache") {
		t.Fatal("cache should still start despite db failure")
	}
}

func TestStopSessionsBounded_StopsDependentsBeforeDependencies(t *testing.T) {
	sp := newGatedStopProvider()
	for _, name := range []string{"db", "api", "worker", "audit"} {
		if err := sp.Start(context.Background(), name, runtime.Config{}); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &config.City{
		Agents: []config.Agent{
			{Name: "worker", DependsOn: []string{"api"}},
			{Name: "audit", DependsOn: []string{"db"}},
			{Name: "api", DependsOn: []string{"db"}},
			{Name: "db"},
		},
	}
	rec := events.NewFake()
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- stopSessionsBounded([]string{"db", "api", "worker", "audit"}, cfg, sp, rec, "gc", &stdout, &stderr)
	}()

	firstWave := sp.waitForStops(t, 1)
	if !containsAll(firstWave, "worker") {
		t.Fatalf("first stop wave = %v, want worker", firstWave)
	}
	sp.ensureNoFurtherStop(t, 150*time.Millisecond)
	sp.release("worker")

	secondWave := sp.waitForStops(t, 2)
	if !containsAll(secondWave, "api", "audit") {
		t.Fatalf("second stop wave = %v, want api+audit", secondWave)
	}
	sp.release("api")
	sp.release("audit")

	thirdWave := sp.waitForStops(t, 1)
	if !containsAll(thirdWave, "db") {
		t.Fatalf("third stop wave = %v, want db", thirdWave)
	}
	sp.release("db")

	select {
	case stopped := <-done:
		if stopped != 4 {
			t.Fatalf("stopped = %d, want 4", stopped)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("stopSessionsBounded did not finish")
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }
