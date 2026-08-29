# Tasks: Parity Gentle 69 — Ledger Budget

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 420–560 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1 → PR2 → PR3 stacked-to-main |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | CAS verify-before-commit | PR1 base `main` ≤30 lines | `go test ./internal/sddattempt -run TestCAS -count=1` | `go test ./internal/sddattempt -count=1` stale R0 rejected | `git revert` `cas_store.go` replay block |
| 2 | Dual-budget single predicate + 2× cap | PR2 base PR1 <400 | `go test ./internal/sddattempt -run TestBudget\|TestRefund -count=1` | `go test ./internal/sddattempt -run TestAcquireSettleBudget -count=1` | `git revert` `sddattempt.go` fields/predicate/cap |
| 3 | Locator hybrid + rescope wedge + taxonomy | PR3 base PR2 <400 | `go test ./internal/sdd -run TestDeclaredStore\|TestRescope -count=1` | `go test ./internal/sdd ./internal/review -count=1` + `biggz sdd-status --json` | `git revert` `status.go` `engram_status.go` `Rescope` `capture.go` |

## Phase 1: PR1 — Ledger Verify-Before-Commit

- [x] 1.1 RED `cas_store_test.go`: stale R0 vs HEAD R1 must fail CAS HEAD stays R1 (`go test -run TestCASRefusesStale -count=1`)
- [x] 1.2 Modify `internal/sddattempt/cas_store.go:375` `commit()`: replay `loadRecord(revision)` inside `withStoreLock` before `writeLedgerHead`; mismatch fail closed
- [x] 1.3 GREEN concurrent serialize R1→R2 vs R1→R3 second rejected (`go test -run TestCAS -count=1`)
- [x] 1.4 RED Git selection: worktree commondir `<common>/biggz/sdd-runtime/v1`; permission error not `isNotGitRepoError` (`go test -run TestWorktreeCommonDir -count=1`)

## Phase 2: PR2 — Dual Budget + Refund

- [x] 2.1 Add `RuntimeAttempt.ChangedLines` + `RuntimeStore.CumulativeChangedLines` `omitempty` in `internal/sddattempt/sddattempt.go`
- [x] 2.2 Add `runtimeChangedLineBudgetExceeded(s,d) bool {s.CumulativeChangedLines+d>s.MaxLines}`; wire `Acquire`/`Finish`/`Settle` single predicate
- [x] 2.3 Add `runtimeAttemptDeliveredIncrement` + `runtimeRefundedAttempts<=MaxAttempts` cap `2×`; wire `Acquire/Begin` blocked(budget_exhausted) at cap
- [x] 2.4 GREEN budget `300/400+150` blocked `+80` ok cum380 (`go test -run TestDualBudget -count=1`); refund `interrupted20` counts `0` refund-eligible `3/3` blocks (`go test -run TestRefund -count=1`)
- [x] 2.5 GREEN `RuntimeRecordRejectedError` typed `errors.As` for hash/schema/lineage stale; no string-only path (`go test -run TestRecordRejected -count=1`)

## Phase 3: PR3 — Locator Hybrid + Rescope + Taxonomy

- [x] 3.1 Add `declaredArtifactStore(ws)` in `internal/sdd/status.go`: read `openspec/config.yaml` `sdd.artifact_store`, `NormalizeArtifactStore`, missing→`openspec`, `none`→empty
- [x] 3.2 Refactor `resolveArtifactPaths(root,store)` + `bigmemArtifactPaths` + `collectBigMemChangesWithArchive` filesystem-wins (`go test -run TestDeclaredStore -count=1`)
- [x] 3.3 Fix `Rescope()` `internal/sddattempt/sddattempt.go:1973`: wedge `newMaxAttempts>cumAttempts && newMaxLines>cumLines`; `5/600→5/700` reject `7/800` admit preserve slice (`go test -run TestRescopeWedge -count=1`)
- [x] 3.4 Modify `internal/review/capture.go`: `wrapRuntimeCandidateUnavailable` + `Binary files differ` typed unavailable (`go test -run TestCaptureUnavailable -count=1`)

## Phase 4: Verification

- [x] 4.1 Integration hybrid+budget+rescope: `go test ./internal/sdd -run TestHybridWins -count=1` + `TestRescopeCumulativePreserved -count=1`
- [x] 4.2 FIXED gate: `go test ./internal/sddattempt ./internal/sdd ./internal/review -count=1` + `go vet` verify `domainHash`+lp, `GitCommonDir/v1/events`, `flock LOCK`, `burned.json` unchanged
- [x] 4.3 E2E `go test ./... -count=1 -timeout 180s` per PR; `git diff --stat` PR1 ≤30 PR2/PR3 <400; work-unit commits keep tests+code
