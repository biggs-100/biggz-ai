# Proposal: sync-config

## Intent

Add `biggz sync` to selectively re-deploy skills, config, prompts, and commands — a post-update and manual-edit workflow. Upgrade `internal/filemerge` with deep merge and atomic write-with-compare so sync is idempotent and safe.

## Scope

### In Scope
- `biggz sync [--skills] [--config] [--prompts] [--commands] [--all] [--dry-run]` — selective re-deploy
- `WriteFileAtomic` — new signature with content comparison, returning `WriteResult{Changed, Created}`. Clean rename, no deprecation.
- `MergeJSONC` deep merge — recursive merge with `__replace__` sentinel for atomic object replacement
- Update all `WriteFile` callers to `WriteFileAtomic`

### Out of Scope
- Backup/rollback pipeline
- Post-sync verification or re-run of checks
- State persistence across syncs
- Diff or preview output (beyond dry-run exit code)

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `cli`: Add `sync` subcommand dispatched via existing switch router, with selective flags and dry-run mode.
- `filemerge`: Upgrade `MergeJSONC` to deep recursive merge; replace `WriteFile` with `WriteFileAtomic`.

## Approach

1. **WriteFileAtomic**: Compare content bytes before writing — skip if identical. Atomic via temp+rename. Return `WriteResult{Changed, Created}`. Update all 3 callers in `internal/install/`.
2. **Deep merge**: `MergeJSONC` merges nested map values recursively. `"__replace__": true` inside a merged object replaces the target entirely. Flat keys and non-map values override as before. All existing tests pass unchanged.
3. **Sync command**: `switch` case + `syncRun()` in `cmd/biggz/main.go`. Parse flags from `os.Args[2:]`. For each selected category, walk source dir and call `WriteFileAtomic`. Blind re-deploy — no diff logic.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/filemerge/json_merge.go` | Modified | Deep merge + `__replace__` sentinel |
| `internal/filemerge/writer.go` | Modified | `WriteFileAtomic` with content comparison |
| `internal/filemerge/writer_test.go` | Modified | Tests for new signature and idempotency |
| `internal/filemerge/json_merge_test.go` | Modified | Tests for deep merge scenarios |
| `cmd/biggz/main.go` | Modified | Add `sync` subcommand dispatch + `syncRun()` |
| `internal/install/install.go` | Modified | Update `WriteFile` → `WriteFileAtomic` |
| `internal/install/install_test.go` | Modified | Update test callers |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Sync overwrites user edits | Low | `WriteFileAtomic` compares content first — no-op on match |
| Deep merge breaks existing JSONC consumers | Low | Existing tests unchanged; new tests cover deep merge |
| `WriteFile` rename misses a caller | Med | Grep all usages; only 3 callers in `install.go` |

## Rollback Plan

Revert the commit. `WriteFileAtomic` returns `(WriteResult, error)` vs. `WriteFile`'s bare `error` — callers must be updated, making revert the only clean path.

## Dependencies

None.

## Success Criteria

- [ ] `biggz sync --dry-run --skills` reports without writing files
- [ ] `biggz sync` deploys all four categories; omitted flags skip their category
- [ ] `WriteFileAtomic` returns `Changed: false` when content is identical
- [ ] Deep merge: nested objects merge, `__replace__` replaces target, flat keys override
- [ ] All existing `filemerge` tests pass unchanged
