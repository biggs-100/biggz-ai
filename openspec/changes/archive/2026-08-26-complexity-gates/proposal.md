# Proposal: Complexity Gates

## Intent

Reviews fail on nested/long functions; debt is invisible. CI (`ci.yml`: gofmt/vet/test/e2e) and `pr-check.yml` (400-line budget) lack function depth. `doctor` (7 checks) and `review` (4 lenses) don't surface complexity. Measure legacy for visibility, **block only new/modified code** (grandfather) from day 1. Users: core team + external contributors (pre-merge PR).

## Scope

### In Scope
- CI `complexity` job (blocking day 1): `gocyclo` >15 + `gocognit` >20, fixed thresholds, critical packages only (`internal/review`, `internal/sdd`, `internal/verification`), diff-aware
- Grandfather: old violations reported, never block
- `biggz doctor` `ComplexityCheck`: top offenders table
- R2 enrichment: `R2-CYCLO`/`R2-COGNIT` inferential, hunk-bounded
- Debt report: `verify-report.md` section + doctor
- `*_test.go` excluded from gate (info only)
- ~6 requirements (refined in spec)

### Out of Scope
- JS/TS, whole-repo Go
- Legacy blocking/refactor
- Configurable thresholds, warn-only period

## Capabilities

### New Capabilities
- `complexity-gates`: Fixed 15/20 enforcement with grandfather, CI block + debt report.

### Modified Capabilities
- `system-diagnostics`: New `ComplexityCheck` (read-only table).
- `review-lenses`: R2 with `R2-CYCLO`/`R2-COGNIT` (inferential).

## Approach

No DAG change:
- **CI**: After `format`; pinned `gocyclo`+`gocognit`; `git diff base...HEAD` filtered; map to changed funcs; fail on new violations; `CostQuick/ReadOnly`.
- **Doctor**: `internal/doctor/complexity.go` (`ID=complexity`), read-only, scoped to 3 packages, `WARNING` + `--json`.
- **Lens**: Extend `readability` via `DeriveRiskInput` hunks; emit `R2-*` with `ProofRef`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `.github/workflows/ci.yml` | Modified | `complexity` job 15/20 diff-aware |
| `internal/doctor/complexity.go` | New | `ComplexityCheck` table |
| `internal/review/lens/readability/*` | Modified | `R2-CYCLO`/`R2-COGNIT` |
| `openspec/specs/complexity-gates/spec.md` | New | Gate spec |
| `openspec/specs/system-diagnostics/spec.md` | Modified | Delta: ComplexityCheck |
| `openspec/specs/review-lenses/spec.md` | Modified | Delta: R2 enrichment |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| False positives block PR | Med | Grandfather + changed funcs only; R2 inferential |
| Tool version drift | Low | Pin in `go.mod` |
| Rename diff misses | Med | File warn fallback, no block |
| Doctor slow | Low | 3-package scope; timeout → WARNING |

## Rollback Plan

Revert `ci.yml` job (gate off). Remove `complexity.go` + registration; doctor → 7 checks. Revert R2; `go test ./...` passes. Stateless, no migration.

## Dependencies

- `gocyclo` + `gocognit` (pinned), `git diff`, `DeriveRiskInput`, `VerificationPlan` (`CostQuick/ReadOnly`)

## Success Criteria

- [ ] CI blocks new funcs >15/>20 in critical packages; legacy not blocking
- [ ] `biggz doctor` table (human + `--json`)
- [ ] `verify-report.md` debt section (counts + top 10)
- [ ] `*_test.go` never blocks
- [ ] `go test ./... -short` + `gofmt` + `vet` pass
- [ ] Grandfather on rename/no-diff (no spurious block)
