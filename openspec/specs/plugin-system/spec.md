# Plugin System Specification

## Purpose

The Plugin System domain defines the plugin interfaces and registry for extending review capabilities. It specifies LensPlugin for analysis, AgentAdapter for agent discovery and integration, the build-time Registry, and the Pipeline execution model with rollback support.

## Requirements

### Requirement: LensPlugin Interface

The system MUST define a LensPlugin interface with ID() returning string, Name() returning string, Version() returning string, Analyze(ctx, subject) returning LensResult, and Policies() returning []Policy.

#### Scenario: Happy path — lens analysis

- GIVEN a LensPlugin registered with ID "dummy-lens" and a valid ReviewSubject
- WHEN Analyze is called with the subject
- THEN the LensResult MUST contain findings relevant to the subject
- AND Policies() MUST return at least one Policy

#### Scenario: Invalid subject

- GIVEN a LensPlugin
- WHEN Analyze is called with a nil or empty subject
- THEN the plugin MUST return an error
- AND NOT panic or hang

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

### Requirement: Build-Time Registry

The system MUST provide a Registry with RegisterLens(plugin), RegisterAdapter(adapter), GetLens(id), GetAdapter(id), and ListAll() returning []CatalogEntry methods. Registration MUST happen at build time via explicit wiring. The Registry MUST NOT support dynamic loading.

#### Scenario: Happy path — register and retrieve agent

- GIVEN an empty Registry
- WHEN an AgentAdapter with ID "test-agent" is registered via RegisterAdapter
- THEN GetAdapter("test-agent") MUST return the registered adapter
- AND GetAdapter("unknown") MUST return nil

#### Scenario: Duplicate agent registration

- GIVEN a Registry with a registered AgentAdapter "test-agent"
- WHEN RegisterAdapter is called again with the same ID "test-agent"
- THEN the Registry MUST return an error or replace the existing registration
- AND the behavior MUST be documented and consistent

### Requirement: Pipeline Stage Execution

The system MUST define a Stage interface with Name() returning string, Execute(ctx, state) returning error, and Rollback(ctx, state) returning error. Pipeline MUST execute stages sequentially and run reverse-ordered rollback on any stage failure.

#### Scenario: Happy path — all stages succeed

- GIVEN a Pipeline with three registered stages: A, B, C
- WHEN Execute is called on a ReviewState
- THEN stage A MUST run, then B, then C
- AND Rollback MUST NOT be called on any stage
- AND the ReviewState MUST be updated by all stages

#### Scenario: Stage failure triggers rollback

- GIVEN a Pipeline with three stages: A, B, C
- WHEN stage B fails with an error
- THEN stage B's Rollback MUST be called
- AND stage A's Rollback MUST be called
- AND stage C's Rollback MUST NOT be called (stage C did not execute)
- AND the pipeline MUST return the error from stage B

### Requirement: Orchestrator

The system MUST define an Orchestrator with a single Execute(ctx, subject) method that runs the full pipeline and returns *ReviewState and error.

#### Scenario: Happy path — full execution

- GIVEN an Orchestrator with a configured pipeline and registry
- WHEN Execute is called with a valid ReviewSubject
- THEN a *ReviewState MUST be returned with Status set to Completed
- AND the evidence chain MUST contain entries from each pipeline stage

#### Scenario: Pipeline failure

- GIVEN an Orchestrator with a configured pipeline
- WHEN Execute is called and a stage fails
- THEN *ReviewState MUST be returned with Status set to Failed
- AND the error MUST be non-nil

## Added Requirements

### Requirement: AgentAdapter Config Path Methods

The AgentAdapter interface MUST define three config path methods: GlobalConfigDir(homeDir string) returning string, SkillsDir(homeDir string) returning string, and SettingsPath(homeDir string) returning string. These methods MUST resolve platform-appropriate paths using `filepath.Join(homeDir, ...)`. (Previously: AgentAdapter had no config path methods.)

#### Scenario: GlobalConfigDir returns agent config directory

- GIVEN a configured AgentAdapter and a valid homeDir path
- WHEN GlobalConfigDir(homeDir) is called
- THEN it MUST return the agent's global configuration directory path
- AND the path MUST be constructed by joining homeDir with the agent-specific config subpath

#### Scenario: SkillsDir returns agent skills directory

- GIVEN a configured AgentAdapter and a valid homeDir path
- WHEN SkillsDir(homeDir) is called
- THEN it MUST return the subdirectory path where the agent stores skills
- AND the path MUST be a subdirectory of the agent's config directory

#### Scenario: SettingsPath returns agent config file path

- GIVEN a configured AgentAdapter and a valid homeDir path
- WHEN SettingsPath(homeDir) is called
- THEN it MUST return the full file path to the agent's settings configuration file
- AND the path MUST include the agent's config file name (e.g., opencode.jsonc)

#### Scenario: Empty homeDir string

- GIVEN a configured AgentAdapter and an empty homeDir string
- WHEN any of the three path methods is called
- THEN the return value MAY be a relative path or have undefined behavior
- AND the system MUST NOT panic

### Requirement: SupportsAutoInstall on AgentAdapter

The AgentAdapter interface MUST add a `SupportsAutoInstall() bool` method. This method MUST return true if the agent supports automatic installation (binary download and setup) without manual intervention.

#### Scenario: Happy path — auto-install supported

- GIVEN an AgentAdapter for an agent that supports binary download
- WHEN SupportsAutoInstall is called
- THEN it MUST return true

#### Scenario: Auto-install not supported

- GIVEN an AgentAdapter for an agent that requires manual installation
- WHEN SupportsAutoInstall is called
- THEN it MUST return false

### Requirement: MCPStrategy on AgentAdapter

The AgentAdapter interface MUST add an `MCPStrategy() string` method. This method MUST return the MCP (Model Context Protocol) strategy name the adapter uses, such as "stdio", "http", or "disabled".

#### Scenario: Happy path — MCP strategy returned

- GIVEN an AgentAdapter with a configured MCP strategy
- WHEN MCPStrategy is called
- THEN it MUST return a non-empty string describing the strategy

#### Scenario: No MCP strategy

- GIVEN an AgentAdapter that does not use MCP
- WHEN MCPStrategy is called
- THEN it MAY return "disabled" or an empty string
- AND the system MUST handle both values without error

### Requirement: AgentAdapter Enriched Capabilities

The AgentAdapter Capabilities() method MUST return `[]string` instead of `[]Capability`. Each string MUST represent a capability name. Existing callers MUST be updated to compare strings instead of structs. (Previously: Capabilities() returned `[]Capability`.)

#### Scenario: Happy path — string capabilities

- GIVEN an AgentAdapter with two capabilities "review" and "install"
- WHEN Capabilities() is called
- THEN it MUST return a slice with "review" and "install"
- AND each entry MUST be a plain string

### Requirement: Registry Integration with Catalog

The Registry MUST expose a `ListAll() []CatalogEntry` method. This method MUST construct CatalogEntry values from each registered adapter's metadata. This bridges the agent registry with the component catalog.

#### Scenario: Happy path — registry returns catalog entries

- GIVEN a Registry with 3 registered adapters
- WHEN ListAll() is called
- THEN it MUST return exactly 3 CatalogEntry values
- AND each entry's ID MUST match the corresponding adapter's ID

### Requirement: Tier on AgentAdapter

The system MUST define Tier() SupportTier on AgentAdapter. This method returns the adapter's support commitment level.

#### Scenario: Tier reflects adapter support

- GIVEN the OpenCodeAgentAdapter
- WHEN Tier() is called
- THEN it MUST return SupportTierFirst (or the adapter's appropriate tier)
- AND the value MUST be one of the 5 known SupportTier constants
