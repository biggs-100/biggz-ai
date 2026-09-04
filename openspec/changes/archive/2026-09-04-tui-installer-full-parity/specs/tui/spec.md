# Delta for tui

## ADDED Requirements

### Requirement: REQ-WIZ-001 — Wizard stage traversal

The system MUST provide a linear installer wizard traversing Welcome → Detection → Agents → Persona → Preset → DependencyTree → SkillPicker → Review → Installing → Complete. Each stage MUST advance on confirm, retreat on back-key, and MUST be fully keyboard-operable.

#### Scenario: Forward traversal
- GIVEN wizard at stage N < Complete
- WHEN user confirms valid input
- THEN system MUST advance to stage N+1 preserving prior selections

#### Scenario: Backward navigation
- GIVEN wizard at stage N > Welcome with selections made
- WHEN user presses back-key
- THEN system MUST return to stage N-1 with selections intact

#### Scenario: Keyboard-only completion
- GIVEN `BIGGZ_NO_ANIMATION=1`
- WHEN user drives all 10 stages via keyboard only
- THEN wizard MUST reach Complete without pointer input

### Requirement: REQ-WIZ-002 — Per-agent pickers adapted

The system MUST provide per-agent model pickers for Claude/Codex/Kiro/OpenCode+Pi backgrounds. Pickers MUST use `internal/tui/styles` tokens only and MUST persist via existing model-routing precedence.

#### Scenario: Picker selection persists
- GIVEN Agents stage with agent selected
- WHEN user picks a model background
- THEN effective model MUST resolve per agents > user > builtin precedence

#### Scenario: Biggz styling only
- GIVEN any picker view renders
- WHEN output inspected
- THEN it MUST contain zero gentle-ai palette tokens

### Requirement: REQ-WIZ-003 — Router linearRoutes

The system MUST define `linearRoutes` in `internal/tui/router.go` fixing the 10-stage order. The router MUST reject out-of-order jumps and MUST extend `install.go` state machine without breaking `Idle→Detect→Select→Review→Running→Done` fallback.

#### Scenario: Ordered routing
- GIVEN wizard at Detection
- WHEN router resolves next
- THEN target MUST be Agents multi-select

#### Scenario: Legacy fallback
- GIVEN `BIGGZ_LEGACY_INSTALL=1`
- WHEN installer starts
- THEN state machine MUST use the lean 6-state flow

### Requirement: REQ-WIZ-004 — Reduced-motion compliance

New wizard views MUST honor `BIGGZ_NO_ANIMATION=1`, `GENTLE_AI_NO_ANIMATION=1`, `TERM=dumb`: spinners frozen, `tickCmd` nil, zero `ESC[?2026h/l` wrappers, zero ANSI under dumb.

#### Scenario: Static under no-animation
- GIVEN `BIGGZ_NO_ANIMATION=1` on Installing stage
- WHEN view renders
- THEN output MUST contain no spinner frames and no CSI 2026

#### Scenario: Dumb terminal plain
- GIVEN `TERM=dumb` on any wizard stage
- WHEN view renders
- THEN output MUST contain zero ANSI escapes

### Requirement: REQ-WIZ-005 — Zero banner references

Ported wizard code MUST NOT reference `RenderLogo`, `Tagline`, `updateBanner`, or advisory banners. Review MUST fail on any match.

#### Scenario: Banner grep clean
- GIVEN ported files under `internal/tui/screens/`
- WHEN searched for `RenderLogo|Tagline|updateBanner|advisory`
- THEN match count MUST be zero
