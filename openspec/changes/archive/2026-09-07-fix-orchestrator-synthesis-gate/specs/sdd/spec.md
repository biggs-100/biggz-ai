# Delta for sdd

## MODIFIED Requirements

### Requirement: Synthesis Gate Markers and 120s Window

The system MUST implement `internal/sdd/synthesis_gate.go` `synthesisMarkers[4]` (`## Sub-agent Result`, `**What was done:**`|`| Topic | Decision |`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**`) with `HasSynthesis` (all 4 + table required; markers stay English via `sanitizePlain` whitelist), `HasSessionRecall` (`## Session Recall`), `IsChildBypass` (`PI_SUBAGENT_CHILD==1`), `IsCheckpointAsk` label-only (envelope `label`/`value`/`id`/`name`/`title` bilingual `proceed|adjust|stop|continue|correct` + `continuar|ajustar|detener|parar|corregir|proseguir|proceder|procede`; body text MUST NOT signal), `HasOptions` (2–4 options heuristic), globals `currentTurnMarkdown` + `currentTurnTime` via `SetCurrentTurnMarkdown` reset on `turn_start`/`agent_start`/`tool_execution_end` success, `ShouldBlock(question, md, now)` (`false` if `IsChildBypass||HasSessionRecall(currentTurn)||!IsCheckpointAsk||now-currentTurnTime>120s||HasSynthesis(currentTurnMarkdown)`, else true), `CheckSynthesisPrecondition` wrapping with `synthesis required: missing ## Sub-agent Result with 4 markers in current turn (120s window)`, and `ShouldBlockApplyAdmission` (write-admission that ignores child/recall bypass to prevent auto back-to-back phases). Thin `countPaths<2||len<50` MUST be warn-only via `BIGGZ_ADVISE=1`. `FormatFallback` MUST render fallback for blocked envelope.
(Previously: 4 markers without Risks/table distinction, IsCheckpointAsk body-scan, no HasOptions/ApplyAdmission/thin/same-turn reset.)

#### Scenario: HasSynthesis requires 4 markers plus table

- GIVEN markdown with all 4 plus `| Topic | Decision |` versus missing `**Artifacts/Paths:**`
- WHEN `HasSynthesis` evaluated
- THEN first MUST be true, second false

#### Scenario: IsCheckpointAsk label-only not body

- GIVEN envelope with checkpoint token only in `question` body versus in `options[].label`
- WHEN `IsCheckpointAsk` evaluated
- THEN body-only MUST be false, label MUST be true

#### Scenario: HasOptions alone never blocks

- GIVEN `HasOptions` true but `IsCheckpointAsk` false
- WHEN `ShouldBlock` evaluates without synthesis
- THEN it MUST return false

#### Scenario: Checkpoint detection and 120s window expiry

- GIVEN `proceed` label and `SetCurrentTurnMarkdown` then `now=+30s` versus `+121s` without synthesis
- WHEN `ShouldBlock` called
- THEN 30s MUST block true, 121s MUST allow false

#### Scenario: Strict currentTurnMarkdown and turn_start reset

- GIVEN synthesis only in history or after `turn_start` reset without new synthesis
- WHEN checkpoint evaluates
- THEN it MUST return true

#### Scenario: Child, recall, and preflight bypass

- GIVEN `PI_SUBAGENT_CHILD=1` or `## Session Recall` in currentTurn or preflight with no prior synthesis ever
- WHEN checkpoint without synthesis evaluates
- THEN it MUST return false

#### Scenario: Thin synthesis warn-only with advise

- GIVEN synthesis with `countPaths=1` or `len<50` and markers present
- WHEN evaluated with `BIGGZ_ADVISE=1` versus unset
- THEN both MUST allow; advise MUST emit `pi.notify` concern, unset MUST be silent

#### Scenario: CheckSynthesisPrecondition message

- GIVEN `ShouldBlock` true
- WHEN `CheckSynthesisPrecondition` called
- THEN it MUST return `(false, "synthesis required: missing ## Sub-agent Result with 4 markers in current turn (120s window)")`

#### Scenario: ApplyAdmission ignores bypass

- GIVEN child bypass active and valid write admission check with missing synthesis
- WHEN `ShouldBlockApplyAdmission` evaluates
- THEN it MUST return true (child/recall ignored for admission)

