# Complexity Gates Specification

## Purpose

Fixed thresholds cyclomatic >15 and cognitive >20 on new/modified Go functions in critical packages. CI blocks only diff-aware violations; legacy and test files are visible but never block.

## Requirements

### Requirement: CI Cyclomatic Gate

The CI `complexity` job MUST fail when any new/modified Go function in critical packages (`internal/review`, `internal/sdd`, `internal/verification`) has cyclomatic >15 via `gocyclo`. It MUST run after `format`, be `CostQuick`/`ReadOnly`, and MUST exclude `*_test.go` from blocking (informational warnings only).

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

### Requirement: CI Cognitive Gate

The CI `complexity` job MUST fail when any new/modified Go function in critical packages has cognitive >20 via `gocognit`. Threshold 20 is fixed global. Same scoping and `*_test.go` exclusion as cyclomatic gate MUST apply.

#### Scenario: New function exceeds cognitive threshold

- GIVEN PR adds `Bar` in `internal/sdd/service.go` with cognitive 22
- WHEN CI `complexity` job runs
- THEN job MUST exit non-zero and report `Bar: cognitive 22 >20`

#### Scenario: Both thresholds evaluated independently

- GIVEN function with cyclomatic 12 and cognitive 25
- WHEN CI `complexity` job runs
- THEN job MUST fail for cognitive and list only that violation

### Requirement: Grandfather Diff Semantics

Existing violations on `base` MUST be reported but MUST NOT block. The system MUST map violations to changed functions via `git diff base...HEAD` filtered to function boundaries. Renames or ambiguous diffs MUST NOT block and MUST emit a warning fallback.

#### Scenario: Legacy violation not re-blocked

- GIVEN `internal/verification/old.go:FuncOld` has cyclomatic 20 on `base` unmodified
- WHEN PR changes unrelated files
- THEN CI MUST report `FuncOld` in totals but exit zero

#### Scenario: Modified legacy function now blocks

- GIVEN `FuncOld` (cyclomatic 20 on base) is modified in the PR diff
- WHEN CI runs
- THEN CI MUST fail for `FuncOld`

#### Scenario: Rename with no function mapping

- GIVEN a file rename where `git diff` reports no mappable function hunks
- WHEN CI evaluates complexity
- THEN CI MUST emit a warning including the file path and MUST exit zero

### Requirement: Debt Report

`verify-report.md` MUST contain a `Complexity Debt` section listing per-package totals (scanned, violations by threshold) and top 10 offenders per package sorted by max complexity descending. `*_test.go` findings MUST be informational only.

#### Scenario: Report with violations

- GIVEN CI reports 12 cyclomatic and 5 cognitive violations in `internal/review`
- WHEN `verify-report.md` is generated
- THEN debt section MUST show counts and top 10 offenders with file:line, function, cyclomatic, cognitive

#### Scenario: No violations

- GIVEN no functions exceed thresholds
- WHEN `verify-report.md` is generated
- THEN the debt section MUST state `0 violations` and list totals

### Requirement: Tool Pinning and Version Parity

`gocyclo` and `gocognit` versions MUST be pinned in `go.mod` (or `tools.go`/`tool` directive). CI MUST invoke the same pinned versions as local `go run`. Drift MUST emit a CI warning with expected vs actual.

#### Scenario: Pinned versions used

- GIVEN `go.mod` pins `gocyclo v0.1.0` and `gocognit v1.2.0`
- WHEN CI `complexity` job runs
- THEN it MUST invoke those exact versions

#### Scenario: Version drift detected

- GIVEN CI resolves a different tool version than `go.mod`
- WHEN CI runs
- THEN output MUST contain a warning with expected vs actual versions
