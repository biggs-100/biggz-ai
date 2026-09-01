# Design: Polish Synthesis Human Language

## Technical Approach

Hybrid A+C. Orchestrator detects via heuristic (diacritics `á/é/í/ó/ú/ñ/¿/¡` + keywords `que,en,por,con,para`; short `hi/ok/dale`→`en`), stores `languageHint` (`pending_question.languageHint`/memory), injects `Human language: es|en — render synthesis content in that language, keep markers English, keep paths/code English` into `sdd-*` prompts, fallback at render (`DetectLanguage(lastHumanMessage)` or `en`). Content inside 5 sections localized; 4 markers + `| Topic | Decision |` + 6 optional stay English so gate `b0d2fc1` never breaks. Paths/`sdd/...`/code whitelisted. Covers PS1-PS5.

## Architecture Decisions

| Decision | Option | Tradeoff | Choice |
|----------|--------|----------|--------|
| D1 Detection | A Heuristic (diacritics+keywords) | Zero deps, 20 LOC, deterministic; weak on 1-word | **A** — Spec PS5 requires it; ambiguous→`en` |
| | B LLM/x/text | Accurate, heavy, latency | Rejected — binary es/en |
| D2 Strategy | A+C Hybrid (hint+fallback) | Native generation + safety net | **A+C** — Proposal rec; handles compaction miss |
| | A Orchestrator-only | Minimal prompt change | Rejected — post-hoc cost |
| | B Prompt-only | Fragile if ignored | Rejected — no fallback |
| D3 API | Wrapper `RenderSynthesisLocalized(r,lang)` → `RenderSynthesis(r)` compat | No break, tests pass | **Wrapper** — preserve gate contract |

## Data Flow

```mermaid
graph LR
  H[Human turn] --> D{detectLanguage es|en}
  D --> S[session.languageHint]
  S --> P[sdd-* prompt injection]
  P --> A[sub-agent executive_summary in hint lang]
  A --> R[RenderSynthesisLocalized]
  R -->|markers English, content localized| G[gate HasSynthesis/isCheckpointAsk]
  G --> U[Human checkpoint]
  R -. fallback .-> F[DetectLanguage last msg else en]
```

```
Human → detectLanguage() → languageHint → prompt hint → sub-agent (localized summary)
                                      ↘ RenderSynthesisLocalized (whitelist, 5 sections) → gate → Human
                                      ↗ fallback DetectLanguage() if hint empty
```

5 sections: 1 Resumen (`| Topic | Decision |`+`◆ Phase · Status · Next`), 2 Decisiones, 3 Evidencia (Preview 300/Diff 80, >50KB paginated), 4 Artefactos, 5 Riesgos/Próximo. Markers English per PS4; content localized, header stays English.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/sdd/synthesis.go` | Modify | Add `DetectLanguage`, `RenderSynthesisLocalized` (keep `RenderSynthesis` compat), whitelist `/`, `sdd/`, `ORDER BY` |
| `internal/sdd/synthesis_test.go` | Modify | Tests es/en, `hi`→en, `¿qué?`→es, whitelist, markers, 5-section |
| `internal/assets/biggz/biggz-orchestrator.md` | Modify | Note: content localized per `languageHint`, markers/tech English invariant |
| `internal/assets/biggz/biggz-orchestrator-workflow.md` | Modify | Add detect→store→inject→fallback step before render |
| `internal/assets/biggz/bigmem-protocol.md` | Modify | Language Boundary note: harness English, synthesis matches human |
| `docs/architecture.md` | Modify | Harness vs artifact boundary note |

No change to `internal/sdd/synthesis_gate.go`, `internal/assets/pi/biggz-synthesis-gate.js` (`b0d2fc1` invariant).

## Interfaces / Contracts

```go
func DetectLanguage(text string) string // "es"|"en"
func RenderSynthesisLocalized(r SubAgentResult, lang string) string
func RenderSynthesis(r SubAgentResult) string // compat → Localized(r,"en")
```

Prompt hint (every `sdd-*` prompt):
```
Human language: es|en — render synthesis content in that language, keep markers English, keep paths/code English
```
Whitelist: never translate strings containing `/`, `sdd/`, `ORDER BY`, `Search`, branch/topic_key patterns (via `sanitizePlain`).

## Testing Strategy

| Layer | What to Test | Approach |
|-------|--------------|----------|
| Unit | DetectLanguage (es/en, ambiguous→en, diacritics), Localized rendering whitelist, markers English, 5 sections, empty→None, >50KB | `go test ./internal/sdd -run TestSynthesis -count=1` + `TestDetectLanguage`/`TestRenderSynthesisLocalized` |
| Integration | Hint stored, prompt injected, pending dual-write carries hint | `go test ./internal/assets/biggz` checks markers still present |
| E2E | Gate blocks missing marker (es content), allows es content+en markers, thin advise lang-agnostic | `node --test internal/assets/pi/biggz-synthesis-gate.test.mjs` |

## Threat Matrix

Standard `references/threat-matrix.md` — no routing/shell/subprocess/VCS/PR/executable boundary:

| Boundary | Applicability | Reason |
|----------|---------------|--------|
| Documentation-like paths | N/A | No executable-MD change |
| Git repository selection | N/A | No `git -C` change |
| Commit state | N/A | No index change |
| Push state | N/A | No push change |
| PR commands | N/A | No PR command change |

Domain threats (propagate to tasks RED):

| Threat | Safe | Failure | RED test |
|--------|------|---------|----------|
| Over-translation | `internal/sdd/synthesis.go`, `sdd/...`, `ORDER BY` verbatim | Translated → lookup fails | es render keeps paths English |
| Marker translation `b0d2fc1` | `## Sub-agent Result:`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**` verbatim | Translated → `HasSynthesis` false → `ShouldBlock` true | es content+en markers → pass; es markers → fail |
| Wrong detection | Short `ok/dale/go` → `en` + fallback re-detect | Wrong locale → still completes (fallback) | `DetectLanguage("ok")==en`, `DetectLanguage("¿qué?")==es` |

## Migration / Rollout

No migration. Defaults `en`; rollback `git revert` synthesis.go + 4 docs; verify `go test ./internal/sdd -run TestSynthesis`, `go vet ./...`, gate tests PASS.

## Open Questions

- [ ] Keep `| Topic | Decision |` English (current) vs localize to `| Tema | Decisión |` (needs gate both) — proposal says English.
- [ ] Replace heuristic with `x/text/language` if third language added — out of scope.
