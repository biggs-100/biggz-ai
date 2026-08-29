# Delta for model-routing

## ADDED Requirements

### Requirement: Per-Agent Model Routing TUI with Thinking Inheritance

The system MUST provide `internal/tui/models.go` Bubbles modal mapping `agents → user > builtin`, persist `~/.biggz/models.json` v1 `{"sdd-design":{"model":"claude-sonnet-4","thinking":"high"}}`, support per-agent `model`+`thinking(off/low/medium/high/inherit)`, emit envelope `gentle-pi.agent_model_routing v1` with `MODEL_EXPORT_KIND/VERSION` and frontmatter `1377-1381`, and expose picker over 30 files.

#### Scenario: Modal precedence and persistence
- GIVEN user selects `sdd-design` model/thinking via modal
- WHEN `~/.biggz/models.json` is written and reloaded
- THEN file MUST contain v1 entry and resolution MUST be `agents > user > builtin`

#### Scenario: Thinking modes
- GIVEN agent `thinking=inherit` with global `high`
- WHEN routing resolves
- THEN effective MUST be `high`; `off/low/medium/high` MUST apply verbatim

#### Scenario: Envelope round-trip
- GIVEN envelope `gentle-pi.agent_model_routing v1` written
- WHEN frontmatter parsed via `1334-1346` path
- THEN fields MUST round-trip lossless

#### Scenario: Picker coverage
- GIVEN picker invoked
- WHEN listing
- THEN it MUST cover 30 files across 4 modes without omission
