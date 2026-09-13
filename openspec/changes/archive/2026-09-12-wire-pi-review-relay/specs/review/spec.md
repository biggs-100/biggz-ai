# Delta for review

## ADDED Requirements

### Requirement: Pi Reviewer Execute Mode

`biggz review capture-result --agent pi --execute` MUST be an explicit mode of the existing verb, accepted only with `--agent pi` and mutually exclusive with `--input`, `--preflight` and `--materialize` (each pair a usage error). The pi relay handshake (`BIGGZ_PI_REVIEW_RELAY_CONTRACT`) and RDD mutation authorization MUST remain preconditions. Execution MUST run Preflight → materialize → `PiAdapter.Review` → `Capture` with the existing admission, slot immutability and `expected_revision` CAS; no new authority, and no failure path MAY capture or leave a partial slot.

#### Scenario: Execute runs the pipeline

- GIVEN a pi-eligible lineage offering `collect` with RDD enabled
- WHEN `capture-result --agent pi --execute` runs
- THEN it MUST preflight, materialize, run the reviewer, and capture its bytes
- AND the lineage MUST advance as `--input` would

#### Scenario: Conflicting flags or missing preconditions refuse

- GIVEN `--execute` with another mode flag, no handshake, or disabled RDD
- WHEN execute is invoked
- THEN conflicting flags MUST be a usage error, the rest MUST refuse before materialization
- AND nothing MUST launch or capture

#### Scenario: Admission failure captures nothing

- GIVEN a reviewer output the existing admission rejects
- WHEN execute submits it through `Capture`
- THEN it MUST be rejected exactly as `--input` does, with no event and no partial slot

### Requirement: Bounded Reviewer Execution and Typed Transport Failures

Execute MUST bound the reviewer with a finite default timeout, accept a `--timeout` override, and terminate the reviewer on cancelation. Timeout, empty stdout and non-zero exit MUST each fail typed naming the stage, capturing nothing and leaving no partial slot (empty stdout already refuses typed: `pi_adapter.go:75-77`). Raw stdout MUST reach `Capture` unmodified (no parsing, filtering or normalization), with the bounded stdout cap enforced before capture.

#### Scenario: Timeout and cancelation terminate the reviewer

- GIVEN a reviewer still running when the timeout elapses or cancelation lands
- WHEN the bound expires or cancelation arrives
- THEN the reviewer MUST be terminated and no capture MUST occur
- AND a timeout MUST fail typed

#### Scenario: Empty stdout and non-zero exit are typed failures

- GIVEN a reviewer exiting non-zero or with empty stdout
- WHEN execute inspects the output
- THEN it MUST fail typed naming the stage, capturing nothing

#### Scenario: Raw stdout reaches Capture unmodified

- GIVEN valid reviewer stdout
- WHEN execute captures it
- THEN `Capture` MUST receive those exact bytes, unparsed and unfiltered

#### Scenario: Over-cap output refuses before capture

- GIVEN reviewer stdout over the artifact byte cap
- WHEN execute proceeds
- THEN it MUST refuse typed before capture and MUST NOT truncate

## MODIFIED Requirements

### Requirement: Verbatim Transport and Tool-Less Reviewer

Hosts MUST forward the materialized bytes to the reviewer unchanged; in the execute route the materialized task MUST travel byte-identical inside the runtime-composed prompt. The reviewer MUST run without repository tools (no live worktree inspection), and the prompt MUST be composed from binary-owned reviewer prompt assets — role assets (`internal/assets/prompts/review/*.md`) and the output-contract asset (`internal/assets/prompts/review-output-contract.md`) — plus the materialized task, never caller-authored. The output-contract asset MUST state the admitted result shape (subject echo, completed inspection of the frozen manifest, findings, concrete evidence, severe-finding evidence class and causal disposition), MUST NOT be a Go template, and MUST appear after the role text and before the materialized task. Model/provider selection MUST remain ambient, never pinned by the binary.

(Previously: only caller-authorship was prohibited.)

#### Scenario: Verbatim transport and tool-less run

- GIVEN a materialized reviewer task on a supported host
- WHEN the reviewer runs
- THEN it MUST receive the bytes unchanged and MUST complete without repository tool access

#### Scenario: Caller-authored prompt discarded

- GIVEN a caller-authored task body attempting to replace the provider prompt
- WHEN the host prepares the reviewer task
- THEN the caller body MUST be discarded and model/provider MUST remain user-selected

#### Scenario: Composed prompt keeps the segment byte-identical

- GIVEN a runtime-composed reviewer prompt
- WHEN the prompt reaches the reviewer
- THEN the materialized task segment MUST be byte-identical to the materialized task bytes
- AND the rest of the prompt MUST come only from binary-owned prompt assets

#### Scenario: Composed prompt carries the output contract before the task

- GIVEN a composed reviewer prompt
- WHEN the prompt is inspected
- THEN it MUST state the admitted result shape via the contract markers (`subject_hash`, `inspection`, `findings`, `evidence`, `evidence_class`, `causal_disposition`, single JSON object) between the role text and the materialized task segment
