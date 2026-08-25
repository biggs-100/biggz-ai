# Review Lenses Specification

## Purpose

Durable slots `risk→resilience→readability→reliability` (gentle-ai order) as sequential heuristics.

## Requirements

### Requirement: Lens Interface

System MUST define `Lens` in `internal/review/lens/types.go` with `ID() string` and `Analyze(ctx,LensInput)(LensResult,error)`. `LensInput` MUST derive from `DeriveRiskInput` (Paths, ChangedLines, DiffSummary, hunks). `LensResult` MUST carry `LensID`, findings with `ProofRefs` and class. MUST NOT be in `plugin/`.

#### Scenario: Interface satisfied

- GIVEN a `Lens` impl
- WHEN `ID()` and `Analyze` called with derived input
- THEN `ID()` MUST return expected ID and result `LensID` MUST match

#### Scenario: Reuses DeriveRiskInput

- GIVEN `DeriveRiskInput` output for a commit
- WHEN lens executes
- THEN it MUST reuse it and MUST NOT run second `git diff`

### Requirement: Registry Contract

System MUST provide build-time `Registry map[string]Lens` in `registry.go` with `RegisterLens` and `Ordered([]string) []Lens`. Population at `cmd/biggz` init. Duplicate ID last-win; unknown ID skipped.

#### Scenario: Ordered lookup

- GIVEN registry with four lenses
- WHEN `Ordered([risk,resilience,readability,reliability])` called
- THEN order preserved and unknowns skipped

### Requirement: Lens Order Freeze

`PlanLenses(RiskHigh,nil)` MUST equal `[risk,resilience,readability,reliability]`. Declared lenses win without aliasing. Asserted by test.

#### Scenario: Canonical high order

- GIVEN `RiskHigh` no declared
- WHEN `PlanLenses` called
- THEN result MUST equal `[risk,resilience,readability,reliability]`

#### Scenario: Declared wins

- GIVEN `RiskHigh` with `[risk]`
- WHEN `PlanLenses` called
- THEN result MUST equal `[risk]`

### Requirement: R2 Readability

R2 (`readability`) MUST emit deterministic finding on `go/parser.ParseFile` failure for changed `.go` files, and inferential when `DiffSummary[path]>400` (any) or `>200` (Go). MUST NOT check mixedCase+underscores.

#### Scenario: Parser failure

- GIVEN changed `foo.go` failing `go/parser`
- WHEN R2 analyzes
- THEN deterministic finding with parser `ProofRef` MUST appear

#### Scenario: Line threshold

- GIVEN `DiffSummary["pkg/foo.go"]=450`
- WHEN R2 analyzes
- THEN inferential finding MUST appear

### Requirement: R3 Reliability

R3 (`reliability`) MUST emit for missing sibling `_test.go` and error-handling token hits. MUST NOT emit volume findings.

#### Scenario: Missing test

- GIVEN `internal/foo/bar.go` without `bar_test.go`
- WHEN R3 analyzes
- THEN inferential finding with `ProofRef` MUST appear

### Requirement: R4 Resilience

R4 (`resilience`) MUST be hunk-bounded, inferential-only for timeout/context/concurrency/cleanup. MUST cap at 8MiB and never fallback to full file.

#### Scenario: Hunk finding

- GIVEN hunk with `http.Client{}` lacking timeout
- WHEN R4 analyzes within cap
- THEN inferential finding with hunk `ProofRef` MUST appear

#### Scenario: Cap enforced

- GIVEN hunks >8MiB
- WHEN R4 analyzes
- THEN it MUST truncate, return truncated flag, no error

### Requirement: ExternalLensAdapter

System MUST provide `ExternalLensAdapter` in `external/adapter.go` wrapping `capture-result` JSON (`gentle-ai.lens-result/v1` prefix) as `Lens`. No schema change; LLM prompts out of scope.

#### Scenario: Capture bridged

- GIVEN valid capture JSON
- WHEN adapter `Analyze` called
- THEN findings MUST equal payload and hash prefix preserved

#### Scenario: Missing capture

- GIVEN no payload
- WHEN adapter analyzes
- THEN error with zero findings MUST return

### Requirement: Sequential Stage Wiring

Each lens MUST be `pipeline.Stage` executed in `PlanLenses` order sequentially with reverse rollback on failure. No `graph.go`, DAG, or parallel execution.

#### Scenario: Sequential rollback

- GIVEN stages `[risk,resilience,readability,reliability]`
- WHEN `readability` fails
- THEN prior stages rollback reverse and later stages not run

#### Scenario: No DAG

- GIVEN `internal/review/lens/...`
- WHEN grepped for planner graph
- THEN zero imports MUST exist

### Requirement: Evidence Classes and Rollback

Findings default `inferential`; only concrete `ProofRefs` MAY be `deterministic`. All candidate-causal. Lenses stateless (no ledger migration); `capture-result --order` obeys freeze; optional `ComponentEntry` for catalog.

#### Scenario: Inferential default

- GIVEN R3/R4 emission without parser proof
- WHEN finding created
- THEN class MUST be `inferential`

#### Scenario: Stateless revert

- GIVEN `internal/review/lens/*` and order reverted
- WHEN `go test ./...` runs
- THEN R1 baseline MUST pass with no migration
