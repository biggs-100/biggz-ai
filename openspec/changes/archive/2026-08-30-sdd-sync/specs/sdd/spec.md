# Delta for sdd

## ADDED Requirements

### Requirement: Sync Phase Lifecycle

The system MUST introduce phase `sdd-sync` between `verify` and `archive`. `sdd-sync` MUST sync file-backed deltas to `openspec/specs/` without moving the change to archive. Lifecycle order MUST be `proposal → spec → design → tasks → apply → verify → sync → archive`.

#### Scenario: Verify-pass exposes sync before archive

- GIVEN `verifyReport` is `PASS`, tasks are `allDone`, and at least one delta spec exists under `openspec/changes/{change}/specs/`
- WHEN status derives `nextRecommended`
- THEN it MUST be `sync` (not `archive`) until sync clears

#### Scenario: Sync clears enables archive

- GIVEN sync successfully applied deltas to `openspec/specs/`
- WHEN status re-derives
- THEN `nextRecommended` MUST become `archive` and `sync` MUST not reappear unless deltas change

#### Scenario: No deltas or non-file store skips sync

- GIVEN declared store is `engram`/`none` or no delta specs exist
- WHEN status derives
- THEN `nextRecommended` MUST NOT be `sync` and `blockedReasons` MUST not contain sync guards

### Requirement: Sync Execution Contract

The system MUST provide agent `sdd-sync`, skill `internal/assets/skills/sdd-sync/SKILL.md`, prompt `sdd-sync.md`, and implementation `internal/sdd/openspec-deltas.go` + `internal/sdd/sync.go` porting ADDED/MODIFIED/REMOVED from `lib/openspec-deltas.ts` 1:1 without auto-commit, child subagents, or archive move.

#### Scenario: Sync executor without archive move

- GIVEN `openspec/changes/{change}/specs/sdd/spec.md` contains valid deltas
- WHEN `sdd-sync` executes on `openspec` store
- THEN `openspec/specs/sdd/spec.md` MUST reflect deltas and `openspec/changes/{change}/` MUST still exist

#### Scenario: No commit created

- GIVEN sync completed successfully
- WHEN git log is inspected
- THEN no new commit with `sdd-sync` auto-commit MUST exist
