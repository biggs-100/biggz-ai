# Delta for tui

## ADDED Requirements

### Requirement: POLISH-TUI-01 — Two-Line Row Layout

Rows MUST be exactly 2 lines: L1 `glyph agent model·state` + fixed `elapsed` 5c + `tokens` 10c; L2 dim `tool path`/`activity` (truncate L2 only). Height MUST be 2.

#### Scenario: Standard row
- GIVEN tool `edit src/foo.go`, activity `compiling`
- WHEN rendered at 100c
- THEN L1 MUST have glyph+model·state+elapsed+tokens, L2 dim path/activity

#### Scenario: Right intact on narrow
- GIVEN 80c long model name
- WHEN rendered
- THEN L1-left/L2 MAY truncate with `…`, right cols MUST NOT

### Requirement: POLISH-TUI-02 — Workflow Two-Line Hierarchy

Workflow rows MUST be 2 lines: L1 `name·state`, L2 dim `gate/next/output` + failure action inline; nested MUST show `│` dim.

#### Scenario: Failure inline
- GIVEN `gate=verify, failure="test FAIL — rerun"`
- WHEN rendered
- THEN L2 MUST show dim gate/next/output, failure action on nested `│` line

#### Scenario: Nested guide dim
- GIVEN nested child
- WHEN rendered
- THEN prefix MUST be `│` dim, not solid

### Requirement: POLISH-TUI-03 — Collapsed Header Two Groups

Collapsed header MUST show 2 groups via `·`: g1 `X running·Y queued·cap U/L·pane ⚠`, g2 `elapsed·tok`; g1 muted g2 dim; ≤2 numerics +1 hint, no row >2 inline points.

#### Scenario: Two groups rendered
- GIVEN 2 running,1 queued,cap 4/8,pane warn,12s,3k
- WHEN header renders
- THEN MUST be `2 running·1 queued·cap 4/8·pane ⚠ · 12s·3k` muted/dim split

#### Scenario: No overflow
- GIVEN many metrics
- WHEN header renders
- THEN MUST NOT emit >2 numerics +1 hint per group

### Requirement: POLISH-TUI-04 — Collapsible Panes Section

Panes MUST be separate collapsible section `── panes ──`, never intermixed with jobs; toggle collapsed/expanded.

#### Scenario: Panes isolated
- GIVEN panes + jobs
- WHEN rendered
- THEN panes MUST be under `── panes ──` distinct from jobs

#### Scenario: Collapsed hides rows
- GIVEN collapsed
- WHEN rendered
- THEN only header visible, zero pane rows

### Requirement: POLISH-TUI-05 — Tail Visibility for Workflow Rows

When `N > visibleLimit`, system MUST hide tail, preserve order, never prepend `… hidden`; append `… +N hidden` at end.

#### Scenario: Tail hidden
- GIVEN 10 workflows limit 6
- WHEN rendered
- THEN first 6 in order visible, last `… +4 hidden`

#### Scenario: Order preserved
- GIVEN `[a,b,c,d]` limit 2
- WHEN rendered
- THEN visible `[a,b]` + `… +2 hidden`

### Requirement: POLISH-TUI-06 — Unified State Color Encoding

Glyph+text `running/queued/complete/failed` MUST share same solid tone; `elapsed` dim, `tokens` muted; MUST NOT double-encode.

#### Scenario: Running single tone
- GIVEN `running`
- WHEN rendered
- THEN glyph+text share solid, elapsed dim tokens muted

#### Scenario: Failed single tone
- GIVEN `failed`
- WHEN rendered
- THEN share solid failed color, no extra border

### Requirement: POLISH-TUI-07 — Layout Stability on Tick

On 500ms tick, if `Δ elapsed <1s` and `tokens Δ<100`, layout MUST NOT jitter; column widths stable.

#### Scenario: Sub-second no shift
- GIVEN `12s`, tokens stable, Δ 0.5s
- WHEN tick renders
- THEN `visibleWidth` unchanged, no jump

#### Scenario: Small token delta stable
- GIVEN tokens Δ 50, Δ 0.2s
- WHEN rendered
- THEN tokens 10c right-aligned stable
