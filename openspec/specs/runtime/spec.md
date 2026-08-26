# Runtime Specification

## Purpose

Runtime covers platform and execution primitives: OpenCode grouped isolation as scheduling-only, Windows beta quoting/process control and handle-relative durable writes, cooperative filecoord locking with `BusyError`, Pi bounded manifest reads and progress tracking, review authority lock hardening, and Codex `hooks.json` atomic delegation.

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
