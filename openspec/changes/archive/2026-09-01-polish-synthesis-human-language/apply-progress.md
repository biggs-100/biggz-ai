# Apply Progress: polish-synthesis-human-language

**Change**: polish-synthesis-human-language
**Mode**: Standard (strict_tdd false)
**Date**: 2026-09-01
**Workload**: single PR, 347 insertions across 7 files (within 400 budget, Low risk, auto-chain)

## Completed Tasks
- [x] 1.1 Add `DetectLanguage(text string) string` in `internal/sdd/synthesis.go` (diacritics + keywords, short ambiguous -> en)
- [x] 1.2 RED Wrong detection — `TestDetectLanguage_Red` (ok->en, ¿qué?->es)
- [x] 1.3 Add `RenderSynthesisLocalized(r SubAgentResult, lang string)` — 5 sections, 4 markers verbatim, whitelist, fallback en
- [x] 1.4 RED Over-translation PS3 — `TestRender_OverTranslation` (paths verbatim in es)
- [x] 1.5 RED Marker invariant `b0d2fc1` — `TestRender_MarkerInvariant` (HasSynthesis/ShouldBlock)
- [x] 1.6 GREEN `TestRenderSynthesisLocalized` — es/en/mixed, whitelist, empty->None, hi->en, 5-section order
- [x] 2.1 Update `internal/assets/biggz/biggz-orchestrator-workflow.md` — languageHint detection/store/inject
- [x] 2.2 Update `internal/assets/biggz/biggz-orchestrator.md` + `bigmem-protocol.md` — Language Boundary note
- [x] 2.3 Verify `internal/assets/pi/biggz-synthesis-gate.js` — isCheckpointAsk scans labels not content (22/22)
- [x] 3.1 Update `docs/architecture.md` — harness vs artifact boundary note
- [x] 3.2 Final E2E — manual Spanish/English flip, automated tests PASS

## Files Changed
| File | Action | What Was Done |
|------|--------|---------------|
| `internal/sdd/synthesis.go` | Modified | Added `DetectLanguage` (heuristic diacritics + keywords, ambiguous short -> en), `RenderSynthesisLocalized` wrapper (markers English, whitelist, fallback en), helpers `extractWordsLower`, `isAmbiguousShort`, keyword maps |
| `internal/sdd/synthesis_test.go` | Modified | Added `TestDetectLanguage`, `TestDetectLanguage_Red`, `TestRender_OverTranslation`, `TestRender_MarkerInvariant`, `TestRenderSynthesisLocalized` (5 sub-tests) |
| `internal/assets/biggz/biggz-orchestrator.md` | Modified | Extended Language Boundary with synthesis localization note: harness English, content per languageHint, markers/tech English b0d2fc1, whitelist, fallback |
| `internal/assets/biggz/biggz-orchestrator-workflow.md` | Modified | Added Human Language Detection — languageHint section (detect/store/inject/fallback, gate invariant) before Preflight |
| `internal/assets/biggz/bigmem-protocol.md` | Modified | Added Language Boundary section (detect/store/inject, markers English, whitelist) |
| `internal/assets/pi/biggz-synthesis-gate.js` | Modified | Added language-aware comment above isCheckpointAsk: content localized, markers English, scans labels only |
| `docs/architecture.md` | Modified | Updated package map for synthesis.go, added Language Boundary harness vs artifact paragraph in Synthesis Gate section |
| `openspec/changes/polish-synthesis-human-language/tasks.md` | Modified | Marked all 11 tasks [x] |
| `openspec/changes/polish-synthesis-human-language/apply-progress.md` | Created | This file |

## Deviations from Design
None — implementation matches design (hybrid heuristic + languageHint + RenderSynthesisLocalized wrapper, 5-section order, whitelist, b0d2fc1 invariant). `DetectLanguage` uses diacritics + keyword counts + short ambiguous handling exactly as spec PS5; `RenderSynthesisLocalized` delegates to `RenderSynthesis` preserving markers/whitelist, fulfilling hybrid A+C with prompt hint + fallback.

## Issues Found
- `ShouldBlock` tests must run with `PI_SUBAGENT_CHILD=0` (sub-agent bypass otherwise always false). Fixed via `t.Setenv("PI_SUBAGENT_CHILD","0")` in marker invariant test.
- Pre-existing dirty files in working directory (`cmd/biggz/cli_bigmem.go`, `cmd/biggz/cli_doctor_help.go`, `cmd/biggz/main.go`, `openspec/specs/*`, `openspec/changes/archive/2026-09-01-fix-bigmem-recall-recency/`) are unrelated to this change (from prior fix-bigmem-recall-recency) and not part of this PR's 347-line budget.

## Work Unit Evidence

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./internal/sdd -run TestDetectLanguage -count=1 -v` PASS (19 cases); `go test ./internal/sdd -run TestRender_OverTranslation -count=1 -v` PASS; `go test ./internal/sdd -run TestRender_MarkerInvariant -count=1 -v` PASS (with PI_SUBAGENT_CHILD=0); `go test ./internal/sdd -run TestRenderSynthesisLocalized -count=1 -v` PASS (6 sub-tests); full `go test ./internal/sdd -count=1` PASS (8.1s, all tests PASS) |
| Focused test command (orchestrator) | `go test ./internal/assets/biggz -count=1 -v` PASS (TestOrchestrator* suites PASS, 69 lines orchestrator, 323 workflow); `node --test internal/assets/pi/biggz-synthesis-gate.test.mjs` 22/22 PASS |
| Runtime harness command/scenario and exact result | Manual: `DetectLanguage("en que nos quedamos?")==es` -> synthesis Spanish (markers English preserved); `DetectLanguage("ok, continue")==en` -> English; `DetectLanguage("hi")==en`, `DetectLanguage("dale")==en` (short ambiguous -> en), `DetectLanguage("¿qué?")==es` (diacritic). `RenderSynthesisLocalized(r,"es")` keeps `internal/sdd/synthesis.go` + `sdd/...` + `ORDER BY` verbatim, HasSynthesis true, ShouldBlock false for correct markers, true for translated markers (verified with t.Setenv). `go vet ./...` PASS (0), `go test ./internal/assets/biggz` PASS, `node --check` PASS |
| Rollback boundary | Exact files/behavior that can be reverted without removing unrelated work: `internal/sdd/synthesis.go` + `internal/sdd/synthesis_test.go` (core detection/render), `internal/assets/biggz/biggz-orchestrator.md` + `biggz-orchestrator-workflow.md` + `bigmem-protocol.md` (prompt injection docs), `internal/assets/pi/biggz-synthesis-gate.js` (comment-only), `docs/architecture.md` (docs-only), `openspec/changes/polish-synthesis-human-language/tasks.md` + `apply-progress.md` (planning artifacts). `git revert` or `git checkout --` on these 7+2 files restores pre-change state; no migration, no ledger token consumed (edit authority OK, allowed roots C:/Users/USER/Desktop/biggz-ai), no staged files. |

## Workload / PR Boundary
- Mode: single PR (forecast 150-200 Low, actual 347 insertions across 7 tracked files within 400 budget, no chaining needed)
- Current work unit: All 3 WUs (Core + Orchestrator + Docs/E2E) in one PR slice
- Boundary: `internal/sdd/synthesis.go` + `synthesis_test.go` + `internal/assets/biggz/*.md` + `bigg s-synthesis-gate.js` + `docs/architecture.md` + `openspec/changes/polish-synthesis-human-language/{tasks.md,apply-progress.md}`
- Estimated review budget impact: 347 changed lines (including 171 test lines), ~113 lines pre-existing dirty unrelated excluded, under 400 budget — no size:exception needed, auto-chain pending not required

## Status
11/11 tasks complete. Ready for verify.

## Manual Verification
- Simulated Spanish human turn `en que nos quedamos?` -> `DetectLanguage()==es` -> sub-agent hint `Human language: es — render synthesis content in that language...` -> `RenderSynthesisLocalized(r,"es")` retains English markers (`## Sub-agent Result:`, `**Artifacts/Paths:**`, etc.) and Spanish content (e.g., `Risks: ninguno` preserved), gate `HasSynthesis==true`, `isCheckpointAsk("continuar")==true` (allows), `isCheckpointAsk` does not scan content.
- Simulated English turn `ok, continue` -> `DetectLanguage()==en` -> English synthesis, markers English, paths English.
- Automated: `go test ./internal/sdd -run TestSynthesis -count=1 -v` PASS, `go vet ./...` PASS, `node --test biggz-synthesis-gate.test.mjs` 22/22 PASS, `go test ./internal/assets/biggz -count=1` PASS.
