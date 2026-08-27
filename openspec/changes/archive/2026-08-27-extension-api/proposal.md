# Proposal: extension-api

## Intent

Unify fragmented `plugin.AgentAdapter`+`lens.Lens`+`plugintest`+`policy.Interceptor` (each tool duplicates register/test/lifecycle) behind oh-my-pi parity `ExtensionAPI` (`events+tools+commands+renderers+providers`, `tool_call`/`tool_result` middleware, `registerFileWriteFallback`, `invokeTool`). Provide `internal/extension` with `Runner` reusing `policy.Interceptor`; `Lens.Analyze` stays pure.

## Scope

### In Scope
- `internal/extension/api.go` — `ExtensionAPI`: `On`/`RegisterLens`/`RegisterCommand`/`RegisterTool`/`RegisterFileWriteFallback`/`InvokeTool` + middleware
- `internal/extension/runner.go` — `Runner` wrapping `pi.on("tool_call")`/`tool_result`/`session_stop` → `policy.Interceptor` + consent `v3`
- `internal/extension/shim.go` — `AgentAdapter` as provider; hooks/custom-tools → `RegisterTool`
- `internal/extension/testutil/` — unified fake `ExtensionAPI`
- 1 lens migrated: `readability` via `ExtensionAPI` (proof)

### Out of Scope
- Rewrite all lenses (only `readability`; others deferred)
- `hashline`/`tui`/`blobstore`/`branching`/`tool-interception` rewrites; `model/fsm.go` untouched; no `ToolSession`
- New MCP server; hooks deletion

## Capabilities

### New Capabilities
- `extension-api`: unified ExtensionAPI + Runner + middleware + fallback + invokeTool + testutil

### Modified Capabilities
- `plugin-system`: `AgentAdapter` shimmed as provider; `LensPlugin` not reintroduced
- `review-lenses`: `readability` registration via `ExtensionAPI`; `Lens` contract unchanged
- `tool-interception`: `Runner` reuses `policy.Interceptor`; no duplicate policy

## Approach

1. `api.go` — `ExtensionAPI` + `ToolCallInterceptor` chain.
2. `runner.go` — `pi.on("tool_call", Before)` → consent `v3` → `After`; keep `registerFileWriteFallback`; `PI_SUBAGENT_CHILD=1` guard.
3. `shim.go` — `AgentAdapter` delegates via `ExtensionAPI`; hooks → `RegisterTool`.
4. `install` — `Deploy` via `ExtensionAPI` (`filemerge` idempotent) + `biggz-extension-api.js`.
5. `testutil/fake.go` — unified fake; `plugintest` stays compat alias.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/extension/api.go` | New | ExtensionAPI + middleware |
| `internal/extension/runner.go` | New | Runner pi.on wrapping |
| `internal/extension/shim.go` | New | AgentAdapter shim |
| `internal/extension/testutil/` | New | Unified fake |
| `internal/review/lens/readability/` | Modified | Proof migration |
| `internal/policy/interceptor.go` | Modified | Reused |
| `internal/agents/pi/adapter.go`, `internal/install/*` | Modified | Deploy via ExtensionAPI |
| `internal/assets/pi/biggz-extension-api.js` | New | Pi wiring |
| `openspec/specs/extension-api/spec.md` | New | Spec (next) |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Pi `pi.on` signature drift | Med | Spike; interface abstracts, fake covers both |
| Shim breaks Deploy | Med | Additive delegation; agents/install tests |
| Readability regression | Low | `Analyze` pure, only wiring moves |
| testutil churn | Low | Keep `plugintest` alias |

## Rollback Plan

`git revert <sha>` deletes `internal/extension/` + JS, restores direct registry/`AgentAdapter`. No migration. `go test ./...` + `go vet` pass.

## Dependencies

- `plugin.AgentAdapter`, `lens.Lens`, `policy.Interceptor`+`ApprovalMode`, consent `v3`, `registerFileWriteFallback`

## Success Criteria

- [ ] `ExtensionAPI`+`Runner` (`pi.on tool_call/tool_result`) reusing `policy.Interceptor`, fallback intact
- [ ] Shim `AgentAdapter` (no `LensPlugin`) + `install.Deploy` via `ExtensionAPI`
- [ ] Unified `testutil`, `t.Setenv` green
- [ ] `readability` via `ExtensionAPI`, `Lens.Analyze` pure, `go test ./...`+`go vet` pass
- [ ] `grep "type LensPlugin"` = 0
