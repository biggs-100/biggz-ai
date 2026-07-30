# Delta for Core Review

## MODIFIED Requirements

### Requirement: ReviewState Structure

The system MUST define a ReviewState structure that carries Status, Evidence chain, MerkleRoot, SchemaVersion, Role, LineageID, and BudgetCounters fields.
(Previously: Status, Evidence, MerkleRoot, SchemaVersion — no Role, LineageID, or BudgetCounters)

| Field | Type | Description |
|-------|------|-------------|
| Role | string | Actor role: Author, Reviewer, Lead, Admin |
| LineageID | string | Reference to the event store lineage |
| BudgetCounters | struct | { FixRounds int, ScopedValidations int } |

#### Scenario: Happy path — full ReviewState

- GIVEN a completed review with Role "Reviewer" and LineageID "abc123"
- WHEN the ReviewState is constructed with Status Completed, three evidence entries, SchemaVersion "1.0", and BudgetCounters {1, 2}
- THEN MerkleRoot MUST be a non-empty string derived from evidence chain
- AND SchemaVersion MUST be "1.0"
- AND Role MUST be "Reviewer"
- AND BudgetCounters.FixRounds MUST be 1

#### Scenario: Edge case — zero evidence entries

- GIVEN a newly created review with Role "Author" and zero evidence
- WHEN the ReviewState is constructed with Status Pending and empty Evidence chain
- THEN MerkleRoot MUST be an empty string
- AND the ReviewState MUST be valid with default BudgetCounters {0, 0}

### Requirement: FSM Transition Validation

The system MUST implement a 13-state finite state machine with validated transitions. The states MUST be: Unreviewed, InReview, NeedsChanges, ChangesSubmitted, ReReview, Approved, Escalated, Invalidated, Blocked, Withdrawn, Superseded, Completed, Archived. The FSM MUST reject transitions that violate role guards, preconditions, or correction budget limits.
(Previously: 5 states — Pending, InProgress, Completed, Archived, Failed — no role guards, preconditions, or budget counters)

| Transition | Role Guard | Precondition | Budget Check |
|-----------|-----------|-------------|--------------|
| Unreviewed → InReview | Reviewer, Lead | Evidence non-empty | None |
| InReview → NeedsChanges | Reviewer, Lead | None | None |
| NeedsChanges → ChangesSubmitted | Author | None | FixRounds < max |
| ChangesSubmitted → ReReview | Reviewer, Lead | None | ScopedValidations < max |
| InReview → Approved | Reviewer, Lead, Admin | All policies pass | None |
| InReview → Escalated | Lead, Admin | Escalation reason provided | None |
| Any → Invalidated | Admin | Scope change detected | None |
| Any → Blocked | Lead, Admin | Policy violation | None |
| Any → Withdrawn | Author | None | None |
| Approved → Superseded | Lead, Admin | Superseding review exists | None |
| Any → Completed | Lead, Admin | All policies pass, receipt valid | None |
| Completed → Archived | Lead, Admin | 30-day minimum since Complete | None |

#### Scenario: Happy path — valid chain Unreviewed → Approved

- GIVEN a ReviewState with Status Unreviewed and Role "Reviewer"
- WHEN transitioning Unreviewed → InReview → Approved
- THEN each transition MUST succeed
- AND the final Status MUST be Approved

#### Scenario: Role guard rejects invalid actor

- GIVEN a ReviewState with Status InReview and actor with Role "Author"
- WHEN the Author attempts the InReview → Escalated transition
- THEN the FSM MUST reject the transition
- AND return a role-not-permitted error

#### Scenario: Precondition blocks approval

- GIVEN a ReviewState with Status InReview where a policy evaluation fails
- WHEN the InReview → Approved transition is attempted
- THEN the FSM MUST reject the transition
- AND return a precondition-failed error

#### Scenario: Budget counter blocks re-review

- GIVEN a ReviewState at max ScopedValidations (5) and Status ChangesSubmitted
- WHEN the ChangesSubmitted → ReReview transition is attempted
- THEN the FSM MUST reject with a budget-exceeded error
