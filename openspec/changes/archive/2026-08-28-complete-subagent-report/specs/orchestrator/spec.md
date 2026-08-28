# Delta for orchestrator

## MODIFIED Requirements

### Requirement: Post-Delegation Human Checkpoint Synthesis

MUST emit chat FIRST same-turn BEFORE checkpoint `ask`. MUST contain 4 markers: `## Sub-agent Result`, `**What was done:**`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**`. SHOULD add 6 optional: Preview, Diff, Decisions, Commands, Validation, Failure (bullets; empty omitted). On failure MUST render summary not raw JSON. On truncation >50KB MUST loop re-reads, verify length. Missing MUST be `isError:true`; param-only MUST NOT count. `hasSynthesis` stays pass.
(Previously: 4 markers only, raw JSON, no loop.)

#### Scenario: Full passes

- GIVEN summary, artifacts ≥2 ≥50 chars, risks, next
- WHEN 4 markers then checkpoint ask same turn
- THEN MUST allow

#### Scenario: Missing blocked

- GIVEN no `## Sub-agent Result` in current turn
- WHEN checkpoint ask
- THEN MUST `isError:true`

#### Scenario: Failure and truncated handled

- GIVEN `blocked` failure JSON and artifact >50KB truncated
- WHEN synthesized
- THEN MUST show human Failure summary and loop re-read to full length

### Requirement: Orchestrator Synthesis Template Invariant

`biggz-orchestrator.md` MUST keep 4-marker example + rich placeholders + `INVALID and will be blocked` rule; drift MUST fail `orchestrator.test.go`. `engram` alias MUST equal `bigmem`.
(Previously: 4 markers + INVALID only.)

#### Scenario: Template holds markers

- GIVEN file read
- WHEN searching
- THEN MUST contain `## Sub-agent Result`, `**Artifacts/Paths:**`, `**Preview:**`, `INVALID`

## ADDED Requirements

### Requirement: Single Ownership and Pending Persistence

Only orchestrator SHALL emit checkpoint asks; sub-agents/Pi MUST NOT. MUST persist envelope to `biggz-ai.pending-question/v1` via dual-write BigMem + `state.yaml` with fallback; MUST verify equality (retry once). On compaction MUST reload and emit fallback if UI unavailable.

#### Scenario: Ownership enforced

- GIVEN sub-agent tries checkpoint ask
- WHEN calling `ask_user_question`
- THEN MUST be blocked

#### Scenario: Dual-write and fallback

- GIVEN pending persisted then compacted
- WHEN readback and resumed
- THEN stores MUST have identical bytes and MUST re-emit full envelope