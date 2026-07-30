# Review Gates Specification

## Purpose

The Review Gates domain defines publication gates that prevent merging or pushing when review criteria are not met. Gates evaluate review state, receipt validity, and scope changes against policy before allowing PR creation or push.

## Requirements

### Requirement: Pre-PR Gate

The system MUST implement `biggz review gate pre-pr` that blocks PR creation when either: the review has unresolved findings, or the receipt is invalid. The gate MUST exit non-zero when blocked.

#### Scenario: Happy path — gate passes

- GIVEN an Approved review with a valid receipt and zero unresolved findings
- WHEN `biggz review gate pre-pr` runs
- THEN the gate MUST exit zero and report pass

#### Scenario: Gate blocks on unresolved findings

- GIVEN a review with status NeedsChanges and open findings
- WHEN `biggz review gate pre-pr` runs
- THEN the gate MUST exit non-zero and list each blocking finding

#### Scenario: Gate blocks on invalid receipt

- GIVEN a review with a tampered chain (receipt invalid)
- WHEN `biggz review gate pre-pr` runs
- THEN the gate MUST exit non-zero and report receipt validation failure

### Requirement: Pre-Push Gate

The system MUST implement `biggz review gate pre-push` that blocks push when the scope has changed since the last gated state and the change is not acknowledged. The gate MUST exit non-zero when blocked.

#### Scenario: Happy path — no scope change

- GIVEN a review where scope has not changed since last pre-PR gate passed
- WHEN `biggz review gate pre-push` runs
- THEN the gate MUST exit zero and report pass

#### Scenario: Unacknowledged scope change blocks push

- GIVEN a review where scope changed after pre-PR gate passed and change is not acknowledged
- WHEN `biggz review gate pre-push` runs
- THEN the gate MUST exit non-zero and report scope delta

### Requirement: Scope Change Detection

The system MUST detect scope changes by comparing the current scope snapshot hash against the snapshot recorded at the last gate pass. Any difference MUST be reported as a scope delta.

#### Scenario: Scope changed

- GIVEN a lineage with scope snapshot at gate time H1 and current scope H2 != H1
- WHEN scope change is checked
- THEN the system MUST report a scope delta between H1 and H2

### Requirement: Gate Result Reporting

Every gate MUST return a structured result with: pass/fail status, list of blocking reasons (if failed), and dry-run indicator. Human-readable output MUST go to stderr; structured JSON output MUST be optionally available via `--json`.

#### Scenario: Structured output

- GIVEN a pre-PR gate that fails on two findings
- WHEN run with `--json`
- THEN the system MUST print a JSON object with pass=false and a reasons array including both findings

### Requirement: Dry-Run Mode

Every gate MUST support `--dry-run` that evaluates all conditions and reports results without exiting non-zero. Dry-run output MUST clearly indicate it is a dry run.

#### Scenario: Dry-run with failures

- GIVEN a review that would fail pre-PR gate (unresolved findings)
- WHEN `biggz review gate pre-pr --dry-run` runs
- THEN the gate MUST exit zero and include dry-run=true in the result
