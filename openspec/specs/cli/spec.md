# CLI Specification

## Purpose

The CLI domain defines the command-line entry point for biggz-ai. It is a switch-based verb dispatcher: each subcommand parses its own flags and delegates to its own run function. A bare interactive invocation launches the TUI; piped stdin without a subcommand is rejected with the help text and a non-zero exit code.

## Requirements

### Requirement: Verb Dispatch

The CLI MUST dispatch the first argument to a run function via a switch-based router covering all supported verbs (install, uninstall, sdd-*, bigmem, backup, release, skill-registry, rdd, tdd, review, doctor, update, sync, plugin, mcp, pr, export, hooks, recovery, version).

#### Scenario: Recognized verb

- GIVEN the CLI is invoked with a supported verb as the first argument
- WHEN main() dispatches
- THEN the matching xxxRun() function MUST be invoked
- AND the process exit code MUST be the value that function returns

#### Scenario: Unknown first argument without --help

- GIVEN the CLI is invoked with an unrecognized first argument
- WHEN no case matches in the router
- THEN help text MUST be printed to stderr
- AND the exit code MUST be non-zero

### Requirement: Bare Invocation Behavior

On bare invocation with an interactive terminal, the CLI MUST launch the TUI. When stdin is piped (not a character device) and no subcommand was given, the CLI MUST print help to stderr and exit non-zero instead of consuming the pipe.

#### Scenario: Interactive terminal

- GIVEN stdin is a character device
- WHEN the CLI starts with no arguments
- THEN the TUI MUST launch

#### Scenario: Piped stdin without a subcommand

- GIVEN stdin is a pipe and no subcommand was given
- WHEN the CLI starts
- THEN help MUST be printed to stderr
- AND the exit code MUST be non-zero

### Requirement: Exit Codes

The CLI MUST exit with code 0 on success and non-zero on any error condition.

#### Scenario: Success exit

- GIVEN a successful command execution
- WHEN the CLI exits
- THEN the exit code MUST be 0

#### Scenario: Error exit

- GIVEN any error during flag parsing or execution
- WHEN the CLI exits
- THEN the exit code MUST be non-zero
- AND an error description MUST be written to stderr

### Requirement: Doctor Subcommand

The CLI MUST add a "doctor" subcommand dispatched via the existing switch-based router.

#### Scenario: Doctor dispatch

- GIVEN the CLI is invoked as `biggz doctor`
- WHEN the router matches the "doctor" command
- THEN doctorRun() MUST be invoked

### Requirement: --json Flag

The doctor subcommand MUST parse a --json flag. When present, output MUST be valid JSON marshaled from the Report struct, parsable by standard JSON tools.

#### Scenario: JSON output

- GIVEN `biggz doctor --json`
- WHEN all checks complete
- THEN stdout MUST contain valid JSON with all check Results and severity buckets
- AND the exit code MUST be 0 (unless a check framework error occurs)

#### Scenario: JSON with --fix

- GIVEN `biggz doctor --fix --json`
- WHEN remedies execute and checks re-run
- THEN remedies MUST execute before JSON serialization
- AND the JSON output MUST reflect the post-fix state

### Requirement: --fix Flag

The doctor subcommand MUST parse a --fix flag. When present, the Runner MUST iterate declared remedies and execute their Actions after all initial checks complete. If no remedies are declared, zero actions execute and the output MUST indicate this.

#### Scenario: Remediation applied

- GIVEN `biggz doctor --fix` and at least one check declares a Remedy
- WHEN checks complete
- THEN each remedy Action MUST execute
- AND the output MUST include post-remedy status per check

#### Scenario: No remedies declared

- GIVEN `biggz doctor --fix` and no check declares a Remedy
- WHEN checks complete
- THEN zero actions MUST execute
- AND the output MUST indicate "0 remedies applied"

### Requirement: Default Renderer

The doctor subcommand MUST render results in human-readable format by default using tabla humana, grouped by severity bucket. Each check row MUST display a status icon: [ok] for pass, [!!] for warn, [xx] for fail. A summary line MUST show total counts per severity.

#### Scenario: Default table output

- GIVEN `biggz doctor` (no flags)
- WHEN all checks complete
- THEN stdout MUST contain severity-grouped sections
- AND each row MUST show check ID, status icon, and message
- AND a footer MUST show total pass/warn/fail counts

### Requirement: Update Subcommand

The CLI MUST add an `update` subcommand dispatched via the existing switch-based router. On Unix, the system MUST download the release archive, verify the checksum signature with the committed minisign public key, extract the binary, and atomically replace the running binary via os.Rename. On Windows, the system MUST NOT replace the binary and MUST instruct the user to run `go install`.

#### Scenario: Update on Unix — success

- GIVEN the CLI is invoked as `biggz update` on Linux or macOS
- WHEN the latest release is fetched, verified, and extracted
- THEN the binary MUST be replaced atomically (os.Rename)
- AND the new version string MUST be printed on success

#### Scenario: Update on Windows — fallback

- GIVEN `biggz update` on Windows
- WHEN the engine identifies the platform
- THEN binary replacement MUST NOT be attempted
- AND the system MUST print `go install github.com/biggs-100/biggz-ai@latest`

#### Scenario: Signature verification failure

- GIVEN checksums.txt.minisig does not verify against the committed public key
- WHEN the engine attempts verification
- THEN the update MUST abort
- AND a signature error MUST be printed to stderr

#### Scenario: Channel-aware update

- GIVEN BIGGZ_CHANNEL=beta
- WHEN `biggz update` fetches releases
- THEN the system MUST select the latest pre-release
- AND proceed with download and verification

#### Scenario: Already up to date

- GIVEN the running binary version equals the latest release version
- WHEN `biggz update` checks
- THEN the system MUST print "already up to date"
- AND exit with code 0

### Requirement: Sync Subcommand

The CLI MUST add a `sync` subcommand dispatched via the existing switch-based router. The system MUST accept the flags `--skills`, `--config`, `--prompts`, `--commands`, `--all`, and `--dry-run`. When `--all` is provided or no category flag is provided, the system MUST deploy all four categories. When specific category flags are provided, the system MUST deploy only those categories. When `--dry-run` is provided, the system MUST NOT write any files and MUST report the sync plan. Each category walks its source directory and calls `WriteFileAtomic` for each file.

#### Scenario: Sync all categories

- GIVEN the CLI is invoked as `biggz sync`
- WHEN the router matches the "sync" command
- THEN `syncRun()` MUST be invoked
- AND all four categories (skills, config, prompts, commands) MUST be deployed

#### Scenario: Selective sync

- GIVEN the CLI is invoked as `biggz sync --skills --config`
- WHEN `syncRun()` executes
- THEN only skills and config MUST be deployed
- AND prompts and commands MUST be skipped

#### Scenario: Dry-run reports without writing

- GIVEN the CLI is invoked as `biggz sync --dry-run`
- WHEN `syncRun()` executes
- THEN no files MUST be written to the filesystem
- AND a summary of what would be deployed MUST be printed to stdout
- AND the exit code MUST be 0

#### Scenario: All flag is equivalent to no flags

- GIVEN the CLI is invoked as `biggz sync --all`
- WHEN `syncRun()` executes
- THEN all four categories MUST be deployed

#### Scenario: Help output

- GIVEN the CLI is invoked as `biggz sync --help` or `-h`
- WHEN the help flag is parsed
- THEN usage information MUST be printed
- AND the exit code MUST be 0

#### Scenario: Unknown flag

- GIVEN the CLI is invoked as `biggz sync --unknown`
- WHEN `syncRun()` parses flags
- THEN the system MUST print an error to stderr
- AND the exit code MUST be non-zero

### Requirement: --from-engram Flag on bigmem sync import

The CLI MUST add `--from-engram` boolean flag to `biggz bigmem sync import` (and `sync`). When set, the CLI MUST route to the Engram import path (`ImportFromEngram`); when absent, the CLI MUST route to the existing BigMem import.

#### Scenario: Flag routes to Engram import

- GIVEN CLI invoked as `biggz bigmem sync import --from-engram`
- WHEN flag parsing completes
- THEN router MUST invoke Engram import handler with default `.engram` dir

#### Scenario: Flag absent uses BigMem

- GIVEN `biggz bigmem sync import` without `--from-engram`
- WHEN dispatched
- THEN existing BigMem `SyncImportDependencySafe` path MUST execute

### Requirement: --engram-dir and --project Flags

The CLI MUST add `--engram-dir <path>` (string) and `--project <name>` (string) flags to `biggz bigmem sync import`. `--engram-dir` MUST override the default Engram dir; `--project` MUST be forwarded as filter. Both SHOULD be valid only with `--from-engram`.

#### Scenario: Custom dir forwarded

- GIVEN `biggz bigmem sync import --from-engram --engram-dir /tmp/.engram`
- WHEN parsed
- THEN handler MUST receive `engramDir="/tmp/.engram"`

#### Scenario: Project filter forwarded

- GIVEN `biggz bigmem sync import --from-engram --project biggz-ai`
- WHEN parsed
- THEN handler MUST filter observations to `project=="biggz-ai"`

#### Scenario: Help documents flags

- GIVEN `biggz bigmem sync import --help`
- WHEN help renders
- THEN output MUST list `--from-engram`, `--engram-dir`, `--project`

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

### Requirement: CodeGraph Report Verb

The CLI MUST add `biggz codegraph report <change> [--cwd <path>] [--json <path>] [--md <path>]` via the switch router, validating `<change>` and delegating to `reportRun` in `cmd/biggz/cli_codegraph.go`; `--cwd` defaults to `.`, `--json`/`--md` MUST override output paths, and `biggz codegraph --help` MUST document `report`.

#### Scenario: Report emits dual output

- GIVEN `biggz codegraph report my-change --cwd .` with valid SDD artifacts
- WHEN `reportRun` executes
- THEN exit code MUST be 0 and JSON `{files,graph}` plus `openspec/changes/my-change/codegraph.md` MUST be written

#### Scenario: Custom output flags

- GIVEN `--json /tmp/cg.json --md /tmp/cg.md`
- WHEN report runs
- THEN outputs MUST be at those exact paths and parents MUST be created

#### Scenario: Missing change fails

- GIVEN `biggz codegraph report` without `<change>` or with missing `proposal.md`
- WHEN parsed
- THEN CLI MUST print usage or `proposal required` to stderr and exit non-zero

#### Scenario: Help documents report

- GIVEN `biggz codegraph --help` or `biggz codegraph report --help`
- WHEN help renders
- THEN output MUST list `report <change>` and flags `--cwd`, `--json`, `--md`

#### Scenario: Existing init preserved

- GIVEN `biggz codegraph init --cwd <root>` or `biggz codegraph guidance`
- WHEN invoked
- THEN behavior MUST remain unchanged and MUST NOT be routed to report

### Requirement: RDD CLI expectedRevision and Scope Wiring

The CLI MUST expose `biggz rdd disable --scope=clone|worktree|global --expected-revision=<rev>`, forward `expectedRevision` to `SetCloneLocalRDDMode`, surface `ErrRDDModeRevisionMismatch` without fallback, and print exact enable command per `Source`.

#### Scenario: Disable forwards expectedRevision on mismatch

- GIVEN head rev `head-rev`
- WHEN `biggz rdd disable --scope=clone --expected-revision=stale-rev` runs
- THEN MUST fail with `expected "stale-rev" but head is "head-rev"`

#### Scenario: Status shows Revision and Reach

- GIVEN `biggz rdd status --json` after clone disable
- WHEN queried
- THEN JSON MUST contain `revision` and `reach` (`machine`/`this_build`/unreported)

### Requirement: REQ-RR1-CLI — Recall/Recent Dispatch

CLI MUST dispatch `biggz recall` / `biggz bigmem recent` to `Search("", opts)`; forwards `--json|--limit|--type|--project|--scope`; caps `--limit` 50.

#### Scenario: Alias works

- GIVEN `biggz recall --json --limit 5`
- WHEN dispatched
- THEN returns `updated_at DESC`

#### Scenario: Flags forwarded

- GIVEN `recent --type session_summary --project biggz-ai --limit 10 --json`
- WHEN run
- THEN forwards all to `Search`

#### Scenario: Help documents

- GIVEN `recall --help` / `recent --help`
- WHEN rendered
- THEN lists `--json --limit --type --project` and recency note

### Requirement: Paged export with explicit cap

`biggz bigmem export` MUST page through `Search` in chunks (respecting the 50-row Store cap) instead of a single `Limit:100000` call, MUST accept `--limit N` (explicit row cap) and `--project P` flags, and MUST write the complete capped set to the file stream.

#### Scenario: Export beyond 50 rows completes
- GIVEN a store with 120 observations
- WHEN `biggz bigmem export out.json` runs
- THEN `out.json` MUST contain all 120 observations

#### Scenario: Explicit cap honored
- GIVEN a store with 120 observations
- WHEN `biggz bigmem export out.json --limit 60` runs
- THEN `out.json` MUST contain exactly 60 observations

#### Scenario: Project filter forwarded
- GIVEN observations in projects P1 and P2
- WHEN `biggz bigmem export out.json --project P1` runs
- THEN only P1 observations MUST be exported

### Requirement: Export shape and conflicts preservation

Export output MUST keep the current JSON array-of-observations shape so `biggz bigmem import` round-trips without change. The `conflicts` CLI MUST keep calling unscoped `ListRelations("")` with byte-identical output format.

#### Scenario: Import round-trip
- GIVEN `out.json` produced by paged export
- WHEN `biggz bigmem import out.json` runs
- THEN all exported observations MUST re-import with zero parse errors

#### Scenario: Conflicts output unchanged
- GIVEN pending relations
- WHEN `biggz bigmem conflicts list` runs before and after the change
- THEN output format and row content MUST be identical
