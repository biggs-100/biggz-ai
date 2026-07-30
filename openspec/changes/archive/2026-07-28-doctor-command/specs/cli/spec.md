# Delta for CLI

## ADDED Requirements

### Requirement: Doctor Subcommand

The CLI MUST add a "doctor" subcommand dispatched via the existing switch-based router. The subcommand MUST NOT read or parse a ReviewSubject from stdin — it operates independently of the review pipeline.

#### Scenario: Doctor dispatch

- GIVEN the CLI is invoked as `biggz doctor`
- WHEN the router matches the "doctor" command
- THEN doctorRun() MUST be invoked
- AND no stdin review parsing MUST occur

### Requirement: --json Flag

The doctor subcommand MUST parse a --json flag. When present, output MUST be valid JSON marshaled from the Report struct, parsable by standard JSON tools.

#### Scenario: JSON output

- GIVEN `biggz doctor --json`
- WHEN all checks complete
- THEN stdout MUST contain valid JSON with all check Results and severity buckets
- AND the exit code MUST be 0 (unless a check framework error occurs)

#### Scenario: JSON with --fix

- GIVEN `biggz doctor --fix --json`
- WHEN remedies execute and checks re-run
- THEN remedies MUST execute before JSON serialization
- AND the JSON output MUST reflect the post-fix state

### Requirement: --fix Flag

The doctor subcommand MUST parse a --fix flag. When present, the Runner MUST iterate declared remedies and execute their Actions after all initial checks complete. If no remedies are declared, zero actions execute and the output MUST indicate this.

#### Scenario: Remediation applied

- GIVEN `biggz doctor --fix` and at least one check declares a Remedy
- WHEN checks complete
- THEN each remedy Action MUST execute
- AND the output MUST include post-remedy status per check

#### Scenario: No remedies declared

- GIVEN `biggz doctor --fix` and no check declares a Remedy
- WHEN checks complete
- THEN zero actions MUST execute
- AND the output MUST indicate "0 remedies applied"

### Requirement: Default Renderer

The doctor subcommand MUST render results in human-readable format by default using tabla humana, grouped by severity bucket. Each check row MUST display a status icon: [ok] for pass, [!!] for warn, [xx] for fail. A summary line MUST show total counts per severity.

#### Scenario: Default table output

- GIVEN `biggz doctor` (no flags)
- WHEN all checks complete
- THEN stdout MUST contain severity-grouped sections
- AND each row MUST show check ID, status icon, and message
- AND a footer MUST show total pass/warn/fail counts
