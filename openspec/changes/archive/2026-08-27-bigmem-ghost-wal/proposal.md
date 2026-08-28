# Proposal: bigmem-ghost-wal (fix ghost WAL/SHM)

## Intent

Prevent Windows ghost `bigmem.db-wal` (0B) + `bigmem.db-shm` (>0B) left when Pi kills `sdd-apply` (30min/240s) or 2 DB holders overlap. Current `Open` only warns + falls back to `bigmem_recovered` without prevention, causing divergence. `DoctorFix` checkpoint+VACUUM+FTS is manual-only. Reclaim stale ghost in `Open` and flush WAL after `Save`/`Search`.

## Scope

### In Scope
- `Open` ghost prevention: `mtime>5min` + `O_EXCL` probe + `os.Remove` wal/shm + `PRAGMA wal_checkpoint(TRUNCATE)` before `sql.Open`
- `isGhostWAL` hardening: require `mtime>5min` for WAL=0/SHM>0 to be stale
- `Save`/`Search` deferred `wal_checkpoint(TRUNCATE)` after success
- Test `GhostWAL` with `t.TempDir` + synthetic wal/shm + `os.Chtimes` stale vs fresh

### Out of Scope
- D1 blobstore (`PutBlob`/`GetBlob`), D2 branching (`parent_id`/`leaf_id`)
- `sdd-apply` / Pi timeout/kill logic
- `DoctorFix` change, TUI/MCP/sync

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `bigmem`: ghost WAL prevention in `Open` (mtime+O_EXCL+Remove+checkpoint) + deferred checkpoint in `Save`/`Search` — delta to `openspec/specs/bigmem/spec.md`

## Approach

1. **Open pre-check**: `Stat` wal/shm; if `wal==0 && shm>0 && Since(ModTime())>5min`, `OpenFile(O_CREATE|O_EXCL)` — success → `Remove` wal/shm + `checkpointDB`; fail/fresh → keep existing recovered fallback.
2. **Save/Search defer**: after `INSERT/UPDATE` or rows close, `defer db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")`.
3. **Test**: `TestGhostWAL_Stale_Removed` (mtime>5min → removed, `Open` writable) and `TestGhostWAL_Fresh_Kept` (mtime<5min → preserved).

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/bigmem/bigmem.go` | Modified | `isGhostWAL`, `Open`/`ResolveDBPath`, `Save`/`Search` defer checkpoint |
| `internal/bigmem/full.go` | Modified | No `DoctorFix` change; verify no checkpoint conflict |
| `internal/bigmem/bigmem_test.go` | Modified | `TestGhostWAL_*` TempDir + Chtimes |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Reclaim live SHM | Medium | 5min + O_EXCL; skip if probe fails/fresh |
| Checkpoint overhead | Low | TRUNCATE cheap; defer only on success; no VACUUM |
| Windows lock race | Medium | Best-effort Remove; fallback to recovered path |
| mtime flake | Low | TempDir isolation, Chtimes + 10ms gap |

## Rollback Plan

`git revert <sha>` — additive guard only, no schema. Restore warning+fallback. Manual cleanup: `biggz bigmem doctor --fix` or `rm *.wal *.shm`.

## Dependencies

- `modernc.org/sqlite`, `os.Stat/Chtimes/OpenFile/Remove`, `go test ./... -count=1 -timeout 180s`

## Success Criteria

- [ ] Stale ghost (wal 0B, shm>0, mtime>5min, O_EXCL ok) removed + checkpoint before open
- [ ] Fresh ghost (mtime<5min or O_EXCL busy) not removed, recovered fallback intact
- [ ] `Save`/`Search` defer checkpoint; WAL bounded after burst
- [ ] `TestGhostWAL_*` pass (Win/Linux); `go test` green; no blobstore/branching diff
