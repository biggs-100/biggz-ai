# Tasks: Complexity Gates

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~560 (prod ~454 + tests ~110) |
| 400-line budget risk | Medium |
| Chained PRs recommended | Yes (single PR allowed under 800) |
| Suggested split | PR1 pin+doctor → PR2 CI+lens+debt |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Pin + Doctor | PR1 → main | `go test ./internal/doctor -run TestComplexity -count=1` | `biggz doctor --json` shows WARNING table | delete `complexity.go`+revert `runner.go, cli_doctor_help.go, go.mod` |
| 2 | CI + Lens + debt | PR2 → main (on PR1) | `go test ./internal/review/lens/readability -count=1` | `git diff base...HEAD` → CI exit 1 on new >15/20 else 0 | revert `ci.yml, readability/*, verification/*` |

## Phase 1: Foundation

- [x] 1.1 Pin `gocyclo`+`gocognit` in `go.mod`/`tools.go`, verify `go list -m` matches `go run`. Cmd: `go mod tidy && go list -m github.com/fzipp/gocyclo github.com/uudashr/gocognit`
- [x] 1.2 Create `internal/review/lens/readability/complexity.go` with `offendersFromHunks`+`findFuncAtLine` via `go/parser`. Verify: `go vet ./internal/review/lens/readability`
- [x] 1.3 RED (git -C threat): failing test absolute vs relative fallback warns. File: `readability/complexity_test.go`. Cmd: `go test ./internal/review/lens/readability -run TestGitPathSelection -count=1`

## Phase 2: Core

- [x] 2.1 Modify `.github/workflows/ci.yml` add `complexity` job `needs: format` CostQuick/ReadOnly, `go run` pinned, `git diff -U0` funcMap, filter 3pkgs ¬`_test.go`, 15/20, drift warn. Verify: `grep -n complexity .github/workflows/ci.yml`
- [x] 2.2 Create `internal/doctor/complexity.go` `ID=complexity` scan 3 pkgs, StatusWarn/Pass, table+JSON offenders, timeout→warn never CRITICAL. Verify: `go vet ./internal/doctor`
- [x] 2.3 Wire `internal/doctor/runner.go` + `cmd/biggz/cli_doctor_help.go` registration (panic-isolated). Verify: `rg -n ComplexityCheck internal/doctor cmd/biggz`
- [x] 2.4 Modify `internal/review/lens/readability/lens.go` emit `R2-CYCLO`/`R2-COGNIT` inferential hunk-bounded via `DeriveRiskInput`, ProofRef `file:line val>thr`, no second diff. Verify: `go vet ./internal/review/lens/readability`

## Phase 3: Integration

- [x] 3.1 Add `verify-report.md` Complexity Debt hook (per-pkg totals + top10 sorted by max) in `internal/sdd/verify.go`. Verify: fixture debt section shows counts
- [x] 3.2 Wire grandfather semantics (changed∩violation blocks; legacy/test/rename warn; out-of-scope `internal/cli` ignored). Verify: `go test ./internal/review/lens/readability -run TestGrandfather -count=1`

## Phase 4: Testing

- [x] 4.1 CI gate RED→GREEN: cyclo18 fail, cognit22 fail, test file info-only, out-of-scope ignored, legacy not block, modified legacy blocks, rename warn. Cmd: `go test ./internal/review/lens/readability -count=1`
- [x] 4.2 Doctor tests: warn table, pass 0, test isolation, JSON offenders, panic/timeout→warn. File: `internal/doctor/complexity_test.go`. Cmd: `go test ./internal/doctor -count=1 -run Complexity`
- [x] 4.3 Lens tests: R2-CYCLO/COGNIT inferential hunk-bounded, no second diff, ProofRef. Cmd: `go test ./internal/review/lens/readability -count=1`
- [x] 4.4 Debt report integration: violations top10, 0-violations case, `_test.go` info only. Cmd: `go test ./internal/sdd -run Verify -count=1`

## Phase 5: Verification

- [x] 5.1 `gofmt -l .` + `go vet ./...` + `go test ./... -count=1 -timeout 180s` (CostQuick). Verify: 0 failures
- [x] 5.2 Drift parity check: CI vs go.mod version mismatch warns expected vs actual. Verify: `go run` pinned version output
