# Design: UI Pi Pretty v2

## Technical Approach

5 harness-only slices stacked to `main`, each <400 LOC revertible. Reuse `isSyncSupported()` / `tuiAnimationsDisabled()` / `IsPrettyEnabled()` and Rose Pine tokens. Wrappers around `syncOutput`, `styles`, `biggz-footer.js`, `biggz-extension-api.js`, `scripts/gallery`. No ledger/lens/provider. Map: TUI-01→S1, TUI-02+PI-02→S2, PI-01→S3, TUI-03→S4, TUI-04+PI-03→S5.

## Architecture Decisions

| Option | Tradeoff | Decision |
|---|---|---|
| 16ms `AfterFunc` vs `tea.Tick` | AfterFunc 1 timer exact; Tick jitters | **Trailing coalesce** `tui.go`: `pendingFrame`+`syncMu`+`AfterFunc(16ms)`→`flushSyncMsg` |
| Lipgloss tokens in `styles.go` vs inline ANSI JS | Tokens theme-aware single source; inline duplicates | **Tokens** (`ToolPendingBg` etc.) + JS `ansiPill` fallback |
| `sergi/go-diff` vs `difflib` vs custom LCS | sergi has `DiffMain` word-level; difflib line-only | **sergi/go-diff** in `diff.go`; 1MB cap before call |
| Separator registry `extension-api.js` vs hardcoded | Registry reuses `STATUS_LINE_PRESETS`; hardcoded drifts | **Registry** `SEPARATORS`/`getSeparator`, footer reads `getSeparator()` |
| Mouse `BIGGZ_MOUSE=1` gate vs always-on | Always-on breaks Pi selection | **Opt-in** gate before `\x1b[?1000h` |
| Gallery `HelpOverlay(w)` vs golden snapshot | Overlay matches live `View()` wrapping | **Overlay** 80/100 via `VisibleWidth` compare |

## Data Flow

```
S1 lens update → pendingFrame → AfterFunc 16ms → flushSyncMsg → View() → syncOutput(CSI) → term
                 └ guards(BIGGZ_PRETTY/NO_ANIMATION/TERM/PI_SUBAGENT_CHILD) → strip CSI
S2 tool_call → getToolPill → styles.PillStyle → collapse >3 → … +N hidden → viewport
   JS pi.on(tool_call) → extension-api 16ms throttle → ansiPill → same collapse
S3 ctx(cwd/branch/tokens/cost/context/model) → buildFooterSegments → renderFooterLine(budget) → sepStr(getSeparator) → theme.fg
   fallback: BIGGZ_PRETTY=0 no setFooter, TERM=dumb ASCII ▕/
S4 raw diff → cap 1MB → DiffMain → width>100? split old|new : unified → word highlight
S5 go run ./scripts/gallery → HelpOverlay@80/100 → docs/gallery/*.ansi + fixtures.json
   TERM=dumb stripAnsi, NO_ANIMATION tickCmd=nil spinner·
```

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/tui/tui.go` | Modify | Throttled sync: `pendingFrame`+`syncMu`+`AfterFunc(16ms)`+`flushSyncMsg`, idempotent CSI |
| `internal/tui/styles/styles.go` | Modify | Pill/diff/footer tokens: `PillRunning/Queued/Complete/Failed`, `DiffAdded/Removed` |
| `internal/tui/diff.go` | Create | `RenderDiff(old,new,width)` sergi/go-diff; split>100 else unified; 1MB cap |
| `internal/tui/screens/*` | Modify | Wire pills/diff; shared 80ms `GetSpinnerFrame()` |
| `internal/assets/pi/biggz-tool-pills.js` | Modify | `TOOL_PILL_MAP`, `collapseOutput(>3→… +N)`, freeze on `NO_ANIMATION` |
| `internal/assets/pi/biggz-footer.js` | Modify | `buildFooterSegments`+`renderFooterLine` order `branch\|change\|lineage\|lens 1/4\|budget 1/1`; Nerd→▕/ fallback |
| `internal/assets/pi/biggz-extension-api.js` | Modify | `SEPARATORS`+`getSeparator`, 16ms pill write, `PI_SUBAGENT_CHILD` bypass |
| `internal/assets/pi/biggz-question-mouse.js` | Modify | `BIGGZ_MOUSE=1` gate before `enableMouse`; respects pretty/animation/dumb |
| `scripts/gallery/main.go` | Modify | Deterministic 80/100 via `HelpOverlay`+`VisibleWidth` |
| `docs/gallery/*` | Modify | Regenerated `help-*-80/100.ansi`, `fixtures.json` |

## Interfaces / Contracts

```go
// tui.go — trailing coalesce (non-obvious)
var pendingFrame string; var syncMu sync.Mutex; var syncTimer *time.Timer
func scheduleSyncFlush(f string) // AfterFunc 16ms → flushSyncMsg
func isSyncSupported() bool // BIGGZ_PRETTY!=0 && !tuiAnimationsDisabled() && TERM!="dumb" && PI_SUBAGENT_CHILD!="1"
func RenderDiff(old, new string, width int) string // >100 split, else unified; cap 1MB
func PillStyle(state string) lipgloss.Style
```

```js
// extension-api.js
export const SEPARATORS = {"powerline-thin":{left:"›"},slash:{left:" / "}}
export function getSeparator(style) // registry lookup
// footer.js
export function buildFooterSegments(theme, footerData, ctx, pi) // returns segs+raw+widths
export function renderFooterLine(width, theme, segs) // budgeted, drops cost→tokens→context
```

## Guards Matrix

| Guard | Value | Effect | Scope |
|---|---|---|---|
| `BIGGZ_PRETTY=0` | `0` | Kill-switch: no CSI/pills/footer/mouse, plain text | All slices |
| `BIGGZ_NO_ANIMATION=1` / `GENTLE_AI_NO_ANIMATION=1` | `1` | `tickCmd=nil`, no CSI, spinner `·` frozen | TUI-01,02,04+PI |
| `TERM=dumb` | `dumb` | Strip ANSI, no CSI/spinner, ASCII `▕`/`/` | All |
| `PI_SUBAGENT_CHILD=1` | `1` | Suppress Pi footer/pill injection | PI-01,02 |
| `BIGGZ_MOUSE=1` | `1` | Opt-in `enableMouse`; default off; overridden by above | PI-03 |

## Testing Strategy

| Layer | What to Test | Approach |
|---|---|---|
| Unit Go | Throttle single CSI, idempotent, guards | `go test -run TestSyncOutput` |
| Unit Go | Pill `>3→… +N`, spinner frozen, diff 80/120, 1MB cap | `go test` + `t.Setenv` |
| Unit JS | Footer order, Nerd fallback, pill order, mouse opt-in | `node --test` |
| Integration | Gallery 80/100 matches `View()` | `go run ./scripts/gallery -- /tmp/g && diff` |
| E2E | `install --agent pi` + dumb/no-animation zero ANSI | `TERM=dumb go test` |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary. Harness ANSI only.

## Migration / Rollout

5 PRs linear `1-sync→2-pills→3-footer→4-diff→5-gallery/mouse`, each <400. Revert `git revert 5..1`. No migration. Kill-switches `BIGGZ_PRETTY=0`/`BIGGZ_NO_ANIMATION=1` disable without revert. `biggz install --agent pi` redeploys.

## Verification

```bash
go vet ./...
go test ./... -count=1 -timeout 180s
node --test internal/assets/pi/*.test.mjs
go run ./scripts/gallery && git diff --exit-code docs/gallery
TERM=dumb go test ./internal/tui -run TestSyncOutput
```

## Open Questions

- [ ] `›` vs `▕` for powerline-thin when NerdFont partial — current `›` thin, `▕` fallback ok?
- [ ] Gallery PNG via `kitty icat` deferred, ANSI sufficient for CI?
