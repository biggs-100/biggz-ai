# Delta for orchestrator

## MODIFIED Requirements

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

## ADDED Requirements

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

