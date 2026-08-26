# Component Catalog Specification

## Purpose

The Component Catalog provides a metadata-driven registry of discoverable agents, components, and skills. It serves as the single source of truth for "what exists in the system" — used by the planner, agent registry, and any future TUI or CLI commands. All entries are hardcoded Go slices, not dynamically loaded.

## Requirements

### Requirement: Catalog Entry Type

The system MUST define a `CatalogEntry` struct with fields: ID (string), Name (string), Description (string), and Tier (string). The system MUST define a `SkillEntry` struct that extends `CatalogEntry` with Platforms ([]string) and DependsOn ([]string). The system MUST define a `ComponentEntry` struct that extends `CatalogEntry` with Dependencies ([]string).

#### Scenario: Happy path — entry construction

- GIVEN a `CatalogEntry` defined with all four fields populated
- WHEN the entry is created
- THEN all fields MUST be accessible by name
- AND the entry MUST NOT require any runtime initialization

#### Scenario: Minimal entry

- GIVEN a `CatalogEntry` with only ID and Name populated
- WHEN the entry is used
- THEN Description and Tier MAY be zero-valued (empty string)
- AND the system MUST NOT panic or error on zero-valued fields

### Requirement: AllAgents Returns Agent Catalog

The system MUST export an `AllAgents()` function returning `[]CatalogEntry`. The returned slice MUST include entries for the three supported agents: OpenCode, Claude, Qwen. Each entry MUST include a descriptive Tier value (e.g., "core", "extended", "community").

#### Scenario: Happy path — all agents returned

- GIVEN the catalog package is imported
- WHEN AllAgents() is called
- THEN it MUST return exactly 3 entries
- AND each entry MUST have a non-empty ID, Name, Description, and Tier

#### Scenario: Slice immutability

- GIVEN the slice returned by AllAgents()
- WHEN a caller modifies a returned entry's field
- THEN the modification MUST NOT affect subsequent calls to AllAgents()
- AND the original backing array MUST remain unmodified

### Requirement: AllComponents Returns Component Catalog

The system MUST export an `AllComponents()` function returning `[]ComponentEntry`. The returned slice MUST include entries for the three components: skills, config, and prompts. Each entry MUST include a Dependencies field referencing the other components it depends on.

#### Scenario: Happy path — all components returned

- GIVEN the catalog package is imported
- WHEN AllComponents() is called
- THEN it MUST return exactly 3 entries
- AND each entry MUST have non-empty fields

#### Scenario: Dependency references

- GIVEN an AllComponents() entry for "skills"
- WHEN its Dependencies field is inspected
- THEN it MUST reference zero or more other component IDs that exist in the catalog

### Requirement: AllSkills Returns Skill Catalog

The system MUST export an `AllSkills()` function returning `[]SkillEntry`. Each skill entry MUST include platforms and dependency references. Skills MUST be organized by tier.

#### Scenario: Happy path — skills returned by tier

- GIVEN the catalog package is imported
- WHEN AllSkills() is called
- THEN it MUST return at least one entry per tier
- AND each entry MUST include Platforms and DependsOn

#### Scenario: Empty platform list

- GIVEN a skill that works on all platforms
- WHEN its Platforms field is inspected
- THEN Platforms MAY be an empty slice (meaning universal)
- AND the skill MUST still be returned by AllSkills()
