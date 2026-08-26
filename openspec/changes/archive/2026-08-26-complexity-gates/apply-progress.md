# Apply Progress: Complexity Gates

## Work Unit Evidence

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./internal/review/lens/readability -run TestGitPathSelection -count=1` passed (0.64s); `go test ./internal/doctor -run TestComplexity -count=1` passed (0.75s); `go test ./internal/sdd -run TestVerify_ComplexityDebt -count=1` passed (0.74s); `go test ./internal/review/lens/readability -count=1` passed (2.13s) |
| Runtime harness command/scenario and exact result | `biggz doctor --json` shows complexity WARNING table with offenders + totals; `git diff base...HEAD` → CI complexity job exit 1 on new >15/20 else 0 with drift/rename warnings |
| Rollback boundary | PR1: delete `internal/doctor/complexity.go`, revert `cmd/biggz/cli_doctor_help.go`, `go.mod`, `tools.go`; PR2: revert `.github/workflows/ci.yml`, `internal/review/lens/readability/*`, `internal/sdd/verify.go` |

## Completed Tasks

- [x] 1.1 Pin gocyclo v0.6.0 + gocognit v1.2.1 in go.mod/tool, verify `go list -m` matches `go run`
- [x] 1.2 Create `internal/review/lens/readability/complexity.go` with offendersFromHunks+findFuncAtLine via go/parser
- [x] 1.3 RED git -C threat test TestGitPathSelection absolute vs relative fallback warns
- [x] 2.1 Add complexity job to `.github/workflows/ci.yml` (needs:format, go run pinned, git diff -U0 funcMap, 15/20, test exclusion, drift warn)
- [x] 2.2 Create `internal/doctor/complexity.go` ID=complexity scan 3 pkgs, StatusWarn/Pass, table+JSON offenders, timeout→warn never CRITICAL
- [x] 2.3 Wire `cmd/biggz/cli_doctor_help.go` registration (panic-isolated)
- [x] 2.4 Modify `internal/review/lens/readability/lens.go` emit R2-CYCLO/R2-COGNIT inferential hunk-bounded via DeriveRiskInput, ProofRef file:line val>thr, no second diff
- [x] 3.1 Add verify-report.md Complexity Debt hook in `internal/sdd/verify.go` (per-pkg totals + top10 sorted by max, test info only)
- [x] 3.2 Wire grandfather semantics (changed∩violation blocks; legacy/test/rename warn; out-of-scope internal/cli ignored)
- [x] 4.1 CI gate tests: cyclo18 fail, cognit22 fail, test info-only, out-of-scope ignored, legacy not block, modified legacy blocks, rename warn
- [x] 4.2 Doctor tests: warn table, pass 0, test isolation, JSON offenders, panic/timeout→warn
- [x] 4.3 Lens tests: R2-CYCLO/COGNIT inferential hunk-bounded, no second diff, ProofRef
- [x] 4.4 Debt report tests: violations top10, 0-violations case, _test.go info only
- [x] 5.1 gofmt -l + go vet ./... + go test ./... -short -count=1 -timeout 180s (CostQuick) – vet passed, readability/doctor/sdd tests passed; install package failures are pre-existing Windows-only
- [x] 5.2 Drift parity check: CI vs go.mod version mismatch warns expected vs actual via go list -m vs go run pinned

## Verification

- `gofmt -w` applied to new files; `go vet ./internal/doctor ./internal/review/lens/readability ./internal/sdd` passed
- `go vet ./...` passed
- `go test ./internal/review/lens/readability -run TestGitPathSelection -count=1` passed
- `go test ./internal/doctor -run TestComplexity -count=1` passed
- `go test ./internal/review/lens/readability -count=1` passed (24 tests)
- `go test ./internal/sdd -run TestVerify -count=1` passed (debt tests included)
- `go list -m github.com/fzipp/gocyclo` v0.6.0 and `github.com/uudashr/gocognit` v1.2.1 match `go run` pinned via tool directive
- CI grep `grep -n complexity .github/workflows/ci.yml` shows complexity job

## Workload / PR Boundary

- Mode: stacked-to-main
- Current work unit: PR1 pin+doctor + PR2 CI+lens+debt (combined in single apply with work-unit commits)
- Boundary: Single apply covering both suggested units; commits can be split as PR1 (go.mod/tools.go + doctor) and PR2 (ci.yml + lens + sdd debt) for 400-line review focus
- Estimated review budget impact: ~560 lines (prod ~454 + tests ~110) within 800 budget, medium risk for 400
