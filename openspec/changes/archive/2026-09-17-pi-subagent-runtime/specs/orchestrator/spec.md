# Delta for orchestrator

## ADDED Requirements

### Requirement: Delegation Tool Naming Contract

`internal/assets/biggz/biggz-orchestrator-delegation.md` MUST cite the registered runtime names — `subagent` (foreground `mode:"task"`, background `mode:"background"`) and `subagent_wait` — and MUST NOT cite unregistered names (`subagent_run`, a native `task` fallback) or retired third-party affordances (`FleetView`, `formatAsyncRunList`). When the runtime is unavailable, the doc MUST carry documented fallback text.

#### Scenario: Delegation doc cites real tools

- GIVEN `biggz-orchestrator-delegation.md` read
- WHEN delegation calls are inspected
- THEN `subagent` and `subagent_wait` MUST be cited and `subagent_run`/native `task` fallback MUST NOT appear

#### Scenario: Retired affordances absent

- GIVEN the same file
- WHEN searched for `FleetView`/`Fleet`
- THEN no reference MUST remain

## REMOVED Requirements

### Requirement: POLISH-ORCH-02 — Wait Headline Data Contract

(Reason: re-homed — the ≤2-line headline is emitted by the own runtime's `subagent_wait`; the orchestrator no longer owns j0k3r-era headline data or `formatAsyncRunList` phrasing.)
(Migration: `pi-subagent-runtime` (Background Mode requirement) owns the bounded headline contract.)
