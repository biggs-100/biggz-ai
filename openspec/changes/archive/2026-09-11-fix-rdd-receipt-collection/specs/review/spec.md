# Delta for review

## ADDED Requirements

### Requirement: Go-Owned Materialization of the Reviewer Task

For a `collect` transition, the runtime MUST materialize the complete reviewer task in Go: binding, repository context, name-status, numstat, and per-path patch bytes with explicit delimiters. A `biggz review capture-result --materialize` surface MUST print exactly those bytes and MUST NOT capture, append events, or persist artifacts.

#### Scenario: Materialize prints the complete task

- GIVEN a lineage with an offered `collect` transition
- WHEN `capture-result --materialize` runs
- THEN it MUST print binding, repository context, name-status, numstat, and per-path patch bytes with explicit delimiters

#### Scenario: Materialize captures nothing

- GIVEN the lineage before and after a materialize run
- WHEN chain events and receipts are inspected
- THEN they MUST be unchanged and no capture MUST have occurred

### Requirement: Frozen-Tree Evidence Fidelity and Typed Byte-Cap Refusal

Patch bytes MUST be derived from the frozen base and candidate trees via per-path patch reads, binary-safe, through an isolated git invocation. Exceeding the byte cap MUST be a typed refusal and MUST NEVER be silent truncation.

#### Scenario: Evidence derived from frozen trees

- GIVEN frozen base/candidate trees including a binary path
- WHEN materialization runs
- THEN patch bytes MUST match per-path patches between the frozen trees, without live worktree reads

#### Scenario: Byte cap exceeded refuses

- GIVEN a candidate whose materialized bytes exceed the configured cap
- WHEN materialization runs
- THEN it MUST fail with a typed refusal naming the cap and MUST NOT emit truncated bytes

### Requirement: Non-Vacuity of Findings and All-Clear Evidence

A candidate with real triggerable findings MUST produce non-empty findings for the lens whose heuristic triggers. An all-clear result MUST carry concrete inspected evidence. An empty-hunk or materialized-nothing run MUST NOT be capturable as an all-clear.

#### Scenario: Known-bad candidate triggers findings

- GIVEN a candidate containing a change that triggers a selected lens heuristic
- WHEN that lens runs and its result is captured
- THEN findings MUST be non-empty for that lens

#### Scenario: Empty materialization cannot be all-clear

- GIVEN a materialization yielding no hunks or no inspected evidence
- WHEN capture admission evaluates
- THEN it MUST reject as vacuous and MUST NOT record an all-clear

### Requirement: Verbatim Transport and Tool-Less Reviewer

Hosts MUST forward the materialized bytes to the reviewer unchanged. The reviewer MUST run without repository tools (no live worktree inspection), and its prompt MUST NOT be caller-authored. Reviewer model/provider selection MUST remain ambient and MUST NOT be pinned by the binary.

#### Scenario: Verbatim transport and tool-less run

- GIVEN a materialized reviewer task on a supported host
- WHEN the reviewer runs
- THEN it MUST receive the bytes unchanged and MUST complete without repository tool access

#### Scenario: Caller-authored prompt discarded

- GIVEN a caller-authored task body attempting to replace the provider prompt
- WHEN the host prepares the reviewer task
- THEN the caller body MUST be discarded and model/provider MUST remain user-selected

### Requirement: Producer Parity Guard

A blocking review surface MUST NOT ship without a producer for every supported host. A guard test MUST fail when a blocking surface exists with no producer.

#### Scenario: Missing producer fails the guard

- GIVEN a blocking review surface with no producer registered for a supported host
- WHEN the guard test runs
- THEN it MUST fail

#### Scenario: Full coverage passes

- GIVEN every blocking surface has a producer for every supported host
- WHEN the guard test runs
- THEN it MUST pass
