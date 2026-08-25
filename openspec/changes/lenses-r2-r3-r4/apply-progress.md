# Apply Progress: Lenses R2-R4 — Hybrid Redesign (PR1 S1 + PR2 R2)

## Summary

PR1 S1 foundation implements the hybrid facade sequential lens scaffold.
Stale `2950a40` progress (70 tests on `internal/lens/*` + `plugin.LensPlugin`) is superseded:
`internal/lens/` is absent, `plugin.LensPlugin` remains deleted (`ea8bad5`). New scaffold
lives in `internal/review/lens/` with build-time `Registry` and `pipeline.Stage` wiring.

PR2 S2 R2 adds the readability heuristic (`internal/review/lens/readability/lens.go`):
`go/parser` deterministic failure + `DiffSummary>400/>200` inferential, ProofRefs
`file:line` from hunks, 8MiB `Truncated` propagation, no `plugin/` or DAG.

This document tracks PR1 (S1) + PR2 (R2). S3 (R3/R4+adapter+gate) remains pending.

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

## PR2 Scope (S2 R2) — Tasks 2.1 + part of 2.5 + 4.2

- [x] 2.1 Create `internal/review/lens/readability/lens.go` R2 `go/parser` fail deterministic, `DiffSummary>400/>200` inferential, drop mixedCase, ProofRefs file:line, 8MiB Truncated, no plugin/graph
- [x] 2.5 (partial) Wire `cmd/biggz/cli_review.go` register readability lens in Registry, Ordered(PlanLenses) → pipeline.Stage reuse DeriveRiskInput (R3/R4/adapter pending PR3)
- [x] 4.2 R2 unit ≥15 parser + threshold — `go test ./internal/review/lens/readability -count=1` (22 top-level + 7 table subtests, all pass)

## Files Changed (PR2 incremental)

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/review/lens/readability/lens.go` | Created | R2 ReadabilityLens ID readability, Analyze: go/parser deterministic + DiffSummary>400/>200 inferential, ProofRefs file:line, Truncated propagate, no mixedCase, no plugin/graph |
| `internal/review/lens/readability/lens_test.go` | Created | 21 tests + 7 table subcases (≥15): parser deterministic/ProofRefs, threshold >400/>200, inferential default, mixedCase absence, Truncated, hunk-bound, Repo fallback, no-DAG guard |
| `cmd/biggz/cli_review.go` | Modified | Register readability lens in init(), document Ordered(PlanLenses)→pipeline.Stage reuse DeriveRiskInput; keep R3/R4 + adapter pending |
| `openspec/changes/lenses-r2-r3-r4/tasks.md` | Modified | Mark 2.1 [x], 4.2 [x], 2.5 partial note |
| `openspec/changes/lenses-r2-r3-r4/apply-progress.md` | Modified | Merge PR2 evidence, preserve PR1 |

## Test Results (final PR2)

- `go vet ./internal/review/lens/readability` → exit 0
- `go vet ./...` → exit 0
- `gofmt -l` → 0 files to format (lens/readability clean)
- `go test ./internal/review/lens/readability -count=1` → exit 0, 21 top-level tests pass + 7 table subcases (total 28) — coverage: parser deterministic (3), threshold >400 high (2), >200 medium (2), inferential default (2), ProofRefs file:line (2), hunk-bound Truncated+Repo fallback (2), mixedCase absence (1), no-DAG import (1), edge table 7
- `go test ./internal/review/lens -count=1` → exit 0, 13 tests pass (7 registry + 6 stage) — unchanged, plus readability package passes independently
- `go test ./internal/review/lens/... -count=1` → exit 0, both packages pass (readability 0.78s, lens 0.66s)
- `go test ./internal/review -run TestPlanLenses -count=1` → exit 0, 2 tests pass (order freeze retained)
- `grep -rn "internal/planner.*graph" internal/review/lens/readability/*.go` → 0 hits — no DAG
- `grep -rn "plugin" internal/review/lens/readability/lens.go` → 0 hits — no plugin import

## Work Unit Evidence (PR2 — R2 Readability)

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./internal/review/lens/readability -count=1 -v` — exit 0, 21 tests PASS + 7 subtests PASS; `go test ./internal/review/lens/readability -count=1` — exit 0, ok 0.78s; `go test ./internal/review/lens -count=1` — exit 0, 13 tests PASS |
| Runtime harness command/scenario and exact result | `go test ./internal/review -run TestPlanLenses -count=1 -v` — exit 0 (frozen 4R plan [risk,resilience,readability,reliability] via DeriveRiskInput reuse); `go test ./cmd/biggz -run TestReviewStart -count=1` — exit 0 (parity harness with registered readability lens); `go vet ./...` — exit 0; pipeline sequential stage via `TestLensStage_SequentialRollback` remains green. Temp-repo parser-fail scenario covered by `TestLens_HunkBound_ParserUsesHunkBytes` (hunk invalid → deterministic, Repo fallback valid). No per-lens diff: lens reuses DeriveRiskInput DiffSummary/Hunks only |
| Rollback boundary | Delete `internal/review/lens/readability/lens.go, lens_test.go` + revert `cmd/biggz/cli_review.go` (remove readability import + init RegisterLens). Tasks re-mark 2.1, 4.2 to [ ]; apply-progress.md retains PR1 history (git history preserves PR1). No migration, stateless |

## TDD Cycle Evidence (Strict TDD false — Standard Mode, PR2)

| Task | RED | GREEN | REFACTOR |
|------|-----|-------|----------|
| 2.1 readability lens | N/A (Standard Mode) — `go test ./internal/review/lens/readability -run TestLens_ParserFailure` first run fail (no impl) → 3 parser tests pass after lens.go | `go vet` pass, 21 tests green, hash not yet needed | split threshold vs parser findings, sorted keys for determinism |
| 2.5 wiring (partial) | `go vet ./cmd/biggz` fail before import fix → exit 0 after init() | `go test ./cmd/biggz -run TestReview` 14s pass | init documents Ordered→Stage reuse DeriveRiskInput |
| 4.2 R2 tests | Table-driven thresholds red → green (7 subcases) + hunk-bound red → green | 28 total pass | extracted ProofRef helper, extractParserLine regex |

## Status

6/6 S1 tasks complete (PR1). 3/3 PR2 R2 tasks complete (2.1, 2.5 partial readability, 4.2). Next: PR3 S3 R3+R4+adapter+gate (`internal/review/lens/reliability`, `resilience`, `external/adapter`, `gate.go`, `catalog.go`). Total progress: 9 tasks complete, 12 pending (2.2-2.4, 2.5 remainder, 3.1-3.3, 4.1, 4.3-4.7, 5.1).

### Workload / PR Boundary

- Mode: stacked PR slice (stacked-to-main)
- Current work unit: S2 R2 readability
- Boundary: PR1 foundation → `internal/review/lens/readability/lens.go` + tests + `cli_review.go` registration
- Estimated review budget impact: ~170 lines prod (lens.go 145 + cli_review wiring 14) + ~350 lines tests (lens_test.go 430 - boilerplate) = ~520 raw; pure authored prod+tests ≈ 520 but SDD doc edits excluded per skill; code-only ≈ 520 within 800 budget, no exception needed. PR2 is autonomous: delete readability/* + revert cli_review.go returns to PR1 baseline. Next PR3 will add reliability/resilience/adapter.

