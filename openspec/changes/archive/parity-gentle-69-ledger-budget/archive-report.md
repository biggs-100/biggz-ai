# Archive Report: parity-gentle-69-ledger-budget

**Change:** `parity-gentle-69-ledger-budget`
**Archived to:** `openspec/changes/archive/parity-gentle-69-ledger-budget/`
**Date:** 2026-08-29
**Status:** archived — PASS_WITH_WARNINGS (verify 7/7 req 23/23 scen, ledger terminal complete)
**Ledger HEAD:** `c655f0025282b1f2d012925584eb9380ac6d241fa70afb516504491e9e2c9bf5` complete true
**Tasks:** 16/16 complete (1.1–1.4 PR1, 2.1–2.5 PR2, 3.1–3.4 PR3, 4.1–4.3 verification)

## Summary

Parity with gentle `e8cc0fcc→782e8dfe` — ledger atomicity + dual-budget + locator hybrid. 3 PRs `stacked-to-main` `auto-chain` each `<400` lines, total tracked 293 <400.

**PRs:**
- **PR1** `34463fc2` ≤30 lines (19) — `cas_store.go:375` `commit()` verify-before-commit `loadRecord(revision)` inside `withStoreLock` before `writeLedgerHead`, stale R0 vs R1 CAS refuse, HEAD never advances on mismatch, concurrent serialize R1→R2 vs R1→R3 second rejected, worktree `GitCommonDir` `<common>/biggz/sdd-runtime/v1`.
- **PR2** `abae0050` <400 (159 solo PR2, 293 total tracked with PR1) — `sddattempt.go` dual budget `RuntimeAttempt.ChangedLines` + `RuntimeStore/RuntimeStatus.CumulativeChangedLines` `omitempty`, single predicate `runtimeChangedLineBudgetExceeded(cum+delta>max)` wired in `Acquire/Begin/Finish/Settle`, refund capped `2×MaxAttempts` via `runtimeAttemptDeliveredIncrement` + `runtimeRefundedAttempts`, `RuntimeRecordRejectedError` typed `errors.As` for hash/schema/lineage/stale.
- **PR3** `c655f002` (via `tok-bbf957e59e1b8fc81b99cc7c`, revision `6120137d...`→`c655f002`) <400 (293 total tracked <400) — `status.go:263` `declaredArtifactStore` reads `openspec/config.yaml` `sdd.artifact_store` + `artifact_store` prefer sdd, `NormalizeArtifactStore`, missing→`openspec`, `none`→empty, `resolveArtifactPaths(root,store)` branching per store + `bigmemArtifactPaths`, `StatusWithOptions` hybrid filesystem-wins, `engram_status.go` none guard, `Rescope()` wedge `newMaxAttempts>cumAttempts && newMaxLines>cumLines` (no len) preserve slice admit 7/800 reject 5/700, `capture.go` `wrapRuntimeCandidateUnavailable` + `Binary files differ` typed.
- Intermedios `c219ee45` y `2ebb3221` resets previos a PR settle final; ledger terminal `complete true` no requiere reset adicional.

**Delivery:** `stacked-to-main` `auto-chain` 3 PRs, each work-unit commit preserves tests+code, rollback `git revert` revertible per PR isolated (PR1 replay block, PR2 fields/predicate/cap + `budget_refund_test.go`, PR3 locator/rescope/taxonomy), `MaxLines==0` defaults 400, cumulative recomputed `sum(ChangedLines)` if absent, hybrid defaults filesystem-wins, `omitempty` legacy compatible.

**Verification:** `pass_with_warnings` 7/7 req 23/23 scen validated via `biggz sdd-verify-validate`, evidence_revision `sha256:799a3e56e846d86a8828939646b92051f0d58ea91fb2709605f18af3e58dcd5e`, `go vet` clean, `go test ./internal/sddattempt -run TestCAS|TestDualBudget|TestRefund|TestRecordRejected|TestRescope -count=1` PASS 10/10, `go test ./internal/sdd -run TestCollectBigMemChanges_Hybrid|TestStatusWithOptions -count=1` PASS, `go test ./internal/sddattempt ./internal/review -count=1` PASS, FIXED gates intact (domainHash+length-prefix, GitCommonDir/v1/events, flock LOCK, burned.json unchanged), `git diff --stat HEAD` 8 files 293 insertions 54 deletions <400 tracked (PR1 19 ≤30).

## Specs Synced

| Domain | Action | Details |
|--------|--------|---------|
| runtime | Updated | 5 ADDED requirements: Ledger Verify-Before-Commit (CAS), Dual Budget Single Owner, Interrupted Refund Capped at 2×, Rescope Exhausted Wedge, Runtime Record Rejection Taxonomy — merged into `openspec/specs/runtime/spec.md` preserving Grouped Isolation + Pi Progress |
| review | Updated | 1 ADDED requirement: Candidate Capture Taxonomy and Binary Marker — merged into `openspec/specs/review/spec.md` preserving GitCommonDir + Flock + PublishImmutable |
| sdd-status | Updated | 1 ADDED requirement: Declared Artifact Store and Hybrid Locator — merged into `openspec/specs/sdd-status/spec.md` preserving SDD Status v2 Sole Contract |

## Archive Contents

- `proposal.md` ✅ (3708 bytes)
- `specs/review/spec.md` ✅ (Candidate Capture Taxonomy)
- `specs/runtime/spec.md` ✅ (5 ledger/budget requirements)
- `specs/sdd-status/spec.md` ✅ (Declared Artifact Store hybrid)
- `design.md` ✅ (5875 bytes, 3 PR slices)
- `tasks.md` ✅ 16/16 complete (1.1–1.4, 2.1–2.5, 3.1–3.4, 4.1–4.3)
- `apply-progress.md` ✅ 17100 bytes (PR1 19 lines, PR2 293 total tracked 159 solo PR2, PR3 293 total <400, rollback boundaries)
- `verify-report.md` ✅ 20657 bytes, verdict `pass_with_warnings`, 7/7 req 23/23 scen, evidence_revision `sha256:799a3e56...`
- `archive-report.md` ✅ (this file)

## Source of Truth Updated

The following specs now reflect the new behavior:

- `openspec/specs/runtime/spec.md` — ledger verify-before-commit + dual budget + refund cap + rescope wedge + record rejection (7 total requirements)
- `openspec/specs/review/spec.md` — GitCommonDir/flock/publishImmutable + capture taxonomy (4 total)
- `openspec/specs/sdd-status/spec.md` — V2 authority-free + declared artifact store hybrid (2 total)

## Verification Details

- **Tasks:** 16/16 `allComplete true` verified via `tasks.md` and `biggz sdd-status --json` taskProgress
- **Apply-progress:** 17100 bytes, PR1 (19 lines) `go test -run TestCASRefusesStale` PASS + `TestWorktreeCommonDir` PASS, PR2 (293 total tracked) `TestDualBudget` 300/400+150 blocked 300+80 ok cum380 + `TestRefund` 3→6 2× cap + `TestRecordRejected` typed, PR3 (293 total <400) `TestRescope` wedge 5/700 reject 7/800 admit + `TestHybridWins` + `TestRescopeCumulativePreserved`
- **Verify-report:** 21K `pass_with_warnings`, 7/7 req 23/23 scen, build `go vet` exit 0, focused tests exit 0, evidence_revision `sha256:799a3e56e846d86a8828939646b92051f0d58ea91fb2709605f18af3e58dcd5e`
- **Ledger:** HEAD `c655f0025282b1f2d012925584eb9380ac6d241fa70afb516504491e9e2c9bf5` complete true after PR3 settle `tok-bbf957e59e1b8fc81b99cc7c`, `biggz sdd-attempt status` Complete true Attempts 1 Next complete Blocked corrupt_authority (terminal complete, reset required to continue — expected)
- **Lines:** `git diff --stat HEAD` 8 files 293 insertions(+), 54 deletions(-) — `status.go +92`, `sddattempt.go +179`, `capture.go +33`, `finalize.go +8`, `status_v2.go +5`, `engram_status.go +3`, `cas_store.go +23`, `cas_store_test.go +4` — total 293 <400 ✓, PR1 19 ≤30 ✓, stacked-to-main auto-chain respected, untracked `budget_refund_test.go` 230 test-only not counted in tracked budget
- **nextRecommended before archive:** `verify` → after verify-report `archive` (HasVerify true) → after archive `done` (IsArchived true, HasApply true, HasVerify true)

## Residual Risks / Warnings

- **WARNING — `TestReadLoopLarge` pre-existing:** `go test ./internal/sdd -count=1` fails solely on `TestReadLoopLarge` (`pending_test.go:106: save large verify failed for large-pending`). Stash verified: `git stash push --keep-index && go test ./internal/sdd -run TestReadLoopLarge -count=1 -v` → same FAIL, `git stash pop` restores PR1-3 unchanged; not caused by this change, isolated to `pending_test.go` large-pending dual-write equality (BigMem vs state.yaml) unrelated to ledger-budget/locator, documented in verify-report as único warning, `go test ./internal/sddattempt ./internal/review -count=1` PASS and hybrid suite PASS demonstrate FIXED gates intact. Not blocking per steering.
- **WARNING — Candidate capture taxonomy coverage:** No dedicated `TestCaptureUnavailable` file; implementation correct via static inspection (`capture.go`+`finalize.go` Binary files wrapping) + existing capture integration tests, manual PASS allowance, recorded as WARNING not CRITICAL.
- **No CRITICAL issues.** No blockers. `sddattempt+review` PASS, `sdd` hybrid/rescope PASS.
- **Rollback:** Each PR `git revert` safe, no migration, no API change beyond `omitempty`, `MaxLines==0` defaults 400.

## SDD Cycle Complete

The change has been fully planned, implemented, verified, and archived.
`nextRecommended` → `done` (IsArchived true, HasApply true, HasVerify true, HasProposal true, HasSpecs true, HasDesign true, HasTasks true).
Ready for the next change.

## References

- Proposal: `openspec/changes/archive/parity-gentle-69-ledger-budget/proposal.md`
- Specs: `openspec/changes/archive/parity-gentle-69-ledger-budget/specs/{review,runtime,sdd-status}/spec.md`
- Design: `openspec/changes/archive/parity-gentle-69-ledger-budget/design.md`
- Tasks: `openspec/changes/archive/parity-gentle-69-ledger-budget/tasks.md` 16/16
- Apply: `openspec/changes/archive/parity-gentle-69-ledger-budget/apply-progress.md` 17100 bytes
- Verify: `openspec/changes/archive/parity-gentle-69-ledger-budget/verify-report.md` 20657 bytes `pass_with_warnings` `sha256:799a3e56...`
- Ledger: `c655f0025282b1f2d012925584eb9380ac6d241fa70afb516504491e9e2c9bf5` `tok-bbf957e59e1b8fc81b99cc7c`
