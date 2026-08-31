# Delta for tui-sanitize

## ADDED Requirements

### Requirement: POLISH-TS-01 — Compact Fleet Token Rendering

The system MUST render fleet token metrics in compact form via `formatFleetTokens`/`render`. When `window == spent` or `window < 1000`, the system MUST hide `window` and show only `spent` compact (e.g., `2.2k`). When distinct and `window >= 1000`, it MUST show `window›spent` with `›` separator compact (e.g., `4.1k›2.2k`) in mono muted color and MUST NOT emit repeated `↓ window·spent` per row.

#### Scenario: Window equals spent hides window
- GIVEN `window==spent==2250`
- WHEN `formatFleetTokens` renders
- THEN output MUST be `2.2k` muted mono with no `›` and no `↓`

#### Scenario: Window distinct shows compact pair
- GIVEN `window=4100, spent=2200`
- WHEN rendered
- THEN output MUST be `4.1k›2.2k` with single `›` and muted style

#### Scenario: Window below threshold hides window
- GIVEN `window=800, spent=600`
- WHEN rendered
- THEN output MUST hide window and show only `0.6k` (or `600`) compact

### Requirement: POLISH-TS-02 — Fixed Right Columns Width Guarantees

The system MUST guarantee fixed right columns: `elapsed` 5 visible cells, `tokens` 10 visible cells right-aligned, never truncated to `…` for widths 80..120. The system MUST truncate left content to `floor((width-16)/2)`-like budget so right `visibleWidth` stays constant. `VisibleWidth` MUST use `go-runewidth` after stripping ANSI.

#### Scenario: Right columns constant 80→120
- GIVEN `width=80` and `width=120`
- WHEN same row renders
- THEN `elapsed` MUST be 5c and `tokens` 10c right-aligned with identical `visibleWidth` and no `…`

#### Scenario: Left truncates, right never truncates
- GIVEN narrow 80c row with long agent+model left
- WHEN `TruncateToWidth` applied
- THEN left MAY end with `…` but `elapsed`/`tokens` MUST NOT contain `…`

### Requirement: POLISH-TS-03 — Stable Truncation and CJK Width

The system MUST truncate only left field (L1 left) or L2 activity; it MUST never cut `elapsed`/`tokens` and MUST support recovery via Fleet inspector. `TruncateToWidth` MUST NOT split wide runes, MUST count CJK as width 2 and SGR as 0, and MUST append `…` (width 1) within budget. At 60c narrow, truncation MUST still preserve right columns.

#### Scenario: CJK counts width 2
- GIVEN `s="a中b"` and `w=4`
- WHEN `TruncateToWidth` called
- THEN width MUST be 4 and `中` MUST NOT be split

#### Scenario: Narrow 60c preserves right
- GIVEN `width=60`
- WHEN row renders with long L1+L2
- THEN truncation MUST affect L1-left or L2 only and `elapsed`/`tokens` MUST remain intact
