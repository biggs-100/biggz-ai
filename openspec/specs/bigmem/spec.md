# BigMem Specification

## Purpose

Local SQLite-backed memory store (Engram-compatible). Covers persistence, FTS dedup, sync, and Engram import.

## Requirements

### Requirement: REQ-1 — Engram Import Dispatch (--from-engram)

The system MUST import from Engram storage when `--from-engram` is set; otherwise MUST use default BigMem transport (`~/.biggz/bigmem`). Flag MUST be exclusive source selector.

#### Scenario: Import from Engram

- GIVEN valid `.engram/manifest.json` with chunks
- WHEN `biggz bigmem sync import --from-engram` runs
- THEN system MUST read manifest + `chunks/*.jsonl.gz` into `bigmem.db`

#### Scenario: Default transport unchanged

- GIVEN no `--from-engram`
- WHEN `biggz bigmem sync import` runs
- THEN existing BigMem transport MUST be used

### Requirement: REQ-2 — Custom Engram Dir (--engram-dir)

System MUST accept `--engram-dir <PATH>` to override default `.engram/`; if omitted MUST resolve default.

#### Scenario: Custom dir

- GIVEN `--from-engram --engram-dir /tmp/.engram`
- WHEN import runs
- THEN manifest/chunks MUST be read from `/tmp/.engram`

#### Scenario: Default resolution

- GIVEN `--from-engram` without `--engram-dir`
- WHEN import runs
- THEN default `.engram` path MUST be resolved

### Requirement: REQ-3 — Project Filter (--project)

With `--project <NAME>`, system MUST import only entities where `project==NAME` after gunzip.

#### Scenario: Filtered import

- GIVEN chunks with `biggz-ai` and `other`
- WHEN `--project biggz-ai` runs
- THEN only `biggz-ai` entities MUST be inserted

#### Scenario: No filter imports all

- GIVEN no `--project`
- WHEN import runs
- THEN all projects MUST be imported

### Requirement: REQ-4 — sync_id to ID Mapping

System MUST map `engram.sync_id -> bigmem.ID`; int64 `id` MUST be ignored. Empty `sync_id` MUST generate deterministic `engram-<sha256(title+content)[0:12]>`.

#### Scenario: sync_id preserved

- GIVEN Engram obs `sync_id="obs-abc123"`, `id=42`
- WHEN imported
- THEN `bigmem.ID` MUST be `"obs-abc123"`

#### Scenario: Empty sync_id fallback

- GIVEN Engram obs with empty `sync_id`
- WHEN imported
- THEN ID MUST be `engram-<hex>` deterministic for same content

### Requirement: REQ-5 — Idempotent Dedup (sync_chunks)

Re-import MUST be no-op. System MUST record `sync_chunks(target_key='engram:'+chunkID)` and skip known chunks; inserts MUST use `ON CONFLICT DO NOTHING`.

#### Scenario: Re-import no-op

- GIVEN chunk `a3f8c1d2` already in `sync_chunks`
- WHEN re-imported
- THEN chunk MUST be skipped, no duplicates created

#### Scenario: Partial import

- GIVEN manifest 3 chunks, 2 already recorded
- WHEN import runs
- THEN only 1 pending chunk MUST be processed

### Requirement: REQ-6 — Error: Missing Manifest

Missing `manifest.json` MUST emit to stderr and exit 1 with zero DB mutations.

#### Scenario: Missing manifest

- GIVEN `--engram-dir /tmp/empty` without `manifest.json`
- WHEN import runs
- THEN stderr MUST mention `manifest.json` and exit 1

### Requirement: REQ-7 — Error: Corrupt Chunk

Corrupt gzip/JSON MUST warn per chunk to stderr, skip chunk, continue others. Exit 0 if any chunk succeeds.

#### Scenario: Corrupt gzip skipped

- GIVEN `chunks/bad.jsonl.gz` invalid gzip
- WHEN import runs
- THEN stderr MUST warn with chunk ID; other chunks MUST import

#### Scenario: Corrupt JSON skipped

- GIVEN gunzipped chunk has invalid JSON
- WHEN import runs
- THEN chunk MUST be skipped with warning, not counted

### Requirement: REQ-8 — Pi Isolation (Pi Untouched)

Feature MUST NOT modify `pi/` or TUI; `.engram/` MUST remain read-only; tracking MUST live only in `bigmem.db`.

#### Scenario: Pi unchanged

- GIVEN change applied
- WHEN `git diff -- pi/` checked
- THEN output MUST be empty

#### Scenario: Engram read-only

- GIVEN import succeeded
- WHEN `.engram/` inspected
- THEN no file MUST be modified; `sync_chunks` rows MUST exist in `bigmem.db`

### Requirement: REQ-B1 — Branching Schema

The system MUST add to `sessions`: `parent_id TEXT` (self-FK, nullable), `leaf_id TEXT`, `branch_summary TEXT`. Roots MUST have `parent_id IS NULL`.

#### Scenario: Fresh DB schema

- GIVEN fresh DB
- WHEN `Open()` completes
- THEN `PRAGMA table_info(sessions)` MUST include `parent_id`, `leaf_id`, `branch_summary`

#### Scenario: Root creation

- GIVEN `CreateBranch(parentID="")`
- WHEN inserted
- THEN `parent_id` MUST be `NULL` and `leaf_id` MUST equal `id`

### Requirement: REQ-B2 — Legacy Migration (DoctorFix)

`migrateSchema()` MUST `ADD COLUMN` idempotently (O(1)). Legacy rows MUST become `parent_id=NULL, leaf_id=self`. `Doctor()` MUST flag missing columns fixable; `DoctorFix()` MUST be idempotent.

#### Scenario: Legacy migration

- GIVEN DB with 2 pre-D2 sessions
- WHEN `DoctorFix()` runs
- THEN rows MUST have `leaf_id=id` and `parent_id IS NULL`

#### Scenario: Idempotent rerun

- GIVEN migrated DB
- WHEN `DoctorFix()` runs twice
- THEN second run MUST succeed with unchanged row count

### Requirement: REQ-B3 — Branch CRUD

The system MUST provide `CreateBranch(parentID, summary)`, `GetBranch(id)`, `ListBranches()`. `CreateBranch` with non-empty `parentID` MUST validate parent exists. `branch_summary` is optional text.

#### Scenario: Create child

- GIVEN root `A`
- WHEN `CreateBranch(parentID=A.id, summary="fix")`
- THEN child MUST have `parent_id=A.id` and `branch_summary="fix"`

#### Scenario: List/Get

- GIVEN chain `A->B->C`
- WHEN `ListBranches()` / `GetBranch(B.id)` called
- THEN list has 3 and `Get` returns `B` with correct parent

#### Scenario: Missing parent error

- GIVEN parent `missing` absent
- WHEN `CreateBranch(parentID="missing")`
- THEN MUST return error, 0 rows inserted

### Requirement: REQ-B4 — Leaf→Root Resolution

The system MUST provide `GetLeafPath(leafID)` and `SessionContextBranched(leafID)` walking `parent_id` iteratively leaf→root, depth limit 100, cycle guard, ordered leaf→root. `""` leafID MUST fallback to linear `SessionContext`.

#### Scenario: Chain resolution

- GIVEN `R->B->L`
- WHEN `GetLeafPath(L.id)` called
- THEN result MUST be `[L, B, R]`

#### Scenario: Cycle and depth guard

- GIVEN `A.parent_id=B, B.parent_id=A`
- WHEN `GetLeafPath(A.id)` called
- THEN traversal MUST terminate without loop (depth/cycle guard)

### Requirement: REQ-B5 — Save Anchoring & SetLeaf

`Save()` MAY accept optional `parentId`; when omitted MUST be no-op. `SetLeaf(leafID)` MUST atomically UPDATE `leaf_id` under `Store.mu`.

#### Scenario: Save with anchoring

- GIVEN active leaf `L`
- WHEN `Save(obs, parentId=L.id)` called
- THEN association MUST persist without breaking legacy dedup

#### Scenario: Save without parent unchanged

- GIVEN `Save(obs)` with no parentId
- WHEN executed
- THEN FTS/dedup behavior MUST match pre-D2

#### Scenario: SetLeaf atomic

- GIVEN concurrent `SetLeaf` calls
- WHEN both complete
- THEN final leaf MUST be one of the values (single UPDATE)

### Requirement: REQ-B6 — Backward Compatibility

`Get`/`Search` MUST work for legacy and branched rows. Existing linear tests MUST pass unchanged.

#### Scenario: Legacy Get/Search

- GIVEN legacy row `parent_id=NULL`
- WHEN `Get(id)` or `Search(q)` called
- THEN results MUST return normally, independent of branching columns

### Requirement: REQ-B7 — No Automatic GC

The system MUST NOT auto-delete branches; retention is indefinite until explicit delete.

#### Scenario: Retention

- GIVEN `R->B->C`
- WHEN no explicit delete issued
- THEN all three MUST remain queryable

### Requirement: REQ-B8 — Minimal MCP & Scope Bound

MCP MUST expose only `bigmem_branch_create/list/get` (internal-only) delegating to Go API. TUI ` /branch`/`/rewind`, `sdd-apply` auto-branch, `SessionEntryIndex` mirror, D1 blob, graph/FTS re-rank, sync branch awareness MUST NOT be implemented.

#### Scenario: MCP minimal

- GIVEN MCP server running
- WHEN tools listed
- THEN `bigmem_branch_create/list/get` MUST exist and create MUST call `CreateBranch`

#### Scenario: No TUI branching

- GIVEN change applied
- WHEN `grep -r "/branch\|/rewind" tui/ cmd/` runs
- THEN output MUST be empty

### Requirement: REQ-GW1 — Stale Ghost Detection (>5 min)

The system MUST classify `bigmem.db-wal`/`-shm` as stale ghost only when `wal size == 0` AND `shm size > 0` AND `time.Since(shm ModTime) > 5 min`. All three MUST hold. Fresh (<5 min) or other sizes MUST NOT be stale.

#### Scenario: Stale ghost detected
- GIVEN `bigmem.db-wal` 0 B and `bigmem.db-shm` >0 B with `ModTime` 6 min ago
- WHEN `isGhostWAL` / `Open` pre-check evaluates
- THEN result MUST be stale ghost

#### Scenario: Fresh ghost not stale
- GIVEN same sizes but `ModTime` 30 s ago
- WHEN evaluation runs
- THEN result MUST be NOT stale

#### Scenario: Non-ghost sizes not stale
- GIVEN `wal` >0 B or `shm` ==0 B (any mtime)
- WHEN evaluation runs
- THEN result MUST be NOT stale

### Requirement: REQ-GW2 — Stale Reclaim in Open (O_EXCL + Remove + TRUNCATE)

When REQ-GW1 is stale, `Open`/`ResolveDBPath` MUST probe liveness via `os.OpenFile(O_CREATE|O_EXCL)`; on success MUST `os.Remove` wal and shm, then execute `PRAGMA wal_checkpoint(TRUNCATE)` before `sql.Open`, and MUST open primary DB with no fallback to `bigmem_recovered`.

#### Scenario: Stale reclaimed, primary used
- GIVEN stale ghost and `O_EXCL` probe succeeds
- WHEN `Open` is called
- THEN wal/shm MUST be removed, `wal_checkpoint(TRUNCATE)` MUST run, and primary `bigmem.db` MUST be opened (no recovered warning)

#### Scenario: Checkpoint best-effort
- GIVEN stale ghost reclaim succeeds but checkpoint returns error
- WHEN `Open` continues
- THEN `Open` MUST still succeed and return primary (checkpoint MUST NOT fail open)

### Requirement: REQ-GW3 — Fresh/Busy Preservation and Race Fallback

If ghost is fresh (<5 min) OR `O_EXCL` probe fails (file exists / locked / race), the system MUST NOT remove wal/shm and MUST preserve existing recovered fallback behavior (`warning` + `bigmem_recovered` merge/promote).

#### Scenario: Fresh ghost preserved with fallback
- GIVEN `wal` 0 B / `shm` >0 B but `ModTime` <5 min
- WHEN `Open` is called
- THEN wal/shm MUST remain and `ResolveDBPath` MUST follow recovered fallback path if blocked

#### Scenario: Busy O_EXCL preserves fallback
- GIVEN stale sizes but `O_EXCL` fails (concurrent holder / Windows lock)
- WHEN `Open` is called
- THEN wal/shm MUST NOT be removed and fallback path MUST remain intact

#### Scenario: No stale → no removal side-effect
- GIVEN no ghost (e.g., `wal` >0)
- WHEN `Open` is called
- THEN no `Remove` on wal/shm MUST occur

### Requirement: REQ-GW4 — Deferred WAL Checkpoint in Save/Search

After successful `Save` or `Search` (rows closed), the system MUST defer `PRAGMA wal_checkpoint(TRUNCATE)` as best-effort, non-blocking cleanup to bound WAL after bursts. Failure MUST NOT propagate to caller. Checkpoint MUST NOT run on error paths.

#### Scenario: Save defers checkpoint
- GIVEN `Save` succeeds (insert/update)
- WHEN `Save` returns
- THEN a deferred `wal_checkpoint(TRUNCATE)` MUST have been attempted (WAL bounded)

#### Scenario: Search defers checkpoint after rows close
- GIVEN `Search` succeeds and rows are consumed/closed
- WHEN `Search` returns results
- THEN deferred `wal_checkpoint(TRUNCATE)` MUST have been attempted

#### Scenario: Checkpoint failure does not fail operation
- GIVEN `Save` succeeded but checkpoint returns `busy`/`locked`
- WHEN caller receives result
- THEN `Save` MUST still report success

#### Scenario: No checkpoint on Save error
- GIVEN `Save` fails (e.g., constraint error)
- WHEN error is returned
- THEN checkpoint MUST NOT be attempted

### Requirement: REQ-RO1 — Atomic Per-Write Session Adoption (resolveWriteProjectTx)

The system MUST provide `resolveWriteProjectTx(sessionID, requestedProject)` that atomically claims an orphan session when no foreign owner exists.

#### Scenario: Orphan adopted atomically in same TX
- GIVEN session `S` exists with `project IS NULL OR trim(project)=''`
- AND `foreignRecordOwnerTx(S)` finds zero rows with `project != requestedProject`
- WHEN `resolveWriteProjectTx(S, "projA")` runs inside the caller's `BEGIN IMMEDIATE` TX
- THEN `sessions.project` for `S` MUST become `projA` in same TX and the caller MUST succeed

#### Scenario: Already-owned session is no-op
- GIVEN session `S` has `project="projA"`
- WHEN `resolveWriteProjectTx(S, "projA")` is called
- THEN no UPDATE MUST occur and the call MUST succeed

### Requirement: REQ-RO2 — Ambiguous Ownership Rejection

The system MUST reject adoption when foreign observations exist and MUST return `ErrProjectOwnershipAmbiguous` with a rescue hint.

#### Scenario: Foreign project blocks adoption
- GIVEN session `S` has `project IS NULL`
- AND an observation exists with `session_id=S AND project="other" AND project != "projA"`
- WHEN `resolveWriteProjectTx(S, "projA")` is invoked
- THEN it MUST return `ErrProjectOwnershipAmbiguous` and NOT update `sessions.project`

#### Scenario: Error carries rescue hint
- GIVEN `ErrProjectOwnershipAmbiguous` is returned for session `S` and project `projA`
- WHEN the error is formatted
- THEN it MUST contain `biggz bigmem rescue-ownership --project projA --session S`

### Requirement: REQ-RO3 — Bulk Rescue with Plan (RescueNullProjectOwnership)

The system MUST provide `RescueNullProjectOwnership(project)` that bulk-adopts all orphan sessions via a two-phase Plan.

#### Scenario: Bulk adopts N orphans
- GIVEN N sessions exist with `project IS NULL OR trim(project)=''`
- AND none have foreign owners for project `projA`
- WHEN `RescueNullProjectOwnership("projA")` executes
- THEN N sessions MUST be updated to `projA`, each via per-session TX with `adoptSessionOwnershipTx`, and result MUST report `adopted=N`

#### Scenario: Plan dry-run matches apply
- GIVEN `RescueNullProjectOwnership.Plan("projA")` is called (or `--dry-run`)
- WHEN the subsequent `RescueNullProjectOwnership("projA")` is applied
- THEN Plan counts/IDs and `ambiguous` list MUST match the apply result; `unknown` project values MUST NOT be rescuable

### Requirement: REQ-RO4 — Save Integration

The system MUST integrate ownership resolution into `Save` before dedup, serialized under `Store.mu` and `BEGIN IMMEDIATE`.

#### Scenario: Save resolves before dedup in single TX
- GIVEN `Save` is called with `sessionID=S` and requested project `projA`
- WHEN `Save` executes
- THEN it MUST call `resolveWriteProjectTx` before FTS dedup, inside a single `BEGIN IMMEDIATE` TX holding `Store.mu`, and succeed after adoption

#### Scenario: Concurrent saves remain serialized
- GIVEN two concurrent `Save` calls for the same orphan session `S` with `projA`
- WHEN both enter `Store.mu` + `BEGIN IMMEDIATE`
- THEN exactly one adoption MUST win, the other MUST see already-owned and proceed without error

### Requirement: REQ-RO5 — CLI rescue-ownership

The system MUST provide `biggz bigmem rescue-ownership --project X [--session Y] [--dry-run] [--json]` for operator-initiated rescue.

#### Scenario: Bulk rescue via CLI
- GIVEN N orphan sessions exist
- WHEN `biggz bigmem rescue-ownership --project projA` runs
- THEN output MUST report adopted count N (or JSON with `adopted`, `skipped`, `ambiguous`)

#### Scenario: Scoped and dry-run modes
- GIVEN orphan session `S` exists
- WHEN `biggz bigmem rescue-ownership --project projA --session S --dry-run --json` runs
- THEN no DB mutation MUST occur, stdout MUST be valid JSON with matching Plan, and `--session S` MUST limit scope to `S` only

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

### Requirement: REQ-RR5 — Docs & Protocol Rank vs Recency

`docs/architecture.md` + `bigmem-protocol.md` MUST document table `ORDER BY rank` vs `ORDER BY updated_at DESC`, when to use each, examples `search --query "session"` vs `search --query ""`/`biggz recall`; help MUST warn.

#### Scenario: Table present

- GIVEN docs read
- WHEN searched
- THEN each MUST contain rank vs `updated_at DESC` table + examples

#### Scenario: Help warns

- GIVEN `biggz bigmem search --help` / `recent --help`
- WHEN rendered
- THEN it MUST note recency uses empty query `updated_at DESC`

#### Scenario: Ordering invariant

- GIVEN `bigmem.go` read
- WHEN checking 1801/1844
- THEN `1801` has `ORDER BY o.updated_at DESC`, `1844` has `ORDER BY rank`
