# Tasks: Polish Synthesis Human Language

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 150-200 (synthesis.go ~60 + tests ~80 + 3 docs ~40) |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | single PR |
| Delivery strategy | auto-chain |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Core detection+render (`DetectLanguage`, `RenderSynthesisLocalized`, whitelist) | PR 1 | `go test ./internal/sdd -run TestDetectLanguage -count=1` | N/A — unit library | `internal/sdd/synthesis.go` + `synthesis_test.go` |
| 2 | Orchestrator injection + gate hardening (`languageHint`, `Human language: es|en` prompt, `b0d2fc1`) | PR 1 | `go test ./internal/assets/biggz -count=1` + `node --test internal/assets/pi/biggz-synthesis-gate.test.mjs` | Manual: `en que nos quedamos?` → Spanish synthesis/markers English | `internal/assets/biggz/*.md` + `bigg s-synthesis-gate.js` |
| 3 | Docs + E2E verify | PR 1 | `go vet ./...` + `go test ./internal/sdd -run TestSynthesis -count=1` | Manual: Spanish vs English turn flips locale, `go test ./...` green | `docs/architecture.md` docs-only |

## Dependency Graph

```
1.1→1.2→1.3→1.4→1.5→1.6→2.1→2.2→2.3→3.1→3.2
```
1.1 blocks 1.3 (wrapper needs detector); 1.3 blocks 1.4-1.6 and 2.1; 2.1 blocks 2.2; 3.x needs 1.x+2.x.

## Phase 1: Core Language Detection + Render

- [x] 1.1 Add `DetectLanguage(text string) string` in `internal/sdd/synthesis.go` (diacritics `á/é/í/ó/ú/ñ/¿/¡`→es, keywords `que/en/por/con/para` vs `hello/continue/proceed`, short `hi/ok/go/dale`→en).
- [x] 1.2 RED Wrong detection — failing `TestDetectLanguage_Red`: `DetectLanguage("ok")==en` and `DetectLanguage("¿qué?")==es` must fail before 1.1.
- [x] 1.3 Add `RenderSynthesisLocalized(r SubAgentResult, lang string)` in `internal/sdd/synthesis.go` — 5 sections localized, 4 markers verbatim `## Sub-agent Result:`/`**Artifacts/Paths:**`/`**Risks / Open Questions:**`/`**Next Recommended:**`+` | Topic | Decision |`, whitelist `sdd/`, `/`, `ORDER BY`; fallback `DetectLanguage` if `lang==""`; keep `RenderSynthesis` compat.
- [x] 1.4 RED Over-translation PS3 — failing `TestRender_OverTranslation` with `lang=es`, paths `internal/sdd/synthesis.go` + `sdd/...` + `ORDER BY` verbatim.
- [x] 1.5 RED Marker invariant `b0d2fc1` PS4 — failing `TestRender_MarkerInvariant`: es content+en markers `HasSynthesis==true`, translated markers `HasSynthesis==false`/`ShouldBlock==true`.
- [x] 1.6 GREEN `internal/sdd/synthesis_test.go` `TestRenderSynthesisLocalized` — es/en/mixed last-turn-wins, whitelist, empty→`None`, `hi→en`, 5-section order; `go test ./internal/sdd -run TestSynthesis -count=1` PASS.

## Phase 2: Orchestrator Prompt Injection + Gate Hardening

- [x] 2.1 Update `internal/assets/biggz/biggz-orchestrator-workflow.md` — detect at Session Recall/pre-decision, store `languageHint` (`pending_question.languageHint`+BigMem `sdd/{change}/pending-question`), inject `Human language: es|en — render synthesis content in that language, keep markers English, keep paths/code English` into `sdd-*` prompts.
- [x] 2.2 Update `internal/assets/biggz/biggz-orchestrator.md` + `bigmem-protocol.md` — Language Boundary note: harness English, synthesis content localized per `languageHint`, markers/tech English `b0d2fc1`; verify `go test ./internal/assets/biggz`.
- [x] 2.3 Verify `internal/assets/pi/biggz-synthesis-gate.js` `isCheckpointAsk` still scans labels not content — Spanish content+English markers passes, missing marker blocks; `node --test internal/assets/pi/biggz-synthesis-gate.test.mjs` 22/22.

## Phase 3: Docs + Verification

- [x] 3.1 Update `docs/architecture.md` BigMem/synthesis section — harness vs artifact boundary note; `go vet ./...` clean.
- [x] 3.2 Final E2E — manual Spanish `en que nos quedamos?`→Spanish, English→English; automated `go test ./internal/sdd -run TestSynthesis`, `node --test biggz-synthesis-gate.test.mjs`, `go vet ./...` PASS.
