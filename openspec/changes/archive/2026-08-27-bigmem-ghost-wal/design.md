# Design: bigmem-ghost-wal

## Technical Approach

Harden ghost WAL/SHM reclamation for Windows Pi kill / dual-holder. `Open` pre-check: `isGhostWAL` requires `wal==0 && shm>0 && Since(ModTime)>5min`; if stale, `O_EXCL` probe → `Remove` wal/shm → `wal_checkpoint(TRUNCATE)` before `sql.Open`. `Save`/`Search` defer `TRUNCATE` best-effort on success. Fresh/busy preserves `bigmem_recovered` fallback. Implements REQ-GW1..GW4; no D1/D2/DoctorFix changes.

## Architecture Decisions

### D1: Stale threshold

| Option | Tradeoff | Decision |
|--------|----------|----------|
| Immediate `wal==0&&shm>0` | Reclaims live SHM → corruption | Rejected |
| `mtime>5min` on SHM | 5 min false-negative but safe | **Chosen** |
| `mtime>10min` | Safer, longer leak window | Rejected |

Requires all three conditions (REQ-GW1). Immediate reclaim caused divergence.

### D2: Liveness probe

| Option | Tradeoff | Decision |
|--------|----------|----------|
| `flock`/lock file | Extra dep, Windows semantics differ | Rejected |
| `O_EXCL` via `OpenFile(CREATE\|EXCL)` | Atomic, no dep, fail=live holder | **Chosen** |
| No probe | Races with concurrent Open | Rejected |

REQ-GW2 mandates probe; success → Remove, fail → preserve fallback (REQ-GW3).

### D3: Checkpoint mode

| Option | Tradeoff | Decision |
|--------|----------|----------|
| `VACUUM` | Heavy, blocks | Rejected (DoctorFix only) |
| `PASSIVE` | May leave WAL | Rejected |
| `TRUNCATE` | Flush+truncate, cheap, bounds WAL | **Chosen** |

Reuses `checkpointDB(TRUNCATE)`; failures best-effort.

### D4: Checkpoint timing

| Option | Tradeoff | Decision |
|--------|----------|----------|
| Immediate | Delays caller on busy | Rejected |
| `defer` after success | Non-blocking, no error propagation | **Chosen** |

REQ-GW4: `Save` after INSERT/UPDATE, `Search` after `rows.Close`; skipped on error.

## Data Flow

```
Open: Stat wal/shm → isGhostWAL? ──stale+O_EXCL ok──→ Remove → checkpoint(TRUNCATE) → sql.Open primary
                        │ fresh/busy ──→ preserve → warning + recovered merge

Save/Search: s.mu.Lock → op → success? ──yes→ defer checkpoint(TRUNCATE) swallowed
                                         └─no→ no checkpoint
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/bigmem/bigmem.go` | Modify | `isGhostWAL` add `mtime>5min`; `ResolveDBPath`/`Open` add O_EXCL+Remove+TRUNCATE pre-check; `Save`/`Search` defer TRUNCATE |
| `internal/bigmem/full.go` | Verify | No change; ensure `DoctorFix` (PASSIVE+TRUNCATE+VACUUM+FTS rebuild) no conflict |
| `internal/bigmem/bigmem_test.go` | Modify | `TestGhostWAL_Stale_Removed` / `Fresh_Kept` via `t.TempDir` + `Chtimes` (6min vs 30s) + busy simulation |

## Interfaces / Contracts

```go
func isGhostWAL(dbPath string) bool // wal==0 && shm>0 && Since(ModTime)>5min
func checkpointDB(dbPath string)    // PRAGMA wal_checkpoint(TRUNCATE) best-effort

// Open pre-check (before sql.Open):
// if isGhostWAL(primary) && probeOExcl()==nil { Remove(wal,shm); checkpointDB(primary) }

// Save/Search post-success (deferred, swallowed):
// defer func(){ _, _ = s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)") }()
```

No new exported API.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|--------------|----------|
| Unit | `isGhostWAL` stale/fresh/size (REQ-GW1) | `TempDir` + `Chtimes` + `Stat` |
| Integration | `Open` stale→primary, fresh/busy→fallback (REQ-GW2/3) | Synthetic ghost, assert wal/shm + writable DB |
| Integration | `Save`/`Search` defer bounds WAL, swallow busy, skip on error (REQ-GW4) | Burst writes, `rows.Close` check, locked DB |
| E2E | `go test ./... -count=1 -timeout 180s` green | Full harness, no blobstore diff |

## Threat Matrix

Ref matrix (`references/threat-matrix.md`) — no shell/routing boundary:

| Boundary | Applicability | Reason |
|----------|---------------|--------|
| Documentation-like paths | N/A | No exec-doc classification |
| Git repository selection | N/A | No `git -C`/cwd change |
| Commit state | N/A | No git commit |
| Push state | N/A | No git push |
| PR commands | N/A | No PR automation |

Domain risks:

| Risk | Safe | Failure | RED test |
|------|------|---------|----------|
| WAL race (concurrent Open) | O_EXCL fail → fallback | Remove live SHM | `TestGhostWAL_Busy_OExcl_Preserved` |
| Windows Remove lock | Best-effort, fallback | Open fails | Existing fallback test |
| mtime flake | TempDir+Chtimes+gap | Flaky | `Fresh_Kept` (30s) |
| busy/locked checkpoint | Swallow, still success | Fail caller | Busy DB still succeeds |

## Migration / Rollout

No migration. Additive guard; revert `git revert <sha>`. Manual: `biggz bigmem doctor --fix` or `rm *.wal *.shm`.

## Open Questions

- [ ] Probe file: SHM probe vs `*.probe` — prefer SHM per spec.
- [ ] Search defer: inside `Search` after `rows` vs caller — inside with `defer` on Close.
