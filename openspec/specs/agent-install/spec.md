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
