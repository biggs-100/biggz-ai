# Delta for cli

## ADDED Requirements

### Requirement: --tui Flag Routing for Help and Backup

The CLI MUST parse boolean `--tui` for `biggz help` and `biggz backup` (and their sub-verbs), branching to the Bubbletea TUI via `tui.RunWithScreen(screenHelp|screenBackup)` when set; without the flag existing `create/list/restore` and help dispatch MUST remain unchanged.

#### Scenario: help --tui launches Help TUI

- GIVEN CLI invoked as `biggz help --tui`
- WHEN `cli_misc.go` parses flags before dispatch
- THEN system MUST call `tui.RunWithScreen(screenHelp)` and exit with its code

#### Scenario: backup --tui launches Backup TUI

- GIVEN CLI invoked as `biggz backup --tui`
- WHEN flag parsing completes
- THEN system MUST call `tui.RunWithScreen(screenBackup)` instead of printing usage

#### Scenario: backup --tui combined with subverb ignored

- GIVEN `biggz backup list --tui` or `biggz help --tui --json`
- WHEN parser sees `--tui` alongside positional or other flags
- THEN `--tui` MUST take routing precedence and launch TUI, not CLI JSON/table path

#### Scenario: Without flag CLI behavior preserved

- GIVEN `biggz backup create /tmp/data` without `--tui`
- WHEN router dispatches
- THEN `backup.Create` CLI path MUST execute and print `Backup created: <id>`

#### Scenario: Help documents --tui flag

- GIVEN `biggz help --help`, `biggz backup --help`, or `biggz backup create --help`
- WHEN help renders
- THEN output MUST list `--tui` as `launch interactive Bubbletea TUI`

#### Scenario: Unknown flag still errors

- GIVEN `biggz backup --tui --unknown`
- WHEN flag parsing runs
- THEN system MUST print error to stderr and exit non-zero without launching TUI

### Requirement: Help Verb Wiring

The CLI MUST wire a `help` verb in `cmd/biggz/main.go` switch router alongside existing verbs, supporting `--help`/`-h` and `--tui` without adding new storage or encryption behavior.

#### Scenario: help verb dispatch

- GIVEN `biggz help` is registered in `main.go` switch
- WHEN invoked without flags
- THEN default help text or existing help handler MUST execute with exit 0

#### Scenario: help verb unknown subarg shows usage

- GIVEN `biggz help --tui` with non-terminal `stdin` (e.g., piped)
- WHEN `checkTUIInteractive` runs before launch
- THEN system MUST print `biggz tui requires both stdin and stdout to be terminals` and exit non-zero
