# Agent Registry Specification

## Purpose

The Agent Registry provides a factory pattern for creating and registering agent adapters. It defines the `Adapter` interface (enriched from gentle-ai), a `Factory` function type, a `Registry` for registration and lookup, and three concrete adapter implementations: OpenCode, Claude, and Qwen.

## Requirements

### Requirement: Adapter Interface

The system MUST define an `Adapter` interface with methods: ID() string, Name() string, Detect(ctx) (string, error), Capabilities() []string, SupportsAutoInstall() bool, GlobalConfigDir(homeDir string) string, SkillsDir(homeDir string) string, SettingsPath(homeDir string) string, MCPStrategy() string, and DeployConfig(ctx, cfg) error.

#### Scenario: Happy path — adapter implements all methods

- GIVEN a concrete type that implements Adapter
- WHEN each method is called
- THEN every method MUST return without panic
- AND Capabilities() MUST return a non-nil slice

#### Scenario: Detect returns error for uninstalled agent

- GIVEN an Adapter for an agent not present on the system
- WHEN Detect is called
- THEN it MUST return an empty string and a non-nil error
- AND the error MUST explain the agent was not found

### Requirement: Factory Function Type

The system MUST define a `Factory` function type: `func() Adapter`. The factory MUST NOT take parameters — all configuration MUST be embedded in the closure or the adapter constructor.

#### Scenario: Happy path — factory produces adapter

- GIVEN a Factory that constructs an OpenCode adapter
- WHEN the factory is invoked
- THEN the returned value MUST satisfy the Adapter interface
- AND Name() MUST return "opencode"

### Requirement: Registry for Registration and Lookup

The system MUST define a `Registry` struct with `Register(name string, factory Factory)` and `ListAll() []CatalogEntry` methods. Registering a duplicate name MUST overwrite the previous factory. ListAll MUST return catalog entries built from the registered adapters.

#### Scenario: Happy path — register and list

- GIVEN an empty Registry
- WHEN a factory is registered under "opencode"
- THEN ListAll() MUST return exactly 1 entry
- AND the entry's ID MUST be "opencode"

#### Scenario: Duplicate registration

- GIVEN a Registry with a registered adapter "opencode"
- WHEN Register is called again with the same name but a different factory
- THEN the old factory MUST be replaced
- AND ListAll() MUST still return exactly 1 entry for "opencode"

### Requirement: Three Concrete Adapters

The system MUST provide three concrete adapters: OpenCodeAdapter, ClaudeAdapter, and QwenAdapter. Each MUST implement the full Adapter interface. Each MUST be registered in the default Registry via an `init()` function or explicit wiring.

#### Scenario: Happy path — all adapters present

- GIVEN the default Registry (populated by init)
- WHEN ListAll() is called
- THEN it MUST return exactly 3 entries
- AND each entry MUST map to one of the three adapters

#### Scenario: Adapter-specific behavior

- GIVEN the QwenAdapter
- WHEN SupportsAutoInstall() is called
- THEN the result MUST reflect Qwen's actual auto-install capability
- AND MCPStrategy() MUST return a strategy value meaningful to Qwen's architecture
