# Proposal: Lenses R2-R4 — Hybrid Redesign

## Intent

Re-design R2-R4 as durable reviewer slots per gentle-ai `reviewtransaction/transaction.go` (`risk→resilience→readability→reliability`), not packages/DAG. `internal/lens/*` deleted, `plugin.LensPlugin` removed (`ea8bad5`). Stale `tasks.md`/`apply-progress.md` (70 tests on deleted paths) must be superseded. Align `PlanLenses` order, add heuristics with external extensibility.

## Scope

### In Scope
- Freeze order to `risk,resilience,readability,reliability` (update `PlanLenses`+tests)
- `internal/review/lens/types.go` (`Lens` iface), `registry.go` (`map[string]Lens`), `external/adapter.go`
- R2: `go/parser` fail + `changedLines>400/200` (drop mixedCase)
- R3: missing `_test.go` + error-handling token (drop volume)
- R4: hunk-bounded timeout/context/concurrency/cleanup, 8MiB cap, inferential
- Supersede stale `tasks.md`/`apply-progress.md`
- Per `exploration.md` Approach 3, sequential, no DAG

### Out of Scope
- Reintroduce `plugin.LensPlugin` or DAG
- LLM prompts (future adapter consumer)
- Duplicate `git diff --numstat -z` — reuse `DeriveRiskInput`
- Change tier volume semantics

## Capabilities

### New Capabilities
- `review-lenses`: Lens iface, Registry, sequential Stages, R2/R3/R4 + ExternalLensAdapter

### Modified Capabilities
- `plugin-system`: LensPlugin stays removed; adapter bridges capture-result
- `review-gates`: lens findings are candidate-causal gate inputs (inferential default)

## Approach

Hybrid facade, sequential, no DAG. `Lens{ID();Analyze}` in `internal/review/lens` (not `plugin/`). Build-time registry at `cmd/biggz` init. `HeuristicLens` + `ExternalLensAdapter` (capture-result JSON). Each lens = `pipeline.Stage` in `PlanLenses` order. Input from `DeriveRiskInput`. Slices: S1 types+registry, S2 R2, S3 R3+R4+adapter+catalog.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/review/risk.go` | Modified | Freeze order, fix `PlanLenses` |
| `internal/review/lens/...` | New | types, registry, R2/R3/R4, external |
| `pipeline/pipeline.go` | Modified | Sequential lens Stages |
| `internal/catalog/catalog.go` | Modified | Optional lens `ComponentEntry` |
| `openspec/specs/plugin-system` | Modified | Delta: LensPlugin removed |
| `tasks.md` / `apply-progress.md` | Superseded | Replace stale plan/report |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Order drift breaks `capture-result --order` | High | Freeze to gentle-ai order + assertion test |
| False positives block gate | Med | Inferential default, concrete ProofRefs only |
| `LensResultHash` coupling | Low | Keep `gentle-ai.lens-result/v1` stable |
| Duplicate git derivation | Med | Reuse `DeriveRiskInput` |
| Plugin scope creep | Low | `Lens` stays in `internal/review` |
| Stale tasks reused | High | Explicit supersede guard |

## Rollback Plan

Revert `internal/review/lens/*` + `risk.go` order commits; `go test ./...` passes on R1 baseline. Remove catalog entries. No ledger migration (stateless). Archive old `apply-progress.md`.

## Dependencies

- `DeriveRiskInput` + `parseNumstatPerPath` in `risk.go`
- `pipeline.Stage` contract
- `capture.go`/`ledger.go` (no schema change)

## Success Criteria

- [ ] `PlanLenses(RiskHigh)==[risk,resilience,readability,reliability]`
- [ ] `go test ./internal/review/lens/...` passes (≥15 each R2/R3/R4)
- [ ] R2-R4 as sequential `pipeline.Stage`, no `graph.go`
- [ ] `go test ./...` + `gofmt` pass
- [ ] Stale `tasks.md`/`apply-progress.md` superseded
- [ ] No `plugin.LensPlugin` or `internal/lens/*` reintroduced
