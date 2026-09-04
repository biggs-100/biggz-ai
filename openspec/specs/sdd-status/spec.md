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

### Requirement: Four-Block Scannable Rendering

The system MUST render `sdd-status` as 4 blocks in `Outcome + Quick path + Details` shape (cognitive-doc-design: answer first, chunking, signposting) with progressive disclosure. Each block MUST contain Outcome (1-line status), Quick path (numbered next steps 1..n), Details (`| Topic | Decision |` table). The 4 blocks MUST be `Status Overview`, `Artifact Progress`, `Next Action`, `Risks/Blockers` (order fixed, 5s scannable). Banner MUST be adaptive (truncated via `TruncateToWidth`). All text MUST be sanitized before width measure.

#### Scenario: Four blocks present
- GIVEN `sdd-status` with artifacts and nextRecommended
- WHEN rendered in terminal
- THEN output MUST contain 4 headings in Outcome/Quick path/Details order and each block MUST have Outcome line + numbered Quick path + Details table

#### Scenario: Quick path actionable
- GIVEN `nextRecommended: sync` and verify PASS
- WHEN Quick path of Next Action block read
- THEN it MUST list `1. biggz sdd-sync <change>` and `2. verify` as numbered steps

#### Scenario: Banner adaptive truncation
- GIVEN change name with 100 chars + ANSI on 80-col terminal
- WHEN banner rendered
- THEN it MUST strip ANSI and `TruncateToWidth` to terminal width with `…` and `VisibleWidth ≤ width`

### Requirement: Progressive Disclosure Chunking and Sanitized Truncation

The system MUST provide progressive disclosure (collapsible blocks with `… +N more` hint) and chunking <7 rows per Details table. Content MUST be sanitized via `stripAnsi/stripOsc/CONTROL_CHAR` then `TruncateToWidth` before `VisibleWidth` (`internal/tui/sanitize.go`). Empty blocks MUST be omitted. Chunk hint MUST show hidden count. Coverage MUST include `sdd-status` blocks and docs in same `Outcome + Quick path + Details` shape.

#### Scenario: Chunking under seven
- GIVEN Details table with 12 rows and terminal width 60
- WHEN rendered
- THEN it MUST split into ≥2 chunks each ≤7 rows with per-cell truncation and show `… +5 more` hint on first chunk

#### Scenario: Collapsed block hint
- GIVEN Risks block with 10 blockers on narrow viewport
- WHEN initially rendered
- THEN it MUST collapse after 7 rows and display hint with hidden count
- AND expansion MUST reveal remaining rows

#### Scenario: Sanitized truncation CJK/ANSI
- GIVEN table cell with `\x1b[31m` ANSI + OSC + `中` CJK + 100 chars
- WHEN `TruncateToWidth` to 20 applied before measure
- THEN `VisibleWidth` MUST be ≤20, CJK counted as 2, ANSI as 0, no split wide rune, ends with `…`

#### Scenario: Empty omitted
- GIVEN `BlockedReasons` empty and no risks
- WHEN `sdd-status` rendered
- THEN Risks/Blockers block MUST be omitted or show `None` without empty table

### Requirement: BigMem Status via Store Ctx API

The system MUST derive BigMem status via Store `*Ctx` methods (`SearchCtx` or equivalent) and MUST NOT use raw `sql.Open` or non-Ctx `db.Query` in `internal/sdd/engram_status.go`.

#### Scenario: Store-sourced collection

- GIVEN BigMem holds `sdd/` observations
- WHEN `collectBigMemChangesWithArchive` runs
- THEN results MUST come from Store `*Ctx` API and grep MUST find no `sql.Open`/`db.Query` in `engram_status.go`

#### Scenario: Absent DB falls back explicitly

- GIVEN no `bigmem.db` file exists
- WHEN collection runs
- THEN it MUST fall back to filesystem-only with an explicit logged warning

### Requirement: SQL-Side Visibility Filtering

The system MUST filter `project`, `scope`, `deleted_at IS NULL`, and `topic_key LIKE 'sdd/%'` in SQL, not by loading full content and filtering in Go.

#### Scenario: Predicates in SQL

- GIVEN mixed-project and deleted observations
- WHEN the Store query executes
- THEN SQL MUST contain project/scope/`deleted_at`/`topic_key` predicates and excluded rows MUST never hydrate `content`

### Requirement: Minimal Hydration

The system MUST fetch `content` only for rows surviving visibility filters; key-only selection MUST precede hydration.

#### Scenario: Visible-only hydration

- GIVEN 100 rows with 2 visible changes
- WHEN status derives
- THEN `content` MUST load only for the 2 survivors and artifact states MUST match full-hydration results

### Requirement: Caller Context With Timeout

Status hot spots MUST propagate caller `ctx` with timeout and MUST NOT use `context.Background` in `status.go` derivation (`IsSessionSummaryBlocked` sites) or the BigMem collector.

#### Scenario: Cancellation fails fast

- GIVEN a cancelled caller ctx
- WHEN `sdd-status` runs
- THEN it MUST return promptly with a wrapped `context.Canceled`/`DeadlineExceeded` error

#### Scenario: No Background at hot spots

- GIVEN `status.go` derivation code
- WHEN inspected
- THEN no `context.Background` MUST remain at the session-guard call sites

### Requirement: Visible BigMem Failures

The system MUST log and wrap DB errors with operation context and MUST NOT return silent `(nil,nil,nil)`; degraded filesystem-only mode is allowed ONLY with an explicit logged warning.

#### Scenario: Query error surfaces

- GIVEN a corrupt/unreadable BigMem DB
- WHEN collection queries
- THEN it MUST return/log a wrapped error naming the operation and MUST NOT return `(nil,nil,nil)` silently

### Requirement: Project Visibility Parity

The system MUST preserve parity: exclude `scope=personal`, match inferred project case-insensitively, and disable the project filter only when the test-store override is set.

#### Scenario: Personal excluded

- GIVEN one `personal` and one project observation for the inferred project
- WHEN status derives
- THEN output MUST include only the project observation

#### Scenario: Project match and override

- GIVEN an observation with non-matching project
- WHEN status derives in production
- THEN it MUST be excluded; AND with test-store override set it MUST be visible
