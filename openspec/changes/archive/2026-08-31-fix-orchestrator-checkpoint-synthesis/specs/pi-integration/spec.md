# Delta for pi-integration

## MODIFIED Requirements

### Requirement: Advisor Inline Watchdog Advise Mode — Strict Block, History Only for Advise

**ID: REQ-001 / REQ-004 / REQ-005 / REQ-006**

`biggz-synthesis-gate.js` MUST block checkpoint `ask` only on strict `currentTurnMarkdown` lacking 4 markers within 120s. History / `lastAssistant` MUST be ignored for block. Block MUST be `isError:true` without handler + `pi.notify`/`ctx.ui.notify`. Thin (`Artifacts/Paths` count<2 OR len<50) with markers MUST NOT block; MUST emit `concern: synthesis is thin` only when `BIGGZ_ADVISE=1` (off by default). Advise MAY fallback `currentTurn→history→lastAssistant(120s)` for concern only, never for block. `PI_SUBAGENT_CHILD=1` MUST bypass both modes. General question without checkpoint tokens MUST bypass synthesis check.

(Previously: blocking checked `currentTurn→history→lastAssistant` with `emitHistoryFallbackWarning` allow; thin advise sharing same source. Strict truth is `synthesis_gate.go:ShouldBlock = !child && !recall && checkpoint && ≤120s && !HasSynthesis(currentTurn)`; JS must match.)

#### Scenario: REQ-001 — Strict block on history-only synthesis

- GIVEN `currentTurnMarkdown=""`, `lastAssistant="## Sub-agent Result...**Artifacts/Paths:** ..."` within 120s
- WHEN `wrapSingleTool` / `tool_call` handler evaluates checkpoint `proceed`
- THEN MUST return `{isError:true, text:"Please synthesize before asking"}` / `{block:true}` and NOT call original

#### Scenario: REQ-004 — Child bypass allows always

- GIVEN `process.env.PI_SUBAGENT_CHILD=1`
- WHEN any checkpoint ask evaluated regardless of markdown
- THEN MUST allow immediately; MUST skip block and advise and NOT notify

#### Scenario: REQ-005 — Session Recall preflight bypass (narrow same-turn)

- GIVEN `currentTurnMarkdown` contains `## Session Recall` and no prior synthesis ever
- WHEN checkpoint ask (preflight `proceed/adjust/stop`) evaluated
- THEN MUST allow; MUST NOT require `## Sub-agent Result` for this transition

#### Scenario: REQ-005 — No Recall emitted yet → normal gating without bypass

- GIVEN `## Session Recall` not yet emitted this session but checkpoint is post-delegation
- WHEN gate evaluates `HasSessionRecall` / `checkSessionRecallInCurrentTurn`
- THEN MUST NOT bypass for missing synthesis; MUST apply strict block logic (preflight `anySynthesis==""` is separate allowance)

#### Scenario: REQ-006 — Advise thin with fallback, never blocks

- GIVEN `BIGGZ_ADVISE=1` and synthesis with 4 markers but `count=1 len=10` in `currentTurn` or fallback chain
- WHEN checkpoint ask evaluated
- THEN MUST allow call and emit `concern: synthesis is thin` via `pi.notify` (concern thin, not block)

#### Scenario: REQ-006 — Advise off silent

- GIVEN same thin markdown with `BIGGZ_ADVISE` unset/off
- WHEN checkpoint ask evaluated
- THEN MUST allow without concern or block

#### Scenario: General question bypass

- GIVEN question `¿por dónde empezamos?` without checkpoint tokens
- WHEN gate evaluates
- THEN MUST allow without synthesis regardless of `currentTurn`

## MODIFIED Requirements

### Requirement: Synthesis Gate Verification and CI — Strict Test Contract

**ID: REQ-008**

`biggz-synthesis-gate.test.mjs` MUST cover strict invariant: 5 tests previously expecting `allow with history fallback` MUST now expect `block when only history has synthesis` (`isError:true`, `originalCalled==false`, notify). MUST still cover rich→pass, thin+advise→concern+pass, thin silent without flag, child bypass, preflight `anySynthesis==""` allow, `PI_SUBAGENT_CHILD`, `## Session Recall`. `node --test biggz-synthesis-gate.test.mjs` and `go test ./internal/sdd -run TestHasSynthesis -count=1` MUST PASS in CI.

(Previously: 5 history-fallback allow tests with `emitHistoryFallbackWarning`; now strict block.)

#### Scenario: REQ-008 — Rewritten 5 tests expect block

- GIVEN `currentTurn=""` with synthesis only in `lastAssistant`/`ctx.history` within 120s and checkpoint `proceed`
- WHEN `biggz-synthesis-gate.test.mjs` fixtures run
- THEN 5 rewritten tests MUST assert `isError:true` / `block:true` and `originalCalled==false` and `node --test` MUST exit 0

#### Scenario: REQ-008 — Rich/thin/child/preflight still pass

- GIVEN rich `currentTurn` ≤120s → allow; thin+`BIGGZ_ADVISE=1` → allow+concern; thin without flag → silent allow; `PI_SUBAGENT_CHILD=1` → allow even missing
- WHEN tests run
- THEN all MUST PASS with no fallback-warning path taken

