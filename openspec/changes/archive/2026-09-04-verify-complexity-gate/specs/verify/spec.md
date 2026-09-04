# Delta for verify

## MODIFIED Requirements

### Requirement: Diff-Aware Complexity Gate at Verify

The system MUST measure cyclomatic (>15) and cognitive (>20) complexity only on functions intersecting the change diff and block the `verify` phase when a NEW offender appears in a critical package (`internal/review`, `internal/sdd`, `internal/verification`). Pre-existing violations outside the diff MUST NOT block (structural grandfathering). Non-critical packages and `_test.go` MUST surface as warnings only. When the workspace is not a git repo, the gate MUST skip with a reason instead of failing.
(Previously: `CollectComplexityDebt` whole-tree informational, called only from tests; enforcement only via R2 lens at review and CI at PR.)

#### Scenario: New offender in critical package blocks
- GIVEN a diff touching `internal/sdd/foo.go` with a function at cyclomatic 22
- WHEN the verify complexity gate runs
- THEN verdict MUST be blocked naming `file:function cyclo=22`

#### Scenario: Pre-existing violation outside diff passes
- GIVEN `internal/sdd/status.go` offender at lines untouched by the diff
- WHEN the verify complexity gate runs on that diff
- THEN verdict MUST pass (grandfathered)

#### Scenario: Non-critical package passes silent
- GIVEN a diff touching `internal/foo/bar.go` with a function at cognitive 30
- WHEN the verify complexity gate runs
- THEN verdict MUST pass (only critical packages block; R2 lens still reports at review)

#### Scenario: Non-git workspace skips
- GIVEN a workspace without `.git`
- WHEN the gatekeeper runs the verify complexity check
- THEN the check MUST be skipped with reason `not a git repo`

#### Scenario: Gatekeeper surfaces via CLI
- GIVEN `sdd-gatekeeper <change>` on a verify result in a repo with a new critical offender
- WHEN the gatekeeper validates
- THEN `details` MUST contain a failing `complexity_gate` check and `passed` MUST be false
