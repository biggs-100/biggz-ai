# Tasks: bigmem-sync-v2 — Ordered Sync Journal for BigMem

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 1200–1500 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1→PR2→PR3→PR4 (stacked-to-main) |
| Delivery strategy | ask-on-risk |
| Chain strategy | stacked-to-main |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | DDL+enqueue/Ack | PR1 | `go test -run TestSyncJournal` | `biggz bigmem sync --status` | `sync_journal.go` DDL |
| 2 | Deferred 5→dead | PR2 | `go test -run TestDeferred` | `ReplayDeferredForScope` | `sync_apply_deferred` |
| 3 | Quarantine+lease | PR3 | `go test -run TestQuarantine` | lease A/B | quarantine+lease cols |
| 4 | cloud_upgrade_state+gates | PR4 | `go test ./internal/bigmem -timeout 180s` | `biggz bigmem sync --status` | `cloud_upgrade_state` |

## Phase 1: PR1 — Journal DDL + Enqueue/Ack

- [x] 1.1 Create `internal/bigmem/sync_journal.go` DDL 3 tables `IF NOT EXISTS`; wire `migrateSchema` in `bigmem.go`
- [x] 1.2 RED `sync_journal_test.go:TestEnqueueOrdered` 2 enqueues `seq2==seq1+1` ordered `ListPending` SYNC-J1 fail
- [x] 1.3 Add `enqueueSyncMutationTx(tx,project,entity,key,op,payload)` `LastInsertId` upsert `sync_state pending`; call from `Save` TX
- [x] 1.4 Add `ListPendingMutations` + `AckSyncMutation` → `last_acked_seq` `healthy`; `go test -run TestEnqueue` passes SYNC-J1/S1
- [x] 1.5 Extend `cmd/biggz/cli_bigmem.go` `sync --status` `lifecycle`+`pending:N`; `go vet ./...` passes SYNC-C1

## Phase 2: PR2 — Deferred 5→Dead

- [x] 2.1 RED `TestDeferredFKMissing` `ApplyPulledMutation` `ErrFKMissing` → `sync_apply_deferred attempts=1` SYNC-D1 fail
- [x] 2.2 Add `sync_apply_deferred` DDL + `ApplyPulledMutation` defer/ack in `sync_journal.go` SYNC-D1
- [x] 2.3 Add `ReplayDeferredForScope` + `deadLetterID` sha256(project+entityKey+payload) 5→deferred 6th→dead; `go test -run TestDeferred` passes
- [x] 2.4 RED `TestDeadLetterHash` `attempts=5` next → `dead` hash==`deadLetterID` FK-cycle fail→pass
- [x] 2.5 Verify SYNC-M1: seed `sync_chunks`, `Open`, enqueue both queryable no backfill; `go test -run TestCoexist` passes

## Phase 3: PR3 — Quarantine + Lease + Backoff

- [x] 3.1 RED `TestLogBlocked` enqueue 10,11 `QuarantineIrreparable(10,ev)` → `ListPending` 11 `degraded` log-blocked fail
- [x] 3.2 RED `TestLeaseSplitBrain` `A` Acquire 1m ok `B` denied `B` Release fail `A` clear expiry `C` ok split-brain fail
- [x] 3.3 Add `QuarantineIrreparable(seq,evJSON)` `quarantined`+evidence `degraded`+`reason_code` advance cursor SYNC-Q1
- [x] 3.4 Add `AcquireSyncLease`/`ReleaseSyncLease` `WHERE lease_until<=now OR owner=owner` + `MarkSyncFailed` backoff SYNC-L1
- [x] 3.5 RED `TestPayloadTamper` corrupt JSON → `quarantined` evidence payload-tamper; `go test -run TestQuarantine` passes

## Phase 4: PR4 — Cloud Upgrade State + CLI

- [x] 4.1 If needed add `cloud_upgrade_state` DDL else omit+doc skip; `go vet ./...` — omitted as optional per proposal/design; journal additive via IF NOT EXISTS, no DDL needed; vet clean
- [x] 4.2 Enrich `cmd/biggz/cli_bigmem.go` `sync --status` `last_enqueued/acked/pulled` `consecutive_failures` `backoff_until` `lease_owner/until` SYNC-C1; `biggz bigmem sync --status` — enriched to show LastEnqueued/LastAcked/LastPulled, consecutive_failures, backoff_until, lease_owner/until

## Phase 5: Verification & Gates

- [x] 5.1 `go vet ./...` zero warnings — vet clean (see verify evidence)
- [x] 5.2 `go test ./internal/bigmem -count=1 -timeout 180s` all green — 6.3s pass (see verify)
- [x] 5.3 `biggz bigmem sync --status` shows `lifecycle` `pending:N` `last_*` `backoff` `lease` — CLI now shows all fields
- [x] 5.4 `git revert` per PR leaves `.bigmem/` + `sync_chunks` intact; no DDL on `observations` — verified: DDL IF NOT EXISTS, no observation DDL, .bigmem untouched, sync_chunks coexistence test passes
