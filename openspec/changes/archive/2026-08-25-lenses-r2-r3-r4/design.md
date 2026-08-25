# Design: Lenses R2-R4 — Hybrid Redesign

## Technical Approach

Hybrid facade, sequential, no DAG. `Lens{ID,Analyze}` in `internal/review/lens` (not `plugin/`), build-time `Registry map[string]Lens` at `cmd/biggz` init. Each lens = `pipeline.Stage` in frozen order `risk→resilience→readability→reliability`. Single `DeriveRiskInput`; R2 `go/parser` + thresholds, R3 missing `_test.go` + error tokens, R4 hunk-bounded 8MiB inferential. `ExternalLensAdapter` bridges `capture-result` JSON (`gentle-ai.lens-result/v1` prefix) stateless; rollback to R1 baseline.

## Architecture Decisions

| Decision | Option | Tradeoff | Choice |
|----------|--------|----------|--------|
| Facade | Native vs `plugin.LensPlugin` vs Hybrid | Native no ext.; Plugin reopens `ea8bad5` | **Hybrid Heuristic+Adapter** |
| Seam | `plugin/` vs `internal/review/lens` | `plugin/` leaks authority | **`internal/review/lens`** |
| Execution | DAG vs `pipeline.Stage` | DAG overkill, gentle-ai has no parallelism | **Sequential, no `graph.go`, reverse rollback** |
| Derivation | Per-lens diff vs reuse `DeriveRiskInput` | Duplicate diverges from tier/manifest | **Reuse single `--numstat -z`** |
| R4 bound | Full-file vs hunk 8MiB | Full-file unsound | **Hunk 8MiB truncate+flag** |
| Registry | Dynamic vs build-time map | Dynamic nondeterministic | **Build-time map, last-win, skip unknown** |

## Data Flow

```
DeriveRiskInput → RiskInput → PlanLenses[ risk,resilience,readability,reliability ]
LensInput{RiskInput+Hunks≤8MiB} → Registry.Ordered → pipeline.Stage×N → LensResult → Capture(LensResultHash) → Gate(warn/block)
```

No second diff, no migration.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/review/lens/types.go` | Create | `Lens` iface, `LensInput`, `LensResult` |
| `internal/review/lens/registry.go` | Create | `Registry`, `RegisterLens`, `Ordered` |
| `internal/review/lens/stage.go` | Create | `LensStage` `pipeline.Stage` adapter |
| `internal/review/lens/readability/lens.go` | Create | R2: parser deterministic, thresholds inferential |
| `internal/review/lens/reliability/lens.go` | Create | R3: missing `_test.go`, error token |
| `internal/review/lens/resilience/lens.go` | Create | R4: timeout/context/concurrency/cleanup 8MiB |
| `internal/review/lens/external/adapter.go` | Create | Adapter wrapping `capture-result` |
| `internal/review/risk.go` | Modify | Freeze `PlanLenses(RiskHigh)` order |
| `internal/review/gate.go` | Modify | Inferential warn / deterministic block |
| `internal/catalog/catalog.go` | Modify | Optional `ComponentEntry` |
| `cmd/biggz/cli_review.go` | Modify | Registry wiring, Stage order |
| `internal/lens/**` | Guard | Assert not exists |

## Interfaces / Contracts

```go
type LensInput struct { review.RiskInput; Hunks map[string][]byte; Truncated bool; Repo string }
type LensResult struct { LensID string; Findings []LensFinding; Evidence []string; ResultHash string; Truncated bool }
type Lens interface { ID() string; Analyze(context.Context, LensInput) (LensResult,error) }
func RegisterLens(l Lens); func Ordered([]string) []Lens
type ExternalLensAdapter struct{ LensID string; Payload []byte }
func (a *ExternalLensAdapter) Analyze(context.Context, LensInput) (LensResult,error) // preserves hash prefix
type LensStage struct{ Lens Lens; Input LensInput }
func (s *LensStage) Execute(context.Context,*model.ReviewState) error
// Hash: domainHash("biggz-ai.lens-result/v1", {lens,findings,evidence})
```

Slices: S1 types+registry, S2 R2, S3 R3+R4+adapter.

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | iface, registry skip/last-win, PlanLenses freeze, R2 parser/thresholds, R3 missing test/token, R4 patterns/cap/inferential | `go test ./internal/review/lens/...` ≥15/lens |
| Integration | single derivation, hunk cap, adapter hash, rollback, order freeze | Temp repo + `pipeline.Execute` |
| E2E | `review start`→`capture-result`→`gate --json`; stateless revert | CLI harness |

## Threat Matrix

No new routing/shell/VCS/PR boundary; reuses `DeriveRiskInput`. All rows **N/A**:

| Boundary | Applicability |
|----------|---------------|
| Documentation-like paths | N/A — no new classification |
| Git repo selection | N/A — no new `git -C` |
| Commit state | N/A — hunks from parent tree |
| Push state | N/A — no push logic |
| PR commands | N/A — no `gh pr` composition |

## Scenario Traceability (15 req, 35 scen)

| Req | Scenarios | Design |
|-----|-----------|--------|
| Lens Interface | satisfied, reuse DeriveRiskInput | `types.go`; embeds RiskInput |
| Registry | ordered lookup | `registry.go` |
| Order Freeze | canonical high, declared wins | `risk.go` freeze |
| R2 | parser failure (deterministic), threshold inferential | `readability/lens.go` |
| R3 | missing test, error token (no volume) | `reliability/lens.go` |
| R4 | hunk finding, cap 8MiB | `resilience/lens.go` |
| ExternalAdapter | bridged hash, missing payload error | `external/adapter.go` |
| Sequential Stage | rollback, no DAG | `stage.go` + pipeline |
| Evidence/Rollback | inferential default, stateless | defaults + no migration |
| LensPlugin Absence | stays absent, legacy path absent | not in `plugin/`, guard `internal/lens/` |
| Adapter Bridge | hash preserve, no DAG | adapter sequential |
| AgentAdapter | 6 guard scenarios | `plugin/interfaces.go` unchanged |
| Gate Candidate-causal | inferential no-block, deterministic block, scope blocks | `gate.go` class check |
| Pre-PR Gate | pass, deterministic block, invalid receipt | `gate.go` |
| Gate Reporting | structured output, no duplicate diff | `--json` lensFindings; single derivation |

## Migration / Rollout

No migration, stateless. Rollback: delete `internal/review/lens/*`, revert `risk.go`/`cli_review.go`/`gate.go`; `go test ./...` passes on R1. Supersede stale tasks.

## Open Questions

- [ ] R3 error-token regex scope — confirm before tasks.
- [ ] Catalog lens tier (`native` vs `community`) — deferred.
