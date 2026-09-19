# Proposal: subagent-parity-tools

## Intent

Close two tool-surface gaps vs gentle-shell (`gentle-pi@3.1.0`, refs `extensions/gentle-agents.ts:1273`, `:1288`); the biggz runtime already owns the machinery (issue #138, approved):

1. `subagent_list_tasks` — today callers only see active runs (`subagent_wait`, `subagent_status` without id) or one id (`subagent_status`); the ring is unreachable.
2. `subagent_send_message` (`task_id`, `message`) — `task.steer(message)` exists (`biggz-subagent-runtime.js:539-541`) and is test-covered (`biggz-subagent-runtime.test.mjs:848-855`), but no tool forwards it.

`createSubagentToolset` registers exactly six tools (`biggz-subagent-runtime.js:861-962`; asserted at `biggz-subagent-runtime.test.mjs:632,672`).

## Scope

### In Scope
- `subagent_list_tasks`: session ring records, newest first, bounded by `TASK_RING_LIMIT` 50 (`biggz-subagent-runtime.js:631`).
- `subagent_send_message`: steer a running task via `task.steer(message)`; bounded errors for unknown/expired or settled tasks.
- Delta spec plus runtime tests.

### Out of Scope
- Changes to the existing six tools or delegation prompts/docs.
- New RPC features, cross-session transports, `subagent_continue`.
- Gate rework beyond legacy-name reconciliation.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `pi-subagent-runtime`: `Tool Surface and Registration Gate` (`openspec/specs/pi-subagent-runtime/spec.md:9`) — add both companions; reconcile the legacy clause, since `subagent_list_tasks` matches `LEGACY_TOOL_RE`'s `subagent_list_*` pattern (`biggz-subagent-runtime.js:557`) that the requirement forbids. Delta: `openspec/changes/subagent-parity-tools/specs/pi-subagent-runtime/spec.md`.

## Approach

- Extend `createSubagentToolset` via existing seams: `registry.all()` reversed for newest-first; state-checked `task.steer(message)` with bounded errors.
- Gate runs before own registration (no self-detection at load); design decides whether `LEGACY_TOOL_RE` narrows to exact j0k3r names.
- Tests: update both tool-name assertions; add list/steer/error tests.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `internal/assets/pi/biggz-subagent-runtime.js` | Modified | Two tool definitions (~50 LOC) |
| `internal/assets/pi/biggz-subagent-runtime.test.mjs` | Modified | Assertions + behavior tests (~40 LOC) |
| `openspec/changes/subagent-parity-tools/specs/pi-subagent-runtime/spec.md` | New | Delta requirement |
| `openspec/specs/pi-subagent-runtime/spec.md` | Modified (archive) | Merge delta |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Legacy-name collision | High | Delta carves out the name; design decides regex narrowing |
| `steer` false between lookup and call | Med | State check + bounded error |
| Ring evicts settled records | Low | Document bounded 50 |
| Review creep | Low | ~80 LOC + tests |

## Rollback Plan

Revert the single commit; no persisted state or migrations. Reinstall redeploys the previous build.

## Dependencies

Issue #138 approved; gentle refs are external read-only evidence. No new dependency.

## Success Criteria

- [ ] `subagent_list_tasks` returns ring records newest first, bounded to 50, settled included, bounded empty state.
- [ ] `subagent_send_message` forwards a steer to a running task (fake-child test proves the RPC line); unknown/expired/settled return bounded errors.
- [ ] Six existing tools unchanged; runtime `node --test` + width harness and Go gates green.
- [ ] Delta spec merges the requirement with the legacy clause reconciled.
