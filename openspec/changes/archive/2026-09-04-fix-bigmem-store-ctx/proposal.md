# Proposal: Context-Aware BigMem Store API

## Intent

BigMem `Store` has zero `context.Context` support — callers cannot cancel or time-bound SQLite work, risking hangs (WAL lock, FTS search). Add additive `*Ctx` API with timeouts; existing callers keep working unchanged.

## Scope

### In Scope
- `SaveCtx/GetCtx/SearchCtx/UpdateCtx/DeleteCtx` on `Store` (ctx + timeout, visible errors)
- `SessionContextCtx/TimelineCtx/SavePromptCtx` in `full.go` (same pattern)
- Non-`Ctx` methods delegate to `Ctx` with `context.Background()`
- Wire ctx to driver (`QueryContext`/`ExecContext`); map `ctx.Err()` explicitly, no silent fallback
- Migrate 3 consumers to `*Ctx`: `cmd/biggz-mcp/main.go`, `session_guard.go`, `doctor/bigmem.go`

### Out of Scope
- Raw-SQL bypass fix, N+1 query fix, blob/DOCS work (SDDs 2–4)
- Signature changes to existing methods; connection-pool or WAL tuning beyond current `busy_timeout=5000`

## Capabilities

### New Capabilities
- None — no new spec domain; additive method variants on existing store.

### Modified Capabilities
- `bigmem`: Store read/write/query operations gain cancellable context-aware variants with timeout and explicit cancellation errors.

## Approach

Add `*Ctx(ctx, ...)` variants sharing one internal timeout helper (`WithTimeout` default e.g. 5s, caller-overridable). Existing methods become thin wrappers. Use driver context methods end-to-end; return wrapped `ctx.Err()` on cancel/deadline. Migrate only the 3 listed consumers; all other callers untouched.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/bigmem/bigmem.go` | Modified | Add `SaveCtx/GetCtx/SearchCtx/UpdateCtx/DeleteCtx`; wrap existing |
| `internal/bigmem/full.go` | Modified | Add `SessionContextCtx/TimelineCtx/SavePromptCtx`; wrap existing |
| `cmd/biggz-mcp/main.go` | Modified | Use `*Ctx` with request ctx |
| `internal/sdd/session_guard.go` | Modified | Use `*Ctx`; keep `select ctx.Done` pre-check |
| `internal/doctor/bigmem.go` | Modified | Use `*Ctx` instead of raw `QueryContext` on db |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Timeout too aggressive breaks slow FTS | Med | Generous default; per-call override; measure before tuning |
| Partial migration leaves two paths | Low | P0 consumer list fixed; grep `\.Save\|\.Search\|\.Get\(` in verify |

## Rollback Plan

Revert commit; `*Ctx` methods are purely additive so deletion is safe. Wrappers restore original bodies. No schema/migration involved — zero data risk.

## Dependencies

- None. `database/sql` context support already used by `doctor` package.

## Success Criteria

- [ ] `rg "context.Context" internal/bigmem/` non-empty; existing signatures unchanged
- [ ] Cancelled ctx returns visible error (no silent fallback)
- [ ] 3 consumers call `*Ctx`; `go build ./...` + `go test ./internal/bigmem/` pass
