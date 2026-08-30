# Delta for tui

## ADDED Requirements

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
