# Tasks: UI Pi Pretty v2

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 1100-1400 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1 sync → PR2 pills → PR3 footer → PR4 diff → PR5 gallery/mouse/a11y |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | 16ms CSI throttle | PR1 | `go test -run TestSyncOutput` | 3 updates/16ms →1 CSI | `tui.go` |
| 2 | Pills collapse | PR2 | `go test -run TestPill; node --test pills.test.mjs` | 5 pills →`… +2 hidden` | `styles.go`+`pills.js` |
| 3 | Powerline footer | PR3 | `node --test biggz-footer.test.mjs` | order `branch▕change▕lineage▕lens▕budget`, Nerd off→`▕`/`/` | `biggz-footer.js`+`extension-api.js` |
| 4 | Word diff split | PR4 | `go test ./internal/tui -run TestRenderDiff` | 120c split, 80c unified, 1.2MB cap | `diff.go` new |
| 5 | Gallery/mouse/a11y | PR5 | `go run ./scripts/gallery -- /tmp/g && diff docs/gallery` | `BIGGZ_MOUSE=1` on, `TERM=dumb` strip ANSI | `scripts/gallery`+`biggz-question-mouse.js` |

## Phase 1: PR1 Sync (tui.go)

- [x] 1.1 Add `pendingFrame/syncMu/syncTimer` + `scheduleSyncFlush` via `AfterFunc(16ms)` in `tui.go`
- [x] 1.2 Make `syncOutput` idempotent, guard `isSyncSupported()` for `BIGGZ_PRETTY=0`/`NO_ANIMATION`/`TERM=dumb`/`PI_SUBAGENT_CHILD=1`
- [x] 1.3 Test burst 3→1 CSI, guard zero CSI, no double-wrap

## Phase 2: PR2 Pills (styles.go + pills.js)

- [x] 2.1 Add tokens `PillRunning/Queued/Complete/Failed` in `styles.go`
- [x] 2.2 Extend `biggz-tool-pills.js`: `TOOL_PILL_MAP`, `collapseOutput >3→… +N`, `ansiPill`, freeze on `NO_ANIMATION`
- [x] 2.3 Wire `screens/*` to `PillStyle`/`GetSpinnerFrame()` with `IsPrettyEnabled()` fallback
- [x] 2.4 Test 5→3+`… +2 hidden` order, spinner static, `BIGGZ_PRETTY=0` plain

## Phase 3: PR3 Footer (footer.js + extension-api.js)

- [x] 3.1 Add `SEPARATORS`/`getSeparator` +16ms throttle +`PI_SUBAGENT_CHILD=1` bypass in `extension-api.js`
- [x] 3.2 Impl `buildFooterSegments`+`renderFooterLine` order `branch|change|lineage|lens 1/4|budget 1/1` in `biggz-footer.js`
- [x] 3.3 Nerd `›`→`▕`/`/` fallback, `BIGGZ_PRETTY=0` off, `TERM=dumb` ASCII
- [x] 3.4 Test order, Nerd fallback, kill-switch no injection

## Phase 4: PR4 Diff (diff.go)

- [x] 4.1 Create `tui/diff.go` `RenderDiff(old,new,width)` via `DiffMain`, 1MB cap, word highlight
- [x] 4.2 Layout width>100 split `old|new`, else unified
- [x] 4.3 Test 120c split, 80c unified, 1.2MB fallback, malformed no panic

## Phase 5: PR5 Gallery/Mouse/A11y (gallery 80/100)

- [x] 5.1 Update `scripts/gallery/main.go` 80/100 via `HelpOverlay(w)`+`VisibleWidth` → `help-*-80/100.ansi`
- [x] 5.2 Gate `BIGGZ_MOUSE=1` before `enableMouse` in `biggz-question-mouse.js`, default off
- [x] 5.3 Guards: `tickCmd` nil on `NO_ANIMATION`/`TERM=dumb`, spinner `·`, `TERM=dumb` strip ANSI
- [x] 5.4 Test gallery matches `View()` at 80/100, reduced-motion no ticks/sync, mouse off/on, dumb zero ANSI

## Phase 6: Verification

- [x] 6.1 `go vet && go test -count=1 && node --test pi/*.test.mjs`
- [x] 6.2 `go run ./scripts/gallery && git diff docs/gallery` + `TERM=dumb go test -run TestSyncOutput`
- [x] 6.3 `biggz install --agent pi` and verify each PR <400 via `git diff --stat`
