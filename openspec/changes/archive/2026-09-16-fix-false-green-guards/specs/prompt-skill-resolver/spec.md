# Delta for prompt-skill-resolver

## MODIFIED Requirements

### Requirement: CI No-fmtSprintf Guard

CI MUST fail when a `fmt.Sprintf` call in `internal/review/lens` builds a prompt. The guard MUST be a compiled check executed by CI (the Go guard test in `internal/review/lens`), MUST resolve its target files from the repository root rather than the package working directory, and MUST fail — not skip — when a target file cannot be read or when the scan resolves zero files. Guard MUST target only `internal/review/lens` and allow `//lint:ignore no-fmtSprintf` for non-prompt uses; non-prompt uses MUST carry that marker instead of narrowing the guard, whose scope stays package-wide. Step MUST be required for merge and MUST NOT depend on a binary no workflow step provisions.
(Previously: the requirement prescribed `rg 'fmt\.Sprintf' internal/review/lens`, absent on the runner, while a Go replica passed silently through repo-relative paths read from the package working directory and an unreadable-file `continue`.)

#### Scenario: CI fails on fmt.Sprintf in lens

- GIVEN a lens file builds a prompt with an unmarked `fmt.Sprintf`
- WHEN the guard runs
- THEN the job MUST fail reporting file:line

#### Scenario: CI passes when clean

- GIVEN `internal/review/lens` contains no unmarked `fmt.Sprintf`
- WHEN the guard runs
- THEN the job MUST pass

#### Scenario: Allowlisted exception permitted

- GIVEN `fmt.Sprintf` carrying `//lint:ignore no-fmtSprintf`
- WHEN the guard runs
- THEN that line MUST not fail

#### Scenario: Reads resolve from repo root

- GIVEN the guard executes via `go test ./internal/review/lens` (working directory = package dir)
- WHEN it reads `internal/review/lens/readability/lens.go`
- THEN the read MUST succeed and every `*.go` file in the package MUST be scanned

#### Scenario: Unreadable file fails the guard

- GIVEN a target file cannot be read
- WHEN the guard runs
- THEN it MUST fail naming that path and MUST NOT skip it via `continue`

#### Scenario: Package-wide scope cannot be narrowed

- GIVEN `readability/lens.go` contains an unmarked `fmt.Sprintf` for a non-prompt finding ID
- WHEN the guard runs
- THEN it MUST fail for that line (the guard MUST NOT exclude that file or narrow its scope)
