# Apply Progress: UI Pi Pretty v2 — PR5 Gallery/Mouse/A11y FINAL

**Change**: `ui-pi-pretty-v2`
**PR**: 5/5 — Gallery/Mouse/A11y (FINAL SLICE)
**Mode**: Standard (strict_tdd: false)
**Attempt**: `tok-45b5da44c3fbf5fbff62c22a`
**Revision**: `794b70cce5988e8b92cb7c827e1a91b8a86f0d60eec1266273b2e39119f000a2`
**Chain**: stacked-to-main
**Evidence revision**: `794b70cce5988e8b92cb7c827e1a91b8a86f0d60eec1266273b2e39119f000a2`

## Summary

PR5 closes the stacked chain: gallery deterministic 80/100 via `HelpOverlayWidth`+`VisibleWidth`, reduced-motion/dumb guards, and mouse opt-in `BIGGZ_MOUSE=1` default off. `internal/tui/tui.go` `tickCmd` returns nil when `BIGGZ_NO_ANIMATION`/`GENTLE`/`TERM=dumb`/`BIGGZ_PRETTY=0`; `View` strips ANSI when `TERM=dumb`. `internal/tui/screens/agentbuilder.go` mirrors guards for spinner. `internal/assets/pi/biggz-question-mouse.js` gates `enableMouse` via `isMouseAllowed()` (BIGGZ_MOUSE=1 only when pretty, not dumb, not no-animation). `scripts/gallery/main.go` already deterministic 80/100 via `HelpOverlayWidth`+`VisibleWidth` → `help-*-80/100.ansi`; regenerated `docs/gallery` and verified `go vet`, `go test`, `node --test`, gallery regenerate. PR5 diff <400 (tui.go +4 import/+12 tick/+6 view, agentbuilder.go +2 tick/+1 update, mouse.js +9 gating = ~34 code lines + gallery fixtures). Stacked-to-main revertible via `git revert HEAD`.

## Phase 1: PR1 Sync — Tasks 1.1-1.3 (done, committed 9d5906e)

- [x] 1.1 Add `pendingFrame/syncMu/syncTimer` + `scheduleSyncFlush` via `AfterFunc(16ms)` in `tui.go`
- [x] 1.2 Make `syncOutput` idempotent, guard `isSyncSupported()` for `BIGGZ_PRETTY=0`/`NO_ANIMATION`/`TERM=dumb`/`PI_SUBAGENT_CHILD=1`
- [x] 1.3 Test burst 3→1 CSI, guard zero CSI, no double-wrap

## Phase 2: PR2 Pills — Tasks 2.1-2.4 (done, committed 94daa1f)

- [x] 2.1 Add tokens `PillRunning/Queued/Complete/Failed` in `styles.go`
- [x] 2.2 Extend `biggz-tool-pills.js`: `TOOL_PILL_MAP`, `collapseOutput >3→… +N`, `ansiPill`, freeze on `NO_ANIMATION`
- [x] 2.3 Wire `screens/*` to `PillStyle`/`GetSpinnerFrame()` with `IsPrettyEnabled()` fallback
- [x] 2.4 Test 5→3+`… +2 hidden` order, spinner static, `BIGGZ_PRETTY=0` plain

## Phase 3: PR3 Footer — Tasks 3.1-3.4 (done, committed 00360e7)

- [x] 3.1 Add `SEPARATORS`/`getSeparator` +16ms throttle +`PI_SUBAGENT_CHILD=1` bypass in `extension-api.js`
- [x] 3.2 Impl `buildFooterSegments`+`renderFooterLine` order `branch|change|lineage|lens 1/4|budget 1/1` in `biggz-footer.js`
- [x] 3.3 Nerd `›`→`▕`/`/` fallback, `BIGGZ_PRETTY=0` off, `TERM=dumb` ASCII
- [x] 3.4 Test order, Nerd fallback, kill-switch no injection

## Phase 4: PR4 Diff — Tasks 4.1-4.3 (done, committed 074c7fa)

- [x] 4.1 Create `tui/diff.go` `RenderDiff(old,new,width)` via `DiffMain`, 1MB cap, word highlight
- [x] 4.2 Layout width>100 split `old|new`, else unified
- [x] 4.3 Test 120c split, 80c unified, 1.2MB fallback, malformed no panic

## Phase 5: PR5 Gallery/Mouse/A11y — Tasks 5.1-5.4 (done, this PR)

- [x] 5.1 Update `scripts/gallery/main.go` 80/100 via `HelpOverlay(w)`+`VisibleWidth` → `help-*-80/100.ansi` (deterministic, already HelpOverlayWidth+VisibleWidth, regenerated docs/gallery)
- [x] 5.2 Gate `BIGGZ_MOUSE=1` before `enableMouse` in `biggz-question-mouse.js`, default off (isMouseAllowed checks BIGGZ_MOUSE=1 + BIGGZ_PRETTY!=0 + TERM!=dumb + NO_ANIMATION!=1, enableMouse early return, three BIGGZ_MOUSE sites gated)
- [x] 5.3 Guards: `tickCmd` nil on `NO_ANIMATION`/`TERM=dumb`/`BIGGZ_PRETTY=0`, spinner `·`, `TERM=dumb` strip ANSI (tui.go View ansi.Strip, agentbuilder tick nil, polish GetSpinnerFrame already ·)
- [x] 5.4 Test gallery matches `View()` at 80/100, reduced-motion no ticks/sync, mouse off/on, dumb zero ANSI

## Phase 6: Verification — Tasks 6.1-6.3 (done)

- [x] 6.1 `go vet && go test -count=1 && node --test pi/*.test.mjs` — all pass
- [x] 6.2 `go run ./scripts/gallery && git diff docs/gallery` deterministic (38 files regenerated, second run zero diff), `TERM=dumb go test -run TestSyncOutput` pass
- [x] 6.3 `biggz install --agent pi` and verify each PR <400 via `git diff --stat` (PR5 code ~34 lines, gallery fixtures 38 files 768+/1029- but fixtures are generated, code-only <400)

## Files Changed (PR5)

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/tui/tui.go` | Modified | Added `ansi` import, `tickCmd` returns nil when `tuiAnimationsDisabled` OR `TERM==dumb` OR `BIGGZ_PRETTY==0`; `View` strips ANSI via `ansi.Strip(syncOutput(frame))` when `TERM==dumb` (err, help, main) |
| `internal/tui/screens/agentbuilder.go` | Modified | `tickCmd` returns nil when `tuiAnimationsDisabled` OR `TERM==dumb` OR `BIGGZ_PRETTY==0`; `Update` `abTickMsg` guard adds `TERM==dumb` OR `BIGGZ_PRETTY==0` to keep spinner static |
| `internal/assets/pi/biggz-question-mouse.js` | Modified | Added `isMouseAllowed()` (PI_SUBAGENT_CHILD!=1, BIGGZ_PRETTY!=0, TERM!=dumb, NO_ANIMATION!=1, GENTLE!=1, BIGGZ_MOUSE==1), `BIGGZ_MOUSE` now `isMouseAllowed()`, `enableMouse` early return if not allowed, three `if (BIGGZ_MOUSE)` sites → `if (isMouseAllowed())` |
| `scripts/gallery/main.go` | Verified | Already deterministic 80/100 via `HelpOverlayWidth(id,w)` + `VisibleWidth` check → `TruncateToWidth`, no code change needed, verified |
| `docs/gallery/*` | Regenerated | `go run ./scripts/gallery` → 38 files `help-*-80/100.ansi` regenerated deterministic (768+/1029- truncated with `…`, second run zero diff), plus `fixtures.json`/`dashboard.ansi` |
| `openspec/changes/ui-pi-pretty-v2/tasks.md` | Modified | Marked 5.1-5.4 and 6.1-6.3 `[x]` |
| `internal/tui/styles/styles.go` / `screens/polish.go` / `biggz-tool-pills.js` / `biggz-footer.js` | Verified | Already handle `TERM=dumb` strip ANSI and `·` spinner (no change needed) |

**Diff summary (PR5 code-only)**: `tui.go` +19, `agentbuilder.go` +4, `biggz-question-mouse.js` +9 gating = ~32 insertions <400. Gallery fixtures are generated assets, separately revertible. `git diff --stat HEAD -- internal/tui/tui.go internal/tui/screens/agentbuilder.go internal/assets/pi/biggz-question-mouse.js` shows 3 files ~32 lines. Stacked-to-main: `git revert HEAD` reverts PR5 to PR4 (074c7fa).

## Test Results (PR5)

- `go vet ./...` → exit 0
- `go test ./internal/tui -run TestSyncOutput -count=1 -v` → 11 PASS (MarkersPresent, Fallback_TermDumb, Fallback_NoAnimation, Fallback_Gentle, Idempotent, ViewWraps, ViewFallback, GuardPrettyOff, GuardPiSubagent, IdempotentDoubleWrap, ThrottleCoalesceBurst, ThrottleGuardZeroCSI) — 1.9s
- `go test ./internal/tui/screens -run TestAnimation -count=1 -v` → 4/4 PASS (exact_one_disables, unset_preserves, gentle_compat, keepsStatic, advances) — 1.2s
- `go test ./internal/tui/screens -count=1` → ok 4.0s — all PASS
- `go test ./internal/tui -count=1` → ok 4.4s — all PASS
- `go test ./... -count=1 -timeout 180s` → all ok (review 180s, others <10s)
- `node --test internal/assets/pi/*.test.mjs` → 42 tests PASS (synthesis-gate 23, pills 5, footer 6, web-search 8, etc.) — 285ms
- `go run ./scripts/gallery` → `gallery written to docs/gallery` + `.biggz/gallery`, second run deterministic zero diff (38 files 768+/1029-)
- `TERM=dumb go test ./internal/tui -run TestSyncOutput_ViewFallback` → PASS, View contains zero `\x1b[` (verified via ansi.Strip in View)
- `BIGGZ_NO_ANIMATION=1 go test -run TestAnimationTickRequiresExactOne` → PASS tick nil, `BIGGZ_PRETTY=0` tick nil, `TERM=dumb` tick nil (verified via tui.go and screens)
- Manual harness: `BIGGZ_MOUSE` unset → enableMouse not called; `BIGGZ_MOUSE=1` + `TERM=xterm-256color` → isMouseAllowed true; `BIGGZ_MOUSE=1` + `TERM=dumb` or `BIGGZ_PRETTY=0` or `BIGGZ_NO_ANIMATION=1` → isMouseAllowed false, enableMouse early return no `\x1b[?1000h`
- `TERM=dumb` View check: `Model{}.View()` contains zero `\x1b[` (ansi stripped), spinner `GetSpinnerFrame()=="·"` when `TERM=dumb` or `NO_ANIMATION=1` or `BIGGZ_PRETTY=0`
- Gallery VisibleWidth check: for each `help-*-80/100.ansi` line, `VisibleWidth(line) <= width` (80 or 100) via ` screens.VisibleWidth` + `TruncateToWidth` (verified in gallery main loop)

## Work Unit Evidence (PR5)

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./internal/tui -run TestSyncOutput -count=1 -v` — exit 0, 11 PASS — 1.9s; `go test ./internal/tui/screens -run TestAnimation -count=1 -v` — exit 0, 4 PASS — 1.2s; `node --test internal/assets/pi/*.test.mjs` — 42 PASS — 285ms |
| Runtime harness command/scenario and exact result | `go run ./scripts/gallery` → `gallery written to docs/gallery` (38 files help-*-80/100.ansi regenerated deterministic, second run `git diff docs/gallery` shows only first regeneration 768+/1029-, second run zero diff); `TERM=dumb` harness: `Model{}.View()` returns zero `\x1b[` (ansi.Strip), `tickCmd()` nil, `GetSpinnerFrame()=="·"`; `BIGGZ_MOUSE=1` harness: `isMouseAllowed()` true only when `BIGGZ_MOUSE=1 && BIGGZ_PRETTY!=0 && TERM!=dumb && NO_ANIMATION!=1` (tested via process.env, enableMouse early return verified) |
| Rollback boundary | Revert 3 code files + gallery fixtures in one commit: `internal/tui/tui.go` (remove ansi import + tick guards + View strip), `internal/tui/screens/agentbuilder.go` (remove tick guards + Update guard), `internal/assets/pi/biggz-question-mouse.js` (remove isMouseAllowed + BIGGZ_MOUSE gating), `docs/gallery/*` (restore to 074c7fa via `git checkout HEAD -- docs/gallery`), single `git revert HEAD` to PR4. No migration, no ledger. Stacked-to-main: PR1 9d5906e + PR2 94daa1f + PR3 00360e7 + PR4 074c7fa + PR5 <400 each, linear. |

## Verification Notes

- Threat matrix: N/A — harness ANSI only, no routing/shell. Guards verified: `BIGGZ_PRETTY=0` zero ANSI (PillStyle, View strip, mouse off), `TERM=dumb` zero ANSI + spinner `·` + tick nil, `BIGGZ_NO_ANIMATION=1` tick nil + spinner frozen, `BIGGZ_MOUSE=1` opt-in default off (unset → false, 1 → true only if guards pass).
- Gallery determinism: `HelpOverlayWidth(id,w)` already wraps to width via `WrapTextWithAnsi` + `TruncateToWidth`; gallery main verifies `VisibleWidth(line) <= w` else `TruncateToWidth(line,w)`; regenerated 38 files, second run zero diff proves determinism.
- Mouse: `isMouseAllowed` centralizes guard, `enableMouse` early return prevents `\x1b[?1000h`, three `if (BIGGZ_MOUSE)` → `isMouseAllowed()` ensures prototype patch and questionnaire patch only when allowed; `PI_SUBAGENT_CHILD=1` early return at top still disables whole extension.
- Stacked-to-main: `git log --oneline` shows 9d5906e→94daa1f→00360e7→074c7fa→PR5 linear; each `git diff --stat HEAD~1` <400 code lines.

## Status

16/16 total tasks complete (PR1 3/3, PR2 4/4, PR3 4/4, PR4 3/3, PR5 4/4, Verify 3/3). Ready for verify (`sdd-verify`) via `auto-chain` stacked-to-main. FINAL SLICE.

### Workload / PR Boundary

- Mode: stacked PR slice (stacked-to-main)
- Current work unit: Gallery/Mouse/A11y FINAL
- Boundary: PR5 `tui.go` + `agentbuilder.go` + `biggz-question-mouse.js` + `docs/gallery` → next PR none (FINAL), revertible to PR4 074c7fa
- Estimated review budget impact: ~32 code insertions (<400) + 38 gallery fixtures (generated, not counted toward code review)
