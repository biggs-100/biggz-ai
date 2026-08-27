# Delta for pi-integration

## MODIFIED Requirements

### Requirement: Advisor Inline Watchdog Advise Mode

The system MUST extend `internal/assets/pi/biggz-synthesis-gate.js` as a blocking gate with optional advise. In blocking mode (default) the system MUST block `ask_user_question`/`question` when preceding assistant markdown lacks all 4 markers (`## Sub-agent Result`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**`). Source resolution MUST be `currentTurnMarkdown` → `ctx.history` → `lastAssistant` with 120s window. Block MUST return `{content:[{type:"text", text:"Please synthesize before asking..."}], isError:true}` and MUST NOT call original handler and MUST notify via `pi.notify`/`ctx.ui.notify`. In advise mode when markers present but synthesis thin, system MUST NOT block; it MUST inject non-blocking `concern` via `pi.notify` with `concern: synthesis is thin`. Thin MUST be `Artifacts/Paths` count <2 OR text len <50. Advise MUST be gated behind `BIGGZ_ADVISE=1` (or settings flag) and off by default. Only `PI_SUBAGENT_CHILD=1` MAY bypass both modes; no orchestrator bypass. Advise path MUST NOT auto-fix nor call a model; heuristic only.
(Previously: dual-mode watchdog with blocking on missing `## Sub-agent Result`/`Artifacts/Paths` and thin `<2`/`<50` advise behind `BIGGZ_ADVISE=1`, but without 4-marker strictness, source priority, `isError`+no-call guarantee, or no-orchestrator-bypass rule.)

#### Scenario: Blocking still enforced on missing markers

- GIVEN assistant markdown lacks 4 markers (no `## Sub-agent Result` or missing `Artifacts/Paths`)
- WHEN `ask_user_question` is called in either mode
- THEN system MUST block with `isError:true` and instruct to emit synthesis markdown first and original MUST NOT be called

#### Scenario: Rich synthesis never triggers concern

- GIVEN markdown has 4 markers and `Artifacts/Paths` count ≥2 and len ≥50 (e.g., 3 paths, 120 chars) with Risks and Next
- WHEN `ask` is called with `BIGGZ_ADVISE=1`
- THEN system MUST allow the call and MUST NOT emit concern or block

#### Scenario: Advise emits concern on thin synthesis

- GIVEN `BIGGZ_ADVISE=1` and markdown has 4 markers but `Artifacts/Paths: -` (1 path, 10 chars)
- WHEN `ask_user_question` is called
- THEN system MUST allow the call and emit `concern: synthesis is thin` notification (not a block)

#### Scenario: Advise off by default — thin synthesis passes silently

- GIVEN `BIGGZ_ADVISE` unset and same thin markdown as above
- WHEN `ask_user_question` is called
- THEN system MUST allow the call without concern or block

#### Scenario: Child subagent bypass

- GIVEN `PI_SUBAGENT_CHILD=1`
- WHEN any gate check runs
- THEN system MUST skip both blocking and advise logic entirely

#### Scenario: Same-turn buffer resolves streaming race

- GIVEN synthesis was emitted milliseconds before `ask` in same turn and not yet in `ctx.history`
- WHEN `checkSynthesisPrecondition` runs
- THEN it MUST find markdown via `currentTurnMarkdown` and allow the call

## ADDED Requirements

### Requirement: Synthesis Gate Verification and CI

The system MUST provide unit tests covering 4 gate scenarios (missing→`isError`, rich→pass, thin+advise→warn pass, thin silent without flag) in `internal/assets/pi/biggz-synthesis-gate.test.mjs` plus an integration test in `internal/assets/biggz/orchestrator.test.go` asserting synthesis before question. CI/test gates MUST run `go vet ./...`, `go test ./...`, `node --check` and `node --test biggz-synthesis-gate.test.mjs` and require green.

#### Scenario: Unit tests cover 4 gate scenarios

- GIVEN `biggz-synthesis-gate.test.mjs` runs via `node --test`
- WHEN fixtures exercise missing, rich, thin+advise, thin silent
- THEN all 4 assertions MUST pass and block path MUST assert `isError:true` and `originalCalled==false`

#### Scenario: CI green gates

- GIVEN code is pushed
- WHEN CI runs `go vet ./...` and `go test ./...` and `node --test internal/assets/pi/biggz-synthesis-gate.test.mjs`
- THEN all commands MUST exit 0; any failure MUST block merge
