package main

import (
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/gastownhall/gascity/internal/beads"
	"github.com/gastownhall/gascity/internal/config"
	"github.com/gastownhall/gascity/internal/fsys"
	"github.com/gastownhall/gascity/internal/materialize"
	"github.com/gastownhall/gascity/internal/runtime"
)

// agentBuildParams holds shared, per-city parameters for building agents.
// These are constant across all agents in a single buildDesiredState call.
type agentBuildParams struct {
	city            *config.City
	cityName        string
	cityPath        string
	workspace       *config.Workspace
	agents          []config.Agent
	providers       map[string]config.ProviderSpec
	lookPath        config.LookPathFunc
	fs              fsys.FS
	sp              runtime.Provider
	rigs            []config.Rig
	sessionTemplate string
	beaconTime      time.Time
	packDirs        []string
	packOverlayDirs []string
	rigOverlayDirs  map[string][]string
	globalFragments []string
	appendFragments []string // V2: city-level [agents].append_fragments / [agent_defaults].append_fragments
	stderr          io.Writer

	// beadStore is the city-level bead store for session bead lookups.
	// When non-nil, session names are derived from bead IDs ("s-{beadID}")
	// instead of the legacy SessionNameFor function.
	beadStore beads.Store

	// sessionBeads caches the open session-bead snapshot for the current
	// desired-state build so per-agent resolution does not rescan the store.
	sessionBeads *sessionBeadSnapshot

	// beadNames caches qualifiedName → session_name mappings resolved
	// during this build cycle. Populated lazily by resolveSessionName.
	beadNames map[string]string

	// skillCatalog is the shared skill catalog for this city (union of
	// city pack's skills/ and every bootstrap implicit-import pack's
	// skills/). Loaded once per build cycle and reused across every
	// agent. Nil when LoadCityCatalog returned an error — the build
	// continues without skill materialization participation in
	// fingerprints or PreStart injection. The load error is logged to
	// stderr at params-construction time.
	skillCatalog *materialize.CityCatalog
	// rigSkillCatalogs caches rig-specific shared catalogs. Each entry
	// includes city-shared skills plus any rig-import shared catalogs.
	rigSkillCatalogs map[string]*materialize.CityCatalog
	// failedRigSkillCatalogs tracks rig scopes whose shared catalog
	// failed to load for this build. Agents in those rigs must not
	// fall back to the city catalog or they will inject stage-2 skill
	// hooks that reload the broken rig catalog and fail at runtime.
	failedRigSkillCatalogs map[string]bool

	// sessionProvider is cfg.Session.Provider (the city-level session
	// runtime selector: "" / "tmux" / "subprocess" / "acp" / "k8s" /
	// etc.). Used by the skill materialization integration to decide
	// stage-2 eligibility.
	sessionProvider string
}

// newAgentBuildParams constructs agentBuildParams from the common startup values.
func newAgentBuildParams(cityName, cityPath string, cfg *config.City, sp runtime.Provider, beaconTime time.Time, store beads.Store, stderr io.Writer) *agentBuildParams {
	params := &agentBuildParams{
		city:            cfg,
		cityName:        cityName,
		cityPath:        cityPath,
		workspace:       &cfg.Workspace,
		agents:          append([]config.Agent(nil), cfg.Agents...),
		providers:       cfg.Providers,
		lookPath:        exec.LookPath,
		fs:              fsys.OSFS{},
		sp:              sp,
		rigs:            cfg.Rigs,
		sessionTemplate: cfg.Workspace.SessionTemplate,
		beaconTime:      beaconTime,
		packDirs:        cfg.PackDirs,
		packOverlayDirs: cfg.PackOverlayDirs,
		rigOverlayDirs:  cfg.RigOverlayDirs,
		globalFragments: cfg.Workspace.GlobalFragments,
		appendFragments: mergeFragmentLists(cfg.AgentDefaults.AppendFragments, cfg.AgentsDefaults.AppendFragments),
		beadStore:       store,
		beadNames:       make(map[string]string),
		stderr:          stderr,
		sessionProvider: cfg.Session.Provider,
	}
	// Load the shared skill catalog once per build cycle. A transient load
	// failure (filesystem race during dolt sync / heavy I/O) used to
	// silently set skillCatalog = nil for that tick, which dropped every
	// `skills:*` entry from FingerprintExtra and flipped CoreFingerprint
	// for every live session → config-drift drain storm. Fall back to the
	// last successfully cached catalog so the fingerprint stays stable
	// across transient failures. A real catalog edit still propagates: the
	// next successful load overwrites the cache.
	cat, err := loadSharedSkillCatalog(cfg, "")
	if err != nil {
		if cached, ok := cachedCityCatalog(cityPath); ok {
			catCopy := cached
			params.skillCatalog = &catCopy
			if stderr != nil {
				fmt.Fprintf(stderr, "buildDesiredState: LoadCityCatalog %v (using cached catalog to avoid drift)\n", err) //nolint:errcheck // best-effort stderr
			}
		} else if stderr != nil {
			fmt.Fprintf(stderr, "buildDesiredState: LoadCityCatalog %v (no cached catalog; skills will not contribute to fingerprints this tick)\n", err) //nolint:errcheck // best-effort stderr
		}
	} else if len(cat.Entries) == 0 {
		// No error but empty catalog: if we've previously seen a non-empty
		// catalog for this city, treat the empty result as a transient
		// input-state glitch (e.g., cfg.PackSkills temporarily empty
		// during a config reload window) and reuse the cached one.
		// Otherwise an empty→non-empty→empty sequence drops `skills:*`
		// entries from FingerprintExtra and triggers the drift drain.
		// Real removal of all skills will only happen via an explicit
		// config change; that path should go through config-reload
		// invalidation rather than this silent shape-flip.
		if cached, ok := cachedCityCatalog(cityPath); ok && len(cached.Entries) > 0 {
			catCopy := cached
			params.skillCatalog = &catCopy
			if stderr != nil {
				fmt.Fprintf(stderr, "buildDesiredState: LoadCityCatalog returned empty (cached had %d entries; reusing cache to avoid drift)\n", len(cached.Entries)) //nolint:errcheck // best-effort stderr
			}
		} else {
			params.skillCatalog = &cat
			setCachedCityCatalog(cityPath, cat)
		}
	} else {
		params.skillCatalog = &cat
		setCachedCityCatalog(cityPath, cat)
	}
	for rigName := range cfg.RigPackSkills {
		cat, err := loadSharedSkillCatalog(cfg, rigName)
		if err != nil {
			if cached, ok := cachedRigCatalog(cityPath, rigName); ok {
				if params.rigSkillCatalogs == nil {
					params.rigSkillCatalogs = make(map[string]*materialize.CityCatalog)
				}
				catCopy := cached
				params.rigSkillCatalogs[rigName] = &catCopy
				if stderr != nil {
					fmt.Fprintf(stderr, "buildDesiredState: LoadCityCatalog rig %q %v (using cached catalog to avoid drift)\n", rigName, err) //nolint:errcheck // best-effort stderr
				}
				continue
			}
			if stderr != nil {
				fmt.Fprintf(stderr, "buildDesiredState: LoadCityCatalog rig %q %v (no cached catalog; skills will not contribute to fingerprints this tick)\n", rigName, err) //nolint:errcheck // best-effort stderr
			}
			if params.failedRigSkillCatalogs == nil {
				params.failedRigSkillCatalogs = make(map[string]bool)
			}
			params.failedRigSkillCatalogs[rigName] = true
			continue
		}
		if len(cat.Entries) == 0 {
			if cached, ok := cachedRigCatalog(cityPath, rigName); ok && len(cached.Entries) > 0 {
				if params.rigSkillCatalogs == nil {
					params.rigSkillCatalogs = make(map[string]*materialize.CityCatalog)
				}
				catCopy := cached
				params.rigSkillCatalogs[rigName] = &catCopy
				if stderr != nil {
					fmt.Fprintf(stderr, "buildDesiredState: LoadCityCatalog rig %q returned empty (cached had %d entries; reusing cache to avoid drift)\n", rigName, len(cached.Entries)) //nolint:errcheck // best-effort stderr
				}
				continue
			}
		}
		if params.rigSkillCatalogs == nil {
			params.rigSkillCatalogs = make(map[string]*materialize.CityCatalog)
		}
		catCopy := cat
		params.rigSkillCatalogs[rigName] = &catCopy
		setCachedRigCatalog(cityPath, rigName, cat)
	}
	return params
}

func (p *agentBuildParams) sharedSkillCatalogForAgent(agent *config.Agent) *materialize.CityCatalog {
	if p == nil || agent == nil {
		return nil
	}
	rigName := agentRigScopeName(agent, p.rigs)
	if rigName != "" && p.failedRigSkillCatalogs != nil && p.failedRigSkillCatalogs[rigName] {
		return nil
	}
	if p.rigSkillCatalogs != nil && rigName != "" {
		if cat := p.rigSkillCatalogs[rigName]; cat != nil {
			return cat
		}
	}
	return p.skillCatalog
}

// effectiveOverlayDirs merges city-level and rig-level pack overlay dirs.
// City dirs come first (lower priority), then rig-specific dirs.
func effectiveOverlayDirs(cityDirs []string, rigDirs map[string][]string, rigName string) []string {
	rigSpecific := rigDirs[rigName]
	if len(rigSpecific) == 0 {
		return cityDirs
	}
	if len(cityDirs) == 0 {
		return rigSpecific
	}
	merged := make([]string, 0, len(cityDirs)+len(rigSpecific))
	merged = append(merged, cityDirs...)
	merged = append(merged, rigSpecific...)
	return merged
}

// templateNameFor returns the configuration template name for an agent.
// For pool instances, this is the original template name (PoolName).
// For regular agents, it's the qualified name.
func templateNameFor(cfgAgent *config.Agent, qualifiedName string) string {
	if cfgAgent.PoolName != "" {
		return cfgAgent.PoolName
	}
	return qualifiedName
}
