# Tasks: Core Protocol & Data Model

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~700 |
| 400-line budget risk | Low |
| Chained PRs recommended | Yes |
| Suggested split | PR 1: Foundation → PR 2: Plugin System → PR 3: Pipeline → PR 4: CLI + Tests |
| Delivery strategy | force-chained |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Model types, FSM, evidence chain | PR 1 | `go test ./model/...` | N/A — pure model types | `model/` directory |
| 2 | Plugin interfaces, policy, registry | PR 2 | `go test ./registry/...` | N/A — interfaces + registry | `plugin/` + `policy/` + `registry/` |
| 3 | Pipeline + orchestrator + integration | PR 3 | `go test ./pipeline/... ./orchestrator/...` | N/A — pipeline unit tests | `pipeline/` + `orchestrator/` |
| 4 | CLI entry point + wiring | PR 4 | `go build ./cmd/biggz/...` | `echo '{}' \| go run ./cmd/biggz` | `cmd/biggz/` |

## Phase 1: Foundation

- [x] 1.1 Create `go.mod` — module `github.com/biggz-ai/biggz`, Go 1.22+
- [x] 1.2 Create `model/review.go` — `ReviewSubject`, `ReviewState` (UUIDv7 ID), `Evidence`, `PolicyVerdict`
- [x] 1.3 Create `model/fsm.go` — `Status` enum, transition map, pure `Transition()` returning error on invalid
- [x] 1.4 Create `model/hash.go` — `AppendEvidence()` with linked `PrevHash`, `MerkleRoot()` via SHA-256 of tail

## Phase 2: Plugin Interfaces

- [x] 2.1 Create `plugin/interfaces.go` — `LensPlugin` (Analyze, Policies) and `ProviderPlugin` (Execute, Capabilities)
- [x] 2.2 Create `policy/evaluator.go` — `Evaluator` interface with `Name()` and `Evaluate(ctx, ReviewState) PolicyVerdict`
- [x] 2.3 Create `registry/registry.go` — build-time `Registry` with `RegisterLens`, `RegisterProvider`, `GetLens`, `GetProvider`

## Phase 3: Pipeline + Orchestrator

- [x] 3.1 Create `pipeline/pipeline.go` — `Stage` interface + `Pipeline` struct with sequential `Execute` and reverse-order `Rollback`
- [x] 3.2 Create `orchestrator/orchestrator.go` — `Orchestrator.Execute()` wrapping pipeline and FSM transitions

## Phase 4: CLI + Wiring

- [x] 4.1 Create `cmd/biggz/dummylens.go` — `DummyLens` returning static findings
- [x] 4.2 Create `cmd/biggz/mockprovider.go` — `MockProvider` returning canned responses
- [x] 4.3 Create `cmd/biggz/main.go` — CLI: stdin → parse JSON → execute → stdout JSON or stderr error + exit 1

## Phase 5: Tests

- [x] 5.1 Write `model/hash_test.go` — evidence chain append, PrevHash link integrity, tamper detection (MerkleRoot changes after mutation)
- [x] 5.2 Write `model/fsm_test.go` — rapid property tests: valid sequence chains succeed, invalid transitions are rejected
- [x] 5.3 RED: Write `registry/registry_test.go` — register/get a lens, register/get a provider, duplicate registration handling
- [x] 5.4 Write `pipeline/pipeline_test.go` — all stages succeed no rollback, middle stage failure triggers reverse rollback on completed stages and the failed stage
