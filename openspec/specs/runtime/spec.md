# Runtime Specification

## Purpose

Runtime covers platform and execution primitives: OpenCode grouped isolation as scheduling-only, Windows beta quoting/process control and handle-relative durable writes, cooperative filecoord locking with `BusyError`, Pi bounded manifest reads and progress tracking, review authority lock hardening, and Codex `hooks.json` atomic delegation. This spec also covers parity-gentle-69 ledger atomicity and dual-budget invariants ported from gentle `e8cc0fcc→782e8dfe`.

## Requirements

### Requirement: Grouped Isolation and Windows Beta

The system MUST implement OpenCode grouped isolation where isolation applies to scheduling only, not a security boundary. On Windows, the system MUST correctly quote paths, handle `rundll32`/`xdg-open` branching, and pass platform-specific process control and lock primitives. Cooperative filecoord lock acquisition MUST be one non-blocking attempt: contention returns `BusyError` without mutation, and caller owns retry pacing.

#### Scenario: Grouped isolation is scheduling-only

- GIVEN OpenCode background subagents run under grouped isolation
- WHEN two lanes schedule concurrent attempts
- THEN ordering MUST be coordinated via scheduling, not via filesystem security isolation

#### Scenario: Windows path and process handling

- GIVEN `biggz` runs on `windows`
- WHEN it quotes a change root or spawns a hook
- THEN it MUST use Windows-safe quoting and `rundll32`/`cmd` branching and MUST NOT attempt Unix-only `os.Rename` atomic replace

#### Scenario: Cooperative lock contention is non-mutating

- GIVEN `filecoord` lock for target `internal/sdd/status.go` is held
- WHEN another `Acquire` attempts the same target
- THEN it MUST return `BusyError` and MUST NOT mutate the protected resource

### Requirement: Pi Progress, Cooperative Locking, and Codex Hooks

The system MUST expose Pi progress and manifest resolution with bounded reads (`MaxPackageManifestBytes`), explicit `manifest-too-large` / `malformed-manifest` kinds, and non-mutating reads. Review authority stores MUST use hardened cooperative `MaintenanceLock` / `AuthorityFileLock` with `no-follow` open and PID/host as non-authoritative metadata. Codex `hooks.json` skill-registry refresh via `SessionStart` MUST be installed and removed atomically.

#### Scenario: Pi manifest bounded read

- GIVEN `package.json` exceeds `MaxPackageManifestBytes`
- WHEN `selectPackageBin` reads it
- THEN it MUST fail with `manifest-too-large` and MUST NOT mutate the manifest

#### Scenario: Pi progress tracking

- GIVEN an install pipeline with steps `prepare`/`apply`/`rollback`
- WHEN `ProgressFromExecution` aggregates result
- THEN `ProgressState` MUST report `Percent`, `CurrentStep`, and `HasFailures` deterministically

#### Scenario: Codex hooks delegation to backup

- GIVEN Codex global config `hooks.json` exists
- WHEN `ensureCodexSkillRegistryHook` runs
- THEN it MUST add `gentle-ai skill-registry refresh` under `hooks.SessionStart` atomically
- AND uninstall MUST remove only that hook entry, preserving other hooks

#### Scenario: Maintenance lock Timeout

- GIVEN review store `v2/LOCK` is held
- WHEN `BurnApprovedCompactAuthority` tries to acquire with `storeResetLockTimeout=2s`
- THEN it MUST fail with `ErrAuthorityLockTimeout` after timeout and MUST NOT delete authority

### Requirement: Ledger Verify-Before-Commit (CAS)

The system MUST enforce verify-before-commit: `commitRecordLocked` MUST replay `loadRecord(revision)` inside `withStoreLock` before `writeLedgerHead` and advance HEAD only if `candidate.Status.Revision == revision`. On mismatch it MUST fail closed without mutating HEAD/records.

#### Scenario: CAS refuses stale revision

- GIVEN HEAD is `R1`
- WHEN commit presents candidate `R0 != R1`
- THEN it MUST be rejected with CAS conflict and HEAD MUST stay `R1`

#### Scenario: HEAD advances on match

- GIVEN HEAD `R1` and candidate derived from `R1`
- WHEN commit presents `Revision == R1`
- THEN record MUST be persisted and HEAD MUST advance to `R2 == sha256(canonical)`

#### Scenario: Concurrent serialize

- GIVEN two writers loaded `R1` and writer A committed `R1→R2`
- WHEN writer B attempts `R1→R3`
- THEN B MUST be rejected with CAS conflict

### Requirement: Dual Budget Single Owner

The system MUST track `MaxAttempts` and `MaxChangedLines` with per-attempt `ChangedLines` and carried `CumulativeChangedLines`. The single predicate `CumulativeChangedLines + changedLines > MaxChangedLines` MUST own line-budget exhaustion for `Acquire`, `Finish`/`Settle`, and replay paths. No duplicate inequality SHALL exist.

#### Scenario: Blocked when cumulative+delta exceeds max

- GIVEN `Cumulative=300`, `MaxLines=400`
- WHEN acquiring `changedLines=150`
- THEN it MUST be rejected `blocked(budget_exhausted)` (300+150>400)

#### Scenario: Admitted within budget

- GIVEN `Cumulative=300`, `MaxLines=400`
- WHEN acquiring `changedLines=80`
- THEN it MUST succeed and after settle cumulative MUST be `380`

#### Scenario: Single predicate ownership

- GIVEN finish and status replay evaluate line budget
- WHEN inspecting code paths
- THEN both MUST call the same predicate without inline duplication

### Requirement: Interrupted Refund Capped at 2×

The system MUST use `runtimeAttemptDeliveredIncrement`: `interrupted && changedLines>0` increments delivered; otherwise not. `runtimeRefundedAttempts() <= MaxAttempts` MUST cap refunds to `2×MaxAttempts` total. `Acquire`/`Begin` MUST reject when cap exhausted (gentle 2243/2217).

#### Scenario: Interrupted with lines counts

- GIVEN `MaxAttempts=3`, `interrupted` `changedLines=20`
- WHEN delivered increment evaluated
- THEN it MUST count as delivered

#### Scenario: Interrupted without lines refunded

- GIVEN `interrupted` `changedLines=0`
- WHEN evaluated
- THEN it MUST NOT count as delivered and MUST be refund-eligible

#### Scenario: Blocks after 2× cap

- GIVEN `MaxAttempts=3`, `refunded==3`
- WHEN acquiring
- THEN it MUST be rejected `blocked(budget_exhausted)`

### Requirement: Rescope Exhausted Wedge

When exhausted (`DecisionRequired` or budgets reached), `Rescope` MUST require `MaxAttempts > carried CumulativeAttempts` AND `MaxChangedLines > carried CumulativeChangedLines`. Cumulative counters MUST never reset; attempts slice MUST be preserved.

#### Scenario: Refuses unless both exceed carried

- GIVEN exhausted `cumul=5/600`, `Max=5/600`
- WHEN `Rescope` proposes `MaxAttempts=5` `MaxLines=700`
- THEN it MUST be rejected (attempts not > carried)

#### Scenario: Admits when wedge satisfied

- GIVEN same exhausted ledger
- WHEN `Rescope` proposes `7/800`
- THEN it MUST succeed and cumulative MUST still read `5/600`

#### Scenario: Cumulative preserved

- GIVEN ledger 4 attempts 350 lines
- WHEN rescope
- THEN attempts length and cumulative sum MUST be unchanged

### Requirement: Runtime Record Rejection Taxonomy

The system MUST use single typed `RuntimeRecordRejectedError` for all record rejections (hash, schema, lineage, staleness). Callers MUST detect via `errors.As`. No parallel string-only paths SHALL exist.

#### Scenario: Hash mismatch typed

- GIVEN record bytes hash to `H' != H`
- WHEN `loadRecord` validates
- THEN it MUST return `RuntimeRecordRejectedError`

#### Scenario: Unified handling

- GIVEN any rejection
- WHEN checking `errors.As(err, *RuntimeRecordRejectedError)`
- THEN it MUST succeed

### Requirement: Background Subagents 4-Source Policy Resolution

The system MUST resolve background subagents policy via `internal/sdd/background.go` `resolveBackgroundSubagentsPolicy(cwd,opts)` with precedence `project > global > env > default off`. `project` reads `cwd/.biggz/background-subagents.json`, `global` reads `~/.biggz/background-subagents.json` (honoring `GENTLE_PI_CONFIG_HOME`/`BIGGZ_CONFIG_HOME`), `env` reads `BIGGZ_BACKGROUND_SUBAGENTS` (fallback `GENTLE_PI_BACKGROUND_SUBAGENTS`), strict 2-key decode `{"schema":"gentle-pi.background-subagents/v1","policy":"on"|"off"}` where extra keys → malformed → `off` no fallback, `malformed true`. Max 2 JSON reads per resolve. Capability `ready|absent` via `subagent_run` probe.
(Previously: `cwd/.pi/settings.json` and `~/.pi/agent/settings.json` with `backgroundSubagents` key and pi-scoped resolution)

#### Scenario: Project overrides global and env
- GIVEN `cwd/.biggz/background-subagents.json` `{"schema":"gentle-pi.background-subagents/v1","policy":"on"}` and global `off` and env `on`
- WHEN `resolveBackgroundSubagentsPolicy(cwd,opts)` called
- THEN it MUST return `policy on` `source project_file` `malformed false`

#### Scenario: Strict 2-key extra fails closed without fallback
- GIVEN project file `{"schema":"gentle-pi.background-subagents/v1","policy":"on","extra":1}`
- WHEN resolving
- THEN it MUST return `policy off` `malformed true` `source project_file` and MUST NOT consult global/env

#### Scenario: Malformed JSON fails closed
- GIVEN project file contains `{bad`
- WHEN resolving
- THEN it MUST return `policy off` `malformed true` and MUST NOT fall back

#### Scenario: Global beats env when project absent
- GIVEN no project file, global `{"schema":"gentle-pi.background-subagents/v1","policy":"off"}` and env `on`
- WHEN resolving
- THEN it MUST return `policy off` `source global_file`

#### Scenario: Env fallback and default
- GIVEN no project/global and env `on`
- WHEN resolving
- THEN it MUST return `policy on` `source environment`; with no env → `policy off` `source default`

### Requirement: Background Policy Delegate and Reporting

The system MUST own resolution in `internal/sdd/background.go`; `internal/opencode/background.go` `BackgroundPolicy`/`BackgroundResolution` and `internal/agents/pi/adapter.go` MUST be thin delegates to `sdd` resolver. `renderBackgroundSubagentsReport` MUST expose `source`, `policy`, `capability`, `malformed`, `projectFile`/`globalFile` existence and `envValue`, and `loadBackgroundSubagentsPolicy` MUST delegate to `resolve().policy`. Unknown env values MUST be reported as ignored; `wrote` outranked by project file MUST warn.
(Previously: pi adapter owned resolution; opencode recomputed sources)

#### Scenario: Delegate preserves policy
- GIVEN `sdd` resolver returns `on` from project
- WHEN `opencode/background.go` `BackgroundPolicy` called
- THEN it MUST return same `on` without recomputing sources

#### Scenario: Report renders source and malformed
- GIVEN resolution `{source:"project_file",policy:"off",malformed:true}`
- WHEN `renderBackgroundSubagentsReport` called with `capability ready`
- THEN output MUST contain `source=project_file`, `policy=off`, `malformed=true`, and `capability: ready`

#### Scenario: Pi adapter delegates
- GIVEN `pi` `ResolveBackgroundSubagentsPolicy` called
- WHEN invoked
- THEN it MUST delegate to `sdd` resolver and preserve precedence

### Requirement: Background Capability Probe and Disabled Reporting

The system MUST compute `capability` as `ready` when `subagent_run` tool or `pi-subagents` package is present, else `absent`. Status line MUST be `background subagents: <policy> (decided by <source>; capability: <capability>)` with `disabled|unmanaged` reporting when policy `off` or capability absent. `BIGGZ_BACKGROUND_SUBAGENTS` takes precedence over `GENTLE_PI_BACKGROUND_SUBAGENTS` when both set.

#### Scenario: Capability ready when subagent_run present
- GIVEN `subagent_run` registered in tool list
- WHEN capability probe runs
- THEN it MUST return `ready`

#### Scenario: Capability absent without probe
- GIVEN no `subagent_run` and no `pi-subagents` package
- WHEN probe runs
- THEN it MUST return `absent` and background launches MUST be inert

#### Scenario: Disabled reporting when policy off
- GIVEN `policy off` `capability absent`
- WHEN status line rendered
- THEN it MUST contain `policy: off` and `capability: absent` and `disabled/unmanaged` notice
