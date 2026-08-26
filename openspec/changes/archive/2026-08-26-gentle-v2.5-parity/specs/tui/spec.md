# Delta for tui

## ADDED Requirements

### Requirement: Reduced-Motion and Gentleman-Cute Refresh

The system MUST support reduced-motion and the Gentleman-Cute palette refresh. When `GENTLE_AI_NO_ANIMATION=1` or `BIGGZ_NO_ANIMATION=1` or `TERM=dumb` is set, the TUI MUST disable spinner ticks and synchronized-output wrappers (`ESC[?2026h/l`) and render without animation. The Rose Pine / Gentleman-Cute palette MUST be the single source of truth in `internal/tui/styles/styles.go`. No other palette pinning in goldens or screens is allowed to diverge.

#### Scenario: Reduced-motion disables animation

- GIVEN `GENTLE_AI_NO_ANIMATION=1`
- WHEN `tuiAnimationsDisabled()` and `tickCmd()` are evaluated
- THEN animations MUST be disabled and `tickCmd` MUST return `nil`

#### Scenario: Synchronized output respects disable flag

- GIVEN `BIGGZ_NO_ANIMATION=1` or `TERM=dumb`
- WHEN `Render()` produces a frame
- THEN output MUST NOT be wrapped with `ESC[?2026h` / `ESC[?2026l`

#### Scenario: Animated terminal allows sync output

- GIVEN terminal supports `CSI 2026` and no disable env is set
- WHEN `Render()` produces a frame
- THEN frame MUST be wrapped with `ESC[?2026h` begin and `ESC[?2026l` end atomically

#### Scenario: Palette single source of truth

- GIVEN `internal/tui/styles/styles.go` defines `ColorBase`, `ColorLavender`, `ColorGreen`, etc.
- WHEN styles are inspected
- THEN values MUST match Rose Pine Gentleman-Cute definitions and no second palette definition MUST exist
