# Delta for review-lenses

## MODIFIED Requirements

### Requirement: R2 Readability

R2 (`readability`) MUST emit a deterministic finding on `go/parser.ParseFile` failure for changed `.go` files, and inferential findings when `DiffSummary[path]>400` (any) or `>200` (Go). R2 MUST additionally emit inferential findings `R2-CYCLO` when cyclomatic >15 and `R2-COGNIT` when cognitive >20 for changed functions in critical packages (`internal/review`, `internal/sdd`, `internal/verification`) that intersect the PR hunk set. All complexity findings MUST be inferential, carry `ProofRefs` on the function (`file:line`, `function`, measured value, threshold), and be hunk-bounded via `DeriveRiskInput` (Paths/ChangedLines/hunks). R2 MUST reuse `DeriveRiskInput` and MUST NOT run a second `git diff`. R2 MUST exclude `*_test.go` from `R2-CYCLO`/`R2-COGNIT` blocking-equivalent signaling (test findings are informational warnings). R2 MUST NOT check mixedCase+underscores.
(Previously: only parser failure and line-threshold findings; no complexity signals, no hunk-bounded complexity, no ProofRef distinction for complexity)

#### Scenario: Parser failure — unchanged

- GIVEN changed `foo.go` failing `go/parser`
- WHEN R2 analyzes
- THEN deterministic finding with parser `ProofRef` MUST appear

#### Scenario: Line threshold — unchanged

- GIVEN `DiffSummary["pkg/foo.go"]=450`
- WHEN R2 analyzes
- THEN inferential finding MUST appear

#### Scenario: R2-CYCLO on changed function

- GIVEN PR hunk modifies `internal/review/lens/foo.go:FuncFoo` with cyclomatic 18 (>15) and `DiffSummary` includes the file
- WHEN R2 analyzes with `DeriveRiskInput` hunks
- THEN inferential finding `R2-CYCLO` with `ProofRef` (`foo.go:10`, `FuncFoo`, `18 >15`) MUST appear

#### Scenario: R2-COGNIT on changed function

- GIVEN PR hunk modifies `internal/sdd/bar.go:FuncBar` with cognitive 25 (>20)
- WHEN R2 analyzes
- THEN inferential finding `R2-COGNIT` with `ProofRef` (`bar.go:42`, `FuncBar`, `25 >20`) MUST appear

#### Scenario: Hunk-bounded — legacy violation not in hunk

- GIVEN `internal/verification/old.go:FuncOld` has cyclomatic 22 on base but PR hunks do not touch `FuncOld`
- WHEN R2 analyzes
- THEN no `R2-CYCLO` finding for `FuncOld` MUST appear

#### Scenario: Reuses DeriveRiskInput — no second diff

- GIVEN `DeriveRiskInput` output for a commit
- WHEN R2 analyzes complexity
- THEN it MUST reuse it and MUST NOT run a second `git diff`

#### Scenario: Test file is informational only

- GIVEN hunk modifies `internal/review/foo_test.go:TestFoo` with cyclomatic 30
- WHEN R2 analyzes
- THEN any `R2-CYCLO` for `TestFoo` MUST be informational warning class and MUST NOT be treated as blocking-equivalent

#### Scenario: Finding is inferential with ProofRef

- GIVEN R2 emits `R2-CYCLO` for a changed function
- WHEN finding is inspected
- THEN class MUST be `inferential` and `ProofRefs` MUST contain file, line, and threshold evidence
