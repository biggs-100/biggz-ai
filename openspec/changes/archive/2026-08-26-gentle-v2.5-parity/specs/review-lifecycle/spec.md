# Delta for review-lifecycle

## ADDED Requirements

### Requirement: Last-Event Closure Burns Lineage

The system MUST implement last-event closure where every terminal capture (`review/capture-result`, `review.capture-refuter`, `review/capture-validation`, `review.capture-correction-plan`) burns the exact active lineage on success. Zero-lens, reviewer, refuter, validator, and correction-plan paths all MUST burn. Burn MUST delete the lineage authority directory plus companion paths (`effect-markers/v1`, `incidents`) under version lock and maintenance lease, verify absence, and retire delivery without receipt, tombstone, or mirror. A burned lineage MUST never be reused; subsequent use MUST fail with lineage-not-found. Compact receipts MUST be retired.

#### Scenario: Reviewer last-event burn

- GIVEN an approved lineage with reviewer lens results captured
- WHEN `review/capture-result` commits terminal `approved`
- THEN system MUST burn the exact lineage directory and return `gentle-ai.review-last-event-closure/v1` with `store_revision`
- AND subsequent `STATUS` for that lineage MUST report not-found

#### Scenario: Zero-lens burn at START

- GIVEN a zero-lens START eligible for immediate closure
- WHEN terminal capture runs
- THEN it MUST close and burn the lineage without requiring lens evidence

#### Scenario: Correction-plan burn

- GIVEN `review.capture-correction-plan` commits with `correction_lines` and `request_hash`
- WHEN burn executes
- THEN it MUST verify revision equality, burn authority, and emit closure envelope
- AND lineages other than the burned one MUST remain unaffected

#### Scenario: Burned lineage rejected

- GIVEN a lineage already burned
- WHEN caller attempts `review/capture-result` or `FINALIZE` on same lineage
- THEN system MUST refuse with lineage-not-found and MUST NOT resurrect authority

### Requirement: Compact Receipts Retired

The system MUST NOT create, persist, or consume compact receipts. Legacy compact receipt fixtures and projections MUST be absent from authority checks.

#### Scenario: No compact receipt emitted

- GIVEN a review completes via last-event closure
- WHEN checking `reviews/transaction.json` or `ProjectStatusV1` paths
- THEN no `compact receipt` file or `reviewReceipt` key MUST exist

#### Scenario: Legacy receipt absence enforced

- GIVEN tagged tests or goldens
- WHEN scanned for `compact receipt` or `reviewReceipt`
- THEN they MUST report absence and tests MUST fail if a receipt is introduced
