# Delta for tui

## ADDED Requirements

### Requirement: Model Picker over 30 Files

The system MUST extend `internal/tui/models.go` with Bubbles picker listing 30 files across 4 thinking modes (`off/low/medium/high`+`inherit`), integrating with per-agent `model` routing modal, respecting `agents > user > builtin` precedence, without adding banner scope.

#### Scenario: Picker lists 30
- GIVEN picker invoked
- WHEN files enumerated
- THEN count MUST be 30 and each MUST be selectable

#### Scenario: Thinking mode selection
- GIVEN agent file selected in picker
- WHEN `thinking` set to `high`/`inherit`
- THEN persisted `~/.biggz/models.json` MUST reflect choice and resolve correctly

#### Scenario: Precedence preserved in picker
- GIVEN `agents` and `user` both define model for same agent
- WHEN picker resolves effective
- THEN `agents` MUST win over `user`, `user` over `builtin`
