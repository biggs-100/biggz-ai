# Design: pi-subagent-runtime

## Technical Approach

Own pi extension pack `internal/assets/pi/biggz-subagent-runtime.js` (deployed by `PiExtensionsStep`): the `subagent*` contract over one `pi --mode rpc` child per task, strict JSONL, watchdog + tree kill, background delivery, rendering only via `@earendil-works/pi-tui`. S0 pins j0k3r; S3 retires it after doctor + harness pass.

## Resolved Decisions (proposal Q1–Q7)

| # | Resolution |
|---|---|
| 1 RPC subset | Commands `prompt`, `steer`, `abort`, `get_last_assistant_text`, `extension_ui_response`; events `agent_start`, `message_update`, `tool_execution_start/end`, `agent_settled`, `auto_retry_end`, `extension_ui_request`, `extension_error`; rest ignored. rpc is the only mode with a dialog channel |
| 2 Widget | `ctx.ui.setWidget("biggz-subagents", rows, { placement: "belowEditor" })` (pi 0.85.1 `ExtensionWidgetOptions`, `types.d.ts:43-47,97`): one row `◐ <agent> · <state> · <elapsed>s` per background run, max 2 + `… +N`, hidden when idle / `PI_SUBAGENT_CHILD=1` / `BIGGZ_PRETTY=0`; rows `truncateToWidth`'d. No panel/overlay |
| 3 History | CUT to in-memory only: bounded per-session ring of task records (state, events, final text); `subagent_result` = ring lookup, unknown/expired id → bounded error. No on-disk `~/.biggz/subagents/tasks/`, no pruning, no BigMem — proposal scopes history out and no delta scenario mandates persistence (spec only requires `subagent_result` callable) |
| 4 Pin / removal | Exact `npm:pi-subagents-j0k3r@1.6.1` in `InstallCommand`, `desiredPiPackages`, remedy; update exact-match assertion `subagents_fork_test.go:22` (`…j0k3r` → `…j0k3r@1.6.1`). Retire at S3 after doctor PASS + harness green; rollback = revert cutover + reinstall. Upstream #25: S0 evidence comment + patch offer, non-blocking |
| 5 Marker | `~/.pi/agent/extensions/biggz-subagent-runtime.js` (`PI_CODING_AGENT_DIR` honored); owned by `internal/sdd` (`SubagentRuntimeTargetName`, `SubagentRuntimeMarkerPath`, `SubagentRuntimeCapability`), consumed by install step, doctor, `agents/pi`, probe. Capability = marker ∧ j0k3r absent from settings `packages` (else `ready` would lie) |
| 6 Reconciliation | See Reconciliation section (memory-chrome, dead funcs, Fleet suffix, spec size) |
| 7 Schemas | See Interfaces (`{agent,task,context,mode}` semantics) |

## Architecture Decisions

| Decision | Choice + rationale |
|---|---|
| Child protocol | `--mode rpc --no-session`; agent body + task ride the first `prompt` message (Windows argv limits); `--tools`/`--model` gate and route |
| Registration gate (dual-registration) | One chain, each link `typeof`-guarded: (1) `pi.getAllTools()` (`ExtensionAPI`, `types.d.ts:997`) scan for `subagent_run`/`subagent_list_*` → present ⇒ register nothing; (2) else `pi.getToolDefinition("subagent_run")` (runner `runner.d.ts:119`; pattern `biggz-synthesis-gate.js:53-58`, `biggz-quiet-tools.js:176-183`); (3) else settings `packages` j0k3r check (`PI_CODING_AGENT_DIR` honored); (4) unprovable/throws ⇒ fail-closed: no registration + logged reason. Never calls `pi.getTool` (absent in 0.85.1) |
| Concurrency cap | Default 2 concurrent background runs; positive-integer `BIGGZ_BACKGROUND_SUBAGENTS` overrides (clamped ≥1); `on`/`off` keep existing four-source policy semantics (Go resolution unchanged); excess queues FIFO in `queued` state, auto-starts when a slot frees |
| Completion card | One `biggz-subagent-completion` renderer emitting one bounded line from exported pure `renderCompletion` |
| Test width oracle | `--import` resolver maps `@earendil-works/pi-tui` → pi install (`PI_TUI_DIR` override) else test-only oracle + fidelity test |

## Data Flow

```
subagent{agent,task,mode} ─ gate: getAllTools() has no subagent_run ∧ settings packages lack j0k3r
 └ AgentFile ~/.pi/agent/agents/{agent}.md → {tools, model, body}
   └ spawn pi --mode rpc --no-session --tools … (env PI_SUBAGENT_CHILD=1)
     │ stdin: prompt | steer | abort | extension_ui_response
     └ stdout JSONL ─ reader (LF only, CR strip, 1 MiB cap)
        ├ message_update / tool_execution_* → progress; extension_ui_request → parent UI → response
        └ agent_settled → get_last_assistant_text → TaskStore{queued|running|completed|failed|stalled|cancelled}
 foreground → inline result   background → notify + widget + transcript
```

## Interfaces / Contracts

| Tool | Params (required `*`) | Returns |
|---|---|---|
| `subagent` | `agent*`, `task*`, `context`=`fresh` (`fork`→bounded unsupported error), `mode`=`task`\|`background` | foreground result + task id; background ids immediately |
| `subagent_wait` | `task_ids?`, `timeout_ms?` | `renderWaitHeadline(runs, elapsed)` ≤2 lines, no run-list dump |
| `subagent_status` | `task_id?` (omit → active) | state + elapsed + last activity |
| `subagent_result` | `task_id*`, `max_lines?`=20 | final text |
| `subagent_cancel` | `task_id*` \| `all` | tree killed, `cancelled` |
| `subagent_agents` | — | id, description, tools, model? |

Spawn rules: `--tools` from frontmatter (missing → read-only `read`); `subagent*` refused on `PI_SUBAGENT_CHILD=1`; unknown agent → bounded error; background beyond cap 2 → `queued` FIFO, auto-start on slot free; cancel of queued removes pre-spawn; watchdog idle 4 min / total 30 min → `stalled`; kill = `abort` → SIGTERM group → SIGKILL (Windows `taskkill /PID /T /F`).

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/assets/pi/biggz-subagent-runtime.js` | Create | Gate, discovery, runner, JSONL, tools, widget, completion renderer |
| `internal/assets/pi/biggz-subagent-runtime.test.mjs` | Create | Runner/JSONL/gate/headline/dialog (fake child) |
| `internal/assets/pi/test/pi-tui-resolver.mjs` + `test/pi-tui-oracle.mjs` + `biggz-subagent-width.test.mjs` | Create | Specifier hook, oracle, width harness |
| `internal/install/steps/pi_extensions.go` | Modify | Runtime in, wait-pretty out, stale set, config deploy retired |
| `internal/agents/pi/adapter.go` | Modify | Pin, reconcile drop, capability delegate |
| `internal/sdd/background.go` | Modify | Marker + capability owner |
| `internal/doctor/pi.go` | Modify | Marker check, remedy `biggz install --agent pi` |
| `internal/install/install.go` | Modify | Delete 4 dead deploy funcs + helper |
| `internal/install/steps/helpers.go` | Modify | Delete `mergeJSONCWrapper` — dead once `deploySubAgentConfig` retires (sole callers `pi_extensions.go:447-452`) |
| `internal/assets/biggz/biggz-orchestrator-delegation.md` | Modify | Tool names, fallback, no Fleet/`subagent_run`/native-task; remove `context:"fork"` guidance (line 46) |
| `internal/assets/pi/biggz-wait-pretty.js`, `subagent-config.json` | Delete | Retired with j0k3r |
| Guard/factory/adapter/doctor/install tests | Modify | JS count stays 13 (swap), pin/reconcile/probe/cutover; `subagents_fork_test.go:22` exact-match for `@1.6.1` |

## Threat Matrix

| Boundary | Applicability | Response | RED tests |
|---|---|---|---|
| Spawn authorization | Applicable | frontmatter `--tools`; read-only default; nested spawn refused | missing frontmatter → read-only; nested spawn errors |
| Process-tree kill | Applicable | abort → SIGTERM group → SIGKILL; `taskkill /T /F` | child + grandchild dead after cancel (win+posix) |
| `PI_SUBAGENT_CHILD` | Applicable | set on spawn, never inherited; child chrome bypass | child env assertion |
| JSONL parsing bounds | Applicable | LF split, CR strip, 1 MiB cap, malformed skipped+logged | partial, CR, U+2028-in-string, oversized fixtures |
| Rendering width | Applicable | every line `truncateToWidth`; `visibleWidth` asserted | two-✅ card at width 188 |
| Dual registration | Applicable | chain: `getAllTools()` no `subagent_run` → `getToolDefinition("subagent_run")` → settings `packages` j0k3r; unprovable ⇒ no registration | j0k3r-present fake pi (`getAllTools` lists `subagent_run`) → zero registrations; chain-degraded fake → fail-closed |
| Doc paths / `git -C` / commit / push / PR | N/A | no git, PR, or exec-classification boundary | — |

## Testing Strategy

| Layer | What | Approach |
|---|---|---|
| JS unit | property `measure ≥ piTui.visibleWidth`, wrap ≤ width, exact `w=190>188` two-✅ fixture | `node --test` + resolver |
| JS unit | exact `Wait 23s · 2 runs (sdd-apply running, sdd-verify queued)`, ≤2 lines, bounded completion card | pure renderers, widths 20–200 |
| JS unit/integration | JSONL fuzz, stall kill, cancel tree, dialog round-trip, steer, gate | fake child over RPC JSONL |
| JS unit/integration | cap default 2; numeric `BIGGZ_BACKGROUND_SUBAGENTS` override (1 → third queues; 4 → four run; `on`/`off` → default 2); queue excess: third launch `queued` FIFO → auto-start on slot free (spec "Cap queues excess runs") | fake child over RPC JSONL |
| Go unit | deploy list + stale + factory 13 + marker agreement; pin exact; cutover reconcile/rollback; capability | `go test ./internal/install/... ./internal/agents/pi ./internal/sdd ./internal/doctor` |
| Smoke | emoji-heavy delegated SDD phase, zero crash-log writes; background two-task notice | apply-report evidence |

## Migration / Rollout

S0 pin + upstream #25 comment (~60) → S1a runner+JSONL+kill (~350) → S1b tools+card+resolver (~360) → S2 background+widget+wait+dialog+cap (~330) → S3 cutover: settings reconcile, deploy/stale retarget, dead funcs, prompts, doctor/probe (~350) → S4 width harness (~220). Auto-chain: 6 slices, every slice ≤400 authored lines, dependency-ordered, revertible.

## Reconciliation (spec fixes before apply)

1. `pi-deploy-list` delta: drop `biggz-memory-chrome.js` from the MUST-NOT list and stale-removal set (stale = `synthesis-gate` + `wait-pretty`); JS count stays 13 via swap.
2. `agent-install` delta: MODIFIED REQ-INST-001 naming `PiExtensionsStep.deploySubAgents` as the live path.
3. Orchestrator delta: delegation-doc edits at lines 37, 44–48, 145; no other asset keeps those strings once wait-pretty is deleted.

## Open Questions

None blocking. Deferred: `context:"fork"` (rejected with bounded unsupported error; delegation-doc guidance removed), `subagent_send_message`/`continue`, model-profiles UI, `/biggz-agents` panel.
