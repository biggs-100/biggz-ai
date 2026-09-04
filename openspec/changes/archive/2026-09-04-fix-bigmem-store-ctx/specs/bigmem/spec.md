# Delta for bigmem

## ADDED Requirements

### Requirement: CTX-1 — Core Store *Ctx variants

The system MUST provide `SaveCtx`, `GetCtx`, `SearchCtx`, `UpdateCtx`, `DeleteCtx` on `Store`. Each MUST take `context.Context` as first parameter and MUST enforce a timeout. Signatures of existing methods MUST NOT change.

#### Scenario: Core *Ctx happy path

- GIVEN a live ctx and valid inputs
- WHEN `SaveCtx`/`GetCtx`/`SearchCtx`/`UpdateCtx`/`DeleteCtx` is called
- THEN behavior MUST match the non-`Ctx` twin (persist/get/rank/update/delete)

#### Scenario: Cancelled ctx fails visibly

- GIVEN an already-cancelled ctx
- WHEN any CTX-1 method is called
- THEN it MUST return an error wrapping `ctx.Err()` with zero silent fallback

### Requirement: CTX-2 — Extended *Ctx variants

The system MUST provide `SessionContextCtx`, `TimelineCtx`, `SavePromptCtx` in `full.go` with the same ctx + timeout pattern as CTX-1.

#### Scenario: Extended happy path

- GIVEN a live ctx
- WHEN `SessionContextCtx`/`TimelineCtx`/`SavePromptCtx` is called
- THEN results MUST match the non-`Ctx` twin

#### Scenario: Extended cancellation

- GIVEN a cancelled ctx
- WHEN any CTX-2 method is called
- THEN it MUST return wrapped `ctx.Err()` and MUST NOT query SQLite

### Requirement: CTX-3 — Shared timeout helper

The system MUST provide one shared helper (e.g. `WithTimeout`) applying a default timeout (e.g. 5s) when the caller ctx has no deadline, and MUST honor a caller-supplied deadline/timeout override. All CTX-1/CTX-2 methods MUST use it.

#### Scenario: Default timeout applied

- GIVEN a ctx without deadline
- WHEN any `*Ctx` method runs
- THEN the helper MUST apply the default timeout

#### Scenario: Caller override honored

- GIVEN a ctx with explicit deadline/timeout
- WHEN any `*Ctx` method runs
- THEN the caller value MUST win and the default MUST NOT extend it

### Requirement: CTX-4 — Wrapper delegation and driver wiring

Existing non-`Ctx` methods MUST become thin wrappers delegating to their `*Ctx` twin with `context.Background()`. Every `*Ctx` method MUST use `QueryContext`/`ExecContext`/`QueryRowContext` end-to-end, MUST check `ctx.Err()` explicitly, and MUST NOT use plain `Query`/`Exec`.

#### Scenario: Wrapper parity

- GIVEN any legacy `Save`/`Get`/`Search`/`Update`/`Delete`/`SessionContext`/`Timeline`/`SavePrompt` call
- WHEN executed
- THEN it MUST delegate to the `*Ctx` twin with `Background()` and return identical results

#### Scenario: Driver cancellation surfaces

- GIVEN a ctx cancelled mid-query (WAL lock / slow FTS)
- WHEN the driver returns a ctx error
- THEN the method MUST return wrapped `ctx.Err()` visibly

### Requirement: CTX-5 — Consumer migration to *Ctx

`cmd/biggz-mcp/main.go`, `internal/sdd/session_guard.go`, and `internal/doctor/bigmem.go` MUST call `*Ctx` variants with a request-scoped ctx instead of legacy methods or raw `db.QueryContext` on the store DB.

#### Scenario: Three consumers use *Ctx

- GIVEN the three listed call sites
- WHEN code is inspected (`rg "\.SaveCtx|\.SearchCtx|\.GetCtx"`)
- THEN each file MUST contain at least one `*Ctx` call and no new legacy `store.Save/Search/Get` at those sites

#### Scenario: session_guard pre-check preserved

- GIVEN `session_guard.go` migrated to `SearchCtx`
- WHEN ctx is already done
- THEN the existing `select ctx.Done` pre-check MUST still short-circuit before SQLite
