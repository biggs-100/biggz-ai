# Model Routing Specification

## Purpose

Per-agent model routing maps `agents → user > builtin` with thinking inheritance and envelope round-trip for TUI-driven model selection. Ported from gentle-ai `gentle-ai.ts` model routing.

## Requirements

### Requirement: Per-Agent Model Routing TUI with Thinking Inheritance

The system MUST provide `internal/opencode/models.go` and `internal/tui/models.go` implementing `THINKING_LEVELS=[off,low,medium,high,inherit]` with `IsValidThinkingLevel`/`normalizeThinking`/`normalizeModelID`/`NormalizeModelConfig`/`EffectiveThinking`, persist `~/.biggz/models.json` v1 map `agent→{model,thinking}`, read `~/.biggz/cache/model-variants.json` via `LoadVariants`/`EnrichWithVariants`, expose `biggz models` BubbleTea table mapping `agents → user > builtin`, emit envelope `biggz-ai.agent_model_routing v1` (`MODEL_EXPORT_KIND`/`MODEL_EXPORT_VERSION`) and update frontmatter `model:`/`thinking:` losslessly, and cover 30 agent files via `PickerAgentFiles`.
(Previously: `gentle-pi.agent_model_routing` kind, stub picker, no cache read, no normalized THINKING_LEVELS export)

#### Scenario: Modal precedence and persistence
- GIVEN user selects `sdd-design` model/thinking via `biggz models`
- WHEN `~/.biggz/models.json` is written and reloaded
- THEN file MUST contain v1 entry and resolution MUST be `agents > user > builtin`

#### Scenario: Thinking inherit resolution
- GIVEN agent `thinking=inherit` with global `high`
- WHEN routing resolves via `EffectiveThinking`
- THEN effective MUST be `high`; `off/low/medium/high` MUST apply verbatim

#### Scenario: Envelope round-trip
- GIVEN envelope `biggz-ai.agent_model_routing v1` marshaled via `MarshalModelEnvelope`
- WHEN `ParseModelEnvelope` and frontmatter `UpdateFrontmatterRouting` run
- THEN fields MUST round-trip lossless and frontmatter MUST preserve `description:`

#### Scenario: Picker coverage 30 files
- GIVEN `PickerAgentFiles()` invoked
- WHEN listing
- THEN it MUST return 30 unique files and `ConfigurableAgentPhases` MUST include orchestrator+SDD+JD+review

#### Scenario: Normalize filters invalid
- GIVEN raw `{"sdd-design":{"model":"bad model with spaces","thinking":"ultra"}}`
- WHEN `NormalizeModelConfig` runs
- THEN invalid entry MUST be dropped and valid `claude-sonnet-4/high` MUST remain

### Requirement: Model Variants Cache Parity

The system MUST read `~/.biggz/cache/model-variants.json` as sorted `Record<provider,Record<model,string[]>>` via `LoadVariants` (tmp→rename atomic write in `model-variants.ts`), merge via `EnrichWithVariants` exact then deterministic fallback by sorted `modelID`, tolerate missing/invalid cache via `LoadVariantsOrEmpty`/`LoadModelsOrEmpty`, and preserve dedup via `blob:sha256` style sorted keys.

#### Scenario: Cache enriches provider models
- GIVEN `~/.biggz/cache/model-variants.json` contains `{"anthropic":{"claude-sonnet-4":["low","high"]}}`
- WHEN `EnrichWithVariants` merges into `LoadModels` result
- THEN `Model.Variants` MUST equal `["high","low"]` sorted and `EffortLevels()` MUST return them

#### Scenario: Missing cache is empty not error
- GIVEN cache file absent
- WHEN `LoadVariantsOrEmpty` or `EnrichWithVariants` runs
- THEN result MUST be empty map and caller MUST skip effort picker without error

#### Scenario: Divergence handled deterministically
- GIVEN cache has provider `openai` model `gpt-5` not in catalog and catalog has `anthropic` without variants
- WHEN enrichment runs with sorted keys
- THEN unassigned models MUST be matched by `modelID` fallback deterministically and extra cache entries MUST be ignored

### Requirement: Export Restore and Walk-Test Validation

The system MUST expose `MarshalModelEnvelope`/`ParseModelEnvelope` validating `kind==MODEL_EXPORT_KIND` and `version==MODEL_EXPORT_VERSION`, and `WriteModelConfig`/`ReadModelConfig` with strict `NormalizeModelConfig` filtering unknown keys; `walk_test` style validation MUST assert sorted JSON output and frontmatter idempotence.

#### Scenario: Export restore with kind version
- GIVEN `AgentModelConfig{"sdd-design":{model:"claude-sonnet-4",thinking:"high"}}`
- WHEN marshaled and parsed
- THEN JSON MUST contain `kind="biggz-ai.agent_model_routing"` `version=1` and parsed config MUST equal original

#### Scenario: Invalid envelope rejected
- GIVEN envelope with `kind="bad.kind"` or `version=2`
- WHEN `ParseModelEnvelope` runs
- THEN it MUST return error and MUST NOT return partial config

#### Scenario: Walk-test sorted validation
- GIVEN `AgentModelConfig` with unsorted keys `{"z-agent":{},"a-agent":{}}`
- WHEN `WriteModelConfig` writes and file is read
- THEN JSON keys MUST be sorted and `UpdateFrontmatterRouting(nil)` MUST clear `model:`/`thinking:` idempotently
