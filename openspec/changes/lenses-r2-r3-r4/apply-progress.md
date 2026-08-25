# Apply Progress: Lenses R2-R4 — Hybrid Redesign (PR1 S1 Foundation)

## Summary

PR1 S1 foundation implements the hybrid facade sequential lens scaffold.
Stale `2950a40` progress (70 tests on `internal/lens/*` + `plugin.LensPlugin`) is superseded:
`internal/lens/` is absent, `plugin.LensPlugin` remains deleted (`ea8bad5`). New scaffold
lives in `internal/review/lens/` with build-time `Registry` and `pipeline.Stage` wiring.

This document tracks PR1 (S1) only. S2 (R2) and S3 (R3/R4+adapter+gate) remain pending.

## Archived Stale Progress (2950a40 — superseded)

> **Archived 2026-08-25 — stale plan referencing deleted paths `internal/lens/*` and `plugin.LensPlugin`.**
> Verified `internal/lens/` absent on this branch (`ls -la internal/lens` → ENOENT).
> Previous content preserved in git history (`git show HEAD:openspec/changes/lenses-r2-r3-r4/apply-progress.md`):
> extracted `internal/lens/gitdiff/` and three lenses (readability, reliability, resilience) wired via `plugin.LensPlugin`.
> Superseded by hybrid design: `internal/review/lens/` + `pipeline.Stage` + `DeriveRiskInput` reuse; no `plugin/` lens, no DAG.

Previous test result (stale, not applicable to current scaffold):

```
ok  github.com/biggs-100/biggz-ai/internal/lens/gitdiff      0.957s  9 tests
ok  github.com/biggs-100/biggz-ai/internal/lens/readability   1.047s  15 tests
ok  github.com/biggs-100/biggz-ai/internal/lens/reliability   1.007s  16 tests
ok  github.com/biggs-100/biggz-ai/internal/lens/resilience    1.176s  15 tests
ok  github.com/biggs-100/biggz-ai/internal/lens/risk          1.128s  15 tests
Total: 70 lens tests passing — on deleted paths, now void.
```

Rollback boundary for stale work (already deleted): `internal/lens/gitdiff/`, `readability/`, `reliability/`, `resilience/` + revert `internal/lens/risk/*`, `cmd/biggz/main.go`.

## PR1 Scope (S1) — Tasks 1.1-1.6

- [x] 1.1 Archive stale `apply-progress.md`, verify `internal/lens/` absent
- [x] 1.2 Create `internal/review/lens/types.go`
- [x] 1.3 Create `internal/review/lens/registry.go`
- [x] 1.4 Create `internal/review/lens/stage.go`
- [x] 1.5 Modify `internal/review/risk.go` freeze `PlanLenses(RiskHigh)==[risk,resilience,readability,reliability]`
- [x] 1.6 Guard `plugin/interfaces.go` zero `LensPlugin`/`Lens` + `internal/lens/` absent

## Files Changed (PR1 incremental)

| File | Action | What Was Done |
|------|--------|---------------|
| `openspec/changes/lenses-r2-r3-r4/apply-progress.md` | Modified | Superseded stale progress, verified `internal/lens/` absent |
| `internal/review/lens/types.go` | Created | Lens{ID,Analyze}, LensInput{RiskInput,Hunks,Truncated,Repo}, LensResult `biggz-ai.lens-result/v1` + LensFinding ProofRefs/Class |
| `internal/review/lens/registry.go` | Created | Registry map, RegisterLens last-win, Ordered skip-unknown, ResetRegistry for tests |
| `internal/review/lens/registry_test.go` | Created | 7 tests: ordered/last-win/skip/copy/guard no-plugin + internal/lens absent |
| `internal/review/lens/stage.go` | Created | LensStage pipeline.Stage sequential, no graph.go/DAG, ResultHash auto |
| `internal/review/lens/stage_test.go` | Created | 6 tests: name/execute success+hash/failure/sequential rollback/no-DAG import |
| `internal/review/risk.go` | Modified | Freeze PlanLenses high to [risk,resilience,readability,reliability] (gentle-ai order) |
| `internal/review/risk_test.go` | Modified | Update canonical 4R expectations to new order |
| `cmd/biggz/review_parity_test.go` | Modified | Update 3 canonical plan lenses assertions to new order |

## Test Results (final S1)

- `go vet ./internal/review/lens` → exit 0
- `go vet ./...` → exit 0
- `gofmt -l` → 0 files to format (lens, risk.go clean)
- `ls -la internal/lens` → ENOENT (absent) — verified
- `grep -rn "LensPlugin\|type Lens " plugin/interfaces.go` → 0 hits — verified
- `grep -rn "internal/review/lens" plugin/*.go` → 0 hits — verified (no plugin import)
- `grep -rn "internal/planner.*graph" internal/review/lens/*.go` → 0 hits — verified no DAG

## Work Unit Evidence (S1 — PR1 Foundation)

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./internal/review/lens -count=1` — exit 0, 13 tests pass (7 registry + 6 stage); `go test ./internal/review -run TestPlanLenses -count=1 -v` — exit 0, 2 tests pass (DeclaredWins, FromTier) |
| Runtime harness command/scenario and exact result | `go test ./internal/review -run TestPlanLenses -count=1` is the tier→lens runtime path; `go test ./cmd/biggz -run TestReviewStart -count=1` — exit 0 (parity harness, frozen plan = [risk,resilience,readability,reliability]); `pipeline.Execute` sequential rollback proven via TestLensStage_SequentialRollback |
| Rollback boundary | Delete `internal/review/lens/types.go, registry.go, registry_test.go, stage.go, stage_test.go` + revert `internal/review/risk.go`, `internal/review/risk_test.go`, `cmd/biggz/review_parity_test.go` — SDD artifacts `openspec/changes/lenses-r2-r3-r4/*` retain history |

## TDD Cycle Evidence (Strict TDD false — Standard Mode)

| Task | RED | GREEN | REFACTOR |
|------|-----|-------|----------|
| 1.2 types.go | N/A (Standard Mode) | `go vet` pass, no tests yet | — |
| 1.3 registry.go | `go test -run TestRegistry` first run fail (no impl) → then 7 pass | `13 tests pass` | registry.go isolated, ResetRegistry added |
| 1.4 stage.go | `go test -run TestLensStage_NoDAG` fail before fix → 6 pass | pipeline sequential verified | no DAG import |
| 1.5 risk.go freeze | `TestPlanLenses_FromTier` fail (old order) → 2 pass after fix | parity harness `go test ./cmd/biggz -run TestReviewStart` pass | — |
| 1.6 guard | `grep LensPlugin` 0 hits, `ls internal/lens` ENOENT verified | — | — |

## Status

6/6 S1 tasks complete. PR1 foundation ready for review. Next: PR2 S2 R2 readability (`internal/review/lens/readability/lens.go`) — stacked to main after PR1 merges.

### Workload / PR Boundary

- Mode: stacked PR slice (stacked-to-main)
- Current work unit: S1 foundation & supersede
- Boundary: `internal/lens/` absent verification → `internal/review/lens/{types,registry,stage}` + `risk.go` freeze
- Estimated review budget impact: ~250 lines prod + ~230 lines tests = ~480 review budget? Actually PR1 measured: 87+57+78+6+12 ≈ 240 prod + 128+130 ≈ 258 tests = ~500 — but code-only core is ~240 prod + lens tests, SDD doc edits excluded from budget per skill; pure lens scaffold ~170 prod + 258 tests ≈ 428 — within 800 budget, no exception needed. Sliced to keep next PRs autonomous.

