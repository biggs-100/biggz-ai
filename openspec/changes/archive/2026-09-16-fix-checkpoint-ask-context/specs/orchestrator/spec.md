# Delta for orchestrator

## MODIFIED Requirements

### Requirement: REQ-ORCH-001 — Blocking Synthesis Checkpoint (120s)

The system MUST enforce `internal/sdd/synthesis_gate.go`. The orchestrator MUST emit `## Sub-agent Result` with 4 markers (`## Sub-agent Result`, `**What was done:**`|`| Topic | Decision |`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**`) in current turn BEFORE any checkpoint ask. Checkpoints are reserved for irreversible actions; routine post-delegation reports stay quiet (one line). Gate MUST check `HasSynthesis` + `IsCheckpointAsk` (bilingual `proceed|adjust|stop|continue|correct` + `continuar|ajustar|detener|parar|corregir|proseguir|proceder|procede` label-only) + 120s window (`currentTurnMarkdown` via `SetCurrentTurnMarkdown`, `now - currentTurnTime <=120s` else expired). Missing/expired MUST block `isError:true`/`{block:true}` and MUST render via `BuildBlockedEnvelope`/`blockedEnvelope` with `FormatFallback`; `## Session Recall` in `currentTurnMarkdown` bypasses only then. Docs `internal/assets/biggz/biggz-orchestrator*.md` MUST state `A checkpoint ask without immediately preceding ## Sub-agent Result markdown is INVALID and will be blocked` and repeat `REMINDER: synthesis markdown is separate chat markdown emitted FIRST, adjacent, same turn, before tool call` 12×, plus self-check re-reading labels before invoking `question`/`ask_user_choice`.

The enforcement path MUST be the deployed ask tool invoking the checkpoint-ask check command — not test-only Go entry points. `internal/assets/biggz/biggz-orchestrator-workflow.md`'s `## Visible Context Before Every Question (MANDATORY)` section MUST carry issue #14's four items: (1) evidence found and how it was verified; (2) per option problem with repo evidence, scope (files/areas), effort estimate, risks, what it unlocks, and deferral cost; (3) a recommendation with its reason stated plainly enough to disagree with; (4) a no-go condition — when that context cannot be produced yet, do more research instead of asking early. `docs/architecture.md` enforcement claims MUST be true or explicitly honest: every enforcement claim MUST name the deployed invocation path, and no claim may assert blocking by a component nothing invokes.
(Previously: 120s check without INVALID wording, REMINDER, or self-check requirement; enforcement existed only as unwired Go entry points.)

#### Scenario: Synthesis within window allows

- GIVEN 4 markers + table emitted 30s ago in `currentTurnMarkdown`
- WHEN `ask_user_choice` with `proceed` evaluates
- THEN `ShouldBlock` MUST be `false`

#### Scenario: Missing or expired blocks with fallback

- GIVEN no synthesis or `now - currentTurnTime = 121s`
- WHEN checkpoint ask evaluates
- THEN `ShouldBlock` MUST be `true` and handler MUST return fallback envelope, NOT call original

#### Scenario: Non-checkpoint never blocks

- GIVEN no synthesis, question `how are you?` with no checkpoint label
- WHEN evaluated
- THEN it MUST be `false`

#### Scenario: Template markers and INVALID present in docs

- GIVEN `biggz-orchestrator.md` read as file
- WHEN searched
- THEN it MUST contain `## Sub-agent Result`, `| Topic | Decision |`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**`, and `INVALID and will be blocked`

#### Scenario: REMINDER convergence and self-check

- GIVEN orchestrator prepares checkpoint after delegation
- WHEN counting doc occurrences and launch prompt
- THEN `REMINDER: synthesis markdown is separate chat markdown emitted FIRST` MUST appear ≥12 times and prompt MUST evidence label re-read self-check

#### Scenario: Docs carry the four context items

- GIVEN `biggz-orchestrator-workflow.md` read as file
- WHEN `## Visible Context Before Every Question (MANDATORY)` is inspected
- THEN it MUST require evidence, per-option problem/scope/effort/risk/deferral, recommendation-with-reason, and research-more over asking early (asserted via `orchestrator_test.go`)

#### Scenario: Enforcement claims match the live path

- GIVEN `docs/architecture.md` and the deployed install list
- WHEN enforcement claims are searched
- THEN any claim MUST name the deployed ask tool → check command path, and MUST NOT assert enforcement by unwired components or the non-deployed `biggz-synthesis-gate.js`
