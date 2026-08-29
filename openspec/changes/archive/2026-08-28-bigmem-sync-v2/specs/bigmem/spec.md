# Delta for bigmem

## ADDED Requirements

### Requirement: SYNC-J1 — Ordered Sync Mutations Journal

The system MUST provide `sync_mutations(seq AUTOINCREMENT, project, entity, entity_key, op, payload, source, disposition)`. `enqueueSyncMutationTx` MUST insert in caller TX with `disposition='pending'` and return `seq`.

#### Scenario: Enqueue ordered pending

- GIVEN observation saved for project `p`
- WHEN `enqueueSyncMutationTx` called in same TX
- THEN row MUST have `seq>0`, `pending`, and `ListPending` MUST return ordered by `seq`

#### Scenario: Sequential monotonic

- GIVEN two enqueues committed in order
- WHEN queried
- THEN second `seq` MUST be first +1

### Requirement: SYNC-S1 — Sync State Lifecycle

The system MUST maintain `sync_state` per `target_key`+`project` with `lifecycle idle|pending|running|healthy|degraded`, `last_enqueued/acked/pulled_seq`, `consecutive_failures`, `backoff_until`, `lease_owner/until`, `reason_code`.

#### Scenario: Lifecycle and seq advance

- GIVEN state `idle` for `t`+`p`
- WHEN enqueue then `Ack` advances
- THEN lifecycle MUST progress and seq fields MUST update

#### Scenario: Failure backoff

- GIVEN sync failure
- WHEN `MarkSyncFailed` called
- THEN `consecutive_failures` MUST increment and `backoff_until` MUST be future

### Requirement: SYNC-D1 — Deferred Retry 5→Dead

The system MUST provide `sync_apply_deferred` with `attempts`. `ApplyPulledMutation` on `ErrFKMissing` MUST defer; `ReplayDeferredForScope` MUST retry 5 times as `deferred`, 6th MUST become `dead` with `pulledSessionDeadLetterSyncID=hash(project+entity_key+payload)`.

#### Scenario: Up to 5 deferred

- GIVEN deferred `attempts=4` with missing FK
- WHEN apply fails again
- THEN row MUST stay `deferred` with `attempts=5` and `relationApplyFailureSyncID`

#### Scenario: 6th dead-letter hash

- GIVEN `attempts=5` deferred
- WHEN next apply fails
- THEN disposition MUST be `dead` with deterministic hash

### Requirement: SYNC-Q1 — Quarantine Irreparable

The system MUST provide `QuarantineIrreparable(seq, evidenceJSON)` for deterministic irreparable mutations. It MUST set `quarantined`, store `evidence` JSON, set `sync_state degraded`+`reason_code`, and advance cursor.

#### Scenario: Quarantine with evidence

- GIVEN `seq=10` irreparable by validator
- WHEN `QuarantineIrreparable(10, evidence)` called
- THEN `seq=10` MUST be `quarantined` with evidence and cursor MUST advance

#### Scenario: Log not blocked

- GIVEN `seq=11` pending after `10` quarantined
- WHEN `ListPending` called
- THEN `11` MUST be returned and state MUST be `degraded`

### Requirement: SYNC-L1 — Lease Acquire/Release

The system MUST provide `AcquireSyncLease(target, owner, ttl)` and `ReleaseSyncLease`. Acquire MUST set `lease_owner/until=now+ttl` only if `lease_until<=now` or same owner; other owner MUST fail. Release MUST succeed only for owner.

#### Scenario: Concurrent acquire denied

- GIVEN no active lease
- WHEN `A` acquires 1m then `B` acquires
- THEN `A` MUST succeed with `lease_until>now`, `B` MUST fail

#### Scenario: Owner release and expiry

- GIVEN lease held by `A`
- WHEN `B` releases then `A` releases then `C` acquires post-expiry
- THEN `B` MUST fail, `A` MUST clear, `C` MUST succeed

### Requirement: SYNC-C1 — Sync Status CLI

The system MUST provide `biggz bigmem sync --status` showing per-target `lifecycle`, `pending` count, `last_enqueued/acked/pulled`, `consecutive_failures`, `backoff_until`, `lease_owner/until`.

#### Scenario: Shows lifecycle pending backoff

- GIVEN 3 pending, `lifecycle=pending`, `backoff_until` future
- WHEN `biggz bigmem sync --status` runs
- THEN output MUST show `pending: 3`, lifecycle, and backoff

#### Scenario: Idle empty

- GIVEN no mutations, `idle`
- WHEN status runs
- THEN output MUST show `idle` and `pending: 0`

### Requirement: SYNC-M1 — Migration Coexistence

The system MUST keep `sync_chunks` and add journals via `CREATE TABLE IF NOT EXISTS` without backfill or DDL on `observations`. New `Save`/`Enroll` MUST enqueue to `sync_mutations`.

#### Scenario: Coexist

- GIVEN existing `sync_chunks` rows
- WHEN new enqueue occurs
- THEN both tables MUST have data and `sync_chunks` MUST remain queryable

#### Scenario: No backfill

- GIVEN 100 observations, 0 journal rows
- WHEN migration runs
- THEN journal MUST stay empty, no backfill occurs
