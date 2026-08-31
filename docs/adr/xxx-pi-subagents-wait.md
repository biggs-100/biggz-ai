# ADR xxx: Pi Subagents Wait Visuals — Shim vs Fork/Vendor/Config

Date: 2026-08-31
Status: Accepted
Context: FleetView pi-subagents flat rows, repeated metrics, no hierarchy, 1s jitter. Need -70% wait noise keep diagnostics, fixed cols 5c/10c, 2-line rows, header 2 groups, throttle 1s→3s, headline ≤2 lines.

## Decision

Use **extension shim** `internal/assets/pi/biggz-pi-pretty.js` as immediate fallback, gate via `BIGGZ_PRETTY=0`, single-file revert, and propose upstream PR to `nicobailon/pi-subagents`.

## Options Considered

### 1) Fork `nicobailon/pi-subagents`

- **How**: Fork repo, patch wait loop + FleetView rendering, publish `npm:pi-subagents-fixed`, point biggz installer to fork.
- **Pros**: Full control, can rewrite 1s→3s, headline, token compact, remove flat rows.
- **Cons**: Drift from upstream, maintenance burden (rebase on each upstream release), package trust + audit, doubles install surface, rollback requires package swap not 1-file.
- **Tradeoff**: Power vs maintenance; upstream may reject forked divergence, community fragmentation.

### 2) Vendor (copy upstream into biggz-ai)

- **How**: Copy `pi-subagents` source into `vendor/pi-subagents-fixed`, import locally, mutate directly.
- **Pros**: Deterministic build, no external drift at release time, quick patch.
- **Cons**: Large vendored diff (~10k lines), blocks upstream fixes, license attribution, duplication PR cannot show clean diff, rollback still 1-dir but heavy.
- **Tradeoff**: Determinism vs freshness; useful as fallback only if shim cannot reach API.

### 3) Extension shim `biggz-pi-pretty.js` (Chosen)

- **How**: Wrapper Pi ExtensionAPI (`pi.on` + method wrap `asyncWaitUpdate`/`detachedForegroundWaitUpdate`), debounce 3s, headline `Wait 23s · 2 runs (…) — open Fleet for detail` 1 solid +1 dim, ≤2 lines, suppress `formatAsyncRunList` dump, reuse `subagent-config.json` `compactResultMaxLines:20` to collapse >20.
- **Pros**: No fork, no vendor, 1-file `git revert` + flag `BIGGZ_PRETTY=0` disables shim, small diff (~80 lines), testable throttle mock, survives upstream releases unless API breaks.
- **Cons**: Depends on ExtensionAPI stability; if upstream renames hooks, shim silently no-ops (degrades to 1s vendor behavior, not crash). Requires defensive multi-event wrapping.
- **Tradeoff**: Minimal maintenance, immediate fallback, reversible. Chosen as immediate path.

### 4) Config only (`subagent-config.json`)

- **How**: Tune `compactResultMaxLines:100→20`, horizontalSpacing, no code.
- **Pros**: Zero code, collapses verbose subagent output >20 lines, already supported.
- **Cons**: Does not fix 1s jitter, flat rows, repeated metrics, nor headline ≤2 lines alone; needs shim for throttle/headline.
- **Tradeoff**: Necessary but not sufficient; combined with shim.

## Consequences

- **Shim** implements: `THROTTLE_MS=3000`, `lastRender` debounce, headline 1+1 dim via `formatHeadline(elapsed, runs)`, suppress duplicate until 3s elapsed, never dumps full list. Checked at `pi.asyncWaitUpdate`/`detachedForegroundWaitUpdate` + `pi.on` events for load-order safety.
- **Config** `compactResultMaxLines:20` collapses subagent tool output beyond 20 lines (verified via `DeployPiSubagentConfig` merge).
- **Flag** `BIGGZ_PRETTY=0` disables shim (early return), revert is `git revert HEAD` single file.
- **PR** to `nicobailon/pi-subagents` proposes upstream 3s default + headline contract; shim stays as fallback until merged.
- **Rollout**: No migration, presentational only; `go vet` + `node --test biggz-synthesis-gate` + `go test -run TestSanitize/TestSynthesis` remain green.

## Alternatives Rejected

- Direct `pi-subagents` edits (violates vendor boundary, not rollback-clean).
- Dynamic 1–3 lines (layout shift, scroll overflow vs fixed `height=2`).

## Links

- Design: `openspec/changes/polish-wait-visuals/design.md`
- Specs: `specs/tui-sanitize`, `tui`, `orchestrator`, `pi-integration` (14 req 31 scenarios)
- Shim: `internal/assets/pi/biggz-pi-pretty.js`
- Config: `internal/assets/pi/subagent-config.json`
