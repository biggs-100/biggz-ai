# Delta for CLI

## ADDED Requirements

### Requirement: Sync Subcommand

The CLI MUST add a `sync` subcommand dispatched via the existing switch-based router. The system MUST accept the flags `--skills`, `--config`, `--prompts`, `--commands`, `--all`, and `--dry-run`. When `--all` is provided or no category flag is provided, the system MUST deploy all four categories. When specific category flags are provided, the system MUST deploy only those categories. When `--dry-run` is provided, the system MUST NOT write any files and MUST report the sync plan. The subcommand MUST NOT read or parse a `ReviewSubject` from stdin. Each category walks its source directory and calls `WriteFileAtomic` for each file.

#### Scenario: Sync all categories

- GIVEN the CLI is invoked as `biggz sync`
- WHEN the router matches the "sync" command
- THEN `syncRun()` MUST be invoked
- AND all four categories (skills, config, prompts, commands) MUST be deployed
- AND no stdin review parsing MUST occur

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
