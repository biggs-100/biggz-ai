# Design: testing-guidance — Contract-First Testing Filter + Lint Guards

## Technical Approach
Docs+analyzer+CI, zero `internal/*` logic. `docs/testing-guidance.md` codifies Good/Bad + bans; `tools/nosourcegrep` via `go vet`/`golangci-lint` + `rg` fallback; CI adds `lint:no-source-grep` + `go test -run TestRapid`. Covers 6 req/9 scen; `bench:guard` doc-only.

## Architecture Decisions

### Decision: Doc location — `docs/testing-guidance.md` vs `docs/architecture.md`

| Option | Tradeoff | Decision |
|---|---|---|
| Extend `docs/architecture.md` | Mixes harness overview with test policy | Rejected |
| New `docs/testing-guidance.md` | Focused, linkable, matches spec req; minor sprawl | **Chosen** |
| `TESTING.md` only | Overloads manual QA guide | Rejected |

**Rationale**: `architecture.md` (220L) is product overview; spec mandates separate doc. Follows `architecture.md`+`validation-guide.md` split.

### Decision: Linter — `golangci-lint` custom vs `rg`

| Option | Tradeoff | Decision |
|---|---|---|
| Pure `rg` | Fast, no AST, false positives, no local `go vet` | Fallback only |
| Custom `tools/nosourcegrep` via `golangci-lint` | AST-precise, `go vet` locally, `.golangci.yml` plugin; needs `tools.go` pin | **Chosen (primary)** |
| `internal/tools/lint` | Couples prod imports | Rejected |

**Rationale**: Follows `tools.go` `tool` pattern (`gocyclo`). Scoped to `*_test.go`, `testdata` allowlist; `rg` is fallback.

### Decision: Rapid invocation — `go test -run TestRapid` vs build tag

| Option | Tradeoff | Decision |
|---|---|---|
| `//go:build rapid` tag | Hidden by default, missed coverage | Rejected |
| `go test -run TestRapid ./... -count=1 -timeout 180s` | Existing convention (10 funcs in `internal/review`), explicit | **Chosen** |
| `-tags=rapid` | Non-standard here | Rejected |

**Rationale**: Uses existing `TestRapid_*` (`compact_state_rapid_test.go`). No tags; spec requires `-run`.

### Decision: Example pinning — inline vs anchored refs

| Option | Tradeoff | Decision |
|---|---|---|
| Inline snippets | Drifts when tests change | Rejected |
| Anchor `blobstore_test.go:TestBlob_ConcurrentSameBytes` + `branch_test.go:TestBranch*` with `// Good:` comments | Traceable, grep-able | **Chosen** |
| No pinning | Gap this change fixes | Rejected |

**Rationale**: Spec needs 3 examples: `TestBlob_ConcurrentSameBytes (-race)`, `TestBranch_Traversal`, Bad `TestExpectSrcContains`.

## Data Flow
```
*_test.go → go vet -vettool=tools/nosourcegrep (local, *_test.go only)
         → golangci-lint -E nosourcegrep (.golangci.yml)
         → CI: lint:no-source-grep (analyzer || rg fallback: os.ReadFile+Contains|expect\(src\))
         → go test -run TestRapid ./... -count=1 -timeout 180s
         → go test ./... + go vet + gofmt -l (existing gates)
         → docs/testing-guidance.md (error links)
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `docs/testing-guidance.md` | Create | Good/Bad table, bans (`mock.module` #12823, source-grep), `bench:guard`, 3 examples |
| `tools/nosourcegrep/analyzer.go` | Create | Analyzer `*ast.CallExpr`, `*_test.go`, `testdata` allowlist |
| `tools/nosourcegrep/main.go` | Create | `vet` entry |
| `.golangci.yml` | Create | `linters-settings.custom.nosourcegrep` |
| `tools.go` | Modify | Add `_ "github.com/biggs-100/biggz-ai/tools/nosourcegrep"` |
| `go.mod` | Modify | `go mod tidy` (no prod import) |
| `.github/workflows/ci.yml` | Modify | Job `lint-no-source-grep` + `go test -run TestRapid` |
| `internal/bigmem/blobstore_test.go` | Modify | Anchor `TestBlob_ConcurrentSameBytes` |
| `internal/bigmem/branch_test.go` | Modify | Anchor traversal tests |

## Interfaces / Contracts
```go
// tools/nosourcegrep/analyzer.go
var Analyzer = &analysis.Analyzer{
  Name: "nosourcegrep",
  Doc:  "bans source-grep in *_test.go",
  Run:  run, // reports os.ReadFile+Contains on *.go/*.md, expect(src).toContain
}
```
```yaml
# .golangci.yml
linters-settings:
  custom: { nosourcegrep: { path: ./tools/nosourcegrep } }
linters: { enable: [nosourcegrep] }
issues: { exclude-rules: [{ path: testdata/.*, linters: [nosourcegrep] }] }
```
CI: lint fails on `mock.module`/source-grep; passes on `TestBlob_ConcurrentSameBytes`. `TestRapid` without `-short`.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Analyzer flags/allow | `analysistest.Run` with `testdata/good.go`/`bad.go` |
| Integration | `go vet` ≈ `golangci-lint` ≈ `rg` | `go test ./tools/nosourcegrep` + `rg -n` parity |
| E2E (CI) | PR blocks source-grep; `TestRapid` green; `go test`+`vet`+`gofmt` clean | `act` or test branch |

## Threat Matrix
CI touches shell (`rg`, `go vet`, `go test`) per `references/threat-matrix.md`:

| Boundary | Applicable | Reason | Response | RED tests |
|----------|------------|--------|----------|-----------|
| Documentation-like paths | N/A | Markdown only, no exec | — | — |
| Git repository selection | N/A | Uses `actions/checkout@v4` cwd | — | — |
| Commit state | N/A | No `git commit` | — | — |
| Push state | N/A | No `git push` | — | — |
| PR commands | N/A | No `gh pr create` | — | — |

Project-specific (task-required):

| Vector | Applicable | Safe | Failure | RED test |
|--------|------------|------|---------|----------|
| Test flake (`TestBlob_ConcurrentSameBytes -race`, `TestRapid`) | **Applicable** | `-count=1 -race`, `t.TempDir` isolation | Shared HOME / missing `-race` → flake | `go test -run TestBlob_ConcurrentSameBytes -race -count=100`; `TestRapid_* -count=5` |

Only test-flake propagates to tasks.

## Migration / Rollout
No migration. `git revert` one commit. Rollout: doc → analyzer+`rg` → non-blocking → blocking.

## Open Questions
- [ ] Pin `golangci-lint` via `tool` directive like `gocyclo` or `go run` latest?
- [ ] `TestBranch_Traversal` maps to `TestBranchCreateChild`/`TestBranchListGetChain` — confirm in tasks.
