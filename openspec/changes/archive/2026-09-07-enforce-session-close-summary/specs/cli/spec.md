# Delta for CLI

## MODIFIED Requirements

### Requirement: Verb Dispatch

The CLI MUST dispatch the first argument to a run function via a switch-based router covering all supported verbs (install, uninstall, sdd-*, bigmem, backup, release, skill-registry, rdd, tdd, review, doctor, update, sync, plugin, mcp, pr, export, hooks, recovery, version, session-close).
(Previously: verb list ended at `version`, with no `session-close` verb.)

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

## ADDED Requirements

### Requirement: Session-Close Verb Contract

The `session-close` verb MUST be a thin wrapper: parse `--cwd DIR` (default `.`),
`--json`, `--check-only` / `--save "text"` (mutually exclusive, one required),
then delegate to `session_guard.go` with no duplicated BigMem logic. Exit codes
MUST be 0 (verified/saved) or 1 (`blocked(session_summary_missing)`/degraded).
`session-close --help` MUST document all flags and both exit codes.

#### Scenario: Verb routes to guard

- GIVEN `biggz session-close --check-only --cwd <dir>`
- WHEN the router dispatches
- THEN the session-close run function MUST execute against `<dir>`

#### Scenario: Conflicting mode flags fail

- GIVEN `biggz session-close --check-only --save "x"`
- WHEN flag parsing runs
- THEN usage MUST be printed to stderr and exit code MUST be non-zero

#### Scenario: Help documents contract

- GIVEN `biggz session-close --help`
- WHEN help renders
- THEN output MUST list `--cwd`, `--json`, `--check-only`, `--save`, and exit codes 0/1
