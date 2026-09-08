# ci-e2e Specification

## Purpose

E2E stays blocking except ticket-quarantined CI failures.

## Requirements

### Requirement: Exact-Scope E2E Quarantine

Only CI-failing tests MUST be quarantined, each citing #24 with CI-log scope (first Slice task); the rest MUST stay blocking.

#### Scenario: Trio skips, rest run

- GIVEN the 3 failures quarantined with #24 refs
- WHEN the e2e job runs
- THEN those MUST skip and the rest MUST execute

#### Scenario: Healthy failure blocks

- GIVEN a failure outside quarantine
- WHEN the e2e job runs
- THEN it MUST fail the job

#### Scenario: Over-broad quarantine rejected

- GIVEN a quarantine covering a passing test
- WHEN reviewed
- THEN it MUST be rejected
