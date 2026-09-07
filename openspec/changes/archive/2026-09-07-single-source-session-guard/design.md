# Design: Single-Source Session Guard

## Technical Approach

Verbatim move of the guard block (`checkSessionStop` + `SESSION_STOP_TIMEOUT_MS` + `_setSessionStopExecForTest`, `internal/assets/pi/biggz-tool-interception.js` lines 8–64) into new `internal/assets/pi/biggz-session-guard.js`. Old path keeps a static re-export; `biggz-extension-api.js` and the test import the guard directly. One deploy-list line ships it. Zero behavior change; the 10 existing JS tests are the contract.

## Architecture Decisions

| Option | Tradeoff | Decision |
|--------|----------|----------|
| Re-export all 3 symbols vs only `checkSessionStop` | Partial re-export breaks old-path consumers of the timeout/seam (test imports all 3 today) | Re-export all 3 (line below) |
| Guard imports `node:child_process` only vs shared helper | Helper adds a second local edge and cycle risk | Zero local imports; cycle impossible by construction |
| Deploy entry grouped with consumers vs appended at end | End-of-list hides ownership | Insert after `extension-api.js` entry (new line 97) |

Re-export text (replaces lines 8–64 in `tool-interception.js`, keeping its header comment):

```js
export { checkSessionStop, SESSION_STOP_TIMEOUT_MS, _setSessionStopExecForTest } from "./biggz-session-guard.js";
```

The `import { execFileSync }` (line 8) goes with the block — no other use in the file. No `embed.go` change: `//go:embed all:pi` already covers the new file.

## Data Flow

Before: `extension-api.js` ──→ `tool-interception.js` (defines guard) ←── test
After: `extension-api.js` ──→ `session-guard.js` ←── test; `tool-interception.js` ──re-export──→ `session-guard.js`. All edges one-way into the guard; guard's only edge is `node:child_process`.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/assets/pi/biggz-session-guard.js` | Create | Guard block moved verbatim incl. APPLY-DECIDE comments |
| `internal/assets/pi/biggz-tool-interception.js` | Modify | Delete lines 8–64, insert re-export above |
| `internal/assets/pi/biggz-extension-api.js` | Modify | Line 16 import source → `./biggz-session-guard.js`; refresh lines 11–15 comment |
| `internal/assets/pi/biggz-session-stop.test.mjs` | Modify | Lines 5–10 import source → `./biggz-session-guard.js` only |
| `internal/install/steps/pi_extensions.go` | Modify | Insert `{"pi/biggz-session-guard.js", "biggz-session-guard.js"},` after line 96 |

## Interfaces / Contracts

Guard exports (unchanged signatures): `checkSessionStop(opts?) → undefined \| { block, reason }`, `SESSION_STOP_TIMEOUT_MS = 1000`, `_setSessionStopExecForTest(fn)`. Re-export surfaces identical bindings (same module instance, so the shared-seam parity test still holds).

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit (JS) | 10/10 existing tests pass, intent unmodified | `node --test internal/assets/pi/biggz-session-stop.test.mjs` |
| Single-definition | Exactly one definition site | `rg "function checkSessionStop\|const SESSION_STOP_TIMEOUT_MS\|function _setSessionStopExecForTest" internal/assets/pi/*.js` → only guard |
| Acyclicity | No local imports in guard; no cycle at load | `grep import biggz-session-guard.js` → only `node:child_process`; both-file parity test proves shared instance |
| Build | Deploy list valid | `go build ./...` + entry present in `pi_extensions.go` |

## Threat Matrix

Subprocess boundary (`execFileSync` argv array, no shell) moves verbatim: applicable as preserved invariant — expected behavior byte-identical (argv `["session-close","--check-only","--cwd",cwd]`, timeout 1000, exit-1 token match, degrade-to-allow); RED test = existing argv-array + degrade cases in the 10. Routing / shell-string / VCS-PR / executable-classification / process-integration changes: N/A (none introduced; no shell string exists before or after).

## Migration / Rollout

No migration. Single-commit move; rollback = revert commit, optionally delete stale deployed `biggz-session-guard.js` (inert after revert).

## Open Questions

None — spec open item resolved above (re-export covers all 3 symbols; deploy insert after line 96).
