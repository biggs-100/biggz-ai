# Installer Pipeline Specification

## Purpose

Lean `internal/pipeline` (~150L) replacing `install.Run` (2689L) / gentle-ai 5491L. Provides `StagePlan` Prepare→Apply, `Orchestrator`, `Step`, `ProgressEvent`, `ExecutionResult`, `RollbackPolicy` for dry-run preview, lossless progress, reversible installs.

## Requirements

### Requirement: REQ-PIPELINE-001 — StagePlan Prepare/Apply Contract

The system MUST define `StagePlan` with `Prepare(ctx) (*PlanPreview, error)` (read-only, zero writes outside TempDir) and `Apply(ctx, ProgressChan) (*ExecutionResult, error)` (sequential). `Step` MUST have `Name()`, `Prepare`, `Apply(chan ProgressEvent)`, `Rollback`.

#### Scenario: Prepare preview without writes

- GIVEN a `StagePlan` with 3 steps
- WHEN `Prepare(ctx)` is called
- THEN it MUST return `PlanPreview` listing step Names in order
- AND no file outside TempDir MUST be modified

#### Scenario: Apply executes steps in order

- GIVEN `Prepare` succeeded and produced a `PlanPreview`
- WHEN `Apply(ctx, ch)` is called
- THEN steps MUST execute in preview order
- AND `ExecutionResult` MUST contain per-step status

#### Scenario: Prepare failure blocks Apply

- GIVEN `Prepare` errors on step 2
- WHEN `Apply` is invoked
- THEN `Apply` MUST NOT execute
- AND error MUST identify the step

### Requirement: REQ-PIPELINE-002 — Orchestrator Execution

The system MUST provide `Orchestrator.Run(ctx, StagePlan) (*ExecutionResult, error)` calling `Prepare` then `Apply`. `ExecutionResult` MUST have `Success bool`, `Steps []StepResult`, `Error error`. MUST support `RollbackPolicy`.

#### Scenario: Orchestrator success

- GIVEN valid `StagePlan` and `Orchestrator`
- WHEN `Orchestrator.Run(ctx, plan)` is called
- THEN `Success` MUST be true
- AND `Steps` MUST have one `Applied==true` per Step

#### Scenario: Orchestrator surfaces Apply error

- GIVEN step 2 `Apply` returns error
- WHEN `Orchestrator.Run` executes
- THEN `ExecutionResult.Success` MUST be false
- AND `Error` MUST wrap the step error with step Name

### Requirement: REQ-PIPELINE-003 — ProgressEvent and Lossless Channel

The system MUST define `ProgressEvent{Step string, Percent 0..100, Message string}` via buffered `ProgressChan`. Events MUST be monotonic, lossless, and channel MUST be closed by `Apply`; success MUST end at 100.

#### Scenario: Lossless 0→100 streaming

- GIVEN `Apply` with buffered `ProgressChan` (cap >= steps*2)
- WHEN steps emit events 0, 50, 100
- THEN receiver MUST observe all events in order
- AND final event `Percent` MUST be 100

#### Scenario: Channel closed on completion

- GIVEN `Apply` completes (success or failure)
- WHEN receiver ranges over channel
- THEN channel MUST be closed
- AND no goroutine leak MUST remain

#### Scenario: No drops under burst

- GIVEN 20 rapid events in Apply
- WHEN drained via receiver
- THEN count received MUST equal count sent

### Requirement: REQ-PIPELINE-004 — RollbackPolicy and Reversibility

The system MUST define `RollbackPolicy` (`RollbackOnFailure`/`NoRollback`) and idempotent `Step.Rollback`. On failure with `RollbackOnFailure`, `Orchestrator` MUST rollback completed steps reverse-order; rollback errors MUST be aggregated without hiding Apply error.

#### Scenario: Rollback on partial failure

- GIVEN steps A,B,C where C `Apply` fails and policy `RollbackOnFailure`
- WHEN `Orchestrator.Run` executes
- THEN `Rollback` MUST be called on B then A in reverse order
- AND C `Rollback` MUST NOT be called (never applied)

#### Scenario: Rollback idempotency

- GIVEN step `Rollback` called twice
- WHEN second `Rollback` executes
- THEN it MUST succeed without side effects
- AND system MUST remain in pre-Apply state

#### Scenario: Rollback error aggregation

- GIVEN rollback of step A fails
- WHEN orchestrator aggregates
- THEN `ExecutionResult.Error` MUST contain both original Apply error and rollback error

### Requirement: REQ-PIPELINE-005 — Dry-Run Preview via Prepare Only

The system MUST route `--dry-run` to `Prepare` only with TempDir isolation; MUST NOT write to `~/.biggz-ai/state.json` or agent dirs and MUST print preview.

#### Scenario: Dry-run zero writes

- GIVEN `--dry-run` flag set and `homeDir` is TempDir
- WHEN `Orchestrator.Run` is invoked via CLI path
- THEN only `Prepare` MUST execute
- AND zero files MUST exist outside TempDir after completion

#### Scenario: Non-dry-run executes Apply after Prepare

- GIVEN `--dry-run` is false
- WHEN CLI invokes install
- THEN both `Prepare` and `Apply` MUST execute
- AND files MUST be written atomically via `filemerge.WriteFileAtomic`

### Requirement: REQ-WIZ-006 — Progress-channel wiring to Installing view

The Installing stage MUST consume the existing Orchestrator `ProgressChan(32)` via `tea.Cmd` without changing pipeline API. Events MUST forward losslessly; channel close at 100% MUST transition to Complete, failure MUST surface error state. `RollbackOnFailure` semantics MUST be unchanged.

#### Scenario: Lossless feed to Installing
- GIVEN Orchestrator emits 10 events 0→100 on `ProgressChan(32)`
- WHEN Installing view consumes via `tea.Cmd`
- THEN all 10 MUST render and final Percent MUST be 100

#### Scenario: Close transitions to Complete
- GIVEN channel closed after 100% with `Success==true`
- WHEN final event processed
- THEN wizard MUST advance Installing → Complete

#### Scenario: Failure surfaces without API change
- GIVEN step Apply fails under `RollbackOnFailure`
- WHEN Installing receives error event
- THEN view MUST show failed state and rollback order MUST stay reverse-Apply
