# Proposal: fix-bigmem-recall-friction

## Intent

"Where did we leave off?" costs ~30 calls / >300k tokens (measured) instead of 1–2 calls / <5k; five verified defects block cheap recall (`obs-1789015493759808200-1`; approval `obs-1789016082002573700-3`).

## Scope

### In Scope
- **Full summary read**: CLI `bigmem context`/`get` + MCP `mem_context` return the untruncated summary by id in one call.
- **Searchable close**: `SessionEnd`/`mem_session_summary` also persists a `session_summary` observation, so recall finds the latest close.
- **FTS robustness**: multi-token queries with hyphens/accents hit; `match_mode=any` works; zero-result `all` adds a retry hint.
- **Ghost WAL**: live-holder probe replaces the shape-only heuristic; no warning/fallback when `biggz-mcp` is the only holder.
- **Recall discipline** in `biggz-orchestrator-workflow.md`: `mem_context` + ≤1 recency call → answer; never FTS chains.

### Out of Scope
- FTS algorithm/schema replacement, ranking/sync changes, opportunistic refactors.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `bigmem`: read contract, close persistence, FTS semantics, ghost-WAL classification (REQ-GW1..GW3).
- `orchestrator`: workflow recall discipline.

## Approach

- **Read/close** (`internal/bigmem/full.go`, `cmd/biggz-mcp/main.go`, `cmd/biggz/cli_bigmem.go`): full-summary fetch by id; `SessionEnd` dual-writes it idempotently.
- **FTS** (`internal/bigmem/bigmem.go`): per-token sanitization before `MATCH` (hyphens, accents), fix `any` mode, explicit zero-result signal; recency/rank ordering unchanged (REQ-RR2).
- **Ghost WAL** (`internal/bigmem/bigmem.go`): probe primary liveness; reclaim only when provably dead; otherwise open primary without fallback.
- **Docs**: Recall section in `internal/assets/biggz/biggz-orchestrator-workflow.md`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/bigmem/full.go` | Modified | Summary fetch; `SessionEnd` dual-write |
| `internal/bigmem/bigmem.go` | Modified | FTS sanitize/any; ghost-WAL probe, `ResolveDBPath` |
| `cmd/biggz-mcp/main.go` | Modified | `mem_context` full summary + close write |
| `cmd/biggz/cli_bigmem.go` | Modified | `context`/`get` full summary; no fallback warning |
| `internal/assets/biggz/biggz-orchestrator-workflow.md` | Modified | Recall discipline |
| `internal/bigmem/*_test.go`, `cmd/biggz*/*_test.go` | Modified | Regression tests |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Dual-write duplicates closes | Med | Idempotent per session id |
| FTS sanitization shifts ranking | Low | Preserve REQ-RR2 ordering; golden tests |
| Windows lock probe misdetects | Med | Conservative reclaim; live-process tests |
| Doc asset drift | Low | Managed-asset parity checks |

## Rollback Plan

Revert the code diff. No schema migration: dual-write only inserts observations; FTS/ghost-WAL changes are query-time. Persisted rows stay valid. Future persistence changes ship separately.

## Dependencies

- None. `go test ./... -count=1 -timeout 180s` must stay green.

## Success Criteria

- [ ] Clean-session recall in 1–2 calls, <5k tokens.
- [ ] Full latest summary in 1 CLI call / MCP `mem_context`.
- [ ] Latest close findable via recall.
- [ ] Multi-token/hyphenated/accented queries hit; `match_mode=any` works.
- [ ] No ghost-WAL warning/fallback with live `biggz-mcp`.
- [ ] `go test ./... -count=1 -timeout 180s` green.
