# Delta for orchestrator

## ADDED Requirements

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
