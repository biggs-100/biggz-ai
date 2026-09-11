# Tasks: fix-bigmem-recall-friction

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated lines | ~550–850 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | 4 chained PRs |
| Delivery strategy | auto-chain |
| Chain strategy | feature-branch-chain |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

Tracker branch: `fix/bigmem-recall-friction`. PR 1 base = tracker; PR N base = PR N-1 branch; only the tracker merges to main.

Size exception (maintainer-approved, human decision): PR 1 = 676 changed lines vs the 400 budget (slice 1). Phase 1b, driven by verify findings W1/W2, added the cross-platform liveness fix, the seam, the W2 assertion and the residual test fix: **PR 1 total = 849 changed lines** (198 tracked + 651 new). Tests ship with the code, so compressing `ghost_test.go` was rejected over losing reclaim-keeps-data and checkpoint-error coverage. Slices 2-4 keep the 400-line target.

### Suggested Work Units

| Unit | Goal | Likely PR | Base | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|------|----------------------|-----------------|-------------------|
| 1 | Ghost-WAL liveness (#4) | PR 1 | tracker `fix/bigmem-recall-friction` | `go test ./internal/bigmem -run 'Ghost\|Liveness\|ReclaimCheckpoint'` | live `biggz-mcp` holder: CLI recall → primary | liveness files + classify; revert → shape-only |
| 2 | Searchable close (#1) | PR 2 | base = PR 1 branch | `go test ./internal/bigmem ./internal/sdd ./cmd/biggz -run 'Session\|Close'` | double `save --type session_summary --session-id S` → one row | dual-write/upsert + guard dedupe; revert → sessions-only |
| 3 | FTS sanitize (#3) | PR 3 | base = PR 2 branch | `go test ./internal/bigmem ./cmd/biggz-mcp -run 'FTS\|Match\|Search'` | `bigmem search --query "gentle-pi ruido" --match-mode any` → hits | sanitize + envelopes; revert → raw MATCH |
| 4 | Summary read + recall doc (#1/#5) | PR 4 | base = PR 3 branch | `go test ./cmd/biggz ./cmd/biggz-mcp ./internal/assets/biggz -run 'Context\|Summary\|Orchestrator'` | seeded long summary → full untruncated text | read paths + asset; revert → 120/150 cuts |

## Phase 1: Ghost-WAL liveness · PR 1

- [x] 1.1 RED `internal/bigmem/ghost_test.go`: classify/probe matrix (fresh, stale-idle, live-holder, inconclusive) (REQ-GW1).
- [x] 1.2 Create `internal/bigmem/liveness_windows.go` (share-0 probe) + `liveness_other.go` (O_EXCL).
- [x] 1.3 `internal/bigmem/bigmem.go`: `classifyGhostWAL` — live holder → primary; dead → GW2 reclaim (Remove + `wal_checkpoint(TRUNCATE)`); inconclusive → recovered (REQ-GW2/GW3).
- [x] 1.4 Integration: held handle → no removal, no fallback; checkpoint error never fails Open.

## Phase 1b: Cross-platform liveness honesty · PR 1 (verify findings W1/W2)

Driven by `verify-report.md`: off Windows the O_EXCL probe cannot observe a live holder, yet it reported `proven=true` (dead), so a live holder's wal/shm could be removed (`os.Remove` succeeds on Unix despite open handles). REQ-GW2 requires the probe to *prove* death; REQ-GW3 requires "no live holder proven" to be inconclusive (keep wal/shm + recovered fallback).

- [x] 1b.1 RED `internal/bigmem/ghost_test.go`: non-Windows probe MUST report inconclusive (never proven-dead); live holder → wal/shm preserved, no removal (REQ-GW2/GW3).
- [x] 1b.2 `internal/bigmem/liveness_other.go`: O_EXCL success → `holder=false, proven=false` (inconclusive). Document that no holder is observable off Windows.
- [x] 1b.3 `internal/bigmem/bigmem.go`: add `ghostProbeDBLiveness` seam over `probeDBLiveness` so tests can force inconclusive.
- [x] 1b.4 Assertion (W2): forced inconclusive → ghost warning emitted, recovered path returned, wal/shm untouched (closes the runtime-only gap).
- [x] 1b.5 Record the tradeoff + POSIX lock-probe follow-up (out of scope, keeps Linux reclaiming) in `apply-progress.md`.

## Phase 2: Searchable close · PR 2

- [x] 2.1 RED: repeat close S → one `session_summary` row; empty-query recency (`updated_at DESC`) finds it (REQ-SC1).
- [x] 2.2 `internal/bigmem/bigmem.go`: `SessionSummaryObsID` + upsert `ON CONFLICT DO UPDATE`; `SaveCtx` routes `session_summary`+`session_id`.
- [x] 2.3 `internal/bigmem/full.go`: `SessionEnd` dual-write, retry once (50 ms); failure → explicit error, `sessions` kept.
- [x] 2.4 `internal/sdd/session_guard.go`: drop duplicate save, pass `--session-id`; `cli_bigmem.go`: `save --session-id`.

## Phase 3: FTS sanitization · PR 3

- [x] 3.1 RED: sanitize table (hyphen `gentle-pi`, accent `sesión`, operators, letterless); any-mode hits; zero signal+hint; REQ-RR2 ordering.
- [x] 3.2 `internal/bigmem/bigmem.go`: per-token sanitize (strip `"`, quote, join AND/OR); explicit zero-result signal.
- [x] 3.3 `cmd/biggz-mcp/main.go` zero envelope (`zero_results`+`hint`); `cli_bigmem.go` mirrors hint.

## Phase 4: Summary read + recall discipline · PR 4

- [x] 4.1 RED: >150-char summary untruncated (MCP `mem_context`/CLI `context`); unknown id → non-zero; previews stay 120.
- [x] 4.2 `cmd/biggz-mcp/main.go`: newest summary full, older 150 preview; `cli_bigmem.go`: newest full, older 120 preview.
- [x] 4.3 `internal/assets/biggz/biggz-orchestrator-workflow.md`: Recall discipline (`mem_context(5)` + ≤1 recency call; never FTS chains); `orchestrator_test.go` markers kept.

## Phase 5: Verification · PR 4

- [ ] 5.1 `go test ./... -count=1 -timeout 180s` green.
- [ ] 5.2 E2E live `biggz-mcp`: CLI recall → no ghost warning, same store; recall ≤2 calls.
