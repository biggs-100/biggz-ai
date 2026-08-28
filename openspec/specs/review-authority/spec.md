# Review Authority Specification

## Purpose

The Review Authority domain defines the content-addressed event store, chain validation, receipt binding (including fix-delta), flock-based file locking, burn semantics, correction-budget counters, role-based FSM transition guards (13-state), and lineage inventory/status. Every review transition is recorded as an immutable SHA-256-named event file under the git common directory; evidence integrity uses `domainHash` + `writeLengthPrefixed` and rejects legacy pipe-concat.

## Requirements

### Requirement: Content-Addressed Event Store

The system MUST persist every review transition as an immutable file resolved via `GitCommonDir` — `git rev-parse --git-common-dir` with fallback to `git rev-parse --git-dir` — at `<commonDir>/biggz/review-transactions/<lineage>/v1/events/<sha256>`. The file name MUST equal the lower-hex SHA-256 of its canonical JSON content (`sha256Hex(data)`). The store MUST use `publishImmutable` (atomically write `.tmp` then rename; same content is idempotent, different content is an error) and MUST maintain a `HEAD` file with the latest revision. Dual-read MUST remain: `EventPath`/`readRecord`/`Validate` MUST fallback to legacy flat `<commonDir>/biggz/review-transactions/<lineage>/<sha256>` when `v1/events/<sha256>` is absent. Each event (`Record`) MUST carry `schema = "biggz-ai.review-record/v1"`, `prevRevision` (empty for genesis), `operation`, `role`, `actor`, `timestamp` (RFC3339Nano), and `payload`.

Canonical layout:

```
<commonDir>/biggz/review-transactions/<lineage>/
  HEAD              — hex of latest event
  .lock             — flock file (see File Lock)
  v1/events/<sha256> — canonical event files
  v1/events/<sha256>.tmp — transient during publish
  receipts/<sha256>.json — persisted receipts (see Receipt Binding)
  burned.json       — tombstone when BurnEnabled (see Burn Semantics)
```

Legacy flat files at `<lineage>/<sha256>` remain readable; new writes go only to `v1/events/`.

#### Scenario: Happy path — append three events to canonical events dir

- GIVEN an empty lineage resolved via `GitCommonDir`
- WHEN three events are appended sequentially via `Store.Append`
- THEN each file MUST be at `v1/events/<sha256>` where `<sha256> = sha256Hex(canonical JSON)`
- AND each event after genesis MUST have `prevRevision == previous file name`
- AND `HEAD` MUST equal the last file name

#### Scenario: Dual-read legacy fallback

- GIVEN an existing legacy flat file at `<lineage>/<sha256>` and no `v1/events/<sha256>`
- WHEN `EventPath(<sha256>)` or `LoadChain()` reads it
- THEN it MUST return the legacy path and successfully load the record (migration compatibility)

#### Scenario: Publish immutable idempotence

- GIVEN `v1/events/<sha256>` already exists with content `C`
- WHEN `publishImmutable` is called again with identical bytes `C`
- THEN it MUST succeed idempotently; with different bytes it MUST return an error (hash collision)

#### Scenario: Empty lineage

- GIVEN no events (no `HEAD`)
- WHEN the system opens the lineage via `LoadChain()`
- THEN it MUST return `Valid=true`, `Count=0`, and empty `HeadHash`/`GenesisHash`

### Requirement: Chain Validation

The system MUST provide `Store.Validate()` and evidence-chain hashing in `model/hash.go` that recompute integrity from the content-addressed store and reject legacy pipe `|` concatenation.

- File integrity: for every 64-hex event file (canonical or legacy fallback), `sha256Hex(fileBytes) MUST equal file name`; otherwise verdict `Valid=false` with `hash mismatch`.
- Link integrity: every `Record.PrevRevision` that is non-empty MUST reference an existing event file; `HEAD` MUST point to an existing event; cycles MUST be detected.
- Evidence chain: `Evidence.Hash` MUST equal `domainHash("biggz-ai.review-evidence/v1\x00" + writeLengthPrefixed(Position, Timestamp, Kind, Payload, PrevHash))` where `writeLengthPrefixed` encodes each field as `u32 BE length || bytes` and `domainHash` is `sha256(domain + "\x00" + payload)` prefixed with `sha256:`. `MerkleRoot` MUST equal `domainHash("biggz-ai.review-merkle/v1\x00" + writeLengthPrefixed(lastHash))` (empty chain returns `""`). Pipe-concatenated `Position|Timestamp|Kind|Payload|PrevHash` or `SHA256(lastHash)` MUST be rejected.
- Store chain identity binding (for `ValidatedChain` receipt linkage) MUST use `domainHash("biggz-ai.review-store-chain/v1\x00" + writeLengthPrefixed(fields...))` when deriving chain identities.

#### Scenario: Valid chain

- GIVEN a lineage with three events and correct `PrevRevision` links under `v1/events/`
- WHEN `Validate()` is called
- THEN the verdict MUST be `Valid=true` with reason `"chain integrity preserved"`

#### Scenario: Tampered file

- GIVEN a lineage where one event file's bytes were modified
- WHEN `Validate()` is called
- THEN the verdict MUST be `Valid=false` with reason containing `hash mismatch`

#### Scenario: Domain vectors match gentle — pipe rejected

- GIVEN gentle v2.5 vectors with `writeLengthPrefixed` + `\x00` domain separation
- WHEN `evidenceHash`/`MerkleRoot` are computed
- THEN they MUST match those vectors and MUST differ from legacy `Position|...|PrevHash` and from `SHA256(lastHash)` hex

#### Scenario: Broken link

- GIVEN an event whose `PrevRevision` names a non-existent SHA
- WHEN `Validate()` is called
- THEN the verdict MUST be `Valid=false` with reason containing `broken link`

### Requirement: Receipt Binding

The system MUST bind every complete chain with verifiable receipts:

- Simple chain receipt (`internal/review/receipt.go`): `Receipt.BindingHash = domainHash("biggz-ai.review-receipt/v1\x00" + writeLengthPrefixed(genesis, head, count, lineage))`. `Receipt.Verify(chain)` MUST recompute and compare `GenesisRevision`, `HeadRevision`, `EventCount`, and `BindingHash`; any mismatch MUST fail.
- Persisted terminal receipt (`internal/review/finalize.go` `PersistedReceipt`): MUST be persisted under `receipts/<sha256>.json` where `<sha256> = sha256Hex(indent JSON)` via `publishNoReplace`. MUST bind `genesis_revision` + `head_revision` + `baseTree` + `initialReviewTree` + `finalCandidateTree` + `pathsDigest` + `fixDeltaHash` + `policyHash` + `evidenceHash` + `selectedLenses` + `lensSubjects` + `resolved/standing finding IDs` + `terminal_state="completed"` + `cumulativeCorrectionLines` under `domainHash("biggz-ai.review-receipt-binding/v1\x00" + jsonPayload)`. `Validate()` MUST verify schema, lineage identity, SHA fields, canonical lens ordering, finding set disjointness, and self-hash (legacy zero-cumulative receipts MAY compare against `computeLegacyHash()` when cumulative is 0).
- Fix-delta binding: MUST provide `FixDeltaHashForSnapshot(baseTree, candidateTree, pathsDigest, cumulative, ledgerIDs)` in `internal/review/finalize.go` (with bindings in `receipt.go`/`snapshot.go`, helpers in `model/hash.go`). When `cumulative == 0` it MUST return `EmptyFixDeltaHash = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"` (SHA-256 of zero bytes). Otherwise it MUST return `domainHash("fix-delta/v1\x00" + writeLengthPrefixed(baseTree, candidateTree, pathsDigest, cumulative, ledgerIDs...))`. Legacy `payloadSHA256("fix-delta:%d")` MUST NOT be used. `PersistedReceipt.Validate()` MUST reject a flat binding.

#### Scenario: Valid receipt verification (simple)

- GIVEN a lineage with four events and a `Receipt` with matching genesis, head, and count
- WHEN `Receipt.Verify(chain)` is called
- THEN verification MUST succeed

#### Scenario: Tampered chain after receipt

- GIVEN a `Receipt` and a lineage where one event was modified post-receipt
- WHEN `Receipt.Verify(chain)` is called
- THEN verification MUST fail (binding hash mismatch or genesis/head/count mismatch)

#### Scenario: Zero cumulative empty hash

- GIVEN `cumulative = 0`
- WHEN `FixDeltaHashForSnapshot` is called
- THEN it MUST return `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`

#### Scenario: Fix-delta binding differs from flat

- GIVEN `cumulative = 2` with non-empty trees
- WHEN `FixDeltaHashForSnapshot` is compared to `payloadSHA256("fix-delta:2")`
- THEN they MUST differ and `PersistedReceipt.Validate()` MUST reject the flat hash

#### Scenario: Persisted receipt self-hash valid

- GIVEN a `PersistedReceipt` built by `Finalize` with correct `FixDeltaHashForSnapshot`
- WHEN `PersistedReceipt.Validate()` is called
- THEN it MUST succeed and `receiptHash == computeHash()`

### Requirement: Flock-based File Lock

The system MUST provide cross-process exclusive locking via advisory `flock` (`LOCK_EX | LOCK_NB`) on `<storeDir>/.lock` (`NewFileLock(dir)`) and on named lock files (`NewNamedFileLock(dir, name)`). `LockFilePath()` MUST resolve to `<dir>/.lock` (or `<dir>/<name>`). `Acquire()` MUST:

- Call `os.MkdirAll(dir, 0755)`, open `.lock` with `O_CREATE|O_RDWR`, attempt `flockExclusive(fd)` (non-blocking).
- On success, truncate, write `"<pid>\n<RFC3339Nano>\n"`, sync, and retain `fd` for `Release()`.
- On contention, inspect staleness: if `mtime > 5m` (`staleLockAge`) OR pid from first line is no longer alive (`kill(pid,0)` on Unix; always alive on Windows), treat as stale and remove then retry once. Otherwise return `*BusyError{Path}`.
- `IsBusy(err)` MUST detect `*BusyError` via `errors.As`. `AcquireWithTimeout(timeout)` MUST poll `Acquire()` every 100 ms until deadline.
- `Release()` MUST call `flockUnlock(fd)`, close `fd`, and remove the lock file. On Windows, fallback to `O_CREATE|O_EXCL` with identical staleness semantics.

The lineage store MUST hold this lock for every `Append`/`LoadChain` mutation (`WithFileLock`/`WithNamedFileLock`).

#### Scenario: Contended lock returns BusyError

- GIVEN `.lock` is held by a live process (mtime < 5m, pid alive)
- WHEN another process calls `Acquire()`
- THEN it MUST return `*BusyError` and `IsBusy(err)` MUST be true

#### Scenario: Stale lock (age > 5m) is reclaimed

- GIVEN `.lock` with mtime older than 5 minutes
- WHEN `Acquire()` is called
- THEN it MUST remove the stale file and acquire successfully on retry

#### Scenario: Stale lock (dead PID) is reclaimed

- GIVEN `.lock` whose PID no longer exists (Unix `kill 0` fails) and mtime < 5m
- WHEN `Acquire()` is called
- THEN it MUST treat it as stale, remove, and acquire

#### Scenario: Release unlocks

- GIVEN a held `FileLock`
- WHEN `Release()` is called
- THEN the file MUST be removed and a subsequent `Acquire()` MUST succeed

### Requirement: Burn Semantics with BurnEnabled

The system MUST gate burn in `internal/review/finalize.go` via `var BurnEnabled bool` (default `true`). Behavior:

- When `BurnEnabled == true`, a successful `Finalize(repo, lineageID)` MUST: write the persisted receipt under `receipts/<sha256>.json`, append a `complete_review` event referencing `receipt_path`/`receipt_hash`, then invoke `burnReceiptLocked` which appends a `burn_review` event (`Operation = "burn_review"`, `Schema = "biggz-ai.review-burn-event/v1"`), writes `burned.json` tombstone (`{receipt_hash, receipt_path, burned_at}`), and deletes the receipt file so it becomes ephemeral. `Store.IsBurned()` / `IsChainBurned()` MUST report true when `burned.json` exists or any record has `Operation == "burn_review"`. The delivery gate (`review gate`) MUST surface `DeliveryBurned` for burned lineages; `os.Remove` without tombstone MUST NOT occur. A second `Finalize` on a burned lineage MUST return `ErrAlreadyBurned`.
- When `BurnEnabled == false`, `Finalize` MUST preserve the receipt file under `receipts/<sha256>.json`, MUST NOT write `burned.json`, MUST NOT append a burn event, and `IsBurned()` MUST remain false.

#### Scenario: Burn tombstones

- GIVEN `BurnEnabled = true` and a lineage that completes `Finalize` successfully
- WHEN `Finalize` returns
- THEN `burned.json` MUST exist, the chain MUST contain a `burn_review` event, the receipt file MUST be deleted, and `IsChainBurned` MUST be true; the gate MUST report `DeliveryBurned`

#### Scenario: Burn disabled preserves receipt

- GIVEN `BurnEnabled = false`
- WHEN `Finalize` succeeds
- THEN `receipts/<hash>.json` MUST remain on disk, `burned.json` MUST NOT exist, and `IsChainBurned` MUST be false

#### Scenario: Re-finalize on burned lineage fails

- GIVEN a lineage already burned
- WHEN `Finalize` is invoked again
- THEN it MUST return `ErrAlreadyBurned` and MUST NOT append further events

### Requirement: Correction Budget Counters

The system MUST enforce `model/review.go` `MaxFixRounds = 1` and `MaxScopedValidations = 1`. The 13-state FSM (`model/fsm.go`) MUST guard `NeedsChanges → ChangesSubmitted` with `BudgetCheck: "fix-rounds"` (`FixRounds < 1`) and `ChangesSubmitted → ReReview` with `BudgetCheck: "scoped-validations"` (`ScopedValidations < 1`). Violation MUST return verbatim `"budget exceeded: fix rounds exhausted (1/1)"` or `"budget exceeded: scoped validations exhausted (1/1)"`. Self-transitions (`current == target`) are always valid.

#### Scenario: Second round blocked

- GIVEN `BudgetCounters{FixRounds: 1}`, `Status = NeedsChanges`, `Role = Author`
- WHEN `FSM.Transition(NeedsChanges, ChangesSubmitted, Author, {FixRounds:1})` is called
- THEN it MUST reject with `budget exceeded: fix rounds exhausted (1/1)`

#### Scenario: First round allowed

- GIVEN `BudgetCounters{FixRounds:0, ScopedValidations:0}`, `Status = NeedsChanges`
- WHEN `FSM.Transition(NeedsChanges, ChangesSubmitted, Author, {0,0})` is called
- THEN it MUST succeed

#### Scenario: Scoped validation exhausted

- GIVEN `BudgetCounters{ScopedValidations:1}`, `Status = ChangesSubmitted`
- WHEN `FSM.Transition(ChangesSubmitted, ReReview, Reviewer, {0,1})` is called
- THEN it MUST reject with `budget exceeded: scoped validations exhausted (1/1)`

### Requirement: Role-Based Transition Guards

The system MUST enforce the FSM guard table (`model/fsm.go` `guardTable`) for the 13-state review lifecycle. `From == ""` denotes a wildcard (`Any` state). Roles are `Author`, `Reviewer`, `Lead`, `Admin`.

| From | To | Permitted Roles | Precondition | Budget Check |
|------|----|-----------------|--------------|--------------|
| unreviewed | in_review | Reviewer, Lead | evidence-non-empty | — |
| in_review | needs_changes | Reviewer, Lead | — | — |
| needs_changes | changes_submitted | Author | — | fix-rounds (<1, MaxFixRounds=1) |
| changes_submitted | re_review | Reviewer, Lead | — | scoped-validations (<1, MaxScopedValidations=1) |
| in_review | approved | Reviewer, Lead, Admin | all-policies-pass | — |
| in_review | escalated | Lead, Admin | escalation-reason-provided | — |
| Any | invalidated | Admin | scope-change-detected | — |
| Any | blocked | Lead, Admin | policy-violation | — |
| Any | withdrawn | Author | — | — |
| approved | superseded | Lead, Admin | superseding-review-exists | — |
| Any | completed | Lead, Admin | all-policies-pass-receipt-valid | — |
| completed | archived | Lead, Admin | 30-day-minimum | — |

`Invalidated`, `Blocked`, `Withdrawn`, and `Completed` are logically terminal but remain subject to correct role/wildcard matching; `Invalidated/Withdrawn` lineages MUST NOT be finalizable (see finalized-state guard in `finalize.go`).

#### Scenario: Author escalates — rejected

- GIVEN state `in_review` and `Role = Author`
- WHEN `Transition(in_review, escalated, Author, {0,0})` is called
- THEN the system MUST reject with `role Author not permitted for transition in_review → escalated`

#### Scenario: Admin invalidates from any state — allowed

- GIVEN state `needs_changes` and `Role = Admin`
- WHEN `Transition(needs_changes, invalidated, Admin, {0,0})` is called
- THEN it MUST succeed (wildcard `Any → invalidated`)

#### Scenario: Lead approves from in_review — allowed

- GIVEN state `in_review` and `Role = Lead`
- WHEN `Transition(in_review, approved, Lead, {0,0})` is called
- THEN it MUST succeed

#### Scenario: Reviewer creates withdrawn — rejected

- GIVEN state `approved` and `Role = Reviewer`
- WHEN `Transition(approved, withdrawn, Reviewer, {0,0})` is called
- THEN it MUST reject (only `Author` may withdraw)

### Requirement: Lineage Inventory

The system MUST implement `biggz review list` enumerating every lineage under `<commonDir>/biggz/review-transactions/` (discovered via `resolveGitCommonDir`), showing lineage ID, current state (derived from last non-burn event `operation`), and last event timestamp. Empty store MUST list zero lineages without error.

#### Scenario: Three lineages

- GIVEN three lineages each with at least one event under `v1/events/`
- WHEN `biggz review list` runs
- THEN all three lineage IDs MUST appear with their derived state and timestamp

#### Scenario: Empty store

- GIVEN no lineage directories
- WHEN `biggz review list` runs
- THEN it MUST return an empty list

### Requirement: Lineage Status

The system MUST implement `biggz review status <lineage-id>` returning `headHash`, `genesisHash`, event `count`, `chainValid` (from `Validate()`), `receiptValid` (from `PersistedReceipt.Validate()` or `Receipt.Verify()` when present), `isBurned`, budget counter values (`FixRounds`, `ScopedValidations` vs `MaxFixRounds=1`/`MaxScopedValidations=1`), and the frozen start-plan budget (`correctionBudget`, `originalChangedLines`) when genesis carries `StartEventPayload`.

#### Scenario: Valid lineage status

- GIVEN a lineage with four events, a valid receipt, and `BurnEnabled=false`
- WHEN `biggz review status <id>` runs
- THEN output MUST include `headHash`, `genesisHash`, `eventCount=4`, `chainValid=true`, `receiptValid=true`, `isBurned=false`, and current budget counters `{0,0}` with maxima `{1,1}`

#### Scenario: Burned lineage status

- GIVEN a lineage finalized with `BurnEnabled=true` (thus `burned.json` present)
- WHEN `biggz review status <id>` runs
- THEN it MUST report `isBurned=true` and `DeliveryBurned` at the gate layer
