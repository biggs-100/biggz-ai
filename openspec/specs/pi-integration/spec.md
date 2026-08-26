# Pi Integration Specification

## Purpose

The Pi Integration domain covers biggz-ai runtime behavior when hosted inside the Pi coding agent (harness, synthesis gate, and inline watchdog). This spec defines the advisor dual-mode watchdog that complements the native RAR gatekeeper with non-blocking concern injection for thin orchestrator synthesis.

## Requirements

### Requirement: Advisor Inline Watchdog Advise Mode

The system MUST extend `internal/assets/pi/biggz-synthesis-gate.js` from a blocking gate to a dual-mode watchdog. In blocking mode (default), the system MUST block `ask_user_question`/`question` when the preceding assistant markdown lacks `## Sub-agent Result` markers. In advise mode, when markers are present but synthesis is thin, the system MUST NOT block; it MUST inject a non-blocking `concern` note via `pi.on(tool_call)` / `pi.notify`. Thin synthesis MUST be defined heuristically as `Artifacts/Paths` count < 2 or `Artifacts/Paths` text length < 50 chars. Advise mode MUST be gated behind `BIGGZ_ADVISE=1` or settings flag and MUST be off by default (encendido suave). The system MUST keep the existing `PI_SUBAGENT_CHILD=1` guard. The advise path MUST NOT auto-fix and MUST NOT call a model; it is purely heuristic.

#### Scenario: Blocking still enforced on missing markers

- GIVEN assistant markdown lacks `## Sub-agent Result` and `Artifacts/Paths`
- WHEN `ask_user_question` is called in either mode
- THEN the system MUST block with error instructing to emit synthesis markdown first

#### Scenario: Advise emits concern on thin synthesis

- GIVEN `BIGGZ_ADVISE=1` and markdown has `## Sub-agent Result` but `Artifacts/Paths: -` (1 path, 10 chars)
- WHEN `ask_user_question` is called
- THEN the system MUST allow the call and emit a `concern` notification (not a block)

#### Scenario: Advise off by default — thin synthesis passes silently

- GIVEN `BIGGZ_ADVISE` unset and same thin markdown as above
- WHEN `ask_user_question` is called
- THEN the system MUST allow the call without concern or block

#### Scenario: Rich synthesis never triggers concern

- GIVEN markdown has `## Sub-agent Result` and `Artifacts/Paths: 3 paths, 120 chars` with Risks and Next
- WHEN `ask_user_question` is called with `BIGGZ_ADVISE=1`
- THEN the system MUST allow the call and MUST NOT emit a concern

#### Scenario: Child subagent bypass

- GIVEN `PI_SUBAGENT_CHILD=1`
- WHEN any gate check runs
- THEN the system MUST skip both blocking and advise logic entirely
