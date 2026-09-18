# Delta for pi-integration

## REMOVED Requirements

### Requirement: POLISH-PI-01 — Throttled Wait via Shim

(Reason: the shim wrapped j0k3r-only hooks/events (`asyncWaitUpdate`, `detachedForegroundWaitUpdate`); with the own runtime serving delegation, the wait headline is owned natively by `subagent_wait` and the shim dies with the plugin.)
(Migration: headline contract re-homed to `pi-subagent-runtime` (Background Mode requirement); retire `internal/assets/pi/biggz-wait-pretty.js` and its deploy/stale self-heal references.)

### Requirement: POLISH-PI-02 — ADR Upstream and Fallback Strategy

(Reason: the requirement's selected fallback was the shim, which is retired with the j0k3r coupling; the documented tradeoff space no longer matches the runtime-ownership decision.)
(Migration: rollback is covered by this change's rollback plan (revert + reinstall restores pinned `pi-subagents-j0k3r@1.6.1`); any successor ADR is a `pi-subagent-runtime` artifact.)
