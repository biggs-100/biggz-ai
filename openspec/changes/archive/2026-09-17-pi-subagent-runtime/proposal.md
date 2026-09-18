# Proposal: pi-subagent-runtime

## Intent

`pi-subagents-j0k3r@1.6.1` (floating) measures width in code points, pi-tui in terminal cells: emoji/CJK cards overflow and pi's fail-closed guard hard-kills the session (crash 2026-09-17; upstream #25 open; 4+ width helpers; not externally patchable). Biggz prompts cite `subagent`/`subagent_wait`, unregistered in j0k3r 1.6.1.

Build a minimal own runtime, safe by construction via pi-tui width primitives, serving SDD delegation (foreground/background, status/result/cancel); pin j0k3r meanwhile, retire after cutover.

## Scope

### In Scope (MVP S0–S4, exploration.md)
- **S0** Pin `pi-subagents-j0k3r@1.6.1` (`internal/agents/pi/adapter.go`), doctor remedy, upstream #25 engagement.
- **S1** Runtime core: extension pack, `subagent` task-mode, one `pi --mode rpc` child per task, strict JSONL, stall watchdog, kill, pi-tui-only rendering.
- **S2** `mode:"background"` + completion notification; `subagent_wait`/`status`/`result`/`cancel`; ≤2 concurrent via `BIGGZ_BACKGROUND_SUBAGENTS`.
- **S3** Cutover: register only when j0k3r absent; retire `biggz-wait-pretty.js` and j0k3r config; retarget installer/doctor/probes; update prompts; drop dead deploy funcs.
- **S4** Width regression harness: `node --test` fixtures (`✅`, CJK, ZWJ, ANSI), property `measure(line) >= piTui.visibleWidth(line)`.

### Out of Scope
Gentle parity (overlay/history/thread), chains, profiles UI, cross-session transports (incl. Windows named pipes), `send_message`/`continue`, fork/vendor/publish, SDD/BigMem semantics.

## Capabilities

### New
- `pi-subagent-runtime`: `subagent`/`subagent_wait` (+status/result/cancel/agents) tools, child lifecycle, background delivery, safe rendering, regression harness.

### Modified
- `runtime`: background capability probe retargeted from `pi-subagents` package to runtime marker.
- `pi-integration`: j0k3r wait-shim/pretty requirements (POLISH-PI-01/02) reworked or retired.
- `orchestrator`: delegation tool names; wait headline re-homed.
- `pi-deploy-list`: deploy entries + factory-test count.
- `agent-install`: pin (S0), j0k3r retirement (S3).

## Approach
- Own extension pack in `internal/assets/pi/`, deployed via existing `PiExtensionsStep`.
- One `pi --mode rpc` child per task (confirmed: child `ask_user_question` + steering parity); strict JSONL; `PI_SUBAGENT_CHILD=1`.
- Render exclusively through pi-tui `visibleWidth`/`truncateToWidth`/`wrapTextWithAnsi`; minimal status widget replaces FleetView.
- No dual registration: gate on j0k3r absence; contract names stay `subagent`/`subagent_wait`.
- Delivery: auto-chain, 400-line review budget; tasks forecast slices.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `internal/assets/pi/` | New/Modified | Runtime pack; retire wait-pretty |
| `internal/install/steps/pi_extensions.go` | Modified | Deploy list + factory test |
| `internal/agents/pi/adapter.go` | Modified | Pin, reconcile, probe |
| `internal/doctor/pi.go` | Modified | Check/remedy retarget |
| `internal/sdd/background.go` | Modified | Probe source |
| `internal/assets/biggz/biggz-orchestrator-delegation.md` | Modified | Tool names, fallback |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| pi RPC/API churn | Med | Pin pi-tui; defensive parsing |
| Dialog relay parity | Med | RPC mode; round-trip test |
| Dual registration | Med | j0k3r-absent gate |
| Over-ownership creep | Med | Out-of-scope list enforced |
| Gentle MIT/trademark | Low | Independent implementation |

## Rollback Plan
- S0: revert pin line; reinstall restores floating.
- S1–S2: behind coexistence guard; remove deploy entry + reinstall disables.
- S3: j0k3r stays pinned until runtime passes doctor + S4 fixtures; revert cutover commit + reinstall restores it.

## Dependencies
pi ≥ 0.85.1 (`--mode rpc`, pi-tui primitives); upstream #25 monitored (non-blocking).

## Success Criteria

| # | Criterion | Check |
|---|---|---|
| 1 | Crash class eliminated | Emoji-heavy delegated run completes; S4 fixtures pass |
| 2 | Contract served | Foreground/background + status/result/cancel verified |
| 3 | Safe by construction | No width math outside pi-tui |
| 4 | Cutover complete | Doctor PASS on runtime marker; j0k3r retired; prompts match |
| 5 | Rollback proven | Revert + reinstall restores pinned j0k3r |

## Open Questions (deferred to design)
RPC subset; widget scope; history format; pin range/removal criterion.
