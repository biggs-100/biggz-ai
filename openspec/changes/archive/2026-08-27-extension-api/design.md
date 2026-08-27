# Design: extension-api

## Technical Approach

Unified `ExtensionAPI` (oh-my-pi parity) is sole registration surface: `On`/`RegisterLens`/`RegisterCommand`/`RegisterTool`/`RegisterFileWriteFallback`/`InvokeTool` + ordered `tool_call`/`tool_result` middleware. `Runner` wraps `pi.on` and delegates to `policy.PolicyInterceptor` (no duplicate policy). Shim keeps `AgentAdapter` deprecated. `testutil.FakeExtensionAPI` unifies testing; only `readability` migrates as proof. Covers proposal + 4 spec deltas.

## Architecture Decisions

### Decision: ExtensionAPI vs plugin.LensPlugin

| Option | Tradeoff | Decision |
|---|---|---|
| Reintroduce `plugin.LensPlugin` | Re-adds static-analysis coupling, breaks `Lens in lens/` | Reject |
| Extend `plugin.AgentAdapter` | Low diff, keeps fragmentation | Reject |
| `internal/extension.ExtensionAPI` | Single surface, fakeable, `Lens.Analyze` pure | **Choose** |

### Decision: Runner vs direct pi.on

| Option | Tradeoff | Decision |
|---|---|---|
| Direct `pi.on` per tool | Duplicates policy/consent/fallback | Reject |
| `Runner` reusing `PolicyInterceptor` | Single `Before/After`, centralized `PI_SUBAGENT_CHILD` bypass | **Choose** |

### Decision: Shim vs rewrite

| Option | Tradeoff | Decision |
|---|---|---|
| Rewrite all adapters | Clean but high blast radius | Reject |
| `shim.go` delegates to `ExtensionAPI`, `// Deprecated` | Additive, `LensPlugin` absent | **Choose** |

### Decision: testutil vs plugintest

| Option | Tradeoff | Decision |
|---|---|---|
| Keep `plugintest.FakeAgent` | No churn, still fragmented | Reject |
| `extension/testutil.FakeExtensionAPI` + alias | Records lenses/tools/handlers, `InvokeTool` executes, `t.Setenv` safe | **Choose** |

### Decision: 1 lens vs all

| Option | Tradeoff | Decision |
|---|---|---|
| Migrate all lenses | Exceeds 800-line budget | Reject |
| Only `readability` | Proves wiring move, `Analyze` pure, no second diff | **Choose** |

## Data Flow

```
pi.on("tool_call") ──→ Runner.Before
                        ├─ PI_SUBAGENT_CHILD==1 → allow
                        ├─ middleware (ordered; block/revise short-circuit)
                        └─ PolicyInterceptor.Before → Evaluator → allow/block/revise/ask
                             ask+ApprovalModeAsk → BIGGZ_TOOL_CONSENT (consent v3)
        ← allow/block/revise
        tool executes
pi.on("tool_result") ─→ Runner.After (no mutate)
pi.on("session_stop")→ Runner.CanStop
RegisterLens(readability) → Registry.Ordered(["readability"])
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/extension/api.go` | Create | `ExtensionAPI` + middleware chain |
| `internal/extension/runner.go` | Create | `Runner` wrapping `pi.on`, reuses `PolicyInterceptor` |
| `internal/extension/shim.go` | Create | `AgentAdapter` shim `// Deprecated` |
| `internal/extension/testutil/fake.go` | Create | `FakeExtensionAPI` in-mem, `InvokeTool` triggers handler |
| `plugintest/agent.go` | Modify | Compat alias to `testutil` |
| `internal/review/lens/readability/register.go` | Create | `Register(api)` wiring only |
| `internal/review/lens/readability/lens.go` | Modify | Remove `registry.RegisterLens` init |
| `internal/review/lens/registry.go` | Modify | Keep `Ordered`/`ResetRegistry` |
| `internal/agents/pi/adapter.go` | Modify | Deploy via `ExtensionAPI` |
| `internal/install/install.go` | Modify | Deploy via `ExtensionAPI` + `filemerge` |
| `internal/assets/pi/biggz-extension-api.js` | Create | JS `pi.on` wiring |

## Interfaces / Contracts

```go
type ToolCallRequest struct{ Tool string; Args map[string]any; CallID string }
type ToolCallResult struct{ Output string; Err error }

type ExtensionAPI interface {
    On(event string, h any)
    RegisterLens(l lens.Lens)
    RegisterCommand(name string, h func(context.Context, map[string]any) error)
    RegisterTool(def ToolDef, h func(context.Context, ToolCallRequest) (ToolCallResult, error))
    RegisterFileWriteFallback(h func(context.Context, ToolCallRequest) (policy.ToolCallDecision, error))
    InvokeTool(ctx context.Context, req ToolCallRequest) (ToolCallResult, error)
    Ordered(ids []string) []lens.Lens
}
type Runner struct{ API ExtensionAPI; Interceptor *policy.PolicyInterceptor }
func (r *Runner) Before(ctx context.Context, req policy.ToolCallRequest) (policy.ToolCallDecision, error)
func (r *Runner) After(ctx context.Context, req policy.ToolCallRequest, res policy.ToolCallResult)
```

JS: `export default function(pi){ if(process.env.PI_SUBAGENT_CHILD==="1")return; pi.on("tool_call",wrap(Before)); pi.on("tool_result",wrap(After)); }` block/revise short-circuits, `tool_result` observability-only.

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | Middleware order, block/revise, `tool_result` no-mutate, `Fake` record/invoke | `internal/extension/*_test.go`, `t.Setenv` |
| Unit | Runner allow/block/ask, fallback, subagent bypass | mock `PolicyEvaluator`, `BIGGZ_TOOL_CONSENT` |
| Unit | Shim delegates, `// Deprecated`, `LensPlugin` absent | `grep "type LensPlugin"`=0, `go vet` |
| Unit | `readability` pure, single `RegisterLens` | snapshot, `grep RegisterLens` count=1 |
| Integration | `install.Deploy` idempotent | `FakeExtensionAPI` + `t.TempDir` |
| E2E | `go vet` + `go test ./... -timeout 180s` | CI gate |

## Threat Matrix

| Boundary | Cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, MDX, `README.sh` | N/A: no classification logic | — | — |
| Git repo selection | `git -C`, relative/absolute | N/A: no VCS routing | — | — |
| Commit state | staged, `commit -a`, empty | N/A: no VCS write | — | — |
| Push state | tracking, first push, refspec | N/A: no push | — | — |
| PR commands | `--head`, env prefix, composed | N/A: no PR automation | — | — |
| Tool-call injection | `user_bash` `rm -rf`/`mkfs`/`:(){:|:&};:`, `tool_result` mutate, `BIGGZ_TOOL_CONSENT` tamper | **Applicable** | `PolicyEvaluator` blocks; `Runner.Before` short-circuits; `After` no-mutate | `TestRunner_BlocksInjectedBash`, `TestRunner_Revise`, `TestRunner_ConsentDeny/Allow`, `TestRunner_ToolResultNoMutate`, `TestRunner_Fallback`, `TestRunner_SubagentBypass` |

Applicable row propagates to tasks; RED before prod.

## Migration / Rollout

No migration. Additive under `internal/extension`; legacy registry stays. Rollback: `git revert` removes `internal/extension/`+JS. Deploy uses `filemerge.WriteFileAtomic`. `plugintest` alias prevents breakage.

## Open Questions

- [ ] `ToolDef.Schema` as `map[string]any` vs JSONSchema?
- [ ] Consent async `v3` channel vs env MVP?
