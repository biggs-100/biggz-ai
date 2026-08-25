# Exploration: lenses-r2-r3-r4 — Re-diseño híbrido (gentle-ai inspired)

## Current State

### gentle-ai (reference `C:/Users/USER/Desktop/herramientas/gentle-ai`)
- Lenses are NOT Go packages under `internal/lens`. They are **external reviewer slots** defined in `internal/reviewtransaction/transaction.go` as constants: `review-risk`, `review-resilience`, `review-readability`, `review-reliability` (`supportedLenses`, canonical order `risk → resilience → readability → reliability`). Each lens is a durable authority slot with counter fields and finding-ID prefixes (`R1-`, `R2-`, `R3-`, `R4-`).
- Risk classification in `internal/reviewtransaction/risk.go` (~1100 lines): `ClassifyRisk(RiskInput{Stats, Signals, OnlyPassiveContentChanges, TouchesConfiguration})` → `RiskHigh|Medium|Low`. Tiers: `hasHighSignal || touchesHotPath → high`, `OnlyPassiveContentChanges && !TouchesConfiguration → low`, else `medium`. `LargeChangeLines=400` is intentionally NOT a tier input. Content proof via `SnapshotBuilder.activePassiveContentPaths` (bounded 8MiB scan), `processBoundaryRiskReasons`.
- Execution is durable authority state machine (`Transaction` 13 states), not DAG. Old `internal/pipeline` + `internal/planner/graph.go` DAG was for component install ordering, never lens parallelism. Lens capture is `review capture-result --lineage --target --lens --order --expected-revision --input`.
- No `internal/lens/*`, no `LensPlugin` interface, no static heuristics for R2-R4. R2-R4 are LLM prompts executed by external reviewer CLIs via `reviewtransaction` contracts.

### biggz-ai current state
- `internal/review/risk.go` is simplified port (~400 lines vs 1100): same 0/1/4 tier semantics but path-token heuristics only. `PlanLenses(tier, declared) → []string{ "risk", "readability", "reliability", "resilience" }` but order differs from gentle-ai (`risk, readability, reliability, resilience` vs `risk, resilience, readability, reliability`).
- `internal/review/` owns full authority lifecycle (`review.go`, `capture.go`, `gate.go`, `snapshot.go`, `ledger.go`). `pipeline/pipeline.go` is simple sequential `Stage` runner — DAG already removed.
- `plugin/interfaces.go` only defines `AgentAdapter` (32 methods). `LensPlugin` deleted in `ea8bad5` and `873ae13`. Docs explicitly document removal of embedded static-analysis engine.
- `internal/planner/graph.go` remains only for component dependency ordering.
- History: `openspec/changes/archive/2026-07-27-real-lenses-r1-r4` implemented R1. `lenses-r2-r3-r4/tasks.md` + `apply-progress.md` implemented `internal/lens/gitdiff` + 3 lenses (70 tests, `2950a40`) then deleted by refactors — currently `internal/lens/` does not exist.

## Affected Areas
- `internal/review/risk.go` — tier classifier and `PlanLenses`; must reconcile lens order
- `internal/review/capture.go`, `ledger.go`, `store.go` — durable slot capture
- `internal/review/finding.go`, `verification.go` — severity/classification
- `internal/review/gate.go` — candidate-causal findings block gates
- `pipeline/pipeline.go` — sequential pipeline is execution substrate; DAG reintroduction confined here (not recommended)
- `plugin/interfaces.go` — decision point for reintroducing minimal `Lens` interface vs keeping AgentAdapter-only
- `internal/catalog/catalog.go` — optional lens catalog entries
- `cmd/biggz/cli_review.go` — flag parsing for --lenses
- `openspec/specs/plugin-system/spec.md` — currently declares LensPlugin removed; must update
- `openspec/changes/lenses-r2-r3-r4/*` — tasks.md/apply-progress.md stale
- `internal/review/snapshot.go` — frozen candidate derivation

## Approaches

### 1. Native-only heuristics inside `internal/review` (no new interface, no plugin)
Keep `internal/review/lens/` as helper functions called directly from `HeuristicStage`. No `Lens` interface, no registry, sequential stages.
- Pros: smallest surface, aligns with current doctrine, <150 lines per lens
- Cons: zero extensibility, hard to test in isolation
- Effort: Low (2-3 days, ~400 lines + 45 tests)

### 2. Reintroduce lightweight Lens interface + build-time registry (no DAG)
Define `type Lens interface { ID() string; Analyze(ctx, LensInput) (LensResult, error) }` in `internal/review/lens/types.go` (not `plugin/`). `internal/review/lens/registry.go` is `map[string]Lens` populated at `cmd/biggz` init. Execution is `pipeline.Stage` per lens in `PlanLenses` order, sequential.
- Pros: clean seam for testing, minimal surface, satisfies nativo half of hybrid
- Cons: still not externally extensible
- Effort: Medium (4-5 days, ~600 lines)

### 3. Hybrid facade (RECOMENDADO): Native heuristics + external-reviewer extensibility via Stage adapter
Combine (2) with `ExternalLensAdapter` delegating `Analyze` to `biggz review capture-result` JSON or to `AgentAdapter`-provided reviewer. Registry holds both kinds. No DAG. Catalog optionally advertises lenses as `ComponentEntry`.
- Pros: satisface requisito híbrido explícito, keeps core deterministic, allows future AgentAdapter to supply R2-R4 via LLM without changing gate/ledger, incremental: R2 heuristic, R3/R4 hybrid
- Cons: slightly more abstraction, need LensInput ↔ Snapshot mapping
- Effort: Medium (5-7 days, ~800 lines + 60 tests)

## Recommendation
**Go with Approach 3 (Hybrid facade, sequential, no DAG).**

Rationale: Respuestas `Re-diseñar` + `Híbrido` + `inspirate de gentle-ai` significan ni recrear (`internal/lens/gitdiff` 2950a40) ni archivar. gentle-ai prueba que R2-R4 no necesitan ser heurísticas estáticas — son slots tipados con reviewers externos — biggz-ai debe reflejar eso con fallback determinístico para CI local sin LLM. Reintroducir `plugin.LensPlugin` + DAG regresaría la eliminación deliberada (`ea8bad5`/`873ae13`).

**Heurísticas recomendadas:**
- **gitdiff helper — KEEP pero simplificar:** Reusar `DeriveRiskInput` con `git diff --numstat -z --no-renames` ya existente. Eliminar `ParseDiffStat` regex separado y `DetectModeChanges` (ya está en `risk.go`).
- **R2 Readability — SIMPLIFY/AMPLIFY:** Cambiar de `additions>500/200` a `DiffSummary[path] total changedLines>400/200`. Eliminar detector `mixedCase+underscores`. Ampliar con check `go/parser.ParseFile` falla → finding deterministic.
- **R3 Reliability — KEEP/AMPLIFY:** Mantener `missing _test.go` y `error-handling` path token. Eliminar `large change set` (tier ya cubre volumen). Deterministic solo cuando proof concreto.
- **R4 Resilience — SIMPLIFY:** Timeout/context/concurrency/cleanup pero acotado a diff hunks con cap 8MiB. Path-token + hunk grep, findings `inferential`.

**Slice order (stacked-to-main, 3 slices, no DAG):**
- Slice 1 (gitdiff + types): `internal/review/lens/types.go`, `registry.go`, reusar `DeriveRiskInput`
- Slice 2 (R2 readability): `internal/review/lens/readability/lens.go` + tests, wire como `pipeline.Stage`
- Slice 3 (R3+R4 + catalog/plug): `reliability` + `resilience` + `ExternalLensAdapter` + optional catalog entries

**File structure:**
```
internal/review/lens/
  types.go        — Lens interface { ID() string; Analyze(ctx, LensInput) (LensResult, error) }
  registry.go     — Registry map[string]Lens, RegisterLens, Ordered([]string) []Lens
  readability/lens.go  — HeuristicLens R2
  reliability/lens.go  — HeuristicLens R3
  resilience/lens.go   — HeuristicLens R4
  external/adapter.go  — ExternalLensAdapter wrapping captured payload
pipeline/pipeline.go   — stays sequential (no graph.go import)
```

## Risks
- Spec drift entre orden gentle-ai (`risk, resilience, readability, reliability`) y biggz-ai (`risk, readability, reliability, resilience`) — rompe binding `capture-result --order`
- Heuristic false positives blocking `gate pre-pr` — new heuristics default to `inferential` with concrete ProofRefs
- Ledger identity coupling: `LensResultHash` SHA-256 con prefix `gentle-ai.lens-result/v1` — renaming breaks chain
- Duplicate Git evidence derivation: reimplementar `git diff --stat` junto a `DeriveRiskInput` causa divergencia
- Rollback scope confusion: tasks.md viejo lista `internal/lens/*` pero nuevo diseño toca `internal/review/lens/*`
- Plugin extensibility scope creep: exponer `Lens` en `plugin/` reabre superficie cerrada intencionalmente

## Ready for Proposal
Yes — con una clarificación pre-proposal: confirmar orden congelado de lenses (alinear a gentle-ai `risk, resilience, readability, reliability` vs mantener actual). Todo lo demás confirmado: alcance=Re-diseñar, arquitectura=Híbrido (native + plugin via adapter, no DAG), prioridad=slot model de gentle-ai.

---
Explored: gentle-ai/internal/reviewtransaction, biggz-ai/internal/review, plugin/interfaces.go, planner, pipeline, catalog, archives.
Source: sdd-explore delegated worker 2026-08-25.
