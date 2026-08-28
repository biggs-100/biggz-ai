# Tasks: bigmem-ghost-wal — Fix Ghost WAL/SHM

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 200–300 |
| 400-line budget risk | Low |
| 800-line budget risk | Low |
| Chained PRs recommended | No |
| Delivery strategy | auto-chain |
| Chain strategy | pending (single PR) |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Ghost WAL GW1-4: mtime+O_EXCL reclaim+deferred TRUNCATE | PR 1 (single) | `go test ./internal/bigmem -run TestGhostWAL -count=1 -timeout 60s` | `go test ./... -count=1 -timeout 180s` | `git revert <sha>` — no schema; `rm *.wal *.shm` or `doctor --fix` |

Deps: 1 RED → 2 isGhostWAL → 3 Open/Save/Search → 4 vet/harness. Strict TDD off.

## Phase 1: RED — Ghost & Threat Failing Tests

- [x] 1.1 RED GW1: add `internal/bigmem/bigmem_test.go` `TestIsGhostWAL` stale 0B/>0B/6min→true vs fresh 30s→false vs wal>0/shm0→false via `t.TempDir`+`Chtimes` — FAIL before D1
- [x] 1.2 RED GW2/GW3+WAL race: add `TestGhostWAL_Busy_OExcl_Preserved` stale sizes but `O_EXCL` fails → wal/shm not removed, fallback preserved — FAIL before D2
- [x] 1.3 RED GW4+busy checkpoint: add `TestGhostWAL_SaveSearch_Checkpoint` Save success→checkpoint, Save error→no checkpoint, busy→still success — FAIL before D4

## Phase 2: Foundation — isGhostWAL Hardening (GW1, D1)

- [x] 2.1 Modify `internal/bigmem/bigmem.go:isGhostWAL` require `wal==0 && shm>0 && Since(ModTime)>5min`; done when `go test -run TestIsGhostWAL` green Win/Linux
- [x] 2.2 Add `probeGhostLiveness` in `internal/bigmem/bigmem.go` via `os.OpenFile(O_CREATE|O_EXCL,0644)`; done when `Busy_OExcl` distinguishes live vs stale

## Phase 3: Core — Open Reclaim & Deferred Checkpoint (GW2-4, D2-D4)

- [x] 3.1 Modify `internal/bigmem/bigmem.go:ResolveDBPath/Open` pre-`sql.Open`: stale+probe ok→`Remove` wal/shm→`checkpointDB(TRUNCATE)` best-effort→primary (GW2); fresh/busy→preserve fallback (GW3); done when `Stale_Removed` gone+writable, `Fresh_Kept` preserved
- [x] 3.2 Modify `internal/bigmem/bigmem.go:Save` add `defer db.Exec(TRUNCATE)` after success only, swallow busy/locked, skip on error (GW4 Save); done when burst WAL bounded, error no checkpoint
- [x] 3.3 Modify `internal/bigmem/bigmem.go:Search` add deferred `TRUNCATE` after `rows.Close`, swallowed, skip on query error (GW4 Search); done when Search+close attempts checkpoint
- [x] 3.4 Verify `internal/bigmem/full.go` `DoctorFix` PASSIVE+TRUNCATE+VACUUM+FTS no conflict with defer; done when no change + `go vet ./internal/bigmem` clean

## Phase 4: Verification & Cleanup

- [x] 4.1 Run `go test ./internal/bigmem -run TestGhostWAL -count=1 -timeout 60s` + `TestIsGhostWAL` — GW1-4 green
- [x] 4.2 Run `go test ./... -count=1 -timeout 180s` + `go vet ./...` clean; validate no blobstore/branching diff
- [x] 4.3 Verify WAL bounded: 50×Save then `Stat -wal` size ≤ threshold, Search close triggers checkpoint
