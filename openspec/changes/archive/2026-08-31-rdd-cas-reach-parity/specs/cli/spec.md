# Delta for CLI

## ADDED Requirements

### Requirement: RDD CLI expectedRevision and Scope Wiring

The CLI MUST expose `biggz rdd disable --scope=clone|worktree|global --expected-revision=<rev>`, forward `expectedRevision` to `SetCloneLocalRDDMode`, surface `ErrRDDModeRevisionMismatch` without fallback, and print exact enable command per `Source`.

#### Scenario: Disable forwards expectedRevision on mismatch

- GIVEN head rev `head-rev`
- WHEN `biggz rdd disable --scope=clone --expected-revision=stale-rev` runs
- THEN MUST fail with `expected "stale-rev" but head is "head-rev"`

#### Scenario: Status shows Revision and Reach

- GIVEN `biggz rdd status --json` after clone disable
- WHEN queried
- THEN JSON MUST contain `revision` and `reach` (`machine`/`this_build`/unreported)
