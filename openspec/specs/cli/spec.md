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
