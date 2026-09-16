# Delta for testing-guidance

## MODIFIED Requirements

### Requirement: CI Enforcement — Lint and Rapid

CI MUST enforce: (a) custom `golangci-lint` `no-source-grep` analyzer (`tools/nosourcegrep` or `internal/tools/lint`, scoped to `*_test.go` with `testdata` allowlist) failing on source-grep and passing on `TestBlob_ConcurrentSameBytes`; the `rg` fallback MUST NOT exist and no CI step MAY invoke `rg`; (b) `go test -run TestRapid ./... -count=1 -timeout 180s`; (c) `go test ./... -count=1 -timeout 180s` + `go vet` + `gofmt -l` MUST remain clean. Every check MUST observe its own checker's exit status: a step MUST NOT derive its verdict from a pipeline whose status belongs to another command, and a missing or failing tool (`go`, `gofmt`) MUST fail the job instead of yielding empty output read as clean.
(Previously: the requirement prescribed the `rg` fallback and left the `go vet` primary piped through `tee` without `pipefail`, so `!` negated `tee`'s status and a real violation printed "nosourcegrep vet passed".)

#### Scenario: CI blocks source-grep and runs TestRapid

- GIVEN PR adds `expect(src).toContain` in `*_test.go`
- WHEN CI `lint:no-source-grep` and `TestRapid` jobs run
- THEN lint MUST fail and block merge, and `go test -run TestRapid` MUST execute

#### Scenario: CI passes on valid Good test

- GIVEN PR contains only `TestBlob_ConcurrentSameBytes` with `-race`
- WHEN CI runs
- THEN lint MUST pass and `TestRapid` MUST pass

#### Scenario: Vet violation is not swallowed by the pipe

- GIVEN the nosourcegrep analyzer reports a violation while its output is piped through `tee`
- WHEN the lint job runs
- THEN the step MUST exit non-zero (via `pipefail` or no pipeline) and MUST NOT print a pass verdict

#### Scenario: Missing formatter is not a clean tree

- GIVEN `gofmt` is unavailable or exits non-zero in the `format` job
- WHEN the formatting step runs
- THEN the step MUST exit non-zero and MUST NOT report "All files are properly formatted."

#### Scenario: No step depends on rg

- GIVEN the CI workflows are inspected for `rg` invocations and `|| true` producers in guard steps
- WHEN the lint job's steps are checked
- THEN zero `rg` invocations MUST be found
