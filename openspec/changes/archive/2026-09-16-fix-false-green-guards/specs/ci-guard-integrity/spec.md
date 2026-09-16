# CI Guard Integrity Specification

## Purpose

Invariants that keep the git-spawn CI guard from reporting green while checking nothing. Covers the compiled in-repo checker — whose absence, build failure, or vacuous scan is RED — and the positive control proving the scanner actually scanned its targets.

## Requirements

### Requirement: Compiled Guard Checker

CI enforcement of source invariants MUST use a compiled in-repo Go checker that the workflow builds from the repository, and MUST NOT depend on a scanner binary that no workflow step provisions. The checker MUST flag a literal `"git"` in command position — including wrapped and indirected spawns that never write `exec.Command` on the offending line — and MUST report every violation as `file:line`. A non-allowlisted match MUST exit non-zero. A checker that fails to build MUST fail the step; the step MUST NOT be warn-only, skippable, or replaceable by an absence check (`… 2>/dev/null | grep -q .`).

#### Scenario: Direct spawn detected

- GIVEN a Go fixture outside `internal/git` containing `exec.Command("git","status")`
- WHEN the checker scans the fixture set
- THEN it MUST exit non-zero and report the site as `file:line`

#### Scenario: Indirected spawn detected

- GIVEN a fixture passing `"git"` to a helper such as `runCmd`/`execFn`, with no `exec.Command` on that line
- WHEN the checker scans it
- THEN it MUST exit non-zero and name the site

#### Scenario: Unbuildable checker fails the step

- GIVEN the checker fails to compile
- WHEN the CI guard step runs
- THEN the step MUST exit non-zero and MUST NOT print a pass verdict

### Requirement: Scanner Positive Control

The guard MUST carry a checked-in fixture set with a known violation count, and the same checker code path CI runs MUST assert that exactly that count is reported. Zero findings MUST NOT count as a clean tree unless the expected-count assertion passed in the same run, and the guard MUST fail when its scan resolves zero target files.

#### Scenario: Expected set asserted

- GIVEN the fixture set contains exactly N known violations
- WHEN the checker scans it
- THEN it MUST report exactly N sites and the self-test MUST pass

#### Scenario: Dead scanner rejected

- GIVEN the checker scans the fixture root but reports zero findings
- WHEN the self-test runs
- THEN it MUST fail and block merge
