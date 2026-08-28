# Delta for sdd-status

## MODIFIED Requirements

### Requirement: SDD Status v2 Sole Contract

The system MUST expose `biggz-ai.sdd-status/v2` (`internal/sdd/status_v2.go`) projecting only authority-free keys: `schemaName,artifactStore,planningHome,changeRoot,artifactPaths,contextFiles,artifacts,taskProgress,dependencies,applyState,actionContext,relationships,remediationState,reviewOffer,consent,nextRecommended,blockedReasons`. `ProjectStatusV2` MUST NOT emit `granted_roots`, `edit_authority_blocked`, `missing_roots` nor call `applyEditAuthorityBlock` (`internal/sdd/edit_authority.go` → pre-apply warning only).
(Previously: `ProjectStatusV2` called `applyEditAuthorityBlock` and emitted `granted_roots`)

#### Scenario: Projection authority-free

- GIVEN `ChangeStatus{GrantedRoots:[/other],EditAuthorityBlocked:true}`
- WHEN `ProjectStatusV2` called
- THEN JSON MUST NOT contain `granted_roots`/`edit_authority_blocked`/`missing_roots`

#### Scenario: Pre-apply warning replaces block

- GIVEN `tasks.md` with backticked `../other/repo/file.go`
- WHEN `sdd-status --json`
- THEN `blockedReasons` MUST be empty and `nextRecommended` NOT `resolve-blockers`
- AND `sdd-apply` MUST warn `blocked(edit_authority_missing)` with both exits

#### Scenario: V1 still refused

- GIVEN `--contract biggz-ai.sdd-status/v1`
- WHEN parsing
- THEN MUST fail `unsupported sdd-status contract` with rerun `biggz-ai.sdd-status/v2`
