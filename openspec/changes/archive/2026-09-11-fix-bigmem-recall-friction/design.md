# Design: fix-bigmem-recall-friction

## Technical Approach

Five seam fixes. Repro: `bigmem_recovered` fallback with live `biggz-mcp`; any-mode `gentle-pi` misses. Targets REQ-FR1/SC1/FTS1/GW1..GW3/RR3.

## Architecture Decisions

### D1 — Ghost-WAL liveness probe (REQ-GW1..GW3)

| Option | Tradeoff | Verdict |
|---|---|---|
| O_EXCL probe (current) | misses live holders | rejected |
| process-name scan | indirect, false negatives | rejected |
| Windows share-0 `CreateFile` | violation ⇔ live handle; success ⇔ dead | **chosen** |

`classifyGhostWAL` runs after unchanged `isGhostWAL`: fresh (<5 min) → normal open (no warning/fallback). Stale: proven holder → primary direct; no holder → GW2 reclaim (handle held through `os.Remove`); inconclusive → recovered fallback. Non-Windows: O_EXCL.

### D2 — Searchable close, exactly-once (REQ-SC1)

| Option | Tradeoff | Verdict |
|---|---|---|
| `topic_key` upsert | merges distinct sessions | rejected |
| random id + dedupe window | not exactly-once | rejected |
| PK `session-summary-{session_id}` + `ON CONFLICT DO UPDATE` | one row/session, update-in-place | **chosen** |

`SessionEnd` updates `sessions`, then upserts the observation (type `session_summary`, scope `project`, no `topic_key`). Retry once (50 ms) on lock; persistent failure surfaces an error, `sessions` row preserved. `SaveCtx` routes `type==session_summary && session_id!=""` to the same upsert; bash fallback passes `--session-id`; `tryMCPSave` drops its duplicate save.

### D3 — Full-summary read (REQ-FR1)

| Surface | Current | Decision |
|---|---|---|
| CLI `get <id>` | already full; unknown id → exit 1 | keep + test |
| CLI `context` | 120-char cut | newest summary full; older 120 preview |
| MCP `mem_context` | 150-char cut | newest full; older 150 preview |
| MCP `mem_search` preview | 120 (test-gated) | unchanged |

Previews stay 120; only the latest summary is untruncated. Rejected: all-full (≤5k KPI).

### D4 — FTS sanitization (REQ-FTS1)

Per token: strip `"`, drop letterless tokens, wrap the rest in `"..."`, join `AND` (`all`) or `OR` (`any`). Quoting both modes fixes any-mode hyphen/operator parsing; accents fold via `unicode61`. Zero: MCP returns `{"results":[],"zero_results":true,"hint":...}`; `hint` (retry `match_mode=any`) on `all`-mode zero only; `any`-mode zero signals `zero_results`; CLI mirrors (delta). Ordering/limits unchanged (REQ-RR2).

### D5 — Recall discipline (REQ-RR3)

`biggz-orchestrator-workflow.md` gains: `mem_context(5)` + ≤1 recency call → answer; never FTS chains; keyword search is post-recap. `orchestrator_test.go` markers preserved.

## Data Flow

```
close : SessionEnd → sessions UPDATE → obs upsert(det.id, retry×1) → recall("", t DESC)
open  : isGhostWAL? no (fresh/other) → normal
        yes (stale) → classifyGhostWAL
          ├ proven holder → primary (no warn/fallback)  ├ no holder → remove wal/shm + checkpoint
          └ inconclusive → recovered fallback
search: sanitize tokens → MATCH AND|OR → rank|t DESC → 120 preview
          └ zero → zero_results (hint on all)
```

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/bigmem/bigmem.go` | Modify | ghost classify + policy; FTS sanitize; summary upsert |
| `internal/bigmem/liveness_{windows,other}.go` | Create | share-0 probe (windows) + O_EXCL fallback (other) |
| `internal/bigmem/full.go` | Modify | `SessionEnd` dual-write + retry-once |
| `cmd/biggz-mcp/main.go` | Modify | `mem_context` full latest; zero envelope; close errors |
| `cmd/biggz/cli_bigmem.go` | Modify | `context` full latest; zero hint; `save --session-id` |
| `internal/sdd/session_guard.go` | Modify | drop duplicate save; sessionID to bash |
| `internal/assets/biggz/biggz-orchestrator-workflow.md` | Modify | Recall budget |
| `internal/bigmem/*_test.go`, `cmd/biggz*/…_test.go` | Modify/Create | tests below |

Drift vs proposal: `session_guard.go` (fix #2, duplicate write) and `liveness_*.go` (fix #4, platform split) were not in its Affected Areas.

## Interfaces / Contracts

```go
type ghostClass int // none|stale|liveHolder|inconclusive
func classifyGhostWAL(dbPath string) ghostClass
func probeDBLiveness(dbPath string) (holder, proven bool)
func sanitizeFTSTerms(query, mode string) (fts string, hasTokens bool)
func SessionSummaryObsID(sessionID string) string // "session-summary-"+sessionID
```

## Testing Strategy

| Layer | What | Approach |
|---|---|---|
| Unit | classify/probe matrix; sanitize (hyphen, accent, `AND`/`*`); repeat close = 1 row; locked write retry×1 at store/CLI close (distinct from guard retry); trim; zero envelope; workflow markers | table tests in `internal/bigmem`, `cmd/biggz*`, `internal/assets/biggz` |
| Integration | held handle → holder detected → primary, no warning; stale+probe-ok → reclaim; inconclusive → fallback intact | Windows tests with real files/handles |
| E2E | live `biggz-mcp`: CLI recall → no ghost warning, same store; `go test ./... -count=1 -timeout 180s` green | manual evidence + existing suites |

## Threat Matrix

| Boundary | Applicability |
|---|---|
| Documentation-like paths | N/A — no executable classification |
| Git repository selection | N/A — no git routing |
| Commit state | N/A — no index automation |
| Push state | N/A — no refspec automation |
| PR commands | N/A — no PR automation |

No row models the Windows lock probe — cases are the integration tests.

## Migration / Rollout

No migration; dual-write touches one obs per close; FTS/ghost are query/open-time. Out of scope (follow-up): stale 300-claims in `docs/bigmem-DOCS.md` and baseline `openspec/specs/bigmem/spec.md`.

## Open Questions

None.
