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
- [ ] 1.5 Modify `internal/review/risk.go` freeze `PlanLenses(RiskHigh)==[risk,resilience,readability,reliability]`
- [ ] 1.6 Guard `plugin/interfaces.go` zero `LensPlugin`/`Lens` + `internal/lens/` absent

## Files Changed (PR1 incremental)

| File | Action | What Was Done |
|------|--------|---------------|
| `openspec/changes/lenses-r2-r3-r4/apply-progress.md` | Modified | Superseded stale progress, verified `internal/lens/` absent |
| `internal/review/lens/types.go` | Created | Lens{ID,Analyze}, LensInput{RiskInput,Hunks,Truncated,Repo}, LensResult `biggz-ai.lens-result/v1` + LensFinding ProofRefs/Class |
| `internal/review/lens/registry.go` | Created | Registry map, RegisterLens last-win, Ordered skip-unknown, ResetRegistry for tests |
| `internal/review/lens/registry_test.go` | Created | 7 tests: ordered/last-win/skip/copy/guard no-plugin + internal/lens absent |
| `internal/review/lens/stage.go` | Created | LensStage pipeline.Stage sequential, no graph.go/DAG, ResultHash auto |
| `internal/review/lens/stage_test.go` | Created | 6 tests: name/execute success+hash/failure/sequential rollback/no-DAG import |

## Test Results (incremental)

- `ls -la internal/lens` → ENOENT (absent) — verified
- `grep -r "LensPlugin\|type Lens" plugin/interfaces.go` → 0 hits — verified
- `go vet ./internal/review/lens` → exit 0 — types.go valid
- `go test ./internal/review/lens -count=1` → ? (no test files) — package compiles

## Work Unit Evidence (S1 — after 1.4)

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./internal/review/lens -run TestLensStage -count=1` — 6 tests pass; `go test ./internal/review/lens -count=1` — 13 tests pass |
| Runtime harness command/scenario and exact result | `go test ./internal/review/lens -count=1` — pass, Stage pipeline sequential |
| Rollback boundary | Delete `internal/review/lens/stage.go` + `stage_test.go` (types+registry stay) |

## Status

4/6 S1 tasks complete. In progress — next: risk.go freeze.
