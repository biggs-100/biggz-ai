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

### Requirement: PRETTY-V2-TUI-01 — Throttled Sync Streaming (16ms / CSI 2026)

The system MUST throttle lens-viewport `syncOutput` in `internal/tui/tui.go` to at most 60fps via 16ms trailing-edge coalescing with flush-on-idle. Frames MUST be wrapped idempotently in `ESC[?2026h`/`ESC[?2026l` only when `isSyncSupported()` is true. The system MUST NOT emit CSI when `BIGGZ_PRETTY=0` or `BIGGZ_NO_ANIMATION=1` or `GENTLE_AI_NO_ANIMATION=1` or `TERM=dumb` or `PI_SUBAGENT_CHILD=1`. Each slice MUST be <400 lines and stacked-to-main revertible.

#### Scenario: Throttle coalesces burst
- GIVEN 3 lens updates at t0, t0+5ms, t0+10ms
- WHEN `syncOutput` throttler runs
- THEN system MUST emit one frame at t0+16ms with a single CSI pair and no tearing

#### Scenario: Guard disables sync
- GIVEN `BIGGZ_PRETTY=0` or `BIGGZ_NO_ANIMATION=1` or `TERM=dumb`
- WHEN frame renders
- THEN output MUST contain zero `ESC[?2026h/l` and MUST NOT garble

#### Scenario: Idempotent nesting
- GIVEN frame already inside `ESC[?2026h`
- WHEN nested `syncOutput` is invoked
- THEN system MUST NOT double-wrap and MUST close with exactly one `ESC[?2026l`

### Requirement: PRETTY-V2-TUI-02 — Tool Pills with Icon / Color / Spinner and +N Collapse

The system MUST render tool pills via `internal/tui/styles/styles.go` (lipgloss tokens) and `internal/assets/pi/biggz-tool-pills.js` with icon, color, and spinner per state (`running`/`queued`/`complete`/`failed`), plus syntax highlight. When pill count >3, the system MUST show first 3 and collapse remainder as `… +N hidden` in order-preserving, collapsible section. The system MUST honor `BIGGZ_PRETTY=0` plain-text fallback and `BIGGZ_NO_ANIMATION` disabling spinner.

#### Scenario: Collapse beyond 3
- GIVEN 5 tool pills `[a,b,c,d,e]`
- WHEN rendered at any width
- THEN output MUST show `a b c` plus `… +2 hidden` and MUST preserve order

#### Scenario: Spinner respects reduced-motion
- GIVEN pill state `running` and `BIGGZ_NO_ANIMATION=1`
- WHEN rendered
- THEN spinner MUST be static and ticks MUST not advance

#### Scenario: Kill-switch plain fallback
- GIVEN `BIGGZ_PRETTY=0`
- WHEN pills render
- THEN output MUST be plain text without lipgloss ANSI and icons MAY be ASCII

### Requirement: PRETTY-V2-TUI-03 — Responsive Inline Word Diff

The system MUST render inline diffs via `internal/tui/diff.go` with `sergi/go-diff`: when viewport width >100 it MUST split `old|new` side-by-side, else unified. Word-level highlights MUST mark added/removed tokens within lines. Diffs >1MB MUST be capped and fall back to line-level highlight without panic.

#### Scenario: Split above threshold
- GIVEN width 120 and changed file with 40-char lines
- WHEN diff renders
- THEN layout MUST be two columns `old | new` with word highlights

#### Scenario: Unified below threshold
- GIVEN width 80
- WHEN same diff renders
- THEN layout MUST be unified single column with inline word highlights

#### Scenario: Cap and fallback
- GIVEN diff payload 1.2MB or malformed hunk
- WHEN rendered
- THEN system MUST cap at 1MB and fall back to line highlight without error

### Requirement: PRETTY-V2-TUI-04 — Gallery Preview and Reduced-Motion / Dumb Guards

The system MUST generate `docs/gallery` previews via `scripts/gallery` at 80c and 100c matching live viewport wrapping/truncation. Gallery regeneration MUST be deterministic and <400 lines. The system MUST disable all animation when `BIGGZ_NO_ANIMATION=1` or `GENTLE_AI_NO_ANIMATION=1` or `TERM=dumb`: `tickCmd` returns nil, sync wrappers suppressed, spinners frozen. When `TERM=dumb`, the system MUST strip ANSI. `BIGGZ_PRETTY=0` MUST disable all pretty features.

#### Scenario: Gallery matches viewport at both widths
- GIVEN `scripts/gallery` executed
- WHEN 80c and 100c galleries compared to live `View()` at same widths
- THEN wrapping, truncation, and token counts MUST match

#### Scenario: Reduced-motion kills ticks and sync
- GIVEN `BIGGZ_NO_ANIMATION=1`
- WHEN `tickCmd()` and `syncOutput` evaluated
- THEN `tickCmd` MUST return nil and frames MUST lack `ESC[?2026h/l`

#### Scenario: Dumb terminal strips ANSI
- GIVEN `TERM=dumb`
- WHEN any pretty view renders
- THEN output MUST contain zero ANSI escapes and no spinner

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
