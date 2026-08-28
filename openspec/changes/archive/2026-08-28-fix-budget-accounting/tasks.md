# Tasks: fix-budget-accounting — Cumulative Correction Budget Ledger

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 250–350 |
| 400-line budget risk | Low |
| 800-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | Single PR (stacked-to-main) |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: stacked-to-main
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Cumulative ledger E2E: receipt field + hash binding + deduction + verify/reconcile | PR 1 | `go test ./internal/review -run TestNextTransition_CorrectionBudgetDeduction -count=1` | `go test ./... -count=1 -timeout 180s` | `git revert <sha>` reverts `finalize.go`, `next_transition.go`, `verify_retry.go`, `reconcile.go`, `correction.go`; legacy receipts valid (0/Empty) |

## Phase 1: Foundation — Receipt Ledger & Hash Binding

- [x] 1.1 Add `CumulativeCorrectionLines int` to `PersistedReceipt` in `internal/review/finalize.go`; normalize missing hash→`EmptyFixDeltaHash` in `readReceiptFile`
- [x] 1.2 Extend `computeHash()` preimage + `Validate()` to bind both fields; reject negative/mismatch
- [x] 1.3 RED: `TestPersistedReceipt_TamperFailsValidate` — mutate 3→4 with old hash must fail (`go test ./internal/review -run TestPersistedReceipt_Tamper -count=1`)
- [x] 1.4 Add helper in `finalize.go` to compute real `sha256:<hex>` delta when correction else `EmptyFixDeltaHash`; extend `finalizeData`/`buildReceipt`

## Phase 2: Core — Budget Deduction & Validation Wiring

- [x] 2.1 RED: `TestNextTransition_BudgetExhaustion` — `budget=3 cum=3→0` then `ValidateCorrectionActual(1,3,3)` fails with `budget` (`go test ./internal/review -run TestNextTransition_BudgetExhaustion -count=1`)
- [x] 2.2 Fix `deriveNextTransition` in `internal/review/next_transition.go:100-108`: `remaining=max(0, budget.CorrectionLines-cumulativeLines)` via `receiptArtifactOf→readReceiptFile`; `nil→0`; verify 10/4→6, 10/10→0
- [x] 2.3 Wire `ValidateCorrectionActual(actual,cumulative,budget)` with persisted cumulative; fail when `cumulative+actual>budget`
- [x] 2.4 Align `internal/review/correction.go` `FixRounds>=MaxFixRounds=3` escalation independent of line budget

## Phase 3: Integration — Verify & Mirror Continuity

- [x] 3.1 Update `finalizeIdempotent` in `internal/review/finalize.go` to preserve `FixDeltaHash`+`cumulativeLines`; assert `ReceiptHash` unchanged
- [x] 3.2 Update `deriveExpectedReceipt`/`reMaterializeReceipt` in `internal/review/verify_retry.go` hash-identical; tampered fails `Validate()` never overwritten
- [x] 3.3 Update `mirrorPayloads`/`receiptMirror`/`gateContextMirror` in `internal/review/reconcile.go` to surface real hash+cumulative; stale check includes them

## Phase 4: Testing & Verification

- [x] 4.1 Add `TestNextTransition_CorrectionBudgetDeduction` — partial 3,2→1, exhausted 10,10→0, nil→0 (`go test ./internal/review -run TestNextTransition -count=1`)
- [x] 4.2 Add `TestPersistedReceipt_*` — hash binding, legacy 0/Empty, real hash after correction (`go test ./internal/review -run TestPersistedReceipt -count=1`)
- [x] 4.3 Add `TestFinalize_IdempotentPreservesCumulative` + `TestRetryFinalVerification_ReMaterializeHashIdentical` (`go test ./internal/review -run TestFinalize -count=1`)
- [x] 4.4 Add `TestStatus_ExposesBudgetRemaining` — `status --json` shows `budget_remaining=1` with cumulative+hash (`go test ./internal/review -run TestStatus -count=1`)
- [x] 4.5 Gate: `go test ./internal/review -count=1 -timeout 180s` + `go test ./... -count=1 -timeout 180s` + `go vet ./...`; verify <800 lines

## Phase 5: Cleanup

- [x] 5.1 Remove debug logs; ensure spec delta matches; run `gofmt`
