# Tasks: Fix BigMem Session Discipline

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 320–420 (1 new +4 mod, ~150 test) |
| 400-line budget risk | Medium |
| Chained PRs recommended | Yes |
| Suggested split | PR1 gate+fallback → PR2 verify+docs |
| Delivery strategy | stacked-to-main |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Gate+fallback (`session_guard.go`, hook, retry) | PR1 → `main` | `go test ./internal/sdd -run TestSessionGuard -count=1` | `biggz bigmem save --type session_summary` + `biggz sdd-status --json` expects `blocked(session_summary_missing)` cleared | Revert `session_guard.go`+hook; manual save fallback |
| 2 | Verify `context(5)+search ""`+docs | PR2 → `main` (base PR1) | `go test ./internal/bigmem -run TestVerifySession -count=1` | `biggz_mem_context(5)`+`search --query ""`; fallback `git log -15` | Revert docs+`status.go`; PR1 stays |

## Phase 1: Foundation

- [x] 1.1 Create `internal/sdd/session_guard.go` — `HasSessionSummary`, `VerifySessionSummary`, `SaveSessionSummaryWithFallback`, `FallbackPath`; reuse `topic_key`/`type` validators, `capture_prompt:false`
- [x] 1.2 Add `FallbackPath` for `openspec/changes/{change}/session-fallback.md` + `PutBlob>100k`/`data:image/` parity

## Phase 2: PR1 — Gate + Bash Fallback (REQ-SD-B1/B2,S1/S2)

- [x] 2.1 `HasSessionSummary` gate: block `done`/batch-close → `blocked(session_summary_missing)`+`needs_decision` (RED: done without summary rejected)
- [x] 2.2 `SaveSessionSummaryWithFallback`: MCP if `available_tools` has `biggz_mem_*` else bash `biggz bigmem save --type session_summary` (RED: MCP absent→bash satisfies B1)
- [x] 2.3 Wire pre-done hook in `internal/sdd/status.go`+workflow; miss→`resolve-blockers`
- [x] 2.4 Retry-once on save fail; persistent fail→write fallback+deliver note `BigMem unavailable — fallback persisted` (RED: timeout→retry succeeds)
- [x] 2.5 RED threat: `workspaceRoot=/tmp/other` must anchor `git log`/`sdd-status` to correct root

## Phase 3: PR2 — Verify + Docs (REQ-SD-B3/B4,S3/S4,O1/O2/O3)

- [x] 3.1 `VerifySessionSummary` via `biggz_mem_context(5)`+`Search("")`/`search --query ""` `updated_at DESC` not FTS rank (RED: no summary→blocked)
- [x] 3.2 Empty-BigMem fallback: `git log --oneline -15`+`biggz sdd-status --json` when context/search empty
- [x] 3.3 Complementary saves: per-task `biggz_mem_save` (dedup 15m, 10m nudge, 5-case `DetectProjectFull`)+`session_summary`; gate blocks if only per-task (RED: N saves without summary→blocked)
- [x] 3.4 Update `internal/assets/biggz/bigmem-protocol.md` SESSION CLOSE table (gate+bash+context/search)
- [x] 3.5 Update `internal/assets/biggz/biggz-orchestrator-workflow.md` hook + `docs/architecture.md` `session_guard.go` note

## Phase 4: Testing

- [x] 4.1 Unit `internal/sdd/session_guard_test.go`: block/allow, routing, retry, FallbackPath, repo-anchor
- [x] 4.2 Integration: `context(5)+search ""` DESC roundtrip+git-log fallback; blob>100k `blob:sha256:`
- [x] 4.3 E2E: MCP persists+verifies, no-MCP bash persists+verifies, persistent fail delivers answer+fallback+next-session retry
- [x] 4.4 `go test ./... -count=1 -timeout 180s` + `go vet ./...` + `biggz sdd-status --json` green

## Phase 5: Cleanup

- [x] 5.1 `gofmt` guard, verify <400L/PR (`git diff --stat`), remove fixtures
- [x] 5.2 Confirm rollback: `git revert` PR1/PR2 → manual `obs-1788387626730819800-1` path
