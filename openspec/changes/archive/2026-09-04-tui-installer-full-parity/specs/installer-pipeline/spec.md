# Delta for installer-pipeline

## ADDED Requirements

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
