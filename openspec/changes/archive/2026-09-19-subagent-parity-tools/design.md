# Design: subagent-parity-tools

## Technical Approach

Extend `createSubagentToolset` with two tools over the existing session ring and `task.steer()`: `subagent_list_tasks` reads `registry.all()` newest-first, capped at the ring limit (50), one `formatTaskResult` row each. `subagent_send_message` state-gates `queued`/settled tasks and forwards a steer through `task.steer(message)`, returning bounded errors. `LEGACY_TOOL_RE` narrows to the exact j0k3r names `subagent_run` and `subagent_list_running`, so the runtime's own `subagent_list_tasks` is never legacy. Spawn, kill, and RPC paths are unchanged.

## Architecture Decisions

### Decision: Reconcile legacy detection by narrowing `LEGACY_TOOL_RE` (not renaming the tool)

| Option | Tradeoff | Verdict |
|---|---|---|
| Narrow `/^subagent_run$\|^subagent_list_/` to exact names `/^subagent_run$\|^subagent_list_running$/` | One line; keeps the approved, gentle-shell-parity names; set derived from evidence | **Chosen** |
| Rename the new tool (e.g. `subagent_tasks`) | Avoids the regex edit, but diverges from gentle-shell/issue #138 naming and forces amending the delta spec before apply | Rejected |

Evidence: the gate test (`biggz-subagent-runtime.test.mjs:590-599`) and the delta scenario (`spec.md:80-84`) name exactly `subagent_run` and `subagent_list_running` as genuine j0k3r names; only the `^subagent_list_` prefix over-captures. Residual risk: if j0k3r exposes another `subagent_list_*` name not in evidence, extend the exact-name alternation; the settings-packages proof (`pi-subagents-j0k3r`, fail-closed) and `getToolDefinition("subagent_run")` check remain the primary detectors.

**If rename had been chosen** (not the path taken): a required pre-apply step would be amending `specs/pi-subagent-runtime/spec.md` — the ADDED requirement naming `subagent_list_tasks` and the "Runtime's own list tool is not legacy" scenario — plus both surface assertions. Under narrowing, the delta spec is consistent as written: no amendment required.

### Decision: List reads only the ring, newest-first, capped

**Choice**: `registry.all()` (insertion order) sliced to the newest `registry.limit` records, reversed; rows via `formatTaskResult(task)`.
**Alternatives**: active-only list like `subagent_status` (rejected: settled records are required); disk/BigMem source (rejected: ring-only contract).
**Rationale**: `all()` filters evicted ids; the slice enforces "at most limit" even when live-work overflow temporarily exceeds the ring (`evict` only removes settled records).

### Decision: Send = state gate + `task.steer()`

**Choice**: lookup, then require state `running`/`spawning`, then `task.steer(message)`; false ⇒ bounded error.
**Alternatives**: `steer()` alone (rejected: the stall→kill window can still accept a write and report false success); pre-check only (rejected: stdin can be unwritable while running).
**Rationale**: `steer()` returns false for settled tasks and for queued tasks with no live child; the state gate removes the stalled window.

## Data Flow

```
subagent_list_tasks ── registry.all().slice(-limit).reverse() ── formatTaskResult rows
subagent_send_message ── registry.get(task_id) ── task.steer(message) ── child stdin {"type":"steer"}
                                          └─ unknown/non-running ⇒ toolError (bounded)
```

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/assets/pi/biggz-subagent-runtime.js` | Modify | Narrow `LEGACY_TOOL_RE` (:557); add `listTasks`/`sendMessage` defs after `agentsTool` (:951-961); return 8 tools (:962) |
| `internal/assets/pi/biggz-subagent-runtime.test.mjs` | Modify | Surface assertions 6→8 (:632, :672); list/steer/error/gate tests |
| `openspec/changes/subagent-parity-tools/design.md` | Create | This document |
| `openspec/changes/subagent-parity-tools/specs/pi-subagent-runtime/spec.md` | Unchanged | Consistent with narrowing; merged at archive |
| `openspec/specs/pi-subagent-runtime/spec.md` | Modified (archive) | sdd-archive merges the delta |

## Interfaces / Contracts

`subagent_list_tasks` — `parameters: paramsOf({})`; result `textResult(rows.join("\n"), { count })`; empty ring ⇒ `"no subagent tasks this session"` (non-error); row = existing `formatTaskResult` (`subagent <id> · <state> · <s>s[ · <reason>]`), settled included; no error path; reads nothing outside the ring.

`subagent_send_message` — `paramsOf({ task_id: string, message: string }, ["task_id","message"])`; success ⇒ `` textResult(`steered ${task.id} · ${state}`, { taskId, state }) ``. Bounded errors (id bounded to 60, state to 20):
- unknown/expired ⇒ `unknown or expired task "<id>"` (reuse `unknownTaskError`)
- queued/settled/stalled ⇒ `task "<id>" is not running (<state>); steer not delivered` — never throws.

Gate order: the factory evaluates `subagentRegistrationGate` before `createSubagentToolset`/`registerTool` (unchanged, :1039-1051). The narrowed regex guarantees our own names never classify as legacy on future loads even if the scan sees them.

## Testing Strategy

| Layer | What to Test | Approach |
|---|---|---|
| Unit | Classification: `subagent_run`/`subagent_list_running` true; `subagent_list_tasks`/`subagent_send_message` false | `LEGACY_TOOL_RE` + `subagentRegistrationGate` assertions |
| Unit | List: newest-first, settled included, empty message, capped at limit | seeded `createTaskRegistry` + `createSubagentToolset({ registry })` |
| Integration | Steer reaches the fake child (`fake_command` steer echo); queued/settled/unknown bounded errors | `toolsetFor` harness (`FAKE_TICK_MS`, cap 1) |
| Integration | 8 tools at both call sites; gate registers when the scan returns our own 8-name surface; legacy still refused | update :632/:672; new `fakePi({ tools: [...] })` cases |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary changes; spawn/kill/RPC paths are untouched. Tool-name gating is covered by the gate tests above.

## Migration / Rollout

No migration. Reinstall redeploys the asset; rollback reverts the single commit (no persisted state).

## Open Questions

None blocking. Residual: the exact j0k3r list-name set is evidence-bound; extend the alternation if new names surface.
