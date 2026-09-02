# Proposal: Fix BigMem Session Discipline

## Proposal question round — Interactivo Mode

Questions via `contact_supervisor` interview timed out. Assumptions below — reply `correct` / `second-round` / `continue`:

- **Q1 Timing**: `session_summary` before every closing `apply` batch AND final `done`
- **Q2 Fallback**: Mandatory `biggz bigmem save` via bash when MCP tool missing
- **Q3 Verify**: `biggz_mem_context(5)` + `search` must show persisted session before `done`
- **Q4 Scope**: Complementary — per-task `save` + `session_summary` on close
- **Q5 Failure**: Retry once, then deliver with brief note; retry next session

## Intent

Session `2026-09-02` reached `done` without `biggz_mem_session_summary`; recovered manually (obs-1788387626730819800-1). Harden discipline: block `done` without summary, add CLI fallback, verify via context/search.

## Scope

### In Scope
- Gate: block `done` without `session_summary` (or CLI fallback)
- Fallback `biggz bigmem save --type session_summary` via bash
- Pre-done verify `context`+`search` + retry-once policy
- Docs: `bigmem-protocol.md` + orchestrator workflow

### Out of Scope
- Backfill old sessions, `sync`/`engram` pipeline changes, auto-GC

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `bigmem`: session-close invariant, fallback, and verification rules
- `sdd`: orchestrator gate wiring session-summary before apply/done continuation

## Approach

Stacked PRs (2 slices, <400 lines each):
- **PR1 Gate+fallback**: `internal/sdd/session_guard.go`, orchestrator pre-done hook, bash fallback, retry-once
- **PR2 Verify+docs**: `context`/`search` readback, `needs decision` on failure, protocol docs update

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/sdd/*` | Modified | Session guard, orchestrator close gate |
| `internal/bigmem/*` | Modified | Fallback CLI path, checkpoint handling |
| `internal/assets/biggz/bigmem-protocol.md` | Modified | Document fallback + verification table |
| `docs/architecture.md` | Modified | Session discipline note |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| False block when BigMem temporarily unavailable | Medium | Retry once, degraded fallback to file + next-session retry |
| CLI fallback diverges from MCP schema | Low | Reuse `topic_key`/`type` validation, add e2e for both paths |
| Verification adds latency to `done` | Low | Context/search limited to 5, cached, <200ms p95 |

## Rollback Plan

`git revert` the 2 PR commits; no DB migration. In-flight sessions fall back to prior `biggz sdd-status` + manual `biggz bigmem save` (as done for obs-1788387626730819800-1). Remove `session_guard.go` and docs delta.

## Dependencies

- `biggz bigmem` CLI (`save`/`context`/`search`) available on PATH
- `internal/sdd/synthesis_gate.go` markers unchanged

## Success Criteria

- [ ] `done` without prior `biggz_mem_session_summary` (or CLI fallback) is rejected by gate
- [ ] When MCP tool absent, bash `biggz bigmem save` persists session and `search` verifies it
- [ ] Session `2026-09-02` recovery pattern no longer needed — new sessions appear in `biggz_mem_context` before close
- [ ] `go test ./...` and doc checks remain green
