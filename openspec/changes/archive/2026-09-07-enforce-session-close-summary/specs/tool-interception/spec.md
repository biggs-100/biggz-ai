# Delta for Tool Interception

## ADDED Requirements

### Requirement: Session-Stop Summary Verification

The `session_stop` guard in `biggz-tool-interception.js` MUST shell out to
`biggz session-close --check-only` (bounded timeout, default 1000ms) AFTER the
existing pending-findings/lenses check. On CLI exit 0 it MUST allow close; on
exit 1 carrying the `blocked(session_summary_missing)` token it MUST return
`{ block: true, reason }` with the CLI's reason. On CLI timeout, execution
failure, or exit 1 WITHOUT the gate token (e.g. a stale binary without the
verb, which exits 1 with help text) it MUST NOT trap the agent: it MUST allow
close and emit a degraded warning. It MUST NOT reimplement BigMem verification in JS.

#### Scenario: Verified summary allows stop

- GIVEN no pending findings/lenses and CLI exits 0
- WHEN `session_stop` fires
- THEN the guard MUST return allow (undefined)

#### Scenario: Missing summary blocks stop

- GIVEN no pending findings/lenses and CLI exits 1 with `blocked(session_summary_missing)`
- WHEN `session_stop` fires
- THEN the guard MUST return `{ block: true }` with `blocked(session_summary_missing)`

#### Scenario: Stale binary never traps stop

- GIVEN the installed `biggz` predates the `session-close` verb (exits 1 with help text, no gate token)
- WHEN `session_stop` fires
- THEN the guard MUST allow close and MUST emit a degraded warning

#### Scenario: Pending work still blocks first

- GIVEN `BIGGZ_PENDING_FINDINGS>0` or `BIGGZ_PENDING_LENSES>0`
- WHEN `session_stop` fires
- THEN the guard MUST return `{ block: true }` without invoking the CLI

#### Scenario: CLI failure degrades without trapping

- GIVEN the CLI times out or crashes
- WHEN `session_stop` fires
- THEN the guard MUST allow close and MUST emit a degraded warning
