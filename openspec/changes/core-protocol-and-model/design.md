# Design: Core Protocol & Data Model

## Technical Approach

Single Go module with six flat packages at root level. Model types in `model/`, FSM as pure functions in `model/fsm.go`, plugin interfaces in `plugin/`, build-time registry in `registry/`, staged pipeline in `pipeline/`, orchestrator in `orchestrator/`. CLI entry at `cmd/biggz/main.go` with inline dummy lens and mock provider. ~700 lines total across all files.

Evidence chain: ordered `[]Evidence` where each entry links to the previous via `PrevHash`. `MerkleRoot` = `SHA-256(last.Hash)`. Simple, tamper-evident, and spec-compliant without a full Merkle tree.

FSM: lookup-table pure function — no library dependency, no interface, just a 5-state transition map in ~20 lines.

Pipeline: `Stage` interface with `Execute`/`Rollback`. Sequential execution with reverse-ordered rollback on any stage failure.

## Architecture Decisions

| Option | Tradeoffs | Decision |
|--------|-----------|----------|
| Flat root-level packages vs nested hierarchy | Flat: no import-cycle risk, simpler navigation. Nested: better long-term isolation. | **Flat** — 6 packages for ~700 lines. Refactor later if needed. |
| Ordered `[]Evidence` with linked hashes vs Merkle tree | Linked hashes: O(1) append, trivial verification. Tree: O(log n) per-entry verification, overkill for review chains. | **Ordered list with linked hashes** — spec requires `SHA-256(last.Hash)`; a tree adds zero value here. |
| FSM as pure function vs `looplab/fsm` library | Pure function: ~20 lines, zero deps. Library: 200+ lines of external dep for 5 states. | **Pure function** — `Transition(current, target Status) error` with a transition map. |
| PolicyEvaluator in fsm package vs separate | In fsm: unnecessary coupling. Separate: clean boundary for different evaluators. | **Separate** — `policy.Evaluator` referenced by pipeline, not FSM. |
| Dummy lens + mock provider in cmd/ vs separate package | Inline: simpler, no extra exports. Separate: reusable in tests. | **Inline in cmd/** — first slice proves the interface; extract to test helpers later. |
| UUIDv7 via `google/uuid` vs `crypto/rand` | UUIDv7: standard format, sortable by time. `crypto/rand`: one less dependency. | **`google/uuid`** — as specified in proposal for ReviewState ID. |

## Data Flow

```
stdin (ReviewSubject JSON)
  → cmd/biggz: json.Decode → Orchestrator.Execute(ctx, subject)
    → Pipeline.Execute(ctx, state)
      ├── LensStage:    lens.Analyze(ctx, subject)      → append Evidence
      ├── PolicyStage:  policy.Evaluate(ctx, state)      → append Evidence
      └── ProviderStage: provider.Execute(ctx, req)      → append Evidence
    → on any failure: rollback completed stages (reverse order)
    → Orchestrator returns *ReviewState, error
  → cmd/biggz: json.Encode → stdout (or error → stderr + exit 1)
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `go.mod` | Create | Module `github.com/biggz-ai/biggz`, Go 1.22+ |
| `model/review.go` | Create | `ReviewSubject`, `ReviewState` (with UUIDv7 ID), `Evidence`, `PolicyVerdict` types |
| `model/fsm.go` | Create | `Status` enum (Pending/InProgress/Completed/Archived/Failed), transition table, `Transition()` function |
| `model/hash.go` | Create | `AppendEvidence()` — appends with PrevHash, computes entry Hash; `MerkleRoot()` — SHA-256 of tail |
| `plugin/interfaces.go` | Create | `LensPlugin` and `ProviderPlugin` interfaces |
| `policy/evaluator.go` | Create | `Evaluator` interface with `Name()` and `Evaluate(ctx, ReviewState) PolicyVerdict` |
| `registry/registry.go` | Create | Build-time `Registry` with `RegisterLens`, `RegisterProvider`, `GetLens`, `GetProvider` |
| `pipeline/pipeline.go` | Create | `Stage` interface, `Pipeline` struct with sequential exec and reverse rollback |
| `orchestrator/orchestrator.go` | Create | `Orchestrator.Execute()` — single entry point, wraps pipeline + FSM transitions |
| `cmd/biggz/main.go` | Create | CLI: read stdin → parse JSON → execute → print JSON to stdout |
| `cmd/biggz/dummylens.go` | Create | Dummy `LensPlugin` — returns static findings |
| `cmd/biggz/mockprovider.go` | Create | Mock `ProviderPlugin` — returns canned responses |

Test files:

| File | Action | Description |
|------|--------|-------------|
| `model/hash_test.go` | Create | Evidence chain integrity: append, verify PrevHash, tamper detection |
| `model/fsm_test.go` | Create | Property-based FSM tests with `flyingmutant/rapid` — valid sequences, invalid rejection |
| `pipeline/pipeline_test.go` | Create | Pipeline execution + rollback order integration tests |
| `registry/registry_test.go` | Create | Register/Get, duplicate registration handling |

## Interfaces / Contracts

### Evidence chain (linked-hash integrity pattern)

```go
type Evidence struct {
    Position  int       `json:"position"`
    Timestamp time.Time `json:"timestamp"`
    Kind      string    `json:"kind"`
    Payload   string    `json:"payload"`
    PrevHash  string    `json:"prevHash"`
    Hash      string    `json:"hash"`
}

// Append creates a new chain with the entry linked to the last hash.
func AppendEvidence(chain []Evidence, kind, payload string) []Evidence

// MerkleRoot returns SHA-256 of the last entry's Hash, or "" for empty chain.
func MerkleRoot(chain []Evidence) string
```

### Pipeline rollback (non-obvious deferred cleanup)

```go
type Stage interface {
    Name() string
    Execute(ctx context.Context, state *model.ReviewState) error
    Rollback(ctx context.Context, state *model.ReviewState) error
}

// Execute runs stages sequentially. On any failure, rollbacks
// completed stages in reverse order. Returns the originating error.
func (p *Pipeline) Execute(ctx context.Context, state *model.ReviewState) error {
    var completed []Stage
    for _, stage := range p.stages {
        if err := stage.Execute(ctx, state); err != nil {
            for i := len(completed) - 1; i >= 0; i-- {
                completed[i].Rollback(ctx, state)
            }
            return fmt.Errorf("stage %s: %w", stage.Name(), err)
        }
        completed = append(completed, stage)
    }
    return nil
}
```

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | Evidence chain integrity | Append entries, verify `PrevHash` links, verify `MerkleRoot` changes on any tamper. |
| Property | FSM transitions | `rapid` state machine: generate random valid/invalid sequences, verify rejected transitions return error and valid ones update `Status`. |
| Integration | Pipeline rollback | Three stages where middle fails; verify rollback called on completed stages only, in reverse order. |
| Integration | CLI end-to-end | Pipe `ReviewSubject` JSON to `cmd/biggz`, verify stdout has valid `ReviewState` JSON with correct `Status`. |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary.

## Migration / Rollout

Greenfield project. No migration required. First slice is disposable by design — if the architecture proves wrong, revert git and open a new change.

## Open Questions

None.
