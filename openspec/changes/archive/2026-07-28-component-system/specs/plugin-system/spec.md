# Delta for Plugin System

## ADDED Requirements

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

## MODIFIED Requirements

### Requirement: AgentAdapter Interface

The system MUST define an AgentAdapter interface with ID() returning string, Name() returning string, Detect(ctx) returning (binaryPath string, err), Capabilities() returning []string, SupportsAutoInstall() returning bool, GlobalConfigDir(homeDir string) returning string, SkillsDir(homeDir string) returning string, SettingsPath(homeDir string) returning string, MCPStrategy() returning string, and DeployConfig(ctx, cfg) returning error.
(Previously: AgentAdapter had Capabilities() returning []Capability, no SupportsAutoInstall, no MCPStrategy.)

#### Scenario: Happy path — agent detected

- GIVEN an AgentAdapter for an agent that is installed on the system
- WHEN Detect is called
- THEN it MUST return the binary path without error

#### Scenario: Agent not installed

- GIVEN an AgentAdapter for an agent that is NOT installed
- WHEN Detect is called
- THEN it MUST return an error indicating the agent was not found

#### Scenario: Auto-install check for installed agent

- GIVEN an AgentAdapter that supports auto-install
- WHEN SupportsAutoInstall is called after a failed Detect
- THEN the caller MAY use SupportsAutoInstall to decide whether to attempt installation

### Requirement: Build-Time Registry

The system MUST provide a Registry with RegisterLens(plugin), RegisterAdapter(adapter), GetLens(id), and GetAdapter(id) methods. Registration MUST happen at build time via explicit wiring. The Registry MUST expose ListAll() returning []CatalogEntry.
(Previously: Registry had RegisterAgent/GetAgent. Renamed to RegisterAdapter/GetAdapter.)

#### Scenario: Happy path — register and retrieve adapter

- GIVEN an empty Registry
- WHEN an Adapter with ID "test-adapter" is registered via RegisterAdapter
- THEN GetAdapter("test-adapter") MUST return the registered adapter
- AND GetAdapter("unknown") MUST return nil

#### Scenario: ListAll reflects registrations

- GIVEN a Registry with two registered adapters
- WHEN ListAll() is called
- THEN it MUST return exactly 2 CatalogEntry values
- AND the entries MUST match the registered adapter IDs
