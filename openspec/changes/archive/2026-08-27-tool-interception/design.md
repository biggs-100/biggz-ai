# Design: tool-interception

## Technical Approach

Additive pre-exec parity with oh-my-pi `ExtensionAPI`; FSM 13 states untouched. `ToolCallInterceptor` (`BeforeToolCall` blocking/revisable, `AfterToolCall` observe-only) implemented by `PolicyInterceptor` wrapping only `PolicyEvaluator`+`ApprovalMode`. `ExtensionRunner.On("tool_call")` → `BeforeToolCall` → consent `biggz-ai.review-integration.consent/v3` on `ask` → exec → `AfterToolCall`. `CanStopSession(state) bool` pure guard in `finalize.go` before `complete_review`. Single PR <250 lines, `t.Setenv` tests.

## Architecture Decisions

| Decision | Options | Tradeoffs | Choice |
|----------|---------|-----------|--------|
| Interceptor contract | FSM hook / Interface / God ToolSession | FSM couples; God >20 fields leaks | **Interface `ToolCallInterceptor` + `PolicyInterceptor` wraps `PolicyEvaluator`+`ApprovalMode`; no `model/fsm` import** |
| Before vs After split | Single `Intercept` / Before+After | Single conflates block+observe | **Split: `BeforeToolCall` sync allow/block/revise; `AfterToolCall` observe-only, no FSM mutate** |
| CanStopSession location | `finalize.go` / `policy.go` / FSM state | `policy.go` scatters; FSM violates invariant | **`internal/review/finalize.go` pure `CanStopSession(state) bool`; idempotent, no states** |
| ApprovalMode wiring | Direct prompt / `consent/v3` | Direct dead-locks CI; breaks parity | **`ToolApprovalRequested` via `biggz-ai.review-integration.consent/v3` via `ExtensionRunner.On("tool_call")`; fallback intact** |

## Data Flow

```
tool_call ──► Runner.On("tool_call", BeforeToolCall)
               │ PolicyInterceptor.BeforeToolCall(ctx, req)
               │  ├─ PolicyEvaluator(allow/deny/ask)
               │  └─ ask→ToolApprovalRequested(consent/v3)→await resolved
               │ emit tool_execution_start on allow
               ▼
          exec (Runner override bash/python; fallback file_write)
               ▼
          AfterToolCall(ctx, req, result) observe-only
               ▼
          session_stop→CanStopSession(state) pure→continue/block→FSM complete_review
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/policy/interceptor.go` | Create | Interface + `PolicyInterceptor` (`Before`/`After`), request/decision/result types, no FSM import, <80 lines |
| `internal/review/finalize.go` | Modify | `CanStopSession(state) bool` pure/idempotent before `Finalize`; no states |
| `internal/assets/pi/*` + `internal/opencodeplugin/plugin.go` | Modify | `Runner.On("tool_call", BeforeToolCall)` + `tool_execution_*`/`tool_approval_*`; keep `registerFileWriteFallback`; bash/python via Runner |
| `internal/install/*` | Modify | Atomic asset deploy if layout changed |
| `internal/policy/interceptor_test.go` | Create | Table Before/After, t.Setenv, allow/block/revise |
| `internal/review/finalize_test.go` | Modify | CanStopSession table allowed/blocked/idempotent |

## Interfaces / Contracts

```go
// internal/policy/interceptor.go — no model/fsm import
type ToolCallRequest struct {
    Tool string            // "user_bash" | "file_write" | ...
    Args map[string]any    // revisable
    CallID string
}
type DecisionKind string // "allow" | "block" | "revise"
type ToolCallDecision struct {
    Kind DecisionKind
    Reason string
    RevisedArgs map[string]any // when Kind=="revise"
}
type ToolCallResult struct { Output string; Err error }
type ToolCallInterceptor interface {
    BeforeToolCall(ctx context.Context, req ToolCallRequest) (ToolCallDecision, error)
    AfterToolCall(ctx context.Context, req ToolCallRequest, res ToolCallResult)
}
type PolicyInterceptor struct { evaluator PolicyEvaluator; approval ApprovalMode }
func (p *PolicyInterceptor) BeforeToolCall(ctx context.Context, req ToolCallRequest) (ToolCallDecision, error)
func (p *PolicyInterceptor) AfterToolCall(ctx context.Context, req ToolCallRequest, res ToolCallResult)

// internal/review/finalize.go
func CanStopSession(state any) bool // pure, idempotent; true only when closure invariants hold
```

Consent `biggz-ai.review-integration.consent/v3`: `ToolApprovalRequested`→`resolved{allow|deny}`; default allow, only deny/ask blocks.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Before allow/block/revise; After no-mutate; CanStopSession true/false/idempotent; no God object | table `go test` + `t.Setenv` BIGGZ_*, `t.TempDir` |
| Integration | Runner On tool_call → tool_execution_start/ApprovalRequested→resolved; fallback intact | fake ExtensionAPI, assert emits/block |
| E2E | `go test ./... -count=1 -timeout 180s` + `go vet`; <250 lines | CI gate |

## Threat Matrix

Per `internal/assets/skills/sdd-design/references/threat-matrix.md`:

| Boundary | Cases | Applicability | Response | RED tests |
|----------|-------|---------------|----------|-----------|
| Documentation-like paths | `requirements.txt`, MDX exec | N/A — no exec classification | — | — |
| Git repo selection | `git -C`, relative/absolute | N/A — no git cwd change | — | — |
| Commit state | staged, `commit -a`, empty | N/A — no VCS commit | — | — |
| Push state | tracking, first push, refspec | N/A — no push | — | — |
| PR commands | `--head`, env prefix | N/A — no `gh pr` | — | — |
| Tool-call injection | `user_bash rm -rf`, arg tamper, `session_stop` bypass | **Applicable** | Before blocks only via PolicyEvaluator deny/ask; default allow; revise records original; After never re-executes; CanStopSession pure | RED: injected bash blocked; revise uses revised; After error no retry; session_stop idempotent; tampered consent propagated |

## Migration / Rollout

No migration. Additive single commit; `git revert <sha>` removes interceptor + guard + hook. Fallback post-hoc gate.

## Open Questions

- [ ] Confirm `ExtensionRunner.On` signature — spike before tasks.
- [ ] Define `CanStopSession` state shape (pending findings vs lens count).
