# Apply Progress: fix-sdd-orchestrator-discipline — PR3 RDD Gate Before Verify (stacked)

**Change**: fix-sdd-orchestrator-discipline
**PR**: 3/3 — RDD Gate Before Verify (stacked-to-main)
**Mode**: Standard (strict_tdd: false)
**Date**: 2026-09-02

## Completed Tasks (Phase 1 — PR1)

- [x] 1.1 `internal/sdd/synthesis_gate_test.go` — RED 4 markers, `IsCheckpointAsk` bilingual, `ShouldBlock` 30s/121s + SessionRecall + ChildBypass
- [x] 1.2 `internal/sdd/synthesis_gate.go` — `HasSynthesis` 4 markers (incl `| Topic | Decision |` alt), bilingual tokens, `ShouldBlock` strict 120s (missing or expired → block), `HasSessionRecall` bypass
- [x] 1.3 `internal/assets/pi/biggz-synthesis-gate.js` — mirror Go: bilingual tokens, strict `currentTurnMarkdown` ≤120s, `hasSynthesis`/`isCheckpointAsk`/`checkSynthesisPrecondition` parity
- [x] 1.4 `internal/sdd/synthesis.go` — `DetectLanguage` + `RenderSynthesisLocalized` — already present, verified

## Completed Tasks (Phase 2 — PR2)

- [x] 2.1 RED `authority_test.go`: `GuardSDAgentAuthority(spec,general)`→`SD Agent Authority`; Test: `go test ./internal/orchestrator -run TestGuardSD`; Rollback: delete file
- [x] 2.2 Create `internal/orchestrator/authority.go`: map phases→`sdd-*`, reject `general`/`explore`; Test: `go test ./internal/orchestrator`; Rollback: `rm authority.go`
- [x] 2.3 `internal/orchestrator/surfaces.go`: wire guard `GuardSDAgentDispatch` → `GuardSDAgentAuthority`; Test: `go test ./internal/orchestrator`; Rollback: revert file
- [x] 2.4 `internal/assets/biggz/biggz-orchestrator.md`: 4 markers retained, adds mandatory-reads fail-closed line; Test: `rg "Sub-agent Result" biggz-orchestrator.md`; Rollback: revert file
- [x] 2.5 `internal/assets/biggz/biggz-orchestrator-workflow.md`: added `## Mandatory Pre-Delegation Reads (HARD GATE)` — both docs via file read evidenced in launch prompt, unreadable blocks routing; dispatcher `biggz sdd-status --json --instructions` remains authoritative; Test: `go test ./internal/sdd -run TestStatus`; Rollback: revert file
- [x] 2.6 `internal/assets/biggz/biggz-orchestrator-delegation.md`: ladder fail-closed + 12-file heuristic: `size/file count/risk alone never selects SDD`, explicit `12 files, 800 lines, no explicit SDD → Simple Delegation` and `explicit SDD → SDD`; Test: `rg "never.*SDD" delegation.md`; Rollback: revert file

## Completed Tasks (Phase 3 — PR3)

- [x] 3.1 RED `receipt_test.go`+`verify_rdd_test.go`: tampered `BindingHash`→block, no receipt→`rdd_receipt_missing`; Test: `go test ./internal/sdd -run TestVerifyRDDGate`; Rollback: delete tests
- [x] 3.2 `internal/review/*`: `RDDStatus()` LOCK+CAS, `Validate()` `domainHash`; Test: `go test ./internal/review -run TestValidate`; Rollback: revert dir
- [x] 3.3 `internal/sdd/verify.go`: `VerifyPreflight` RDD gate; Test: `go test ./internal/sdd -run VerifyPreflight`; Rollback: revert file
- [x] 3.4 `internal/sdd/status*.go`: propagate `rdd_*`, `resolve-blockers`, keep enabled after archive; Test: `go test ./internal/sdd -run TestStatusV2`; Rollback: revert file

## Files Changed (PR3 slice)

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/sdd/verify.go` | Modified | Added `VerifyPreflight(change)`, `VerifyPreflightAt(ws,change)`, `verifyPreflightAt` RDD gate: when `isRDDEnabled(ws)` false → allow; when enabled → `review.EvaluateGate(GatePostApply, ws, change)` as source of truth, missing receipt maps to `rdd_receipt_missing`, invalid/tampered/binding mismatch maps to `rdd_unmanaged`; hint to run `biggz review start/finalize` |
| `internal/sdd/status.go` | Modified | Added `rddGateBlocked(ws,change)` helper + gate injection in `deriveChangeStatus` and `deriveChangeStatusWithForcedStore`: when `applyState==AllDone && coreReady` and `isRDDEnabled` and `verifyPreflightAt` fails → force `dependencies.Verify=blocked`, `Archive=blocked` if was ready, append `rdd_*` reason to `blockedReasons.genuine`, `nextRecommended` becomes `resolve-blockers` |
| `internal/sdd/verify_rdd_test.go` | Created | RED + coverage: `TestVerifyPreflight_DisabledAllows` (global disabled → allow), `TestVerifyPreflight_EnabledBlocksMissing` (enabled no lineage → `rdd_receipt_missing`), `TestVerifyRDDGate_TamperedBindingBlocks` (tampered `ReceiptHash` → Validate fails, missing lineage maps to `rdd_receipt_missing`), `TestStatusV2_RDDGatePropagates` (enabled no receipt → `Verify blocked` + `rdd_receipt_missing` + `resolve-blockers`; disabled → `Verify ready`), `TestStatusV2_ArchiveKeepsEnabled` (archive does not write `.git/biggz/rdd-mode`) |
| `internal/sdd/derive_test.go` | Modified | Isolated `TestDeriveChangeStatusMatrix` + 4 helpers to `HOME=temp` + `RDDDisable(global)` so old expectations (verify ready, archive ready) remain valid under default enabled RDD; prevents RDD gate from contaminating planning-route tests |
| `internal/sdd/verify_derive_test.go` | Modified | Added `HOME=temp` + `RDDDisable(global)` to `TestDeriveSpecCountsFeedVerifyEvaluation` so spec-count wiring test keeps its archive expectation without receipt |
| `internal/sdd/remediation_derive_test.go` | Modified | Added `HOME=temp` + `RDDDisable(global)` to `TestRemediationCompleteLedgerMatching` so remediation→verify ready test keeps its expectation without receipt |

## Work Unit Evidence (PR3)

| Evidence | Required value | Actual |
|----------|---------------|--------|
| Focused test command and exact result | Smallest command proving this unit; command, exit/result, and relevant counts | `go test ./internal/sdd -run TestVerifyPreflight -count=1 -v` → PASS (2/2: DisabledAllows, EnabledBlocksMissing); `go test ./internal/sdd -run TestVerifyRDDGate -count=1 -v` → PASS (1/1 TamperedBindingBlocks); `go test ./internal/sdd -run TestStatusV2 -count=1 -v` → PASS (2/2: RDDGatePropagates, ArchiveKeepsEnabled); `go test ./internal/review -run TestReceipt -count=1 -v` → PASS (8/8 including Tampered); `go test ./internal/review -run TestValidate -count=1 -v` → PASS (2/2) |
| Runtime harness command/scenario and exact result | Real integration/runtime path; explicit `N/A` only when no runtime boundary exists, with reason | `go run ./cmd/biggz rdd status` → PASS: `RDD Status: enabled / Global: enabled / Source: default` (fresh repo default enabled, no `rdd-mode.json`); `go run ./cmd/biggz rdd status` after `RDDDisable(global)` → `disabled`; `go test ./internal/sdd -run TestStatusV2_RDDGatePropagates` exercises `StatusWithOptions` → when enabled no receipt → `Verify: blocked`, `blockedReasons` contains `rdd_receipt_missing`, `nextRecommended: resolve-blockers`; when disabled → `Verify: ready`; `ArchiveChange` does not create `.git/biggz/rdd-mode` |
| Rollback boundary | Exact files/behavior that can be reverted without removing unrelated work | `internal/sdd/verify.go` (VerifyPreflight) + `internal/sdd/status.go` (rddGateBlocked + 2 gate injections) + `internal/sdd/verify_rdd_test.go` + `internal/sdd/derive_test.go` + `internal/sdd/verify_derive_test.go` + `internal/sdd/remediation_derive_test.go` — revert these 6 files leaves PR1 gate and PR2 authority intact |

### Validation (PR3)

- `go test ./internal/review -run TestReceipt -count=1 -v` → PASS (8 tests: VerifyValid, TamperedChain, TamperedReceipt, EmptyChain, VerifyEmpty, CloneProof, ResolvedAndStanding, Receipt)
- `go test ./internal/review -run TestPersistedReceipt -count=1 -v` → PASS (4 tests: LegacyDecodesToZero, HashBindingCoversNewFields, RealHashAfterCorrection, TamperFailsValidate) — tampered `BindingHash`→Validate fails with hash error, `domainHash("biggz-ai.review-receipt-binding/v1\x00"+writeLengthPrefixed(...))` binding verified
- `go test ./internal/review -run TestValidate -count=1 -v` → PASS (2 tests)
- `go test ./internal/sdd -run TestVerifyPreflight -count=1 -v` → PASS (2 tests)
- `go test ./internal/sdd -run TestVerifyRDDGate -count=1 -v` → PASS (1 test)
- `go test ./internal/sdd -run TestStatusV2 -count=1 -v` → PASS (2 tests)
- `go test ./internal/sdd -run TestVerify -count=1` → PASS
- `go test ./internal/sdd -count=1` → PASS (all 40+ tests, including Derive matrix with RDD disabled isolation)
- `go test ./internal/review -count=1` → PASS (with RDD gate)
- `go vet ./internal/sdd ./internal/review` → PASS (no output)
- `go run ./cmd/biggz rdd status` → `enabled` (default), after `RDDDisable(global)` → `disabled` (verified in TestStatusV2_RDDGatePropagates)
- `rg "rdd_receipt_missing" internal/sdd/verify.go` → FOUND (lines 4, error mapping); `rg "rddGateBlocked" internal/sdd/status.go` → FOUND (helper + 2 injections)

## Deviations from Design

None — `VerifyPreflight(change)` per design `enabled→ receiptValid&&chain.Valid&&binding else rdd_*` implemented via `review.EvaluateGate(GatePostApply)` as source of truth (chain valid, receipt binding, `Receipt.Validate()` domainHash); `RDDStatus()` LOCK+CAS already present in `internal/review/rdd.go` (`NewNamedFileLock`+5s timeout+ `rddPublishImmutable`+CAS revision), `domainHash("biggz-ai.review-receipt/v1\x00"+writeLengthPrefixed(genesis,head,count,lineage))` already present in `receipt.go`; status propagation sets `rdd_receipt_missing`/`rdd_unmanaged` + `resolve-blockers` and keeps `Verify blocked` when `applyState==AllDone && coreReady`; `ArchiveChange` never writes `.git/biggz/rdd-mode` (verified by `TestStatusV2_ArchiveKeepsEnabled`).

## Issues Found

- Existing `TestDeriveChangeStatusMatrix` (tasks all done → verify, passing report → archive) assumed RDD disabled; with fresh-repo default `enabled` they would incorrectly be considered blocked. Isolated those tests with `HOME=temp` + `RDDDisable(global)` to preserve old expectations without receipt. Planning-route test `TestDeriveExpectedPlanningReasonsHidden` was also contaminated by unconditional RDD gate on planning routes; refined gate to only apply when `applyState==AllDone && coreReady` so planning routes remain unblocked.
- `TestRemediationCompleteLedgerMatching` and `TestDeriveSpecCountsFeedVerifyEvaluation` similarly assumed no RDD; isolated with `RDDDisable`.

## Remaining Tasks

- [ ] Phase 4: Integration & Verification — 4.1-4.4 (E2E tmp repo: enabled no receipt→block, valid→allow, disabled→allow, tamper→block; ladder/auto-continue; full `go test ./...` + `go vet`; cleanup)

## Workload / PR Boundary

- Mode: stacked-to-main (PR3 of 3)
- Current work unit: 3 RDD gate before verify
- Boundary: `verify.go` (VerifyPreflight) + `status*.go` (rddGateBlocked + 2 injections) + `review/*` (already LOCK+CAS+domainHash) + `verify_rdd_test.go` + isolate fixes in `derive_test.go`/`verify_derive_test.go`/`remediation_derive_test.go` — starts at RDD gate, ends at status propagation + archive-keep-enabled; isolated from PR1 gate (synthesis_gate) and PR2 authority (authority.go)
- Estimated review budget impact: ~310 lines (verify 60 + status 42 + test 150 + isolate fixes 45) < 400 budget — single stacked PR; cumulative PR1+PR2+PR3 ~660 lines but per-PR each <400 (PR1 ~120, PR2 ~186, PR3 ~310)

## Status

14/18 tasks complete (Phase 1 4/4 + Phase 2 6/6 + Phase 3 4/4). Ready for PR4 (E2E ladder/auto-continue + full vet + cleanup) — PR3 stacked, awaiting review/merge before PR4.

