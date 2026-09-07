# Proposal: Enforce Session-Close Summary

## Intent

Issue #9: `session_summary` protocol is unenforced outside SDD `done`/`apply` gate. Agents close without verifiable summary, claiming inability despite working bash fallback. Make close verifiable everywhere via shared guard.

## Scope

### In Scope
- Cut 1: `biggz session-close [--cwd DIR] [--json] [--check-only | --save "text"]` thin wrapper, no duplicated logic
- Cut 2: extend `session_stop` guard in `biggz-tool-interception.js` to invoke CLI; unify duplicate in `biggz-extension-api.js`
- Decisions: project filter stays `biggz-ai`-only; `session-fallback.md` degrades only, never satisfies gate

### Out of Scope
- `on_session_close` hooks event as enforcement (voluntary only, discarded)
- Widening filter beyond `biggz-ai`; BigMem schema/MCP changes; TUI changes

## Capabilities

### New Capabilities
- `session-close`: verify/fallback/block contract for session-summary close (`--cwd`, `--json`, `--check-only`/`--save`; exit 0 present, exit 1 `blocked(session_summary_missing)`)

### Modified Capabilities
- `cli`: add `session-close` verb to switch router
- `tool-interception`: `session_stop` verifies via CLI, blocks on exit 1
- `extension-api`: converge duplicate `session_stop` into single delegation

## Approach

Cut 1 first: CLI reuses `VerifySessionSummaryWithWorkspace` + `SaveSessionSummaryWithFallbackForChange` from `internal/sdd/session_guard.go`. Cut 2: `session_stop` shells to CLI (~100-300ms), returns `{ block: true }` on exit 1. No new BigMem logic; `sdd`/`bigmem` specs unchanged.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `cmd/biggz/main.go` | Modified | Register `session-close` verb |
| `cmd/biggz/cli_session_close.go` | New | Thin wrapper, flag parsing, JSON output |
| `internal/sdd/session_guard.go` | Reused | No logic duplication; reuse as-is |
| `internal/assets/pi/biggz-tool-interception.js` | Modified | `session_stop` verify → fallback → block |
| `internal/assets/pi/biggz-extension-api.js` | Modified | Remove duplicate; delegate to single guard |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Non-TUI/no-extension bypasses `session_stop` | High | CLI remains auditable/scriptable fallback |
| Slow/corrupt BigMem traps close | Med | Degrade to `session-fallback.md`, never trap; retry once |
| Divergent `session_stop` handlers | Med | Single guard; second file delegates |
| Filter widening causes surprise blocks | Low | Keep `biggz-ai`-only; generalize later |

## Rollback Plan

Revert CLI verb registration + Pi guard to prior allow behavior (one commit each cut). `session-fallback.md` files are harmless orphan evidence; delete or leave.

## Dependencies

- `cmd/biggz/cli_bigmem.go` `save --type session_summary` bash fallback; MCP `mem_session_summary`

## Success Criteria

- [ ] `session-close --check-only` exit 0 when summary verified, exit 1 + fallback instructions otherwise
- [ ] `session_stop` blocks close on exit 1, allows on exit 0
- [ ] No duplicated guard logic; 400-line budget kept via chained PRs
