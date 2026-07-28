# Exploration: Core Protocol & Data Model

> **Change**: `core-protocol-and-model`
> **Phase**: Explore
> **State**: Complete — ready for proposal

## Context

This is a greenfield rebuild of `gentle-ai` (an AI-assisted code review tool). The old codebase had 8 critical architectural problems identified during analysis. This exploration designs the foundation for `biggz-ai`, a from-scratch Go implementation that avoids every one of those problems.

The directory is empty — no code exists yet. All findings are derived from architectural lessons, not codebase investigation.

---

## A. Core Data Model

### Single `ReviewState` (replacing Transaction + CompactState)

The fundamental insight: a review is an append-only chain of evidence. Everything else is derivable.

```go
type ReviewState struct {
    // Identity
    ID           uuid.UUID      `json:"id"`
    SchemaVersion int           `json:"schema_version"` // monotonically increasing

    // Lifecycle
    Status       ReviewStatus   `json:"status"`        // coarse FSM state
    CreatedAt    time.Time      `json:"created_at"`
    UpdatedAt    time.Time      `json:"updated_at"`

    // Subject — what is being reviewed
    Subject      ReviewSubject  `json:"subject"`

    // Evidence chain — ordered, append-only
    Evidence     []Evidence     `json:"evidence"`

    // Corrections — minimal record ("a correction happened")
    Corrections  []Correction   `json:"corrections"`

    // Integrity
    MerkleRoot   string         `json:"merkle_root"`   // single root hash

    // Policy results — externalized, not embedded
    PolicyResults []PolicyResult `json:"policy_results,omitempty"`
}
```

| Field | Essential | Derivable | Notes |
|-------|-----------|-----------|-------|
| `ID` | Yes | — | UUIDv7 for timestamp ordering |
| `SchemaVersion` | Yes | — | Enables migration without dual state machines |
| `Status` | Yes | — | Single source of truth for lifecycle |
| `Subject` | Yes | — | Input to the review process |
| `Evidence` | Yes | — | The chain of what happened |
| `Corrections` | Yes | — | Minimal record, no budget fields |
| `MerkleRoot` | Yes | — | Single integrity check |
| `PolicyResults` | No | Yes | Can be recomputed from Evidence + Policies |
| Individual hashes | No | Yes | Each derivable from Merkle chain |
| Counter fields | No | Yes | Length of Corrections[] etc. |
| Lens-specific data | No | Yes | Plugin responsibility, not core |

### Schema Versioning (not two parallel machines)

```go
type SchemaVersioned interface {
    UpgradeTo(ctx context.Context, targetVersion int) error
}

// Migration registry — NOT a second struct
var migrations = map[int]MigrationFunc{
    1: migrateV1ToV2,
    2: migrateV2ToV3,
}

type MigrationFunc func(ctx context.Context, data []byte) ([]byte, error)
```

**Rule**: There is exactly ONE `ReviewState` struct. When the schema evolves:
1. Add a new migration function to the registry
2. The `SchemaVersion` field records which version the data is in
3. On read, lazy-migrate to the latest version
4. The struct itself gains optional fields via `any` or a `Fields` map for forward compatibility

**What we avoid**: Two parallel state machines (Transaction + CompactState) with separate schemas, separate validation, and drift between them.

### Merkle Root (not hash-mania)

Instead of 8+ individual SHA-256 hashes cross-validated against each other:

```go
type Evidence struct {
    Position    int       `json:"position"`    // ordinal in chain
    Timestamp   time.Time `json:"timestamp"`
    Kind        string    `json:"kind"`        // "lens_result", "policy_eval", "correction", "human_input"
    Payload     any       `json:"payload"`     // structured data
    PrevHash    string    `json:"prev_hash"`   // SHA-256 of previous evidence
    Hash        string    `json:"hash"`        // SHA-256 of this entry
}
```

- The **Merkle root** is `SHA-256(chain)` where chain = last evidence entry's hash
- Tampering any entry changes the root
- Individual hashes (fix_delta_hash, policy_hash, etc.) are DERIVABLE — compute by filtering evidence by `Kind` and hashing `Payload`
- **Not hashed**: Policy configurations, plugin metadata, timestamps outside the chain, external system state

**Why this wins**:
- Single integrity check point instead of N cross-checked hashes
- Append-only = audit trail
- Evidence kind filtering replaces individual hash fields

---

## B. State Machine

### Coarse States (5, not 13-20)

```
   ┌──────────┐
   │  Pending  │
   └────┬─────┘
        │ start
   ┌────▼──────┐
   │ InProgress │◄────┐
   └────┬───────┘     │
        │ complete    │ correct
   ┌────▼─────┐       │
   │ Completed │───────┘
   └────┬─────┘
        │ archive
   ┌────▼──────┐
   │ Archived   │
   └────────────┘
```

| State | Description | Transitions Out |
|-------|-------------|-----------------|
| `Pending` | Created, not yet started | → `InProgress` |
| `InProgress` | Actively being reviewed | → `Completed`, → `Pending` (reset) |
| `Completed` | Review finished with result | → `InProgress` (if corrections needed), → `Archived` |
| `Archived` | Immutable historical record | none (terminal) |
| `Failed` | Irrecoverable error | → `Pending` (retry) |

**What we removed from the old 13-state model**:
- All "sub-review" states (ScopedReview, ScopedRejudge, JudgeReview, ScopedJudge, etc.) — these are POLICY concepts, not state machine concepts
- All "budget" states (FixApproved, FixRejected) — budget decisions are policy results
- All "transition" states (FixPhaseStarted, FixPhaseCompleted) — phase transitions are pipeline stages

### Policy Separation Boundary

```go
// PolicyEvaluator — external, pluggable, separate from state machine
type PolicyEvaluator interface {
    Name() string
    Evaluate(ctx context.Context, state ReviewState) (*PolicyVerdict, error)
}

type PolicyVerdict struct {
    Policy   string `json:"policy"`
    Passed   bool   `json:"passed"`
    Reason   string `json:"reason"`
    Severity string `json:"severity"` // "blocker", "warning", "info"
}
```

**The separation contract**:
- The state machine knows ONLY: valid transitions between coarse states
- The PolicyEvaluator knows: "FixRounds must be 1 or 2", "ScopedRejudgments must be < 2", budget limits
- When a policy fails, the evaluator returns a verdict — the orchestrator decides what to do (not the FSM)

```
State Machine:  "Can I go from InProgress to Completed?"  → YES
Policy Evaluator: "Wait — 3 corrections happened, max is 2" → BLOCKER
Orchestrator: "OK, stay InProgress, emit warning, ask user"
```

**Why this matters**: In the old code, policy checks were embedded in FSM transition functions. Adding/changing a business rule meant modifying the FSM. Now, policies are external, testable independently, and replaceable.

---

## C. Plugin System

### LensPlugin Interface

```go
type LensPlugin interface {
    // Metadata
    ID() string           // e.g., "risk"
    Name() string          // e.g., "Risk Assessment"
    Version() string       // semantic version

    // Execution
    Analyze(ctx context.Context, subject ReviewSubject) (*LensResult, error)

    // Lifecycle
    Policies() []Policy   // policies this lens contributes (may be empty)
}
```

**Why no `Supported`/`Enabled` method**: That's configuration, not plugin behavior. The registry decides what's enabled.

**Each lens is a struct implementing this interface**. No hardcoded arrays, no prefix maps (R1-R4), no counter fields in the core schema.

### ProviderPlugin Interface

```go
type Capability string

const (
    CapAnalyze   Capability = "analyze"
    CapGenerate  Capability = "generate"
    CapEmbed     Capability = "embed"
)

type ProviderPlugin interface {
    ID() string
    Name() string
    Capabilities() []Capability

    Execute(ctx context.Context, req ProviderRequest) (*ProviderResponse, error)
}

type ProviderRequest struct {
    Model    string         `json:"model"`
    Prompt   string         `json:"prompt"`
    Input    any            `json:"input,omitempty"`
    Params   map[string]any `json:"params,omitempty"`
}

type ProviderResponse struct {
    Content     string `json:"content"`
    ModelUsed   string `json:"model_used"`
    Usage       Usage  `json:"usage,omitempty"`
}
```

**No 17 in-tree adapters**: Providers implement this interface in their own packages. The core has zero provider adapter code. The interface is the contract — documented, stable, with a reference implementation.

### Registration (no dynamic loading)

```go
type Registry struct {
    lenses    map[string]LensPlugin
    providers map[string]ProviderPlugin
    policies  []Policy
}

func NewRegistry() *Registry { ... }

// Registration via explicit wiring — NOT init(), NOT plugin.Open()
func (r *Registry) RegisterLens(l LensPlugin) { r.lenses[l.ID()] = l }
func (r *Registry) RegisterProvider(p ProviderPlugin) { r.providers[p.ID()] = p }
```

**Approach**: Build-time composition (not runtime discovery):

```go
// cmd/biggz/main.go
registry := core.NewRegistry()
registry.RegisterLens(risk.NewLens())
registry.RegisterLens(resilience.NewLens())
registry.RegisterLens(readability.NewLens())
registry.RegisterProvider(openaiprovider.NewProvider(apiKey))
```

**Why not `plugin.Open()`**: 
- Cross-platform compatibility issues (Go plugin is Linux-only in practice)
- Version lock between main binary and plugin binary
- Debugging complexity
- Deployment simplicity wins

**Why not `init()`**:
- Implicit registration makes dependency tracking harder
- Testing requires controlling what's registered
- Explicit wiring is self-documenting

**When runtime plugins ARE needed later**: Add a sidecar process model (gRPC/JSON-RPC over localhost) — NOT dynamic loading.

---

## D. Orchestrator / Pipeline

### Architecture

The orchestrator is the HEART of the system. Everything else serves it.

```go
type Orchestrator struct {
    registry *Registry
    pipeline *Pipeline
}

func (o *Orchestrator) Execute(ctx context.Context, subject ReviewSubject) (*ReviewState, error)
```

### Pipeline Stages

```
Input ──► Stage 1: Ingest ──► Stage 2: Analyze ──► Stage 3: Evaluate ──► Stage 4: Report ──► Output
               │                    │                     │                    │
               ▼                    ▼                     ▼                    ▼
         Validate input      Run lenses in        Run policies on       Produce final
         normalize format    parallel, collect     evidence chain       review artifact
                             evidence
```

```go
type Stage interface {
    Name() string
    Execute(ctx context.Context, state *ReviewState) error
    Rollback(ctx context.Context, state *ReviewState) error
}

type Pipeline struct {
    stages []Stage
}

func (p *Pipeline) Run(ctx context.Context, state *ReviewState) error {
    for _, stage := range p.stages {
        if err := stage.Execute(ctx, state); err != nil {
            // Rollback in reverse order
            for i := len(p.stages) - 1; i >= 0; i-- {
                p.stages[i].Rollback(ctx, state)
            }
            return fmt.Errorf("stage %s failed: %w", stage.Name(), err)
        }
    }
    return nil
}
```

**Key design decisions**:
- State is mutated through the pipeline (in-repo pattern, not pass-by-value)
- Rollback undoes side effects (e.g., deleting temp files, reverting DB writes)
- Stages are sequential by default; parallelism is a stage's internal concern (e.g., Analyze runs lenses concurrently)
- The pipeline IS the orchestrator's execution path — no separate controller

### Execution Graph (future evolution)

The pipeline can evolve into a DAG when needed:

```go
// Future: dependency-based execution
type StageNode struct {
    Stage    Stage
    DependsOn []string // stage names this depends on
}
```

But start sequential. Optimize later.

---

## E. Testing Strategy

### Property-Based Tests (state machine invariants)

| Invariant | Tool | What it proves |
|-----------|------|----------------|
| "Valid states always reach a terminal state" | rapid/quickcheck | No deadlock states |
| "Merkle root changes when any evidence is modified" | rapid/quickcheck | Integrity check works |
| "Schema migration round-trip (n → n+1 → n) is identity modulo version" | rapid/quickcheck | Migrations are reversible |
| "Plugin isolation: panicking plugin doesn't crash orchestrator" | rapid/quickcheck | Fault boundary works |
| "Evidence chain is always append-only" | rapid/quickcheck | Immutability invariant |

### Integration Tests (full flow)

| Test | What it covers |
|------|----------------|
| `TestFullReviewFlow` | Ingest → Analyze → Evaluate → Report with 2 lenses |
| `TestPolicyRejection` | Policy blocks completion → state stays InProgress |
| `TestCorrectionCycle` | Review → correct → re-review → complete |
| `TestPluginRegistration` | Registry accepts/rejects plugins correctly |
| `TestPipelineRollback` | Stage 3 fails → rollback stages 2, 1 cleanly |

### What we DON'T do

- No golden file test overload (old problem #8)
- No per-lens golden outputs — test lens behavior at the plugin level, not at the integration level
- No golden test for every state transition — property tests cover that

---

## F. First Build Slice

### Minimum Viable Architecture Validator

This is the smallest thing that proves the protocol works:

```
cmd/biggz/main.go         → Wiring + CLI
internal/core/
  review_state.go          → ReviewState struct, Merkle chain
  status.go               → Coarse FSM (5 states)
  pipeline.go             → Pipeline with stages
  registry.go             → Plugin registry
  orchestrator.go         → Orchestrator.Execute()
  policy.go               → PolicyEvaluator interface
internal/plugins/
  lens/
    dummy/                → One reference lens (returns hardcoded result)
  provider/
    mock/                 → One mock provider (returns configurable response)
```

**What it proves**:
1. Protocol: `ReviewState` + Merkle chain works end-to-end
2. FSM: States transition correctly, policy separation is clean
3. Plugins: Interface is usable, registration works
4. Pipeline: Sequential execution + rollback works
5. Architecture: Wiring at main() is clean, no cycles

**What it does NOT do**:
- Real AI provider calls (mock is fine for validation)
- Multiple lenses (dummy lens proves the interface)
- Real policies (one hardcoded policy proves the evaluator interface)
- Persistence (in-memory only)
- CLI polish (basic stdin/stdout)

**Estimated effort**: ~500-700 lines of Go, 1-2 sessions
**Deliverable**: `go run ./cmd/biggz` reads a subject from stdin, runs the pipeline, prints the final ReviewState as JSON

---

## Options Considered

| Decision | Option A (chosen) | Option B (rejected) | Why |
|----------|-------------------|---------------------|-----|
| State machine complexity | 5 coarse states | 13+ detailed states (legacy) | Policy separation removes the need for state explosion |
| Hash model | Single Merkle root over evidence chain | N individual hashes (legacy) | Single root is verifiable, individual hashes are derivable |
| Plugin registration | Explicit wiring in main.go | `plugin.Open()` or `init()` | Explicit is testable, cross-platform, debuggable |
| Pipeline execution | Sequential stages with internal parallelism | Fully parallel DAG from start | Sequential is simpler, correct, and optimizable later |
| Testing approach | Property-based + integration | Golden file heavy (legacy) | Golden files make change expensive; properties prove invariants |
| Schema versioning | Version field + migration registry | Parallel structs (legacy) | One struct with version-aware migration is simpler to audit |
| Provider model | Interface contract + community packages | 17 in-tree adapters (legacy) | Zero provider code in core reduces maintenance burden |

---

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Over-engineering the evidence chain | Medium | Start with simple ordered list, only add Merkle tree if tamper evidence is needed |
| State machine too coarse | Low | States can be split later; splitting is safe, merging is hard |
| Plugin interface too generic | Low | Start with concrete lens/provider interfaces; they can be unified later if patterns emerge |
| Pipeline rollback is hard to implement correctly | Medium | Rollback is stage responsibility; start with stages that have no side effects (pure computation) |
| First slice is too small to be meaningful | Medium | The first slice proves protocol — it's intentionally minimal. Acceptance criteria are clear. |

---

## Ready for Proposal

Yes. The exploration is thorough enough to move to sdd-propose. The orchestrator should tell the user:

- The architecture is protocol-first: ReviewState → FSM → Plugins → Pipeline
- First slice validates the core protocol with ~500-700 lines of Go
- 8 architectural lessons from gentle-ai are directly addressed in the design
- sdd-propose will define scope, approach, and rollback plan for the first slice
