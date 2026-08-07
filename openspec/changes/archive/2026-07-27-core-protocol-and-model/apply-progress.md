# Apply Progress: core-protocol-and-model

## Batch Info

- **Batch**: PR #1 — Foundation
- **Mode**: Standard (strict_tdd: false)
- **Date**: 2026-07-27
- **Driver**: sdd-apply executor

---

- **Batch**: PR #2 — Plugin Interfaces
- **Mode**: Standard (strict_tdd: false)
- **Date**: 2026-07-27
- **Driver**: sdd-apply executor

---

- **Batch**: PR #3 — Pipeline + Orchestrator
- **Mode**: Standard (strict_tdd: false)
- **Date**: 2026-07-27
- **Driver**: sdd-apply executor

---

- **Batch**: PR #4 — CLI + Wiring
- **Mode**: Standard (strict_tdd: false)
- **Date**: 2026-07-27
- **Driver**: sdd-apply executor

## Completed Tasks

### Phase 1: Foundation
- [x] 1.1 Create `go.mod` — module `github.com/biggs-100/biggz-ai`, Go 1.22+
- [x] 1.2 Create `model/review.go` — `ReviewSubject`, `ReviewState` (UUIDv7 ID), `Evidence`, `PolicyVerdict`
- [x] 1.3 Create `model/fsm.go` — `Status` enum, transition map, pure `Transition()` returning error on invalid
- [x] 1.4 Create `model/hash.go` — `AppendEvidence()` with linked `PrevHash`, `MerkleRoot()` via SHA-256 of tail

### Phase 2: Plugin Interfaces
- [x] 2.1 Create `plugin/interfaces.go` — `LensPlugin` (Analyze, Policies) and `ProviderPlugin` (Execute, Capabilities)
- [x] 2.2 Create `policy/evaluator.go` — `Evaluator` interface with `Name()` and `Evaluate(ctx, ReviewState) PolicyVerdict`
- [x] 2.3 Create `registry/registry.go` — build-time `Registry` with `RegisterLens`, `RegisterProvider`, `GetLens`, `GetProvider`

### Phase 3: Pipeline + Orchestrator
- [x] 3.1 Create `pipeline/pipeline.go` — `Stage` interface + `Pipeline` struct with sequential `Execute` and reverse-order `Rollback` (includes failed stage rollback)
- [x] 3.2 Create `orchestrator/orchestrator.go` — `Orchestrator.Execute()` wrapping pipeline and FSM transitions

### Phase 4: CLI + Wiring
- [x] 4.1 Create `cmd/biggz/dummylens.go` — `DummyLens` returning static findings
- [x] 4.2 Create `cmd/biggz/mockprovider.go` — `MockProvider` returning canned responses
- [x] 4.3 Create `cmd/biggz/main.go` — CLI: stdin → parse JSON → execute → stdout JSON or stderr error + exit 1

### Phase 5: Tests
- [x] 5.1 Write `model/hash_test.go` — evidence chain append, PrevHash link integrity, tamper detection
- [x] 5.2 Write `model/fsm_test.go` — rapid property tests: valid sequence chains succeed, invalid transitions rejected
- [x] 5.3 Write `registry/registry_test.go` — register/get a lens, register/get a provider, duplicate registration handling
- [x] 5.4 Write `pipeline/pipeline_test.go` — all stages succeed no rollback, middle stage failure triggers reverse rollback on completed stages and the failed stage

## Files Changed

| File | Action | What Was Done |
|------|--------|---------------|
| `go.mod` | Created | Module `github.com/biggs-100/biggz-ai`, Go 1.22, deps google/uuid + rapid |
| `go.sum` | Created | Dependency checksums |
| `model/review.go` | Created | `ReviewStatus` enum, `ReviewSubject`, `ReviewState` (UUIDv7 via google/uuid), `Evidence`, `PolicyVerdict`, `Correction` types |
| `model/fsm.go` | Created | 5-state transition map, pure `Transition()` function, `AllowedTransitions()` helper |
| `model/hash.go` | Created | `AppendEvidence()` with linked PrevHash + SHA-256 self-hash, `MerkleRoot()` as SHA-256 of tail |
| `model/hash_test.go` | Created | 8 tests: empty chain, chain links, hash uniqueness, immutability, MerkleRoot empty/non-empty, tamper detection, tamper via rebuild |
| `model/fsm_test.go` | Created | 6 tests: 3 rapid property tests (arbitrary pairs, valid chains, invalid rejection), self-transition, table-driven valid/invalid |
| `plugin/interfaces.go` | Created | `LensPlugin`, `ProviderPlugin` interfaces with `LensResult`, `Finding`, `Policy`, `ProviderRequest`, `ProviderResponse`, `Usage` types |
| `policy/evaluator.go` | Created | `Evaluator` interface with `Name()` and `Evaluate(ctx, *model.ReviewState)` |
| `registry/registry.go` | Created | `Registry` struct with `sync.RWMutex`, `RegisterLens`, `RegisterProvider`, `GetLens`, `GetProvider` methods |
| `registry/registry_test.go` | Created | 7 tests: register/get lens, register/get provider, duplicate lens registration, duplicate provider registration, unknown lens returns nil, unknown provider returns nil, empty registry |
| `pipeline/pipeline.go` | Created | `Stage` interface (`Name`, `Execute`, `Rollback`), `Pipeline` struct with sequential `Execute` and reverse-ordered rollback of failed + completed stages |
| `orchestrator/orchestrator.go` | Created | `Orchestrator` struct with `Registry` + `Pipeline`, `Execute(ctx, subject)` creating state, transitioning FSM, running pipeline, returning state + error |
| `pipeline/pipeline_test.go` | Created | 6 tests: empty pipeline, all succeed, middle stage fails, rollback order, first stage fails, single stage success/failure |
| `cmd/biggz/dummylens.go` | Created | `DummyLens` implementing `plugin.LensPlugin` — static finding, subject validation, policies |
| `cmd/biggz/mockprovider.go` | Created | `MockProvider` implementing `plugin.ProviderPlugin` — canned response, capability check |
| `cmd/biggz/main.go` | Created | CLI entry point — stdin JSON → registry wiring → inline stages → orchestrator → stdout JSON/stderr+exit 1 |

## Work Unit Evidence (PR #1)

| Evidence | Value |
|----------|-------|
| Focused test command and exact result | `go test ./model/... -v -count=1` — PASS, 17 tests (3 rapid × 100 iterations each), 0 failures, 0.769s |
| Runtime harness command/scenario and exact result | N/A — pure model types with no runtime boundary (no I/O, no network, no CLI) |
| Rollback boundary | `model/` directory + `go.mod` + `go.sum` — revert any of these files without affecting other phases |

## Work Unit Evidence (PR #2)

| Evidence | Value |
|----------|-------|
| Focused test command and exact result | `go test ./plugin/... ./policy/... ./registry/... -v -count=1` — PASS, 7 tests, 0 failures, 0.438s |
| Runtime harness command/scenario and exact result | N/A — interfaces + registry with no runtime boundary (no I/O, no network, no CLI) |
| Rollback boundary | `plugin/` + `policy/` + `registry/` directories — revert any of these without affecting `model/` or future packages |

## Work Unit Evidence (PR #3)

| Evidence | Value |
|----------|-------|
| Focused test command and exact result | `go test ./pipeline/... ./orchestrator/... -v -count=1` — PASS, 6 pipeline tests, 0 failures, 0.419s; orchestrator compiles/vets clean (no test files) |
| Runtime harness command/scenario and exact result | N/A — pipeline + orchestrator unit tests with no I/O or network boundary |
| Rollback boundary | `pipeline/` + `orchestrator/` directories — revert any of these without affecting `model/`, `plugin/`, `registry/`, or `cmd/` packages |

## Work Unit Evidence (PR #4)

| Evidence | Value |
|----------|-------|
| Focused test command and exact result | `go build ./cmd/biggz/...` — PASS, exit 0; `go vet ./cmd/biggz/...` — PASS, exit 0 |
| Runtime harness command/scenario and exact result | `echo '{"repository":"test/repo","commit_sha":"abc123"}' \| go run ./cmd/biggz` — valid JSON to stdout (Status: completed, 3 evidence entries, non-empty MerkleRoot), exit 0 |
| Rollback boundary | `cmd/biggz/` directory — revert these files without affecting any library packages |

## Deviations from Design

- **Pipeline rolls back the failed stage**: The design reference code in `design.md` only rolls back `completed` (prior) stages. The spec explicitly requires the failed stage's Rollback to be called as well. Pipeline.Execute now calls `stage.Rollback()` on the failed stage before rolling back prior completed stages in reverse order.
- **Orchestrator uses pure Transition + explicit assignment**: The design shows `Transition(&state.Status, ...)` as a mutation function. The existing Phase 1 FSM implements `Transition(current, target)` as a pure validation function. The orchestrator calls `model.Transition(state.Status, target)` to validate, then assigns `state.Status = target` explicitly.
- Added `TestPipeline_EmptyStages`, `TestPipeline_FirstStageFails`, and `TestPipeline_SingleStage` beyond the 3 required in task 5.4 for robust edge-case coverage.

## Issues Found

None.

## Remaining Tasks

None — all phases are complete.

## Workload / PR Boundary

- Mode: force-chained / stacked-to-main
- Current work unit: PR #4 — CLI + Wiring (cmd/biggz/)
- Boundary: Starts from previous PR #3 boundary, ends at `cmd/biggz/` directory
- Estimated review budget impact: ~150 lines (low)

## Status

15/15 tasks complete (all phases). Ready for verify.
