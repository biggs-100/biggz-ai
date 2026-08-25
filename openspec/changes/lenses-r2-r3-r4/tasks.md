# Tasks: Lenses R2-R4 — Hybrid Redesign

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~950 (prod ~600 + tests ~350) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1→PR2→PR3 stacked |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | S1 foundation + freeze | PR1 | `go test ./internal/review/lens -run TestRegistry -count=1` | `go test ./internal/review -run TestPlanLenses -count=1` | delete `lens/types.go,registry.go,stage.go` + revert `risk.go` |
| 2 | S2 R2 readability | PR2 | `go test ./internal/review/lens/readability -count=1` | temp repo parser-fail → `go test ./...` | delete `lens/readability/*` + revert `cli_review.go` |
| 3 | S3 R3+R4+adapter+gate | PR3 | `go test ./internal/review/lens/... -count=1` | `review start→capture-result→gate --json` | delete `reliability/*,resilience/*,external/*` + revert `gate.go,catalog.go` |

## Phase 1: Foundation & Supersede (S1)

- [x] 1.1 Archive stale `apply-progress.md` (`internal/lens/*` 2950a40), verify `internal/lens/` absent
- [x] 1.2 Create `internal/review/lens/types.go` `Lens{ID,Analyze}`, `LensInput{RiskInput,Hunks,Truncated}`, `LensResult` `biggz-ai.lens-result/v1` not in `plugin/`
- [x] 1.3 Create `internal/review/lens/registry.go` `Registry`, `RegisterLens`, `Ordered` last-win skip unknown at `cmd/biggz` init
- [x] 1.4 Create `internal/review/lens/stage.go` `LensStage` as `pipeline.Stage` no `graph.go`
- [x] 1.5 Modify `internal/review/risk.go` freeze `PlanLenses(RiskHigh)==[risk,resilience,readability,reliability]`
- [x] 1.6 Guard `plugin/interfaces.go` zero `LensPlugin`/`Lens` + `internal/lens/` absent

## Phase 2: Core Lenses & Adapter

- [x] 2.1 Create `internal/review/lens/readability/lens.go` R2 `go/parser` fail deterministic, `DiffSummary>400/>200` inferential
- [ ] 2.2 Create `internal/review/lens/reliability/lens.go` R3 missing `_test.go` + error token inferential, no volume
- [ ] 2.3 Create `internal/review/lens/resilience/lens.go` R4 hunk timeout/context/concurrency/cleanup 8MiB cap inferential only
- [ ] 2.4 Create `internal/review/lens/external/adapter.go` `ExternalLensAdapter` wraps `capture-result` JSON preserves hash
- [ ] 2.5 Wire `cmd/biggz/cli_review.go` register R2/R3/R4+adapter, `Ordered(PlanLenses)` → `pipeline.Stage` reuse `DeriveRiskInput` (PR2 partial: readability registered, R3/R4/adapter pending PR3)

## Phase 3: Integration

- [ ] 3.1 Modify `internal/review/gate.go` inferential warn exit0, deterministic blocks `pre-pr` exit1 `--json pass:false`, reuse `DeriveRiskInput`
- [ ] 3.2 Modify `internal/catalog/catalog.go` `ComponentEntry` lens tier, stateless
- [ ] 3.3 Wire `LensInput` hunks `≤8MiB` + `Truncated` from `DeriveRiskInput`, no per-lens diff

## Phase 4: Testing & Verification

- [ ] 4.1 Unit registry/types ordered/last-win/skip + no `plugin/` lens — `go test ./internal/review/lens -count=1`
- [x] 4.2 R2 unit ≥15 parser + threshold — `go test ./internal/review/lens/readability -count=1`
- [ ] 4.3 R3 unit ≥15 missing test + error token — `go test ./internal/review/lens/reliability -count=1`
- [ ] 4.4 R4 unit ≥15 hunk + 8MiB cap — `go test ./internal/review/lens/resilience -count=1`
- [ ] 4.5 Adapter unit hash + empty error — `go test ./internal/review/lens/external -count=1`
- [ ] 4.6 Integration temp-repo single derivation, hunk cap, rollback, order freeze, no DAG — `go test ./internal/review -run TestLens -count=1`
- [ ] 4.7 E2E `review start→capture-result→gate --json` + revert + `gofmt && go vet && go test ./... -count=1 -timeout 180s`

## Phase 5: Cleanup

- [ ] 5.1 `gofmt -w` `go vet ./...` remove remnants, verify `go test ./...` green
