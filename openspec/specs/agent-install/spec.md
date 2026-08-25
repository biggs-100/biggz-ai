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
