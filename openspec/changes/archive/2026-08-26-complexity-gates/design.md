# Design: Complexity Gates

## Technical Approach

Three read-only layers, no DAG change, fixed thresholds 15 (cyclomatic via `gocyclo`) / 20 (cognitive via `gocognit`), scoped to `internal/{review,sdd,verification}`, grandfathered and `*_test.go`-excluded. CI blocks only new/modified violations (diff-aware); Doctor tabulates debt (WARNING/`--json`); R2 enriches findings inferentially per hunk. All `CostQuick`/`ReadOnly`.

## Architecture Decisions

| Decision | Options | Tradeoff | Choice |
|----------|---------|----------|--------|
| CI placement | parallel job vs step in `test` | parallel=isolated signal; step=shared env | `complexity` job `needs: format` like `test`/`e2e` |
| Diff semantics | full scan vs `git diff`→func map | full blocks legacy | diff-aware: `git diff -U0` → changed funcs ∩ violations; legacy as debt |
| Tool pinning | `@latest` vs `tool` directive vs `go.mod` | latest drifts | pin `go.mod`/`tools.go`, `go run @pinned`, drift→warn |

## Data Flow

```
git diff base...HEAD (-U0 + --numstat)
        │
        ├──→ funcMap (go/parser on hunks) ──→ {file: [funcRange]}
        │              ▲
        │   DeriveRiskInput reused (Lens, no second diff)
        │
  gocyclo -over 15 ─→ {file:line func cyclo}
  gocognit -over 20 ─→ {file:line func cognit}
        │
  filter: critical pkg? ¬_test.go? func ∈ changed?
        ├── CI: changed∩violation → exit 1; else exit 0 + warn (rename/no-map, test, legacy)
        ├── Doctor: WARNING table + offenders[] JSON
        └── Lens R2: inferential R2-CYCLO/R2-COGNIT, ProofRef file:line val>thr
  verify-report.md: per-pkg totals + top10 sorted by max(cyclo,cognit)
```

```mermaid
sequenceDiagram
  participant GH as PR
  participant CI as complexity job
  participant Git as git diff
  participant T as gocyclo/gocognit
  GH->>CI: base, HEAD
  CI->>Git: diff base...HEAD -U0
  Git-->>CI: hunks + funcMap
  CI->>T: gocyclo -over 15; gocognit -over 20 (go run pinned)
  T-->>CI: violations
  CI->>CI: intersect (changed ∧ critical ∧ ¬test)
  alt violation in changed
    CI-->>GH: fail 1 "Func 18 >15"
  else only legacy/test/rename
    CI-->>GH: pass 0 + warn + debt
  end
```

## File Changes

| File | Action | Description | Est. |
|------|--------|-------------|------|
| `.github/workflows/ci.yml` | Modify | `complexity` job (`needs: format`, setup-go, `go run` pinned tools, diff+funcmap, 15/20, test exclusion, drift warn) | ~60 |
| `internal/doctor/complexity.go` | Create | `ComplexityCheck(ID=complexity)`: scans 3 pkgs, `StatusWarn`/`Pass`, table + `--json` offenders, timeout→warn, panic-isolated | ~180 |
| `internal/doctor/runner.go` | Modify | Registration (no logic) | 2 |
| `cmd/biggz/cli_doctor_help.go` | Modify | Add to `doctorRun()` slice | 2 |
| `internal/review/lens/readability/lens.go` | Modify | Add R2-CYCLO/COGNIT via hunk-bounded check, inferential, ProofRefs | ~120 |
| `internal/review/lens/readability/complexity.go` | Create | `offendersFromHunks(LensInput)` + `findFuncAtLine` helpers | ~80 |
| `go.mod` + `tools.go` | Modify | Pin `fzipp/gocyclo`, `uudashr/gocognit` | ~10 |
| `openspec/specs/complexity-gates/spec.md` | New | Gate spec | — |
| `openspec/specs/system-diagnostics/spec.md` | Modify | Δ ComplexityCheck | — |
| `openspec/specs/review-lenses/spec.md` | Modify | Δ R2 enrichment | — |

## Interfaces / Contracts

```go
// doctor — implements Check (types.go)
const ComplexityCheckID CheckID = "complexity"
type ComplexityCheck struct{} // ID() CheckID; Run(ctx)*Result; Remedy()*Remedy
type Offender struct {
  Package, File, Function string; Line, Cyclomatic, Cognitive int
}
// Run: StatusWarn→WARNING table, StatusPass→INFO "0 violations"; JSON via offenders[]
// Error/timeout/panic → StatusWarn with reason (never CRITICAL)

// lens — readability enrichment
// Finding.ID: R2-CYCLO-001 / R2-COGNIT-001, LensID: readability
// Class: EvidenceInferential, ProofRefs: ["pkg/foo.go:42: FuncFoo 18 >15"], Severity: info
// Test files → informational class only

// VerificationPlan
VerificationObligation{ID:"complexity", Contract:"biggz.complexity/v1",
  Cost: CostQuick, ReadOnly: true, Mandatory: true}
```

Helpers: `funcMap(hunks)` via `parser.ParseFile`; violations from tool stdout.

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | CI: grandfather, test exclude, out-of-scope, rename warn, drift | Mock diff+tool output |
| Unit | Doctor: warn/pass, test isolation, json, panic/timeout→warn | `NewComplexityCheckWithCustom` |
| Unit | Lens: R2-CYCLO/COGNIT inferential, hunk-bounded, no 2nd diff | Synthetic `LensInput` |
| Integration | `verify-report.md` totals+top10 | Fixtures |
| E2E | `go test`/`gofmt`/`vet` pass | — |

## Threat Matrix

| Boundary | Applicability | Design response | Planned RED tests |
|----------|---------------|-----------------|-------------------|
| Documentation-like paths | N/A — gate scans only `.go` in 3 pkgs | Exclude at file filter; no tool on docs | — |
| Git repository selection (`git -C`, paths) | Applicable | CI at repo root; Doctor/Lens reuse `DeriveRiskInput` repo arg; `go run` anchored to `go.mod` | RED: absolute path; relative fallback warns |
| Commit state (staged, `commit -a`, empty index) | N/A — CI diffs `base tree...candidate tree`, not index | Empty set → warn, not block | — |
| Push state | N/A — no push automation | — | — |
| PR commands (`--head`, env prefix) | N/A — no PR commands; `pr-check.yml` untouched | — | — |

## Migration / Rollout

No migration. Rollback: revert `ci.yml` job, delete `complexity.go` + helpers, revert `lens.go`. Stateless; grandfather = zero legacy breakage day-1.

## Open Questions

- [ ] Exact pinned versions for `gocyclo`/`gocognit` (latest stable vs toolchain)?
- [ ] Doctor timeout (proposed 10s for 3 pkgs)?
