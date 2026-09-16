# Delta for complexity-gates

## MODIFIED Requirements

### Requirement: CI Cyclomatic Gate

The CI `complexity` job MUST fail when any new/modified Go function in critical packages (`internal/review`, `internal/sdd`, `internal/verification`) has cyclomatic >15 via `gocyclo`. It MUST run after `format`, be `CostQuick`/`ReadOnly`, and MUST exclude `*_test.go` from blocking (informational warnings only). The producer MUST observe `gocyclo`'s own exit status: `|| true` masking MUST NOT be used, a non-zero exit or a failed tool build MUST fail the job, and the job MUST fail when zero functions were evaluated while the diff contains in-scope Go functions, so an empty violation list can never be read as "no violations".
(Previously: the producer ran `go run … gocyclo … || true`, so any tool failure produced empty output that the job mapped onto "no violations".)

#### Scenario: New function exceeds cyclomatic threshold

- GIVEN PR adds `Foo` in `internal/review/lens/foo.go` with cyclomatic 18
- WHEN CI `complexity` job runs on `git diff base...HEAD`
- THEN job MUST exit non-zero and report `Foo: cyclomatic 18 >15`

#### Scenario: Test file violation does not block

- GIVEN PR modifies `internal/review/foo_test.go` with cyclomatic 25
- WHEN CI `complexity` job runs
- THEN job MUST exit zero with informational warning listing the function

#### Scenario: Out-of-scope package ignored

- GIVEN PR adds function with cyclomatic 30 in `internal/cli`
- WHEN CI `complexity` job runs
- THEN job MUST exit zero and MUST NOT report it

#### Scenario: Producer failure fails the job

- GIVEN the pinned `gocyclo` fails to build or exits non-zero
- WHEN the `complexity` job runs
- THEN the job MUST exit non-zero and MUST NOT report zero violations

#### Scenario: Empty scan is a failure

- GIVEN the diff modifies Go functions in critical packages and the producer evaluated zero functions
- WHEN the `complexity` job runs
- THEN the job MUST exit non-zero

### Requirement: CI Cognitive Gate

The CI `complexity` job MUST fail when any new/modified Go function in critical packages has cognitive >20 via `gocognit`. Threshold 20 is fixed global. Same scoping and `*_test.go` exclusion as cyclomatic gate MUST apply. The producer MUST observe `gocognit`'s own exit status under the same fail-closed rule: no `|| true` masking, and a non-zero exit or failed tool build MUST fail the job.
(Previously: the producer ran `go run … gocognit … || true`, so a failed tool build produced an empty violation list.)

#### Scenario: New function exceeds cognitive threshold

- GIVEN PR adds `Bar` in `internal/sdd/service.go` with cognitive 22
- WHEN CI `complexity` job runs
- THEN job MUST exit non-zero and report `Bar: cognitive 22 >20`

#### Scenario: Both thresholds evaluated independently

- GIVEN function with cyclomatic 12 and cognitive 25
- WHEN CI `complexity` job runs
- THEN job MUST fail for cognitive and list only that violation

#### Scenario: Cognitive producer failure fails the job

- GIVEN the pinned `gocognit` fails to build or exits non-zero
- WHEN the `complexity` job runs
- THEN the job MUST exit non-zero and MUST NOT report zero violations
