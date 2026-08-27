# Delta for orchestrator

## ADDED Requirements

### Requirement: Post-Delegation Human Checkpoint Synthesis

The orchestrator MUST emit synthesis markdown as separate chat markdown, FIRST adjacent same-turn BEFORE every `ask_user_question`/`question` call. The block MUST contain 4 markers: `## Sub-agent Result: {phase/agent}`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**`. Content MUST be derived from sub-agent `executive_summary`, `artifacts`, `risks`, `next_recommended`. Any `ask_user_question`/`question` without immediately preceding markdown MUST be INVALID and blocked (`isError:true`). Synthesis inside tool `question` param MUST NOT satisfy the check. The asset `internal/assets/biggz/biggz-orchestrator.md` MUST contain a copy-paste example block and state `INVALID and will be blocked` same-turn rule with `REMINDER: synthesis markdown is separate chat markdown emitted FIRST...` after every ask reference.

#### Scenario: Full synthesis before ask passes

- GIVEN sub-agent returned summary, artifacts (≥2 paths, ≥50 chars), risks, next
- WHEN orchestrator emits markdown with 4 markers then calls `ask_user_question` same turn
- THEN gate MUST allow the call and human decision flow MUST proceed

#### Scenario: Missing synthesis is INVALID and blocked

- GIVEN current turn has no `## Sub-agent Result` markdown
- WHEN orchestrator calls `ask_user_question` or `question`
- THEN gate MUST return `{isError:true, text:"Please synthesize before asking"}` and MUST NOT invoke original handler

#### Scenario: Synthesis inside tool param does not count

- GIVEN orchestrator embeds synthesis only inside `ask_user_question` `question` param
- WHEN gate checks `currentTurnMarkdown` → `ctx.history` → `lastAssistant` (120s)
- THEN it MUST treat as missing and block with `isError:true`

#### Scenario: Thin synthesis satisfies orchestrator checkpoint

- GIVEN markdown has 4 markers but `Artifacts/Paths: -` (1 path, <50 chars)
- WHEN orchestrator calls `ask`
- THEN orchestrator checkpoint MUST consider it present (pass); thin handling is pi-integration advise concern, not a block

### Requirement: Orchestrator Synthesis Template Invariant

The file `internal/assets/biggz/biggz-orchestrator.md` MUST keep the mandatory example block and same-turn rule convergent with the gate; prompt drift that removes markers or `INVALID` rule MUST be detectable by integration test.

#### Scenario: Template contains example and INVALID rule

- GIVEN `biggz-orchestrator.md` is read
- WHEN searching for the example block
- THEN it MUST contain `## Sub-agent Result: {phase/agent}` and `**Artifacts/Paths:**` and phrase `INVALID and will be blocked`

#### Scenario: Integration test guards drift

- GIVEN orchestrator template is changed to remove `REMINDER` or marker
- WHEN `internal/assets/biggz/orchestrator.test.go` runs
- THEN it MUST fail asserting synthesis before question
