# Proposal: tool-interception

## Intent

biggz-ai has `policy.PolicyEvaluator` + `ApprovalMode` + 13-state FSM but no pre-exec BeforeLens/AfterLens at tool_call; review gate is post-hoc. oh-my-pi `ExtensionAPI` provides blockable/revisable tool_call pre-exec, tool_execution_start/update/end, tool_approval_requested/resolved, session_stop continue/block via `ExtensionRunner`. Add minimal parity: intercept tool_call before exec, route via consent, guard session_stop — no God objects or FSM rewrites.

## Scope

### In Scope
- ToolCallInterceptor BeforeToolCall/AfterToolCall — blockable pre-exec + observable post-exec
- ApprovalMode hook ToolApprovalRequested con consent v3 — biggz-ai.review-integration.consent/v3 via ExtensionAPI
- session_stop guard CanStopSession — continue/block before termination
- 1 día single PR <250 líneas, t.Setenv tests, go test ./... -count=1 -timeout 180s

### Out of Scope
- no God object ToolSession 70 campos
- no reescribir FSM (model/fsm.go untouched)
- no RDD kill switch changes

## Capabilities

### New Capabilities
- tool-interception: blockable tool_call interception, approval consent v3, session_stop guard (new spec tool-interception)

### Modified Capabilities
- None — additive only; core-review/review-gates/review-lifecycle unchanged

## Approach

1. internal/policy/interceptor.go interface — ToolCallInterceptor with BeforeToolCall/AfterToolCall; PolicyInterceptor wraps PolicyEvaluator+ApprovalMode; no FSM dep.
2. ExtensionAPI On tool_call — ExtensionRunner On("tool_call", BeforeToolCall) emits tool_execution_start, ToolApprovalRequested on ask, awaits resolved; registerFileWriteFallback intact; user_bash/python via Runner override.
3. finalize.go CanStopSession — pure CanStopSession(state) bool before complete_review; no FSM state added.
4. Tests — t.Setenv for BIGGZ_*, table-driven interceptor/consent/CanStopSession; no git/network.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/policy/interceptor.go` | New | ToolCallInterceptor + PolicyInterceptor |
| `internal/review/finalize.go` | Modified | CanStopSession guard |
| `internal/assets/pi/*`, `internal/opencodeplugin/plugin.go` | Modified | ExtensionAPI On tool_call wiring |
| `openspec/specs/tool-interception/spec.md` | New | Spec (next phase) |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Blocks legit tool | Med | Default allow; only PolicyEvaluator deny/ask blocks |
| Consent v3 deadlock | Low | Timeout + t.Setenv tests |
| session_stop livelock | Low | CanStopSession pure/idempotent; manual override |
| Creep to God object | Low | <250 cap; reject ToolSession |

## Rollback Plan

Single commit revert: git revert <sha> removes interceptor.go, CanStopSession, Runner hook. No migration. Fallback = post-hoc review gate. go test ./... passes.

## Dependencies

- PolicyEvaluator, ApprovalMode, FSM 13 estados, biggz-ai.review-integration.consent/v3
- ExtensionAPI/ExtensionRunner, registerFileWriteFallback

## Success Criteria

- [ ] BeforeToolCall blocks/revises pre-exec; AfterToolCall observes post-exec
- [ ] ToolApprovalRequested consent v3 respected (allow/deny)
- [ ] session_stop via CanStopSession continue/block
- [ ] Single PR <250 líneas, no FSM/RDD, no ToolSession
- [ ] go test ./... -count=1 -timeout 180s + go vet pass, t.Setenv isolation
