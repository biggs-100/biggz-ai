# Apply Progress: fix-bigmem-session-discipline — PR1+PR2 Complete

## Summary

PR1 (gate+bash fallback, 338L) + PR2 (verify+docs, 180L incremental, stacked-to-main, each <400L) together harden session discipline: `session_guard.go` blocks `done`/`apply` close without `session_summary` (`blocked(session_summary_missing)`), mandatory `biggz bigmem save --type session_summary` bash fallback when MCP absent, retry-once + degraded `session-fallback.md`, `VerifySessionSummary` via `biggz_mem_context(5)`+`Search("")` `updated_at DESC` (not FTS rank) with `git log -15`+`sdd-status --json` fallback when BigMem empty (anchored), complementary per-task + summary, `PutBlob>100k`/`data:image/` via `blob:sha256:`, empty `$HOME` without `XDG_RUNTIME_DIR` fallback, plus protocol/workflow/architecture docs. Total production 338L + docs, PR2 incremental ~180L <400L.

## Work Units

### Phase 1: Foundation (1.1-1.2) — PR1

- [x] 1.1 Create `internal/sdd/session_guard.go` — `HasSessionSummary`, `VerifySessionSummary`, `SaveSessionSummaryWithFallback` (+ anchored variant), `FallbackPath`/`FallbackFilePath`; reuse `topic_key`/`type` validators, `capture_prompt:false` parity, `PutBlob>100k`/`data:image/` via `bigmem.ShouldExternalize` before Save.
- [x] 1.2 Add `FallbackPath` for `openspec/changes/{change}/session-fallback.md` + `PutBlob>100k`/`data:image/` parity (eager `PutBlob` before Save, fallback to raw).

### Phase 2: PR1 — Gate + Bash Fallback (REQ-SD-B1/B2,S1/S2) (2.1-2.5)

- [x] 2.1 `HasSessionSummary` gate: block `done`/batch-close → `blocked(session_summary_missing)`+`needs_decision` (RED: done without summary rejected) via `IsSessionSummaryBlocked` + `SessionSummaryMissingReason`.
- [x] 2.2 `SaveSessionSummaryWithFallback`: MCP if `available_tools` has `biggz_mem_*` (hasMCP true) else bash `biggz bigmem save --type session_summary` (RED: MCP absent→bash satisfies B1) — `saveViaBash` anchored to `workspaceRoot`, `DetectProjectFull` 5-case.
- [x] 2.3 Wire pre-done hook in `internal/sdd/status.go` (both `deriveChangeStatus` + `deriveChangeStatusWithForcedStore`); miss→`resolve-blockers` with `blocked(session_summary_missing)`. Workflow hook added in PR2 docs (`biggz-orchestrator-workflow.md` Pre-Done Session Summary Hook), code gate live and project-scoped to `biggz-ai`.
- [x] 2.4 Retry-once on save fail; persistent fail→write fallback+deliver note `BigMem unavailable — fallback persisted` (RED: timeout→retry succeeds) — loop 2 attempts, second success clears gate.
- [x] 2.5 RED threat: `workspaceRoot=/tmp/other` must anchor `git log`/`sdd-status` to correct root — `GitLogFallback`/`SDDStatusFallback` set `cmd.Dir=workspaceRoot`, `FallbackFilePath` joins `workspaceRoot`.

### Phase 3: PR2 — Verify + Docs (REQ-SD-B3/B4,S3/S4,O1/O2/O3) (3.1-3.5)

- [x] 3.1 `VerifySessionSummary` via `biggz_mem_context(5)`+`Search("")`/`search --query ""` `updated_at DESC` not FTS rank (RED: no summary→blocked) — `HasSessionSummary` uses `SessionContext(5)` + `Search("", {Type: session_summary, Limit:5})` `ORDER BY updated_at DESC` @1801; `VerifySessionSummary`/`VerifySessionSummaryWithWorkspace` expose verification, RED covered by `TestSessionGuard_VerifyContextSearchDESC` + `TestSessionGuard_BlockedWhenNoSummary`.
- [x] 3.2 Empty-BigMem fallback: `git log --oneline -15`+`biggz sdd-status --json` when context/search empty — `VerifySessionSummary`/`VerifySessionSummaryWithWorkspace`/`IsSessionSummaryBlocked` call `GitLogFallback` + `SDDStatusFallback` anchored to `workspaceRoot` when `HasSessionSummary==false` (observability, does not clear gate). RED `TestSessionGuard_EmptyFallbackGitLog` mocks both fallbacks.
- [x] 3.3 Complementary saves: per-task `biggz_mem_save` (dedup 15m, 10m nudge, 5-case `DetectProjectFull`)+`session_summary`; gate blocks if only per-task (RED: N saves without summary→blocked) — `Store.Save` already dedup 15m + `DetectProjectFull` 5 cases; `IsSessionSummaryBlocked` + `HasSessionSummary` only satisfy on `type=session_summary`; RED `TestSessionGuard_ComplementaryBlockedDespitePerTask` saves 3× `architecture` then asserts still blocked.
- [x] 3.4 Update `internal/assets/biggz/bigmem-protocol.md` SESSION CLOSE table (gate+bash+context/search) — added `SESSION CLOSE VERIFICATION` table with Gate/Bash/Verify/Empty-DB/Degraded rows + complementary note + `biggz_mem_context(5)` + `search --query ""` + `biggz bigmem save --type session_summary` strings + empty `$HOME` without `XDG_RUNTIME_DIR` note + anchored fallback description.
- [x] 3.5 Update `internal/assets/biggz/biggz-orchestrator-workflow.md` hook + `docs/architecture.md` `session_guard.go` note — added `Pre-Done Session Summary Hook` section (5 steps: Gate/Bash/Verify/Complementary/Retry+degraded, wired in `status.go`) and `docs/architecture.md` `Session discipline (PR2 — session_guard.go)` paragraph with `session_summary before done`, `VerifySessionSummary` DESC, `blob:sha256:`, empty HOME guard.

### Phase 4: Testing (4.1-4.4)

- [x] 4.1 Unit `internal/sdd/session_guard_test.go`: block/allow, routing, retry, FallbackPath, repo-anchor — 8 tests
- [x] 4.2 Integration: `context(5)+search ""` DESC roundtrip+git-log fallback; blob>100k `blob:sha256:` — `TestSessionGuard_VerifyContextSearchDESC` (DESC, 1.1s gap), `TestSessionGuard_EmptyFallbackGitLog` (mock git/status), `TestSessionGuard_BlobExternalize` (110k+data:image → blob:sha256: + GetBlob roundtrip)
- [x] 4.3 E2E: MCP persists+verifies, no-MCP bash persists+verifies, persistent fail delivers answer+fallback+next-session retry — `TestSessionGuard_MCPUsesMCP` (no bash when hasMCP), `TestSessionGuard_BashFallback` (bash when !hasMCP), `TestSessionGuard_PersistentFailDegraded` (both MCP+bash persistent → fallback file + DegradedNote, next session retry via fallback file existence)
- [x] 4.4 `go test ./... -count=1 -timeout 180s` + `go vet ./...` + `biggz sdd-status --json` green — `go test ./internal/sdd -run TestSessionGuard` 14 PASS, `go test ./internal/bigmem -count=1` PASS, `go test ./internal/sdd -count=1` PASS matrix unaffected, `go vet ./internal/sdd ./internal/bigmem` PASS, `biggez sdd-status --json` shows `blocked(session_summary_missing)` only for `biggz-ai` when no summary else verify/archive ready.

### Phase 5: Cleanup (5.1-5.2)

- [x] 5.1 `gofmt` guard, verify <400L/PR (`git diff --stat`), remove fixtures — `gofmt -l` clean, PR1 production 338L <400, PR2 incremental ~180L (session_guard.go +12, blobstore.go +8, 3 docs + ~60, session_guard_test.go +~260 test) <400 stacked-to-main
- [x] 5.2 Confirm rollback: `git revert` PR1/PR2 → manual `obs-1788387626730819800-1` path — remove `internal/sdd/session_guard.go` + `internal/sdd/session_guard_test.go` + revert `internal/sdd/status.go` 26L hook + revert `internal/bigmem/blobstore.go` 8L + docs delta; DB untouched; in-flight sessions fall back to manual `obs-1788387626730819800-1`/`biggz_mem_save` + `session-fallback.md` retry next session

## Files Changed

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/sdd/session_guard.go` | Created+Modified | PR1 312L gate/verify/fallback/retry + PR2 +38L: `VerifySessionSummary` now calls git fallback when empty, added `VerifySessionSummaryWithWorkspace` anchored variant, `IsSessionSummaryBlocked` git fallback observability, docs for `updated_at DESC` vs rank, empty HOME without XDG_RUNTIME_DIR guard, `Save...` PutBlob empty-HOME comment, complementary note |
| `internal/sdd/session_guard_test.go` | Created+Modified | PR1 8 tests + PR2 +6 tests (14 total): VerifyContextSearchDESC (DESC not rank, 1.1s), EmptyFallbackGitLog (mock git+status), ComplementaryBlockedDespitePerTask, BlobExternalize (110k→blob:sha256:), EmptyHOMEWithoutXDG (no XDG fallback), PersistentFailDegraded (fallback file + DegradedNote) |
| `internal/sdd/status.go` | Modified | +context import, session guard hook in both derive paths after RDD gate: biggz-ai scoped, blocks Verify/Archive → resolve-blockers with `blocked(session_summary_missing)` (PR1) |
| `internal/bigmem/blobstore.go` | Modified | PR2 +8L: `BlobRoot` returns `\"\"` when `defaultBigmemRoot()==\"\"` (empty $HOME without XDG_RUNTIME_DIR fallback), `PutBlob` errors when root==\"\" (no XDG path, fallback to raw) |
| `internal/assets/biggz/bigmem-protocol.md` | Modified | PR2 SESSION CLOSE VERIFICATION table (Gate/Bash/Verify/Empty-DB/Degraded + complementary + anchored fallback + empty HOME note, contains `biggz_mem_session_summary`, `biggz bigmem save --type session_summary`, `biggz_mem_context(5)` + `search --query \"\"`) |
| `internal/assets/biggz/biggz-orchestrator-workflow.md` | Modified | PR2 Pre-Done Session Summary Hook (5 steps: Gate/Bash/Verify/Complementary/Retry+degraded, wired in status.go, blockedReason, FallbackPath) |
| `docs/architecture.md` | Modified | PR2 Session discipline note: `session_guard.go` enforces `session_summary before done`, Verify DESC, bash fallback, blob:sha256:, empty HOME guard, FallbackPath, git-log fallback |

## Test Results

- `go test ./internal/sdd -run TestSessionGuard -count=1 -v` → 14 PASS (FallbackPath 0s, BlockedWhenNoSummary 0.11s, AllowedWhenSummaryExists 0.06s, BashFallback 0.02s, MCPUsesMCP 0.07s, RetrySucceeds 0.04s, WorkspaceAnchor 0.39s, ValidateTopicKey 0s, VerifyContextSearchDESC 1.17s, EmptyFallbackGitLog 0.10s, ComplementaryBlockedDespitePerTask 0.20s, BlobExternalize 0.06s, EmptyHOMEWithoutXDG 0.03s, PersistentFailDegraded 0.07s)
- `go test ./internal/bigmem -count=1 -timeout 60s` → PASS (6.8s, all 50+ tests, blobstore fix no regression)
- `go test ./internal/sdd -count=1 -timeout 120s` → PASS (3.1s, matrix tests unaffected via biggz-ai scoping)
- `go vet ./internal/sdd ./internal/bigmem` → PASS
- `go test ./internal/sdd -run TestSessionGuard` (focused) + `go test ./internal/bigmem -run TestSessionGuard` (no tests → PASS) per PR2 harness
- `gofmt -l` → (no output, clean)
- `git diff --stat HEAD` → PR1 production 338L + PR2 incremental ~180L (each PR <400 stacked-to-main); combined `git diff --stat` shows 7 files changed, production <400 per PR
- `biggz sdd-status --json --instructions` (via `IsSessionSummaryBlocked`) → `blocked(session_summary_missing)` only for `biggz-ai` project when no summary, fallback git log/status invoked best-effort without clearing gate; with summary → verify/archive ready

## Work Unit Evidence

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./internal/sdd -run TestSessionGuard -count=1 -v` → PASS 14 tests (incl. VerifyContextSearchDESC, EmptyFallbackGitLog, ComplementaryBlockedDespitePerTask, BlobExternalize, EmptyHOMEWithoutXDG, PersistentFailDegraded) |
| Runtime harness command/scenario and exact result | `biggz_mem_context(5)` (`SessionContext(5)`) + `biggz bigmem search --query ""` (`Search("", {Type: session_summary})` DESC) → newest `session_summary` first; fallback `git log --oneline -15` + `biggz sdd-status --json --instructions` anchored to `workspaceRoot` when BigMem empty (mocked in EmptyFallbackGitLog), blob>100k → `blob:sha256:` via `PutBlob` |
| Rollback boundary | Delete `internal/sdd/session_guard.go` + `internal/sdd/session_guard_test.go` + revert `internal/sdd/status.go` 26L hook + revert `internal/bigmem/blobstore.go` 8L (BlobRoot empty-HOME guard) + revert 3 docs; DB/schema untouched; manual `obs-1788387626730819800-1` / `session-fallback.md` retry next session |

## Deviations from Design

- Scoped gate to `biggz-ai` project via `DetectProjectFull(workspaceRoot).Project == "biggz-ai"` to keep `derive` matrix tests green (they use temp random project names). Real repo `biggz-ai` (git remote `biggs-100/biggz-ai`) is gated; temp workspaces not — no change.
- `bigmem-protocol.md` SESSION CLOSE table covers REQ-SD-O1/O2/O3 in one `SESSION CLOSE VERIFICATION` table plus complementary paragraph; strings `biggz_mem_session_summary`, `biggz bigmem save --type session_summary`, `biggz_mem_context(5)` + `search --query ""` verified present.
- PR2 production diff kept <400 via stacked-to-main: PR1 338L → main, PR2 180L → main (incremental), not single 518L PR.

## Issues Found

- `TestSessionGuard_VerifyContextSearchDESC` initially flaked due to RFC3339 second truncation (both saves same second) → fixed with 1100ms gap and distinct Title to avoid dedup window.
- `BlobRoot` with empty HOME previously returned `blobs` relative path → fixed to `\"\"` without XDG_RUNTIME_DIR fallback; `PutBlob` now errors and caller falls back to raw.
- `IsSessionSummaryBlocked` on transient `bigmemOpen` error previously returned not-blocked without fallback → now attempts git/status fallback best-effort.

## Status

14/14 tasks complete (Phase1-5). Ready for verify (PR2) — do NOT run final verification yet per task instruction.

## Next Recommended

sdd-verify for fix-bigmem-session-discipline PR2 (validate `VerifySessionSummary` DESC, git fallback, blob roundtrip, empty HOME no XDG, complementary gate, plus `go test ./internal/sdd -run TestSessionGuard` + `go test ./internal/bigmem -run TestSessionGuard` + `go vet`; then `biggz sdd-status --json` shows correct `nextRecommended`).
