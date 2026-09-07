# Delta for Extension API

## MODIFIED Requirements

### Requirement: Runner Wrapping pi.on and Reusing PolicyInterceptor

The system MUST provide `internal/extension/runner.go:Runner` subscribing to `pi.on("tool_call"/"tool_result"/"session_stop")`. `Before` MUST delegate to `policy.PolicyInterceptor` (no duplicate logic), enforce consent `v3` allow/deny, preserve `registerFileWriteFallback`, and bypass when `PI_SUBAGENT_CHILD=1`. The duplicated `session_stop` handler in `biggz-extension-api.js` MUST delegate to the single guard in `biggz-tool-interception.js` and MUST NOT implement its own pending-work or summary-verification logic.
(Previously: two independent `session_stop` handlers each checked pending findings/lenses inline.)

#### Scenario: Runner delegates to PolicyInterceptor allow

- GIVEN a `Runner` with `PolicyInterceptor` verdict `allow` and `ApprovalMode=auto`
- WHEN `pi` emits `tool_call`
- THEN `Runner` MUST return `allow` and the tool MUST proceed

#### Scenario: Runner blocks on consent deny

- GIVEN a `Runner` with `ApprovalMode=ask` and consent resolves to `deny`
- WHEN `pi` emits `tool_call`
- THEN `Runner` MUST return `block` with reason and the tool MUST NOT execute

#### Scenario: Subagent child bypasses Runner

- GIVEN `PI_SUBAGENT_CHILD=1` is set
- WHEN `pi` emits `tool_call`
- THEN `Runner` MUST skip `PolicyInterceptor` and consent checks and return `allow`

#### Scenario: Single session-stop guard

- GIVEN `session_stop` fires with identical env in either extension file
- WHEN each handler resolves
- THEN both verdicts MUST be identical (one implementation, one delegation)
