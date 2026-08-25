# Delta for plugin-system

## ADDED Requirements

### Requirement: LensPlugin Absence Invariant

The system MUST NOT reintroduce `plugin.LensPlugin`, `internal/lens/*`, or embedded static-analysis engine. `Lens` MUST remain in `internal/review/lens/types.go`, not `plugin/`. Any PR reintroducing `LensPlugin` MUST fail `gofmt`/`go test` validation via missing import guard.

#### Scenario: LensPlugin stays absent

- GIVEN codebase after change
- WHEN searching for `type LensPlugin`
- THEN zero definitions MUST be found
- AND `plugin/interfaces.go` MUST not import `internal/review/lens`

#### Scenario: Legacy path absent

- GIVEN filesystem check
- WHEN `internal/lens/` directory is queried
- THEN it MUST not exist (lenses live under `internal/review/lens/`)

### Requirement: ExternalLensAdapter Bridge

The system MUST provide `ExternalLensAdapter` in `internal/review/lens/external/adapter.go` implementing `Lens` by delegating to `biggz review capture-result` JSON. It MUST translate `LensResultHash` with prefix `gentle-ai.lens-result/v1` without changing `capture.go`/`ledger.go` schema. Build-time registry wiring lives in `cmd/biggz` init.

#### Scenario: Bridge preserves hash contract

- GIVEN a capture-result JSON with `gentle-ai.lens-result/v1` hash
- WHEN adapter returns `LensResult`
- THEN hash prefix MUST be preserved
- AND downstream `capture`/`ledger` MUST accept it without schema change

#### Scenario: No plugin DAG

- GIVEN adapter registration
- WHEN pipeline executes with external lens
- THEN execution MUST remain sequential `pipeline.Stage` ordered by `PlanLenses`
- AND no DAG scheduler MUST be invoked

## MODIFIED Requirements

### Requirement: AgentAdapter Interface

The system MUST define an AgentAdapter interface with these methods: ID() model.AgentID, Name() string, Tier() SupportTier, Detect(ctx, homeDir) (bool, string, string, bool, error), InstallCommand(profile) ([][]string, error), Capabilities() []string, SupportsAutoInstall() bool, SupportsSkills() bool, SupportsSystemPrompt() bool, SupportsMCP() bool, SupportsOutputStyles() bool, SupportsSlashCommands() bool, SupportsSubAgents() bool, GlobalConfigDir(homeDir) string, SystemPromptDir(homeDir) string, SystemPromptFile(homeDir) string, SkillsDir(homeDir) string, CommandsDir(homeDir) string, SubAgentsDir(homeDir) string, EmbeddedSubAgentsDir(homeDir) string, OutputStyleDir(homeDir) string, SettingsPath(homeDir) string, MCPConfigPath(homeDir, serverName) string, SystemPromptStrategy() SystemPromptStrategy, MCPStrategy() model.MCPStrategy, DeployConfig(ctx, cfg) error.
(Previously: 11 methods with ID() string, Detect(ctx) (binaryPath, err), MCPStrategy() string — legacy LensPlugin existence assumed)

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

#### Scenario: Lens seam not in plugin

- GIVEN `plugin/interfaces.go`
- WHEN inspected for `Lens` or `LensPlugin`
- THEN it MUST contain zero lens types
- AND `internal/review/lens/types.go` MUST be sole `Lens` owner
