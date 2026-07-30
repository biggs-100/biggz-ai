# Delta for Plugin System

## MODIFIED Requirements

### Requirement: AgentAdapter Interface

The system MUST define an AgentAdapter interface with these methods: ID() model.AgentID, Name() string, Tier() SupportTier, Detect(ctx, homeDir) (bool, string, string, bool, error), InstallCommand(profile) ([][]string, error), Capabilities() []string, SupportsAutoInstall() bool, SupportsSkills() bool, SupportsSystemPrompt() bool, SupportsMCP() bool, SupportsOutputStyles() bool, SupportsSlashCommands() bool, SupportsSubAgents() bool, GlobalConfigDir(homeDir) string, SystemPromptDir(homeDir) string, SystemPromptFile(homeDir) string, SkillsDir(homeDir) string, CommandsDir(homeDir) string, SubAgentsDir(homeDir) string, EmbeddedSubAgentsDir(homeDir) string, OutputStyleDir(homeDir) string, SettingsPath(homeDir) string, MCPConfigPath(homeDir, serverName) string, SystemPromptStrategy() SystemPromptStrategy, MCPStrategy() model.MCPStrategy, DeployConfig(ctx, cfg) error.
(Previously: 11 methods with ID() string, Detect(ctx) (binaryPath, err), MCPStrategy() string)

#### Scenario: Happy path — agent detected with full metadata

- GIVEN an AgentAdapter for an installed agent
- WHEN Detect(ctx, homeDir) is called
- THEN it MUST return (true, binaryPath, configPath, autoInstallCapable, nil)
- AND ID() MUST return a valid model.AgentID

#### Scenario: Agent not installed

- GIVEN an AgentAdapter for an agent NOT installed
- WHEN Detect(ctx, homeDir) is called
- THEN it MUST return (false, "", "", false, error)
- AND the error MUST explain the agent was not found

#### Scenario: Guard methods for optional features

- GIVEN any AgentAdapter
- WHEN SupportsSkills(), SupportsSystemPrompt(), SupportsMCP(), SupportsOutputStyles(), SupportsSlashCommands(), or SupportsSubAgents() is called
- THEN each MUST return bool
- AND the adapter MUST NOT panic regardless of support level

#### Scenario: InstallCommand generates setup steps

- GIVEN an AgentAdapter that supports auto-install
- WHEN InstallCommand(profile) is called
- THEN it MUST return a non-nil [][]string with executable commands
- AND each command MUST be a valid shell invocation
- AND an adapter without auto-install MAY return nil

#### Scenario: Path methods resolve correctly

- GIVEN an AgentAdapter and a valid homeDir
- WHEN SystemPromptDir, SystemPromptFile, CommandsDir, SubAgentsDir, EmbeddedSubAgentsDir, OutputStyleDir, or MCPConfigPath is called
- THEN each MUST return a non-empty path joined under homeDir
- AND MUST NOT panic for valid inputs

#### Scenario: SystemPromptStrategy returns valid strategy

- GIVEN an AgentAdapter
- WHEN SystemPromptStrategy() is called
- THEN it MUST return one of the 6 known SystemPromptStrategy values
- AND the value MUST match the adapter's prompt injection model

## ADDED Requirements

### Requirement: Tier on AgentAdapter

The system MUST define Tier() SupportTier on AgentAdapter. This method returns the adapter's support commitment level.

#### Scenario: Tier reflects adapter support

- GIVEN the OpenCodeAgentAdapter
- WHEN Tier() is called
- THEN it MUST return SupportTierFirst (or the adapter's appropriate tier)
- AND the value MUST be one of the 5 known SupportTier constants
