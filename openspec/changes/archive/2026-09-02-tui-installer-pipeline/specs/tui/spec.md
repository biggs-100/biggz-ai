# Delta for tui

## Purpose

Extend TUI with `internal/tui/screens/installing.go` progress screen replacing running spinner. Provides 30-char bar `█`/`░`, lossless progress channel, and animation guards.

## ADDED Requirements

### Requirement: REQ-TUI-PIPE-001 — Installing Screen Progress Bar (30 chars █/░)

The system MUST provide `installing.go` screen with `InstallingModel` rendering a progress bar exactly 30 chars wide using `█` for filled and `░` for empty. Bar fill MUST be `Percent *30/100` rounded. The system MUST stream `ProgressEvent` via `tea.Msg` and render `Step` name and `Message`. The system MUST use `lipgloss` tokens from `internal/tui/styles`.

#### Scenario: Bar 0% empty

- GIVEN `Percent==0`
- WHEN `View()` renders at any width >=40
- THEN bar MUST be `░` repeated 30 times

#### Scenario: Bar 50% half filled

- GIVEN `Percent==50`
- WHEN `View()` renders
- THEN bar MUST be 15 `█` + 15 `░` (30 total)

#### Scenario: Bar 100% full

- GIVEN `Percent==100`
- WHEN `View()` renders
- THEN bar MUST be 30 `█` and show completion state

#### Scenario: Step name displayed

- GIVEN event `Step=="deploy-skills" Message=="copying..."`
- WHEN model updates
- THEN `View()` MUST contain step name and message

### Requirement: REQ-TUI-PIPE-002 — Lossless Progress Channel Integration

The system MUST consume `ProgressChan chan ProgressEvent` via a `tea.Cmd` that forwards every event without drop. The channel MUST be buffered (cap >=16) and the TUI MUST NOT block `Apply`. On channel close the model MUST transition to Done/Fail state. Progress MUST be monotonic 0→100 per Orchestrator contract.

#### Scenario: Events forwarded without drop

- GIVEN channel with 10 events 0..100
- WHEN TUI `Update` handles `ProgressEvent` msgs
- THEN model `Percent` MUST equal last event and count processed MUST be 10

#### Scenario: Channel close transitions to Done

- GIVEN `Percent==100` and channel closed
- WHEN final `ProgressEvent` processed
- THEN model state MUST be `Done` and `View()` MUST show success

#### Scenario: Failure event shows error

- GIVEN `ProgressEvent` with error or orchestrator `ExecutionResult.Success==false`
- WHEN model handles it
- THEN `View()` MUST show failed state without panic

### Requirement: REQ-TUI-PIPE-003 — isSyncSupported and Animation Guards

The system MUST implement `isSyncSupported()` detecting `TERM` support for CSI 2026 and MUST honor `BIGGZ_NO_ANIMATION=1`, `GENTLE_AI_NO_ANIMATION=1`, `BIGGZ_PRETTY=0`, and `TERM=dumb`. When disabled, `installing.go` MUST NOT emit `ESC[?2026h/l`, `tickCmd` MUST return nil, and spinner MUST freeze. The TUI MUST call `Orchestrator` via `tea.Cmd` and MUST remain responsive during Apply.

#### Scenario: Sync supported emits CSI

- GIVEN `TERM=xterm-256color` and no disable env
- WHEN `isSyncSupported()==true` and `View()` renders
- THEN output MAY be wrapped `ESC[?2026h`/`ESC[?2026l` as per base spec

#### Scenario: BIGGZ_NO_ANIMATION disables animation

- GIVEN `BIGGZ_NO_ANIMATION=1`
- WHEN `tickCmd()` evaluated for installing screen
- THEN it MUST return nil and bar MUST update only on ProgressEvent
- AND no `ESC[?2026h/l` MUST appear

#### Scenario: Dumb terminal plain fallback

- GIVEN `TERM=dumb`
- WHEN installing screen renders
- THEN output MUST contain zero ANSI/CSI escapes and bar MUST be plain text

#### Scenario: Orchestrator via tea.Cmd non-blocking

- GIVEN `installing` screen starts `Orchestrator.Run` as `tea.Cmd`
- WHEN Apply is long-running
- THEN UI MUST remain responsive and progress MUST stream incrementally
