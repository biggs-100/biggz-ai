# Design: polish-wait-visuals (B intermedia)

## Technical Approach

B = A + fixed-right 5c/10c + 2-line rows + 2-group header + collapsible panes + shim 3s + ADR. `sanitize.go` guarantees width (CJK2/ANSI0), `synthesis.go` chunks table (budget 17, chunk 7), `screens/*` 2-line layout with tail, `biggz-pi-pretty.js` coalesces Pi wait to ≤2 lines. Presentation-only.

## Architecture Decisions

| Decision | Option | Tradeoff | Choice |
|----------|--------|----------|--------|
| Token compaction | `formatFleetTokens` in `sanitize.go` reused in `synthesis.go` | Single authority, `›`, hide `window==spent` or `<1k` → `2.2k` vs `4.1k›2.2k` | **Chosen** |
| | Duplicate per renderer | Drift, repeated `↓ window·spent` | Rejected |
| Fixed right cols | `VisibleWidth=runewidth(ansi.Strip)` + `TruncateToWidth` budget `(width-6)/2` | 5c elapsed dim +10c tokens muted constant 80→120c | **Chosen** |
| | `lipgloss.Width()` | CJK/ANSI inconsistent | Rejected |
| 2-line row | `height=2` fixed, L1 clamp + rightAlign, L2 dim | `terminal.rows` stable, no jitter | **Chosen** |
| | Dynamic 1–3 lines | Layout shift, scroll overflow | Rejected |
| Wait throttle | `biggz-pi-pretty.js` wrap `asyncWaitUpdate` 1s→3s debounce + headline | No fork, 1-file revert, ADR PR | **Chosen** |
| | Fork/vendor | Maintenance/drift | Rejected (fallback only) |
| Panes state | `panesCollapsed bool` + header `── panes ──` toggle | Isolated | **Chosen** |

## Data Flow

```
Pi runs → biggz-pi-pretty.js (3s debounce, headline 1+1 dim) → FleetView
            └→ subagent-config.json compactResultMaxLines:20 (collapse >20)

TUI: terminal.width → VisibleWidth/TruncateToWidth (coalesceSGR→runewidth) → row()
      → elapsed 5c dim + tokens 10c muted (right never `…`)
      → synthesis.go renderTable(17, chunk 7) → screens/* View()
      → header 2 groups → jobs 2-line → workflow │ dim → panes ── → tail hidden
Height = rows*2 + header(1) + panesHeader(1) + tailHint(0/1) ≤ terminal.rows
```

Layout:
- `row(width)`: `rightW=16` (5+1+10); `leftBudget=(width-6)/2`; `L1 = TruncateToWidth(left, leftBudget) + rightAlign(elapsed,5) + rightAlign(tokens,10)`; `L2 = dim(TruncateToWidth(activity,width))`; `height=2`.
- Workflow: L1 `name·state`, L2 `gate/next/output` dim + failure inline; nested prefix `│` dim per depth, overflow → L2 truncate.
- Header: `g1=X running·Y queued·cap U/L·pane ⚠` muted + `g2=elapsed·tok` dim, `·` separator, ≤2 numerics +1 hint.
- `visibleWorkflowRows(rows,limit)`: slice `[:limit]` preserve order, append `… +N hidden` tail, never prepend.

Colors/stability: glyph+text solid same tone per state, elapsed dim, tokens muted; `go-runewidth` CJK=2, `coalesceSGR` dedups, `VisibleWidth` after `ansi.Strip`; tick 500ms coalesced — `Δ<1s` and `tokens Δ<100` no layout shift.

## File Changes

| File | Action | Desc | Est. |
|------|--------|------|------|
| `internal/tui/sanitize.go` | Modify | Add `formatFleetTokens`/`compactK`, keep `VisibleWidth`/`TruncateToWidth`/`coalesceSGR` | +35 |
| `internal/tui/screens/sanitize.go` | Modify | Mirror (cycle-safe) | +35 |
| `internal/sdd/synthesis.go` | Modify | `renderTable` budget 17 `(40-6)/2`, `chunkTable` 7, right 5c/10c fixed; headline hook | +50 |
| `internal/tui/screens/*` | Modify | `row()` clamp, workflow `│`, header 2 groups, panes collapsible, `visibleWorkflowRows` tail | +90 |
| `internal/assets/pi/biggz-pi-pretty.js` | Create | Wrap `asyncWaitUpdate`/`detachedForegroundWaitUpdate`, 3s debounce, headline `Wait {s}s · {N} runs (…) — open Fleet…` | +80 |
| `internal/assets/pi/subagent-config.json` | Modify | `compactResultMaxLines: 100→20` | 1 |
| `docs/adr/xxx-pi-subagents-wait.md` | Create | ADR fork/vendor/shim/config tradeoffs → shim+PR | +70 |

Total ~361 lines, 6 mod +2 new. Rollback `git revert HEAD` <5min.

## Interfaces / Contracts

```go
func VisibleWidth(s string) int // ansi.Strip → runewidth.StringWidth
func TruncateToWidth(s string, w int) string // CJK-safe, SGR, … width 1
func compactK(n int) string // 4100→"4.1k"
func formatFleetTokens(window, spent int) string // "4.1k›2.2k" or "2.2k" muted 10c
func row(width int, ...) (l1, l2 string, h int) // leftBudget (width-6)/2, rightAlign fixed
func visibleWorkflowRows(rows []Row, limit int) ([]Row, int) // tail hidden
func headerGroups(running, queued, capU, capL int, paneWarn bool, elapsed, tok string) string
func renderTable(rows [][]string, width int) string // budget 17, chunk 7
```

Throttle: `lastRender=0, pending=null, delay=3000; if now-lastRender<3000 debounce headline, suppress duplicate; else render 1 solid +1 dim, never dump `formatAsyncRunList`.

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | `formatFleetTokens` hide `›` when `==spent`/`<1k` | Table `2250→2.2k`, `4100/2200→4.1k›2.2k` |
| Unit | `VisibleWidth` CJK2/ANSI0, `TruncateToWidth` no split `中`, 60c right intact | `sanitize_test.go` golden |
| Integration | 80→120c right constant, 60c narrow no right `…` | `View(80)` vs `View(120)` `visibleWidth` equal |
| Integration | 2-line height, `│` dim, header 2 groups, panes toggle | TUI width tests `VisibleWidth(View(80))≤80` |
| Mock | Throttle 1s→3s suppress, headline ≤2 lines, collapse 20 | Node mock time 0/1.5s/3s |

`go vet PASS`, `node --test biggz-synthesis-gate 22/22`, `go test -run TestSanitize/TestSynthesis`.

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary. Shim is revertible ExtensionAPI wrapper, flag-gated.

## Migration / Rollout

No migration. `BIGGZ_PRETTY=0` disables shim. ADR PR to upstream; shim fallback until merged.

## Open Questions

- [ ] ADR number `xxx` — assign sequential id on merge.
- [ ] `compactK` rounding `%.1fk` 1k–10k vs integer >10k — confirm.
