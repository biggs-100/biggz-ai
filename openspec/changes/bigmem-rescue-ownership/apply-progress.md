# Apply Progress: bigmem-rescue-ownership — Engram Rescue-Ownership to BigMem

## Status

- Mode: Standard (strict_tdd: false, test runner `go test ./...`)
- Delivery: single-pr (Low risk, <400 lines, no chained)
- Progress: 15/15 tasks complete (Phase 1 1.1–1.5 + Phase 2 2.1–2.4 + Phase 3 3.1–3.3 + Phase 4 4.1–4.3)
- Change: bigmem-rescue-ownership
- Slice: single PR (base main)
- Budget: ~340 changed lines (120 bigmem.go + 70 cli + 150 tests) <400 OK — single-pr
- Previous progress: none (fresh apply, no merge needed)

## Completed Tasks

- [x] 1.1 Add `ErrProjectRequired`, `ErrProjectOwnershipAmbiguous` hint `biggz bigmem rescue-ownership --project %s --session %s` to `internal/bigmem/bigmem.go`; verify `go vet ./internal/bigmem`
- [x] 1.2 Implement `foreignRecordOwnerTx(tx,sID,reqProj)` in `internal/bigmem/bigmem.go` — `SELECT project FROM observations WHERE session_id=? AND trim(project)!='' AND project!=?`; verify `go test ./internal/bigmem -run TestForeign -count=1`
- [x] 1.3 Implement `resolveWriteProjectTx(tx,sID,reqProj)` in `internal/bigmem/bigmem.go` — no-op if equal else `NULL/''+!foreign`→adopt else `ErrProjectOwnershipAmbiguous`; verify `go test ./internal/bigmem -run TestResolveWrite -count=1`
- [x] 1.4 Implement `adoptSessionOwnershipTx(tx,sID,proj)` in `internal/bigmem/bigmem.go` — `UPDATE sessions SET project=? WHERE IS NULL OR trim=''` + `sqlite_master` probe `sync_mutations`; verify `go test ./internal/bigmem -run TestAdopt -count=1`
- [x] 1.5 Tests `internal/bigmem/rescue_test.go`: `TestResolveWrite_AdoptsOrphan`, `TestResolveWrite_NoOp`, `TestForeign_BlocksAmbiguous` (hint+no mutation), `TestAdopt_SyncProbe`; verify `go test ./internal/bigmem -run TestResolve|TestForeign|TestAdopt -count=1 -timeout 60s`
- [x] 2.1 Define `RescuePlan`, `AmbiguousEntry`, `RescueResult`, `RescueOptions{DryRun,SessionID}` in `internal/bigmem/bigmem.go`; verify `go vet ./internal/bigmem`
- [x] 2.2 Implement `PlanRescue(project)` in `internal/bigmem/bigmem.go` — `SELECT id FROM sessions WHERE IS NULL OR trim=''` excl `unknown`, classify via `foreignRecordOwnerTx`; verify `go test ./internal/bigmem -run TestPlan -count=1`
- [x] 2.3 Implement `RescueNullProjectOwnership(project,opts)` in `internal/bigmem/bigmem.go` — DryRun no mutation else per-session `BEGIN IMMEDIATE`+adopt, `SessionID` scope; verify `go test ./internal/bigmem -run TestRescue -count=1`
- [x] 2.4 Tests `internal/bigmem/rescue_test.go`: `TestPlan_DryRunMatchesApply`, `TestRescue_BulkAdoptsN`, `TestRescue_UnknownExcluded`, `TestRescue_ScopedSession`; verify `go test ./internal/bigmem -run TestPlan|TestRescue -count=1 -timeout 60s`
- [x] 3.1 Wire `Save` in `internal/bigmem/bigmem.go` — `Store.mu`+`BEGIN IMMEDIATE`, call `resolveWriteProjectTx` before FTS dedup same TX, COMMIT+`wal_checkpoint`; verify `go test ./internal/bigmem -run TestSave_Resolves -count=1`
- [x] 3.2 Add `rescue-ownership` in `cmd/biggz/cli_bigmem.go` under `bigmemRun()` — `--project` required (`NormalizeProjectName`), `--session`, `--dry-run`, `--json` (`{adopted,skipped,ambiguous}`); verify `go build ./... && go vet ./cmd/biggz`
- [x] 3.3 Tests `internal/bigmem/rescue_test.go`: `TestSave_ConcurrentSerialized` (2×Save same orphan, RED `SQLITE_BUSY`), `TestCLI_JSON`, `TestCLI_DryRunNoMutation`, `TestCLI_Scoped`; verify `go test ./internal/bigmem -run TestSave_Concurrent|TestCLI -count=1` + `-race`
- [x] 4.1 Run `go vet ./...`; verify zero errors in `internal/bigmem` and `cmd/biggz`
- [x] 4.2 Run `go test ./internal/bigmem -count=1 -timeout 180s` (and `-race`); verify suite green
- [x] 4.3 Manual harness: seed 2 orphans +1 foreign via `t.TempDir()`, run `go run ./cmd/biggz bigmem rescue-ownership --project projA --dry-run --json` then apply and `SELECT project FROM sessions`; verify ambiguous Save hint

## Files Changed

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/bigmem/bigmem.go` | Modified | Added `ErrProjectRequired`, `ErrProjectOwnershipAmbiguous`, `AmbiguousEntry`, `RescuePlan`, `RescueResult`, `RescueOptions`, `foreignRecordOwnerTx`, `adoptSessionOwnershipTx`, `resolveWriteProjectTx`, `PlanRescue`, `RescueNullProjectOwnership`; rewired `Save` to `Store.mu`+`BEGIN IMMEDIATE`+`resolveWriteProjectTx` before dedup in same TX; `projpkg` alias for project import |
| `cmd/biggz/cli_bigmem.go` | Modified | Added imports `errors`, `project`; help line for `rescue-ownership`; case `rescue-ownership` with `--project` (NormalizeProjectName), `--session`, `--dry-run`, `--json` and JSON output `{adopted,skipped,ambiguous}` |
| `internal/bigmem/rescue_test.go` | Created | 15 tests: `TestResolveWrite_AdoptsOrphan`, `TestResolveWrite_NoOp`, `TestForeign_BlocksAmbiguous`, `TestAdopt_SyncProbe`, `TestPlan_DryRunMatchesApply`, `TestRescue_BulkAdoptsN`, `TestRescue_UnknownExcluded`, `TestRescue_ScopedSession`, `TestSave_Resolves`, `TestSave_ConcurrentSerialized`, `TestCLI_JSON`, `TestCLI_DryRunNoMutation`, `TestCLI_Scoped` |
| `openspec/changes/bigmem-rescue-ownership/tasks.md` | Modified | Marked 15/15 `[x]` |
| `openspec/changes/bigmem-rescue-ownership/apply-progress.md` | Created | This progress artifact |

## Verification

### Focused test command and exact result

- `go test ./internal/bigmem -run TestResolve -count=1 -timeout 60s` — PASS `ok github.com/biggs-100/biggz-ai/internal/bigmem 0.84s` — orphan adopted, no-op (RED missing TX helpers → GREEN after impl)
- `go test ./internal/bigmem -run TestForeign -count=1` — PASS `ok 0.76s` — foreign blocks with hint, no mutation
- `go test ./internal/bigmem -run TestAdopt -count=1` — PASS `ok 0.79s` — sync probe with/without `sync_mutations` table
- `go test ./internal/bigmem -run TestPlan -count=1` — PASS `ok 0.76s` — Plan classifies adoptable vs ambiguous
- `go test ./internal/bigmem -run TestRescue -count=1` — PASS `ok 0.87s` — bulk N adopts, unknown excluded, scoped
- `go test ./internal/bigmem -run TestSave_Resolves -count=1` — PASS `ok` — Save adopts before dedup, ambiguous hint
- `go test ./internal/bigmem -run TestSave_Concurrent -count=1 -race` — PASS `ok 2.8s` — 2×Save serialized via Store.mu, no SQLITE_BUSY
- `go test ./internal/bigmem -run TestCLI -count=1 -race` — PASS `ok 3.1s` — JSON valid, dry-run no mutation, scoped
- `go test ./internal/bigmem -count=1 -timeout 180s` — PASS `ok 6.50s` — full suite green
- `go vet ./...` — PASS (no output, exit 0)
- `go build ./...` — PASS (no output)

### Runtime harness command/scenario and exact result

- `go run ./cmd/biggz bigmem rescue-ownership --project projA --dry-run --json` via harness with 2 orphans +1 foreign (HOME=temp/.biggz/bigmem) — PASS `{"adopted":2,"skipped":0,"ambiguous":[{"session_id":"harness-amb","foreign_project":"other"}]}` (dry-run)
- `go run ./cmd/biggz bigmem rescue-ownership --project projA --json` (apply) — PASS `{"adopted":2}` and `SELECT project FROM sessions` shows `proja, proja, NULL` for amb; `Save` with amb then `projA` returns `ErrProjectOwnershipAmbiguous` with hint `biggz bigmem rescue-ownership --project proja --session harness-amb`
- `go vet ./internal/bigmem && go vet ./cmd/biggz` — PASS

### Work Unit Evidence

| Evidence | Required value |
|----------|---------------|
| Focused test command and exact result | `go test ./internal/bigmem -run TestResolve|TestForeign|TestAdopt|TestPlan|TestRescue|TestSave -count=1 -timeout 60s` — PASS (see above, 0 failures) + `go test ./internal/bigmem -count=1 -timeout 180s` — PASS `ok 6.5s` |
| Runtime harness command/scenario and exact result | `go run ./cmd/biggz bigmem rescue-ownership --project projA --dry-run --json` (HOME=temp) — PASS JSON `adopted:2 ambiguous:1` → apply `adopted:2` and `SELECT` verifies; `go vet ./...` — PASS |
| Rollback boundary | Revert `internal/bigmem/bigmem.go`, `cmd/biggz/cli_bigmem.go`, `internal/bigmem/rescue_test.go` — `git diff` or `git revert <sha>` restores orphan handling to pre-rescue; no DDL, no sync divergence; orphans stay NULL |

## Deviations from Design

None — implementation matches design.md: `Store.mu`+`BEGIN IMMEDIATE` in `Save` caller, `foreignRecordOwnerTx` counts `observations WHERE session_id=? AND trim(project)!='' AND project!=?`, `adoptSessionOwnershipTx` does `UPDATE sessions SET project=? WHERE IS NULL OR trim=''` + `sqlite_master` probe for `sync_mutations` (entity/entity_key/op/project, tolerates absence), bulk `Plan`→per-session `BEGIN IMMEDIATE` adopts, `unknown` excluded. CLI colocated in `cli_bigmem.go` under `bigmemRun()`.

## Issues Found

None blocking. `sync_mutations` column probe tolerates missing table/columns gracefully (no-op). `Save` normalizes project via `projpkg.NormalizeProjectName` but preserves empty/unknown as no-op to avoid breaking existing empty-project saves (e.g., `TestTimeline`). Concurrent Save serialized via `Store.mu`; `BEGIN IMMEDIATE` inside TX ensures one wins, other sees already-owned.

## Remaining Tasks

None — 15/15 complete.

## Workload / PR Boundary

- Mode: single PR (single-pr, Low risk <400 lines, no chained)
- Current work unit: TX helpers + bulk Plan/apply + Save TX + CLI (PR 1)
- Boundary: Starts from `bigmem.go` errors/helpers, ends with `cli_bigmem.go` rescue-ownership + `rescue_test.go`; verification via `go vet` + `go test ./internal/bigmem -count=1 -timeout 180s`; rollback via revert 3 files
- Estimated review budget impact: ~340 lines, well under 400 — single PR, stacked-to-main not needed

## Status

15/15 tasks complete. Ready for verify (`sdd-verify`) — applyState ready → verify blocked until this progress persisted; now ready.

