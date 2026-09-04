# Delta for sdd-status

## ADDED Requirements

### Requirement: BigMem Status via Store Ctx API

The system MUST derive BigMem status via Store `*Ctx` methods (`SearchCtx` or equivalent) and MUST NOT use raw `sql.Open` or non-Ctx `db.Query` in `internal/sdd/engram_status.go`.

#### Scenario: Store-sourced collection

- GIVEN BigMem holds `sdd/` observations
- WHEN `collectBigMemChangesWithArchive` runs
- THEN results MUST come from Store `*Ctx` API and grep MUST find no `sql.Open`/`db.Query` in `engram_status.go`

#### Scenario: Absent DB falls back explicitly

- GIVEN no `bigmem.db` file exists
- WHEN collection runs
- THEN it MUST fall back to filesystem-only with an explicit logged warning

### Requirement: SQL-Side Visibility Filtering

The system MUST filter `project`, `scope`, `deleted_at IS NULL`, and `topic_key LIKE 'sdd/%'` in SQL, not by loading full content and filtering in Go.

#### Scenario: Predicates in SQL

- GIVEN mixed-project and deleted observations
- WHEN the Store query executes
- THEN SQL MUST contain project/scope/`deleted_at`/`topic_key` predicates and excluded rows MUST never hydrate `content`

### Requirement: Minimal Hydration

The system MUST fetch `content` only for rows surviving visibility filters; key-only selection MUST precede hydration.

#### Scenario: Visible-only hydration

- GIVEN 100 rows with 2 visible changes
- WHEN status derives
- THEN `content` MUST load only for the 2 survivors and artifact states MUST match full-hydration results

### Requirement: Caller Context With Timeout

Status hot spots MUST propagate caller `ctx` with timeout and MUST NOT use `context.Background` in `status.go` derivation (`IsSessionSummaryBlocked` sites) or the BigMem collector.

#### Scenario: Cancellation fails fast

- GIVEN a cancelled caller ctx
- WHEN `sdd-status` runs
- THEN it MUST return promptly with a wrapped `context.Canceled`/`DeadlineExceeded` error

#### Scenario: No Background at hot spots

- GIVEN `status.go` derivation code
- WHEN inspected
- THEN no `context.Background` MUST remain at the session-guard call sites

### Requirement: Visible BigMem Failures

The system MUST log and wrap DB errors with operation context and MUST NOT return silent `(nil,nil,nil)`; degraded filesystem-only mode is allowed ONLY with an explicit logged warning.

#### Scenario: Query error surfaces

- GIVEN a corrupt/unreadable BigMem DB
- WHEN collection queries
- THEN it MUST return/log a wrapped error naming the operation and MUST NOT return `(nil,nil,nil)` silently

### Requirement: Project Visibility Parity

The system MUST preserve parity: exclude `scope=personal`, match inferred project case-insensitively, and disable the project filter only when the test-store override is set.

#### Scenario: Personal excluded

- GIVEN one `personal` and one project observation for the inferred project
- WHEN status derives
- THEN output MUST include only the project observation

#### Scenario: Project match and override

- GIVEN an observation with non-matching project
- WHEN status derives in production
- THEN it MUST be excluded; AND with test-store override set it MUST be visible
