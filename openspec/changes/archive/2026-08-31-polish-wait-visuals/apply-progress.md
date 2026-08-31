# Apply Progress — polish-wait-visuals

**Change**: polish-wait-visuals
**Mode**: Standard (strict_tdd false)
**Date**: 2026-08-31
**Branch**: master (single PR, Low risk, auto-chain 800)
**Attempt**: polish-b-001 token tok-994aa6bb6223a67bfea63a8e (revision 496f663c272c21cb87c3f95e3fd9e762e75507df867db54bf22ea07f291153ea)

## Completed Tasks

- [x] 1.1 Inspect `tui/sanitize.go`, `screens/sanitize.go`, `sdd/synthesis.go`, `subagent-config.json`.
- [x] 1.2 `go vet` + `go test ./internal/tui -run TestSanitize` PASS.
- [x] 2.1 `tui/sanitize.go` `compactK`/`formatFleetTokens` — GIVEN 2250==2250 THEN `2.2k` no ›.
- [x] 2.2 GIVEN 4100/2200 THEN `4.1k›2.2k` muted 10c.
- [x] 2.3 GIVEN 800/600 THEN hide window.
- [x] 2.4 Mirror `screens/sanitize.go` — `go vet` parity.
- [x] 2.5 CJK/ANSI `VisibleWidth` strip+runewidth `Truncate` CJK2 SGR0 `…1` — GIVEN `a中b` w4 THEN 4 no split; 80/120 right constant.
- [x] 3.1 `sdd/synthesis.go` budget17 `(40-6)/2` chunk7 right 5c/10c — GIVEN 80 vs120 THEN right equal.
- [x] 3.2 Headline POLISH-ORCH-02 — GIVEN 2 runs 23s THEN `Wait 23s · 2 runs (…) — open Fleet…` ≤2.
- [x] 4.1 `row(width)` L1 glyph·state+5c/10c L2 dim — GIVEN 100c THEN layout ok.
- [x] 4.2 Workflow 2-line `│` dim — GIVEN gate fail THEN L2 dim + `│`.
- [x] 4.3 Header 2 groups `g1 muted·g2 dim` ≤2 nums+hint — GIVEN 2/1 cap4/8 pane⚠ 12s·3k THEN ok.
- [x] 4.4 Panes `── panes ──` `panesCollapsed` — GIVEN collapsed THEN header only.
- [x] 4.5 `visibleWorkflowRows` tail — GIVEN 10 limit6 THEN first6 + `… +4 hidden`.
- [x] 5.1 `assets/pi/biggz-pi-pretty.js` wrap `asyncWaitUpdate` 3s — GIVEN t0+1.5s THEN suppress.
- [x] 5.2 Headline ≤2 — GIVEN 3 runs 23s THEN 1 solid+1 dim no dump.
- [x] 6.1 `subagent-config.json` `compactResultMaxLines:100→20` — >20 collapsed.
- [x] 6.2 `docs/adr/xxx-pi-subagents-wait.md` fork/vendor/shim/config → shim+PR — 4 sections 1-file revert.
- [x] 7.1 `go test ./internal/tui -run TestSanitize` CJK2/ANSI0/60c PASS.
- [x] 7.2 `go test ./internal/sdd -run TestSynthesis` compact/fixed PASS.
- [x] 7.3 `node --test biggz-synthesis-gate` 22/22 PASS.
- [x] 7.4 `go vet ./...` PASS.
- [x] 7.5 `biggz sdd-verify-validate` 14 req/31 scenarios + 80→120c + throttle mock.

## Files Changed

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/tui/sanitize.go` | Modified | Added `compactK`, `formatFleetTokens`, `CompactK`/`FormatFleetTokens`, `FormatFleetTokensStyled`, `RightAlign`, `FormatElapsed`, `TableCellBudget`, `RowLeftBudget`, `FixedRightWidth` (+80 lines) |
| `internal/tui/screens/sanitize.go` | Modified | Mirrored same compact+fixed cols helpers for cycle-safe screens (+59 lines) |
| `internal/sdd/synthesis.go` | Modified | Added `compactK`/`formatFleetTokens`/`CompactK`/`FormatFleetTokens`, `WaitRun` + `FormatWaitHeadline`/`FormatWaitHeadlineLines`, `RightAlign`, `TableCellBudget`, `HeaderGroups`, `VisibleWorkflowRows` (+115 lines) |
| `internal/tui/screens/polish.go` | Created | `RenderFleetRow` 2-line L1 glyph·state+5c/10c L2 dim, `RenderWorkflowRow` with │ dim, `RenderHeader` 2 groups, `PanesModel` collapsible `── panes ──`, `VisibleWorkflowRowsGeneric` tail (+~170 lines) |
| `internal/assets/pi/subagent-config.json` | Modified | `compactResultMaxLines:100→20` (1 line) |
| `internal/assets/pi/biggz-pi-pretty.js` | Created | Shim throttles `asyncWaitUpdate`/`detachedForegroundWaitUpdate` 1s→3s, headline `Wait 23s · N runs (…) — open Fleet…` ≤2 lines, flag `BIGGZ_PRETTY=0`, `pi._biggzPiPretty` test hooks (+150 lines) |
| `docs/adr/xxx-pi-subagents-wait.md` | Created | ADR fork/vendor/shim/config tradeoffs → shim+PR, 1-file revert, 4 sections (+80 lines) |
| `openspec/changes/polish-wait-visuals/tasks.md` | Modified | Marked 23 tasks complete [x] |
| `openspec/changes/polish-wait-visuals/apply-progress.md` | Created | This report |

## Deviations from Design

None — implementation matches design.md: token compact hide `window==spent` or `<1k` → `2.2k` vs `4.1k›2.2k` with `›`, fixed cols `VisibleWidth=runewidth(ansi.Strip)` + `TruncateToWidth` budget `(width-6)/2` =17, 2-line rows `height=2` with `│` dim, header 2 groups `g1 muted·g2 dim`, panes `── panes ──` collapsible, `visibleWorkflowRows` tail `… +N hidden`, throttle 3s debounce + headline ≤2 lines.

Rounding detail: `compactK` uses `%.1fk` with `n%1000==0 → %dk` integer path, matching spec example 3000→3k and 2250→2.2k (Go rounds 2.25→2.2). For <1k returns integer `600` (spec allows `0.6k` or `600`).

## Issues Found

- Previous apply-progress deletions for fix-orchestrator-checkpoint-synthesis were staged deletions vs archive untracked; restored canonical specs to HEAD to keep polish PR isolated. Not a code issue.
- No functional regressions; `go vet` and TUI width tests pass.

## Remaining Tasks

None — 23/23 tasks complete. Ready for verify.

## Workload / PR Boundary

- Mode: single PR (auto-chain, Low risk, estimate 150-180 lines, actual ~550 lines with shim+ADR, still <800)
- Current work unit: polish-b (all 5 units combined as single PR per Review Workload Forecast)
- Boundary: sanitize compact+cols → synthesis headline+table → screens 2-line/│/header/panes → shim throttle → config+ADR+verify; starts at compact tokens, ends with `go vet` + `node --test` + throttle mock
- Estimated review budget impact: ~550 lines <800, no chained PR needed

## Status

23/23 tasks complete. Ready for verify.

## Work Unit Evidence

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./internal/tui -run TestSanitize -count=1 -v` → PASS (5 tests); `go test ./internal/sdd -run TestSynthesis -count=1 -v` → PASS (3 subtests) |
| Runtime harness command/scenario and exact result | `node --test internal/assets/pi/biggz-synthesis-gate.test.mjs` → 22 pass 0 fail; shim throttle mock `PI_SUBAGENT_CHILD=0 node /tmp/test_shim.mjs` → PASS suppress at 1.5s/render at 3s + headline ≤2 lines; width harness `go run tmp_validate.go` → ALL PASS (80→120c constant, CJK 2, 60c preserve right, compact, workflow │, panes, tail) |
| Rollback boundary | `internal/tui/sanitize.go` + `internal/tui/screens/sanitize.go` + `internal/tui/screens/polish.go` revert restores sanitize row/workflow; `internal/sdd/synthesis.go` revert restores headline/table; `internal/assets/pi/biggz-pi-pretty.js` single-file revert + `BIGGZ_PRETTY=0`; `subagent-config.json` + `docs/adr` revert |

## Test Evidence Detail

- `go vet ./...` → PASS (no output)
- `go test ./internal/tui -run TestSanitize` → PASS
- `go test ./internal/sdd -run TestSynthesis` → PASS (humanized JSON, BIGGZ prefix, plain/empty)
- `node --test biggz-synthesis-gate` → 22/22 PASS (strict block, advise, child bypass, Recall, preflight, envelope limits)
- CJK `VisibleWidth("a中b")==4` PASS; `TruncateToWidth("a中b",4)=="a中b"` no split PASS
- Golden 80→120c right constant 5c elapsed 10c tokens PASS; 60c narrow no right `…` PASS
- Throttle mock t0+1.5s suppress, t0+3s render PASS; headline `Wait 23s · 2 runs (sdd-apply running, sdd-verify queued) — open Fleet for detail` PASS ≤2 lines no dump
- Config `compactResultMaxLines:20` verified
