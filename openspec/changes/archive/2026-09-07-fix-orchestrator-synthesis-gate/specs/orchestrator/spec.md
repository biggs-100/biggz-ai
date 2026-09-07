# Delta for orchestrator

## MODIFIED Requirements

### Requirement: REQ-ORCH-001 — Blocking Synthesis Checkpoint (120s)

The system MUST enforce `internal/sdd/synthesis_gate.go`. After EVERY sub-agent (SDD or non-SDD) the orchestrator MUST emit `## Sub-agent Result` with 4 markers (`## Sub-agent Result`, `**What was done:**`|`| Topic | Decision |`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**`) in current turn BEFORE any checkpoint ask. Gate MUST check `HasSynthesis` + `IsCheckpointAsk` (bilingual `proceed|adjust|stop|continue|correct` + `continuar|ajustar|detener|parar|corregir|proseguir|proceder|procede` label-only) + 120s window (`currentTurnMarkdown` via `SetCurrentTurnMarkdown`, `now - currentTurnTime <=120s` else expired). Missing/expired MUST block `isError:true`/`{block:true}` and MUST render via `BuildBlockedEnvelope`/`blockedEnvelope` with `FormatFallback`; `## Session Recall` in `currentTurnMarkdown` bypasses only then. Docs `internal/assets/biggz/biggz-orchestrator*.md` MUST state `A checkpoint ask without immediately preceding ## Sub-agent Result markdown is INVALID and will be blocked` and repeat `REMINDER: synthesis markdown is separate chat markdown emitted FIRST, adjacent, same turn, before tool call` 12×, plus self-check re-reading labels before invoking `question`/`ask_user_choice`.
(Previously: 120s check without INVALID wording, REMINDER, or self-check requirement.)

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

### Requirement: Orchestrator Synthesis Template Invariant

`internal/assets/biggz/biggz-orchestrator.md` MUST keep 4-marker example + `| Topic | Decision |` table + checklist + `◆ Phase · Status · Next` one-line lifecycle placeholders + `INVALID and will be blocked` rule; drift MUST fail `orchestrator.test.go`. `engram` alias MUST equal `bigmem`. Docs MUST retain un-retired blocking language (no `ENFORCEMENT RETIRED` passthrough).
(Previously: 4 markers + INVALID only; no table/lifecycle enforcement and allowed retired wording.)

#### Scenario: Template holds new markers

- GIVEN file read
- WHEN searching
- THEN it MUST contain `## Sub-agent Result`, `| Topic | Decision |`, `- [ ]`, `◆`, and `INVALID`

#### Scenario: Retired wording absent

- GIVEN `biggz-orchestrator.md` and `biggz-synthesis-gate.js` headers read
- WHEN searched for `ENFORCEMENT RETIRED` passthrough comment
- THEN none MUST be present

#### Scenario: Alias invariant preserved

- GIVEN config with `engram`
- WHEN normalized
- THEN it MUST equal `bigmem` and test MUST enforce
