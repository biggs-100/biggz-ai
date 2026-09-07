# Tool Interception Specification

## Purpose

Minimal pre-execution parity with oh-my-pi `ExtensionAPI`: blockable `tool_call` interception, consent-based approval, and session-stop guard without FSM rewrite or God objects. The 13-state FSM remains sole review authority.

## Requirements


### Requirement: BeforeToolCall Blocking

The system MUST provide `ToolCallInterceptor.BeforeToolCall` that executes synchronously before tool execution and MAY return allow, block, or revise. Default MUST be allow; only `PolicyEvaluator` deny/ask MAY block. Block MUST prevent execution and surface reason.

#### Scenario: Allowed tool proceeds

- GIVEN a tool_call with policy verdict allow
- WHEN BeforeToolCall evaluates it
- THEN it MUST return allow and execution MUST proceed

#### Scenario: Denied tool blocked pre-exec

- GIVEN a tool_call denied by `PolicyEvaluator`
- WHEN BeforeToolCall evaluates it
- THEN it MUST return block with reason and tool MUST NOT execute

#### Scenario: Revised tool args

- GIVEN a tool_call with revisable args
- WHEN BeforeToolCall returns revise
- THEN execution MUST use revised args and record original

### Requirement: AfterToolCall Observability

The system MUST provide `ToolCallInterceptor.AfterToolCall` that runs after execution for observability only. It MUST NOT block, revise, or re-execute. It MUST receive execution result/error.

#### Scenario: Success observed

- GIVEN a completed tool execution with output
- WHEN AfterToolCall is invoked
- THEN it MUST record output and MUST NOT alter FSM state

#### Scenario: Failure observed without mutation

- GIVEN a tool that errored
- WHEN AfterToolCall is invoked
- THEN it MUST record error and MUST NOT retry or block

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

### Requirement: Session Stop Guard CanStopSession

The system MUST provide pure `CanStopSession(state) bool` checked before session termination in `finalize.go`. It MUST return true only when closure invariants hold; otherwise MUST block termination. It MUST be idempotent and add no FSM states.

#### Scenario: Stop allowed

- GIVEN state satisfies closure invariants
- WHEN CanStopSession is called
- THEN it MUST return true and termination MAY continue

#### Scenario: Stop blocked idempotent

- GIVEN state with pending work
- WHEN CanStopSession is called twice
- THEN it MUST return false both times and MUST NOT mutate state

### Requirement: FSM Authority Invariant

The FSM in `model/fsm.go` MUST remain sole authority for 13-state transitions. Interceptor and `CanStopSession` MUST NOT add states, transitions, or bypass role/budget guards. Interceptor decision MUST precede FSM; FSM gate stays post-hoc.

#### Scenario: FSM unchanged

- GIVEN any interceptor outcome
- WHEN inspecting `model/fsm.go`
- THEN 13 states and transition table MUST be identical to baseline

#### Scenario: Interceptor does not bypass gate

- GIVEN BeforeToolCall returned allow
- WHEN FSM evaluates review completion
- THEN gate MUST still reject if policy/budget guards fail

### Requirement: No God Object and Size Budget

The system MUST NOT introduce a God object (`ToolSession` or >20-field aggregate). `PolicyInterceptor` MUST wrap only `PolicyEvaluator`+`ApprovalMode` with no FSM dependency. The change MUST be <250 lines single PR, verified by `go test ./... -count=1 -timeout 180s` with `t.Setenv` isolation.

#### Scenario: Size and coupling check

- GIVEN the change diff
- WHEN counting lines and inspecting `internal/policy/interceptor.go`
- THEN total MUST be <250 lines and imports MUST NOT include `model/fsm`

#### Scenario: God object absent

- GIVEN codebase after change
- WHEN searching for `type ToolSession` or struct with >20 fields for session
- THEN zero matches MUST be found

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

### Requirement: Session-Stop Summary Verification

The `session_stop` guard in `biggz-tool-interception.js` MUST shell out to
`biggz session-close --check-only` (bounded timeout, default 1000ms) AFTER the
existing pending-findings/lenses check. On CLI exit 0 it MUST allow close; on
exit 1 carrying the `blocked(session_summary_missing)` token it MUST return
`{ block: true, reason }` with the CLI's reason. On CLI timeout, execution
failure, or exit 1 WITHOUT the gate token (e.g. a stale binary without the
verb, which exits 1 with help text) it MUST NOT trap the agent: it MUST allow
close and emit a degraded warning. It MUST NOT reimplement BigMem verification in JS.

#### Scenario: Verified summary allows stop

- GIVEN no pending findings/lenses and CLI exits 0
- WHEN `session_stop` fires
- THEN the guard MUST return allow (undefined)

#### Scenario: Missing summary blocks stop

- GIVEN no pending findings/lenses and CLI exits 1 with `blocked(session_summary_missing)`
- WHEN `session_stop` fires
- THEN the guard MUST return `{ block: true }` with `blocked(session_summary_missing)`

#### Scenario: Stale binary never traps stop

- GIVEN the installed `biggz` predates the `session-close` verb (exits 1 with help text, no gate token)
- WHEN `session_stop` fires
- THEN the guard MUST allow close and MUST emit a degraded warning

#### Scenario: Pending work still blocks first

- GIVEN `BIGGZ_PENDING_FINDINGS>0` or `BIGGZ_PENDING_LENSES>0`
- WHEN `session_stop` fires
- THEN the guard MUST return `{ block: true }` without invoking the CLI

#### Scenario: CLI failure degrades without trapping

- GIVEN the CLI times out or crashes
- WHEN `session_stop` fires
- THEN the guard MUST allow close and MUST emit a degraded warning
