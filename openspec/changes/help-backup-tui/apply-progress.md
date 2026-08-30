# Apply Progress — help-backup-tui

**Change**: help-backup-tui
**Mode**: Standard (strict_tdd false, runner `go test ./... -count=1 -timeout 180s`, artifact_store `openspec`)
**PR Strategy**: stacked-to-main, auto-chain, 3 PRs
**PR1**: foundation — styles + tui registry + dashboard (182 lines, token tok-f695dbda8ab15f102715a270)
**PR2**: Help — textinput+viewport filter (339 lines, token tok-b7841b63d57877c9bc0f97de)

## Completed Tasks (PR1+PR2)

- [x] 1.1 Extend `internal/tui/styles/styles.go` with `TableHeader`/`TableSelected`/`PreviewPane`/`ModalOverlay` (Rose Pine) — `VisibleWidth(View(80))≤80`
- [x] 1.2 Add `screenHelp`/`screenBackup` in `internal/tui/tui.go`, export `ScreenHelp`/`ScreenBackup`, unknown → `screenDashboard`
- [x] 1.3 Implement `RunWithScreen(id)` with `tea.WithAltScreen`, fallback dashboard — `Model.currentScreen==screenHelp` on init
- [x] 2.1 Create `HelpModel` in `screens/help.go`: `textinput`+`viewport`, `filterHelp(q)` case-insensitive `Title`/`Keys`/`Paragraph` over `helpData`
- [x] 2.2 `Update`: `/` focus, live narrow, `ESC` clear→back, `?`/`q` suppressed when focused
- [x] 2.3 `View`: `viewport.SetContent`+`lipgloss`+`TruncateToWidth`+`syncOutput` guard (`BIGGZ_NO_ANIMATION`/`TERM=dumb` no `ESC[?2026h`)
- [x] 3.4 Dashboard: add `Help`/`Backup & Restore` to `dashboardActions` with `▸`, `enter`→`NavigateMsg`

## Files Changed (PR1+PR2)

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/tui/styles/styles.go` | Modified | Added `TableHeader`/`TableSelected`/`PreviewPane`/`ModalOverlay` Rose Pine styles, VisibleWidth ≤80 |
| `internal/tui/tui.go` | Modified | Added `screenHelp` constant, exported `ScreenHelp`/`ScreenBackup`, `Model.help` field, `New()` init, `RunWithScreen` alt screen fallback, `Init`/`Update`/`View` routing, WindowSize forwarding, `?` toggle suppressed when help input focused |
| `internal/tui/screens/dashboard.go` | Modified | Added `Help` (21) and `Backup & Restore` (5) to `dashboardActions` with `▸`, keys `h`/`b`, enter→NavigateMsg |
| `internal/tui/screens/help.go` | Modified | Full `HelpModel` with `textinput`+`viewport`, `filterHelp` case-insensitive, `buildHelpContent` with TruncateToWidth/WrapTextWithAnsi, `Update` with `/` focus/live narrow/ESC, `View` with viewport.SetContent+lipgloss |
| `internal/tui/screens/help_test.go` | Created | Tests: filterHelp empty/case-insensitive/narrows/no-matches placeholder, View shortcuts, viewport scroll, narrow truncation w=40, ESC clear/back, input focus suppression, viewport rendering |

## Work Unit Evidence

| Evidence | Required value |
|---|---|
| Focused test command and exact result (PR1) | `go test ./internal/tui -count=1` → PASS; `go vet ./internal/tui/...` → PASS |
| Focused test command and exact result (PR2) | `go test ./internal/tui/screens -run TestHelp -count=1 -v` → PASS (7 tests: EmptyShowsAll, CaseInsensitive, NarrowsBackup, NoMatchesPlaceholder, ViewContainsShortcuts, ViewportScroll, NarrowTruncation, ESC_ClearsFilter, ESC_ClosesWhenEmpty, InputFocusSuppression, ViewportRendering) |
| Runtime harness command/scenario and exact result | `go vet ./...` → PASS; `go test ./internal/tui/screens -run TestFilterHelp -count=1 -v` → PASS; `TERM=dumb go test ./internal/tui/screens -run TestHelp -count=1` → PASS (no ESC[?2026h) |
| Rollback boundary (PR1) | Revert `styles/styles.go`, `tui.go`, `screens/dashboard.go`, `screens/help.go` stub — `git revert 1a76308` |
| Rollback boundary (PR2) | Revert `screens/help.go` full + `screens/help_test.go` + `tui.go` ?-suppression — `git checkout HEAD~1 -- internal/tui/screens/help.go internal/tui/tui.go` and `git rm internal/tui/screens/help_test.go` |

## Deviations from Design

None — Help implementation matches design: textinput+viewport over helpData, filter derived state, TruncateToWidth narrow handling, syncOutput via tui wrapper, ?/q suppression.

## Issues Found

None.

## Remaining Tasks (for PR3)

- [ ] 3.1 Promote `screens/backup.go` to `table.Model`+preview: `listBackups() tea.Cmd` wraps `backup.List`→`backupListMsg`, `SetRows` ID/size/date
- [ ] 3.2 Create flow: `c`→`backupCreating`+`tea.Cmd` `backup.Create`; err→`backupError`+`ErrorBox`, ok→refresh+preview sync
- [ ] 3.3 Restore modal: `enter`→`backupRestoring` `y/N` preview; `y`→`Create` snapshot then `Restore`, `n`/`ESC` cancel no mutation; narrow `VisibleWidth≤width`
- [ ] 3.5 CLI `cmd/biggz/cli_misc.go` parse `--tui` (precedence over `--json`/subverbs→`RunWithScreen`); `main.go` wire `help` verb + `checkTUIInteractive` guard
- [ ] 4.1 Unit: `filterHelp` empty/case-insensitive/no-match + `TruncateToWidth` w=40 + `tuiAnimationsDisabled`/`tickCmd=nil`/`syncOutput` matrix
- [ ] 4.2 Integration temp `T.TempDir()`: `List→Rows`, preview cursor sync, `Create` err→`ErrorBox`, `y`/`n` modal side-effect check
- [ ] 4.3 teatest goldens `TERM=dumb`: Help filter `backup`, `ESC` clear, scroll; Backup nav/create/confirm `y` vs `n`, no home-dir leaks
- [ ] 4.4 CLI verify: `help --tui`/`backup --tui` precedence, `backup create` without `--tui` prints `Backup created:`, `--help` lists `--tui`
- [ ] 5.1 `go vet`+`gofmt -l` clean, zero `tar`/`gzip` in `screens/backup.go`, `go test ./... -count=1 -timeout 180s` pass

## Workload / PR Boundary

- Mode: stacked-to-main, auto-chain
- Current work unit: PR2 Help (2.1-2.3)
- Boundary: Starts at `screens/help.go` textinput/viewport, ends at `help_test.go` + `tui.go` ?-suppression; autonomous verifiable via `go test ./internal/tui/screens -run TestHelp -count=1`; rollback via revert of 3 files
- Estimated review budget impact: help.go 157 + help_test 200 + tui.go 10 = 339 lines, fits 400

## Status

7/16 tasks complete. Ready for PR3 (Backup+CLI) on branch `help-backup-tui-pr2-help` → next `help-backup-tui-pr3-backup-cli` stacked on PR2.

## Commands Run

- `biggz sdd-attempt acquire help-backup-tui --request-id pr1-foundation --work-unit pr1-foundation --evidence-goal "styles+tui+dashboard" --max-attempts 10 --max-lines 400` → tok-f695...
- `go vet ./internal/tui/...` → PASS
- `go test ./internal/tui -count=1` → PASS
- `biggz sdd-attempt settle` PR1 → cede54e8...
- `biggz sdd-attempt acquire help-backup-tui --request-id pr2-help --work-unit pr2-help --evidence-goal "help textinput viewport" --max-attempts 10 --max-lines 400` → tok-b7841b63...
- `go test ./internal/tui/screens -run TestHelp -count=1 -v` → PASS (11 tests)
- `go vet ./...` → PASS
