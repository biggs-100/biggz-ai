# Delta for sdd-status

## ADDED Requirements

### Requirement: Sync Routing and Guardrail Projection

The system MUST derive `nextRecommended: sync` and `blockedReasons` for sdd-sync guardrails in `internal/sdd/status*.go` and project them via `biggz-ai.sdd-status/v2`. Routing MUST consider store gate, destructive approval, collision without order, RENAMED presence, legacy flat, verify PASS, and actionContext constraints.

#### Scenario: Store gate not-applicable

- GIVEN declared store is `engram` or `none`
- WHEN `Status`/`ProjectStatusV2` derives
- THEN `nextRecommended` MUST NOT be `sync` and `blockedReasons` MUST be empty for sync

#### Scenario: Sync required after verify-pass

- GIVEN store is `openspec`/`hybrid`, `verifyReport` is `PASS`, deltas exist and no guard blocks
- WHEN status derives
- THEN `nextRecommended` MUST be `sync` and `blockedReasons` MUST be empty

#### Scenario: Destructive without approval blocks sync

- GIVEN delta has `REMOVED` or large `MODIFIED` and no explicit prompt approval
- WHEN status derives
- THEN `nextRecommended` MUST be `sync` and `blockedReasons` MUST contain destructive approval hint

#### Scenario: Collision without order blocks sync

- GIVEN two active changes delta the same `openspec/specs/{domain}/spec.md` without order decision
- WHEN status derives for either change
- THEN `blockedReasons` MUST list colliding domain and the other change

#### Scenario: RENAMED and legacy flat block

- GIVEN delta contains `## RENAMED` or main spec is legacy flat
- WHEN status derives
- THEN `nextRecommended` MUST be `sync` with `blockedReasons` containing `RENAMED` or `legacy flat` hint respectively

#### Scenario: Verify not PASS or actionContext violation blocks

- GIVEN `verifyReport` is not `PASS` or `actionContext.mode`/`allowedEditRoots` would be violated
- WHEN status derives
- THEN sync MUST NOT be `ready` and `blockedReasons` MUST describe the violation
