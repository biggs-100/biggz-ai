# Apply Progress — help-backup-tui

**Change**: help-backup-tui
**Mode**: Standard (strict_tdd false, runner `go test ./... -count=1 -timeout 180s`, artifact_store `openspec`)
**PR Strategy**: stacked-to-main, auto-chain, 3 PRs
**PR1**: foundation — styles + tui registry + dashboard (182 lines, token tok-f695dbda8ab15f102715a270)

## Completed Tasks (PR1)

- [x] 1.1 Extend `internal/tui/styles/styles.go` with `TableHeader`/`TableSelected`/`PreviewPane`/`ModalOverlay` (Rose Pine) — `VisibleWidth(View(80))≤80`
- [x] 1.2 Add `screenHelp`/`screenBackup` in `internal/tui/tui.go`, export `ScreenHelp`/`ScreenBackup`, unknown → `screenDashboard`
- [x] 1.3 Implement `RunWithScreen(id)` with `tea.WithAltScreen`, fallback dashboard — `Model.currentScreen==screenHelp` on init
- [x] 3.4 Dashboard: add `Help`/`Backup & Restore` to `dashboardActions` with `▸`, `enter`→`NavigateMsg`

## Files Changed (PR1)

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/tui/styles/styles.go` | Modified | Added `TableHeader`/`TableSelected`/`PreviewPane`/`ModalOverlay` Rose Pine styles, derived from shared palette, VisibleWidth ≤80 |
| `internal/tui/tui.go` | Modified | Added `screenHelp` constant, exported `ScreenHelp`/`ScreenBackup`, `Model.help` field, `New()` init, `RunWithScreen(id)` with alt screen fallback, `Init`/`Update`/`View` routing for help, WindowSize forwarding |
| `internal/tui/screens/dashboard.go` | Modified | Added `Help` (view 21) and `Backup & Restore` (view 5) to `dashboardActions` with `▸` cursor, keys `h`/`b`, enter→NavigateMsg |
| `internal/tui/screens/help.go` | Modified | Added minimal `HelpModel` stub with `filterHelp` case-insensitive, `NewHelpModel`/`Init`/`Update`/`View`, reuse helpData, ESC clear→back |

## Work Unit Evidence (PR1)

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./internal/tui -count=1` → PASS (4.041s); `go vet ./internal/tui/...` → PASS (no output) |
| Runtime harness command/scenario and exact result | `go vet ./...` → PASS; `go test ./internal/tui -count=1` → PASS; manual `go run` check dashboard View contains `Help` and `Backup & Restore` with `▸` |
| Rollback boundary | Revert `styles/styles.go`, `tui.go`, `screens/dashboard.go`, `screens/help.go` stub → `git checkout HEAD~1 -- <files>` or `git revert 1a76308` |

## Deviations from Design

None — foundation matches design: shared styles single source, screen registry, RunWithScreen alt screen, dashboard tiles.

## Issues Found

None.

## Remaining Tasks (for PR2/PR3)

- [ ] 2.1 Create `HelpModel` in `screens/help.go`: `textinput`+`viewport`, `filterHelp(q)` case-insensitive `Title`/`Keys`/`Paragraph` over `helpData`
- [ ] 2.2 `Update`: `/` focus, live narrow, `ESC` clear→back, `?`/`q` suppressed when focused
- [ ] 2.3 `View`: `viewport.SetContent`+`lipgloss`+`TruncateToWidth`+`syncOutput` guard (`BIGGZ_NO_ANIMATION`/`TERM=dumb` no `ESC[?2026h`)
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
- Current work unit: PR1 foundation (1.1-1.3, 3.4)
- Boundary: Starts at `styles/styles.go` TableHeader, ends at `help.go` stub + dashboard tiles; autonomous verifiable via `go vet ./internal/tui` + `go test ./internal/tui -count=1`; rollback via revert of 4 files
- Estimated review budget impact: 182 insertions, 2 deletions → 184 lines, Low risk fits 400

## Status

4/16 tasks complete. Ready for PR2 (Help) on branch `help-backup-tui-pr1-foundation` → next `help-backup-tui-pr2-help` stacked on PR1.

## Commands Run

- `biggz sdd-attempt acquire help-backup-tui --request-id pr1-foundation --work-unit pr1-foundation --evidence-goal "styles+tui+dashboard" --max-attempts 10 --max-lines 400` → token tok-f695dbda8ab15f102715a270 revision 691b7a2efe6b...
- `go vet ./internal/tui/...` → PASS
- `go test ./internal/tui -count=1` → PASS
- `biggz sdd-attempt settle help-backup-tui --token tok-f695dbda8ab15f102715a270 --request-id pr1-settle2 --outcome passed --evidence-revision 1a76308 --diagnosis "PR1 foundation pass" --harness-disposition passed --cleanup-evidence passed --process-evidence passed` → revision cede54e8...
