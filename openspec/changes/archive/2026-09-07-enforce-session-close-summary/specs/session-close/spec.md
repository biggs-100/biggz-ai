# session-close Specification

## Purpose

Verifiable session-close gate for BigMem `session_summary` (issue #9).
Single Go implementation reused by CLI, Pi guards, scripts, and MCP-less agents.
Project filter stays `biggz-ai`-only; degraded fallback never satisfies the gate;
the agent MUST never be trapped (bounded retry, then degrade and deliver).

## Requirements

### Requirement: Verify Summary Status

The system MUST provide `biggz session-close --check-only [--cwd DIR] [--json]`.
It MUST reuse `VerifySessionSummaryWithWorkspace` from `internal/sdd/session_guard.go`
and MUST NOT duplicate BigMem logic. Exit code MUST be 0 when a `session_summary`
is verified, and 1 with `blocked(session_summary_missing)` plus fallback
instructions otherwise.

#### Scenario: Summary present allows close

- GIVEN a verified `session_summary` exists for the workspace project
- WHEN `biggz session-close --check-only` runs
- THEN the exit code MUST be 0

#### Scenario: Summary absent blocks with reason

- GIVEN no `session_summary` is verified
- WHEN `biggz session-close --check-only` runs
- THEN the exit code MUST be 1
- AND stderr MUST contain `blocked(session_summary_missing)` and fallback instructions

#### Scenario: JSON output shape

- GIVEN `biggz session-close --check-only --json`
- WHEN verification completes
- THEN stdout MUST be valid JSON with `verified` (bool), `reason` (string), and `fallback` (string, empty when verified)

### Requirement: Save Summary with Fallback

`biggz session-close --save "text" [--cwd DIR] [--json]` MUST persist via MCP
when available, else via the `bigmem save --type session_summary` bash path. It MUST
retry once on failure. On persistent failure it MUST write
`openspec/changes/{change}/session-fallback.md`, report status `degraded` with the
file path, and return WITHOUT hanging. `--check-only` and `--save` MUST be
mutually exclusive; passing both MUST fail with usage on stderr and non-zero exit.

#### Scenario: Save reaches BigMem

- GIVEN `--save "text"` with a working store
- WHEN the command runs
- THEN the summary MUST be persisted and exit code MUST be 0

#### Scenario: Persistent failure degrades without trapping

- GIVEN `--save "text"` with BigMem persistently failing
- WHEN the command runs after one retry
- THEN `session-fallback.md` MUST exist with the content
- AND output MUST report status `degraded`, NOT `verified`

### Requirement: Project Filter Scope

The gate MUST apply only to project `biggz-ai`. Any other project MUST verify
as true (allow) without writing fallback files.

#### Scenario: Foreign project always allows

- GIVEN workspace project is not `biggz-ai`
- WHEN `biggz session-close --check-only` runs
- THEN the exit code MUST be 0 and no fallback file MUST be written

### Requirement: Fallback File Never Satisfies Gate

A `session-fallback.md` file MUST be treated as degraded evidence only. Its
presence MUST NOT make verification return true; only a persisted
`session_summary` clears the gate.

#### Scenario: Fallback alone still blocks

- GIVEN `session-fallback.md` exists but no `session_summary` is persisted
- WHEN `biggz session-close --check-only` runs
- THEN the exit code MUST still be 1
