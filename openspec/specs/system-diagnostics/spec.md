# System Diagnostics Specification

## Purpose

The system-diagnostics domain defines a health check framework for biggz-ai installations. It provides typed checks, a panic-isolated runner, severity-categorized reports, and atomic remedies.

## Requirements

### Requirement: Check Framework

MUST define CheckID, Status (pass/warn/fail), Result, Check (ID + Run func), and Runner (ordered checks with Run()). The Runner MUST isolate panics per check via recover — one panic MUST NOT abort remaining checks.

#### Scenario: Panic isolation

- GIVEN a Runner with checks [A, B, C] where B panics
- WHEN Run() completes
- THEN Result[0] and Result[2] have their check outputs
- AND Result[1] MUST have Status=fail with the captured panic message

### Requirement: Report Categorization

MUST produce a Report grouping results into CRITICAL (fail), WARNING (warn), and INFO (pass) buckets. Each bucket MUST be independently iterable.

#### Scenario: Severity groups

- GIVEN a Report with 1 fail, 1 warn, 2 passes
- WHEN accessing severity buckets
- THEN CRITICAL MUST contain 1 result, WARNING 1, INFO 2

### Requirement: Atomic Remedies

Checks MAY declare a Remedy (ID + Description + Action func). The system MUST iterate declared remedies for --fix. Each Action MUST complete atomically and return an error on failure.

#### Scenario: Remedy dispatch

- GIVEN a check with a declared Remedy
- WHEN the remedy Action executes
- THEN it MUST complete atomically
- AND return an error if it cannot be applied

### Requirement: SQLite Integrity Check

MUST verify bigmem SQLite integrity via PRAGMA integrity_check. A failure MUST produce a CRITICAL result.

#### Scenario: Database corruption

- GIVEN a bigmem SQLite file with integrity violations
- WHEN the check runs
- THEN Result MUST have Status=fail with severity CRITICAL

### Requirement: Config Directory Check

MUST verify the biggz-ai config directory and required subdirectories exist. Missing directories MUST produce a CRITICAL result.

#### Scenario: Missing subdirectory

- GIVEN a config directory missing the `plugins` subdirectory
- WHEN the check runs
- THEN Result MUST have Status=fail

### Requirement: MCP Binary Presence

MUST verify biggz-mcp exists at the expected location and is executable. Absence MUST produce a CRITICAL result.

#### Scenario: Binary not found

- GIVEN biggz-mcp is absent from the expected path
- WHEN the check runs
- THEN Result MUST have Status=fail
- AND the message MUST include the expected path

### Requirement: Review Store Chain Integrity

MUST verify the review transaction chain via store.Validate(). A broken chain MUST produce a CRITICAL.

#### Scenario: Missing transaction

- GIVEN a review store with a gap in the transaction chain
- WHEN the check runs
- THEN Result MUST have Status=fail

### Requirement: PATH Shadowing Check

MUST scan PATH for duplicate binaries named biggz or biggz-mcp. Duplicates across directories MUST produce a WARNING result.

#### Scenario: Shadowed binary

- GIVEN PATH contains two directories each with biggz-mcp
- WHEN the check runs
- THEN Result MUST have Status=warn
- AND the message MUST list both paths

### Requirement: Disk Space Check

MUST check available disk space on the biggz data partition. Below 500 MB free MUST produce a WARNING result with remaining space in the message.

#### Scenario: Low disk space

- GIVEN the data partition has 200 MB free
- WHEN the check runs
- THEN Result MUST have Status=warn
- AND the message MUST include "200 MB"

### Requirement: Git Availability

MUST verify git is in PATH and the working directory is a git repository. Git absence MUST produce a CRITICAL result; not a git repo MUST produce a WARNING.

#### Scenario: Git not installed

- GIVEN git is not found in PATH
- WHEN the check runs
- THEN Result MUST have Status=fail
- AND severity MUST be CRITICAL

### Requirement: Version Information

MUST report the installed and latest available biggz-ai version. A mismatch MUST produce an INFO result with both versions.

#### Scenario: Version drift

- GIVEN installed v1.0.0 and latest is v1.1.0
- WHEN the check runs
- THEN Result MUST have Status=pass with INFO severity
- AND the message MUST include "v1.0.0" and "v1.1.0"

### Requirement: Backup State Check

MUST verify the last backup timestamp is within 7 days. A backup older than 7 days MUST produce a WARNING result with age in days.

#### Scenario: Stale backup

- GIVEN the last backup was 10 days ago
- WHEN the check runs
- THEN Result MUST have Status=warn
- AND the message MUST include "10 days"

### Requirement: REQ-DIAG-001 — Pi Web Search Health Check

The system MUST provide `PiWebSearchCheck` that verifies file and env prerequisites only — no live network probe. `ID` MUST be `pi-web-search`. The check MUST inspect `~/.pi/agent/extensions/biggz-web-search.js` existence and env `TAVILY_API_KEY`/`BRAVE_API_KEY`/`BIGGZ_DDG_FALLBACK`/`BIGGZ_WEB_FETCH_HEADLESS`. It MUST return `pass` when file present and at least one provider is configured, `warn` when file present but no provider, and `fail` when file is absent. Severity MUST map via `Report` buckets (fail→CRITICAL, warn→WARNING, pass→INFO). `Remedy` MUST be `biggz install --agent pi`.

#### Scenario: File missing — fail

- GIVEN `~/.pi/agent/extensions/biggz-web-search.js` does not exist
- WHEN `PiWebSearchCheck.Run()` executes
- THEN `Result.Status` MUST be `fail` with CRITICAL severity and message containing the expected path

#### Scenario: File present with Tavily key — pass

- GIVEN the extension file exists and `TAVILY_API_KEY` is set
- WHEN `PiWebSearchCheck.Run()` executes
- THEN `Result.Status` MUST be `pass` with INFO severity

#### Scenario: File present, no keys, DDG fallback off — warn

- GIVEN the extension file exists and no `TAVILY_API_KEY`/`BRAVE_API_KEY`/`BIGGZ_DDG_FALLBACK=1` is set
- WHEN `PiWebSearchCheck.Run()` executes
- THEN `Result.Status` MUST be `warn` with WARNING severity and message hinting `TAVILY_API_KEY` or `BIGGZ_DDG_FALLBACK=1`

#### Scenario: No live probe

- GIVEN network is unavailable but file and env are valid
- WHEN `PiWebSearchCheck.Run()` executes
- THEN it MUST NOT attempt HTTP calls and MUST return based solely on file+env

#### Scenario: Remedy executes atomically

- GIVEN `PiWebSearchCheck` declares `Remedy(ID=pi-web-search, Description=install pi web search)`
- WHEN `doctor --fix` iterates remedies and calls `Action()`
- THEN it MUST invoke `biggz install --agent pi` (via `DeployPiWebSearch`) atomically and return error on failure

#### Scenario: Runner panic isolation

- GIVEN `Runner` includes `PiWebSearchCheck` among other checks and one check panics
- WHEN `Runner.Run()` completes
- THEN `PiWebSearchCheck` result MUST still be present with correct status and other checks MUST be unaffected

#### Scenario: Headless flag visibility

- GIVEN `BIGGZ_WEB_FETCH_HEADLESS=1` is set
- WHEN `PiWebSearchCheck.Run()` executes
- THEN the result message SHOULD note headless tier is enabled (without affecting pass/warn threshold)

### Requirement: REQ-DIAG-002 — Doctor Runner Registration

The system MUST register `PiWebSearchCheck` in `cmd/biggz/cli_doctor_help.go` `doctorRun()` check slice alongside `PiSubagentsCheck` and `PiLastModelCheck`, respecting panic isolation and `--json`/`--fix` flags.

#### Scenario: Doctor lists pi-web-search

- GIVEN `biggz doctor` is invoked
- WHEN checks complete
- THEN output MUST include a row with ID `pi-web-search` and its status icon

#### Scenario: JSON output includes check

- GIVEN `biggz doctor --json` is invoked
- WHEN checks complete
- THEN JSON MUST contain `pi-web-search` entry with status and severity

#### Scenario: --fix invokes remedy then re-checks

- GIVEN `biggz doctor --fix` and `PiWebSearchCheck` is in warn/fail
- WHEN remedies execute
- THEN `PiWebSearchCheck` remedy MUST run before final Report serialization

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

### Requirement: SDD Asset Drift Read-Only Checks

The system MUST add `biggz doctor` RO checks `sddGlobalAssetDriftCount` and `sddLocalAgentOverrideCount` computed via `assets/managed.go:ManagedAssetHash` SHA256 against `managed-assets.json` v1, report `warn: Global SDD asset drift N` when `N>0` with status `warn` (not `fail`), expose no `--fix`, and keep Runner panic-isolated via `diagnostics/doctor.go`.

#### Scenario: Global drift warn
- GIVEN one global `sdd-*.md` hash differs from manifest
- WHEN `biggz doctor` runs
- THEN `sddGlobalAssetDriftCount` MUST be `1` and result MUST be `warn` with message containing `Global SDD asset drift 1`

#### Scenario: Local override warn
- GIVEN local agent override hash differs
- WHEN doctor runs
- THEN `sddLocalAgentOverrideCount` MUST reflect count and status `warn`

#### Scenario: No drift pass and no fix
- GIVEN all hashes match
- WHEN `biggz doctor` and `biggz doctor --json` run
- THEN both counts MUST be `0` with `pass`; CLI MUST NOT accept `--fix`

#### Scenario: Panic isolation
- GIVEN drift check panics
- WHEN `Runner.Run()` completes
- THEN drift result MUST be `warn`/`fail` with panic message and other checks unaffected
