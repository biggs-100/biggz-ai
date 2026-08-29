# Design: bigmem-rescue-ownership — Port Engram Rescue-Ownership to BigMem

## Technical Approach

Port Engram atomic orphan adoption to BigMem (`sessions.project`/`observations.project`). `Save` holds `Store.mu` + `BEGIN IMMEDIATE` → `resolveWriteProjectTx` before FTS dedup (atomic claim+insert). `foreignRecordOwnerTx` rejects ambiguous; `adoptSessionOwnershipTx` does `UPDATE sessions` + `sync_mutations` probe via `sqlite_master`. Bulk rescue is 2-phase `Plan` → per-session adopts. Covers REQ-RO1..RO5.

## Architecture Decisions

| Decision | Option A | Option B (rejected) | Rationale |
|----------|----------|---------------------|-----------|
| Adoption TX | `Store.mu` + `BEGIN IMMEDIATE` in `Save` caller | `INSERT OR REPLACE` without lock | A serializes concurrent `Save` on same orphan (one wins, other sees owned); matches existing `Save`/`GW4` pattern; B risks `SQLITE_BUSY` under WAL |
| Ambiguous check | `foreignRecordOwnerTx`: count `observations WHERE session_id=? AND project!='' AND project!=?` | Check `sessions` only | A fails loud before UPDATE, prevents cross-project split; parity with Engram |
| Bulk plan | 2-phase `Plan` then apply | Single-pass direct UPDATE | A gives dry-run `adoptable/ambiguous` identical to apply; required for `--dry-run --json` |
| Sync enqueue | Probe `sqlite_master` for `sync_mutations` before `INSERT` | Always `INSERT` ignoring error | A no-op when table absent, enqueue when present; no DDL migration |
| CLI placement | `cmd/biggz/cli_bigmem.go` under `bigmemRun()` | New `cli_rescue.go` | A colocates with `projects/sync/doctor`, shares `Store.Open("")` |

## Data Flow

```
Save{SessionID=S, Project=P}
  └─ Store.mu + BEGIN IMMEDIATE
     └─ resolveWriteProjectTx(tx,S,P)
          ├─ SELECT project FROM sessions WHERE id=S
          ├─ P==existing → no-op
          ├─ NULL/'' && !foreignRecordOwnerTx → adoptSessionOwnershipTx
          │     ├─ UPDATE sessions SET project=P WHERE id=S
          │     └─ if sync_mutations exists → INSERT sync_mutations
          └─ foreignOwner → ErrProjectOwnershipAmbiguous(hint)
     └─ FTS dedup → INSERT observation → COMMIT + wal_checkpoint(TRUNCATE)

RescueNullProjectOwnership(P):
  Plan(P) [read TX] → {adoptable[], ambiguous[], total}
  Apply  → per-session BEGIN IMMEDIATE + adoptSessionOwnershipTx → Result{adopted,skipped,ambiguous}
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/bigmem/bigmem.go` | Modify | Add `ErrProjectRequired`, `ErrProjectOwnershipAmbiguous`, `foreignRecordOwnerTx`, `resolveWriteProjectTx`, `adoptSessionOwnershipTx`, `RescuePlan/Result` + `PlanRescue`/`RescueNullProjectOwnership`; wire `resolveWriteProjectTx` into `Save` under `mu`+`BEGIN IMMEDIATE`; `sqlite_master` probe for `sync_mutations` |
| `cmd/biggz/cli_bigmem.go` | Modify | Add `rescue-ownership` under `bigmemRun()`: `--project` required (`NormalizeProjectName`), `--session`, `--dry-run`, `--json`; JSON `{adopted,skipped,ambiguous}` |
| `internal/bigmem/rescue_test.go` | Create | Adoption, ambiguous fail, bulk dry-run vs apply, `unknown` excluded, Save integration, concurrent Save; may extend `bigmem_test.go` |
| `internal/project/detect.go` | Ref | No change — 5-case detection reused |

## Interfaces / Contracts

```go
var ErrProjectRequired = errors.New("project required")
var ErrProjectOwnershipAmbiguous = errors.New("project ownership ambiguous")
func (s *Store) resolveWriteProjectTx(tx *sql.Tx, sessionID, requestedProject string) error
func (s *Store) adoptSessionOwnershipTx(tx *sql.Tx, sessionID, project string) error
func foreignRecordOwnerTx(tx *sql.Tx, sessionID, requestedProject string) (bool, string, error)
type RescuePlan struct { Project string; Total int; Adoptable []string; Ambiguous []AmbiguousEntry }
type AmbiguousEntry struct { SessionID, ForeignProject string }
type RescueResult struct { Adopted, Skipped int; Ambiguous []AmbiguousEntry }
type RescueOptions struct { DryRun bool; SessionID string }
func (s *Store) PlanRescue(project string) (*RescuePlan, error)
func (s *Store) RescueNullProjectOwnership(project string, opts RescueOptions) (*RescueResult, error)
// CLI: biggz bigmem rescue-ownership --project X [--session Y] [--dry-run] [--json]
```
Only `WHERE project IS NULL OR trim(project)=''`, `"unknown"` excluded. `resolveWriteProjectTx` must run inside caller's `BEGIN IMMEDIATE` tx.

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | Orphan adopted / no-op / foreign blocks with hint | `t.TempDir()` store; NULL/'' `sessions`; assert `project` after Save |
| Unit | Bulk Plan == apply; `unknown` excluded | N orphans + 1 foreign; `Plan().Adoptable` vs `Adopted` |
| Unit | Concurrent Save serialized | `WaitGroup` 2× Save same orphan `projA`; final `projA` |
| Integration | CLI flags: required, scoping, dry-run, json | `bigmemRun` override or `exec`; JSON + DB unchanged |
| Vet | `go vet` + `-race` | `go test ./internal/bigmem -count=1 -timeout 180s` |

## Threat Matrix

| Threat | Applicable | Safe / Failure | RED test |
|--------|------------|----------------|----------|
| Race concurrent Save on same orphan | Y — `Store.mu`+`BEGIN IMMEDIATE` | Safe: one UPDATE wins; Fail: `SQLITE_BUSY` → `busy_timeout=5000` retry | concurrent Saves |
| Ambiguous silent adoption | Y — `foreignRecordOwnerTx` | Safe: `ErrProjectOwnershipAmbiguous` + hint `biggz bigmem rescue-ownership --project X --session Y`; Fail: no mutation | foreign obs blocks |
| Sync divergence (not enqueued) | Y — probe `sqlite_master` | Safe: enqueue if exists else no-op | with/without `sync_mutations` |
| Routing / shell / VCS / executable | N/A — `?` params only; `NormalizeProjectName` | — | — |

## Migration / Rollout

No DDL. Only `UPDATE sessions SET project=? WHERE id=? AND (project IS NULL OR trim(project)='')`. Idempotent, additive. Rollback `git revert <sha>` or restore `~/.biggz/bigmem/backup-*.db`; orphans stay NULL. No feature flag.

## Open Questions

- [ ] Reconcile `observations` with NULL project linked to rescued sessions? Scope is `sessions` only per proposal.
- [ ] Confirm `sync_mutations` column names (`entity`/`entity_key`/`op`/`project`) before enqueue — probe tolerates absence.
