# Delta for Review Lenses

## ADDED Requirements

### Requirement: Readability Registration via ExtensionAPI

The system MUST register the `readability` lens via `ExtensionAPI.RegisterLens`; `Lens` interface (`ID() string`, `Analyze(ctx,LensInput)(LensResult,error)` in `internal/review/lens/types.go`) MUST remain unchanged and `Lens.Analyze` MUST stay pure with no `ExtensionAPI` dependency. Registration wiring MUST move from build-time `registry.go` direct map to `ExtensionAPI`; `LensInput` MUST still derive from `DeriveRiskInput`. Only `readability` is migrated; other lenses remain deferred.

#### Scenario: Readability registered through ExtensionAPI

- GIVEN `ExtensionAPI` with no lenses
- WHEN `RegisterLens(readability.Lens{})` is called
- THEN `Ordered(["readability"])` MUST return the lens and `Analyze` MUST execute pure

#### Scenario: Lens.Analyze stays pure

- GIVEN `readability.Lens` after migration
- WHEN `Analyze` is called with `LensInput` containing `DiffSummary[path]>400`
- THEN it MUST emit the same inferential finding without importing `internal/extension`

#### Scenario: Single lens migrated

- GIVEN codebase after change
- WHEN counting `RegisterLens` calls
- THEN exactly one call for `readability` MUST exist and no other lens MUST use `ExtensionAPI`
