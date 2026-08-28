# Proposal: testing-guidance — Contract-First Testing Filter + Lint Guards

## Intent

biggz-ai has contract tests (rapid FSM, `ledger_regression_test`, `modernc.org/sqlite` no mocks) but no documented Good vs Bad filter or lint. Bad tests (static echo, passthrough, wording, duplicate) and antipatterns (`mock.module` leak oven-sh/bun#12823, source-grep `expect(src).toContain`) pass. Codify oh-my-pi filter + CI guards, zero business-logic change.

## Scope

### In Scope
- `docs/testing-guidance.md` — Good/Bad filter + bans + `bench:guard`:
  - **Good**: failure mode, transformation/branch, external contract, regression
  - **Bad**: static echo, passthrough, wording, duplicate
  - **Good ex**: `TestBlob_ConcurrentSameBytes` (`-race`), `TestBranch_Traversal`
  - **Bad ex**: `TestExpectSrcContains` (fragile source-grep)
  - Rules: never `mock.module` (global leak), never source-grep, `bench:guard` — `go test ./bench` never proves driven execution
  - Context: rapid FSM + `ledger_regression_test` + `modernc.org/sqlite` exist; gap = filter undocumented + no lint
- `golangci-lint` custom `no-source-grep` (bans `expect(src).toContain`, `os.ReadFile`+`Contains` on source)
- CI: `go test -run TestRapid`

### Out of Scope
- `internal/*` logic changes
- `hashline`, `tui`, `blobstore`, `branching`, `tool-interception`, `extension-api`
- Runtime/migrations

## Capabilities

### New Capabilities
- `testing-guidance`: Good/Bad filter, antipattern bans (mock.module, source-grep), bench guard, CI enforcement

### Modified Capabilities
- None — docs + CI only; no spec behavior change to existing capabilities

## Approach

Docs + linter + CI, no prod code. Write `docs/testing-guidance.md` from oh-my-pi filter. Add `golangci-lint` custom analyzer `no-source-grep` (`tools/nosourcegrep` or plugin) → `.golangci.yml` + `go vet` fallback. Extend `ci.yml` with lint + `go test -run TestRapid ./...`. Doc ~50 lines, linter <100.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `docs/testing-guidance.md` | New | Good/Bad filter, examples, bans, bench:guard |
| `tools/nosourcegrep/` or `internal/tools/lint/` | New | Custom analyzer for source-grep |
| `.golangci.yml` | New/Modified | Enable custom linter |
| `.github/workflows/ci.yml` | Modified | Add lint + `go test -run TestRapid` jobs |
| `go.mod` | Modified | Pin deps |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Linter false positives | Med | Allowlist testdata, scope to `*_test.go` |
| Analyzer breaks CI install | Low | Fallback `rg` guard |
| Doc drift | Low | Pin examples to `blobstore_test.go`, `branch_test.go` |

## Rollback Plan

`git revert` one commit: delete doc + linter dir, revert `.golangci.yml` + `ci.yml`, `go mod tidy`. No migration. <5 min.

## Dependencies

- `pgregory.net/rapid`, `modernc.org/sqlite` (existing)
- `golangci-lint` custom analyzer API

## Success Criteria

- [ ] `docs/testing-guidance.md` exists with Good/Bad table, 3+ biggz-ai examples, bans, bench:guard
- [ ] `golangci-lint` fails on `expect(src).toContain`-style source-grep; passes on `TestBlob_ConcurrentSameBytes`
- [ ] CI has `lint:no-source-grep` + `go test -run TestRapid` steps green
- [ ] `go test ./... -count=1 -timeout 180s` + `go vet` + `gofmt -l` clean
- [ ] Zero changes to `internal/*` logic
