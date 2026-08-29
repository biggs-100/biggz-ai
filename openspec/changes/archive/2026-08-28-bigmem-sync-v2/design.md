# Design: bigmem-sync-v2 — Ordered Sync Journal for BigMem

## Technical Approach

Port Engram project-scoped journal as local-only foundation. Add `sync_mutations(seq AUTOINCREMENT, project, entity, entity_key, op, payload, source, disposition)`, `sync_state(target_key, project, lifecycle, last_enqueued/acked/pulled, failures, backoff_until, lease_owner/until, reason_code)`, `sync_apply_deferred`, `sync_enrolled_projects` (+ optional `cloud_upgrade_state`) via `CREATE TABLE IF NOT EXISTS`, coexisting with `sync_chunks`. No DDL on `observations`, no backfill. Enqueue in caller's `BEGIN IMMEDIATE` TX under `Store.mu`. File sync (`sync.go`) untouched. Covers SYNC-J1/S1/D1/Q1/L1/C1/M1; maps to PR1-PR4 stacked-to-main.

## Architecture Decisions

| Decision | Option A | Option B | Tradeoff | Decision |
|----------|----------|----------|----------|----------|
| Journal ordering | Global `seq AUTOINCREMENT` | Per-project seq + vector clock | A simple ORDER BY, Engram parity; B merge complex | **A** — seq global, project filter |
| Deferred retry | 5→dead fixed | Unbounded | A bounded; B infinite | **A** — 5 deferred, 6th dead `hash(project+entity_key+payload)` |
| Quarantine | Deterministic validator | Heuristic | A advances cursor; B flaky | **A** — quarantined+evidence, degraded+reason_code |
| Lease TTL | 1m + owner check | 5m + heartbeat goroutine | A atomic WHERE, no goroutine; B out-of-scope | **A** — TTL 1m, deny other owner |
| Status CLI | Enrich `sync --status` | New `sync journal` | A reuses UX; B fragments | **A** — extend `cli_bigmem.go` |

## Data Flow

```
Save/Enroll ─► BEGIN IMMEDIATE (Store.mu) ─► enqueueSyncMutationTx → seq pending
                                                    ▼
                                          sync_state pending last_enqueued=seq
                                                    ▼
                                           ListPending ORDER BY seq
                                                    ▼
                                   ApplyPulledMutation ─► Ack → healthy
                                         │ └─ErrFKMissing→deferred ≤5 → ReplayDeferred →5→dead(hash)
                                         └─irreparable→Quarantine→quarantined+degraded cursor advances
```

Lifecycle `idle→pending→running→healthy/degraded`; `MarkSyncFailed` increments failures, sets `backoff_until`.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/bigmem/sync_journal.go` | Create | DDL + journal API: enqueue/ListPending/Ack/ApplyPulled/ReplayDeferred/Quarantine/Lease/MarkSync, deadLetter hash |
| `internal/bigmem/bigmem.go` | Modify | Call `enqueueSyncMutationTx` in `Save` TX after `resolveWriteProjectTx`; enroll project; migrateSchema |
| `internal/bigmem/sync.go` | Modify | Keep transport; extend SyncStatus to read sync_state/pending counts |
| `internal/bigmem/sync_test.go` | Create | Tests: ordered enqueue, lifecycle, deferred 5→dead, quarantine, lease |
| `cmd/biggz/cli_bigmem.go` | Modify | Extend `sync --status` to show lifecycle/pending/last_* /failures/backoff/lease |

## Interfaces / Contracts

```go
type SyncMutation struct { Seq int64; Project, Entity, EntityKey, Op, Source, Disposition string; Payload []byte; CreatedAt string }
type SyncState struct { TargetKey, Project, Lifecycle string; LastEnqueuedSeq, LastAckedSeq, LastPulledSeq int64; ConsecutiveFailures int; BackoffUntil, LeaseOwner, LeaseUntil, ReasonCode *string }

func enqueueSyncMutationTx(tx *sql.Tx, project, entity, entityKey, op string, payload []byte) (int64, error)
func ListPendingMutations(project string, limit int) ([]SyncMutation, error)
func AckSyncMutation(seq int64) error
func ApplyPulledMutation(m SyncMutation) error
func ReplayDeferredForScope(scope string) error
func QuarantineIrreparable(seq int64, evidence string) error
func AcquireSyncLease(targetKey, owner string, ttl time.Duration) (bool, error)
func ReleaseSyncLease(targetKey, owner string) error
func MarkSyncFailed(targetKey, project, reasonCode string) error

var ErrFKMissing = errors.New("fk missing")
func deadLetterID(project, entityKey string, payload []byte) string
```

`enqueueSyncMutationTx` uses `LastInsertId`→seq, upserts `sync_state` `pending`.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Enqueue ordered monotonic | Two Saves, seq2=seq1+1, ListPending ordered |
| Unit | Ack lifecycle | Enqueue→Ack check last_acked, healthy |
| Unit | Deferred 5→dead hash | 5× deferred, 6th dead `sha256` |
| Unit | Quarantine deterministic | Quarantine sets quarantined+degraded, next pending returned |
| Unit | Lease TTL/owner | A ok, B denied, B release fail, A clear, expiry ok |
| Integration | Save atomic enqueue | Save→check pending+state |
| Integration | Migration coexist | Seed sync_chunks, Open, both queryable no backfill |
| CLI | sync --status | Seed pending/backoff/lease, assert output |

## Threat Matrix

| Threat | Applicable | Reason | Expected Behavior | RED Test |
|--------|------------|--------|-------------------|----------|
| Log blocked | Applicable | One bad seq blocks | Quarantine advances cursor, degraded | Enqueue 10,11 quarantine 10→ListPending 11 |
| FK cycle | Applicable | FK miss defer loop | Cap 5, 6th dead hash | attempts=5→dead hash |
| Lease split-brain | Applicable | Concurrent acquire | Atomic WHERE deny other | A acquire, B denied, B release fail |
| Payload tamper | Applicable | Corrupt JSON | Validate→quarantine evidence | Corrupt→quarantined |
| Shell/subprocess | N/A | No exec/VCS | — | — |
| Routing/executable | N/A | No HTTP routing | — | — |

Applicable rows propagate to tasks/RED.

## Migration / Rollout

`migrateSchema` creates journals IF NOT EXISTS; no backfill; `.bigmem/` untouched; `sync_chunks` stays primary for file sync. New Save/Enroll enqueues. `cloud_upgrade_state` optional (PR4). Rollback `git revert` per PR; tables inert.

## Open Questions

- [ ] Dead-letter hash exact length/input — `sha256(project+entity_key+payload)` hex[:12] vs Engram `store.go.safe`.
- [ ] `sync_enrolled_projects` auto-enroll on Save vs explicit — do both `INSERT OR IGNORE`.
