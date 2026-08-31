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

### Requirement: Screen Registration and RunWithScreen

The TUI MUST register `screenHelp` and `screenBackup` (Helps+Backup models) in `internal/tui/tui.go` screen constants and router, and expose `RunWithScreen(id int)` that launches the program at the given screen with `tea.WithAltScreen`.

#### Scenario: RunWithScreen opens Help

- GIVEN `tui.RunWithScreen(screenHelp)` is called from CLI `--tui` path
- WHEN program initializes
- THEN `Model.currentScreen` MUST equal `screenHelp` and first `View()` MUST be Help model output

#### Scenario: RunWithScreen opens Backup

- GIVEN `tui.RunWithScreen(screenBackup)` is called
- WHEN program initializes
- THEN current screen MUST be `screenBackup` and `BackupModel.Init()` listing MUST be scheduled

#### Scenario: Unknown screen falls back

- GIVEN `RunWithScreen` receives unknown id
- WHEN routing resolves
- THEN model MUST default to `screenDashboard` without panic

### Requirement: Dashboard Tiles and Navigation

The dashboard MUST display navigable tiles/actions for Help and Backup alongside existing actions, with `?` toggling help overlay and `ENTER` navigating via `NavigateMsg`.

#### Scenario: Dashboard shows tiles

- GIVEN dashboard `View()` renders
- WHEN actions enumerate
- THEN output MUST contain tiles `Help` and `Backup & Restore` with keys and cursor indicator `▸`

#### Scenario: Tile navigation

- GIVEN dashboard cursor on Help tile
- WHEN user presses `enter`
- THEN model MUST emit `NavigateMsg{Screen: screenHelp}` and switch screen

#### Scenario: ? opens help overlay on any screen

- GIVEN current screen is Help or Backup
- WHEN user presses `?`
- THEN `showHelp` MUST toggle and `HelpOverlay(screenID)` MUST render on next `View()`

### Requirement: Shared Table Styles and Business Logic Isolation

The TUI MUST provide shared table and preview styles in `internal/tui/styles` (lipgloss table, preview pane, modal) reused by Help and Backup, and MUST NOT duplicate `internal/backup` storage logic.

#### Scenario: Shared styles single source

- GIVEN `internal/tui/styles/styles.go` defines table header/row selected styles
- WHEN Help or Backup renders a `table.Model`
- THEN bubble table styles MUST derive from those shared definitions with no per-screen palette duplication

#### Scenario: Backup screen reuses internal/backup only

- GIVEN Backup screen triggers list/create/restore
- WHEN implementation is inspected
- THEN code MUST call `backup.List`/`backup.Create`/`backup.Restore` via `tea.Cmd` and MUST contain zero direct `tar`/`gzip` or backup file logic

### Requirement: Animation and SyncOutput Honoring BIGGZ_NO_ANIMATION

The TUI MUST honor `BIGGZ_NO_ANIMATION=1`, `GENTLE_AI_NO_ANIMATION=1`, and `TERM=dumb` by disabling `tickCmd` and `syncOutput` wrappers (`ESC[?2026h/l`) across all new screens, and existing screens MUST not regress.

#### Scenario: Tick disabled when env set

- GIVEN `BIGGZ_NO_ANIMATION=1`
- WHEN `tickCmd()` is evaluated for help or backup spinners
- THEN result MUST be `nil`

#### Scenario: SyncOutput wraps only when supported

- GIVEN `TERM=xterm-256color` without disable env
- WHEN Help/Backup `View()` returns via `syncOutput`
- THEN frame MUST be wrapped with `ESC[?2026h` begin and `ESC[?2026l` end

#### Scenario: No animation wrapper in dumb terminal

- GIVEN `TERM=dumb`
- WHEN any screen renders
- THEN output MUST NOT contain `ESC[?2026h`/`ESC[?2026l`

### Requirement: Testing via teatest and Goldens

The new screens MUST be covered by `teatest` interactive tests plus golden files, exercising search, navigation, and restore confirmation, with `TERM=dumb` and isolated temp dirs to prevent flakes.

#### Scenario: Help teatest

- GIVEN `teatest.NewTestModel` with `HelpModel` and `TERM=dumb`
- WHEN input simulation types filter and presses `ESC`
- THEN model messages MUST produce expected golden and no artifact leaks to home dir

#### Scenario: Backup teatest confirm

- GIVEN `teatest` Backup model with temp `backupDir` containing one `backup.List` entry
- WHEN flow navigates, opens restore modal, presses `y` then `n` in separate sub-tests
- THEN `backup.Restore` MUST be invoked only on `y` path and both renders MUST match goldens

### Requirement: POLISH-TUI-01 — Two-Line Row Layout

Rows MUST be exactly 2 lines: L1 `glyph agent model·state` + fixed `elapsed` 5c + `tokens` 10c; L2 dim `tool path`/`activity` (truncate L2 only). Height MUST be 2.

#### Scenario: Standard row
- GIVEN tool `edit src/foo.go`, activity `compiling`
- WHEN rendered at 100c
- THEN L1 MUST have glyph+model·state+elapsed+tokens, L2 dim path/activity

#### Scenario: Right intact on narrow
- GIVEN 80c long model name
- WHEN rendered
- THEN L1-left/L2 MAY truncate with `…`, right cols MUST NOT

### Requirement: POLISH-TUI-02 — Workflow Two-Line Hierarchy

Workflow rows MUST be 2 lines: L1 `name·state`, L2 dim `gate/next/output` + failure action inline; nested MUST show `│` dim.

#### Scenario: Failure inline
- GIVEN `gate=verify, failure="test FAIL — rerun"`
- WHEN rendered
- THEN L2 MUST show dim gate/next/output, failure action on nested `│` line

#### Scenario: Nested guide dim
- GIVEN nested child
- WHEN rendered
- THEN prefix MUST be `│` dim, not solid

### Requirement: POLISH-TUI-03 — Collapsed Header Two Groups

Collapsed header MUST show 2 groups via `·`: g1 `X running·Y queued·cap U/L·pane ⚠`, g2 `elapsed·tok`; g1 muted g2 dim; ≤2 numerics +1 hint, no row >2 inline points.

#### Scenario: Two groups rendered
- GIVEN 2 running,1 queued,cap 4/8,pane warn,12s,3k
- WHEN header renders
- THEN MUST be `2 running·1 queued·cap 4/8·pane ⚠ · 12s·3k` muted/dim split

#### Scenario: No overflow
- GIVEN many metrics
- WHEN header renders
- THEN MUST NOT emit >2 numerics +1 hint per group

### Requirement: POLISH-TUI-04 — Collapsible Panes Section

Panes MUST be separate collapsible section `── panes ──`, never intermixed with jobs; toggle collapsed/expanded.

#### Scenario: Panes isolated
- GIVEN panes + jobs
- WHEN rendered
- THEN panes MUST be under `── panes ──` distinct from jobs

#### Scenario: Collapsed hides rows
- GIVEN collapsed
- WHEN rendered
- THEN only header visible, zero pane rows

### Requirement: POLISH-TUI-05 — Tail Visibility for Workflow Rows

When `N > visibleLimit`, system MUST hide tail, preserve order, never prepend `… hidden`; append `… +N hidden` at end.

#### Scenario: Tail hidden
- GIVEN 10 workflows limit 6
- WHEN rendered
- THEN first 6 in order visible, last `… +4 hidden`

#### Scenario: Order preserved
- GIVEN `[a,b,c,d]` limit 2
- WHEN rendered
- THEN visible `[a,b]` + `… +2 hidden`

### Requirement: POLISH-TUI-06 — Unified State Color Encoding

Glyph+text `running/queued/complete/failed` MUST share same solid tone; `elapsed` dim, `tokens` muted; MUST NOT double-encode.

#### Scenario: Running single tone
- GIVEN `running`
- WHEN rendered
- THEN glyph+text share solid, elapsed dim tokens muted

#### Scenario: Failed single tone
- GIVEN `failed`
- WHEN rendered
- THEN share solid failed color, no extra border

### Requirement: POLISH-TUI-07 — Layout Stability on Tick

On 500ms tick, if `Δ elapsed <1s` and `tokens Δ<100`, layout MUST NOT jitter; column widths stable.

#### Scenario: Sub-second no shift
- GIVEN `12s`, tokens stable, Δ 0.5s
- WHEN tick renders
- THEN `visibleWidth` unchanged, no jump

#### Scenario: Small token delta stable
- GIVEN tokens Δ 50, Δ 0.2s
- WHEN rendered
- THEN tokens 10c right-aligned stable
