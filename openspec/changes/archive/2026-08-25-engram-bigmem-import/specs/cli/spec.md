# Delta for cli

## ADDED Requirements

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
