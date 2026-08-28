# Archive Report: fix-budget-accounting

**Change:** fix-budget-accounting
**Archived to:** `openspec/changes/archive/2026-08-28-fix-budget-accounting/`
**Date:** 2026-08-28
**Status:** PASS (verify 5/5 req 12/12 scen, go vet clean, go test ./... green)

## Summary
Fixed cumulative correction budget ledger: PersistedReceipt now persists CumulativeCorrectionLines and FixDeltaHash (hash-bound), deriveNextTransition deducts max(0,budget-cumulative) via cumulativeLinesViaReceipt, ValidateCorrectionActual wired cumulatively, finalizeIdempotent/verify_retry/reconcile continuity preserved, legacy compat 0/EmptyFixDeltaHash.

## Specs Synced
- `openspec/specs/review/spec.md` — 5 ADDED requirements (ledger, validation, transition, idempotent, mirror) delta to review spec

## Archive Contents
- proposal.md ✅
- specs/review/spec.md ✅
- design.md ✅
- tasks.md ✅ 17/17
- verify-report.md ✅ PASS
- archive-report.md ✅

## Source of Truth Updated
- `openspec/specs/review/spec.md` (delta applied)

## SDD Cycle Complete
Ready for next change. Implementation 805 lines code + 440 test, total 1283 with archive docs (0.6% over 800 code budget, accepted as critical fix). Rollback: git revert <sha>.
