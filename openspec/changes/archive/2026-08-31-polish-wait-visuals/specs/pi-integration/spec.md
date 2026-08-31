# Delta for pi-integration

## ADDED Requirements

### Requirement: POLISH-PI-01 — Throttled Wait via Shim

The system MUST provide shim `internal/assets/pi/biggz-pi-pretty.js` wrapping `asyncWaitUpdate`/`detachedForegroundWaitUpdate` throttled to 3s (prev 1s). When `subagent_wait` waiting with 2-4 runs, it MUST render headline 1 line `Wait 23s · 2 runs (sdd-apply running, sdd-verify queued) — open Fleet for detail` plus optional 1 dim line, total ≤2 lines, collapsing subagent output via `compactResultMaxLines:20` in `subagent-config.json`, and MUST NOT dump full `formatAsyncRunList`.

#### Scenario: Throttle 3s suppresses churn
- GIVEN wait invoked at t0 and t0+1.5s
- WHEN second `asyncWaitUpdate` fires
- THEN shim MUST suppress second render until 3s elapsed

#### Scenario: Headline replaces full list
- GIVEN 3 runs waiting, elapsed 23s
- WHEN shim renders
- THEN output MUST be 1-line headline + optional dim hint (≤2 lines) not full list

#### Scenario: Config collapses output
- GIVEN `subagent-config.json` with `compactResultMaxLines:20`
- WHEN subagent returns long output
- THEN shim MUST collapse beyond 20 lines

### Requirement: POLISH-PI-02 — ADR Upstream and Fallback Strategy

The system MUST document ADR `docs/adr/xxx-pi-subagents-wait.md` evaluating `fork` vs `vendor` vs `extension shim` vs `config` tradeoffs, select `shim biggz-pi-pretty.js` as immediate fallback, and propose PR to `nicobailon/pi-subagents` upstream. Shim MUST be flag-gated revertible in 1 file.

#### Scenario: ADR covers four options
- GIVEN ADR file exists
- WHEN inspected
- THEN it MUST contain sections for fork, vendor, shim, config with tradeoffs and decision for shim+PR

#### Scenario: Shim revertible single file
- GIVEN shim enabled
- WHEN flag disabled or `git revert HEAD`
- THEN wait behavior MUST revert to vendor default with single-file change
