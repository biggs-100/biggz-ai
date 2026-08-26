# Apply Progress: Pi Enhancements from oh-my-pi — TUI Sync (PR1)

## Summary

PR1 (TUI CSI 2026 + bracketed paste) implements `isSyncSupported()` / `syncOutput(frame)` gated on `TERM` and `BIGGZ_NO_ANIMATION`/`GENTLE_AI_NO_ANIMATION`, wraps `Model.View()` atomically with `ESC[?2026h`/`ESC[?2026l`, and buffers `ESC[200~`..`ESC[201~` into single `PasteMsg` (large pastes >10 lines as one event, incomplete flushes on next input, paste content not interpreted as keys). Central View provides atomic render (idempotent); screens can opt-in via same helper. Verified with fixture sequence, `go vet` and `go test ./internal/tui -count=1` green. No hashline/web/advisor touched. Retry avoided `grep -r` on GOPATH/pkg/mod; exploration limited to `rg` inside `internal/tui`.

## PR1 Scope (TUI — Tasks 1.1-1.2 + 2.1-2.4)

- [x] 1.1 Add `isSyncSupported()` + `syncOutput(frame)` in `internal/tui/tui.go` (TERM/`BIGGZ_NO_ANIMATION` gate). Verify: `TERM=dumb` → plain.
- [x] 1.2 Add `PasteMsg{Text}` + buffer in `internal/tui/tui.go`. Verify: incomplete `ESC[200~` flushes.
- [x] 2.1 Implement `syncOutput` with `ESC[?2026h`/`ESC[?2026l`. Verify: markers present; fallback no garble.
- [x] 2.2 Implement paste buffer `ESC[200~`..`ESC[201~` → one `PasteMsg`. Verify: 15 lines single event; `ctrl+c` ignored.
- [x] 2.3 Wire `internal/tui/screens/*.go` via `syncOutput`. Verify: atomic render. — central `Model.View()` wraps with `syncOutput` (idempotent); screens opt-in available via same helper, minimal touch as instructed.
- [x] 2.4 Add `internal/tui/tui_test.go` (sync, fallback, paste). Verify: `go test ./internal/tui -count=1` passes.

Pending: Phase 3 hashline (3.1-3.4), Phase 4 web/advisor (4.1-4.5), Phase 5 verification (5.1-5.3).

## Files Changed (PR1 incremental)

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/tui/tui.go` | Modified | Add `syncBegin`/`syncEnd`/`PasteMsg`, `isSyncSupported()` (TERM != dumb, `BIGGZ_NO_ANIMATION`/`GENTLE_AI_NO_ANIMATION` gate), `syncOutput(frame)` (idempotent), `pasteActive`/`pasteBuf` on Model, `feedPaste`/`flushPaste` buffer, `Update` handles `string` bracketed chunks and `PasteMsg` without key interpretation, `View` wraps frame with `syncOutput` (error/help/switch branches) |
| `internal/tui/tui_test.go` | Modified | Add 10 tests: `TestSyncOutput_MarkersPresent`, `Fallback_TermDumb`, `Fallback_NoAnimation`, `Fallback_GentleAnimation`, `Idempotent`, `ViewWraps`, `ViewFallback`, `BracketedPaste_SingleEvent_15Lines` (fixture), `CtrlCIgnored`, `IncompleteFlush` (flush + Update flush), `MultiChunkSplit` — fixture sequence no network, no GOPATH grep |
| `openspec/changes/2026-08-26-pi-enhancements-from-omp/tasks.md` | Modified | Mark Phase 1 (1.1,1.2) and Phase 2 (2.1-2.4) as [x]; set Chain strategy stacked-to-main; leave 3.x/4.x/5.x pending |
| `openspec/changes/2026-08-26-pi-enhancements-from-omp/apply-progress.md` | Created | This progress file |

No changes to `internal/filemerge`, `internal/review`, `internal/assets/pi/biggz-web-search.js`, `biggz-synthesis-gate.js` — boundaries respected per slice instruction.

## Test Results (PR1)

- `go vet ./internal/tui/...` → exit 0 (no output)
- `go test ./internal/tui -count=1 -v` → exit 0, 18 top-level tests PASS (0.19-0.40s each, total ~4s)
  - Animation: `TestAnimationRequiresExactOne` (7 subcases) + `TestAnimationDisabledWithEnv` (2) — pre-existing, still green
  - Core: `TestNewModel`, `TestNavigate`, `TestHelpToggle`, `TestQuit`, `TestHelpContent`, `TestHelpOverlay` (6) — still green
  - Sync: `TestSyncOutput_MarkersPresent`, `Fallback_TermDumb`, `Fallback_NoAnimation`, `Fallback_GentleAnimation`, `Idempotent`, `ViewWraps`, `ViewFallback` (7) — new, all PASS
  - Paste: `TestBracketedPaste_SingleEvent_15Lines`, `CtrlCIgnored`, `IncompleteFlush`, `MultiChunkSplit` (4) — new, all PASS (fixture 15 lines single PasteMsg, ctrl+c preserved not quit, incomplete flushes, multi-chunk split merges)
- `go test ./internal/tui -count=1` → ok 4.045s — satisfies tasked harness `go test ./internal/tui -run TestSync -count=1` (sync tests) and full package

## Work Unit Evidence (PR1 — TUI CSI 2026 + bracketed paste)

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./internal/tui -run TestSync -count=1 -v` — exit 0, 7 tests PASS (MarkersPresent, Fallback_TermDumb, Fallback_NoAnimation, Fallback_GentleAnimation, Idempotent, ViewWraps, ViewFallback); `go test ./internal/tui -run TestBracketed -count=1 -v` — exit 0, 4 tests PASS (SingleEvent_15Lines, CtrlCIgnored, IncompleteFlush, MultiChunkSplit); full `go test ./internal/tui -count=1` — exit 0, 18 tests PASS, ok 4.045s |
| Runtime harness command/scenario and exact result | `BIGGZ_NO_ANIMATION=1` vs `TERM=dumb` fallback verified via `TestSyncOutput_Fallback_*` (plain without garble); 15-line fixture verified via `TestBracketedPaste_SingleEvent_15Lines` — `bracketedPasteStart + 15 lines + bracketedPasteEnd` → single `PasteMsg` with 15 lines, `strings.Count == 15`; `ctrl+c` ignored verified via `TestBracketedPaste_CtrlCIgnored` (paste preserves `ctrl+c` text, `PasteMsg` Update does not quit, direct `tea.KeyCtrlC` still quits); incomplete flush verified via `TestBracketedPaste_IncompleteFlush` — `ESC[200~partial` without end `feedPaste` nil + `pasteActive` true, `flushPaste` returns `partial`, Update next non-paste `hello` flushes `partial` and clears `pasteActive`; `go vet ./internal/tui/...` — exit 0, no garbled ESC |
| Rollback boundary | Revert `internal/tui/tui.go` to pre-sync version (remove `syncBegin/syncEnd/PasteMsg/isSyncSupported/syncOutput/feedPaste/flushPaste/pasteActive/pasteBuf/View wrapping/Update paste handling`) + revert `internal/tui/tui_test.go` to 6 tests (remove 10 sync/paste tests + helper min/max) + revert `tasks.md` Phase 1/2 checkboxes to [ ] and Chain pending + delete this `apply-progress.md`; `git revert` single commit, no migration, no screens touched (central View only), no hashline/web/advisor affected |

## TDD Cycle Evidence (Strict TDD false — Standard Mode, PR1)

| Task | RED | GREEN | REFACTOR |
|------|-----|-------|----------|
| 1.1 isSyncSupported + syncOutput | N/A (Standard Mode) — RED would be `TestSyncOutput_MarkersPresent` fail without impl → GREEN after `isSyncSupported`/`syncOutput` + `View` wrapping | `go vet` pass, 7 sync tests green | Idempotent guard added to avoid double-wrap |
| 1.2 PasteMsg + buffer | N/A — RED `TestBracketedPaste_IncompleteFlush` flush nil fail before `pasteBuffer` → GREEN after `PasteMsg`/`feedPaste`/`flushPaste` | `go vet` pass, incomplete flush verified | Extracted `feedPaste`/`flushPaste` helpers, string Update handling without bubbletea internals |
| 2.1 sync markers | Same as 1.1 — `TestSyncOutput_MarkersPresent` fail before `syncBegin/syncEnd` → 7 PASS | View wraps all branches, fallback plain | No screens mass-edit, central wrapper |
| 2.2 paste buffer 15 lines + ctrl+c | `TestBracketedPaste_SingleEvent_15Lines` (15 lines single event) fail before buffer → PASS after `bracketedPasteStart/End` buffering; `CtrlCIgnored` ensures no quit | `go test -run TestBracketed` 4 PASS | MultiChunkSplit handles split chunks |
| 2.3 wire screens | Central `Model.View` → `syncOutput(frame)` covers `screens/*.go` via View; verified `TestSyncOutput_ViewWraps` PASS atomic, `ViewFallback` plain | No per-screen edit, opt-in idempotent | Kept screens untouched per “mínimo y opt-in” |
| 2.4 tests | `go test ./internal/tui -count=1` 6 tests baseline → 18 tests after 10 new | Full suite exit 0, 4.045s, `go vet` 0 | Fixture-based, no network, no `go env GOPATH`/`pkg/mod` grep |

## Status

6/6 TUI tasks complete (Phase 1 2/2 + Phase 2 4/4). 10/16 tasks remain (Phase 3 4, Phase 4 5, Phase 5 3 — verify tasks intentionally pending per slice). Next: PR2 hashline (`internal/filemerge/hashline.go`, `correction.go`). No blockers.

### Workload / PR Boundary

- Mode: auto-chain stacked-to-main (budget 800)
- Current work unit: PR1 TUI CSI 2026 + bracketed paste (Unit 1)
- Boundary: `4f6d...` (pre-TUI) → `internal/tui/tui.go` + `internal/tui/tui_test.go` + `tasks.md` + `apply-progress.md`; start `New()` baseline, end `syncOutput`/`PasteMsg` with fixture tests; rollback deletes TUI markers/buffer + reverts checkboxes + deletes progress file, leaves hashline/web/advisor untouched
- Estimated review budget impact: tui.go +~182 net (+205/-23), tui_test.go +255, tasks.md +~8, apply-progress.md new — raw diff 437 lines prod+tests, within 800 budget, single commit, stacked-to-main PR1 targets `master`
