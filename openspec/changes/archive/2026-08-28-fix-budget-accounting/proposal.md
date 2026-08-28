# Proposal: fix-budget-accounting — Cumulative Correction Budget Ledger

## Intent

Fix cumulative correction budget accounting so `MaxFixRounds` / frozen `CorrectionLines` actually limits total lines across correction rounds. Today `internal/review/next_transition.go:100-108` reports `budget_remaining = budget.CorrectionLines` (full budget still remaining) with comment _"This port has no correction-line consumption accounting (the receipt fix delta stays at EmptyFixDeltaHash)"_ . The receipt's `FixDeltaHash` is always `EmptyFixDeltaHash` (finalize.go:909) and no `cumulativeLines` is persisted, so callers never deduct consumed lines and `ValidateCorrectionActual(actual,cumulative,budget)` is never wired cumulatively. Follow-up to archived `bigmem-ghost-wal` verify which documented this bug.

## Scope

### In Scope
- `internal/review/next_transition.go:100-108` — replace `remaining = budget.CorrectionLines` with `remaining = budget.CorrectionLines - cumulativeLines` (clamped to ≥0), where `cumulativeLines` is derived from persisted receipt + correction events; wire `ValidateCorrectionActual` cumulatively
- `internal/review/finalize.go` — persist `FixDeltaHash` (hash of fix delta, not `EmptyFixDeltaHash` when correction exists) and `cumulativeLines` / attempt counters in `PersistedReceipt`; expose cumulative via `receiptArtifactOf`/chain so `deriveNextTransition` can deduct
- `internal/review/verify_retry.go` and `internal/review/reconcile.go` — persist / surface cumulative accounting consistently (re-materialize path keeps hash binding)
- `internal/review/correction.go` / `model.BudgetCounters` integration — ensure `FixRounds` / `MaxFixRounds` enforcement aligns with cumulative line accounting (use existing `ValidateCorrectionBudget` / `ValidateCorrectionActual`)
- Tests: unit coverage for cumulative deduction, budget exhaustion across rounds, and `FixDeltaHash` persistence
- Keep change small (<800 lines), single PR `stacked-to-main`

### Out of Scope
- `internal/bigmem` ghost WAL, blobstore, branching (already archived)
- SDD orchestration, TUI, MCP, sync, `sdd-apply` / Pi timeout logic
- Risk tier / lens selection, refutation batch logic, gate publication gates
- Schema migration of old lineages beyond additive `cumulativeLines` field (legacy receipts remain valid with 0 cumulative)

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `review-budget-ledger`: cumulative correction budget ledger — `ValidateCorrectionActual(actual,cumulative,budget)` is now enforced with real `cumulativeLines` from receipt/chain, `FixDeltaHash` persisted beyond `EmptyFixDeltaHash`, and `deriveNextTransition` returns `budget_remaining = budget - cumulative` — delta to `openspec/specs/review/spec.md` (budget/correction section)

## Approach

1. **Persist cumulative state** — Extend `PersistedReceipt` to carry `FixDeltaHash` (real delta hash) and `CumulativeCorrectionLines` (or equivalent `correction_lines_consumed` field). On `Finalize`/`ReMaterialize`, compute delta hash from correction artifacts when present; otherwise `EmptyFixDeltaHash`. Ensure `computeHash()` / `Validate()` bind the new field and keep legacy receipts (missing field ⇒ 0) valid.
2. **Wire cumulative validation** — At correction creation/completion, call `ValidateCorrectionActual(actualLines, cumulativeLines, budget)` with the persisted cumulative, then increment and persist the new cumulative in the receipt/event chain. Fail closed (escalate) on `cumulative+actual > budget` and when `FixRounds >= MaxFixRounds`.
3. **Fix next_transition** — In `deriveNextTransition` (next_transition.go:102), replace the no-op `remaining = budget.CorrectionLines` with `remaining = max(0, budget.CorrectionLines - cumulativeLines)` derived via `receiptArtifactOf` → `readReceiptFile` cumulative or by summing correction `LinesChanged` from chain. Keep budget-nil guard (`remaining=0`).
4. **finalize/verify/reconcile continuity** — `finalizeIdempotent`, `deriveExpectedReceipt`, `RetryFinalVerification`, and `mirrorPayloads` must read/write the cumulative field and `FixDeltaHash` consistently so re-materialized receipts stay content-addressed and hash-identical.
5. **Tests** — Add `TestNextTransition_CorrectionBudgetDeduction` (budget 10, cumulative 4 → remaining 6; cumulative 10 → 0; over-budget → escalate), `TestPersistedReceipt_CumulativeHashBinding`, and regression for `FixDeltaHash != EmptyFixDeltaHash` after correction.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/review/next_transition.go` | Modified | Deduct `cumulativeLines` from frozen budget in `deriveNextTransition` correction branch |
| `internal/review/finalize.go` | Modified | Persist `FixDeltaHash` + cumulative lines in `PersistedReceipt`; wire `ValidateCorrectionActual` cumulatively; keep legacy compat |
| `internal/review/verify_retry.go` | Modified | Persist/surface cumulative through `deriveExpectedReceipt` / `reMaterializeReceipt` |
| `internal/review/reconcile.go` | Modified | Mirror `cumulativeLines` / `FixDeltaHash` via `receiptMirror` / `gateContextMirror` if needed |
| `internal/review/correction.go` | Modified | Align `FixRounds` / budget counters with cumulative lines where applicable |
| `internal/review/*_test.go` | Modified | Cumulative budget + `FixDeltaHash` regression tests |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Hash binding break breaks old receipts | Medium | New field participates in `computeHash`, but decode defaults missing → 0; keep legacy validation path; add `receipt Validate` compat test |
| Double-counting cumulative across resumes | Medium | Derive cumulative only from terminal receipt + post-finalize correction events; single source of truth; test resume flow |
| Budget off-by-one (ceil vs floor) | Low | Reuse `DeriveCorrectionBudget` / `ValidateCorrectionActual` verbatim; test `ceil(5/2)=3` and cap 200 |
| Expands scope to gate/refutation | Low | Explicit Out of Scope; review gate keeps current `EmptyFixDeltaHash` check as `fixDeltaDelivered` predicate |

## Rollback Plan

`git revert <sha>` — additive receipt field only (defaults to 0 / `EmptyFixDeltaHash`); no schema migration, no `bigmem` change. Previous `next_transition` recomputes full budget (conservative). Manual: no data cleanup needed; lineage receipts stay valid.

## Dependencies

- `biggz-ai` review store: `ValidatedChain`, `frozenBudgetOf`, `receiptArtifactOf`, `readReceiptFile`, `ValidateCorrectionActual`, `CorrectionBudgetCap=200`, `MaxFixRounds=3`
- Tests: `go test ./internal/review -run TestNextTransition -count=1`, `go test ./... -count=1 -timeout 180s`, `go vet ./...`

## Success Criteria

- [ ] `deriveNextTransition` after 1 correction of 2 lines on budget 3 reports `budget_remaining = 1` (not 3); after exhausting budget reports 0 and forces `escalate` on next over-budget correction
- [ ] `PersistedReceipt.FixDeltaHash` is real delta hash after correction (not `EmptyFixDeltaHash`); `cumulativeLines` persisted and survives `Finalize` idempotent + `RetryFinalVerification` re-materialize (content-address hash matches)
- [ ] `ValidateCorrectionActual(actual=2,cumulative=2,budget=3)` rejects with budget error; `cumulative+actual <= budget` passes
- [ ] `MaxFixRounds` enforcement consistent with cumulative lines (3 rounds cap)
- [ ] `go test ./internal/review -count=1` + `go vet ./...` green; change <800 lines single PR
