# Session Guard Ownership Specification

## Purpose

Single ownership of the `session_stop` summary guard: one ES module defines
`checkSessionStop`, `SESSION_STOP_TIMEOUT_MS`, and `_setSessionStopExecForTest`;
both pi extensions resolve to that same implementation. Behavioral contract is
unchanged — see `openspec/specs/tool-interception/spec.md`
(Session-Stop Summary Verification) and `openspec/specs/extension-api/spec.md`
(Runner Wrapping pi.on and Reusing PolicyInterceptor).

## Requirements

### Requirement: Single Guard Definition

The system MUST define `checkSessionStop`, `SESSION_STOP_TIMEOUT_MS` (= 1000),
and `_setSessionStopExecForTest` in exactly one module:
`internal/assets/pi/biggz-session-guard.js`. No other `internal/assets/pi/*.js`
file MUST contain a competing definition; `biggz-tool-interception.js` MUST only
re-export them for backward compatibility.

#### Scenario: Search proves single definition

- GIVEN the moved guard module exists
- WHEN searching `rg "function checkSessionStop|const SESSION_STOP_TIMEOUT_MS|function _setSessionStopExecForTest" internal/assets/pi/*.js`
- THEN exactly one definition site MUST match (the guard module)

#### Scenario: Old import paths keep working

- GIVEN a consumer importing `checkSessionStop` from `./biggz-tool-interception.js`
- WHEN the module resolves
- THEN it MUST receive the guard module's function (re-export, no local logic)

### Requirement: Acyclic One-Way Import Graph

`biggz-extension-api.js` and `biggz-session-stop.test.mjs` MUST import the guard
directly from `./biggz-session-guard.js`. The guard module MUST import only
`node:child_process`; it MUST NOT import any local extension file, so no import
cycle SHALL exist.

#### Scenario: Both extensions share one instance

- GIVEN identical env and the same mocked exec seam
- WHEN `session_stop` fires in each extension file
- THEN both verdicts MUST be identical (same module instance)

#### Scenario: No cycle at load time

- GIVEN the guard module with zero local imports
- WHEN pi loads both extensions
- THEN startup MUST succeed with no circular-import error

### Requirement: Zero Behavior Change

The move MUST be verbatim (logic, timeout value, block/degrade semantics
byte-identical). All 10 existing tests in `biggz-session-stop.test.mjs` MUST
pass with unmodified intent; only import sources MAY change.

#### Scenario: Existing contract passes unmodified

- GIVEN the repointed test file
- WHEN running `node --test internal/assets/pi/biggz-session-stop.test.mjs`
- THEN 10/10 tests MUST pass without intent changes to any case
