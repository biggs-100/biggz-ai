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

The system MUST define an AgentAdapter interface with ID() returning string, Name() returning string, Detect(ctx) returning (binaryPath string, err), Capabilities() returning []Capability, and DeployConfig(ctx, cfg) returning error.

#### Scenario: Happy path — agent detected

- GIVEN an AgentAdapter for an agent that is installed on the system
- WHEN Detect is called
- THEN it MUST return the binary path without error

#### Scenario: Agent not installed

- GIVEN an AgentAdapter for an agent that is NOT installed
- WHEN Detect is called
- THEN it MUST return an error indicating the agent was not found

### Requirement: Build-Time Registry

The system MUST provide a Registry with RegisterLens(plugin), RegisterAgent(adapter), GetLens(id), and GetAgent(id) methods. Registration MUST happen at build time via explicit wiring. The Registry MUST NOT support dynamic loading.

#### Scenario: Happy path — register and retrieve agent

- GIVEN an empty Registry
- WHEN an AgentAdapter with ID "test-agent" is registered via RegisterAgent
- THEN GetAgent("test-agent") MUST return the registered adapter
- AND GetAgent("unknown") MUST return nil

#### Scenario: Duplicate agent registration

- GIVEN a Registry with a registered AgentAdapter "test-agent"
- WHEN RegisterAgent is called again with the same ID "test-agent"
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
