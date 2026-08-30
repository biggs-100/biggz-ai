# SDD Status Specification

## Purpose

SDD Status projects `biggz-ai.sdd-status/v2` for artifact routing, hybrid locator, and progress dependencies.

## Requirements

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

### Requirement: Declared Artifact Store and Hybrid Locator

The system MUST resolve the declared artifact store by reading `openspec/config.yaml` key `artifact_store` (normalized: `hybrid` and `engram`/`bigmem` alias to hybrid reading; missing or unreadable file defaults to `openspec`; `none` disables planning I/O). `resolveArtifactPaths`/`declaredArtifactStore` MUST branch per store: `openspec` returns filesystem `openspec/changes/{change}/…` paths; `engram`/`bigmem` returns `bigmem:sdd/{change}/…` paths; `hybrid` merges both stores with filesystem-wins on name collision; `none` returns empty paths. `artifactPaths` and `contextFiles` projected by `ProjectStatusV2` / `StatusWithOptions` MUST reflect the resolved store.

#### Scenario: Reads declared store from config

- GIVEN `openspec/config.yaml` contains `artifact_store: hybrid`
- WHEN `declaredArtifactStore(workspaceRoot)` is called
- THEN it MUST return `hybrid` (normalized) and not default to `openspec`

#### Scenario: Defaults to openspec when config absent

- GIVEN no `openspec/config.yaml` exists
- WHEN status derivation runs
- THEN `ArtifactStore` MUST be `openspec` and `artifactPaths` MUST contain filesystem paths

#### Scenario: Hybrid routing filesystem-wins

- GIVEN BigMem and filesystem both contain change `parity-gentle-69-ledger-budget`
- WHEN `collectBigMemChangesWithArchive` merges
- THEN the resulting `ChangeStatus` MUST be the filesystem entry and the BigMem duplicate MUST be discarded

#### Scenario: artifactPaths per store

- GIVEN store is `engram`
- WHEN `resolveArtifactPaths` projects `proposal`
- THEN it MUST return `bigmem:sdd/{change}/proposal` and not a filesystem path
- AND when store is `openspec` it MUST return `openspec/changes/{change}/proposal.md`

#### Scenario: None store disables planning I/O

- GIVEN `artifact_store: none`
- WHEN status is derived
- THEN `artifactPaths` fields MUST be empty and no filesystem or BigMem read MUST be attempted for planning artifacts

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
