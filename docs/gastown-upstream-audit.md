# Gas Town Upstream Audit — Parity Tracking

Audit of 574 + 151 commits from `gastown:upstream/main` since Gas City was
created (2026-02-22). Delta 1: 574 commits through 2026-03-01. Delta 2: 151
non-merge, non-backup commits 977953d8..04e7ed7c (2026-03-01 to 2026-03-03).
Organized by theme so we can review together and decide actions.

**Legend:** `[ ]` = pending review, `[x]` = addressed, `[-]` = skipped (N/A), `[~]` = deferred

---

## 1. Persistent Polecat Pool (ARCHITECTURAL)

The biggest change in Gas Town: polecats no longer die after completing work.
"Done means idle, not dead." Sandboxes preserved for reuse, witness restarts
instead of nuking, completion signaling via agent beads instead of mail.

### 1a. Polecat lifecycle: done = idle
- [~] **c410c10a** — `gt done` sets agent state to "idle" instead of self-nuking
  worktree. Sling reuses idle polecats before allocating new ones.
- [~] **341fa43a** — Phase 1: `gt done` transitions to IDLE with sandbox preserved,
  worktree synced to main for immediate reuse.
- [~] **0a653b11** — Polecats self-manage completion, set agent_state=idle directly.
  Witness is safety-net only for crash recovery.
- [~] **63ad1454** — Branch-only reuse: after done, worktree syncs to main, old
  branch deleted. Next sling uses `git checkout -b` on existing worktree.
- **Action:** Update `mol-polecat-work.formula.toml` — line 408 says "You are
  GONE. Done means gone. There is no idle state." Change to reflect persistent
  model. Update polecat prompt similarly.

### 1b. Witness: restart, never nuke
- [~] **016381ad** — All `gt polecat nuke` in zombie detection replaced with
  `gt session restart`. "Idle Polecat Heresy" replaced with "Completion Protocol."
- [~] **b10863da** — Idle polecats with clean sandboxes skipped entirely by
  witness patrol. Dirty sandboxes escalated for recovery.
- **Action:** Update witness patrol formula and prompt: replace automatic
  nuking with restart-first policy. Idle polecats are healthy.

### 1c. Bead-based completion discovery (replaces POLECAT_DONE mail)
- [~] **c5ce08ed** — Agent bead completion metadata: exit_type, mr_id, branch,
  mr_failed, completion_time.
- [~] **b45d1511** — POLECAT_DONE mail deprecated. Polecats write completion
  metadata to agent bead + send tmux nudge. Witness reads bead state.
- [~] **90d08948** — Witness patrol v9: survey-workers Step 4a uses
  DiscoverCompletions() from agent_state=done beads.
- **Action:** Update witness patrol formula: mark POLECAT_DONE mail handling
  as deprecated fallback. Step 4a is the PRIMARY completion signal.

### 1d. Polecat nuke behavior
- [~] **330664c2** — Nuke no longer deletes remote branches. Refinery owns
  remote branch cleanup after merge.
- [~] **4bd189be** — Nuke checks CommitsAhead before deleting remote branches.
  Unmerged commits preserved for refinery/human.
- **Action:** Update polecat prompt if it discusses cleanup behavior.

> **Deferred** — requires sling, `gc done`, idle state management, and
> formula-on-bead (`attached_molecule`) infrastructure that Gas City
> doesn't have yet. The persistent polecat model is hidden inside
> upstream's compiled `gt done` command; Gas City needs explicit
> SDK support before this can be ported.

---

## 2. Polecat Work Formula v7

Major restructuring from 10 steps to 7, removing preflight tests entirely.

- [~] **12cf3217** — Drop full test suite from polecat formula. Refinery owns
  main health via bisecting merge queue. Steps: remove preflight-tests, replace
  run-tests with build-check (compile + targeted tests only), consolidate
  cleanup-workspace and prepare-for-review.
- [~] **9d64c0aa** — Sleepwalking polecat fix: HARD GATE requiring >= 1 commit
  ahead of origin/base_branch. Zero commits is now a hard error in commit-changes,
  cleanup-workspace, and submit-and-exit steps.
- [~] **4ede6194** — No-changes exit protocol: polecat must run `bd close <id>
  --reason="no-changes: <explanation>"` + `gt done` when bead has nothing to
  implement. Prevents spawn storms.
- **Action:** Rewrite `mol-polecat-work.formula.toml` to match v7 structure.
  Add the HARD GATE commit verification and no-changes exit protocol.

> **Deferred** — formula v7's submit step runs `gt done` (compiled Go).
> The HARD GATE and no-changes exit protocol can be ported independently
> as prompt-level guidance, but the full v7 restructuring depends on
> the persistent polecat infrastructure from S1.

---

## 3. Communication Hygiene: Nudge over Mail

Every mail creates a permanent Dolt commit. Nudges are free (tmux send-keys).

### 3a. Role template sections
- [x] **177606a4** — "Communication Hygiene: Nudge First, Mail Rarely" sections
  added to deacon, dog, polecat, and witness templates. Dogs should NEVER send
  mail. Polecats have 0-1 mail budget per session.
- [x] **a3ee0ae4** — "Dolt Health: Your Part" sections in polecat and witness
  prompts. Nudge don't mail, don't create unnecessary beads, close your beads.
- **Action:** ~~Add Communication Hygiene + Dolt Health sections to all four
  role prompts in examples/gastown.~~ DONE.

### 3b. Mail-to-nudge conversions (Go + formula)
- [x] **7a578c2b** — Six mail sends converted to nudges: MERGE_FAILED,
  CONVOY_NEEDS_FEEDING, worker rejection, MERGE_READY, RECOVERY_NEEDED,
  HandleMergeFailed. Mail preserved only for convoy completion (handoff
  context) and escalation to mayor.
  **Done:** All role prompts updated with role-specific comm rules. Generic
  nudge-first-mail-rarely principle extracted to `operational-awareness`
  global fragment. MERGE_FAILED as nudge in refinery. Protocol messages
  listed as ephemeral in global fragment.
- [x] **5872d9af** — LIFECYCLE:Shutdown, MERGED, MERGE_READY, MERGE_FAILED
  are now ephemeral wisps instead of permanent beads.
  **Done:** Listed as ephemeral protocol messages in global fragment.
- [x] **98767fa2** — WORK_DONE messages from `gt done` are ephemeral wisps.
  **Done:** Listed as ephemeral in global fragment.

### 3c. Mail drain + improved instructions
- [x] **655620a1** — Witness patrol v8: `gt mail drain` step archives stale
  protocol messages (>30 min). Batch processing when inbox > 10 messages.
  **Done:** Added Mail Drain section to witness prompt.
- [x] **9fb00901** — Overhauled mail instructions in crew and polecat templates:
  `--stdin` heredoc pattern, address format docs, common mistakes section.
  **Done:** `--stdin` heredoc pattern in global fragment. Common mail mistakes
  + address format in crew prompt.
- [x] **8eb3d8bb** — Generic names (`alice/`) in crew template mail examples.
  **Done:** Changed wolf → alice in crew prompt examples.

---

## 4. Batch-then-Bisect Merge Queue

Fundamental change to refinery processing model.

- [-] **7097b85b** — Batch-then-bisect merge queue. SDK-level Go machinery.
  Our event-driven one-branch-per-wisp model is intentional. N/A for pack.
- [-] **c39372f4** — `gt mq post-merge` replaces multi-step cleanup. Our direct
  work-bead model (no MR beads) already handles this atomically. N/A.
- [x] **048a73fe** — Duplicate bug check before filing pre-existing test failures.
  Added `bd list --search` dedup check to handle-failures step.
- **Also ported:** ZFC decision table in refinery prompt, patrol-summary step
  in formula for audit trail / handoff context.

---

## 5. Refinery Target-Aware Merging

Support for integration branches (not just always merging to main).

- [x] **75b72064 + 15b4955d + 33534823 + 87caa55d** — Target Resolution Rule.
  **Disposition:** No global toggle needed — polecat owns target via `metadata.target`,
  refinery reads it mechanically. Ported: FORBIDDEN clause for raw integration branch
  landing (prompt + formula), epic bead assignment for auto-land (formula), fixed
  command quick-reference to use `$TARGET` instead of hardcoded default branch.

---

## 6. Witness Patrol Improvements

### 6a. MR bead verification
- [-] **55c90da5** — Verify MR bead exists before sending MERGE_READY.
  **Disposition:** N/A — we don't use MR beads. Polecats assign work beads
  directly to refinery with branch metadata. The failure mode doesn't exist.

### 6b. Spawn storm detection
- [x] **70c1cbf8** — Track bead respawn count, escalate on threshold.
  **Disposition:** Implemented as exec automation `spawn-storm-detect` in
  maintenance pack. Script tracks reset counts in a ledger, mails mayor
  when any bead exceeds threshold. Witness sets `metadata.recovered=true`
  on reset beads to feed the detector.

### 6c. MQ verification in recovery
- [-] **b5553115** — Three-verdict recovery model.
  **Disposition:** N/A — our reset-to-pool model covers this. Work bead
  assignment to refinery IS submission. Witness already checks assignee
  before recovering. No intermediate MR state to verify.

### 6d. Policy decisions moved to prompts (ZFC)
- [x] **977953d8 + 3bf979db** — Remove hardcoded escalation policy.
  **Disposition:** Replaced "In ALL cases: notify mayor" with judgment-based
  notification table in witness formula and prompt. Routine pool resizes
  no longer generate mayor mail. Witness decides severity.

---

## 7. Root-Only Wisps Architecture

From batch 3 analysis (session summary).

- [x] Root-only wisps: `--root-only` flag added to all `bd mol wisp` calls
  in patrol formulas (deacon, witness, refinery) and polecat work formula.
  Formula steps are no longer materialized as child beads — agents read step
  descriptions directly from the formula definition. Reduces Dolt write churn
  by ~15x.
- [x] All `bd mol current` / `bd mol step done` references removed from
  shared templates (following-mol, propulsion), all role prompts, and all
  formula descriptions. Replaced with "read formula steps and work through
  them in order" pattern.
- [x] Crash recovery: agents re-read formula steps on restart and determine
  resume position from context (git state, bead state, last completed action).
  No step-tracking metadata needed on the wisp bead.
- **Disposition:** No new `gc` command needed (upstream's `gt prime` with
  `showFormulaSteps()` is unnecessary — the LLM reads formula steps directly).
  We keep the explicit `bd mol wisp`/`bd mol burn` dance but with `--root-only`.

---

## 8. Infrastructure Dogs (New Formulas)

### 8a. Existing dogs updated
- [x] **d2f9f2af** — JSONL Dog: spike detection + pollution firewall. New
  `verify` step between export and push. `spike_threshold` variable.
  **Done:** mol-dog-jsonl.formula.toml created with verify step.
- [x] **37d57150** — Reaper Dog: auto-close step for issues > 30 days
  (excluding epics, P0/P1, active deps). `stale_issue_age` variable.
  **Done:** mol-dog-reaper.formula.toml created. ZFC revert noted (no
  auto-close decisions in Go).
- [x] **bc9f395a** — Doctor Dog: structured JSON reporting model (advisory).
  **Then** 176b4963 re-adds automated actions with 10-min cooldowns.
  **Then** 89ccc218 reverts to configurable advisory recommendations.
  **Done:** mol-dog-doctor.formula.toml uses advisory model. References
  `gc dolt cleanup` for orphan detection.

### 8b. New dog formulas
- [x] **739a36b7** — Janitor Dog: cleans orphan test DBs on Dolt test server.
  4 steps: scan, clean, verify (production read-only check), report.
  **Done:** mol-dog-stale-db.formula.toml. References `gc dolt cleanup --force`.
- [x] **85887e88** — Compactor Dog: flattens Dolt commit history. Steps:
  inspect, compact, verify, report. Threshold 10,000. Formula-only pattern.
  **Done:** mol-dog-compactor.formula.toml.
- [x] **1123b96c** — Surgical rebase mode for Compactor. `mode` config
  ('flatten'|'surgical'), `keep_recent` (default 50).
  **Done:** Included in mol-dog-compactor.formula.toml vars.
- [x] **3924d560** — SQL-based flatten on running server. No downtime.
  **Done:** mol-dog-compactor.formula.toml uses SQL-based approach.
- [x] mol-dog-phantom-db.formula.toml — Detect phantom database resurrection.
- [x] mol-dog-backup.formula.toml — Database backup verification.

### 8c. Dog lifecycle
- [x] **b4ed85bb** — `gt dog done` auto-terminates tmux session after 3s.
  Dogs should NOT idle at prompt.
  **Done:** Dog prompt updated with auto-termination note.
- [x] **427c6e8a** — Lifecycle defaults: Wisp Reaper (30m), Compactor (24h),
  Doctor (5m), Janitor (15m), JSONL Backup (15m), FS Backup (15m),
  Maintenance (daily 03:00, threshold 1000).
  **Done:** 7 automation wrappers in `maintenance/formulas/automations/mol-dog-*/`
  dispatch existing dog formulas on cooldown intervals via the generic automation
  system. No Go code needed — ZFC-compliant.

### 8d. CLI: `gc dolt cleanup`
- [x] `gc dolt cleanup` — List orphaned databases (dry-run).
- [x] `gc dolt cleanup --force` — Remove orphaned databases.
- [x] `gc dolt cleanup --max N` — Safety limit (refuse if too many orphans).
- [x] City-scoped orphan detection: `FindOrphanedDatabasesCity`, `RemoveDatabaseCity`.
- [x] Dolt package synced from upstream at 117f014f (25 commits of drift resolved).

### 8e. Dolt-health pack extraction
- [x] Dolt health formulas extracted from gastown into standalone reusable
  pack at `examples/dolt-health/`. Dog formulas + exec automations.
- [x] Fallback agents (`fallback = true`) — pack composition primitive.
  Non-fallback wins silently over fallback; two fallbacks keep first loaded.
  `resolveFallbackAgents()` runs before collision detection.
- [x] Dolt-health pack ships a `fallback = true` dog pool so it works
  standalone. When composed with maintenance (non-fallback dog), maintenance wins.
- [x] `pack.requires` validation at city scope via `validateCityRequirements()`.
- [x] Hybrid session provider (`internal/session/hybrid/`) — routes sessions
  to tmux (local) or k8s (remote) based on name matching. Registered as
  `provider = "hybrid"` in providers.go.

---

## 9. Prompt Template Updates

### 9a. Mayor
- [x] **4c9309c8** — Rig Wake/Sleep Protocol: dormant-by-default workflow.
  All rigs start suspended. Mayor resumes/suspends as needed.
  **Done:** Added Rig Wake/Sleep Protocol section + suspend/resume command table.
- [-] **faf45d1c** — Fix-Merging Community PRs: `Co-Authored-By` attribution.
  N/A — not present in Gas Town upstream mayor template either.
- [-] **39962be0** — `auto_start_on_boot` renamed to `auto_start_on_up`.
  N/A — Gas City uses `Suspended` field, not `auto_start_on_boot`.

### 9b. Crew
- [x] **12cf3217** — Identity clarification: "You are the AI agent (crew/...).
  The human is the Overseer."
  **Done:** Added explicit identity line to crew prompt.
- [-] **faf45d1c** — Fix-Merging Community PRs section.
  N/A — not present in Gas Town upstream crew template either.
- [x] **9fb00901** — Improved mail instructions with `--stdin` heredoc pattern,
  common mistakes section.
  **Done:** Added `--stdin` heredoc pattern and common mail mistakes to crew
  prompt. Generic example names (alice instead of wolf).

### 9c. Boot
- [x] **383945fb** — ZFC fix: removed Go decision engine from degraded triage.
  Decisions (heartbeat staleness, idle detection, backoff labels, molecule
  progress) now belong in boot formula, not Go code.
  **Done:** Boot already uses judgment-based triage (ZFC-correct). Added
  decision summary table, mail inbox check step, and explicit guidance.

### 9d. Template path fix
- [x] (batch 3) Template paths changed from `~/gt` to `{{ .TownRoot }}`.
  **Done:** All `~/gt` references replaced with `{{ .CityRoot }}` in mayor,
  crew, and polecat prompts.

---

## 10. Formula System Enhancements

- [-] **67b0cdfe** — Formula parser now supports: Extends (composition), Compose,
  Advice/Pointcuts (AOP), Squash (completion behavior), Gate (conditional
  step execution), Preset (leg selection). Previously silently discarded.
  N/A — Gas City's formula parser is intentionally minimal (Name, Steps with
  DAG Needs). Advanced features (convoys, AOP, presets) are spec-level concepts
  to be added when needed, not ported from Gas Town's accretion.
- [-] **330664c2** — GatesParallel=true by default: typecheck, lint, build,
  test run concurrently in merge queue (~2x gate speedup).
  N/A — Gas City formulas use `Needs` for DAG ordering. Gate step types
  don't exist yet. When added, parallelism would be the default.

---

## 11. ZFC Fixes (Zero Framework Cognition)

Go code making decisions that belong in prompts — moved to prompts.

- [-] **915f1b7e + f61ff0ac** — Remove auto-close of permanent issues from
  wisp reaper. Reaper only operates on ephemeral wisps.
  N/A — Gas City wisp GC only deletes closed molecules past TTL. No
  auto-close decisions in Go.
- [x] **977953d8** — Witness handlers report data, don't make policy decisions.
  Done in Section 6d.
- [x] **3bf979db** — Remove hardcoded role names from witness error messages.
  Done in Section 6d.
- [-] **383945fb** — Remove boot triage decision engine from Go.
  N/A — Gas City reconciler is purely mechanical. Triage is data collection;
  all decisions driven by config (`max_restarts`, `restart_window`,
  `idle_timeout`) and agent requests.
- [x] **89ccc218** — Doctor dog: advisory recommendations, not automated actions.
  Done in Section 8a.
- [-] **eb530d85** — Restart tracker crash-loop params configurable via
  `patrols.restart_tracker`.
  N/A — Gas City's `[daemon]` config has `max_restarts` and `restart_window`
  fully configurable since inception. Crash tracker disabled if max_restarts ≤ 0.
- **Remaining:** `roleEmoji` map in `tmux.go` is a display-only hardcode
  (see 12a — deferred, low priority).

---

## 12. Configuration / Operational

### 12a. Per-role config
- [-] **bd8df1e8** — Dog recognized as role in AgentEnv(). N/A — Gas City
  has no role concept; per-agent config via `[[agents]]` entries.
- [-] **e060349b** — `worker_agents` map. N/A — crew members are individual
  `[[agents]]` entries with full config blocks.
- [-] **2484936a** — Role registry (`autonomous`, `emoji`). N/A — `autonomous`
  is prompt-level (propulsion.md.tmpl). `emoji` field on Agent would remove
  the hardcoded roleEmoji map in tmux.go (ZFC violation) — deferred, low priority.

### 12b. Rig lifecycle
- [x] **95eff925** — `auto_start_on_boot` per-rig config. Gas City already has
  `rig.Suspended`. Added `gc rig add --start-suspended` for dormant-by-default.
  Sling enforcement deferred (prompt-level: mayor undocks rigs).
- [x] **d2350f27** — Polecat pool: `pool-init` maps to `pool.min` (reconciler
  pre-spawns). Local branch cleanup added to mol-polecat-work submit step
  (detach + delete local branch after push, before refinery assignment).

### 12c. Operational thresholds (ZFC)
- [-] **3c1a9182 + 8325ebff** — OperationalConfig: 30+ hardcoded thresholds
  now configurable via config sub-sections (session, nudge, daemon, deacon,
  polecat, dolt, mail, web).
- N/A — Gas City was designed config-first; thresholds were never hardcoded.
  `[session]`, `[daemon]`, `[dolt]`, `[automations]` cover all operational
  knobs. JSON schema (via `genschema`) documents all fields with defaults.

### 12d. Multi-instance isolation
- [x] **33362a75** — Per-city tmux sockets via `tmux -L <cityname>`. Prevents
  session name collisions across cities.
- **Done:** `[session] socket` config field. `SocketName` flows through tmux
  `run()`, `Attach()`, and `Start()`. Executor interface + fakeExecutor tests.

### 12e. Misc operational
- [x] **dab8af94** — `GIT_LFS_SKIP_SMUDGE=1` during worktree add. Reduces
  polecat spawn from ~87s to ~15s.
  **Done:** Added to worktree-setup.sh.
- [x] **a4b381de** — Unified rig ops cycle group: witness, refinery, polecats
  share one n/p cycle group.
  **Done:** cycle.sh updated with unified rig ops group.
- [x] **6ab5046a** — Town-root CLAUDE.md template with operational awareness
  guidance for all agents.
  **Done:** `operational-awareness` global fragment with identity guard + Dolt
  diagnostics-before-restart protocol.
- [x] **b06df94d** — `--to` flag for mail send. Accepts well-known role addresses.
  **Done:** `--to` flag added. Recipients validated against config agents (ZFC).
- [-] **9a242b6c** — Path references fixed: `~/.gt/` to `$GT_TOWN_ROOT/`.
  N/A — Gas Town-only path fix. Gas City uses `{{ .CityRoot }}` template vars.

---

## 13. New Formulas (from batch 3)

- [~] 9 new formula files identified: idea-to-plan pipeline + dog formulas.
  Dog formulas done (Section 8). Idea-to-plan pipeline blocked on Section 1
  (persistent polecat pool changes dispatch model).
- [~] Witness behavioral fixes: persistent polecat model, swim lane rule.
  Blocked on Section 1 (persistent polecat pool).
- [~] Polecat persist-findings.
  Blocked on Sections 1/2 (polecat lifecycle).
- [-] Settings: `skipDangerousModePermissionPrompt`.
  N/A — Gas Town doesn't have this setting either. Gas City already handles
  permission warnings via `AcceptStartupDialogs()` in dialog.go.
- [-] Dangerous-command guard hooks.
  N/A — prompts already describe preferred workflow (push to main, use
  worktrees). Hard-blocking PRs and feature branches limits implementer
  creativity. The witness wisp-vs-molecule guards remain (correctness),
  but workflow guards are prompt-level guidance, not enforcement.
- **Action:** Items 1-3 unblock after Sections 1/2.

---

## Delta 2: Commits 977953d8..04e7ed7c (2026-03-01 to 2026-03-03)

151 non-merge, non-backup commits. Organized by theme for triage.
Cross-references to Delta 1 sections (S1-S13) where themes continue.

---

## 14. ZFC Fixes (Delta 2)

Extends Section 11. Go code making decisions that belong in prompts or
formulas — refactored or removed.

- [-] **ee0cef89** — Remove `IsBeadActivelyWorked()` (ZFC violation). Go was
  deciding whether a bead was "actively worked" — a judgment call that belongs
  in the agent prompt via bead state inspection.
  N/A — Gas City never had this function. Witness prompt already handles
  orphaned bead recovery and dedup at the prompt layer (lines 85-104).
- [-] **7e7ec1dd** — Doctor Dog delegated to formula. 565 lines of Go decision
  logic replaced with formula-driven advisory model. The Go code only provides
  data; the formula makes decisions.
  N/A — Gas City was formula-first for Doctor Dog. `mol-dog-doctor.formula.toml`
  in `dolt-health/` topology already uses the advisory model upstream is
  converging toward. No imperative Go health checks ever existed.
- [-] **efcb72a8** — Wisp reaper restructured as thin orchestrator. Decision
  logic (which wisps to reap, when) moved to formula; Go code only executes
  the mechanical reap operation.
  N/A — Gas City has no wisp reaper Go code. Our `mol-dog-reaper.formula.toml`
  already has the 5-step formula (scan → reap → purge → auto-close → report)
  that upstream's Go is converging toward.
- [-] **1057946b** — Convoy stuck classification. Replaced Go heuristics for
  "is this convoy stuck?" with raw data surfacing. Agent reads convoy state
  and decides.
  N/A — Gas City has no convoy Go code. Convoys are an open design item
  (FUTURE.md). When built, will surface raw data per ZFC from the start.
- [-] **4cc3d231** — Replace hardcoded role strings with constants. Removes
  string literals like `"polecat"`, `"witness"` from Go logic paths.
  N/A — Gas City has zero hardcoded roles by design. Upstream centralizes
  role names as Go constants; Gas City eliminates them entirely. The
  `roleEmoji` map remains a known deferred item from S11.
- [-] **a54bf93a** — Centralize formula names as constants. Formula name
  strings gathered into a single constants file instead of scattered literals.
  N/A — Gas City discovers formula names from TOML files at runtime.
  Formula names live in config, not Go constants.
- [-] **1cae020a** — Typed `ZombieClassification` replaces string matching.
  Go switches on typed enum instead of `if classification == "zombie"`.
  N/A — Gas City has no compiled zombie classifier. Witness handles
  zombie/stuck detection via prompt-level judgment.
- [x] **376ca2ef** — Compactor ZFC exemption documented. Compactor's Go-level
  decisions (when to compact, threshold checks) explicitly documented as
  acceptable ZFC exceptions with rationale.
  Done: `mol-dog-compactor.formula.toml` updated to v2 — added surgical mode,
  ZFC exemption section, concurrent write safety docs, `mode`/`keep_recent`
  vars, `dolt_gc` in compact step, pre-flight row count verification.
  Also updated `mol-dog-reaper.formula.toml` to v2 — added anomaly detection,
  mail purging, parent-check in reap query, `mail_delete_age`/`alert_threshold`/
  `dry_run`/`databases`/`dolt_port` vars.

---

## 15. Config-Driven Thresholds (Delta 2)

Extends Section 12c. More hardcoded thresholds moved to config.

- [-] **f71e914b** — Witness patrol thresholds config-driven (batch 1).
  Heartbeat staleness, idle detection, and escalation thresholds now read
  from config instead of Go constants.
  N/A — Gas City was config-first from inception. `[daemon]` section has
  `max_restarts`, `restart_window`, `idle_timeout`, `health_check_interval`
  all configurable. Thresholds were never hardcoded.
- [-] **a3e646e3** — Daemon/boot/deacon thresholds config-driven (batch 2).
  Boot triage intervals, deacon patrol frequency, and daemon restart windows
  all configurable.
  N/A — same as above. Gas City daemon config covers these knobs.

---

## 16. Formula & Molecule Evolution (Delta 2)

Extends Sections 8 and 10. New formula capabilities and molecule lifecycle
improvements.

- [x] **ecc6a9af** — `pour` flag for step materialization. When set, formula
  steps are materialized as child beads (opt-in). Default remains root-only
  wisps per Section 7.
  Done: Added `Pour` and `Version` fields to `Formula` struct in
  `internal/formula/formula.go`. Parser preserves the field; schema
  regenerated. Behavioral use (creating child beads) deferred until
  molecule creation supports it.
- [x] **8744c5d7** — `dolt-health` step added to deacon patrol formula.
  Deacon checks Dolt server health as part of its regular patrol cycle.
  Done: Added `gc dolt health` command (`--json` for machine-readable output)
  to `internal/dolt/health.go` + `cmd/gc/cmd_dolt.go`. Checks server status,
  per-DB commit counts, backup freshness, orphan DBs, zombie processes.
  Added `dolt-health` step to deacon patrol formula with threshold table
  and remediation actions (compactor dispatch, backup nudge, orphan cleanup).
  Existing `system-health` step (gc doctor) retained as a separate step.
- [~] **f11e10c3** — Patrol step self-audit in cycle reports. Patrol formulas
  emit a summary of which steps ran, skipped, or errored at end of cycle.
  Deferred: requires `gc patrol report --steps` (no patrol reporting CLI yet).
  Concept is valuable — implement when patrol reporting infrastructure exists.
- [x] **3accc203** — Deacon Capability Ledger. Already at parity: all 6 role
  templates include `shared/capability-ledger.md.tmpl` (work/patrol/merge
  variants). Hooked/pinned terminology also already correct in propulsion
  templates. Gas City factored upstream's inline approach into shared fragments.
- [x] **117f014f** — Auto-burn stale molecules on re-dispatch. Confirmed Gas
  City had the same bug: stale wisps from failed mid-batch dispatch blocked
  re-sling. Fixed: `checkNoMoleculeChildren` and `checkBatchNoMoleculeChildren`
  now skip closed molecules and auto-burn open molecules on unassigned beads.
- [-] **9b4e67a2** — Burn previous root wisps before new patrol. Gas City's
  controller-level wisp GC (`wisp_gc.go`) handles accumulation on a timer.
  Upstream needed per-cycle GC because Gas Town lacks controller-level GC.
- [-] **53abdc44** — Pass `--root-only` to `autoSpawnPatrol`. Gas City is
  root-only by default (MolCook creates no child step beads). Already at parity.
- [-] **5b9aafc3** + **5769ea01** — Wisp orphan prevention. Gas City's
  formula-driven patrol loop (agent pours next wisp before burning current)
  avoids the status-mismatch bug that caused duplicate wisps in Gas Town's
  Go-level autoSpawnPatrol.

---

## 17. Witness & Health Patrol (Delta 2)

Extends Section 6. Witness patrol behavioral improvements and health
monitoring enhancements.

- [-] **cee8763f** + **35353a80** — Handoff cooldown. Gas Town Go-level patrol
  logic. In Gas City, anti-ping-pong behavior is prompt guidance in the
  witness formula, not SDK infrastructure (ZFC principle).
- [x] **ac859828** — Verify work on main before resetting abandoned beads.
  Added merge-base check to witness patrol formula Step 3: if branch is
  already on main, close the bead instead of resetting to pool.
- [-] **a237024a** — Spawning state in witness action table. Gas Town
  Go-level survey logic. Gas City witness checks live session state via CLI;
  spawning agents have active sessions visible to the witness.
- [-] **c5d486e2** — Heartbeat v2: agent-reported state. Requires Go changes
  to agent protocol. Gas City uses inference-based health (wisp freshness,
  bead timestamps). Self-reported state deferred to heartbeat SDK work.
- [-] **33536975** — Witness race conditions. Gas Town-internal fix for
  concurrent witness patrol runs conflicting on Dolt writes. N/A — Gas City
  uses filesystem beads with atomic writes.
- [-] **1cd600fc** + **21ec786e** — Structural identity checks. Gas Town
  internal validation that agent identity matches expected role assignment.
  N/A — Gas City agents are identified by config name, not role.

---

## 18. Sling & Dispatch (Delta 2)

Extends Section 12b. Dispatch improvements and error handling.

- [-] **a6fa0b91** + **5c9c749a** + **65ee6d6d** — Per-bead respawn circuit
  breaker. Already covered by Gas City's `spawn-storm-detect` exec
  automation in maintenance pack (ported in S6b).
- [-] **783cbf77** — `--agent` override for formula run. Gas City sling
  already takes target agent as positional arg. N/A.
- [-] **d980d0dc** — Resolve rig-prefixed beads in sling. Already at parity:
  `findRigByPrefix`, `beadPrefix`, `checkCrossRig` in cmd_sling.go.

### 18f. Convoy parity gaps (discovered during S18.2 review)

Gas Town convoys are a cross-rig coordination mechanism with reactive
event-driven feeding. Gas City has convoy CRUD/status/autoclose but is
missing the coordination layer:

- [ ] **Reactive feeding** — `feedNextReadyIssue` triggered by bead close
  events via `CheckConvoysForIssue`. Without this, convoy progress depends
  on polling (patrol cycles finding stranded work).
- [ ] **`tracks` dependency type** — convoys use `tracks` deps to link
  issues across rigs. Gas City beads use parent-child only.
- [ ] **Cross-rig dependency resolution** — `isIssueBlocked` checks
  `blocks`, `conditional-blocks`, `waits-for` dep types with cross-rig
  status freshness.
- [ ] **Staged convoy statuses** — `staged_ready`, `staged_warnings`
  prevent feeding before convoy is launched.
- [ ] **Rig-prefix dispatch** — `rigForIssue` + `dispatchIssue` routes
  each convoy leg to its rig's polecat pool based on bead ID prefix.
  Gas City sling has prefix resolution but convoy doesn't use it.
- [-] **9f33b97d** — Nil `cobra.Command` guard. Gas Town internal defensive
  check. N/A.
- [-] **5d9406e1** — Prevent duplicate polecat spawns. Gas Town internal
  race condition in spawn path. N/A — Gas City's reconciler handles this
  via config-driven pool sizing.

---

## 19. Convoy Improvements (Delta 2)

New theme. Convoy is Gas Town's multi-leg work coordination mechanism
(a molecule whose steps route to different agents).

- [-] **22254cca** + **c9f2d264** — Custom convoy statuses: `staged_ready`
  and `staged_warnings`. Captured in S18f convoy parity gaps (staged
  convoy statuses).
- [-] **860cd03a** — Non-slingable blockers in wave computation. Captured
  in S18f convoy parity gaps (cross-rig dependency resolution).
- [-] **85b75405** — Capture `bd` stderr in convoy ops. Gas Town internal
  error handling improvement. N/A.

---

## 20. Pre-Verification & Merge Queue (Delta 2)

Extends Section 4. Adds a pre-verification step before merge queue entry.

- [~] **2966c074** — Pre-verify step in polecat work formula. Concept is
  sound (polecat runs build+test before submission to reduce refinery
  rejects). Deferred: add pre-verify step between self-review and
  submit-and-exit in mol-polecat-work when we tune the pipeline.
- [-] **73d4edfe** — `gt done --pre-verified` flag. Gas Town CLI flag.
  Gas City can use bead metadata (`--set-metadata pre_verified=true`)
  directly. N/A.
- [~] **5fe1b0f6** — Refinery pre-verification fast-path. Deferred with
  S20 pre-verify step above — refinery checks `metadata.pre_verified`
  and skips its own test run.
- [-] **07b890d0** — `MRPreVerification` bead fields. Gas Town MR bead
  infrastructure. N/A — Gas City uses work beads directly.
- [-] **b24df9ea** — Remove "reject back to polecat" from refinery template.
  Gas Town template simplification. Our refinery formula already handles
  rejection cleanly via pool reset.
- [-] **33364623**, **45541103**, **e2695fd6** — Gas Town internal MR/refinery
  fixes. Bug fixes in MR state machine. N/A.

---

## 21. Persistent Polecat Pool (Delta 2)

Extends Section 1. Incremental improvements to the persistent polecat model.

- [-] **4037bc86** — Unified `DoneIntentGracePeriod` constant. Gas Town Go
  daemon code. N/A.
- [-] **e09073eb** — Idle sandbox detection matches actual `cleanupStatus`.
  Gas Town Go witness code. N/A.
- [-] **082fbedc** + **5fa9dc2b** — Docs: remove "Idle Polecat Heresy".
  Gas Town moved to persistent polecats where idle is normal. Gas City
  polecats are still ephemeral (spawn, work, exit) — the Heresy framing
  is correct for our model. Update when/if we add persistent polecats.
- [-] **c6173cd7** — `gt done` closes hooked bead regardless of status.
  Gas Town `gt done` CLI code. N/A — Gas City polecats use `bd update`
  directly in the formula submit step.

---

## 22. Low-Relevance / Gas Town Internal

Bulk N/A items grouped by sub-theme for fast scanning. These are Gas Town
implementation details that don't affect Gas City's architecture or
configuration patterns.

### 22a. TOCTOU race fixes
- [-] ~7 commits fixing time-of-check/time-of-use races in compiled Go code.
  Gas Town-specific concurrency bugs in daemon, witness, and sling hot paths.
  N/A — Gas City's architecture avoids these patterns (filesystem beads with
  atomic rename, no concurrent Dolt writes).

### 22b. OTel / Telemetry
- [-] ~10 commits adding/refining OpenTelemetry spans, trace propagation,
  and metrics collection. Gas City has no OTel integration. N/A.

### 22c. Dolt operational
- [-] ~10 commits for Dolt SQL admin operations, server restart logic,
  connection pool tuning, and query optimization. Gas City uses filesystem
  beads, not Dolt. N/A.

### 22d. Daemon PID / lifecycle
- [-] ~7 commits improving daemon PID file handling, process discovery,
  and graceful shutdown sequencing. Gas City's controller uses `flock(2)`
  for singleton enforcement and direct process table queries. N/A.

### 22e. Proxy / mTLS sandbox
- [-] ~3 commits for sandbox proxy mTLS certificate rotation and proxy
  health checks. Gas Town infrastructure for isolated polecat networking.
  N/A — Gas City sandboxes are local worktrees.

### 22f. Namepool custom themes
- [-] ~6 commits adding themed name pools (e.g., mythology, astronomy) for
  agent naming. Gas Town-specific flavor. N/A — Gas City uses config-defined
  agent names.

### 22g. Agent memory
- [~] ~3 commits for `gt remember` / `gt forget` commands — persistent
  agent memory across sessions. Deferred — interesting capability but
  requires `gc remember`/`gc forget` CLI commands and agent bead metadata
  fields. Low priority vs core SDK work.

### 22h. Cross-platform / build / CI / deps
- [-] ~12 commits for Windows/macOS compatibility, CI pipeline fixes,
  dependency updates, and build system changes. Gas Town build infrastructure.
  N/A.

### 22i. Misc operational
- [-] ~15 commits for miscellaneous Gas Town bug fixes: tmux session cleanup,
  log rotation, error message improvements, CLI help text updates. N/A.

### 22j. Docs
- [-] ~2 commits: agent API inventory and internal architecture docs.
  Informational only — already captured in Gas City's spec documents.

---

## Review Order (Suggested)

1. [~] **Persistent Polecat Pool** (Section 1) — deferred, requires sling + `gc done` + idle state infrastructure
2. [~] **Polecat Work Formula v7** (Section 2) — deferred, depends on S1 persistent polecat infrastructure
3. [x] **Communication Hygiene** (Section 3) — nudge-first in global fragment + role-specific rules
4. [x] **Batch-then-Bisect MQ** (Section 4) — refinery formula rewrite
5. [x] **Witness Patrol** (Section 6) — many behavioral changes
6. [x] **Prompt Updates** (Section 9) — wake/sleep, identity, triage, paths
7. [x] **ZFC Fixes** (Section 11) — all clean, Gas City designed ZFC-first
8. [x] **Infrastructure Dogs** (Section 8) — new formulas + dolt-health extraction + fallback agents
9. [x] **Config/Operational** (Section 12) — SDK-level features
10. [-] **Formula System** (Section 10) — N/A, designed minimal-first
11. [~] Remaining sections (5, 7, 13) — 5+7 done; 13.4-5 done; 13.1-3 deferred (blocked on S1/S2)
12. [-] **ZFC Fixes Delta 2** (S14) — all N/A (Gas Town Go code)
13. [x] **Formula/Molecule Delta 2** (S16) — pour flag, auto-burn stale molecules, dolt-health step, capability ledger already at parity
14. [-] **Witness/Health Delta 2** (S17) — verify-before-reset ported to witness formula; rest N/A (Go code)
15. [-] **Sling/Dispatch Delta 2** (S18) — all N/A; convoy parity gaps captured in S18f
16. [~] **Pre-verification Delta 2** (S20) — deferred (polecat pre-verify + refinery fast-path)
17. [-] **Persistent Polecat Delta 2** (S21) — all N/A (Go code, persistent polecat model)
18. [-] **Low-relevance bulk** (S22) — TOCTOU, OTel, Dolt, daemon, proxy, namepool, build/CI
19. [ ] **Convoy parity** (S18f) — reactive feeding, tracks deps, staged statuses, cross-rig dispatch
