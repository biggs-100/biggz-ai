# Tasks: Context-Aware BigMem Store API

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 450-600 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1 → PR2 → PR3 (see Work Units) |
| Delivery strategy | auto-chain |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | WithTimeout + 5 core Ctx in bigmem.go (CTX-1/3/4) | PR1 | `go test ./internal/bigmem/ -run 'TestSaveCtx\|TestSearchCtx' -count=1` | `go test ./internal/bigmem/ -count=1` | Revert `internal/bigmem/bigmem.go` only |
| 2 | 3 extended Ctx in full.go + 8 wrappers parity (CTX-2/4) | PR2 | `go test ./internal/bigmem/ -run 'TestSessionContextCtx\|TestParity' -count=1` | `go test ./internal/bigmem/ -count=1` | Revert `internal/bigmem/full.go` only |
| 3 | 3-consumer migration + full verification (CTX-5) | PR3 | `go build ./... && go test ./internal/bigmem/ -count=1` | `biggz-mcp` stdio search + `biggz doctor` run | Revert 3 consumer files only |

## Phase 1: Foundation — timeout + core Ctx

- [x] 1.1 Add `WithTimeout` helper in `internal/bigmem/bigmem.go` (5s default, caller deadline wins) (CTX-3)
- [x] 1.2 Add `SaveCtx/GetCtx/SearchCtx/UpdateCtx/DeleteCtx` in `internal/bigmem/bigmem.go` via `WithTimeout` (CTX-1)
- [x] 1.3 Wire `QueryContext/ExecContext/QueryRowContext/BeginTx` + wrapped `ctx.Err()` in 5 core methods, no plain Query/Exec (CTX-4)

## Phase 2: Extended Ctx + wrappers

- [x] 2.1 Add `SessionContextCtx/TimelineCtx/SavePromptCtx` in `internal/bigmem/full.go` with same pattern (CTX-2)
- [x] 2.2 Convert 8 legacy methods to `Background()` wrappers delegating to Ctx twins (CTX-4)
- [x] 2.3 Verify `rg 'Query\(|Exec\('` finds no plain calls on Store paths in `bigmem.go`/`full.go` (CTX-4)

## Phase 3: Consumer migration to Ctx

- [x] 3.1 Migrate `internal/sdd/session_guard.go` to `SearchCtx/SessionContextCtx` with inbound ctx, keep `select ctx.Done` pre-check (CTX-5)
- [x] 3.2 Migrate `cmd/biggz-mcp/main.go` handlers to `*Ctx` with `Background()` (CTX-5)
- [x] 3.3 Migrate `internal/doctor/bigmem.go` Remedy to `SearchCtx` probe, keep PRAGMA wiring + pre-check (CTX-5)

## Phase 4: Testing / Verification

- [x] 4.1 Add unit table tests in `internal/bigmem/`: cancelled ctx errors for all 8, parity, default vs override (CTX-1..CTX-4)
- [x] 4.2 Add integration tests on temp DB: `SearchCtx` under WAL contention, round-trip, guard pre-check short-circuits (CTX-4/5)
- [x] 4.3 Run e2e: `rg '*Ctx'` per consumer file, `go build ./...`, `go test ./internal/bigmem/` green (CTX-5)
