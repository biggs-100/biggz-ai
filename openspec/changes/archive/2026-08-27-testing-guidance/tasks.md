# Tasks: testing-guidance — Contract-First Testing Filter + Lint Guards

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 300–400 (prod+tests+docs) |
| 400-line budget risk | Low |
| 800-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | auto-chain |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Docs + linter + CI | PR 1 (single) | `go test ./tools/nosourcegrep -count=1` | `go vet -vettool=./tools/nosourcegrep ./...` + `rg -n "os\.ReadFile.*Contains|expect\(src\)" --glob '*_test.go'` | Delete `docs/testing-guidance.md`, `tools/nosourcegrep/`; revert `.golangci.yml`, `tools.go`, `go.mod`, `.github/workflows/ci.yml`; remove anchors in `internal/bigmem/*_test.go` |

Dependencies: doc → analyzer → config → CI; anchors after doc.

## Phase 1: Foundation

- [x] 1.1 Create `docs/testing-guidance.md` with Good/Bad table (Good: failure mode, transformation/branch, external contract, regression; Bad: static echo, passthrough, wording, duplicate), bans (`mock.module` `oven-sh/bun#12823`, source-grep `expect(src).toContain` / `os.ReadFile`+`Contains` on `*.go`/`*.md`), `bench:guard` note, and 3+ pinned examples (`TestBlob_ConcurrentSameBytes` with `-race`, `TestBranch_Traversal` via `modernc.org/sqlite`, Bad `TestExpectSrcContains`) referencing `rapid` FSM + `ledger_regression_test` — Done: file exists, contains Good/Bad table, bans with #12823 rationale, bench:guard, and 3 pinned examples with file anchors
- [x] 1.2 Create `tools/nosourcegrep/analyzer.go` stub — `var Analyzer = &analysis.Analyzer{Name: "nosourcegrep", Doc: "bans source-grep in *_test.go", Run: run}` with `testdata` allowlist and `*_test.go`-only scoping — Done: `go build ./tools/nosourcegrep/...` passes and `go vet -vettool` loads without panic

## Phase 2: Core

- [x] 2.1 Implement `tools/nosourcegrep/analyzer.go` `Analyzer.Run` — inspect `*ast.CallExpr` for `os.ReadFile`+`strings.Contains`/`bytes.Contains` on `*.go`/`*.md` source paths and `expect(src).toContain` pattern; scoped to `*_test.go`, skip `testdata/` allowlist; report `file:line` with banned pattern name — Done: `go test ./tools/nosourcegrep -run TestAnalyzer -count=1` flags bad and passes good (analysistest)
- [x] 2.2 Create `tools/nosourcegrep/main.go` — `package main` with `golang.org/x/tools/go/analysis/singlechecker.Main(Analyzer)` as `go vet` entry point — Done: `go vet -vettool=$(go env GOPATH)/pkg/tool/.../nosourcegrep ./...` executes and honors `*_test.go` scope
- [x] 2.3 Create `.golangci.yml` — `linters-settings.custom.nosourcegrep: {path: ./tools/nosourcegrep, description: "bans source-grep"}` + `linters.enable: [nosourcegrep]` + `issues.exclude-rules: [{path: testdata/.*, linters: [nosourcegrep]}]` — Done: `golangci-lint run ./...` flags source-grep in `*_test.go`, passes on `TestBlob_ConcurrentSameBytes`, ignores `testdata/`
- [x] 2.4 Modify `tools.go` — add blank import `_ "github.com/biggs-100/biggz-ai/tools/nosourcegrep"` (and `golang.org/x/tools/go/analysis` if needed), then `go mod tidy` — Done: `go list -m all` clean, `go vet ./...` unchanged, no prod import in `internal/*`

## Phase 3: Integration

- [x] 3.1 Modify `.github/workflows/ci.yml` — add job `lint-no-source-grep` (primary: `go vet -vettool=./tools/nosourcegrep ./...` or `golangci-lint run`, fallback: `rg -n "os\.ReadFile.*Contains|expect\(src\)" --glob '*_test.go'` failing on match) and step `go test -run TestRapid ./... -count=1 -timeout 180s` (`-race` optional); ensure existing `go test ./...` + `go vet` + `gofmt -l` gates remain — Done: `yamllint`/YAML valid, `act -j lint-no-source-grep --dryrun` or `rg` parity check matches analyzer on fixture, `go test -run TestRapid` step present without `-short`
- [x] 3.2 Anchor `internal/bigmem/blobstore_test.go` — add `// Good: failure mode + external contract — races PutBlob with -race vs TestExpectSrcContains (Bad)` comment above `TestBlob_ConcurrentSameBytes` (or at file header referencing the 3 examples) linking to `docs/testing-guidance.md` — Done: `rg -n "Good:" internal/bigmem/blobstore_test.go` hits, `docs/testing-guidance.md` reference stable
- [x] 3.3 Anchor `internal/bigmem/branch_test.go` — add `// Good: transformation/branch + external contract via modernc.org/sqlite` above traversal test (`TestBranchCreateChild`/`TestBranchListGetChain` as `TestBranch_Traversal` anchor) — Done: `rg -n "Good:" internal/bigmem/branch_test.go` hits, no logic change (`git diff --stat` shows comment-only)

## Phase 4: Verification

- [x] 4.1 Unit: `analysistest` good/bad — `tools/nosourcegrep/testdata/src/bad/bad.go` (`os.ReadFile("internal/foo.go")` + `strings.Contains`) and `good/good.go` (`TestBranch_Traversal` DB-query assertion), `analyzer_test.go` via `analysistest.Run(t, Analyzer, "bad", "good")` — Done: `go test ./tools/nosourcegrep -count=1` PASS (bad flagged, good passes); threat test-flake N/A here
- [x] 4.2 Integration: `go vet` ≈ `golangci-lint` ≈ `rg` parity — run `go vet -vettool=./tools/nosourcegrep ./...`, `golangci-lint run ./...`, and `rg -n "os\.ReadFile.*Contains|expect\(src\)" --glob '*_test.go'` on same fixture set — Done: all three agree (flag bad, pass `TestBlob_ConcurrentSameBytes`); logs captured for evidence
- [x] 4.3 E2E: CI and gates green — `go test -run TestRapid ./... -count=1 -timeout 180s` PASS (10 funcs in `internal/review`), `go test ./... -count=1 -timeout 180s` PASS, `go vet ./...` clean, `gofmt -l .` empty, diff only doc+linter+CI+anchors (zero `internal/*` logic) — Done: no `mock.module` in repo (`rg -n "mock\.module"` empty), bench guard doc states `go test ./bench` never proves driven execution

## Dependencies

- 1.1 (doc) → 1.2 (stub) → 2.1 (analyzer impl) → 2.2 (vet entry) → 2.3 (golangci config) → 2.4 (tools.go pin) → 3.1 (CI) → 3.2/3.3 (anchors) → 4.1 → 4.2 → 4.3
- `rapid` (`pgregory.net/rapid`) and `modernc.org/sqlite` already in `go.mod`; `golangci-lint` custom analyzer API is only new external contract
- Anchors (3.2, 3.3) depend on doc 1.1 for pinned example names; CI 3.1 depends on 2.1–2.3 being runnable

## Evidence

- `go test ./tools/nosourcegrep -count=1` — unit analysistest good/bad (4.1)
- `go vet -vettool=./tools/nosourcegrep ./...` — local vet on `*_test.go`, flags source-grep (4.2)
- `golangci-lint run ./...` — custom `nosourcegrep` vs `rg` parity (4.2)
- `rg -n "os\.ReadFile.*Contains|expect\(src\)" --glob '*_test.go'` — fallback harness, must agree with analyzer (4.2)
- `go test -run TestRapid ./... -count=1 -timeout 180s` — CI Rapid step green (4.3)
- `go test ./... -count=1 -timeout 180s` + `go vet ./...` + `gofmt -l .` — gates clean, zero logic change (4.3)
- `rg -n "Good:" internal/bigmem/blobstore_test.go internal/bigmem/branch_test.go` — anchors present (3.2, 3.3)
- `rg -n "mock\.module"` empty — ban enforced
