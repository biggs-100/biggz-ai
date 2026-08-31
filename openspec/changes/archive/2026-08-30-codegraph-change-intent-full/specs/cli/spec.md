# Delta for cli

## ADDED Requirements

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
