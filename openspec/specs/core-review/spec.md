# Core Review Specification

## Purpose

The Core Review domain defines the fundamental data model and lifecycle for code review state management. It encompasses the ReviewState structure, evidence chain with Merkle integrity, finite state machine with validated transitions, and the policy evaluation interface.

## Requirements

### Requirement: ReviewState Structure

The system MUST define a ReviewState structure that carries Status, Evidence chain, MerkleRoot, and SchemaVersion fields.

#### Scenario: Happy path — full ReviewState

- GIVEN a completed review
- WHEN the ReviewState is constructed with Status set to Completed, three evidence entries, and SchemaVersion "1.0"
- THEN MerkleRoot MUST be a non-empty string derived from the evidence chain
- AND SchemaVersion MUST be "1.0"

#### Scenario: Edge case — zero evidence entries

- GIVEN a newly created review
- WHEN the ReviewState is constructed with Status set to Pending and an empty Evidence chain
- THEN MerkleRoot MUST be an empty string
- AND the ReviewState MUST be valid

### Requirement: Evidence Chain Integrity

The system MUST implement an append-only ordered evidence chain where each entry carries Position, Timestamp, Kind, Payload, PrevHash, and Hash. MerkleRoot MUST equal SHA-256 of the last entry's Hash.

#### Scenario: Happy path — three evidence entries

- GIVEN an empty evidence chain
- WHEN three evidence entries are appended sequentially
- THEN each entry MUST have PrevHash equal to the Hash of the previous entry (or empty string for the first entry)
- AND MerkleRoot MUST equal SHA-256 of the third entry's Hash

#### Scenario: Tamper detection

- GIVEN an evidence chain with three entries and a known MerkleRoot
- WHEN the Payload of the second entry is modified after append
- THEN recomputing the Hashes MUST produce a different MerkleRoot
- AND the modification MUST be detectable

### Requirement: Schema Versioning

The system MUST support schema versioning via the SchemaVersion field. Data serialized with one SchemaVersion MUST be readable but MAY require migration.

#### Scenario: Happy path — matching version

- GIVEN a ReviewState with SchemaVersion "1.0"
- WHEN the system reads this state with a reader expecting "1.0"
- THEN no migration is required

#### Scenario: Version mismatch

- GIVEN a serialized ReviewState with SchemaVersion "0.9"
- WHEN the system reads this state with a reader expecting "1.0"
- THEN the system SHOULD report a version mismatch
- AND the system MAY attempt migration

### Requirement: FSM Transition Validation

The system MUST implement a 5-state finite state machine with validated transitions. The states MUST be Pending, InProgress, Completed, Archived, and Failed. The FSM MUST reject invalid transitions.

#### Scenario: Happy path — valid transition chain

- GIVEN a ReviewState with Status Pending
- WHEN transitioning through Pending → InProgress → Completed
- THEN each transition MUST succeed
- AND the final Status MUST be Completed

#### Scenario: Invalid transition

- GIVEN a ReviewState with Status Archived
- WHEN attempting to transition to InProgress
- THEN the FSM MUST reject the transition
- AND return an error indicating the transition is not allowed

### Requirement: PolicyEvaluator Interface

The system MUST define a PolicyEvaluator interface with Name() returning a string and Evaluate(ctx, ReviewState) returning a PolicyVerdict struct containing Policy name, Passed boolean, Reason string, and Severity field.

#### Scenario: Happy path — passing policy

- GIVEN a PolicyEvaluator named "minimum-evidence" and a ReviewState with three evidence entries
- WHEN Evaluate is called
- THEN the Verdict MUST have Passed set to true
- AND Reason MUST describe why it passed

#### Scenario: Failing policy

- GIVEN a PolicyEvaluator named "minimum-evidence" configured to require at least one evidence entry
- WHEN Evaluate is called on a ReviewState with zero evidence entries
- THEN the Verdict MUST have Passed set to false
- AND Severity MUST indicate the severity level of the failure
