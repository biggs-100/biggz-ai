# Delta for core-review

## MODIFIED Requirements

### Requirement: FSM Transition Validation

The system MUST enforce `model/review.go` `MaxFixRounds=1`, `MaxScopedValidations=1` and `model/fsm.go` guards `<1`. Transitions `NeedsChanges→ChangesSubmitted` MUST fail if `FixRounds>=1`; `ChangesSubmitted→ReReview` MUST fail if `ScopedValidations>=1`.
(Previously: 3 and 5)

#### Scenario: Second round blocked

- GIVEN `BudgetCounters{FixRounds:1}`, `Status NeedsChanges`, `Role Author`
- WHEN `Transition(NeedsChanges,ChangesSubmitted,Author,{1,0})`
- THEN MUST reject `budget exceeded: fix rounds exhausted (1/1)`

#### Scenario: First round allowed

- GIVEN `BudgetCounters{0,0}`, `Status NeedsChanges`
- WHEN `Transition(NeedsChanges,ChangesSubmitted,Author,{0,0})`
- THEN MUST succeed

### Requirement: Evidence Chain Integrity

The system MUST compute `Evidence.Hash` in `model/hash.go` as `domainHash("biggz-ai.review-evidence/v1\x00"+writeLengthPrefixed(Position,Timestamp,Kind,Payload,PrevHash))` and `MerkleRoot` as `domainHash("biggz-ai.review-merkle/v1\x00"+writeLengthPrefixed(lastHash))`. Pipe `|` concat MUST be rejected.
(Previously: `Position|Timestamp|Kind|Payload|PrevHash`, `MerkleRoot=SHA256(lastHash)`)

#### Scenario: Domain vectors match gentle

- GIVEN gentle v2.5 vectors with `writeLengthPrefixed`+`\x00`
- WHEN `evidenceHash`/`MerkleRoot` computed
- THEN MUST match vectors and differ from legacy pipe

## ADDED Requirements

### Requirement: FixDelta Binding via FixDeltaHashForSnapshot

The system MUST provide `FixDeltaHashForSnapshot(baseTree,candidateTree,pathsDigest,cumulative,ledgerIDs)` in `internal/review/finalize.go` (and bindings in `internal/review/receipt.go`, `internal/review/snapshot.go`, helpers in `model/hash.go`). It MUST return `EmptyFixDeltaHash` if `cumulative==0` else `domainHash("fix-delta/v1\x00"+writeLengthPrefixed(...))`. Legacy `payloadSHA256("fix-delta:%d")` MUST NOT be used.

#### Scenario: Zero cumulative empty hash

- GIVEN `cumulative=0`
- WHEN `FixDeltaHashForSnapshot` called
- THEN MUST return `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`

#### Scenario: Binding differs from flat

- GIVEN `cumulative=2`
- WHEN `FixDeltaHashForSnapshot` vs `payloadSHA256("fix-delta:2")`
- THEN MUST differ; `PersistedReceipt.Validate()` MUST reject flat

### Requirement: Burn Semantics with BurnEnabled

The system MUST gate burn in `internal/review/finalize.go` via `BurnEnabled bool` (default true). When true, success MUST write `burned.json` tombstone and gate MUST surface `DeliveryBurned`; `os.Remove` without tombstone MUST NOT occur.

#### Scenario: Burn tombstones

- GIVEN `BurnEnabled=true` lineage finalized
- WHEN `Finalize` succeeds
- THEN `burned.json` MUST exist and gate reports `DeliveryBurned`

#### Scenario: Burn disabled preserves receipt

- GIVEN `BurnEnabled=false`
- WHEN `Finalize` succeeds
- THEN `receipts/<hash>.json` MUST remain and `IsChainBurned` false
