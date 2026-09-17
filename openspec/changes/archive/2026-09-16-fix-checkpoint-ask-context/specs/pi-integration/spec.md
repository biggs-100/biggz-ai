# Delta for pi-integration

## MODIFIED Requirements

### Requirement: Question Envelope Validation

`validateQuestionEnvelope` MUST reject when header>16, label>60, questions>4, or options∉[2,4]; reject MUST be `isError:true` naming limit, NOT call handler, emit fallback. Valid MUST allow.

| Field | Limit |
|-------|-------|
| header | ≤16 |
| label | ≤60 |
| questions | ≤4 |
| options | 2–4 |

Rejection is enforced by the deployed asset: `internal/assets/pi/ask-user-choice.ts` MUST invoke the checkpoint-ask check command before presenting any question and MUST refuse to present it when the check decides a block, surfacing the check's message to the agent. The invocation MUST follow the deployed-asset contract (argv array, no shell; bounded timeout; injectable test seam — `biggz-session-guard.js` precedent) and MUST NOT depend on the non-deployed `biggz-synthesis-gate.js`. A check pass MUST NOT skip the envelope's own limits (table above). A check that cannot decide (timeout, missing/broken binary, crash) MUST present with a visible notice that enforcement was skipped — no silent pass for the synthesis precondition, no bricked ask. A node test MUST prove the deployed asset invokes the check (argv + bounded timeout) and the refuse path — asserting the call, not merely the check's existence.
(Previously: validation defined only on the undeployed side, with no production call site and no asset invocation.)

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

#### Scenario: Deployed asset invokes the check

- GIVEN `ask-user-choice.ts` execute path with a question
- WHEN presenting
- THEN it MUST invoke the check via argv array (no shell) under a bounded timeout, asserted via the test seam

#### Scenario: Decided block refuses presentation

- GIVEN check exits non-zero for option description `yes, go ahead`
- WHEN the asset executes
- THEN it MUST NOT present the question and MUST surface the check's message to the agent

#### Scenario: Pass does not skip envelope limits

- GIVEN check exits 0 but header is 17 chars
- WHEN the asset validates
- THEN header MUST still be rejected naming limit 16

#### Scenario: Indeterminate check degrades visibly

- GIVEN check binary missing or timeout
- WHEN the asset executes
- THEN the question MUST present with a visible warning that enforcement was skipped
