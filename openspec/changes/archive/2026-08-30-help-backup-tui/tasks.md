# Tasks: Help + Backup TUI (Full Bubbletea Port)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~470 (help ~120 + backup ~150 + tui/dashboard/styles ~80 + cli ~40 + tests ~80) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1→PR2→PR3 stacked-to-main |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Styles + `tui.go` `screenHelp`/`screenBackup`+`RunWithScreen` + dashboard tiles | PR1 (base main) | `go test ./internal/tui -run TestDashboard -count=1` | `go vet ./internal/tui` + `go test ./internal/tui -count=1` | Revert `styles/styles.go`, `tui.go`, `screens/dashboard.go` |
| 2 | `HelpModel` `textinput`+`viewport` filter over `helpData` | PR2 (base PR1) | `go test ./internal/tui/screens -run TestHelp -count=1` | `TERM=dumb go test ./internal/tui -run TestHelp -count=1` | Revert `screens/help.go` only |
| 3 | `BackupModel` table/preview/confirm + `cli --tui` + teatest | PR3 (base PR2) | `go test ./internal/tui/screens -run TestBackup -count=1` | `biggz backup --tui` TTY + `go test ./... -count=1` | Revert `screens/backup.go`, `cmd/biggz/cli_misc.go`, `main.go` |

## Phase 1: Foundation

- [x] 1.1 Extend `internal/tui/styles/styles.go` with `TableHeader`/`TableSelected`/`PreviewPane`/`ModalOverlay` (Rose Pine) — `VisibleWidth(View(80))≤80`
- [x] 1.2 Add `screenHelp`/`screenBackup` in `internal/tui/tui.go`, export `ScreenHelp`/`ScreenBackup`, unknown → `screenDashboard`
- [x] 1.3 Implement `RunWithScreen(id)` with `tea.WithAltScreen`, fallback dashboard — `Model.currentScreen==screenHelp` on init

## Phase 2: Core — Help

- [x] 2.1 Create `HelpModel` in `screens/help.go`: `textinput`+`viewport`, `filterHelp(q)` case-insensitive `Title`/`Keys`/`Paragraph` over `helpData`
- [x] 2.2 `Update`: `/` focus, live narrow, `ESC` clear→back, `?`/`q` suppressed when focused
- [x] 2.3 `View`: `viewport.SetContent`+`lipgloss`+`TruncateToWidth`+`syncOutput` guard (`BIGGZ_NO_ANIMATION`/`TERM=dumb` no `ESC[?2026h`)

## Phase 3: Core — Backup + Routing

- [x] 3.1 Promote `screens/backup.go` to `table.Model`+preview: `listBackups() tea.Cmd` wraps `backup.List`→`backupListMsg`, `SetRows` ID/size/date
- [x] 3.2 Create flow: `c`→`backupCreating`+`tea.Cmd` `backup.Create`; err→`backupError`+`ErrorBox`, ok→refresh+preview sync
- [x] 3.3 Restore modal: `enter`→`backupRestoring` `y/N` preview; `y`→`Create` snapshot then `Restore`, `n`/`ESC` cancel no mutation; narrow `VisibleWidth≤width`
- [x] 3.4 Dashboard: add `Help`/`Backup & Restore` to `dashboardActions` with `▸`, `enter`→`NavigateMsg`
- [x] 3.5 CLI `cmd/biggz/cli_misc.go` parse `--tui` (precedence over `--json`/subverbs→`RunWithScreen`); `main.go` wire `help` verb + `checkTUIInteractive` guard

## Phase 4: Testing

- [x] 4.1 Unit: `filterHelp` empty/case-insensitive/no-match + `TruncateToWidth` w=40 + `tuiAnimationsDisabled`/`tickCmd=nil`/`syncOutput` matrix
- [x] 4.2 Integration temp `T.TempDir()`: `List→Rows`, preview cursor sync, `Create` err→`ErrorBox`, `y`/`n` modal side-effect check
- [x] 4.3 teatest goldens `TERM=dumb`: Help filter `backup`, `ESC` clear, scroll; Backup nav/create/confirm `y` vs `n`, no home-dir leaks
- [x] 4.4 CLI verify: `help --tui`/`backup --tui` precedence, `backup create` without `--tui` prints `Backup created:`, `--help` lists `--tui`

## Phase 5: Cleanup

- [x] 5.1 `go vet`+`gofmt -l` clean, zero `tar`/`gzip` in `screens/backup.go`, `go test ./... -count=1 -timeout 180s` pass

## Dependencies

1→2→3→4→5. Threat `N/A` → no RED.

## Evidence

```
go test ./internal/tui/screens -run TestHelp -count=1
go test ./internal/tui -count=1 -timeout 180s
go test ./... -count=1 -timeout 180s && go vet ./...
```
