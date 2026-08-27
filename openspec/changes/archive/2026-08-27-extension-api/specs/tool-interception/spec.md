# Delta for Tool Interception

## ADDED Requirements

### Requirement: Runner Reuses PolicyInterceptor

The system MUST provide `Runner` in `internal/extension/runner.go` that reuses `policy.PolicyInterceptor` + `PolicyEvaluator` + `ApprovalMode` for `BeforeToolCall` decisions; it MUST NOT duplicate policy logic. `Runner` MUST wrap `pi.on("tool_call")`/`pi.on("tool_result")` and delegate synchronously to `PolicyInterceptor.BeforeToolCall`/`AfterToolCall`. `registerFileWriteFallback` semantics MUST remain intact.

#### Scenario: Runner delegates allow

- GIVEN `Runner` with evaluator verdict `allow`
- WHEN `pi` emits `tool_call`
- THEN `Runner` MUST return `allow` without reimplementing policy

#### Scenario: Runner delegates block

- GIVEN evaluator verdict `deny`/`block`
- WHEN `pi` emits `tool_call`
- THEN `Runner` MUST return `block` with the evaluator's reason and MUST NOT execute the tool

#### Scenario: Fallback preserved

- GIVEN `Runner` with fallback registered via `RegisterFileWriteFallback`
- WHEN a file-write tool_call is intercepted
- THEN fallback handler MUST still be invocable exactly as before

## MODIFIED Requirements

### Requirement: ApprovalMode Hook via Consent v3

When `ApprovalMode` requires ask, the system MUST emit `ToolApprovalRequested` via `biggz-ai.review-integration.consent/v3` through `ExtensionAPI`/`Runner` `On("tool_call")`, await `resolved` (allow/deny), and enforce resolved decision. `registerFileWriteFallback` MUST remain intact. `Runner` is the sole implementation of this hook; no second consent path MUST exist. `PI_SUBAGENT_CHILD=1` MUST bypass consent.
(Previously: described hook without Runner as sole implementation or subagent bypass)

#### Scenario: Consent allow resumes

- GIVEN a tool_call requiring approval
- WHEN consent resolves to allow
- THEN execution MUST proceed and `tool_execution_start` MUST have been emitted

#### Scenario: Consent deny blocks

- GIVEN a tool_call requiring approval
- WHEN consent resolves to deny
- THEN execution MUST be blocked and reason MUST propagate

#### Scenario: Subagent child bypasses consent

- GIVEN `PI_SUBAGENT_CHILD=1`
- WHEN a tool_call requiring approval is emitted
- THEN `Runner` MUST bypass consent and allow without emitting `ToolApprovalRequested`
