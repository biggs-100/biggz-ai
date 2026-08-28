# testing-guidance Specification

## Purpose

Contract-first testing filter for biggz-ai: codify Good vs Bad test criteria, ban `mock.module` and source-grep, enforce bench guard and CI lint. Zero `internal/*` logic change; docs + linter + CI only.

## Requirements

### Requirement: Test Filter — Good vs Bad Classification

The system MUST classify tests via the Good/Bad filter. Good tests MUST satisfy >=1 of: (a) name failure mode, (b) transformation/branch, (c) external contract, (d) regression. Bad tests MUST be rejected if they are static echo, passthrough, wording, or duplicate.

#### Scenario: Good test passes filter

- GIVEN `TestBlob_ConcurrentSameBytes` names race failure mode and races `PutBlob` with `-race`
- WHEN filter evaluates it
- THEN it MUST be classified Good (failure mode + external contract)

#### Scenario: Bad test is rejected

- GIVEN `TestExpectSrcContains` asserts `expect(src).toContain("foo")`
- WHEN filter evaluates it
- THEN it MUST be classified Bad (static echo + source-grep) and MUST be rejected

### Requirement: Ban — mock.module

The system MUST NOT use `mock.module` in any test. All mocking MUST use explicit interfaces or `t.TempDir`-scoped fakes. Rationale MUST cite `oven-sh/bun#12823` global leak.

#### Scenario: mock.module is rejected

- GIVEN a `*_test.go` imports or calls `mock.module`
- WHEN lint or review runs
- THEN it MUST fail and report the violating file and line

### Requirement: Ban — Source-Grep Assertions

The system MUST NOT assert on source text. The following MUST be banned: `expect(src).toContain`, `os.ReadFile`+`Contains` on source, or equivalent string search on `*.go`/`*.md` source.

#### Scenario: Source-grep is flagged

- GIVEN `*_test.go` does `os.ReadFile("internal/foo.go")` then `strings.Contains`
- WHEN `no-source-grep` lint runs
- THEN it MUST fail and identify the banned pattern

#### Scenario: Valid contract assertion passes

- GIVEN `TestBranch_Traversal` asserts DB state via `modernc.org/sqlite` query output
- WHEN lint runs
- THEN it MUST pass (no source-grep)

### Requirement: Bench Guard

`go test ./bench` MUST NOT be considered proof of driven execution. Bench/journey tests MUST be informational only; driven-mode proof MUST require explicit journey corpus or contract tests.

#### Scenario: Bench success does not prove driven

- GIVEN `go test ./bench -count=1` passes
- WHEN CI evaluates driven-mode proof
- THEN it MUST NOT mark driven execution verified

### Requirement: Guidance Documentation

`docs/testing-guidance.md` MUST exist and MUST contain: Good/Bad table, bans (`mock.module`, source-grep), `bench:guard`, and >=3 pinned biggz-ai examples including `TestBlob_ConcurrentSameBytes` (`-race`), `TestBranch_Traversal`, and `TestExpectSrcContains` (Bad). Examples MUST reference `rapid` FSM and `ledger_regression_test` context.

#### Scenario: Doc completeness

- GIVEN `docs/testing-guidance.md` is rendered
- WHEN checked against checklist
- THEN it MUST contain Good/Bad table, bans, bench:guard, and the three pinned examples

### Requirement: CI Enforcement — Lint and Rapid

CI MUST enforce: (a) custom `golangci-lint` `no-source-grep` analyzer (`tools/nosourcegrep` or `internal/tools/lint`, scoped to `*_test.go` with `testdata` allowlist, `rg` fallback), failing on source-grep and passing on `TestBlob_ConcurrentSameBytes`; (b) `go test -run TestRapid ./... -count=1 -timeout 180s`; (c) `go test ./... -count=1 -timeout 180s` + `go vet` + `gofmt -l` MUST remain clean.

#### Scenario: CI blocks source-grep and runs TestRapid

- GIVEN PR adds `expect(src).toContain` in `*_test.go`
- WHEN CI `lint:no-source-grep` and `TestRapid` jobs run
- THEN lint MUST fail and block merge, and `go test -run TestRapid` MUST execute

#### Scenario: CI passes on valid Good test

- GIVEN PR contains only `TestBlob_ConcurrentSameBytes` with `-race`
- WHEN CI runs
- THEN lint MUST pass and `TestRapid` MUST pass
