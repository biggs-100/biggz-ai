# Design: fix-budget-accounting — Cumulative Correction Budget Ledger

## Technical Approach

Make `PersistedReceipt` the cumulative ledger: add `CumulativeCorrectionLines int` and real `FixDeltaHash sha256:<hex>` (bound by `computeHash`). Wire `ValidateCorrectionActual(actual,cumulative,budget)` at correction completion, deduct in `deriveNextTransition` as `max(0, budget - cumulative)`. Legacy receipts decode `0`/`EmptyFixDeltaHash` and stay valid. Mirrors and re-materialization carry fields hash-identically. Covers proposal steps 1-4 and spec `review-budget-ledger` (5 reqs, 12 scenarios).

## Architecture Decisions

| Decision | Options | Tradeoffs | Choice |
|----------|---------|-----------|--------|
| Cumulative source | A: sum `LinesChanged` from chain each call B: field `CumulativeCorrectionLines` on receipt | A: no schema change but O(n) scan, double-count on resume/idempotent. B: additive, single source, needs persist+hash | **B** — receipt field `cumulative_correction_lines`; `next_transition` reads terminal receipt + post-finalize events only |
| Hash binding | A: exclude from `computeHash` B: include in `ReceiptBindingDomain` preimage | A: old receipts unchanged, tamper blind. B: new hash covers fields, tamper fails `Validate()` | **B** — include both fields in `computeHash()`; new receipts bound, tamper rejected |
| Legacy compat | A: reject missing B: missing→0/Empty C: migrate rewrite | A breaks old lineages. C churn. B additive defaults, conservative | **B** — missing JSON→0, missing hash→`EmptyFixDeltaHash` normalized before `Validate()`; `publishNoReplace` unchanged |
| Delta hash compute | A: new `artifact.go` helper B: inline `finalize.go` | A reusable. B co-located with `finalizeData`/`buildReceipt` owning manifest/evidence | **B** — helper in `finalize.go`; `buildReceipt` sets real hash if correction else `EmptyFixDeltaHash` |

## Data Flow

```
start_review(frozen budget) → chain genesis
lens_results → finalize → buildReceipt → PersistedReceipt{FixDeltaHash, Cumulative} → receipts/<sha256>.json → complete_review ref
status → receiptArtifactOf → readReceiptFile → PersistedReceipt ┬→ deriveNextTransition: remaining=max(0,budget-cumulative)
                                                                ├→ gate/recomputeGateFindings (fixDeltaDelivered)
                                                                ├→ verify_retry deriveExpected/reMaterialize (hash-identical)
                                                                └→ reconcile mirrorPayloads
correction → ValidateCorrectionActual(actual,cumulative,budget) → escalate if exceed → next finalize increments cumulative
```

Nil budget→`remaining=0`; clamped ≥0. If receipt burned/unreadable, fallback `countCandidateCausalFindings` (safe).

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/review/finalize.go` | Modify | Add `CumulativeCorrectionLines` field; extend `computeHash()` preimage + `Validate()`; extend `finalizeData`/`buildReceipt` to set real `FixDeltaHash`+cumulative; compat normalize missing→0/Empty in `readReceiptFile` |
| `internal/review/next_transition.go` | Modify | Replace `remaining=budget.CorrectionLines` (L100-108) with `max(0,budget-cumulative)` via `receiptArtifactOf→readReceiptFile`; keep `nil budget→0` |
| `internal/review/verify_retry.go` | Modify | `deriveExpectedReceipt`/`reMaterializeReceipt` rebuild same fields; fail-closed on tamper (no overwrite), hash-identical assert |
| `internal/review/reconcile.go` | Modify | `mirrorPayloads`/`receiptMirror`/`gateContextMirror` surface new fields; stale detection includes them |
| `internal/review/correction.go` | Modify | Align `FixRounds`/`MaxFixRounds=3` line-budget exhaustion to escalate (no new counter) |
| `internal/review/*_test.go` | Modify | Add deduction, hash-binding, `FixDeltaHash!=Empty` regressions |

No new files; <800 lines, single PR `stacked-to-main`.

## Interfaces / Contracts

```go
type PersistedReceipt struct {
    FixDeltaHash              string `json:"fix_delta_hash"`              // sha256:<hex> or EmptyFixDeltaHash
    CumulativeCorrectionLines int    `json:"cumulative_correction_lines"` // >=0 total
    // ReceiptHash = domainHash(ReceiptBindingDomain, preimage incl. both)
}
func (r PersistedReceipt) computeHash() string // now includes new fields
func (r PersistedReceipt) Validate() error     // rejects negative/bad sha256/hash mismatch

func ValidateCorrectionActual(actual, cumulative, budget int) error // cumulative+actual>budget → "budget" error
func DeriveCorrectionBudget(lines int) (int, error)                // min(200,max(2,ceil(lines/2)))
const EmptyFixDeltaHash = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
type NextTransition struct { Action string; BudgetRemaining int `json:"budget_remaining,omitempty"` }
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | `ValidateCorrectionActual` pass/fail/negative/budget-naming; `computeHash` binds fields; legacy decode 0/Empty | `go test ./internal/review -run TestPersistedReceipt -count=1` |
| Integration | `deriveNextTransition` 10/4→6, 10/10→0, over→0, nil→0; `finalizeIdempotent` preserves; `reMaterialize` hash-identical | `finalizeFixtureRepo` + `Authority.Status` |
| E2E | `status --json` exposes `budget_remaining`+cumulative+hash; exhausted→escalate | `go test ./... -count=1 -timeout 180s` |

## Threat Matrix

`references/threat-matrix.md` generic rows: all **N/A** (no routing/shell/VCS/PR/executable boundary).

| Boundary | Applicability | Reason |
|----------|---------------|--------|
| Documentation-like paths | N/A | Receipt JSON under `receipts/` only |
| Git repo selection | N/A | Reuses `detectRDDDirs`/`git -C`, no new selector |
| Commit state | N/A | No index mutation |
| Push state | N/A | No push/refspec |
| PR commands | N/A | No `gh --head` |

Domain cases (→ tasks RED tests):

| Case | Safe | Failure | RED test |
|------|------|---------|----------|
| Chain/receipt tamper (mutate cumulative/hash) | `Validate()` fail, `readReceiptFile` error, fallback to `countCandidateCausalFindings`, verify_retry no overwrite | Never fabricates `remaining`/`gate` | `TestPersistedReceipt_TamperFailsValidate` (3→4, old hash rejected) |
| Budget exhaustion double-spend (cum+actual>budget) | `remaining` clamped 0, `ValidateCorrectionActual` → budget error→escalate | Never silent over-budget | `TestNextTransition_BudgetExhaustion` (budget 3 cum3→0, Validate(1,3,3) fails) |

## Migration / Rollout

No migration. Additive fields default compatibly; old receipts valid. Revert: `git revert` (conservative full budget). Gate: `go vet ./...` + `go test ./internal/review -count=1`.

## Open Questions

- [ ] `CumulativeCorrectionLines` counts `additions+deletions` (same as `DeriveCorrectionBudget`/`countNumstatLines`) — confirm.
- [ ] `FixDeltaHash` bytes: diff of correction vs `baseTree` or filemerge hash? Use `sha256(empty)`→`domainHash` pattern implied by `EmptyFixDeltaHash`.
