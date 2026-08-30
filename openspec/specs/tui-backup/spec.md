### Requirement: Backup Table Listing via internal/backup

The Backup TUI MUST list snapshots by wrapping `backup.List` in a `tea.Cmd`, rendering a sortable `table.Model` with columns ID, size, and date; it MUST NOT reimplement storage logic.

#### Scenario: Table populates from backup.List

- GIVEN `~/.biggz/backups` contains 2 manifests via `backup.List`
- WHEN model receives `backupListMsg` from the `tea.Cmd`
- THEN table rows MUST show each ID, formatted size, and `CreatedAt` date

#### Scenario: Cursor navigation

- GIVEN table with 3 rows and cursor at 0
- WHEN user presses `down`/`j`
- THEN cursor MUST move to 1 and selected row style MUST update

#### Scenario: Empty list message

- GIVEN `backup.List` returns zero entries
- WHEN `View()` renders
- THEN output MUST show `No backups found` and hint `Press [C] to create`

### Requirement: Preview Pane

The Backup TUI MUST show a preview pane for the selected row with ID, formatted size, formatted date, and backed-up paths; pane MUST update synchronously on cursor change.

#### Scenario: Preview updates on selection

- GIVEN table with IDs `backup-20260101-120000` and `backup-20260102-120000`
- WHEN cursor moves from row 0 to row 1
- THEN preview MUST display ID `backup-20260102-120000`, size, date, and paths of that backup

#### Scenario: Preview after create

- GIVEN a newly created backup via `backup.Create`
- WHEN success message arrives
- THEN list MUST refresh and preview MUST show the new entry

### Requirement: Create Backup Flow

The model MUST create snapshots via `backup.Create` wrapped in a `tea.Cmd`, handling mkdir, `Skipped` warnings, and result display without blocking UI.

#### Scenario: Create via tea.Cmd

- GIVEN user presses `c` from `backupListing` or `backupDone`
- WHEN model handles the key
- THEN it MUST set step `backupCreating` and return a `tea.Cmd` invoking `backup.Create`

#### Scenario: Create failure surfaces error

- GIVEN `backup.Create` returns `mkdir: permission denied`
- WHEN `backupResultMsg{err}` arrives
- THEN step MUST become `backupError` and `View()` MUST render error via `ErrorBox`

### Requirement: Restore with Double Confirmation

Restore MUST require a double-confirm modal: preview first, then `y/N` prompt; `backup.Restore` MUST be called only on explicit `y`/`Y` confirmation, otherwise no filesystem mutation.

#### Scenario: Restore requires confirmation

- GIVEN a selected row in `backupListing`
- WHEN user presses `enter` to restore
- THEN model MUST enter `backupRestoring` with confirm modal showing preview size/paths/date and prompt `y/N`

#### Scenario: Confirm calls backup.Restore

- GIVEN confirm modal visible
- WHEN user presses `y`
- THEN model MUST invoke `backup.Restore(backupDir, id, target)` via `tea.Cmd`

#### Scenario: Deny cancels without side effect

- GIVEN confirm modal visible
- WHEN user presses `n` or `ESC`
- THEN model MUST return to `backupListing`, MUST NOT call `backup.Restore`, and status MUST indicate cancelled

#### Scenario: Pre-restore safety snapshot

- GIVEN confirm modal accepted with `y`
- WHEN restore `tea.Cmd` begins
- THEN model SHOULD first invoke `backup.Create` to snapshot current state before `backup.Restore` writes target

#### Scenario: Narrow terminal collapses columns

- GIVEN terminal width 50
- WHEN table renders
- THEN columns MUST collapse or truncate with `…` and `VisibleWidth` MUST stay ≤ width

### Requirement: Animation Guard and Testing

The Backup TUI MUST honor `BIGGZ_NO_ANIMATION=1`/`TERM=dumb` via `syncOutput` and `tuiAnimationsDisabled`, and MUST be covered by `teatest` + goldens isolated to temp backup dirs.

#### Scenario: Animation disabled suppresses wrappers

- GIVEN `BIGGZ_NO_ANIMATION=1`
- WHEN Backup `View()` renders through `syncOutput`
- THEN output MUST NOT contain `ESC[?2026h`/`ESC[?2026l` and tick MUST return `nil`

#### Scenario: teatest covers nav and confirm

- GIVEN `teatest` with `TERM=dumb` and `t.TempDir()` as `backupDir`
- WHEN test navigates rows, triggers create, and confirms/denies restore
- THEN goldens MUST capture table, preview, and modal states deterministically
