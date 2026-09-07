# Delta for sdd-discipline-gates

## MODIFIED Requirements

### Requirement: REQ-DG-1 — Checkpoint-scoped synthesis block

`ShouldBlock` (Go `internal/sdd/synthesis_gate.go`, JS `biggz-synthesis-gate.js`) MUST require `IsCheckpointAsk` scanning ONLY option `label`/`value`/`id`/`name`/`title` with bilingual `proceed|adjust|stop|continue|correct` + `continuar|ajustar|detener|parar|corregir|proseguir|proceder|procede`; body text MUST NOT signal. `HasOptions` alone MUST NOT block. `HasSynthesis` MUST require 4 English markers (`## Sub-agent Result`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**`) plus `**What was done:**` OR `| Topic | Decision |`. `ShouldBlock` MUST be `!IsChildBypass && !HasSessionRecall(currentTurn) && IsCheckpointAsk && (now-currentTurnTime<=120s) && !HasSynthesis(currentTurnMarkdown)` where `currentTurnMarkdown` is STRICT same-turn buffer via `SetCurrentTurnMarkdown` reset on `turn_start`/`agent_start`; history MUST NOT satisfy. Thin `countPaths<2 || len<50` MUST warn only when `BIGGZ_ADVISE=1`, never block. Bypasses: `PI_SUBAGENT_CHILD=1`, same-turn `## Session Recall`, narrow preflight. Go canonical; JS MUST mirror.
(Previously: HasOptions alone never blocks; no same-turn/label-only/thin/bypass.)

#### Scenario: Checkpoint without synthesis blocks

- GIVEN checkpoint ask with no synthesis in `currentTurnMarkdown`
- WHEN `ShouldBlock` evaluates within 120s
- THEN it MUST return true

#### Scenario: History-only still blocks

- GIVEN valid synthesis only in `ctx.history` or expired 121s
- WHEN checkpoint evaluates
- THEN it MUST return true

#### Scenario: Valid same-turn allows

- GIVEN 4 markers + table in `currentTurnMarkdown` 30s ago
- WHEN checkpoint `proceed` evaluates
- THEN it MUST return false

#### Scenario: Turn reset clears

- GIVEN `turn_start` fired after synthesis
- WHEN next checkpoint without new synthesis evaluates
- THEN it MUST return true

#### Scenario: Free-text never blocks

- GIVEN ask with no options or `how are you?` without checkpoint label
- WHEN evaluated without synthesis
- THEN it MUST return false

#### Scenario: Body-token never blocks

- GIVEN token only in question body with neutral labels
- WHEN `IsCheckpointAsk` evaluates
- THEN it MUST return false

#### Scenario: Child and recall bypass

- GIVEN `PI_SUBAGENT_CHILD=1` or `## Session Recall` in currentTurn
- WHEN checkpoint without synthesis evaluates
- THEN it MUST return false

#### Scenario: Thin warns only with advise

- GIVEN synthesis `countPaths<2 || len<50` and `BIGGZ_ADVISE=1`
- WHEN checkpoint evaluates
- THEN it MUST allow and emit `concern: synthesis is thin`; without advise silent allow

#### Scenario: Go/JS parity

- GIVEN same inputs
- WHEN evaluated by Go and JS
- THEN verdicts MUST match

### Requirement: REQ-DG-2 — Blocked-path fallback envelope

On block, Go `BuildBlockedEnvelope` and JS `blockedEnvelope` MUST emit same-turn payload with `context` + full question via `FormatFallback` — nothing swallowed. JS MUST wrap `registerTool` + sweep (`pi.tools`/`pi._tools`/`getAllTools`) for `ask_user_choice`/`ask_user_question`/`question`; `execute` MUST return `{isError:true}`, `tool_call` MUST return `{block:true}` without calling original. Envelope MUST contain `context` and `fallback`.
(Previously: generic fallback without JS isError/block.)

#### Scenario: Blocked emits fallback

- GIVEN checkpoint blocked
- WHEN envelope built
- THEN it MUST contain `context` AND `fallback`

#### Scenario: Fallback preserves question

- GIVEN blocked ask with options
- WHEN `FormatFallback` renders
- THEN all options and prompt MUST be present

#### Scenario: JS execute blocks

- GIVEN checkpoint without synthesis, wrapped tool registered
- WHEN `execute` invoked
- THEN it MUST return `{isError:true}` without calling original

#### Scenario: JS tool_call blocks

- GIVEN checkpoint without synthesis via `tool_call`
- WHEN handler evaluates
- THEN it MUST return `{block:true}`

## ADDED Requirements

### Requirement: REQ-DG-5 — Transcript lint regression

CI lint MUST replay `branch-worktree-cleanup` transcript (8 phases, 2 syntheses) and assert 7 expected blocks when synthesis missing same-turn. Each phase checkpoint without preceding `HasSynthesis` MUST be blocked.

#### Scenario: Missing synthesis transcript fails lint

- GIVEN 8-phase transcript with 6 missing intermediate syntheses
- WHEN lint scans turns for `IsCheckpointAsk` without preceding `HasSynthesis` in same turn
- THEN it MUST report 7 expected blocks

#### Scenario: Complete syntheses pass lint

- GIVEN same 8 phases each preceded by valid 4-marker synthesis in same turn
- WHEN lint scans
- THEN it MUST report 0 missing blocks
