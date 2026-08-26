# SDD Status Specification

## Purpose

SDD Status v2 (`biggz-ai.sdd-status/v2`) is the sole contract for planning, tasks, verification, and attempt authority. It replaces v1 with a clean break: authority-free projection, explicit v2 default, and cumulative rescope semantics that never inherit a fresh budget.

## Requirements

### Requirement: SDD Status v2 Sole Contract

The system MUST expose `biggz-ai.sdd-status/v2` as the sole status contract. `SchemaVersion` MUST be `2` and default contract MUST be `biggz-ai.sdd-status/v2`. Requests for `v1` or unknown contracts MUST fail read-only with the single instruction: start a fresh implementation state and rerun `biggz sdd-status --contract biggz-ai.sdd-status/v2`. The v2 projection MUST contain only SDD planning/tasks/verification/attempt authority-free keys (`schemaName`, `artifactStore`, `planningHome`, `changeRoot`, `artifactPaths`, `contextFiles`, `artifacts`, `taskProgress`, `dependencies`, `applyState`, `actionContext`, `relationships`, `remediationState`, `reviewOffer`, `consent`, `nextRecommended`, `blockedReasons`) and MUST NOT retain `reviewGate`, `reviewTransaction`, `runtimeStatus`, `lineageId`, `generation`, `fixBatch`, or `correctionBudget`. Rescope MUST carry forward cumulative attempts/lines unchanged and MUST NOT inherit a fresh budget. Tagged tests, shipped assets, and goldens MUST NOT pin `v1`.

#### Scenario: Default is v2

- GIVEN `biggz sdd-status thin` with no `--contract`
- WHEN parsing runs
- THEN contract MUST be `biggz-ai.sdd-status/v2` and `SchemaVersion` MUST be `2`

#### Scenario: v1 fails with fresh instruction

- GIVEN `biggz sdd-status --contract biggz-ai.sdd-status/v1`
- WHEN validation runs
- THEN it MUST fail with `unsupported sdd-status contract` and the fresh v2 rerun instruction
- AND default parsing after refusal MUST remain `v2`

#### Scenario: v2 projection is authority-free

- GIVEN a resolved status
- WHEN projected via `ProjectStatusV2`
- THEN JSON keys MUST exactly match the v2 set and MUST NOT contain `runtimeStatus` or lineage keys
- AND `artifactPaths`/`artifacts`/`remediationState` MUST contain only the frozen v2 sub-keys

#### Scenario: Rescope never inherits exhausted allowance

- GIVEN a terminal objective with `CumulativeAttempts=5` and `MaxAttempts=5`
- WHEN rescope to narrower `MaxAttempts=3` is evaluated
- THEN new ceiling MUST be measured against `5` already consumed, not a fresh `0`
- AND system MUST NOT reset cumulative counters

#### Scenario: Historical v1 pins are rejected

- GIVEN tagged `*_test.go` and shipped `sdd-status-contract.md` and goldens
- WHEN scanned
- THEN they MUST NOT contain `gentle-ai.sdd-status/v1` or `ProjectStatusV1` pinning
