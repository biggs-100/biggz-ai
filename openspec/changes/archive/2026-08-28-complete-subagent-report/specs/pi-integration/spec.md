# Delta for pi-integration

## MODIFIED Requirements

### Requirement: Advisor Inline Watchdog Advise Mode

`biggz-synthesis-gate.js` MUST block checkpoint `ask` only when strict `currentTurnMarkdown` lacks 4 markers; history ignored for block. Block MUST be `isError:true` without handler + notify. Thin (count<2 OR len<50) with markers MUST NOT block; MUST emit `concern: synthesis is thin` when `BIGGZ_ADVISE=1` (off default); advise MAY fallback `currentTurn→history→lastAssistant(120s)` for concern only. `PI_SUBAGENT_CHILD=1` bypasses; no orchestrator bypass. `hasSynthesis` stays pass. General without checkpoint tokens MUST bypass.
(Previously: block without same-turn strictness.)

#### Scenario: Missing blocks

- GIVEN lacks 4 markers
- WHEN checkpoint ask
- THEN MUST block `isError:true`

#### Scenario: Thin advises not blocks

- GIVEN `BIGGZ_ADVISE=1`, 4 markers count 1 len 10
- WHEN checkpoint ask
- THEN MUST allow + `concern: synthesis is thin`

#### Scenario: General bypasses

- GIVEN "¿por dónde empezamos?" no checkpoint tokens
- WHEN gate evaluates
- THEN MUST allow without synthesis

### Requirement: Synthesis Gate Verification and CI

`biggz-synthesis-gate.test.mjs` MUST cover 4 gates + `orchestrator.test.go`; MUST cover >50KB loop, envelope reject, pending equality, engram alias. CI MUST run `go vet`, `go test`, `node --test` green.
(Previously: 4 gates only; now adds loop/envelope/pending.)

#### Scenario: Gate tests pass

- GIVEN `node --test biggz-synthesis-gate.test.mjs`
- WHEN fixtures run
- THEN MUST pass and block asserts `isError:true`

## ADDED Requirements

### Requirement: Question Envelope Validation

`validateQuestionEnvelope` MUST reject when header>16, label>60, questions>4, or options∉[2,4]; reject MUST be `isError:true` naming limit, NOT call handler, emit fallback. Valid MUST allow.

| Field | Limit |
|-------|-------|
| header | ≤16 |
| label | ≤60 |
| questions | ≤4 |
| options | 2–4 |

#### Scenario: Header too long

- GIVEN header 17 chars
- WHEN validated
- THEN MUST reject naming header 16

#### Scenario: Options range

- GIVEN question with 1 option
- WHEN validated
- THEN MUST reject and emit fallback

#### Scenario: Valid passes

- GIVEN header 12, 3 questions each 3 options <60
- WHEN validated
- THEN MUST allow native ask