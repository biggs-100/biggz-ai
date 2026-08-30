# Design: Help + Backup TUI (Full Bubbletea Port)

## Technical Approach

Port `gentle-pi` `help.go`+`backup.go` to `biggz-ai` as full Bubbletea models. Wrap `internal/backup` (`List`/`Create`/`Restore`) via `tea.Cmd`, add `bubbles/table`+`textinput`+`viewport` with shared `lipgloss` styles. Route `--tui` in `cli_misc.go` to `tui.RunWithScreen` (`tea.WithAltScreen`); dashboard tiles via `NavigateMsg`. Honor `BIGGZ_NO_ANIMATION`/`TERM=dumb` via `tuiAnimationsDisabled()`+`syncOutput`; reuse `helpData`/`GetHelp`.

## Architecture Decisions

| Decision | Options | Tradeoff | Choice |
|---|---|---|---|
| Help model structure | Static overlay vs full Bubbletea Model | Static is trivial but misses live filter/viewport spec | Full Model with `textinput`+`viewport` over `helpData`; filter is derived state, not copy |
| Backup I/O | Inline `os`/`tar` vs `internal/backup` wrapper | Inline duplicates tested logic | `tea.Cmd` wrapping `backup.List/Create/Restore`; model only maps `Backup -> backupEntry` |
| Table library | Manual list rendering vs `bubbles/table` | Manual is flexible but re-implements sort/truncation | `bubbles/table` with shared `styles` table header/selected; narrow-term collapse via `TruncateToWidth` |
| Animation guard | Per-screen check vs shared helpers | Per-screen drifts | Reuse `tui.tuiAnimationsDisabled()`+`isSyncSupported()`+`syncOutput`+`tickCmd()` nil-guard everywhere |
| Routing | New binary vs flag branch | New binary fragments UX | `--tui` in `cli_misc.go` takes precedence; `checkTUIInteractive()` guards non-TTY |

## Data Flow

```
CLI args ──parse --tui──┬─→ tui.RunWithScreen(id) ──→ Model.currentScreen ──→ View via syncOutput
                         │         │                       │
                         │    tea.WithAltScreen     HelpModel(textinput→filter→viewport)
                         │                             │                ↓
                         └─→ (no flag) → backup CLI ─┘         BackupModel(table→preview→confirm→tea.Cmd→backup.*)
                                         create/list/restore         │
                          Dashboard tile ──NavigateMsg────────────────┘
                                                        backup.List/Create/Restore → ~/.biggz/backups
```

Help: `textinput.Value()` → case-insensitive filter over `helpData` (`Title`/`Keys`/`Paragraph`) → `filtered` → `viewport.SetContent`. Backup: `backupListing`→`tea.Cmd(listBackups)`→`backupListMsg`→`table.SetRows()`+preview; `c`→`backupCreating`; `enter`→`backupRestoring` modal (`y`→`Create` then `Restore`, `n`/`ESC`→cancel).

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/tui/screens/help.go` | Modify | Add `HelpModel` (`textinput`+`viewport`, `filter`/`filtered`); filter handles `/`, `ESC`, scroll; reuse `helpData` |
| `internal/tui/screens/backup.go` | Modify | Promote to `table.Model`+preview+confirm modal; `listBackups`/`createBackup`/`restoreBackup` as `tea.Cmd`; safety `Create` before `Restore` |
| `internal/tui/tui.go` | Modify | Add `screenHelp`, `RunWithScreen(id)` (`tea.WithAltScreen`, fallback dashboard); route `Update`/`View`; export constants |
| `internal/tui/screens/dashboard.go` | Modify | Add Help/Backup tiles to `dashboardActions`; `▸` cursor |
| `internal/tui/styles/styles.go` | Modify | Add `TableHeader`/`TableSelected`/`PreviewPane`/`ModalOverlay` shared styles |
| `cmd/biggz/cli_misc.go` | Modify | Parse `--tui` before dispatch for `help`/`backup`; precedence over `--json`/subverbs |
| `cmd/biggz/main.go` | Modify | Wire `help` verb, delegate to `cli_misc`, keep `checkTUIInteractive()` guard |

## Interfaces / Contracts

```go
// Help model
type HelpModel struct {
  input    textinput.Model
  viewport viewport.Model
  filter   string
  filtered []HelpContent // derived from helpData
  focused  bool
  width, height int
}
func NewHelpModel() HelpModel
func (m HelpModel) filterHelp(q string) []HelpContent // case-insensitive across Title/Keys/Paragraph

// Backup model extension
type BackupModel struct {
  table          table.Model
  step           backupStep
  items          []backupEntry
  cursor         int
  preview        Backup // selected for pane
  confirmPending bool
  width          int
}
type backupListMsg struct { items []backupEntry; err string }
type backupResultMsg struct { status string; err string }

// Router
func RunWithScreen(id int) // tea.WithAltScreen, fallback to screenDashboard
const ScreenHelp = screenHelp; const ScreenBackup = screenBackup

// Shared style contract: styles.TableHeader/Selected derive from Rose Pine; screens MUST NOT define own palette
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Help filter (case-insensitive, empty, no-match), `tuiAnimationsDisabled`, `syncOutput` wrapper, `TruncateToWidth` narrow | `go test ./internal/tui/...` with `TERM` env matrix |
| Integration | Backup `tea.Cmd` mapping `backup.List`→`table.Rows`, preview sync on cursor, `backup.Create` error → `ErrorBox` | Temp `backupDir` via `t.TempDir()`, no home dir writes |
| E2E (teatest) | Help: `teatest.NewTestModel` type filter, `ESC` clear, scroll, `?` focus guard; Backup: nav, create, restore `y`/`n` modal, goldens deterministic with `TERM=dumb` | Golden files + `teatest` |

## Threat Matrix

| Boundary | Applicability | Design response | Planned RED tests |
|---|---|---|---|
| Documentation-like paths | N/A — no executable-doc classification; TUI does not execute files | — | — |
| Git repository selection | N/A — no `git -C` or repo cwd logic; `backupDir` is explicit arg or `~/.biggz/backups` | — | — |
| Commit state | N/A — no commit/index handling | — | — |
| Push state | N/A — no push/ref handling | — | — |
| PR commands | N/A — no PR composition; CLI routing is in-process `tea.Model` switch, not shell | — | — |

Note: `--tui`→`RunWithScreen` is in-process switch, not shell; `checkTUIInteractive` (stdin+stdout TTY) already covered in `main_tty_test.go`.

## Migration / Rollout

No migration required. CLI `backup create/list/restore` unchanged; `--tui` is additive. Rollback: revert `--tui` parsing in `cli_misc.go`/`main.go`, deregister `screenHelp`/`screenBackup` in `tui.go`, restore `screens/help.go`+`backup.go` from git. No data migration.

## Open Questions

- [ ] Confirm `gentle-pi` viewport max height convention for help overflow (current: 10→dynamic via `WindowSizeMsg`)
- [ ] Pre-restore safety snapshot: `SHOULD` vs `MUST` — keep optional per spec but implement as default before `Restore`
