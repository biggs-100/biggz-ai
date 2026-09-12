# Delta for sdd-status

## ADDED Requirements

### Requirement: Outstanding Review Obligation Projection

When RDD is enabled and the verified candidate has no valid receipt, status MUST surface the outstanding review obligation pre-publication via `nextRecommended` and/or `blockedReasons`, including the actionable producer command that satisfies it. When a valid receipt exists or RDD is disabled, the obligation MUST NOT be surfaced.

#### Scenario: Missing receipt surfaces obligation

- GIVEN `RDD enabled`, `verifyReport` PASS, and no receipt for the change
- WHEN `biggz sdd-status --json` derives
- THEN `nextRecommended`/`blockedReasons` MUST surface the obligation including the actionable producer command

#### Scenario: Valid receipt clears obligation

- GIVEN `RDD enabled` and a valid receipt bound to the verified candidate
- WHEN status derives
- THEN the obligation MUST NOT appear and routing MUST proceed to the ready phase

#### Scenario: Disabled RDD omits obligation

- GIVEN `RDD disabled`
- WHEN status derives
- THEN no review obligation MUST be surfaced
