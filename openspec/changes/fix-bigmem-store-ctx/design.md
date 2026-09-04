# Design: Context-Aware BigMem Store API

## Technical Approach

Additive `*Ctx` twins hold logic; legacy become `Background()` wrappers. One `WithTimeout` helper enforces timeouts; driver calls use `*Context` methods with explicit `ctx.Err()` mapping. Three consumers migrate; rest untouched. No schema/pool changes (SDDs 2–4 out).

| Req | Answered in |
|-----|-------------|
| CTX-1 core 5 | `bigmem.go` rows, Interfaces |
| CTX-2 extended 3 | `full.go` rows, Interfaces |
| CTX-3 helper | D1, Interfaces |
| CTX-4 | D2–D3, Data Flow |
| CTX-5 consumers | D5, Migration |

## Architecture Decisions

### D1 — Timeout helper

| Option | Tradeoff | Decision |
|--------|----------|----------|
| `WithTimeout(ctx)` → `(ctx, cancel)`; 5s only when no deadline | Caller wins, never extended | **Chosen** in `bigmem.go`; every `*Ctx` defers `cancel()` |
| Per-method inline timeouts | Drifts; violates CTX-3 | Rejected |
| Bare passthrough, no default | Hangs return (WAL/FTS) | Rejected |

### D2 — Wrapper direction

| Option | Tradeoff | Decision |
|--------|----------|----------|
| Legacy → `*Ctx` with `Background()`; logic in `*Ctx` | One path; parity by construction | **Chosen** |
| `*Ctx` → legacy | Logic stays ctx-blind | Rejected |

### D3 — Driver + tx wiring

| Option | Tradeoff | Decision |
|--------|----------|----------|
| `Get/Search/Delete/SessionContext/Timeline/SavePrompt` → `*Context` variants; `SaveCtx` → `BeginTx(ctx)` + `ctx.Err()` checks before lock/commit/on error | `Tx` lacks `*Context` methods; `BeginTx` is the hook | **Chosen.** `UpdateCtx` = `GetCtx`+`SaveCtx`, never legacy. `mu` wait non-cancellable (bounded by `busy_timeout=5000`). |
| ctx into `tx.Exec` | API does not exist | Rejected |

### D4 — Default 5s + measurement

| Option | Tradeoff | Decision |
|--------|----------|----------|
| 5s default, overridable | Matches `busy_timeout=5000`; generous for FTS | **Chosen.** Measure p50/p95 of `Search`/`Save` idle + WAL-contended before tuning; lower only if p95 < 1s, raise if FTS p95 > 4s. |
| Aggressive 2s/500ms | Breaks slow FTS | Rejected |

### D5 — Consumer ctx sources

| Option | Tradeoff | Decision |
|--------|----------|----------|
| `session_guard.go`: → `SessionContextCtx`/`SearchCtx` with inbound ctx; keep `select ctx.Done` pre-check | Preserves CTX-5 scenario | **Chosen** |
| `main.go`: handlers → `*Ctx` with `Background()` | No request ctx in stdio loop; timeout still enforced | **Chosen phase 1** |
| `doctor`: Remedy adds `SearchCtx` probe; `Run` keeps ctx-wired PRAGMA + pre-check | Satisfies CTX-5 grep without inventing `DoctorCtx` (widens CTX-1/2) | **Chosen** |

## Data Flow

Cancelled ctx fails before SQLite; driver errors map to wrapped `ctx.Err()`.

    caller ── *Ctx ── WithTimeout ── ctx.Err? ── mu ── QueryContext/ExecContext/BeginTx
      │                   │ (caller wins; 5s if none)           ▼
      │                                                       wrap ctx.Err(), no fallback
      └─ legacy() ── Background() ── *Ctx (identical results)

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/bigmem/bigmem.go` | Modify | `WithTimeout` + 5 `*Ctx`; legacy → wrappers; `*Context`/`BeginTx` wiring |
| `internal/bigmem/full.go` | Modify | 3 `*Ctx`; legacy → wrappers; same wiring |
| `cmd/biggz-mcp/main.go` | Modify | `Save/Search/Get/SavePrompt` → `*Ctx` (`Background()`) |
| `internal/sdd/session_guard.go` | Modify | → `*Ctx` with inbound ctx; keep `Done` pre-check |
| `internal/doctor/bigmem.go` | Modify | Remedy `SearchCtx` probe + pre-check; keep ctx-wired PRAGMA |

## Interfaces / Contracts

```go
func WithTimeout(ctx context.Context) (context.Context, context.CancelFunc)
func (s *Store) SaveCtx(ctx context.Context, obs *Observation, parentID ...string) error
func (s *Store) GetCtx(ctx context.Context, id string) (*Observation, error)
func (s *Store) SearchCtx(ctx context.Context, q string, o SearchOptions) ([]*Observation, error)
func (s *Store) UpdateCtx(ctx context.Context, id string, u map[string]any) (*Observation, error)
func (s *Store) DeleteCtx(ctx context.Context, id string) error
func (s *Store) SessionContextCtx(ctx context.Context, limit int) ([]Session, error)
func (s *Store) TimelineCtx(ctx context.Context, o TimelineOptions) ([]TimelineEntry, error)
func (s *Store) SavePromptCtx(ctx context.Context, content, sessionID string) (*SavedPrompt, error)
// legacy e.g.: func (s *Store) Save(obs *Observation, p ...string) error { return s.SaveCtx(context.Background(), obs, p...) }
// errors: if ctx.Err() != nil → fmt.Errorf("bigmem <op>: %w", ctx.Err()); never silent
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Cancelled ctx → `ctx.Err()` (all 8); parity; default vs override; no plain `Query`/`Exec` | Table tests with cancelled/short ctx; `rg QueryContext\|BeginTx` |
| Integration | `SearchCtx` under WAL contention; round-trip; guard pre-check short-circuits | Temp-DB tests; slow-FTS/lock injection |
| E2E | 3 consumers emit `*Ctx`; build + tests green | `rg "\.SaveCtx\|\.SearchCtx\|\.GetCtx"` per file; `go build`, `go test ./internal/bigmem/` |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR, executable-classification, or process-integration boundary (additive SQLite ctx plumbing only).

## Migration

No migration (no schema; additive). Land helper + 8 `*Ctx` + wrappers, then `session_guard.go` → `main.go` → `doctor` one file at a time with `rg` + build + tests after each. Rollback: revert commit.

## Open Questions

- [ ] Doctor: `SearchCtx` probe vs new `DoctorCtx` twins — confirm scope.
- [ ] MCP per-request cancel via stdio loop now, or `Background()`+timeout as phase 1?
