# Review Authority Specification

## Purpose

The Review Authority domain defines the content-addressed event store, chain validation, receipt binding, role-based transition guards, correction budget counters, and lineage inventory/status commands. Every review transition is recorded as an immutable SHA-256-named event file.

## Requirements

### Requirement: Content-Addressed Event Store

The system MUST persist every review transition as a file under `.git/biggz/review-transactions/<lineage>/`. Each file name MUST equal the SHA-256 hex digest of its content. Each event MUST carry:

| Field | Type | Description |
|-------|------|-------------|
| PrevHash | string | SHA-256 of preceding event (empty for genesis) |
| Role | string | Actor role: Author, Reviewer, Lead, Admin |
| Action | string | Transition action taken |
| Timestamp | string | ISO 8601 timestamp |
| Payload | object | Transition-specific data |

#### Scenario: Happy path — append three events

- GIVEN an empty lineage directory
- WHEN three events are appended sequentially
- THEN each file MUST be named SHA-256(content)
- AND each event (except genesis) MUST have PrevHash == previous file name

#### Scenario: Empty lineage

- GIVEN no events in a lineage directory
- WHEN the system opens the lineage
- THEN it MUST return an empty lineage with event count zero

### Requirement: Chain Validation

The system MUST provide Validate() that recomputes the hash chain from genesis through head and returns an integrity verdict.

#### Scenario: Valid chain

- GIVEN a lineage with three events and correct PrevHash links
- WHEN Validate() is called
- THEN the verdict MUST indicate integrity preserved

#### Scenario: Tampered file

- GIVEN a lineage where one event file's content was modified
- WHEN Validate() is called
- THEN the verdict MUST indicate integrity broken

### Requirement: Receipt Binding

A receipt MUST bind genesis hash + head hash + total event count. Receipt verification MUST recompute the chain and match all three.

#### Scenario: Valid receipt verification

- GIVEN a lineage with four events and a receipt with matching genesis, head, and count
- WHEN the receipt is verified
- THEN verification MUST succeed

#### Scenario: Tampered chain after receipt

- GIVEN a receipt and a lineage where one event was modified post-receipt
- WHEN the receipt is verified
- THEN verification MUST fail

### Requirement: Role-Based Transition Guards

The system MUST reject transitions where the actor's role is not permitted for the current state and action.

| From State | To State | Permitted Roles |
|-----------|----------|-----------------|
| Unreviewed | InReview | Reviewer, Lead |
| NeedsChanges | ChangesSubmitted | Author |
| InReview | Escalated | Lead, Admin |

#### Scenario: Author escalates — rejected

- GIVEN a review InReview and an Author actor
- WHEN Author attempts Escalated
- THEN the system MUST reject with a role-not-permitted error

### Requirement: Correction Budget Counters

Each lineage MUST track fix rounds and scoped validations. The system MUST reject transitions exceeding configured maximums.

#### Scenario: Fix rounds exhausted

- GIVEN a lineage at max fix rounds (3)
- WHEN attempting NeedsChanges → ChangesSubmitted
- THEN the system MUST reject with a budget-exceeded error

### Requirement: Lineage Inventory

The system MUST implement `biggz review list` enumerating every lineage in the store, showing lineage ID, current state, and last event timestamp.

#### Scenario: Three lineages

- GIVEN three lineages with events
- WHEN `biggz review list` runs
- THEN all three lineage IDs appear with state and timestamp

### Requirement: Lineage Status

The system MUST implement `biggz review status <lineage-id>` returning head hash, event count, chain valid status, receipt status, and budget counter values.

#### Scenario: Valid lineage status

- GIVEN a lineage with four events and a valid receipt
- WHEN `biggz review status <id>` runs
- THEN output includes head hash, event count=4, chain=valid, receipt=valid, and current budget counters
