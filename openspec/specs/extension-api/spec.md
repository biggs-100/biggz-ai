# Extension API Specification

## Purpose

Unified `ExtensionAPI` (oh-my-pi parity): events/tools/commands/lenses, `tool_call`/`tool_result` middleware, `registerFileWriteFallback`, `invokeTool`. `Runner` wraps `pi.on` reusing `policy.Interceptor`; `testutil` provides fake; `Lens.Analyze` stays pure.

## Requirements

### Requirement: ExtensionAPI Interface

The system MUST provide `internal/extension/api.go:ExtensionAPI` with `On`, `RegisterLens`, `RegisterCommand`, `RegisterTool`, `RegisterFileWriteFallback`, `InvokeTool(ctx, ToolCallRequest) (ToolCallResult, error)`. It MUST be the sole registration surface; direct registry mutation outside it MUST NOT be required.

#### Scenario: RegisterTool and InvokeTool round-trip

- GIVEN a `FakeExtensionAPI`
- WHEN `RegisterTool("my_tool", h)` then `InvokeTool{Tool:"my_tool"}`
- THEN handler `h` MUST be invoked and result returned

#### Scenario: RegisterLens via ExtensionAPI

- GIVEN an `ExtensionAPI`
- WHEN `RegisterLens(readability.Lens{})` is called
- THEN lens MUST be retrievable via `Ordered` and `Analyze` MUST remain pure

#### Scenario: Fallback registration

- GIVEN an `ExtensionAPI` with no fallback
- WHEN `RegisterFileWriteFallback(h)` is called
- THEN file-write interception MUST delegate to `h`

### Requirement: ToolCall Middleware Chain

The system MUST support ordered `tool_call`/`tool_result` middleware via `On`. Execution MUST be registration order; `block`/`revise` MUST short-circuit remaining `tool_call` handlers. `tool_result` MUST be observability-only and MUST NOT block or re-execute.

#### Scenario: Blocking middleware short-circuits

- GIVEN two `tool_call` handlers where the first returns `block`
- WHEN a tool_call is dispatched
- THEN the second handler MUST NOT run and the tool MUST NOT execute

#### Scenario: tool_result does not mutate

- GIVEN a `tool_result` handler
- WHEN it returns after a successful tool execution
- THEN the original result MUST be preserved unchanged

### Requirement: Runner Wrapping pi.on and Reusing PolicyInterceptor

The system MUST provide `internal/extension/runner.go:Runner` subscribing to `pi.on("tool_call"/"tool_result"/"session_stop")`. `Before` MUST delegate to `policy.PolicyInterceptor` (no duplicate logic), enforce consent `v3` allow/deny, preserve `registerFileWriteFallback`, and bypass when `PI_SUBAGENT_CHILD=1`.

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

### Requirement: Unified testutil FakeExtensionAPI

The system MUST provide `internal/extension/testutil:FakeExtensionAPI` (in-memory `ExtensionAPI`). It MUST record lenses/commands/tools/fallback/`On` handlers, allow `InvokeTool` to trigger handler, support `t.Setenv` isolation; `plugintest` MUST remain compat alias to `testutil`.

#### Scenario: Fake records and invokes

- GIVEN a `FakeExtensionAPI`
- WHEN `RegisterCommand("x", h)` and `RegisterTool` are called then `InvokeTool` targets `x`'s tool
- THEN `FakeExtensionAPI` MUST have recorded both registrations and invoked the correct handler

#### Scenario: plugintest alias still works

- GIVEN existing tests importing `plugintest.FakeAgent`
- WHEN they run against the new `testutil` alias
- THEN they MUST pass without modification

### Requirement: Single Lens Migration Readability via ExtensionAPI

Exactly one lens `readability` MUST migrate to `ExtensionAPI.RegisterLens`; `Lens.Analyze` in `readability/lens.go` MUST stay pure unchanged, still reuse `DeriveRiskInput` without second `git diff`. Only wiring moves; other lenses MUST stay on legacy registry.

#### Scenario: Readability pure Analyze unchanged

- GIVEN `readability.Lens` registered via `ExtensionAPI`
- WHEN `Analyze` is called with a `LensInput` containing parser failure
- THEN it MUST return the same deterministic finding as before migration

#### Scenario: Other lenses not migrated

- GIVEN the codebase after this change
- WHEN grepping for `RegisterLens` calls
- THEN exactly one call with `readability` MUST exist and no other lens IDs MUST appear
