# Proposal: Single-Source Session Guard

## Intent

`checkSessionStop()` + `SESSION_STOP_TIMEOUT_MS` + test seam live in `biggz-tool-interception.js`; `biggz-extension-api.js` imports them. Two owners = divergence risk (prior change already unified a duplicate once). One shared ES module must own the guard.

## Scope

### In Scope
- New `internal/assets/pi/biggz-session-guard.js`: verbatim move of `checkSessionStop`, `SESSION_STOP_TIMEOUT_MS` (1000), `_setSessionStopExecForTest` (zero logic change)
- `biggz-tool-interception.js`: remove moved block, re-export from guard (backward-compat, no cycle)
- `biggz-extension-api.js`: repoint import to `./biggz-session-guard.js` (one-way, guard imports only `node:child_process`)
- Register guard in `internal/install/steps/pi_extensions.go` deploy list (~lines 93-102)
- Repoint `biggz-session-stop.test.mjs` imports to guard; 10 tests pass with unmodified intent

### Out of Scope
- Any guard behavior change (timeout value, block/degrade semantics, CLI argv)
- Go logic changes (deploy list entry only); other extensions; BigMem/review tooling
- New tests (existing 10 are the contract); `biggz-footer.js` untouched (prior-art pattern only)

## Capabilities

### New Capabilities
- None (pure move; no spec-level behavior added)

### Modified Capabilities
- None (requirements unchanged; file ownership only)

## Approach

Copy guard block verbatim into `biggz-session-guard.js` (keeps APPLY-DECIDE comments). `tool-interception.js` re-exports from guard so old import paths keep working; `extension-api.js` and test import guard directly. Precedent: `biggz-footer.js` statically imports `./biggz-extension-api.js` (jiti resolves same-dir ESM).

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/assets/pi/biggz-session-guard.js` | New | Sole owner of guard (moved verbatim) |
| `internal/assets/pi/biggz-tool-interception.js` | Modified | Remove block; re-export from guard |
| `internal/assets/pi/biggz-extension-api.js` | Modified | Import line only (→ guard) |
| `internal/install/steps/pi_extensions.go` | Modified | Add guard to deploy list |
| `internal/assets/pi/biggz-session-stop.test.mjs` | Modified | Import source only |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Import cycle breaks pi startup | Low | Guard has zero local imports; one-way edges only |
| jiti fails to resolve new file | Low | Same-dir static ESM, proven by footer precedent; test covers |
| Deployed env missing guard file | Med | Deploy-list entry + rollback below; verify step checks |

## Rollback Plan

Revert the move commit(s): delete `biggz-session-guard.js`, restore guard block in `tool-interception.js`, restore old imports. Stale deployed `biggz-session-guard.js` in `~/.pi/agent/extensions/` is inert (nothing imports it after revert); remove manually if desired.

## Dependencies

- None (self-contained JS move + one deploy-list line)

## Success Criteria

- [ ] `node --test internal/assets/pi/biggz-session-stop.test.mjs` → 10/10 pass, intent unmodified
- [ ] `rg checkSessionStop internal/assets/pi/*.js` shows single definition (guard) + re-export + 2 import sites
- [ ] `go build ./...` passes; guard file present in deploy list
- [ ] No behavior delta: timeout 1000, block/degrade semantics byte-identical
