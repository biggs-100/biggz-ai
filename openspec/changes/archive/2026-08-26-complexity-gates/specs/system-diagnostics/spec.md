# Delta for system-diagnostics

## ADDED Requirements

### Requirement: ComplexityCheck

The system MUST provide `ComplexityCheck` in `internal/doctor/complexity.go` with `ID=complexity`. The check MUST be read-only (`CostQuick` equivalent, no writes), panic-isolated via Runner `recover`, and scoped to critical packages (`internal/review`, `internal/sdd`, `internal/verification`). It MUST compute cyclomatic (gocyclo) and cognitive (gocognit) per function, emit `Status=warn` (WARNING bucket) when any function exceeds thresholds (>15 cyclomatic, >20 cognitive), and `Status=pass` otherwise. Human output MUST render a table of top offenders plus totals; `--json` MUST emit a machine list with `{package, file, line, function, cyclomatic, cognitive}`. The check MUST exclude `*_test.go` from WARNING promotion (test violations are informational only) and MUST respect grandfather semantics in its messaging (message MUST distinguish actionable vs legacy totals). Panic or timeout MUST yield `Status=warn` with the failure reason, not abort other checks.

#### Scenario: Violations produce WARNING table

- GIVEN `internal/review/foo.go:FuncA` has cyclomatic 18 and cognitive 10
- WHEN `ComplexityCheck.Run()` executes
- THEN `Result.Status` MUST be `warn` with WARNING severity
- AND message MUST contain a table row for `FuncA` and totals `1 cyclomatic violation`

#### Scenario: No violations yields pass

- GIVEN all functions in critical packages are within thresholds
- WHEN `ComplexityCheck.Run()` executes
- THEN `Result.Status` MUST be `pass` with INFO severity
- AND message MUST contain `0 violations` and total functions scanned

#### Scenario: Test file violation is informational only

- GIVEN only `internal/review/foo_test.go:TestFoo` exceeds thresholds
- WHEN `ComplexityCheck.Run()` executes
- THEN `Result.Status` MUST be `pass` (or `warn` only if non-test violations exist)
- AND test offender MUST appear under `informational` not in blocking WARNING count

#### Scenario: JSON output is machine-parsable

- GIVEN `biggz doctor --json` with violations
- WHEN Report is marshaled
- THEN JSON MUST contain `complexity` entry with `status` and `details.offenders[]` containing function records

#### Scenario: Panic isolation

- GIVEN Runner includes `ComplexityCheck` that panics
- WHEN `Runner.Run()` completes
- THEN `ComplexityCheck` result MUST have `Status=warn` or `fail` with panic message
- AND other check results MUST be present unaffected

#### Scenario: Timeout or scan error degrades gracefully

- GIVEN scan exceeds internal timeout or `gocognit` invocation fails
- WHEN `ComplexityCheck.Run()` completes
- THEN `Result.Status` MUST be `warn` with message containing the reason
- AND `Result` MUST NOT have `Status=fail` with CRITICAL severity
