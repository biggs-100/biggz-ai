# Apply Progress — bigmem-sync-v2 PR1+PR2+PR3

**Change**: bigmem-sync-v2
**Slice**: PR3 quarantine+lease (3.1-3.5) + PR2 deferred 5→dead + PR1 journal
**Mode**: Standard
**Progress**: 15/21 tasks

## Completed Tasks
- [x] 1.1 DDL 3 tables IF NOT EXISTS via ensureSyncJournalTables in Open
- [x] 1.2 RED TestEnqueueOrdered 2 enqueues seq2==seq1+1 ordered ListPending
- [x] 1.3 enqueueSyncMutationTx with LastInsertId upsert sync_state pending; called from Save TX via tryEnqueueObservationTx
- [x] 1.4 ListPendingMutations + AckSyncMutation -> last_acked healthy
- [x] 1.5 CLI sync --status lifecycle+pending
- [x] 2.1 RED TestDeferredFKMissing ApplyPulledMutation ErrFKMissing -> sync_apply_deferred attempts=1 SYNC-D1
- [x] 2.2 Add sync_apply_deferred DDL + ApplyPulledMutation defer/ack in sync_journal.go SYNC-D1
- [x] 2.3 Add ReplayDeferredForScope + deadLetterID sha256(project+entityKey+payload) 5->deferred 6th->dead; go test -run TestDeferred passes
- [x] 2.4 RED TestDeadLetterHash attempts=5 next -> dead hash==deadLetterID FK-cycle
- [x] 2.5 Verify SYNC-M1: seed sync_chunks, Open, enqueue both queryable no backfill; go test -run TestCoexist passes
- [x] 3.1 RED TestLogBlocked quarantine 10,11 -> ListPending 11 degraded (SYNC-Q1)
- [x] 3.2 RED TestLeaseSplitBrain A 1m ok B denied B Release fail A clear expiry C ok (SYNC-L1)
- [x] 3.3 QuarantineIrreparable(seq,evJSON) quarantined+evidence degraded+reason_code cursor advances
- [x] 3.4 AcquireSyncLease/ReleaseSyncLease WHERE lease_until<=now OR owner=owner + MarkSyncFailed exponential backoff 2^failures*2s
- [x] 3.5 RED TestPayloadTamper corrupt JSON -> quarantined payload_tamper deterministic (SYNC-Q1)

## Files Changed
- internal/bigmem/sync_journal.go — Modified: added evidence column migration, ValidateSyncMutation deterministic validator, IsIrreparablePayload, QuarantineIrreparable, AcquireSyncLease/ReleaseSyncLease with TTL, MarkSyncFailed/MarkSyncSucceeded exponential backoff
- internal/bigmem/sync_quarantine_lease_test.go — Created: 6 tests TestQuarantineLogBlocked, TestQuarantinePayloadTamper, TestLeaseSplitBrain, TestLeaseBackoff (+TTL concurrent merged), TestQuarantine, TestLease, TestLogBlocked, TestPayloadTamper deterministic
- internal/bigmem/sync_deferred_test.go — Unchanged (PR2)
- internal/bigmem/sync_journal_test.go — Unchanged (PR1)
- openspec/changes/bigmem-sync-v2/tasks.md — Updated: mark PR3 tasks [x] 15/21
- openspec/changes/bigmem-sync-v2/apply-progress.md — Updated: merged PR1+PR2+PR3 15/21

## Work Unit Evidence PR3
| Evidence | Value |
|---|---|
| Focused test | `go test ./internal/bigmem -run TestQuarantine\|TestLease -count=1` — PASS 6 tests (TestQuarantineLogBlocked, TestQuarantinePayloadTamper, TestLeaseSplitBrain, TestLeaseBackoff (+concurrent), TestQuarantine, TestLease) 1.0s |
| Full suite | `go test ./internal/bigmem -count=1 -timeout 180s` — PASS 5.7s |
| go vet | `go vet ./...` — PASS 0 warnings |
| go vet bigmem | `go vet ./internal/bigmem` — PASS |
| Runtime harness | Lease A/B concurrent + TTL expiry + backoff_until future & exponential 2^failures*2s, quarantine evidence JSON + degraded + cursor |
| Rollback | Revert sync_journal.go quarantine/lease/backoff block + sync_quarantine_lease_test.go |

## Work Unit Evidence PR2
| Evidence | Value |
|---|---|
| Focused test | `go test ./internal/bigmem -run TestDeferred -count=1` — PASS 4 tests |
| Full suite | `go test ./internal/bigmem -count=1 -timeout 180s` — PASS |
| go vet | `go vet ./...` — PASS |

## Work Unit Evidence PR1 (merged)
| Evidence | Value |
|---|---|
| Focused test PR1 | `go test ./internal/bigmem -run TestSync -count=1` — PASS 4 tests |
| Full suite PR1 | `go test ./internal/bigmem -count=1 -timeout 180s` — PASS |

## Next Recommended
PR4 verify (cloud_upgrade_state+gates) + `biggz bigmem sync --status` enrich

## Notes
PR3 slice 380 lines (216 sync_journal + 165 test -1 header = 380) <400 stacked-to-main base a39e956. Evidence column added via ALTER if missing, deterministic validator ValidateSyncMutation/IsIrreparablePayload no time/random. Lease uses WHERE lease_until<=now OR lease_owner=owner for split-brain deny. Backoff 2^failures*2s capped 1h, MarkSyncSucceeded resets.
