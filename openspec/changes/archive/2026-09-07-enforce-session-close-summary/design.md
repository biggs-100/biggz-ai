# Design: Enforce Session-Close Summary

## Technical Approach

Two cuts, no new BigMem logic. Cut 1: `cmd/biggz/cli_session_close.go` thin wrapper over `VerifySessionSummaryWithWorkspace` + `SaveSessionSummaryWithFallbackForChange` (`internal/sdd/session_guard.go`), registered as `session-close` in `cmd/biggz/main.go`. Cut 2: `session_stop` in `biggz-tool-interception.js` shells to the CLI with a 1000ms timeout (exit 1 + gate token → `{block:true}`); the duplicate handler in `biggz-extension-api.js` statically imports and delegates to the same exported guard. Covers specs `session-close`, `cli`, `tool-interception`, `extension-api`.

## Architecture Decisions

| Option | Tradeoff | Decision |
|---|---|---|
| Check via `VerifySessionSummaryWithWorkspace` vs `IsSessionSummaryBlocked` | `IsSessionSummary…` treats `session-fallback.md` as satisfying the gate — forbidden by spec | Use `Verify…` directly; only persisted `session_summary` clears |
| `--save` via direct store (`hasMCP=true` path) vs `saveViaBash` child process | Child re-execs `biggz` (PATH-dependent, double process); direct store is the same `tryMCPSave` code MCP uses | Direct store; retry-once + degraded-file semantics stay inside the guard |
| Flag-usage errors exit 2 vs 1 | Convention is 1, but 1 already means gate-blocked; scripts must distinguish | Exit 2 for usage (spec allows non-zero), 1 for blocked/degraded, 0 for verified/saved |
| Fallback `change` attribution | Guard needs `change` for `FallbackPath`; default `fix-bigmem-session-discipline` misattributes | New optional `--change` flag, default `session-close` |
| JS `execFileSync` (1000ms) vs async spawn | Sync blocks the close path ~300ms typical; async risks unresolved verdict | `execFileSync` with `timeout:1000`, argv array (no shell, no injection) |
| Convergence: shared export vs deleting one handler | Interception early-returns on `BIGGZ_PRETTY=0`/`PI_SUBAGENT_CHILD=1`; load order varies | Export `checkSessionStop()` from interception; extension-api keeps its registration but delegates via static ESM import (precedent: footer → extension-api) |

## Data Flow

Cut 1:

    session-close --check-only --cwd DIR → DetectProjectFull(DIR)
      ├─ proj ≠ biggz-ai → exit 0 {verified:true} (no files)
      └─ proj = biggz-ai → VerifySessionSummaryWithWorkspace
           ├─ summary found → exit 0 {verified:true}
           └─ absent → exit 1 blocked(session_summary_missing) + instructions

    session-close --save "text" → Save…ForChange (direct store, retry×1)
      ├─ persisted → exit 0 {verified:true}
      └─ failed → write session-fallback.md → exit 1 {verified:false,status:degraded}

Cut 2:

    session_stop → pending findings/lenses? → block (no CLI call)
      └─ else execFileSync(biggz session-close --check-only, 1000ms)
           ├─ exit 0 → allow
           ├─ exit 1 + blocked(session_summary_missing) token → {block:true, reason}
           ├─ exit 1 without token (stale binary, help text) → allow + degraded warn
           └─ timeout/crash → allow + warn

## File Changes

| File | Action | Description |
|---|---|---|
| `cmd/biggz/cli_session_close.go` | Create | `sessionCloseRun()`; manual flag parse (`--cwd/--json/--check-only/--save/--change/--help`), 10s ctx timeout, JSON/text output |
| `cmd/biggz/main.go` | Modify | `case "session-close": os.Exit(sessionCloseRun())` |
| `cmd/biggz/cli_doctor_help.go` | Modify | One help line for `session-close` |
| `cmd/biggz/cli_session_close_test.go` | Create | Flag conflicts, foreign-project allow, JSON shape, save→check round-trip |
| `internal/sdd/session_guard.go` | Reuse | No changes; caller-side handles fallback-never-satisfies |
| `internal/assets/pi/biggz-tool-interception.js` | Modify | Export `checkSessionStop()`; pending check → CLI verify → block/allow/degraded-warn |
| `internal/assets/pi/biggz-extension-api.js` | Modify | Replace inline pending logic with delegation to shared guard |
| `internal/assets/pi/biggz-session-stop.test.mjs` | Create | Verdict parity: allow/block/degraded + delegation identity |

## Interfaces / Contracts

```
biggz session-close (--check-only | --save "text") [--cwd DIR] [--change C] [--json]
--check-only --json → stdout {"verified":bool,"reason":string,"fallback":string}
--save --json       → stdout {"verified":bool,"reason":string,"fallback":string,"status":"verified"|"degraded"}
```

Text mode: human line on stdout; `blocked(session_summary_missing)` + fallback instructions on stderr. `--check-only` + `--save` together → usage on stderr, exit 2. JS: `checkSessionStop(): Promise<undefined | {block:true, reason:string}>`, never throws (all failures → allow + `console.warn`).

## Testing Strategy

| Layer | What to Test | Approach |
|---|---|---|
| Unit (Go) | Flag conflicts, `--help` contract, foreign-project fast path | `go test` with temp `--cwd` dirs; no store needed |
| Integration (Go) | Save→check round-trip; fallback file never verifies; degraded path | Temp workspace + stubbed `bigmemOpen`/`execCommand` via guard seams |
| Unit (JS) | Block on exit 1, allow on 0, allow+warn on timeout, pending-first ordering | `.test.mjs` with mocked `execFileSync` (mirror `biggz-synthesis-gate.test.mjs`) |
| E2E | `session_stop` parity between both JS files | Same env in both handlers → identical verdicts |

## Threat Matrix

| Boundary | Applicability | Design response | Planned RED tests |
|---|---|---|---|
| Documentation-like paths | N/A: no file classification/execution | — | — |
| Git repository selection | Applicable: `--cwd` authority + fallback write target | Resolve `workspaceRoot` from `--cwd` only; `FallbackPath` already sanitizes `..`→`_`; JS passes argv array, no shell | Relative `--cwd`, absolute `--cwd`, traversal `--change ../x` stays anchored |
| Commit state | N/A: no index/worktree writes | — | — |
| Push state | N/A: no remotes/refs | — | — |
| PR commands | N/A: no PR automation | — | — |

Safe behavior: foreign project allows without writes; store/timeout failures degrade to allow (JS) or `degraded` file (CLI) — never trap. Failure behavior: exit 1 + reason, always with fallback instructions.

## Migration / Rollout

No migration. Two chained PRs per 400-line budget: PR1 Cut 1 (Go, ~150–200 lines incl. tests), PR2 Cut 2 (JS, ~60–90 lines incl. tests). Rollback: revert verb registration (PR1) / restore allow-only `session_stop` (PR2); orphan `session-fallback.md` files are harmless.

## Open Questions

- [x] Default `--change "session-close"` acceptable, or derive from active SDD change? → Decided Cut 1: keep default (deriving active change out of scope).
- [x] 250ms CLI timeout final, or measure `biggz` cold-start on target machines first? → Measured 2026-09-07: 274–289ms cold-start on win32, so finalized at **1000ms** (~3x headroom; close stays bounded, timeout degrades to allow).
- [x] Confirm ESM import path between the two pi assets at install/sync time → Confirmed: static `import ... from "./biggz-tool-interception.js"`, precedent `biggz-footer.js → biggz-extension-api.js` (jiti resolves); one-way direction, no cycle.
