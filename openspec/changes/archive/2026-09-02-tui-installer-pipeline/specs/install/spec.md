# Delta for agent-install

## Purpose

Decompose monolithic `internal/install/install.go` (2689L) into reversible pipeline steps executed via `Orchestrator`/`StagePlan`. Preserve `install.Run` contract while enabling Prepare preview, atomic deploys, and rollback.

## ADDED Requirements

### Requirement: REQ-INSTALL-PIPE-001 — Step Decomposition of Monolith

The system MUST split `install.go` 2689L into discrete `steps/` each implementing `pipeline.Step`. Each step MUST encapsulate one domain: skills deploy, overlay merge, `~/.pi` extensions, or `state.json` update. The system MUST preserve existing `install.Run(ctx, cfg) (Result, error)` signature delegating to `Orchestrator`. Each step `Name()` MUST be stable and human-readable.

#### Scenario: Run delegates to Orchestrator with steps

- GIVEN `Run(ctx, cfg)` called with agent `opencode`
- WHEN it executes
- THEN it MUST construct `StagePlan` with steps covering skills + overlay + state
- AND `Orchestrator.Run` MUST be invoked

#### Scenario: Step count covers monolith responsibilities

- GIVEN monolith responsibilities enumerated
- WHEN steps are listed
- THEN count MUST be >=3 and each responsibility MUST map to exactly one step

#### Scenario: Step Name stable

- GIVEN step `Name()` called twice
- WHEN compared
- THEN results MUST be identical

### Requirement: REQ-INSTALL-PIPE-002 — Reversible and Idempotent Steps

Each `Step` MUST implement `Apply` that is idempotent and `Rollback` that restores pre-Apply state. `Rollback` MUST be safe to call even if `Apply` never completed or was partially applied. Steps MUST use `filemerge.WriteFileAtomic` (temp+rename) for all file writes and MUST NOT leave partial files on failure.

#### Scenario: Idempotent Apply twice

- GIVEN step already applied (files match expected)
- WHEN `Apply` is called again with same inputs
- THEN no file MUST be modified
- AND result MUST be success

#### Scenario: Rollback restores state after Apply

- GIVEN step `Apply` created file `skills/foo.md`
- WHEN `Rollback` is called
- THEN file MUST be removed or restored to pre-Apply content atomically

#### Scenario: Partial Apply rollback cleans writes

- GIVEN `Apply` fails after writing 2 of 5 files
- WHEN `Orchestrator` triggers rollback
- THEN those 2 files MUST be reverted and remaining 3 MUST not exist

### Requirement: REQ-INSTALL-PIPE-003 — Prepare Zero-Write and --agent Routing

`Prepare` on each step MUST validate inputs, resolve paths via `Adapter.GlobalConfigDir/SkillsDir`, and MUST NOT write outside TempDir. The system MUST route `--agent <id>` through `agent-registry` Adapter and MUST merge per-agent overlay. `--yes` MUST skip prompts but MUST still run `Prepare` validation.

#### Scenario: Prepare validates without writes

- GIVEN `--agent pi` and steps constructed
- WHEN `Prepare` runs in TempDir mode
- THEN it MUST validate embedded assets exist
- AND zero files MUST be written to real HOME

#### Scenario: Agent routing per Adapter

- GIVEN `--agent opencode` vs `--agent pi`
- WHEN steps resolve target dirs
- THEN `GlobalConfigDir`/`SkillsDir` MUST match the selected adapter

#### Scenario: --yes still validates

- GIVEN `--yes` set with invalid `--agent` id
- WHEN `Prepare` executes
- THEN it MUST return error and `Apply` MUST not run

### Requirement: REQ-INSTALL-PIPE-004 — Plugintest TempDir Isolation Preserved

`plugintest.FakeAgent` with `SetTempDir(t.TempDir())` MUST route all step `Apply`/`Rollback` file operations to TempDir. No file outside TempDir MUST be modified during tests.

#### Scenario: Steps write to TempDir only

- GIVEN `FakeAgent` with TempDir `/tmp/test-xxx`
- WHEN `Run(ctx, cfg)` with pipeline executes
- THEN all skill/config files MUST exist under `/tmp/test-xxx`
- AND no file outside MUST be created

#### Scenario: Rollback in TempDir

- GIVEN steps applied in TempDir and then `Rollback` triggered
- WHEN rollback completes
- THEN TempDir MUST be restored to pre-Apply state
