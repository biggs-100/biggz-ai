# Delta for Agent Registry

## ADDED Requirements

### Requirement: Model Identity Types

The system MUST define `model.AgentID` (typed string), `model.SupportTier` (First, Extended, Community, Experimental, Retired), `model.SystemPromptStrategy` (6-value int enum), and `model.MCPStrategy` (5-value int enum: None, Stdio, HTTP, StreamableHTTP, Disabled). All MUST support string conversion and equality. AgentID MUST be usable as a map key.

#### Scenario: AgentID typed comparison

- GIVEN model.AgentID("opencode") and model.AgentID("claude")
- WHEN compared
- THEN they MUST NOT be equal
- AND each MUST work as a Registry map key

### Requirement: Discovery Returns All Agents

The system MUST return ALL installed agents from `Discover(ctx, homeDir)`, not just the first match. DiscoveryContext MUST include one entry per detected agent with its AgentID, binary path, config path, and resolved SupportTier.

#### Scenario: Multiple agents installed

- GIVEN a system with two installed agents
- WHEN Discover is called
- THEN it MUST return both agents in the result set

#### Scenario: No agents installed

- GIVEN a system with no agents
- WHEN Discover is called
- THEN it MUST return an empty slice without error

### Requirement: Capability Manifest System

The system MUST define `AgentCapabilityManifest` (struct embedding AgentFeatureClaims + AgentID + SupportTier) and `AgentFeatureClaims` (8 boolean fields: Reviews, GeneratesCode, AutoInstalls, Skills, SystemPrompt, MCP, OutputStyles, SlashCommands). A canonical `featureClaimsByAgent` map MUST contain exactly 16 entries indexed by model.AgentID. `ResolveCapabilityManifest(adapter)` MUST validate adapter claims against the canonical entry.

#### Scenario: Canonical map completeness

- GIVEN the featureClaimsByAgent map
- WHEN all entries are enumerated
- THEN exactly 16 entries MUST be present
- AND each AgentFeatureClaims field MUST be set per entry

#### Scenario: Manifest validation

- GIVEN an adapter with matching canonical claims
- WHEN ResolveCapabilityManifest is called
- THEN it MUST return a validated manifest without error

#### Scenario: Mismatched claims

- GIVEN an adapter whose claims differ from canonical
- WHEN ResolveCapabilityManifest is called
- THEN it MUST return an error describing the mismatch

## MODIFIED Requirements

### Requirement: Adapter Interface

The system MUST define an Adapter interface with methods: ID() model.AgentID, Name() string, Tier() SupportTier, Detect(ctx, homeDir) (bool, string, string, bool, error), InstallCommand(profile) ([][]string, error), Capabilities() []string, SupportsAutoInstall() bool, SupportsSkills() bool, SupportsSystemPrompt() bool, SupportsMCP() bool, SupportsOutputStyles() bool, SupportsSlashCommands() bool, SupportsSubAgents() bool, GlobalConfigDir(homeDir) string, SystemPromptDir(homeDir) string, SystemPromptFile(homeDir) string, SkillsDir(homeDir) string, CommandsDir(homeDir) string, SubAgentsDir(homeDir) string, EmbeddedSubAgentsDir(homeDir) string, OutputStyleDir(homeDir) string, SettingsPath(homeDir) string, MCPConfigPath(homeDir, serverName) string, SystemPromptStrategy() SystemPromptStrategy, MCPStrategy() model.MCPStrategy, DeployConfig(ctx, cfg) error.
(Previously: ID() string, Detect(ctx) (string, error), MCPStrategy() string, 11 methods total)

#### Scenario: Happy path — full implementation

- GIVEN a concrete adapter implementing all methods
- WHEN each method is called
- THEN every method MUST return without panic
- AND guard methods (Supports*) MUST return bool without error

#### Scenario: Detect with new signature

- GIVEN an installed agent
- WHEN Detect(ctx, homeDir) is called
- THEN it MUST return (true, binaryPath, configPath, autoInstallCapable, nil)

#### Scenario: Detect returns not-found

- GIVEN an adapter for an agent not present
- WHEN Detect(ctx, homeDir) is called
- THEN it MUST return (false, "", "", false, error)
- AND the error MUST explain the agent was not found

### Requirement: Registry for Registration and Lookup

The system MUST define a Registry struct with Register(id model.AgentID, factory Factory) and ListAll() []CatalogEntry methods. Registering a duplicate ID MUST overwrite the previous factory. ListAll MUST return catalog entries built from the registered adapters.
(Previously: Register(name string, factory Factory))

#### Scenario: Happy path — register and list

- GIVEN an empty Registry
- WHEN a factory is registered under model.AgentID("opencode")
- THEN ListAll() MUST return exactly 1 entry
- AND the entry's ID MUST be model.AgentID("opencode")

#### Scenario: Duplicate registration

- GIVEN a Registry with a registered adapter "opencode"
- WHEN Register is called with the same model.AgentID but a different factory
- THEN the old factory MUST be replaced
- AND ListAll() MUST still return exactly 1 entry
