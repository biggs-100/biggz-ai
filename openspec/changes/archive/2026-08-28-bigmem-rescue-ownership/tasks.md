# Tasks: bigmem-rescue-ownership — Engram Rescue-Ownership to BigMem

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 300-360 (120 bigmem.go + 70 cli + 130 tests) |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | single-pr |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | TX helpers + bulk Plan/apply + Save TX + CLI | PR 1 | `go test ./internal/bigmem -run TestResolve -count=1 -timeout 60s` | `go run ./cmd/biggz bigmem rescue-ownership --project projA --dry-run --json` | Revert `internal/bigmem/bigmem.go`, `cmd/biggz/cli_bigmem.go`, `internal/bigmem/rescue_test.go` |

## Phase 1: Foundation — TX Helpers & Errors

- [x] 1.1 Add `ErrProjectRequired`, `ErrProjectOwnershipAmbiguous` hint `biggz bigmem rescue-ownership --project %s --session %s` to `internal/bigmem/bigmem.go`; verify `go vet ./internal/bigmem`
- [x] 1.2 Implement `foreignRecordOwnerTx(tx,sID,reqProj)` in `internal/bigmem/bigmem.go` — `SELECT project FROM observations WHERE session_id=? AND trim(project)!='' AND project!=?`; verify `go test ./internal/bigmem -run TestForeign -count=1`
- [x] 1.3 Implement `resolveWriteProjectTx(tx,sID,reqProj)` in `internal/bigmem/bigmem.go` — no-op if equal else `NULL/''+!foreign`→adopt else `ErrProjectOwnershipAmbiguous`; verify `go test ./internal/bigmem -run TestResolveWrite -count=1`
- [x] 1.4 Implement `adoptSessionOwnershipTx(tx,sID,proj)` in `internal/bigmem/bigmem.go` — `UPDATE sessions SET project=? WHERE IS NULL OR trim=''` + `sqlite_master` probe `sync_mutations`; verify `go test ./internal/bigmem -run TestAdopt -count=1`
- [x] 1.5 Tests `internal/bigmem/rescue_test.go`: `TestResolveWrite_AdoptsOrphan`, `TestResolveWrite_NoOp`, `TestForeign_BlocksAmbiguous` (hint+no mutation), `TestAdopt_SyncProbe`; verify `go test ./internal/bigmem -run TestResolve|TestForeign|TestAdopt -count=1 -timeout 60s`

## Phase 2: Bulk Rescue — Plan + Apply

- [x] 2.1 Define `RescuePlan`, `AmbiguousEntry`, `RescueResult`, `RescueOptions{DryRun,SessionID}` in `internal/bigmem/bigmem.go`; verify `go vet ./internal/bigmem`
- [x] 2.2 Implement `PlanRescue(project)` in `internal/bigmem/bigmem.go` — `SELECT id FROM sessions WHERE IS NULL OR trim=''` excl `unknown`, classify via `foreignRecordOwnerTx`; verify `go test ./internal/bigmem -run TestPlan -count=1`
- [x] 2.3 Implement `RescueNullProjectOwnership(project,opts)` in `internal/bigmem/bigmem.go` — DryRun no mutation else per-session `BEGIN IMMEDIATE`+adopt, `SessionID` scope; verify `go test ./internal/bigmem -run TestRescue -count=1`
- [x] 2.4 Tests `internal/bigmem/rescue_test.go`: `TestPlan_DryRunMatchesApply`, `TestRescue_BulkAdoptsN`, `TestRescue_UnknownExcluded`, `TestRescue_ScopedSession`; verify `go test ./internal/bigmem -run TestPlan|TestRescue -count=1 -timeout 60s`

## Phase 3: Integration — Save TX + CLI

- [x] 3.1 Wire `Save` in `internal/bigmem/bigmem.go` — `Store.mu`+`BEGIN IMMEDIATE`, call `resolveWriteProjectTx` before FTS dedup same TX, COMMIT+`wal_checkpoint`; verify `go test ./internal/bigmem -run TestSave_Resolves -count=1`
- [x] 3.2 Add `rescue-ownership` in `cmd/biggz/cli_bigmem.go` under `bigmemRun()` — `--project` required (`NormalizeProjectName`), `--session`, `--dry-run`, `--json` (`{adopted,skipped,ambiguous}`); verify `go build ./... && go vet ./cmd/biggz`
- [x] 3.3 Tests `internal/bigmem/rescue_test.go`: `TestSave_ConcurrentSerialized` (2×Save same orphan, RED `SQLITE_BUSY`), `TestCLI_JSON`, `TestCLI_DryRunNoMutation`, `TestCLI_Scoped`; verify `go test ./internal/bigmem -run TestSave_Concurrent|TestCLI -count=1` + `-race`

## Phase 4: Verification & Gates

- [x] 4.1 Run `go vet ./...`; verify zero errors in `internal/bigmem` and `cmd/biggz`
- [x] 4.2 Run `go test ./internal/bigmem -count=1 -timeout 180s` (and `-race`); verify suite green
- [x] 4.3 Manual harness: seed 2 orphans +1 foreign via `t.TempDir()`, run `go run ./cmd/biggz bigmem rescue-ownership --project projA --dry-run --json` then apply and `SELECT project FROM sessions`; verify ambiguous Save hint
