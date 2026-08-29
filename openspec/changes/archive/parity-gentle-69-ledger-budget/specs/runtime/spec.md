# Delta for runtime

## ADDED Requirements

### Requirement: Ledger Verify-Before-Commit (CAS)

The system MUST enforce verify-before-commit: `commitRecordLocked` MUST replay `loadRecord(revision)` inside `withStoreLock` before `writeLedgerHead` and advance HEAD only if `candidate.Status.Revision == revision`. On mismatch it MUST fail closed without mutating HEAD/records.

#### Scenario: CAS refuses stale revision

- GIVEN HEAD is `R1`
- WHEN commit presents candidate `R0 != R1`
- THEN it MUST be rejected with CAS conflict and HEAD MUST stay `R1`

#### Scenario: HEAD advances on match

- GIVEN HEAD `R1` and candidate derived from `R1`
- WHEN commit presents `Revision == R1`
- THEN record MUST be persisted and HEAD MUST advance to `R2 == sha256(canonical)`

#### Scenario: Concurrent serialize

- GIVEN two writers loaded `R1` and writer A committed `R1→R2`
- WHEN writer B attempts `R1→R3`
- THEN B MUST be rejected with CAS conflict

### Requirement: Dual Budget Single Owner

The system MUST track `MaxAttempts` and `MaxChangedLines` with per-attempt `ChangedLines` and carried `CumulativeChangedLines`. The single predicate `CumulativeChangedLines + changedLines > MaxChangedLines` MUST own line-budget exhaustion for `Acquire`, `Finish`/`Settle`, and replay paths. No duplicate inequality SHALL exist.

#### Scenario: Blocked when cumulative+delta exceeds max

- GIVEN `Cumulative=300`, `MaxLines=400`
- WHEN acquiring `changedLines=150`
- THEN it MUST be rejected `blocked(budget_exhausted)` (300+150>400)

#### Scenario: Admitted within budget

- GIVEN `Cumulative=300`, `MaxLines=400`
- WHEN acquiring `changedLines=80`
- THEN it MUST succeed and after settle cumulative MUST be `380`

#### Scenario: Single predicate ownership

- GIVEN finish and status replay evaluate line budget
- WHEN inspecting code paths
- THEN both MUST call the same predicate without inline duplication

### Requirement: Interrupted Refund Capped at 2×

The system MUST use `runtimeAttemptDeliveredIncrement`: `interrupted && changedLines>0` increments delivered; otherwise not. `runtimeRefundedAttempts() <= MaxAttempts` MUST cap refunds to `2×MaxAttempts` total. `Acquire`/`Begin` MUST reject when cap exhausted (gentle 2243/2217).

#### Scenario: Interrupted with lines counts

- GIVEN `MaxAttempts=3`, `interrupted` `changedLines=20`
- WHEN delivered increment evaluated
- THEN it MUST count as delivered

#### Scenario: Interrupted without lines refunded

- GIVEN `interrupted` `changedLines=0`
- WHEN evaluated
- THEN it MUST NOT count as delivered and MUST be refund-eligible

#### Scenario: Blocks after 2× cap

- GIVEN `MaxAttempts=3`, `refunded==3`
- WHEN acquiring
- THEN it MUST be rejected `blocked(budget_exhausted)`

### Requirement: Rescope Exhausted Wedge

When exhausted (`DecisionRequired` or budgets reached), `Rescope` MUST require `MaxAttempts > carried CumulativeAttempts` AND `MaxChangedLines > carried CumulativeChangedLines`. Cumulative counters MUST never reset; attempts slice MUST be preserved.

#### Scenario: Refuses unless both exceed carried

- GIVEN exhausted `cumul=5/600`, `Max=5/600`
- WHEN `Rescope` proposes `MaxAttempts=5` `MaxLines=700`
- THEN it MUST be rejected (attempts not > carried)

#### Scenario: Admits when wedge satisfied

- GIVEN same exhausted ledger
- WHEN `Rescope` proposes `7/800`
- THEN it MUST succeed and cumulative MUST still read `5/600`

#### Scenario: Cumulative preserved

- GIVEN ledger 4 attempts 350 lines
- WHEN rescope
- THEN attempts length and cumulative sum MUST be unchanged

### Requirement: Runtime Record Rejection Taxonomy

The system MUST use single typed `RuntimeRecordRejectedError` for all record rejections (hash, schema, lineage, staleness). Callers MUST detect via `errors.As`. No parallel string-only paths SHALL exist.

#### Scenario: Hash mismatch typed

- GIVEN record bytes hash to `H' != H`
- WHEN `loadRecord` validates
- THEN it MUST return `RuntimeRecordRejectedError`

#### Scenario: Unified handling

- GIVEN any rejection
- WHEN checking `errors.As(err, *RuntimeRecordRejectedError)`
- THEN it MUST succeed
