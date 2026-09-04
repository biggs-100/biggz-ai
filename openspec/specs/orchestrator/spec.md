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

### Requirement: CodeGraph Advisory Scope Hint

The orchestrator MUST optionally read `biggz codegraph report` JSON (`{files:[{path,reasons}], graph:{nodes,edges}}`) when present under `openspec/changes/{change}/` to surface an advisory scope hint before spec/design; it MUST NOT auto-scope, auto-edit, or block SDD when the report is absent or stale, and MUST surface hints visibly in pre-spec output.

#### Scenario: Report present surfaces hint

- GIVEN `openspec/changes/{change}/codegraph.md` and JSON exist before spec phase
- WHEN orchestrator prepares scope
- THEN it SHOULD read JSON and surface `files` with reasons in its summary without modifying tasks

#### Scenario: Report absent continues normally

- GIVEN no JSON/Markdown report exists for `<change>`
- WHEN orchestrator evaluates scope
- THEN it MUST continue SDD without error and MUST NOT block `sdd-spec` or `sdd-design`

#### Scenario: Advisory does not auto-apply

- GIVEN report suggests files `[a.go, b.go]`
- WHEN orchestrator displays the hint
- THEN it MUST require explicit human approval before any edit or task scoping

### Requirement: POLISH-ORCH-01 — Synthesis Table Compact Tokens and Fixed Columns

The system MUST render synthesis tables (proposal/spec/design/tasks/verify) with compact tokens and fixed right columns: tokens compact like `4.1k›2.2k` with `›` (hide `window` if `==spent` or `<1k`), 10c right-aligned muted, `elapsed` 5c dim; left cell MUST truncate via `TruncateToWidth` to keep right `visibleWidth` constant 80..120c.

#### Scenario: Synthesis row hides window when equal
- GIVEN synthesis row `window==spent==3000`
- WHEN `internal/sdd/synthesis.go` renders at 100c
- THEN tokens cell MUST be `3k` muted 10c, not `3k›3k` nor `↓`

#### Scenario: Distinct window shows pair with separator
- GIVEN `window=4100, spent=2200`
- WHEN table renders
- THEN cell MUST be `4.1k›2.2k` right-aligned 10c muted with `›`

#### Scenario: Fixed column stability 80→120c
- GIVEN same table at 80c and 120c
- WHEN rendered
- THEN right columns MUST have identical `visibleWidth`, left truncated only

### Requirement: POLISH-ORCH-02 — Wait Headline Data Contract

The system MUST provide wait headline data for `subagent_wait`: when 2-4 runs waiting, headline MUST be 1 line `Wait {elapsed}s · {N} runs ({summaries}) — open Fleet for detail` (e.g., `Wait 23s · 2 runs (sdd-apply running, sdd-verify queued) — open Fleet for detail`) plus optional 1 dim hint line; it MUST NOT dump full `formatAsyncRunList`.

#### Scenario: Headline single line with summaries
- GIVEN 2 runs: `sdd-apply running`, `sdd-verify queued`, elapsed 23s
- WHEN headline generated
- THEN output MUST be `Wait 23s · 2 runs (sdd-apply running, sdd-verify queued) — open Fleet for detail`

#### Scenario: Limits to ≤2 lines
- GIVEN 4 runs waiting
- WHEN headline rendered
- THEN output MUST be ≤2 lines, first solid, second optional dim, never full list dump

### Requirement: REQ-RR3 — Session Recall Gate Hardening

Session Boot Recall MUST use empty-query recency (`biggz_mem_context(5)`/`biggz recall`/`Search("",…)`) for "en que nos quedamos?" plus `git log --oneline -15` and `sdd-status --json` fallback; MUST NOT use FTS.

#### Scenario: Recent wins

- GIVEN `2026-09-01` summary exists
- WHEN gate runs
- THEN synthesis includes `2026-09-01`, not stale `2026-08-27`

#### Scenario: Fallback

- GIVEN BigMem empty
- WHEN gate runs
- THEN `git log --oneline -15` and `sdd-status --json` run, fallback noted

#### Scenario: No FTS for latest

- GIVEN "en que nos quedamos?"
- WHEN resolving latest
- THEN helper used, never `search --query "session"`

### Requirement: REQ-RR4 — Agent Prompt Guardrail

Prompt (`APPEND_SYSTEM.md` via `install.go` + `bigmem-protocol.md`) MUST contain literal string: For recency use `bigmem search --query "" ORDER BY updated_at DESC` or `biggz recall`; never use FTS term search for 'latest'.

#### Scenario: Prompt contains

- GIVEN prompt file read
- WHEN searched
- THEN literal guardrail MUST be present

#### Scenario: Install preserves

- GIVEN `biggz install`
- WHEN `APPEND_SYSTEM.md` regenerated
- THEN guardrail stays inside `<!-- biggz:bigmem-protocol -->`

#### Scenario: TUI visible

- GIVEN TUI help/protocol view
- WHEN rendered
- THEN guardrail visible

### Requirement: REQ-PS1 — Language-Aware Synthesis Content
System MUST render content after markers in last human language; markers/ids stay English.
#### Scenario: Spanish → Spanish
- GIVEN last human `en que nos quedamos?`
- WHEN synthesis renders
- THEN content MUST be Spanish, markers English
#### Scenario: English → English
- GIVEN last human `let's continue`
- WHEN synthesis renders
- THEN content MUST be English
#### Scenario: hi/hello → English
- GIVEN last human `hi`/`hello`
- WHEN detection runs
- THEN MUST treat as English
#### Scenario: Mixed → last turn wins
- GIVEN mixed history, last `ok, continua con el spec`
- WHEN rendering
- THEN MUST be Spanish

### Requirement: REQ-PS2 — Scannable Structure (5 sections)
MUST render 5 sections in human language order: 1 Resumen, 2 Decisiones `| Topic | Decision |`+`◆ Phase · Status · Next`, 3 Evidencia, 4 Artefactos, 5 Riesgos/Próximo. Empty Preview/Diff omitted; >50KB paginate.
#### Scenario: Same structure all phases
- GIVEN phase propose/spec/design/tasks/apply/verify/archive
- WHEN synthesis renders
- THEN MUST contain 5 sections in order
#### Scenario: Empty omitted
- GIVEN Preview/Diff empty
- WHEN RenderSynthesis runs
- THEN MUST omit or show `None`
#### Scenario: >50KB paginated
- GIVEN artifact >50KB
- WHEN previewing
- THEN MUST paginate via ReadLoop, Preview 300 width

### Requirement: REQ-PS3 — Technical Whitelist
Paths, `sdd/...`, code `ORDER BY`/`Search`, branches, IDs MUST stay English.
#### Scenario: Path stays English
- GIVEN human Spanish, artifact `internal/sdd/synthesis.go`
- WHEN listing Artifacts/Paths
- THEN MUST stay `internal/sdd/synthesis.go`
#### Scenario: Topic key stays English
- GIVEN human Spanish
- WHEN referencing BigMem
- THEN MUST stay `sdd/polish-synthesis-human-language/proposal`
#### Scenario: Code stays English
- GIVEN snippet `ORDER BY updated_at DESC`
- WHEN rendered Spanish
- THEN MUST stay English

### Requirement: REQ-PS4 — Marker Invariant (b0d2fc1)
Markers `## Sub-agent Result:`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**` MUST stay English; missing MUST block.
#### Scenario: Spanish keeps English markers
- GIVEN content Spanish with markers English
- WHEN `HasSynthesis` checks
- THEN MUST pass
#### Scenario: Missing blocks
- GIVEN missing `**Artifacts/Paths:**`
- WHEN checkpoint ask ≤120s
- THEN MUST `isError:true`/`block:true`
#### Scenario: Session Recall exception
- GIVEN `## Session Recall` present
- WHEN checkpoint without synthesis
- THEN MUST allow
#### Scenario: Thin advise language-agnostic
- GIVEN thin synthesis `count<2`/`len<50` with markers and `BIGGZ_ADVISE=1`
- WHEN gate evaluates
- THEN MUST not block, MAY emit `concern: synthesis is thin`

### Requirement: REQ-PS5 — Detection + Hint Propagation
MUST detect language via keywords/diacritics and inject `Respond executive_summary in {lang}; keep paths English` into `sdd-*` prompts; ambiguous short MUST default English + fallback at render.
#### Scenario: Detect Spanish → hint Spanish
- GIVEN last `ok, continua`
- WHEN launching sdd-spec
- THEN prompt MUST contain `in Spanish; keep paths English`
#### Scenario: Detect English → hint English
- GIVEN last `ok, continue`
- WHEN launching
- THEN prompt MUST contain `in English`
#### Scenario: Short ambiguous defaults English
- GIVEN last `ok`/`dale`/`go`
- WHEN ambiguous
- THEN MUST default English, fallback-translate at render

### Requirement: REQ-ORCH-001 — Blocking Synthesis Checkpoint (120s)

The system MUST enforce `internal/sdd/synthesis_gate.go`. After EVERY sub-agent (SDD or non-SDD) the orchestrator MUST emit `## Sub-agent Result` with 4 markers (`## Sub-agent Result`, `**What was done:**`|`| Topic | Decision |`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**`) in current turn BEFORE any checkpoint ask. Gate MUST check `HasSynthesis` + `IsCheckpointAsk` (bilingual `proceed|adjust|stop|continue|correct` + `continuar|ajustar|detener|parar|corregir|proseguir`) + 120s window. Missing/expired MUST block `isError:true`/`{block:true}`. `## Session Recall` in current turn bypasses only then.

#### Scenario: Synthesis within window allows

- GIVEN 4 markers emitted 30s ago
- WHEN `ask_user_choice` with `proceed` evaluated
- THEN `ShouldBlock` MUST be `false`

#### Scenario: Missing or expired blocks

- GIVEN no synthesis or `now - currentTurnTime = 121s`
- WHEN checkpoint ask evaluated
- THEN `ShouldBlock` MUST be `true` and handler MUST NOT run

#### Scenario: Non-checkpoint never blocks

- GIVEN no synthesis, question `how are you?`
- WHEN `ShouldBlock` evaluated
- THEN it MUST be `false`

### Requirement: REQ-ORCH-002 — SD Agent Authority

SDD phases (`propose/spec/design/tasks/apply/verify/archive`, plus `explore/research` for SDD change) MUST use `sdd-*` agents only. `general`/`explore` for SDD MUST be rejected fail-closed with `SD Agent Authority` error. `general` remains allowed for non-SDD bounded work only.

#### Scenario: SDD via general rejected

- GIVEN orchestrator tries `general` for `spec` artifact
- WHEN guard evaluates
- THEN it MUST error with `SD Agent Authority` and NOT launch

#### Scenario: SDD via sdd-* allowed

- GIVEN orchestrator launches `sdd-apply` for SDD change
- WHEN guard evaluates
- THEN it MUST allow

### Requirement: REQ-ORCH-003 — Mandatory Pre-Delegation Reads

Orchestrator MUST read `biggz-orchestrator-workflow.md` and `biggz-orchestrator-delegation.md` before routing/continuing/delegating any SDD request. Launch prompt MUST evidence reads. Unreadable MUST block routing.

#### Scenario: Reads evidence routing

- GIVEN both docs read this session
- WHEN routing `sdd-spec`
- THEN it MUST proceed and prompt MUST contain workflow/delegation context

#### Scenario: Missing read blocks

- GIVEN delegation doc skipped/unreadable
- WHEN delegating SDD work
- THEN it MUST block with mandatory-read error

### Requirement: REQ-ORCH-004 — No Fast-Forward Inline or Auto-Continue

Orchestrator MUST NOT inline-write SDD spec/design/tasks artifacts that replace delegated `sdd-*` execution, and MUST NOT auto-continue without explicit token (`proceed|adjust|stop` or `continue|correct`, bilingual). `auto` preflight MAY auto-continue only when gate passes; otherwise MUST STOP and await confirmation. File count/size alone MUST never select SDD.

#### Scenario: Fast-forward inline blocked

- GIVEN SDD spec artifact requested without explicit inline scope
- WHEN ladder evaluated
- THEN it MUST delegate to `sdd-spec`, NOT inline-write

#### Scenario: Interactive auto-continue blocked

- GIVEN preflight `interactive`, spec done
- WHEN evaluating launch of `sdd-design` without `proceed`
- THEN it MUST STOP and emit synthesis + checkpoint first

### Requirement: REQ-SD-O1 — Protocol Docs and Architecture Note

`internal/assets/biggz/bigmem-protocol.md` MUST document under SESSION CLOSE PROTOCOL: mandatory `biggz_mem_session_summary` before every closing `apply` batch and final `done`, bash fallback `biggz bigmem save --type session_summary` when `available_tools` lacks `biggz_mem_*`, and verification via `biggz_mem_context(5)` + `search --query "" ORDER BY updated_at DESC`. `docs/architecture.md` MUST add session-discipline note referencing `internal/sdd/session_guard.go`.

#### Scenario: Protocol contains gate table
- GIVEN `bigmem-protocol.md` rendered
- WHEN searched
- THEN it MUST contain `SESSION CLOSE PROTOCOL`, `biggz_mem_session_summary`, `biggz bigmem save --type session_summary`, and `biggz_mem_context(5)` + `search --query ""`

#### Scenario: Arch note present
- GIVEN `docs/architecture.md` read
- WHEN searched
- THEN it MUST contain `session_guard.go` and `session_summary before done`

### Requirement: REQ-SD-O2 — Workflow Gate Wiring (Q1+Q3)

Orchestrator workflow (`biggz-orchestrator-workflow.md` / `internal/sdd/*`) MUST wire pre-done hook: before `done`, verify `context(5)+search`; missing MUST block with `needs_decision`. Fallback to file (`openspec/changes/{change}/session-fallback.md`) when BigMem unavailable is allowed only after retry-once, with note.

#### Scenario: Workflow blocks done until verified
- GIVEN no verified `session_summary` in `context(5)`/`search`
- WHEN `biggz sdd-status --json --instructions` evaluated for `done`
- THEN orchestrator MUST report `blocked(session_summary_missing)` and NOT emit `done`

### Requirement: REQ-SD-O3 — Complementary + Retry Discipline Visibility (Q4+Q5)

Orchestrator MUST surface complementary discipline in synthesis: per-task saves count + `session_summary` status. On persistent BigMem failure, it MUST note `retry once failed — fallback persisted`, deliver answer, and schedule next-session retry (manual recovery `obs-1788387626730819800-1` no longer needed).

#### Scenario: Synthesis shows both layers
- GIVEN session with 3 per-task saves and 1 `session_summary`
- WHEN synthesis `## Sub-agent Result` rendered
- THEN `Artifacts/Paths` MUST list `sdd/{change}/...` topic keys and `APPLICABLE` fallback path

#### Scenario: Degraded path visible
- GIVEN retry-once still failed
- WHEN closing
- THEN synthesis `Risks` MUST contain `BigMem unavailable — degraded fallback persisted` and answer delivered

### Requirement: REQ-OR-001 — Organic Implementation Routing

The orchestrator MUST route authorized work through exactly one of three implementation routes based on context file count and ambiguity. Size, risk, or perception alone NEVER selects SDD.

| Route | When | What happens |
|---|---|---|
| **Direct inline** | Understand/verify from 1–3 files; or one mechanical, already-understood file with no research or unresolved design decision | Inline edit. No artifacts. No delegation. No SDD. |
| **Delegated direct** | Understand needs 4+ files; reading prepares a write; broad research needed; or writer touches 2+ non-trivial files | One scout or one writer. Bounded. No SDD artifacts. |
| **Optional SDD** | Substantial ambiguity where durable proposal/spec/design/tasks materially reduce uncertainty | Propose SDD. Selected only by explicit request or accepted proposal. |

File counts describe context needed for the current action, not a risk score and not an SDD threshold.

#### Scenario: Direct inline for 2-file fix
- GIVEN user says `fix the typo in README.md and update the test in parser_test.go`
- WHEN orchestrator evaluates route
- THEN it MUST route direct-inline: no SDD artifacts, no delegation, inline edit

#### Scenario: Delegated direct for broad refactor
- GIVEN user says `add error handling to all 5 files in internal/sdd/`
- WHEN orchestrator evaluates route
- THEN it MUST delegate one writer (2+ non-trivial files) and produce no SDD artifacts

#### Scenario: SDD only on explicit request
- GIVEN user says `implement full authentication middleware`
- WHEN orchestrator evaluates route
- THEN it MUST propose SDD (explicit request)
- AND it MUST NOT auto-enroll without acceptance

#### Scenario: Size never selects SDD
- GIVEN user says `rename one variable across 12 files`
- WHEN orchestrator evaluates route
- THEN it MUST route delegated-direct (4+ context files) regardless of size
- AND it MUST NOT route to SDD because of file count

#### Scenario: Ambiguity offers SDD at next boundary
- GIVEN work starts inline, reveals substantial ambiguity mid-flight
- WHEN orchestrator reaches next safe boundary
- THEN it MUST offer SDD via explicit proposal
- AND declining MUST lead to Needs your decision — never silent enrollment

#### Scenario: Per-action delegation does not change route
- GIVEN work routed direct-inline, tests need fresh worker
- WHEN orchestrator dispatches test runner
- THEN it MAY use a fresh worker without changing route
- AND route MUST remain direct-inline

#### Scenario: Direct/delegated creates zero SDD artifacts
- GIVEN work routed direct-inline or delegated-direct
- WHEN implementation completes
- THEN it MUST NOT create proposal.md, spec.md, tasks.md, or phase attempts
- AND it MUST NOT create synthetic SDD runs

### Requirement: REQ-OR-002 — Public Implementation States

The orchestrator MUST expose exactly four public states. These replace current synthesis markers.

| State | Meaning |
|---|---|
| **Working** | Implementation can still change |
| **Checking** | Functional proof and bounded review in progress |
| **Ready** | Exact candidate has sufficient evidence for the delivery route |
| **Needs your decision** | Safe convergence impossible; orchestrator presents cause, impact, and choices |

#### Scenario: Working state on active edit
- GIVEN orchestrator is editing inline
- WHEN state is evaluated
- THEN it MUST report `Working`

#### Scenario: Checking state on test run
- GIVEN orchestrator launched `go test` after direct-inline edit
- WHEN tests are running
- THEN it MUST report `Checking`

#### Scenario: Ready state on passing tests
- GIVEN tests pass, no pending changes
- WHEN orchestrator evaluates state
- THEN it MUST report `Ready`

#### Scenario: Needs your decision on ambiguity
- GIVEN work revealed ambiguity, SDD offered, user declined
- WHEN orchestrator cannot safely converge
- THEN it MUST report `Needs your decision` with cause, impact, and choices

#### Scenario: State replaces synthesis markers
- GIVEN old marker format `◆ spec · success · design`
- WHEN new state system active
- THEN output MUST use `Ready` or `Needs your decision` — NOT old ◆ markers

### Requirement: REQ-OR-003 — Route Field in Status and Continue

`sdd-status --json` MUST include a `route` field on active changes. `sdd-continue` MUST include route context in output.

#### Scenario: Status reports route for direct work
- GIVEN work routed direct-inline, no SDD active
- WHEN `biggz sdd-status --json` called
- THEN `route` MUST be `direct-inline`
- AND `nextRecommended` MUST be empty (no SDD next)

#### Scenario: Status reports route for delegated work
- GIVEN work routed delegated-direct
- WHEN `biggz sdd-status --json` called
- THEN `route` MUST be `delegated-direct`

#### Scenario: Status reports route for SDD work
- GIVEN work routed SDD, spec phase active
- WHEN `biggz sdd-status --json` called
- THEN `route` MUST be `sdd`
- AND `nextRecommended` MUST contain the SDD next phase

#### Scenario: Continue includes route context
- GIVEN work routed direct-inline
- WHEN `biggz sdd-continue <change>` called
- THEN output MUST include `route: direct-inline`

### Requirement: REQ-OR-004 — SDD Opt-In Gate

The orchestrator MUST NOT auto-enroll work into SDD. SDD is selected only by: (1) explicit user request (`use SDD`, `propose a spec`, or equivalent), or (2) accepted proposal after the orchestrator offered it.

#### Scenario: Explicit request triggers SDD
- GIVEN user says `use SDD for this`
- WHEN orchestrator evaluates
- THEN it MUST route to SDD and begin with `sdd-new` or `sdd-propose`

#### Scenario: Proposal accepted triggers SDD
- GIVEN orchestrator offered SDD proposal, user accepted
- WHEN orchestrator evaluates
- THEN it MUST route to SDD and begin planning phases

#### Scenario: Auto-enrollment blocked
- GIVEN orchestrator working inline, detects ambiguity
- WHEN it considers SDD enrollment
- THEN it MUST offer SDD at next safe boundary
- AND it MUST NOT auto-enroll without explicit acceptance

#### Scenario: SDD declined stays on current route
- GIVEN orchestrator offered SDD, user declined
- WHEN work continues
- THEN route MUST remain direct-inline or delegated-direct
- AND state MAY become Needs your decision if ambiguity persists

### Requirement: REQ-OR-005 — Routing Ladder Thresholds

The orchestrator MUST use these exact thresholds for route selection:

| Context files | Route |
|---|---|
| 1–3 | Direct inline |
| 4+ | Delegated direct |
| Ambiguous + user request | Optional SDD |

Writer dispatch: 2+ non-trivial files → delegated direct. Per-action delegation (tests/builds) does not change route.

#### Scenario: Threshold 3 stays inline
- GIVEN 3 files to understand
- WHEN route evaluated
- THEN it MUST be direct-inline

#### Scenario: Threshold 4 delegates
- GIVEN 4 files to understand
- WHEN route evaluated
- THEN it MUST be delegated-direct

#### Scenario: Writer 2+ non-trivial delegates
- GIVEN writer needs to change 2 non-trivial files
- WHEN route evaluated
- THEN it MUST be delegated-direct

#### Scenario: Writer 1 non-trivial stays inline
- GIVEN writer needs to change 1 non-trivial file
- WHEN route evaluated
- THEN it MAY stay direct-inline

### Requirement: REQ-OR-006 — Direct/Delegated Create Zero SDD Artifacts

Direct-inline and delegated-direct work MUST NOT create SDD artifacts, phase attempts, synthetic SDD runs, or change `openspec/changes/` state. Only SDD-routed work touches SDD artifact store.

#### Scenario: Direct work leaves openspec clean
- GIVEN direct-inline fix applied
- WHEN `openspec/changes/` inspected
- THEN no new directories or files MUST exist

#### Scenario: Delegated work leaves openspec clean
- GIVEN delegated-direct refactor completed
- WHEN `openspec/changes/` inspected
- THEN no new directories or files MUST exist

#### Scenario: SDD work creates artifacts
- GIVEN SDD route accepted, spec phase launched
- WHEN phase completes
- THEN `openspec/changes/<change>/specs/` MUST exist with spec.md
