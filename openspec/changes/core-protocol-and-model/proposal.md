# Proposal: Core Protocol & Data Model

## Intent

Build biggz-ai's foundation: a single `ReviewState` with Merkle-rooted evidence chain, 5-state FSM with policy separation, LensPlugin/ProviderPlugin interfaces, and pipeline orchestrator. Solves 8 architectural problems from gentle-ai by building protocol-first from scratch.

## Scope

### In Scope
- Core model: `ReviewState`, `Evidence` chain, Merkle root, schema versioning
- FSM: 5 coarse states (Pending/InProgress/Completed/Archived/Failed) + `PolicyEvaluator` interface
- Plugin system: `LensPlugin` and `ProviderPlugin` interfaces + explicit build-time `Registry`
- Pipeline: sequential stages with reverse rollback
- Orchestrator: `Execute()` as the single entry point
- First slice: ~500–700 line Go binary (stdin → pipeline → JSON output)
- Dummy lens + mock provider for validation
- Property-based FSM tests (`flyingmutant/rapid`) + integration tests

### Out of Scope
- Real AI provider calls (mock only)
- Multiple lenses (single dummy lens proves the interface)
- Real policies (single hardcoded evaluator suffices)
- Persistence layer (in-memory only)
- CLI polish (basic stdin/stdout)
- Dynamic plugin loading (explicit wiring always)
- Sidecar/gRPC plugin model (future concern)

## Capabilities

### New Capabilities
- `core-review`: ReviewState lifecycle, FSM transitions, evidence chain with Merkle integrity, schema versioning, PolicyEvaluator interface
- `plugin-system`: LensPlugin and ProviderPlugin interfaces with explicit Registry (build-time wiring, no init/plugin.Open)
- `cli`: Entry point at `cmd/biggz` — reads ReviewSubject from stdin, runs pipeline, prints ReviewState as JSON

### Modified Capabilities

None — greenfield project, no existing specs.

## Approach

Build protocol-first: (1) define `ReviewState` + `Evidence` types in isolation, (2) implement 5-state FSM as pure functions, (3) define `LensPlugin`/`ProviderPlugin` interfaces + `Registry`, (4) build `Pipeline` with sequential stages and reverse rollback, (5) wire `Orchestrator.Execute()`, (6) create `cmd/biggz` with dummy lens and mock provider. Test with rapid property checks on FSM invariants and integration tests on full pipeline. Single Go module ~700 lines.

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Over-engineering evidence chain | Low | Start with simple ordered list; Merkle tree only if tamper evidence is needed |
| Plugin interface too generic | Low | Two concrete interfaces (lens, provider); unification deferred until patterns emerge |
| Pipeline rollback complexity | Medium | Start with side-effect-free stages; rollback is no-op for pure computations |
| First slice too small to validate | Medium | Success criteria are explicit: protocol works end-to-end, not production-ready |

## Rollback Plan

No production rollback needed — greenfield code, not yet deployed. If architecture proves wrong: (1) revert git, (2) open new change with corrected model. First slice is disposable by design.

## Dependencies

- Go 1.22+ (standard library only for production code)
- `github.com/flyingmutant/rapid` (test dependency)
- `github.com/google/uuid` (UUIDv7, test/production)

## Success Criteria

- [ ] `go run ./cmd/biggz` reads stdin, runs pipeline, outputs valid `ReviewState` JSON
- [ ] Merkle root changes when any evidence element is modified (property test)
- [ ] FSM rejects invalid transitions (e.g., Archived → InProgress) (property test)
- [ ] Pipeline rollback runs all prior stage rollbacks on stage failure (integration test)
- [ ] Registry accepts explicit lens and provider registration (unit test)
- [ ] All code fits ~700 lines across model, FSM, pipeline, registry, orchestrator, CLI
