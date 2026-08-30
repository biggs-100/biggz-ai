# Orchestrator Specification

## Purpose

The orchestrator is the coordinator that gates SDD progression on explicit user intent. Investigative and conditional phrases are read-only; pre-proposal requires confirmed product decisions, valid evidence references, matching hybrid state, and completion of selected research.

## Requirements

### Requirement: Explicit Intent Required

The orchestrator MUST require explicit user intent before editing, applying, or continuing SDD work. Investigative phrases (`investigate`, `explore`, `check`, `look into`) and conditional phrases (`if possible`, `maybe`, `consider`, `when ready`) MUST NOT be treated as permission. Pre-proposal gate MUST have confirmed product decisions, valid evidence references, and matching hybrid state; selected research `done` MUST block `propose` until satisfied. The orchestrator MUST offer `sdd-research` after `sdd-explore` and treat selection as mandatory.

#### Scenario: Explicit intent permits apply

- GIVEN user says `apply the fix to internal/sdd/status.go`
- WHEN orchestrator evaluates intent
- THEN it MUST treat as explicit permission and may launch `sdd-apply`

#### Scenario: Investigate does not grant permission

- GIVEN user says `investigate the status bug`
- WHEN orchestrator evaluates intent
- THEN it MUST NOT treat as permission to edit files
- AND it MUST limit to read-only exploration

#### Scenario: Conditional does not grant permission

- GIVEN user says `fix it if possible` or `consider updating the task`
- WHEN orchestrator evaluates intent
- THEN it MUST NOT auto-apply edits and MUST ask for explicit confirmation

#### Scenario: Research blocks propose until done

- GIVEN `sdd-research` was selected and status is `partial`
- WHEN orchestrator attempts `sdd-propose`
- THEN it MUST block and report `blockedReasons` with research incompleteness
- AND MUST NOT invoke proposer

#### Scenario: Unselected research bypasses gate

- GIVEN `sdd-research` was not selected
- WHEN orchestrator evaluates pre-proposal gate
- THEN `propose` MUST be allowed when decisions are `confirmed` and references valid

### Requirement: Post-Delegation Human Checkpoint Synthesis

The system MUST emit chat FIRST same-turn BEFORE checkpoint `ask`. MUST render `**What was done:**` as `| Topic | Decision |` table plus checklist (no prose paragraph) and one-line lifecycle `◆ Phase · Status · Next` with warning/success/error color and dim detail. Preview/Diff and optional blocks MUST use sanitized truncation via `stripAnsi/stripOsc/CONTROL_CHAR` + `TruncateToWidth` before `VisibleWidth` measure (`internal/tui/sanitize.go`). MUST keep 4 mandatory markers (`## Sub-agent Result`, `**What was done:**`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**`) and SHOULD include 6 optional blocks `Preview, Diff, Decisions, Commands, Validation, Failure` (empty omitted). On failure MUST render humanized summary not raw JSON. On truncation >50KB MUST loop re-reads and verify length. Missing MUST be `isError:true`; param-only MUST NOT count. `hasSynthesis` MUST pass when table present.

(Previously: prose What was done only; no table/checklist/lifecycle/sanitized truncation)

#### Scenario: Table replaces prose
- GIVEN `WhatDone` with 3 topics/decisions
- WHEN `RenderSynthesis` is called
- THEN output MUST contain `| Topic | Decision |` header and 3 rows
- AND MUST NOT contain a prose paragraph for What was done

#### Scenario: Checklist rendered
- GIVEN tasks with completed/pending items
- WHEN synthesized
- THEN output MUST contain `- [x]` / `- [ ]` checklist after table

#### Scenario: One-line lifecycle with color
- GIVEN phase `spec`, status `success`, next `design`
- WHEN lifecycle line rendered
- THEN it MUST be single line `◆ spec · success · design` with success color and dim detail
- AND warning MUST be yellow, error MUST be red

#### Scenario: Full passes with table
- GIVEN summary, artifacts ≥2 ≥50 chars, risks, next
- WHEN 4 markers plus table then checkpoint ask same turn
- THEN it MUST allow

#### Scenario: Missing blocked
- GIVEN no `## Sub-agent Result` in current turn
- WHEN checkpoint ask
- THEN it MUST be `isError:true`

#### Scenario: Failure and truncated handled
- GIVEN `blocked` failure JSON and artifact >50KB truncated
- WHEN synthesized
- THEN it MUST show human Failure summary and loop re-read to full length

### Requirement: Orchestrator Synthesis Template Invariant

`internal/assets/biggz/biggz-orchestrator.md` MUST keep 4-marker example + `| Topic | Decision |` table + checklist + `◆ Phase · Status · Next` one-line lifecycle placeholders + `INVALID and will be blocked` rule; drift MUST fail `orchestrator.test.go`. `engram` alias MUST equal `bigmem`.

(Previously: 4 markers + INVALID only; no table/lifecycle markers)

#### Scenario: Template holds new markers
- GIVEN file read
- WHEN searching
- THEN it MUST contain `## Sub-agent Result`, `| Topic | Decision |`, `- [ ]`, `◆`, and `INVALID`

#### Scenario: Alias invariant preserved
- GIVEN config with `engram`
- WHEN normalized
- THEN it MUST equal `bigmem` and test MUST enforce

### Requirement: Single Ownership and Pending Persistence

Only orchestrator SHALL emit checkpoint asks; sub-agents/Pi MUST NOT. MUST persist envelope to `biggz-ai.pending-question/v1` via dual-write BigMem + `state.yaml` with fallback; MUST verify equality (retry once). On compaction MUST reload and emit fallback if UI unavailable.

#### Scenario: Ownership enforced

- GIVEN sub-agent tries checkpoint ask
- WHEN calling `ask_user_question`
- THEN MUST be blocked

#### Scenario: Dual-write and fallback

- GIVEN pending persisted then compacted
- WHEN readback and resumed
- THEN stores MUST have identical bytes and MUST re-emit full envelope

### Requirement: Task-Scoped Repository-Relative Path Validation

The system MUST implement `internal/orchestrator/surfaces.go` `isTaskScopedRepositoryRelativePath(value string) bool` that normalizes `\` to `/`, rejects empty, absolute (`/` or `C:`), `~`-prefixed, whitespace-containing, or `..`-segment paths, strips leading `./` segments, and rejects if the first path segment contains glob characters `*?[]{}`. Only narrow repository-relative paths pass.

#### Scenario: Rejects parent traversal

- GIVEN value `../x`
- WHEN `isTaskScopedRepositoryRelativePath` is called
- THEN it MUST return `false`

#### Scenario: Rejects absolute and home paths

- GIVEN value `/etc/passwd` or `~/x`
- WHEN validated
- THEN it MUST return `false`

#### Scenario: Rejects glob in first segment

- GIVEN value `*.go` or `a[0]/b` or `a{b}/c`
- WHEN validated
- THEN it MUST return `false`

#### Scenario: Rejects whitespace paths

- GIVEN value `a b/c`
- WHEN validated
- THEN it MUST return `false`

#### Scenario: Accepts scoped and dot-normalized paths

- GIVEN value `src/pkg/file.go` or `./src/file.go`
- WHEN validated
- THEN it MUST return `true` (after `./` stripped and first-segment check)

### Requirement: Bounded Writer Dispatch Surface Guard

The system MUST implement `internal/sdd/status.go` `ShouldEnforceScopedSurfaces(fileCount int) bool` returning `fileCount >= 4` (3 allow, 4 enforce) and `ValidateBoundedWriterSurfaces` / `internal/orchestrator/surfaces.go` `rejectUnscopedBoundedWriterDispatch` that when `fileCount >= 4` checks each allowed surface via `isTaskScopedRepositoryRelativePath` and, on any unscoped entry, returns `ScopedSurfaceRejection{Block:true, Reason:WRITER_EDIT_SURFACE_REJECTION}` instructing parent to derive narrow surfaces and relaunch.

#### Scenario: FileCount 3 allows without per-path check

- GIVEN `fileCount` is `3` and surfaces contain `../x`
- WHEN `ValidateBoundedWriterSurfaces` / `rejectUnscopedBoundedWriterDispatch` is called
- THEN it MUST allow (return nil) without per-path enforcement

#### Scenario: FileCount 4 enforces per-path and rejects unscoped

- GIVEN `fileCount` is `4` and one surface is `../x` or `*.go`
- WHEN guard is called
- THEN it MUST return a rejection with `WRITER_EDIT_SURFACE_REJECTION` and `Block true`

#### Scenario: FileCount 4 passes when all surfaces scoped

- GIVEN `fileCount` is `4` and surfaces are `["src/a.go", "internal/b.go"]`
- WHEN guard is called
- THEN it MUST return nil (allow)

### Requirement: Sealed Explorer Scout Fallback

The system MUST implement sealed-explorer semantics via `internal/orchestrator/surfaces.go` (`isTaskScopedRepositoryRelativePath`, `readAllowedEditSurfaceEntries`/`hasTaskScopedAllowedEditSurfaces`, `rejectUnscopedBoundedWriterDispatch`) and `internal/sdd/status.go` `ShouldEnforceScopedSurfaces`/`ValidateBoundedWriterSurfaces`: when a bounded writer agent (`worker`|`gentle-ai-worker`) is dispatched and surfaces are absent, unscoped, or fileCount enforces but no valid `## Allowed edit surfaces` block exists, the parent MUST NOT launch the writer as bounded editor; it MUST relaunch as `scout` read-only (no `write`/`edit` tool calls, read-only exploration, log fallback, no human block/prompt). With at least one valid scoped surface and `fileCount>=4` passing, writer MUST proceed.

#### Scenario: Writer without surfaces becomes scout read-only

- GIVEN `input{agent:worker, task:"explore repo", context:""}` with no `## Allowed edit surfaces` heading
- WHEN `rejectUnscopedBoundedWriterDispatch`/`ValidateBoundedWriterSurfaces` (fileCount 4) is called
- THEN it MUST return `Block` with `WRITER_EDIT_SURFACE_REJECTION` and caller MUST relaunch as scout with only read tools; no file write MUST occur

#### Scenario: Writer with valid surfaces passes

- GIVEN `task:"## Allowed edit surfaces\n- internal/orchestrator/surfaces.go\n- docs/guide.md\n"`
- WHEN guard is called
- THEN it MUST return `nil` and writer MAY use `write`/`edit` limited to those surfaces

#### Scenario: Scout fallback is logged without human block

- GIVEN scout relaunch after rejection
- WHEN fallback occurs
- THEN system MUST log `scout_fallback` with reason `WRITER_EDIT_SURFACE_REJECTION` and MUST NOT emit `ask_user_question`/human notification

#### Scenario: Non-writer agent never becomes scout

- GIVEN `input{agent:researcher, task:"no surfaces"}`
- WHEN guard is called
- THEN it MUST return `nil` regardless of surfaces

### Requirement: Task-Scoped Surface Validation and Surface Consistency

The system MUST implement `isTaskScopedRepositoryRelativePath` (normalize `\→/`, reject empty/absolute `C:` or `/` or `~`, reject whitespace/`\t`/`\n`, strip leading `./+`, reject any `..` segment, reject if first segment contains `*?[]{}`) and `hasTaskScopedAllowedEditSurfaces`/`readAllowedEditSurfaceEntries` that extracts the block after `## Allowed edit surfaces` (case-insensitive, up to next `#{1,2}` heading), parses bullet/numbered entries (strip `` ` ``), requires ≥1 entry per heading, validates each via `isTaskScoped...`, dedups/sorts and requires all headings to agree on identical set; `ShouldEnforceScopedSurfaces(fileCount)` MUST return `fileCount>=4` (`3→false`, `4→true`).

#### Scenario: Rejects traversal, absolute, glob first-segment, whitespace

- GIVEN `../x`, `/etc/passwd`, `~/x`, `*.go`, `a[0]/b`, `a b/c`, `""`, `.`
- WHEN `isTaskScopedRepositoryRelativePath` is called
- THEN it MUST return `false`

#### Scenario: Accepts dot-normalized and deep glob

- GIVEN `./src/file.go`, `internal/orchestrator/surfaces.go`, `internal/foo*.go` (glob in second segment)
- WHEN validated
- THEN it MUST return `true`

#### Scenario: Heading parsing requires valid scoped entries

- GIVEN `good="## Allowed edit surfaces\n- \`internal/a.go\`\n"` versus `bad="## Allowed edit surfaces\n- \`../x\`\n"` versus `missing="no heading"`
- WHEN `hasTaskScopedAllowedEditSurfaces` is called
- THEN `good` MUST return `true`; `bad` and `missing` MUST return `false`

#### Scenario: FileCount threshold 3 allows, 4 enforces

- GIVEN `fileCount=3` with `../x` versus `fileCount=4` with same
- WHEN `ValidateBoundedWriterSurfaces` is called
- THEN `3` MUST return `nil`; `4` MUST return `Block` with `WRITER_EDIT_SURFACE_REJECTION`

### Requirement: Sealed Orchestration Logging

The system MUST log sealed decisions (path validation failures, scout fallback) with `agent`, `fileCount`, and offending surface where applicable, at debug/info level without blocking human flow.

#### Scenario: Invalid surface logs offending entry

- GIVEN writer surfaces `["../x"]` at `fileCount=4`
- WHEN rejected
- THEN log MUST contain the offending surface `../x` and `Block true`

### Requirement: Synthesis Sanitized Truncation and Chunking

The system MUST sanitize `Preview` and `Diff` via `stripAnsi`/`stripOsc`/`CONTROL_CHAR` removal then `TruncateToWidth` before `VisibleWidth` measure (`internal/tui/sanitize.go` + `x/ansi`/`go-runewidth`). `Preview` MUST be 300 chars sanitized with `…`; `Diff` MUST be `N files ±` summary sanitized. Tables MUST chunk at <7 rows per chunk with per-cell `TruncateToWidth` for narrow mux (CJK width 2, ANSI width 0). Coverage MUST apply to chat synthesis, `sdd-status` 4 blocks, and docs (`proposal/spec/design/tasks/verify-report`) in `Outcome + Quick path + Details` shape.

#### Scenario: Preview sanitized 300c
- GIVEN artifact with ANSI + OSC + 500 chars + CJK
- WHEN Preview built
- THEN it MUST strip ANSI/OSC/controls, truncate to 300 visible width with `…`, and `VisibleWidth ≤300`

#### Scenario: Diff sanitized and chunked
- GIVEN 10 topics with ANSI and CJK
- WHEN table rendered on 40-col width
- THEN it MUST chunk into ≥2 tables of ≤7 rows each and each cell `VisibleWidth` ≤ column budget with `…`

#### Scenario: Doc coverage shape
- GIVEN `proposal/spec/design/tasks/verify-report` rendered
- WHEN inspected
- THEN each MUST start with Outcome, then Quick path steps, then Details table
