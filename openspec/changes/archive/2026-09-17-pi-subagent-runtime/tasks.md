# Tasks: pi-subagent-runtime

## Review Workload Forecast

| Slice | Scope | Est. authored lines |
|-------|-------|---------------------|
| S0 | Pin j0k3r `@1.6.1` + upstream #25 evidence | ~60 |
| S1a | Runtime core: discovery, RPC child, JSONL, watchdog, kill | ~350 |
| S1b | Registration gate, tool surface, completion card, pi-tui resolver | ~360 |
| S2 | Background delivery, widget, `subagent_wait`, dialog relay, cap | ~330 |
| S3 | Cutover: deploy/reconcile retarget, dead funcs, prompts, doctor/probe | ~350 |
| S4 | Width regression harness + smoke | ~220 |
| **Total** | 6 slices, each ≤400 authored lines | **~1,670** |

| Field | Value |
|-------|-------|
| Estimated changed lines | ~1,670 authored (additions + deletions) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → PR 2 → PR 3 → PR 4 → PR 5 → PR 6 |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Ledger `--work-unit` | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|----------------------|------|-----------|----------------------|-----------------|-------------------|
| S0 | `S0-pin` | j0k3r pinned `@1.6.1` in install, reconcile, remedy | PR 1 | `go test ./internal/agents/pi ./internal/doctor -count=1` | `pi install` re-run idempotent; settings show `@1.6.1` | revert pin + remedy lines; reinstall restores floating |
| S1a | `S1a-runner` | one RPC child per task, strict JSONL, stall kill | PR 2 | `node --test internal/assets/pi/biggz-subagent-runtime.test.mjs` | fake child over RPC JSONL; child + grandchild dead after kill | revert runtime asset sections + tests (asset not yet deployed) |
| S1b | `S1b-tools` | gate, `subagent` task-mode + companions, card, resolver | PR 3 | `node --test internal/assets/pi/biggz-subagent-runtime.test.mjs` | fake pi load: j0k3r-present → zero registrations | revert gate/tool/card hunks + resolver files |
| S2 | `S2-background` | background ids/notice, widget, wait, dialog, cap 2 | PR 4 | `node --test internal/assets/pi/biggz-subagent-runtime.test.mjs` | two background tasks E2E; widget rows; Windows cancel kills tree | revert background/widget/cap hunks |
| S3 | `S3-cutover` | j0k3r retired, dead code gone, probes retargeted | PR 5 | `go test ./internal/install/... ./internal/agents/pi ./internal/sdd ./internal/doctor -count=1` | temp-HOME `biggz install --agent pi` → doctor PASS | revert cutover + reinstall restores pinned j0k3r |
| S4 | `S4-harness` | width regression harness + smoke evidence | PR 6 | `node --test internal/assets/pi/biggz-subagent-width.test.mjs` | emoji-heavy delegated SDD phase, zero crash-log writes | revert harness files only |

Evidence shorthands: `js:runtime`/`js:factory`/`js:width` = `node --test internal/assets/pi/{biggz-subagent-runtime,biggz-pi-extensions-factory,biggz-subagent-width}.test.mjs`; `go:pi` = `go test ./internal/agents/pi -count=1`; `go:install` = `go test ./internal/install/... -count=1`; `go:sdd` = `go test ./internal/sdd -count=1`; `go:doctor` = `go test ./internal/doctor -count=1`.

## Phase 1 — S0: Pin j0k3r (~60, WU `S0-pin`)

- [x] 1.1 `internal/agents/pi/adapter.go:145` + `:323`: pin `npm:pi-subagents-j0k3r` → `npm:pi-subagents-j0k3r@1.6.1` (exact, never floating). Evidence: `go test ./internal/agents/pi -run TestInstallCommand_UsesJ0k3rFork -count=1`.
- [x] 1.2 `internal/agents/pi/subagents_fork_test.go:22`: exact-match assertion updated to `pi install npm:pi-subagents-j0k3r@1.6.1`; add desired-packages pin assertion. Evidence: `go:pi`.
- [x] 1.3 `internal/doctor/pi.go:187` (`PiSubagentsCheck.Remedy()`): remedy installs the pinned identity. Evidence: `go test ./internal/doctor -run TestPiSubagents -count=1`.
- [x] 1.4 Upstream #25: post/record the engagement comment (crash evidence + verified patch offer) in the change evidence trail; non-blocking, no code.

## Phase 2 — S1a: Runner, JSONL, kill (~350, WU `S1a-runner`)

- [x] 2.1 RED `internal/assets/pi/biggz-subagent-runtime.test.mjs`: fake-child harness + JSONL fuzz — partial line, CR strip, U+2028-in-string, malformed line skipped+logged, 1 MiB cap/oversized fixture. Evidence: `js:runtime`.
- [x] 2.2 GREEN `internal/assets/pi/biggz-subagent-runtime.js`: strict JSONL reader — LF-only split, no `readline`, bounded 1 MiB buffer; malformed/partial logged and skipped; session survives.
- [x] 2.3 RED spawn authorization: `--tools` from frontmatter; missing frontmatter → read-only `read`; child env assertion `PI_SUBAGENT_CHILD=1` set on spawn only; nested spawn refused; unknown agent → bounded error naming available agents.
- [x] 2.4 GREEN agent discovery (`~/.pi/agent/agents/*.md`, frontmatter `tools`/`model`) + spawn `pi --mode rpc --no-session --tools …` with `PI_SUBAGENT_CHILD=1`.
- [x] 2.5 RED watchdog/kill: idle 4 min / total 30 min → `stalled`; kill = `abort` → SIGTERM group → SIGKILL (Windows `taskkill /PID /T /F`); child + grandchild dead after cancel (win+posix); session alive.
- [x] 2.6 GREEN watchdog + process-tree kill escalation.

## Phase 3 — S1b: Gate, tools, card, resolver (~360, WU `S1b-tools`)

- [x] 3.1 RED dual-registration threat: fake pi `getAllTools()` lists `subagent_run` → zero registrations; j0k3r in settings `packages` → zero; chain-degraded fake (throws) → fail-closed, no registration + logged reason.
- [x] 3.2 GREEN registration gate: one `typeof`-guarded chain `getAllTools()` → `getToolDefinition("subagent_run")` → settings `packages` → fail-closed; never `pi.getTool`.
- [x] 3.3 RED `subagent` task-mode: returns result + task id; `context` default `fresh`, `fork` → bounded unsupported error; `mode` default `task`; unknown agent bounded.
- [x] 3.4 GREEN tool surface: `subagent`, `subagent_status`, `subagent_result`, `subagent_cancel`, `subagent_agents`; in-memory bounded ring only (no disk, no BigMem).
- [x] 3.5 RED completion card `renderCompletion`: one bounded line, widths 20–200, `truncateToWidth`, exact two-`✅` asset at width 188. Evidence: `js:runtime`.
- [x] 3.6 GREEN `biggz-subagent-completion` message renderer from exported pure `renderCompletion`.
- [x] 3.7 Create `internal/assets/pi/test/pi-tui-resolver.mjs` + `internal/assets/pi/test/pi-tui-oracle.mjs`: `--import` specifier hook resolves `@earendil-works/pi-tui` to the pi install (`PI_TUI_DIR` override) else test-only oracle + fidelity test.

## Phase 4 — S2: Background, widget, wait, dialog, cap (~330, WU `S2-background`)

- [x] 4.1 RED dialog relay: fake child `extension_ui_request` (`ask_user_question`) → parent presents → `extension_ui_response` answer returns, run continues; steering reaches a running child mid-run.
- [x] 4.2 GREEN RPC subset wiring: commands `prompt`/`steer`/`abort`/`get_last_assistant_text`/`extension_ui_response`; events `agent_start`, `message_update`, `tool_execution_start/end`, `agent_settled`, `auto_retry_end`, `extension_ui_request`, `extension_error`; rest ignored.
- [x] 4.3 RED background: ids return immediately; each completion delivered without polling; widget rows `◐ <agent> · <state> · <elapsed>s`, max 2 + `… +N`, `truncateToWidth`; hidden when idle / `PI_SUBAGENT_CHILD=1` / `BIGGZ_PRETTY=0`.
- [x] 4.4 GREEN `mode:"background"` + completion delivery + `ctx.ui.setWidget("biggz-subagents", rows, { placement: "belowEditor" })`.
- [x] 4.5 RED cap: default 2; `BIGGZ_BACKGROUND_SUBAGENTS` numeric override (1 → third queues; 4 → four run; `on`/`off` → default 2; clamp ≥1); third launch `queued` FIFO → auto-starts on slot free; cancel of queued removes pre-spawn.
- [x] 4.6 GREEN cap + FIFO queue (`queued` state); Go four-source policy resolution unchanged.
- [x] 4.7 RED `subagent_wait` headline: exact `Wait 23s · 2 runs (sdd-apply running, sdd-verify queued)` + at most one dim hint, ≤2 lines, no run-list dump.
- [x] 4.8 GREEN `renderWaitHeadline` + `subagent_wait` tool.

## Phase 5 — S3: Cutover (~350, WU `S3-cutover`)

- [x] 5.1 `internal/install/steps/pi_extensions.go:71-93`: deploy list — runtime pack in, `biggz-wait-pretty.js` out; keep `biggz-session-guard.js` + `biggz-memory-chrome.js`. Evidence: `go test ./internal/install/steps -run TestPiExtensions -count=1`.
- [x] 5.2 Stale self-heal `pi_extensions.go:258`: add `biggz-wait-pretty.js` to the removal set (keep `biggz-synthesis-gate.js`); missing files silent no-op. Evidence: `go:install`.
- [x] 5.3 Delete `internal/assets/pi/biggz-wait-pretty.js` + `internal/assets/pi/subagent-config.json`; drop `deploySubAgentConfig` call (`pi_extensions.go:233`) + func (`:419`) + `mergeJSONCWrapper` (`internal/install/steps/helpers.go:83`, sole callers `pi_extensions.go:447-452`).
- [x] 5.4 Dead sweep `internal/install/install.go` — delete `DeployPiWaitPretty:1873`, `DeployPiPrettyWrapper:1971`, `DeployPiSubAgents:2013`, `DeployPiSubagentConfig:1390`: all four verified zero production callers (only test refs in `pi_subagents_test.go`); retarget meaningful coverage to `PiExtensionsStep.deploySubAgents`; no remaining install-flow or test references. Evidence: `go:install`.
- [x] 5.5 `internal/assets/pi/biggz-pi-extensions-factory.test.mjs`: swap wait-pretty → runtime entry; **JS count guard stays 13** (swap semantics). Evidence: `js:factory`.
- [x] 5.6 `internal/sdd/background.go`: add `SubagentRuntimeTargetName`/`SubagentRuntimeMarkerPath`/`SubagentRuntimeCapability`; retarget `ResolveBackgroundSubagentsCapability:266` to marker ∧ j0k3r absent from settings `packages` (`PI_CODING_AGENT_DIR` honored). Evidence: `go:sdd` (marker → ready; no marker → absent; j0k3r alone → absent).
- [x] 5.7 `internal/agents/pi/adapter.go`: cutover reconcile — marker present → drop j0k3r from settings `packages`, `InstallCommand`, `desiredPiPackages`; capability probe delegates to the `internal/sdd` owner (`:858`). Evidence: `go:pi` (cutover removes j0k3r; revert + reinstall restores `@1.6.1`; idempotent).
- [x] 5.8 `internal/doctor/pi.go`: check the runtime marker; remedy `biggz install --agent pi`. Evidence: `go:doctor`.
- [x] 5.9 `internal/assets/biggz/biggz-orchestrator-delegation.md` lines 37, 44–48, 145: cite `subagent`/`subagent_wait`; drop `subagent_run`/native `task` fallback/FleetView/`context:"fork"`; documented unavailable-fallback text.
- [x] 5.10 Gates: `go build ./...` + `go test ./internal/install/... ./internal/agents/pi ./internal/sdd ./internal/doctor -count=1`.

## Phase 6 — S4: Width harness + smoke (~220, WU `S4-harness`)

- [x] 6.1 `internal/assets/pi/biggz-subagent-width.test.mjs`: fixtures emoji `✅`, CJK, Hangul, ZWJ, ANSI/OSC; exact `w=190` vs width `188` two-`✅` crash shape; property `measure(line) >= piTui.visibleWidth(line)`; wrapped output ≤ width. Evidence: `js:width`.
- [x] 6.2 Source scan: width measurement only via pi-tui imports (no independent width math); resolver fidelity against real pi-tui when installed.
- [x] 6.3 Smoke (apply-report evidence): emoji-heavy delegated SDD phase completes with zero `pi-tui-crash.log` writes; background two-task notice rendered.

## Follow-up Candidates (out of scope — NOT tasks of this change)

- `DeployPiWebSearch`/`Result.PiWebSearch` — no production caller (test-only coverage).
- `DeployPiThinkingWrap`, `DeployPiLastModel` — zero production callers.
- Deferred design surface: `context:"fork"`, `subagent_send_message`/`continue`, model-profiles UI, `/biggz-agents` panel.

## Apply / Ledger Notes

- Acquire per work unit: `biggz sdd-attempt acquire pi-subagent-runtime --work-unit <label> --request-id <id>`; labels `S0-pin`, `S1a-runner`, `S1b-tools`, `S2-background`, `S3-cutover`, `S4-harness`; settle before starting the next slice.
- Slices are dependency-ordered (S0 → S1a → S1b → S2 → S3 → S4), each ≤400 authored lines, with the rollback boundary tabulated above; `S1a`–`S2` ship inert (not yet in the deploy list) until `S3`.
- Delivery: auto-chain + 400-line review budget — chained PRs required; no `size:exception` needed.
- Delivery re-cut (orchestrator, 2026-09-17): S1a landed at 672 authored lines (> 400 single-PR budget) → deliver S1a as 2 stacked PRs (2a1: JSONL + discovery + spawn authorization; 2a2: runner + watchdog + tree kill). S1b landed at 688 (> 400) → deliver as 2 stacked PRs (3a: gate + ring + tool surface; 3b: completion card + renderer + resolver/oracle). S2 landed at 511 (> 400) → deliver as 2 stacked PRs (4a: RPC subset + relay/steer + wait headline/subagent_wait; 4b: background delivery + widget + cap/FIFO). S3 landed at ~1614 (+591/−1023, mandated deletions dominate) → deliver as 2 stacked PRs (5a: deploy/stale/reconcile/marker/probe/doctor/doc; 5b: dead-code sweep incl. install.go removals). S4 landed at 237 (≤400) → single PR. **Delivery total: 10 stacked PRs to master (S0, 2a1, 2a2, 3a, 3b, 4a, 4b, 5a, 5b, S4).**

Spec coverage: `agent-install` → S0/S3; `pi-deploy-list` → S3; `runtime` → S3; `orchestrator` → S3; `pi-integration` → S3; `pi-subagent-runtime` → S1a/S1b/S2/S4.
