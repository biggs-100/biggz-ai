# Proposal: Help + Backup TUI (Full Bubbletea Port)

## Intent
Port `gentle-pi` `runtime/screens/help.go`+`backup.go` to `biggz-ai`. CLI `biggz backup create/list/restore --json` works but lacks discoverability; users memorize flags. Full Bubbletea TUI (search, tables, guided restore) restores parity, reusing `internal/backup` without duplicating business logic.

## Scope

### In Scope
- Help Bubbletea: searchable helpData, shortcuts, examples, lipgloss filter/viewport
- Backup Bubbletea: `List` table, preview (size/paths/date), create + restore with double-confirm
- Entry: `biggz help --tui`, `biggz backup --tui`, dashboard `?`/menu tiles
- Reuse `internal/backup` Create/List/Restore — no storage logic
- Tests: `teatest` + goldens, ~300 lines

### Out of Scope
- New storage/encryption/retention, daemon/schedule, remote sync
- Doctor TUI or help-editing
- Minimal static help — full interactive only

## Capabilities

### New Capabilities
- `tui-help`: Search/filter across HelpContent, shortcut tables, lipgloss viewport
- `tui-backup`: Sortable table, preview pane, create/restore with confirmation

### Modified Capabilities
- `cli`: Add `--tui` routing for `backup`/`help`; keep `create/list/restore`
- `tui`: Router + dashboard tiles, `screenHelp`/`screenBackup` registration, shared styles

## Approach
Port gentle-pi models faithfully: wrap `backup.List/Create/Restore` via `tea.Cmd`, add `table`+`textinput`+`viewport`. Help model adds filter state over `helpData`. Honor `BIGGZ_NO_ANIMATION`/`TERM=dumb` via `syncOutput`. Flag parser in `cli_misc.go` branches to `tui.RunWithScreen` when `--tui` set.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/tui/screens/help.go` | Modified | Bubbletea Model + search/filter/viewport |
| `internal/tui/screens/backup.go` | Modified | Table/preview/restore Model |
| `internal/tui/tui.go` | Modified | Screen registry + dashboard links |
| `cmd/biggz/cli_misc.go` | Modified | `--tui` flag parsing |
| `cmd/biggz/main.go` | Modified | Wire help verb |
| `internal/tui/styles/styles.go` | Modified | Shared table styles |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Restore without confirmation | High | Double-confirm modal + preview + pre-restore backup |
| Table overflow narrow term | Med | Collapse columns, viewport, truncation |
| gentle-pi API drift | Low | Pin to `backup.*` signatures; adapter |
| teatest flakes | Med | `TERM=dumb`, temp backup dir |

## Rollback Plan
Revert `--tui` parsing in `cli_misc.go`/`main.go`, deregister screens in `tui.go`, restore `screens/help.go`+`backup.go` from git. CLI `create/list/restore` untouched — no migration.

## Dependencies
- `bubbletea`, `bubbletea/table`, `lipgloss`, `teatest` (in `go.mod`)
- `internal/backup` API; no new dep

## Success Criteria

- [ ] `biggz help --tui` searchable; filter narrows keys/titles, ESC clears
- [ ] `biggz backup --tui` table from `backup.List`, preview id/size/paths/date
- [ ] Restore needs y/N confirm, calls `backup.Restore` only on accept
- [ ] Dashboard tiles navigate to Help/Backup
- [ ] `go test ./...` passes; teatest covers search/nav/confirm
- [ ] `BIGGZ_NO_ANIMATION=1` disables anim/syncOutput
