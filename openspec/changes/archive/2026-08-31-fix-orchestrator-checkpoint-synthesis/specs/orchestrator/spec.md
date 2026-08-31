# Delta for orchestrator

## MODIFIED Requirements

### Requirement: Post-Delegation Human Checkpoint Synthesis — Strict Same-Turn Invariant

**ID: REQ-001 / REQ-002**

The system MUST enforce strict checkpoint synthesis: a checkpoint `ask_user_question`/`question` with options `proceed|adjust|stop|continue|correct` MUST be allowed only when `currentTurnMarkdown` contains all 4 markers (`## Sub-agent Result`, `**What was done:**` or `| Topic | Decision |`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**`) and `now - currentTurnTime ≤120s`. History (`ctx.history` / `lastAssistant` / `lastAssistantMarkdown`) MUST NOT satisfy blocking. Table+checklist and one-line `◆ Phase · Status · Next` lifecycle MUST remain, sanitized via `stripAnsi/stripOsc/TruncateToWidth` before `VisibleWidth`. Missing MUST return `isError:true` / `{block:true}` + notify via `pi.notify`/`ctx.ui.notify` and MUST NOT call original handler.

(Previously: prose-only What was done; relaxed fallback allowed history within 120s with warning instead of block; no strict currentTurn-only guarantee.)

#### Scenario: REQ-001 — Block when only history has synthesis (strict currentTurn only)

- GIVEN `currentTurnMarkdown` is empty and `ctx.history`/`lastAssistant` contains 4 markers within 120s
- WHEN `ShouldBlock` / JS gate evaluates checkpoint ask
- THEN MUST block with `isError:true` / `{block:true, reason}` + notify and MUST NOT call handler

#### Scenario: REQ-001 — Block despite history timestamp fresh

- GIVEN checkpoint ask without `## Sub-agent Result` in `currentTurnMarkdown` but with synthesis in `lastAssistantMarkdown` updated 10s ago
- WHEN gate evaluates `checkSynthesisPrecondition` / `ShouldBlock`
- THEN MUST block; history freshness MUST NOT override strict source

#### Scenario: REQ-002 — Allow with fresh synthesis in currentTurn ≤120s

- GIVEN `currentTurnMarkdown` has 4 markers emitted 30s ago and `HasSynthesis(currentTurnMarkdown)==true`
- WHEN checkpoint ask evaluated
- THEN MUST allow and downstream `SavePendingDualWrite` MAY proceed

#### Scenario: REQ-002 — Expired window does not satisfy

- GIVEN `currentTurnMarkdown` has 4 markers but `now - currentTurnTime = 121s`
- WHEN `ShouldBlock` evaluated
- THEN MUST block (window expired) — callers MUST re-emit synthesis same turn

#### Scenario: Param-only synthesis does not count

- GIVEN synthesis embedded only inside `ask_user_question` `question` param with no `currentTurnMarkdown`
- WHEN gate checks
- THEN MUST treat as missing and block with `isError:true`

### Requirement: Single Ownership and Pending Persistence — Dual-Write Contract

**ID: REQ-007**

Only orchestrator SHALL emit checkpoint asks. On allowed checkpoint the orchestrator MUST persist envelope via `SavePendingDualWrite` to `biggz-ai.pending-question/v1` dual-write: BigMem `sdd/{change}/pending-question` and `openspec/changes/{change}/state.yaml` `pending_question`, MUST verify equality with `VerifyEquality` retry once. On blocked checkpoint it MUST NOT persist new pending.

(Previously: dual-write existed but JS relaxed allow bypassed persistence, leaving pending unwritten.)

#### Scenario: REQ-007 — Dual-write on allowed

- GIVEN checkpoint allowed with fresh synthesis
- WHEN orchestrator calls `SavePendingDualWriteAt`
- THEN MUST write BigMem and `state.yaml` and `VerifyEquality` retry-once MUST pass with identical bytes

#### Scenario: REQ-007 — Blocked persists nothing

- GIVEN checkpoint blocked (strict missing)
- WHEN persistence would run
- THEN MUST NOT write BigMem nor `state.yaml` new entry; prior pending unchanged

#### Scenario: REQ-003 — Preflight bypass allows first ask

- GIVEN `getCurrentTurnSynthesis(ctx)==nil` and `getSynthesisSource(ctx)==nil` (no synthesis ever) and no `## Session Recall` yet required
- WHEN first `ask_user_question` evaluated (e.g., SDD Session Preflight)
- THEN MUST allow — not a post-delegation violation

#### Scenario: REQ-005 — Session Recall bypass narrow same-turn

- GIVEN `currentTurnMarkdown` contains `## Session Recall` (boot gate emitted before preflight)
- WHEN checkpoint ask evaluated
- THEN MUST bypass synthesis check and allow; history-only Recall MUST NOT bypass

