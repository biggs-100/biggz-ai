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

The system MUST resolve background subagents policy via `internal/agents/pi/adapter.go` `resolveBackgroundSubagentsPolicy(cwd,opts)` with precedence `project > global > env > default off`. `project` reads `cwd/.pi/settings.json`, `global` reads `gentleAiConfigHome(homeDir)` (`~/.pi/agent/settings.json`), `env` reads `BIGGZ_BACKGROUND_SUBAGENTS`/`PI_BACKGROUND_SUBAGENTS`. Malformed JSON MUST fail closed to `off` without falling back to the next source and MUST set `malformed true`.

#### Scenario: Project overrides global and env

- GIVEN `cwd/.pi/settings.json` contains `{"backgroundSubagents":"on"}` and global is `off` and env is `on`
- WHEN `resolveBackgroundSubagentsPolicy(cwd, opts)` is called
- THEN it MUST return `policy on` with source `project_file` and `malformed false`

#### Scenario: Malformed file fails closed without fallback

- GIVEN `cwd/.pi/settings.json` contains malformed JSON `{bad`
- WHEN `resolveBackgroundSubagentsPolicy` reads it
- THEN it MUST return `policy off` with `malformed true` and MUST NOT fall back to global or env

#### Scenario: Global off beats env on when project absent

- GIVEN no project file, global `~/.pi/agent/settings.json` is `{"backgroundSubagents":"off"}` and env is `on`
- WHEN resolving
- THEN it MUST return `policy off` from `global` and MUST NOT use env

#### Scenario: Default off when no source present

- GIVEN no project, no global, and no env variable
- WHEN resolving
- THEN it MUST return `policy off` with source `default`

### Requirement: Background Policy Delegate and Reporting

The system MUST provide `internal/opencode/background.go` `BackgroundPolicy`/`BackgroundResolution` as thin delegates to the `pi` resolver, and `internal/agents/pi/adapter.go` `renderBackgroundSubagentsReport` MUST expose `source`, `policy`, `capability`, and `malformed` for status reporting while `loadBackgroundSubagentsPolicy` delegates to `resolve().policy` and keeps `Resolve*` wrappers.

#### Scenario: Delegate preserves policy

- GIVEN `pi` resolver returns `on` from project
- WHEN `opencode/background.go` `BackgroundPolicy` is called
- THEN it MUST return the same `on` without recomputing sources

#### Scenario: Report renders source and malformed

- GIVEN resolution is `{source:"project_file", policy:"off", malformed:true}`
- WHEN `renderBackgroundSubagentsReport` is called
- THEN output MUST contain `source=project_file`, `policy=off`, and `malformed=true`
