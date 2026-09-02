# Design: Fix BigMem Session Discipline

## Context

`2026-09-02` reached `done` without `biggz_mem_session_summary`; manual `obs-1788387626730819800-1` recovered. No gate, no CLI fallback when MCP `biggz_mem_*` missing, no `context(5)+search` verify. Incorporates `gentle-ai/protocol.md` SESSION CLOSE + `engram/store.go` (~8897L: `SessionActivity` 10m/20 calls, 5-case `DetectProjectFull`, 15m dedup, `PutBlob >100k`/`data:image/`, `EndSession`, empty-query `updated_at DESC` vs FTS `rank`).

## Goals

- Gate `done`/batch-close on verified `session_summary` (B1/S1).
- Bash `biggz bigmem save --type session_summary` when `available_tools` lacks `biggz_mem_*` (B2/S2).
- Verify `biggz_mem_context(5)` + `search --query ""`; fallback `git log -15` + `sdd-status --json` when empty (B3/S3).
- Complementary per-task `save` + `session_summary` (B4/S4).
- Retry-once + `session-fallback.md` + delivery guarantee saving≠replying (B5/S5/O3).

## Non-Goals

Backfill old sessions, `sync`/engram pipeline, auto-GC.

## Technical Approach

Two slices <400 lines:

**PR1 Gate+fallback** — `internal/sdd/session_guard.go` (`HasSessionSummary`, `SaveWithFallback`, `Verify`). Orchestrator hook before `apply` batch close and final `done`; if missing, use MCP `session_summary` else bash `biggz bigmem save --type session_summary` (mandatory). Returns `blocked(session_summary_missing)` + `needs_decision`. Reuses `topic_key`/`type` validation, `capture_prompt:false`.

**PR2 Verify+docs** — post-save `context(5)` then `search --query "" --limit 5` (`Search("",opts)` `updated_at DESC`). Miss → retry once → fallback file + deliver with note + next-session retry. Update `bigmem-protocol.md`, workflow, `docs/architecture.md`.

## Architecture Decisions

| Decision | Choice | Alternatives | Rationale |
|---|---|---|---|
| Gate | `session_guard.go` + hook; `resolve-blockers` on block | In `sdd-apply` only | Covers `done`+batch; mirrors `synthesis_gate.go` |
| Fallback | Bash `biggz bigmem save` when MCP absent | Require MCP | Converged `ShouldExternalize`; Q2 mandatory |
| Verify | `context(5)` → `search ""` DESC → git-log fallback | Single `search` | Cheap recency, avoids FTS drift, handles fresh clone |
| Complementary | Per-task `save` (dedup 15m, 10m nudge, `PutBlob>100k`) + summary | Summary replaces tasks | Different granularity; ports engram 5-case/dedup |
| Retry | Once → degraded file → deliver | Block forever | Saving≠replying; handles transient vs persistent |

## Data Flow

```
sub-agent → per-task save → synthesis
                    ↓
hook → HasSessionSummary? —no→ MCP/bash save → Verify context(5)+search → allow
                                    └miss→retry→fallback+note
```

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/sdd/session_guard.go` | Create | Guard/verify/fallback/retry |
| `internal/assets/biggz/bigmem-protocol.md` | Modify | SESSION CLOSE, fallback, verify table |
| `internal/assets/biggz/biggz-orchestrator-workflow.md` | Modify | Pre-done hook `blocked(session_summary_missing)` |
| `docs/architecture.md` | Modify | Note `session_guard.go` discipline |
| `internal/sdd/status.go` | Modify | Surface `session_summary_missing` if needed |

## Interfaces / Contracts

```go
func HasSessionSummary(ctx context.Context, project, sessionID string) (bool, error)
func VerifySessionSummary(ctx context.Context, project string) (bool, error) // context(5)+Search("",...)
func SaveSessionSummaryWithFallback(ctx context.Context, project, sessionID, content string, hasMCP bool) (string, error)
func FallbackPath(change string) string // openspec/changes/{change}/session-fallback.md
// fallback when hasMCP==false:
biggz bigmem save --type session_summary --scope project --project <proj> --json
// verify: biggz_mem_context(5) + biggz bigmem search --query "" --limit 5 --json
// fallback when empty: git log --oneline -15 + biggz sdd-status --json --instructions
```

## Alternatives Considered

| Alt | Tradeoff | Verdict |
|---|---|---|
| Apply-only gate | misses `done` | Rejected |
| Infinite retry | blocks reply | Rejected |
| Backfill migration | scope creep | Rejected |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Transient outage → false block | Med | Retry + degraded file + next-session retry |
| CLI/MCP drift | Low | Shared validators + e2e both paths |
| Verify latency | Low | Limit 5, cached, <200ms p95 |

## Threat Matrix

Bash subprocess applicable; VCS rows N/A.

| Boundary | Applicability | Response | RED tests |
|---|---|---|---|
| Documentation-like paths | N/A — no execution classification | — | — |
| Git repository selection | Applicable | Fallback anchored to `workspaceRoot` | RED: `workspaceRoot=/tmp/other` → correct log |
| Commit state | N/A — no index mutation | — | — |
| Push state | N/A — no push handling | — | — |
| PR commands | N/A — no PR composition | — | — |

## Testing Strategy

| Layer | What | Approach |
|---|---|---|
| Unit | Guard block/allow, routing, retry, fallback | `go test ./internal/sdd -run TestSessionGuard` |
| Integration | `context(5)+search ""` DESC; git-log fallback; blob>100k | `recall_test.go` pattern + save roundtrip |
| E2E | MCP→MCP; no MCP→bash persists+verifies; fail→deliver+note | Hook dry-run + `search --query ""` |

Delivery guarantee: failure still delivers answer with `BigMem unavailable — fallback persisted`.

## Migration / Rollout

No migration. `git revert` 2 commits. PR1 then PR2. In-flight uses manual save `obs-1788387626730819800-1`; fallback retried next session.

## Open Questions

- [ ] Surface `session_summary_missing` in `sdd-status` TUI or orchestrator-only?
- [ ] CLI parity for `capture_prompt:false` — explicit flag or implicit?

---
*Covers Q1–Q5, engram/gentle-ai learnings, saving≠replying.*
