# Delta for Plugin System

## ADDED Requirements

### Requirement: AgentAdapter Shim via ExtensionAPI

The system MUST provide `internal/extension/shim.go` where `AgentAdapter` delegates to `ExtensionAPI` as a provider. Hooks and custom-tool registrations MUST map to `ExtensionAPI.RegisterTool`; adapter methods MUST forward to the underlying `ExtensionAPI` instance. The shim type and its `AgentAdapter` alias MUST be annotated `// Deprecated: use ExtensionAPI` and `LensPlugin` MUST NOT be reintroduced. `install.Deploy` MUST use `ExtensionAPI` for registration.

#### Scenario: Shim delegates RegisterTool

- GIVEN a shimmed `AgentAdapter` backed by `FakeExtensionAPI`
- WHEN adapter registers a hook/custom tool
- THEN `FakeExtensionAPI` MUST record a `RegisterTool` call with the same def

#### Scenario: Deprecated annotation present

- GIVEN `internal/extension/shim.go`
- WHEN inspected for `type AgentAdapter` or shim type
- THEN it MUST contain `// Deprecated: use ExtensionAPI`

#### Scenario: LensPlugin not reintroduced

- GIVEN codebase after change
- WHEN searching for `type LensPlugin`
- THEN zero definitions MUST be found

## MODIFIED Requirements

### Requirement: LensPlugin Absence Invariant

The system MUST NOT reintroduce `plugin.LensPlugin`, `internal/lens/*`, or embedded static-analysis engine. `Lens` MUST remain in `internal/review/lens/types.go`, not `plugin/`. `AgentAdapter` shim delegation via `ExtensionAPI` MUST be the only compatibility layer; it MUST NOT re-expose `LensPlugin` types or interfaces. Any PR reintroducing `LensPlugin` MUST fail `gofmt`/`go test` validation via missing import guard.
(Previously: forbade LensPlugin without mentioning shim as sole compat layer)

#### Scenario: LensPlugin stays absent

- GIVEN codebase after change
- WHEN searching for `type LensPlugin`
- THEN zero definitions MUST be found
- AND `plugin/interfaces.go` MUST not import `internal/review/lens`

#### Scenario: Legacy path absent

- GIVEN filesystem check
- WHEN `internal/lens/` directory is queried
- THEN it MUST not exist (lenses live under `internal/review/lens/`)

#### Scenario: Shim is sole compat layer

- GIVEN `internal/extension/shim.go` exists
- WHEN `plugin/` is inspected for `LensPlugin` or `Lens` types
- THEN zero such types MUST be found outside the deprecated shim alias
