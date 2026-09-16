# Delta for review-authority

## ADDED Requirements

### Requirement: Verification Subject Tree Validity

Every git tree value accepted as a verification subject (`BaseTree`, `CandidateTree`) MUST match `^(?:[0-9a-f]{40}|[0-9a-f]{64})$` AND MUST NOT be the all-zero OID (a value that is empty after trimming `0`s). An unresolved tree MUST fail validation; it MUST NOT pass as empty, and empty trees MUST NOT be treated as a valid comparison pair. This contract is satisfiable by fixing the verification-subject surface (`VerificationSubject` / `SnapshotVerificationSubject` / `Resnapshot` in `internal/review/convergence.go`) or by deleting that module; a shipped snapshot path violating it MUST NOT exist.

#### Scenario: Zero OID and empty value rejected

- GIVEN tree fixtures `strings.Repeat("0", 40)`, `strings.Repeat("0", 64)`, and `""`
- WHEN tree validation runs against them
- THEN each MUST be rejected as invalid and MUST NOT be accepted as a real tree

#### Scenario: Resolved non-zero trees accepted, unresolved rejected

- GIVEN non-zero 40-hex and 64-hex OIDs, and `""` for an unresolved tree
- WHEN each value is validated
- THEN both OIDs MUST pass, and the unresolved value MUST be rejected rather than pass as empty

### Requirement: Snapshot Git Error Propagation

Every git invocation in the verification-subject snapshot path MUST propagate its error. A failed invocation MUST NOT be representable as an empty-but-valid tree, and the snapshot path MUST NOT return `nil` alongside empty trees. Either the surface satisfies this or it MUST be absent.

#### Scenario: Broken git yields an error, not an empty subject

- GIVEN a fixture where the snapshot git invocation fails (git absent from `PATH` or non-zero exit)
- WHEN the snapshot path runs
- THEN it MUST return a non-nil error, or the surface MUST be absent; a nil error with empty trees MUST NOT be produced

#### Scenario: Candidate invocation failure propagates too

- GIVEN a fixture where candidate tree derivation fails while base resolution succeeds
- WHEN the snapshot path runs
- THEN the failure MUST propagate as a non-nil error, or the surface MUST be absent

### Requirement: Workspace Projection Observability

For `projection == "workspace"` the candidate tree MUST be derived from the workspace state (staged plus unstaged changes), so a workspace mutation is observable as a difference from the base tree. A projection whose candidate command is byte-identical to its base command MUST be forbidden. Either the surface satisfies this or it MUST be absent.

#### Scenario: Workspace mutation changes the candidate

- GIVEN a healthy repository with a committed tree and an uncommitted modification to a tracked file, `projection=workspace`
- WHEN the subject is captured
- THEN `CandidateTree` MUST differ from `BaseTree`

#### Scenario: Candidate derivation is distinct from base derivation

- GIVEN `projection=workspace`
- WHEN the candidate derivation is inspected or executed
- THEN it MUST NOT equal the base command (`git rev-parse HEAD^{tree}`) and MUST reflect workspace content

### Requirement: Typed Verification Subject Mismatch

A mismatch between the expected and observed verification subject MUST be reported as a distinct, typed failure — not a generic error and never as convergence. Re-snapshot MUST return `nil` only when the subject genuinely equals the captured one. Either the surface satisfies this or it MUST be absent.

#### Scenario: Detected mutation reports a typed failure

- GIVEN a captured, valid subject and a subsequent workspace mutation
- WHEN re-snapshot detects a different tree hash
- THEN it MUST return a typed mutation error exposing expected and observed hashes (e.g. `*VerificationSubjectMutationError`), MUST NOT return `nil`, and MUST NOT degrade to a generic untyped message

#### Scenario: Genuine equality is the only convergence

- GIVEN a healthy repository, a valid captured subject, and no mutation in between
- WHEN re-snapshot runs
- THEN it MUST return `nil`, and no other outcome MAY be reported as convergence
