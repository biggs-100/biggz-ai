# Agent Install Specification

## Purpose

The Agent Install domain defines how biggz-ai detects installed AI coding agents, deploys SDD skill files and configuration to them, and performs safe file merge operations. It enables the `biggz install` command.

## Requirements

### Requirement: Agent Detection

The system MUST detect installed AI coding agents via the AgentAdapter.Detect() method. The OpenCode adapter MUST use `exec.LookPath("opencode")` for detection. If the agent is not found, the install process MUST report "not detected" and exit with a non-zero status.

#### Scenario: Agent installed — binary path returned

- GIVEN the OpenCode agent binary is on the system PATH
- WHEN Detect() is called
- THEN it MUST return the full binary path without error

#### Scenario: Agent not installed — error returned

- GIVEN the OpenCode agent binary is NOT on the system PATH
- WHEN Detect() is called
- THEN it MUST return an error indicating the agent was not found

### Requirement: Asset Deployment

The system MUST deploy SDD skill files to the agent's skills directory. The system MUST merge an SDD overlay configuration into the agent's JSONC configuration file. The system MUST support a `--dry-run` flag that reports what actions would be performed without writing any files.

#### Scenario: Dry-run reports what would be installed

- GIVEN the `--dry-run` flag is set
- WHEN the install command is invoked
- THEN the system MUST print which skills would be deployed
- AND the system MUST print which config changes would be made
- AND the system MUST NOT write any files to disk

#### Scenario: Actual deploy writes skills and merges config

- GIVEN the `--dry-run` flag is NOT set
- WHEN the install command is invoked
- THEN skill files MUST be written to the agent's skills directory
- AND the SDD overlay MUST be merged into the agent's JSONC config
- AND the command MUST exit with status zero

#### Scenario: Skills already deployed — idempotent

- GIVEN all SDD skills are already present in the agent's skills directory
- AND the agent's config already contains the SDD overlay
- WHEN the install command is invoked
- THEN no files MUST be modified
- AND the command MUST exit with status zero

### Requirement: File Merge

The system MUST support atomic file writes that complete fully or fail without leaving partial output. The system MUST merge JSON/JSONC files using sentinel-based section replacement. The system MUST inject content sections into markdown files using section markers.

#### Scenario: Atomic write completes fully or fails cleanly

- GIVEN a target file path
- WHEN a write operation is invoked
- THEN the content MUST be written to a temporary path first
- AND the temp path MUST be renamed atomically to the target path
- AND if the write or rename fails, the target path MUST be unchanged

#### Scenario: JSON merge adds new section to config

- GIVEN an existing JSONC config with "skills" and "mcp_servers" sections
- WHEN a JSON merge adds a "sub_agents" section
- THEN the "skills" and "mcp_servers" sections MUST be preserved unchanged
- AND a new "sub_agents" section MUST appear in the output

#### Scenario: Existing section in JSONC is replaced, others preserved

- GIVEN an existing JSONC config with a "skills" section containing two entries
- WHEN a JSON merge replaces the "skills" section with one entry
- THEN the "skills" section MUST contain only the new entry
- AND all other sections MUST be preserved unchanged

### Requirement: Plugintest Support

The plugintest.FakeAgent MUST support a TempDir for filesystem-based tests. SetTempDir(t.TempDir()) MUST route all config paths (GlobalConfigDir, SkillsDir, SettingsPath) to the specified temp directory.

#### Scenario: TempDir set — Detect returns configured path

- GIVEN a FakeAgent with TempDir set to a temporary directory
- WHEN Detect() is called
- THEN it MUST return a path within the configured TempDir

#### Scenario: DeployConfig writes to TempDir — files exist after deploy

- GIVEN a FakeAgent with TempDir set to a temporary directory
- WHEN DeployConfig() is invoked
- THEN written skill files MUST exist under the configured TempDir
- AND no files MUST be written outside the TempDir

### Requirement: REQ-INST-001 — Pi Web Search Extension Deployment

The system MUST provide `DeployPiWebSearch(ctx, homeDir)` that writes `internal/assets/pi/biggz-web-search.js` to `~/.pi/agent/extensions/biggz-web-search.js` via `filemerge.WriteFileAtomic`. It MUST create parent directories, MUST be idempotent, MUST integrate with `Run()` and `Result.PiWebSearch`, and MUST support TempDir routing for tests.

#### Scenario: Atomic deploy creates extension

- GIVEN Pi is installed and `homeDir` resolves to `~/.pi/agent`
- WHEN `DeployPiWebSearch(ctx, homeDir)` is called
- THEN `extensions/biggz-web-search.js` MUST exist with embedded bytes written atomically via temp+rename
- AND `Result.PiWebSearch` MUST indicate deployed

#### Scenario: Idempotent second deploy

- GIVEN `biggz-web-search.js` already exists with identical embedded bytes
- WHEN `DeployPiWebSearch` is called again
- THEN no file MUST be modified and the function MUST return success

#### Scenario: Deploy via Run()

- GIVEN `install --agent pi` invokes `Run(ctx, cfg)`
- WHEN `Run()` executes
- THEN it MUST call `DeployPiWebSearch` alongside `DeployPiSubAgents`, `DeployPiThinkingWrap`, and `DeployPiLastModel`

#### Scenario: TempDir isolation for tests

- GIVEN a `plugintest.FakeAgent` with `TempDir` set
- WHEN `DeployPiWebSearch` is invoked
- THEN the file MUST be written under `TempDir` and no file outside `TempDir` MUST be modified

#### Scenario: Self-heal removes legacy if present

- GIVEN a legacy extension `biggz-pi-pretty.js` exists (or any deprecated web-search variant)
- WHEN `DeployPiWebSearch` or `Run()` executes
- THEN the legacy file MUST be removed atomically if its content is outdated

### Requirement: REQ-INST-002 — Overlay and Skill Gating Integration

The system MUST expose `web_search`/`web_fetch` to `sdd-research` only via `internal/assets/opencode/sdd-overlay-multi.json` and `sdd-research` skill docs. Non-research agents MUST NOT receive the tools. `internal/assets/embed.go` with `//go:embed all:pi` MUST automatically include the new asset without code change.

#### Scenario: Overlay allows web tools for sdd-research

- GIVEN `sdd-overlay-multi.json` is merged for `sdd-research`
- WHEN agent tools are resolved under `open-web` grant
- THEN `web_search` and `web_fetch` MUST be in the tool allowlist

#### Scenario: Non-research overlay unchanged

- GIVEN `sdd-overlay-multi.json` is resolved for `sdd-explore` or `sdd-apply`
- WHEN tools are listed
- THEN `web_search`/`web_fetch` MUST be absent

#### Scenario: Embed coverage

- GIVEN `biggz-web-search.js` exists under `internal/assets/pi`
- WHEN `assets.FS` is read
- THEN the file MUST be included via `//go:embed all:pi` without modifying `embed.go`

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
