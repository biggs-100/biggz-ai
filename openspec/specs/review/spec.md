# Delta for review

## ADDED Requirements

### Requirement: PersistedReceipt Cumulative Ledger

The system MUST persist `FixDeltaHash` and `CumulativeCorrectionLines` in `PersistedReceipt`. `FixDeltaHash` MUST be fix-delta `sha256:<hex>` when correction exists else `EmptyFixDeltaHash`. `CumulativeCorrectionLines` MUST be total lines across rounds (≥0). `computeHash()` MUST bind both fields. `Validate()` MUST reject bad identities/hash. Missing `cumulativeLines` MUST decode as `0`; missing `fix_delta_hash` as `EmptyFixDeltaHash` (additive, legacy-compatible).

#### Scenario: Persist real hash after correction

- GIVEN finalized lineage with 2-line correction
- WHEN `buildReceipt` materializes receipt
- THEN `FixDeltaHash` MUST NOT equal `EmptyFixDeltaHash` and `cumulativeLines` MUST be `2`

#### Scenario: Legacy receipt decodes to zero

- GIVEN receipt JSON without `cumulativeLines`
- WHEN decoded and `Validate()` runs
- THEN `cumulativeLines` MUST be `0` and `Validate()` MUST pass

#### Scenario: Hash binding covers new fields

- GIVEN valid receipt with `cumulativeLines=3`
- WHEN changed to `4`
- THEN `computeHash()` MUST differ and validation with old hash MUST fail

### Requirement: Cumulative Validation via ValidateCorrectionActual

The system MUST wire `ValidateCorrectionActual(actual,cumulative,budget)` with persisted `cumulativeLines` at correction completion. It MUST fail when `cumulative+actual > budget` with budget-naming error for escalation. Negative inputs MUST be rejected. `FixRounds`/`MaxFixRounds=3` MUST stay independently enforced.

#### Scenario: Within budget passes

- GIVEN `budget=3` `cumulative=1`
- WHEN `ValidateCorrectionActual(1,1,3)` called
- THEN MUST return nil

#### Scenario: Cumulative over-budget escalates

- GIVEN `budget=3` `cumulative=2`
- WHEN `ValidateCorrectionActual(2,2,3)` called (total 4)
- THEN MUST return error containing `budget` and `escalate`

### Requirement: deriveNextTransition Deducts Consumed Lines

`deriveNextTransition` on correction branch (finalized + blocking findings) MUST return `budget_remaining = max(0, budget.CorrectionLines - cumulativeLines)` where `cumulativeLines` is terminal receipt value plus post-finalize events. When `budget` is nil, `remaining` MUST be `0`. Lying `remaining = budget.CorrectionLines` MUST be removed.

#### Scenario: Partial consumption

- GIVEN `CorrectionLines=3` `cumulative=2`
- WHEN `deriveNextTransition` runs with blocking findings
- THEN `budget_remaining` MUST be `1` and `action` MUST be `correction`

#### Scenario: Exhausted budget clamped to zero

- GIVEN `CorrectionLines=10` `cumulative=10` (or 7>5)
- WHEN `deriveNextTransition` runs
- THEN `budget_remaining` MUST be `0`

#### Scenario: Nil budget

- GIVEN lineage without frozen budget
- WHEN `deriveNextTransition` runs
- THEN `budget_remaining` MUST be `0`

### Requirement: Verification and Mirror Continuity

`finalizeIdempotent`, `deriveExpectedReceipt`, `reMaterializeReceipt`, `RetryFinalVerification`, and `mirrorPayloads` MUST carry `FixDeltaHash`/`cumulativeLines` consistently. Re-materialized receipt MUST be hash-identical (`receipts/<sha256>.json`, same `ReceiptHash`). Tampered receipt failing `Validate()` MUST NOT be overwritten. `receiptMirror`/`gateContextMirror` MUST surface real `FixDeltaHash` and cumulative.

#### Scenario: Idempotent preserves cumulative

- GIVEN finalized lineage with `cumulative=2` real hash
- WHEN `finalizeIdempotent` runs
- THEN receipt MUST retain same `FixDeltaHash`, `cumulativeLines`, `ReceiptHash`

#### Scenario: Re-materialization hash-identical

- GIVEN missing receipt file but valid `complete_review` reference
- WHEN `RetryFinalVerification` rebuilds
- THEN `ReceiptHash` and path MUST equal original

### Requirement: Budget Exhaustion Surfaces as Blocking Reason

When `cumulative >= budget.CorrectionLines` or `FixRounds >= MaxFixRounds`, system MUST surface exhaustion as blocking. `deriveNextTransition` with `remaining=0` MUST stay `correction` and next `actual` MUST be rejected via `ValidateCorrectionActual`. `biggz review status --json` MUST expose `budget_remaining` and cumulative fields for `blockedReasons` routing.

#### Scenario: Zero remaining forces escalation

- GIVEN `budget=3` `cumulative=3` blocking findings remain
- WHEN `deriveNextTransition` runs then `ValidateCorrectionActual(1,3,3)` called
- THEN `action` MUST be `correction` with `0` and validation MUST fail

#### Scenario: Status exposes budget for blockedReasons

- GIVEN `Cumulative=2` `Budget=3`
- WHEN `biggz review status --json` runs
- THEN `next_transition.budget_remaining` MUST be `1` with `cumulativeLines` and `FixDeltaHash` visible
