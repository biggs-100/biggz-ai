# TUI Specification

## Purpose

The TUI domain provides terminal rendering and input handling for biggz-ai interactive workflows, including synchronized output for flicker-free rendering and bracketed paste handling for large paste events. Ported from oh-my-pi differential rendering and input improvements.

## Requirements

### Requirement: Synchronized Output Rendering (CSI 2026)

The system MUST wrap TUI frame renders in synchronized output sequences `ESC[?2026h` (begin) and `ESC[?2026l` (end) to present atomic updates and eliminate flicker. The system MUST auto-detect terminal capability: it MUST enable sync only when `TERM` indicates support and MUST fall back to plain render when `BIGGZ_NO_ANIMATION=1` or `GENTLE_AI_NO_ANIMATION=1` is set or when `TERM=dumb` or equivalent non-supporting value is detected. The system SHOULD expose a `syncOutput` helper in `internal/tui/tui.go` that screens opt into for differential renders.

#### Scenario: Atomic render with sync markers

- GIVEN terminal supports CSI 2026 and no animation-disable env is set
- WHEN `Render()` produces a frame
- THEN output MUST be prefixed with `ESC[?2026h` and suffixed with `ESC[?2026l`
- AND the frame MUST appear atomically without intermediate tearing

#### Scenario: Fallback when unsupported or disabled

- GIVEN `BIGGZ_NO_ANIMATION=1` or `TERM=dumb`
- WHEN `Render()` is called
- THEN the system MUST emit the frame without CSI 2026 wrappers
- AND rendering MUST not produce garbled escape sequences

#### Scenario: Screen opt-in

- GIVEN a screen with prior flicker observed
- WHEN its view is rendered via `syncOutput`
- THEN differential updates MUST be buffered within the sync window

### Requirement: Bracketed Paste Handling

The system MUST detect bracketed paste sequences `ESC[200~` (start) and `ESC[201~` (end) in input handling and MUST buffer all content between them into a single paste event. Pastes exceeding 10 lines MUST arrive as one event, not as individual keystrokes. The system MUST NOT interpret paste content as commands.

#### Scenario: Large paste as single event

- GIVEN bracketed paste mode enabled
- WHEN input `ESC[200~` + 15 lines + `ESC[201~` is received
- THEN the system MUST emit exactly one paste event containing all 15 lines

#### Scenario: Paste content not executed as keys

- GIVEN bracketed paste contains text resembling `ctrl+c` or `esc`
- WHEN paste is processed
- THEN content MUST be inserted as text and MUST NOT trigger quit or navigation

#### Scenario: Incomplete bracketed sequence

- GIVEN `ESC[200~` received without closing `ESC[201~` within input buffer limit
- WHEN timeout or next non-paste input arrives
- THEN the system MUST flush buffered content as a paste and reset state
