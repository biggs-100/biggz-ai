# Apply Progress: core-protocol-and-model

## Batch Info

- **Batch**: PR #1 — Foundation
- **Mode**: Standard (strict_tdd: false)
- **Date**: 2026-07-27
- **Driver**: sdd-apply executor

## Completed Tasks

### Phase 1: Foundation
- [x] 1.1 Create `go.mod` — module `github.com/biggz-ai/biggz`, Go 1.22+
- [x] 1.2 Create `model/review.go` — `ReviewSubject`, `ReviewState` (UUIDv7 ID), `Evidence`, `PolicyVerdict`
- [x] 1.3 Create `model/fsm.go` — `Status` enum, transition map, pure `Transition()` returning error on invalid
- [x] 1.4 Create `model/hash.go` — `AppendEvidence()` with linked `PrevHash`, `MerkleRoot()` via SHA-256 of tail

### Phase 5: Tests
- [x] 5.1 Write `model/hash_test.go` — evidence chain append, PrevHash link integrity, tamper detection
- [x] 5.2 Write `model/fsm_test.go` — rapid property tests: valid sequence chains succeed, invalid transitions rejected

## Files Changed

| File | Action | What Was Done |
|------|--------|---------------|
| `go.mod` | Created | Module `github.com/biggz-ai/biggz`, Go 1.22, deps google/uuid + rapid |
| `go.sum` | Created | Dependency checksums |
| `model/review.go` | Created | `ReviewStatus` enum, `ReviewSubject`, `ReviewState` (UUIDv7 via google/uuid), `Evidence`, `PolicyVerdict`, `Correction` types |
| `model/fsm.go` | Created | 5-state transition map, pure `Transition()` function, `AllowedTransitions()` helper |
| `model/hash.go` | Created | `AppendEvidence()` with linked PrevHash + SHA-256 self-hash, `MerkleRoot()` as SHA-256 of tail |
| `model/hash_test.go` | Created | 8 tests: empty chain, chain links, hash uniqueness, immutability, MerkleRoot empty/non-empty, tamper detection, tamper via rebuild |
| `model/fsm_test.go` | Created | 6 tests: 3 rapid property tests (arbitrary pairs, valid chains, invalid rejection), self-transition, table-driven valid/invalid |

## Work Unit Evidence

| Evidence | Value |
|----------|-------|
| Focused test command and exact result | `go test ./model/... -v -count=1` — PASS, 17 tests (3 rapid × 100 iterations each), 0 failures, 0.769s |
| Runtime harness command/scenario and exact result | N/A — pure model types with no runtime boundary (no I/O, no network, no CLI) |
| Rollback boundary | `model/` directory + `go.mod` + `go.sum` — revert any of these files without affecting other phases |

## Deviations from Design

- Added `Correction` struct in `model/review.go` — referenced by `ReviewState.Corrections` but undefined in the design. Minimal struct with ID, Field, OldValue, NewValue, Reason, CreatedAt.
- Added `AllowedTransitions()` helper in `model/fsm.go` — useful for the rapid test generator and explicitly used in `TestFSM_ValidSequenceChain` to generate valid paths through the state graph.
- Made `evidenceHash()` a package-level (unexported) function in `hash.go` so tests can recompute hashes for tamper detection.

## Issues Found

None.

## Remaining Tasks

- Phase 2: Plugin interfaces (tasks 2.1-2.3)
- Phase 3: Pipeline + orchestrator (tasks 3.1-3.2)
- Phase 4: CLI + wiring (tasks 4.1-4.3)
- Phase 5: Registry tests (5.3), pipeline tests (5.4)

## Workload / PR Boundary

- Mode: force-chained / stacked-to-main
- Current work unit: PR #1 — Foundation (model package + tests)
- Boundary: Starts from empty project, ends at `model/` package with FSM, evidence chain, and Merkle root fully tested
- Estimated review budget impact: ~380 lines (low)

## Status

6/10 tasks complete. Ready for next apply batch (PR #2 — Plugin Interfaces).
