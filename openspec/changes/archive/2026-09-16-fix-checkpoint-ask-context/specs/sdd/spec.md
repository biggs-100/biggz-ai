# Delta for sdd

## MODIFIED Requirements

### Requirement: Synthesis Gate Markers and 120s Window

The system MUST implement `internal/sdd/synthesis_gate.go` `synthesisMarkers[4]` (`## Sub-agent Result`, `**What was done:**`|`| Topic | Decision |`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**`) with `HasSynthesis` (all 4 + table required; markers stay English via `sanitizePlain` whitelist), `HasSessionRecall` (`## Session Recall`), `IsChildBypass` (`PI_SUBAGENT_CHILD==1`), `IsCheckpointAsk` label-only (envelope `label`/`value`/`id`/`name`/`title` bilingual `proceed|adjust|stop|continue|correct` + `continuar|ajustar|detener|parar|corregir|proseguir|proceder|procede`; body text MUST NOT signal), `HasOptions` (2–4 options heuristic), globals `currentTurnMarkdown` + `currentTurnTime` via `SetCurrentTurnMarkdown` reset on `turn_start`/`agent_start`/`tool_execution_end` success, `ShouldBlock(question, md, now)` (`false` if `IsChildBypass||HasSessionRecall(currentTurn)||!IsCheckpointAsk||now-currentTurnTime>120s||HasSynthesis(currentTurnMarkdown)`, else true), `CheckSynthesisPrecondition` wrapping with `synthesis required: missing ## Sub-agent Result with 4 markers in current turn (120s window)`, and `ShouldBlockApplyAdmission` (write-admission that ignores child/recall bypass to prevent auto back-to-back phases). Thin `countPaths<2||len<50` MUST be warn-only via `BIGGZ_ADVISE=1`. `FormatFallback` MUST render fallback for blocked envelope.

For checkpoint asks, envelope validation MUST additionally reject, fail-closed, any option whose `description` does not carry decision context proportional to the decision, with a message naming the offending option and question; a context-bearing description MUST pass, a non-empty but context-free description (e.g. `yes, go ahead`) MUST NOT pass, and non-checkpoint asks MUST remain unaffected. How substance is measured (threshold, required signals, field shape) is design-owned. The system MUST expose a `biggz` CLI surface (the checkpoint-ask check command; verb/name/flags design-owned) that evaluates BOTH the synthesis precondition and the envelope validation including the substance rule, exits `0` when both pass, and exits non-zero with an actionable message naming the failing condition when either fails. It MUST be callable under a bounded timeout so a harness invocation cannot hang the ask path. A check that cannot render a decision (timeout, spawn failure, missing/broken binary, unparsable output) MUST report an indeterminate outcome — never a pass for the synthesis precondition — leaving the caller to degrade-open with a visible notice; a decided non-zero exit MUST block.
(Previously: gate surface with no option-substance rule and no live CLI check making it reachable — `yes, go ahead` passed, and every entry point had test-only call sites.)

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

#### Scenario: Thin option description rejected

- GIVEN checkpoint-ask option description `yes, go ahead` (non-empty, context-free)
- WHEN envelope validation runs
- THEN it MUST reject fail-closed, naming the offending option/question

#### Scenario: Context-bearing option description passes

- GIVEN checkpoint ask with option descriptions carrying problem, scope, effort, risk, and deferral context
- WHEN envelope validation runs
- THEN it MUST pass

#### Scenario: Non-checkpoint ask unaffected

- GIVEN non-checkpoint question with terse option and valid limits
- WHEN the checkpoint-ask check runs
- THEN it MUST exit 0 (no substance rejection outside checkpoint asks)

#### Scenario: Check balances both preconditions

- GIVEN missing synthesis + valid envelope, then valid synthesis + thin option, then both valid
- WHEN the checkpoint-ask check command runs
- THEN it MUST exit non-zero naming `synthesis required` / the offending option, and `0` when both pass

#### Scenario: Check invocation is bounded

- GIVEN the check does not return before the bounded timeout
- WHEN the caller invokes it
- THEN the invocation MUST terminate at the bound and report indeterminate (not a pass, not a hang)

#### Scenario: Indeterminate outcome never a silent pass

- GIVEN the check binary is missing/broken or output unparsable
- WHEN the outcome is consumed
- THEN it MUST surface a visible notice that enforcement was skipped, never a silent allow
