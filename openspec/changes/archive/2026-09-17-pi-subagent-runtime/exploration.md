# Exploration: pi-subagent-runtime

**Change**: `pi-subagent-runtime`
**Date**: 2026-09-17
**Store**: openspec (file-backed)
**Status**: exploration complete — go/no-go recommendation below

---

## Problem Statement

pi's TUI hard-crashes while rendering a subagent completion card produced by `pi-subagents-j0k3r@1.6.1` (unpinned, floating). The crash is deterministic, kills the whole pi process, and — because the completion message is persisted in the session log — makes the session un-resumable until the extension is patched or the message removed.

### Evidence (verified, not inferred)

| # | Evidence | Detail |
|---|----------|--------|
| E1 | Crash log `~/.pi/agent/pi-tui-crash.log` (2026-09-17T16:16:11.429Z) | `Terminal width: 188`, `Line 741 visible width: 190`; line `[741] (w=190)` is the subagent result card line and contains exactly **2× `✅` U+2705** (verified by codepoint count). Delta = 2 = number of 2-cell graphemes. |
| E2 | pi core guard (fail-closed, by design) | `dist/bundle/chunks/chunk-JVUZSMYM.js:599-601`: `if (!isImage && visibleWidth(line) > width) { ...write crash log...; throw new Error("Rendered line ... exceeds terminal width ... Use visibleWidth() to measure and truncateToWidth() to truncate lines.") }`. The guard protects the terminal; pi core is NOT the defect. |
| E3 | Root cause in the plugin | `src/render/text-width.ts:7-9` `visibleWidth(text) = [...stripAnsi(text)].length` (code points as cells); `wrapLineToWidth` (line 49) counts every non-ANSI token as width 1. All other renderers re-export this one implementation (`src/ui/theme.ts:220-222` re-exports `stripAnsi/visibleWidth/truncateToWidth` from it). `src/render/tools/components.ts:71,108-109` renders the "Subagent result" box via `boxedComponent(..., { wrapped: true })` → `wrapLineToWidth`. |
| E4 | pi-tui measures in cells, not code points | `@earendil-works/pi-tui@0.85.1` `dist/utils.js` imports `eastAsianWidth` from `get-east-asian-width`; exports `visibleWidth`, `truncateToWidth`, `wrapTextWithAnsi` (`utils.d.ts:13,45,75`). |
| E5 | Upstream knows, latest release does not fix it | `j0k3r-dev-rgl/pi-subagents-j0k3r` issue **#25** (open since 2026-09-09, updated 2026-09-17): "TUI crash: subagent completion card pads with grapheme count, so lines with 2-cell graphemes (✅) overflow the terminal". Latest version published = **1.6.1** (2026-09-16), the installed one; no width-fix PR open (`#26` is an unrelated feature). Reporter states a locally verified cell-aware patch exists and offers a PR. |
| E6 | Bug surface is larger than the completion card | Issue #25 comment (2026-09-17): second trigger path = running-subagent **tool row** (`src/render/tools/subagent-run.ts:69` + `boxedComponent`); **three more duplicated naive width helpers** on `main`: `src/render/tools/components.ts` (`visibleTextWidth`), `src/ui/subagents-history-panel.ts`, `src/thread-view.ts` (`terminalVisibleWidth`). No width tests upstream (`test/render/text-width.test.ts` does not exist). |
| E7 | biggz wires the plugin unpinned | `internal/agents/pi/adapter.go:145` (`pi install npm:pi-subagents-j0k3r`), `:323` (`desiredPiPackages`), `internal/install/steps/pi_extensions.go` (config + agents), `internal/doctor/pi.go` (presence check + remedy installs the package), `~/.pi/agent/settings.json:523` (no version). |
| E8 | Prior decision superseded by events | BigMem `obs-1789013349914486900-1` (2026-09-10): "seguir con j0k3r pero pinear versión; no construir runtime propio". The pin was never implemented; the crash materializes the accepted "at the mercy of a third party" risk. User explicitly re-opens the decision (BigMem `obs-1789662550521274800-2`, topic `architecture/subagent-runtime-decision`). |
| E9 | Reference implementation exists | `C:\Users\USER\Desktop\herramientas\gentle-shell` (npm `gentle-pi@3.1.0`, MIT — LICENSE preserves Mario Zechner's notice; trademarks belong to Alan Buscaglia, so any code adaptation requires renaming). It replaces j0k3r with its own runtime and renders **only** through pi-tui width primitives (safe by construction). |

**Why now**: the crash class is not shimmable from the outside (the plugin owns its renderers), upstream has not fixed it in 8 days despite an offered patch, the dependency floats, and pi's guard makes each occurrence catastrophic for the session.

---

## Current Subagent Surface Inventory

### A. Tool surface actually registered today (`pi-subagents-j0k3r@1.6.1`)

`src/tools/registry.ts` registers 8 tools (names verified in `src/tools/*.ts`):

| Tool | Params / notes |
|------|----------------|
| `subagent_run` | `agent`, `task`, `name?`, `display_name?`, `context?`, `mode?` (`task` waits; `background` returns task ids). Primary delegation entry point. `renderShell: 'self'`. |
| `subagent_status` | task snapshot polling |
| `subagent_result` | fetch result/history for a task id |
| `subagent_cancel` | cancel by id / running |
| `subagent_list_tasks` | lists tasks |
| `subagent_list_agents` | lists markdown agents |
| `subagent_send_message` | message a running child |
| `subagent_continue` | only registered when config `enable_continue` |
| (not a tool) | background widget (`setWidget('subagents-claude-background')`), `/subagents` panel, model-profiles UI, `subagent-completion` message renderer, interaction bridging for child `ask_user_question`. |
| (config) | `~/.pi/agent/extensions/subagent/config.json` → deployed from `internal/assets/pi/subagent-config.json` (`inlineToolDisplay: rich`, `fleetView: true`, `fleetViewPlacement: belowEditor`, `asyncWidget: true`, `defaultSubagentContext: fresh`, `compactResultMaxLines: 20`). |

**Naming drift (discovered, must be resolved in design)**: biggz prompts reference a `subagent({ agent, task, context, mode })` call shape and `~/.pi/agent/settings.json` permission keys `subagent`/`task` — but 1.6.1 registers only `subagent_*` names; `subagent_wait` (wrapped defensively by `biggz-wait-pretty.js`) and the "native `task` tool" fallback (delegation doc line 47) do not exist in pi 0.85.1 either. `subagent`/`task` keys are effectively stale.

### B. What biggz actually consumes (owner paths)

| Consumer | Path | What it depends on |
|----------|------|--------------------|
| Delegation contract | `internal/assets/biggz/biggz-orchestrator-delegation.md:37,44-48,145` | Delegation Runtime Preference (pi), foreground `mode:"task"` for SDD phases, `context:"fresh"` default, background only for independent read-only scans, ≤2 concurrent background, completion-notification only (no polling), fallback text if `subagent_run` unavailable |
| Background policy (Go) | `internal/agents/pi/adapter.go:617-905`, `internal/sdd/background.go:240-297` | 4-source fail-closed policy (`background-subagents.json` project > global > `BIGGZ_BACKGROUND_SUBAGENTS` > off) + capability probe (currently: `pi-subagents`/`pi-subagents-j0k3r` package.json presence) |
| Installer | `internal/agents/pi/adapter.go:114-151,314-364,538-615` | `InstallCommand` list, `desiredPiPackages` reconciliation, `legacyPiSubagentPackageIdentities` filter, `removeLegacyPiSubagents` |
| Installer step | `internal/install/steps/pi_extensions.go:62-93,159-386,419-461` | extension deploy list, SDD agents deploy (`~/.pi/agent/agents/sdd-*.md`, 16 files on disk incl. `general`, `explore`), `subagent/config.json` merge |
| Dead legacy deploy funcs | `internal/install/install.go:1873,1971,2013` | `DeployPiWaitPretty`, `DeployPiPrettyWrapper`, `DeployPiSubAgents` — defined but NOT called by the pipeline (only tests); candidates for removal/consolidation |
| Doctor | `internal/doctor/pi.go:14-206` | `PiSubagentsCheck` presence chain + `Remedy` running `pi install npm:pi-subagents-j0k3r` |
| Wait shim | `internal/assets/pi/biggz-wait-pretty.js` (244 LOC) | wraps `pi.asyncWaitUpdate`/`detachedForegroundWaitUpdate` + events `asyncWaitUpdate`, `detachedForegroundWaitUpdate`, `subagent_wait`, `async_wait`; 3s throttle, 2-line FleetRow. **These hooks/events come from j0k3r** — they die with the plugin |
| Pretty wrapper | `internal/assets/pi/biggz-pi-pretty.js` (legacy, not in deploy list), npm `@heyhuynhgiabuu/pi-pretty` (floating, latest 0.6.29; gentle pins 0.6.27) | generic tool-call rendering; NOT subagent-specific |
| Child guards | 10+ biggz extensions (`biggz-extension-api.js:66-70,397`, footer, pills, last-model, session-guard, memory-chrome, quiet-tools, question-mouse, thinking-wrap, wait-pretty, tool-interception) | `PI_SUBAGENT_CHILD=1` bypass contract (set by the dispatcher when spawning children) |
| SDD agents | `~/.pi/agent/agents/*.md` (16 files, frontmatter name/description/tools; sdd-* carry `ask_user_question`, `bash`, `edit`, `write`, `read`) | markdown agent discovery + per-agent tool whitelists + BigMem protocol appendix |

### C. pi-side primitives available for an own runtime (verified in pi 0.85.1)

- **`pi --mode rpc`** — headless JSON protocol over stdin/stdout, strict JSONL (LF delimiter only; `readline` is not compliant), commands (`prompt` with `streamingBehavior: steer`, etc.) + streamed events (`docs/rpc.md`). gentle-shell builds on this.
- **`pi --mode json`** — one-shot structured output; used by pi's official example extension (`examples/extensions/subagent/index.ts`, 944 LOC, tool named `subagent`, single/parallel(max 8, 4 concurrent)/chain modes, spawns one `pi` child per task, streaming, usage stats, agent discovery in `agents.ts`). Lighter than rpc but no interactive dialog channel.
- **`@earendil-works/pi-tui`** — `visibleWidth/truncateToWidth/wrapTextWithAnsi` (cell-aware), `Container/Markdown/Text` components; bundled inside the pi install (`pi-coding-agent/node_modules/@earendil-works/pi-tui`).
- **Extension API** — `registerTool` (+ custom `renderShell`), `registerMessageRenderer`, `ctx.ui.setWidget/setStatus/notify/confirm/onTerminalInput`, `pi.on(...)` events (session_start, tool_call/tool_result, terminal input).
- **Agent discovery** — `~/.pi/agent/agents/*.md` with YAML frontmatter (`name`, `description`, `tools`, `model?`); project scope `.pi/agents/*.md` (trust-gated).

### D. gentle-shell reference (product `gentle-shell`, npm `gentle-pi`)

Architecture facts measured on disk:

- One `pi --mode rpc` child per task; strict JSONL framing; queue + `maxConcurrency` (default 5); stall watchdog (4 min idle / 30 min in-flight); SIGTERM→SIGKILL + quarantine (`lib/agents-runner.ts`, 935 lines).
- Typed task protocol + `TaskStore` with bounded per-task threads (`lib/agents-protocol.ts`, 432); agents in markdown + YAML frontmatter (`lib/agents-config.ts`, 293); background delivery as custom message type `gentle-agents.result` (`lib/agents-completion-delivery.ts`, 68); per-task JSON history (`lib/agents-history.ts`, 69; `agents-transcript.ts`, 76).
- UI: `lib/agents-view.ts` (837), `lib/agents-widget.ts` (216), `lib/agents-thread-view.ts` (51), `lib/agents-view-layout.ts` (36), `lib/shell-card.ts` (111) + `extensions/gentle-agents.ts` (1294). All rendering goes through pi-tui width helpers (18 files import `visibleWidth/truncateToWidth/wrapTextWithAnsi`) — safe by construction against this crash class.
- Messaging: `lib/agents-messaging.ts` (160) + session transport (POSIX registry + Windows named pipes via PowerShell helper: `agents-session-transport.ts` 637, `windows-session-transport.ts` 852) — for cross-session parent/child handoff, a separate concern from child stdio.
- Core runtime total ≈ 4.6k LOC TS (runner+protocol+config+view+widget+card+extension) excluding transport/metrics; test suite: 24 `agents-*` test files; whole `tests/` dir 233 files / ~58k lines.
- Coexistence guard: `extensions/gentle-agents.ts:242-259` — while `pi-subagents-j0k3r` is in `settings.json` `packages`, gentle stays out of the way (`LEGACY_SUBAGENTS_PACKAGE`). Tool names match the retired `pi-subagents` package so prompts keep working (`TOOL_PREFIX = "subagent_"`).
- `assets/chains/*.chain.md` (4 files) ship **without** a chain executor.
- Dependencies pinned: `@earendil-works/pi-tui@0.85.1`, `@heyhuynhgiabuu/pi-pretty@0.6.27`; peer `@earendil-works/pi-coding-agent>=0.85.1`.

### E. Prompt/config staleness found during exploration (to fix in design)

1. Delegation doc asks for `subagent(...)`; the registered tool is `subagent_run(...)`.
2. "Fall back to Pi's native `task` tool" — no native `task` tool in pi 0.85.1 (nor in `pi-agent-core`).
3. `settings.json` agent permission/tools keys `subagent` and `task` reference non-existent tool names; the real gate today is the `subagent_*` registration itself.
4. `ResolveBackgroundSubagentsCapability` probes package presence — it returns `absent` for any future own runtime unless updated.

---

## Affected Areas

If the recommended approach proceeds, expected touch points:

- `internal/assets/pi/` — new runtime asset(s) (JS/TS extension pack), retirement or rework of `biggz-wait-pretty.js` and `subagent-config.json`; keep `PI_SUBAGENT_CHILD` contract.
- `internal/install/steps/pi_extensions.go` — deploy list, agents deploy, config deploy.
- `internal/agents/pi/adapter.go` — `InstallCommand`, `desiredPiPackages`, legacy filter map, capability probe.
- `internal/doctor/pi.go` — `PiSubagentsCheck` + `Remedy` retarget (own runtime marker instead of j0k3r package).
- `internal/sdd/background.go` — capability probe.
- `internal/assets/biggz/biggz-orchestrator-delegation.md` — tool names, fallback text, FleetView phrasing.
- `~/.pi/agent/settings.json` reconciliation (via code, not manual).
- `openspec/specs/pi-integration/spec.md` (+ maybe `pi-deploy-list`) as delta targets in spec phase.
- Tests: `internal/install/pi_subagents_test.go`, `internal/agents/pi/adapter_test.go`, `subagents_fork_test.go`, `internal/doctor/pi_subagents_test.go`, plus new JS test harness in `internal/assets/pi/*.test.mjs` style.
- Verification: crash-class regression fixtures (emoji/CJK/ZWJ width fuzz with asymmetric property `mine >= pi-tui.visibleWidth`).

---

## Approaches

### Approach A — Own full extension pack (gentle-shell parity)

Adapt gentle-shell's architecture wholesale: runner + protocol + TaskStore + view overlay + widget + history + session transport + messaging, deployed via the existing installer.

- Pros: proven design; full feature parity (FleetView-like overlay, history, profiles); single owner of the whole surface; removes every j0k3r coupling (`wait-pretty`, config, panels).
- Cons: largest scope by far (~4.6k LOC core + ~1.5k transport + equivalent tests; gentle's whole test dir is ~58k LOC); many features biggz does not consume (chains, profiles UI, cross-session IPC, review bridges); MIT notice + trademark renaming obligations if code is adapted; slowest path to killing the crash (weeks); review budget vs 400 lines forces many chained PRs.
- Effort: **High**. Risk: **High** (schedule; parity gaps; Windows transport work).

### Approach B — Minimal own runtime over pi primitives (recommended target)

Own extension pack in biggz (`internal/assets/pi/biggz-agents/…` style, deployed by the existing installer) implementing only the consumed surface:

- `subagent_run` (foreground `mode:"task"` + `mode:"background"`), plus `subagent_status`, `subagent_result`, `subagent_cancel`; optional `subagent_list_agents`.
- One `pi` child per task — start from pi's official example (`--mode json`) and move to `--mode rpc` only for what needs interactivity (steering, child `ask_user_question`); strict JSONL parsing.
- Queue + concurrency cap (default ≤2 background as per policy; configurable), stall watchdog, SIGTERM→SIGKILL, `PI_SUBAGENT_CHILD=1` on children, background completion via custom message/notification.
- Rendering ONLY through pi-tui `visibleWidth/truncateToWidth/wrapTextWithAnsi` (safe by construction), reusing `subagent-config.json` budgets; minimal async status widget replaces FleetView for background runs.
- Agent discovery from `~/.pi/agent/agents/*.md` (already biggz-owned); model inherited from session or frontmatter `model`.
- Coexistence rule borrowed from gentle (inverted): own runtime registers only when j0k3r is NOT present, or behind a flag, until cutover.
- Retire `biggz-wait-pretty.js` (j0k3r-hook shim becomes dead) and the j0k3r-specific `subagent-config.json` keys; doctor/installer/capability probes retarget to the own runtime.

- Pros: fixes the crash class by construction (no width math outside pi-tui); owns the delegation contract biggz actually uses (~1 tool + 3 siblings); deploys through the existing embedded-assets + installer path (no new infrastructure); incremental — each slice is independently verifiable and revertible in 1 file/flag; keeps markdown agents and SDD prompts working; biggz already absorbed the same authoring model for 13 extensions.
- Cons: still a real runtime to own (interactive dialogs, cancel semantics, Windows process handling, history) — estimate 1.5–2.5k LOC JS + tests if scoped strictly; RPC/extension API churn risk from pi upgrades; feature loss vs j0k3r (model-profiles UI, `/subagents` panel, rich FleetView overlay, `subagent_send_message`, `subagent_continue`) — acceptable only if the delegation contract stops mentioning them; `pi --mode json` children cannot answer `ask_user_question` (SDD agents carry it in their tool whitelist) → rpc or tool-whitelist change needed.
- Effort: **Medium** (first slice small; total comparable to a medium SDD change with chained PRs). Risk: **Medium**.

### Approach C — Status quo hardened (pin + local patch + upstream)

Keep j0k3r as the runtime; pin `npm:pi-subagents-j0k3r@1.6.1`; patch the width helpers locally; push the offered upstream fix.

- Variant C1 (patch `node_modules`): patch `src/render/text-width.ts` + 3 duplicated helpers + re-export users (4+ files) after every `pi install`/`pi update --extensions`. Not durable by itself.
- Variant C2 (fork/vendor patched package): publish `npm:pi-subagents-fixed` or vendor ~10k lines (ADR `docs/adr/xxx-pi-subagents-wait.md` already evaluated fork/vendor for a lesser (visual) problem and rejected both in favor of a shim; the crash raises the stakes but not the cost profile).
- Pros: fastest crash mitigation (hours); tiny diff for C1; leverages the upstream issue (#25) where a verified patch is offered; zero new runtime ownership; the pin alone implements the 2026-09-10 decision.
- Cons: C1 is not durable (override re-applied on every dependency update) and leaves 3 other unfixed width copies (E6); C2 re-introduces exactly the maintenance-at-arms-length the ADR rejected, plus package publishing/trust surface; both keep biggz's delegation stack coupled to j0k3r internals (wait hooks, config keys, panel events); prompts stay stale (`subagent`/`subagent_wait`/`task` naming); upstream fix timing is unbounded.
- Effort: **Low** (C1). Risk: **High** for durability/longevity, **Low** for immediate symptom.

### Variant — external shim to neutralize the crash without patching the plugin

A biggz extension could intercept `pi.registerTool` before the plugin loads and replace the `subagent_*` renderers with pi-tui-based ones. Investigated conceptually: load-order dependent, must re-implement j0k3r's renderers 1:1, and breaks silently on plugin refactors. Rejected as primary path; only worth it if C1's patch is deemed unacceptable and B cannot start immediately.

### Comparison

| Approach | Fixes crash class | Time to mitigation | Ownership cost | Feature loss | Effort | Risk | Converges to own runtime |
|----------|-------------------|--------------------|----------------|--------------|--------|------|--------------------------|
| A — full parity pack | Yes (by construction) | Weeks | ~4.6k+ LOC TS + tests | None (exceeds) | High | High | Yes |
| B — minimal own runtime | Yes (by construction) | Days–2 weeks, slice 1 | ~1.5–2.5k LOC TS + tests | Profiles/panel/overlay rich UI (needs minimal equivalents) | Medium | Medium | Yes |
| C1 — pin + node_modules patch | Yes (until reinstall) | Hours | Patch maintenance per update | None | Low | Med (durability) | No |
| C2 — fork/vendor patched package | Yes | Days | Fork drift + publish surface | None | Medium | Med-High | No |

---

## Recommendation

**GO — adopt Approach B as the target for this change, with Approach C1 as a mandatory, independent Phase 0 slice.**

Rationale:

1. The crash class is structural, not cosmetic: 4+ render paths in the plugin do their own width math (E6), the completion message persists (session un-resumable), and pi's guard is fail-closed (E2). Shims cannot fix what they do not own; only a runtime that renders exclusively through pi-tui primitives removes the class.
2. The consumed surface is small (one primary tool + 3–4 siblings + background policy + agents/*.md), so a minimal runtime is far below gentle's 4.6k LOC — and biggz's installer/asset pipeline already exists to ship it.
3. Upstream is informed (issue #25) but has shipped two releases since the report without a fix; waiting under a floating dependency is the risk the 2026-09-10 memo accepted and the crash has now made concrete.
4. The pin (Phase 0) costs one slice, implements the never-executed prior decision, and buys calm time while B is built; it also should accompany an upstream engagement (offer to submit the verified patch) because a merged upstream fix remains the cheapest long-term win for users of the plugin outside biggz.
5. Approach A is not justified now: it pays for features biggz does not consume; the design phase can still borrow A's architecture (child=rpc process, TaskStore, safe rendering) without its full UI/teleport surface.

### Explicit scope boundaries for `pi-subagent-runtime` (v1)

**In scope**
- Own pi extension pack under `internal/assets/pi/` (deployed by `PiExtensionsStep`), single registration namespace `subagent_*` with parity semantics for `subagent_run` (+ `status`/`result`/`cancel`).
- Child process management: one `pi` child per task, strict JSONL, queue, concurrency cap, stall watchdog, SIGTERM→SIGKILL, `PI_SUBAGENT_CHILD=1` on children.
- Foreground `mode:"task"` (primary) + background `mode:"background"` with completion notification (custom message type) and ≤2 concurrent per current policy; `BIGGZ_BACKGROUND_SUBAGENTS` policy integration unchanged.
- Rendering exclusively via `@earendil-works/pi-tui` width primitives; minimal status widget for background runs; crash-class regression suite (fuzz emoji/CJK/ZWJ/ANSI with asymmetric property vs pi-tui).
- Installer/doctor/capability-probe retargeting; settings reconciliation switch; pin of any remaining third-party (j0k3r during Phase 0; upstream fix monitoring).
- Prompt updates in `biggz-orchestrator-delegation.md` to the real tool surface.

**Out of scope (v1)**
- Fullscreen agents view/overlay, history panel, thread view, chains executor, model-profiles UI, cross-session session transport (named pipes / registry), review/remediation bridges (gentle specific), `subagent_send_message`/`subagent_continue` unless usage evidence demands them, macOS/Linux-only niceties.
- Vendoring or publishing third-party packages.
- Any change to SDD phase semantics or the BigMem protocol.

### MVP slice candidates (for `sdd-tasks`)

| Slice | Content | Exit evidence |
|-------|---------|---------------|
| S0 — Pin + upstream | Pin `npm:pi-subagents-j0k3r@1.6.1` in `InstallCommand` + `desiredPiPackages`; update doctor remedy text; file/comment upstream engagement on issue #25 (offer the verified patch path) | Installer tests updated; `pi install` idempotent; settings shows `@1.6.1` |
| S1 — Runtime core (foreground) | Ext pack skeleton + `subagent_run` task-mode: spawn `pi` child, JSONL parse, timeout/stall kill, safe rendering via pi-tui | Unit tests (runner fuzz with fake child), manual smoke: SDD phase delegated ink-session, no crash with emoji-heavy report fixture |
| S2 — Background + status tools | `mode:"background"`, completion message delivery, `subagent_status`/`subagent_result`/`subagent_cancel`, concurrency cap + policy wiring | E2E background two-task run; widget shows states; cancel kills child tree (Windows) |
| S3 — Cutover + retirement | Coexistence guard/cutover switch, retire `biggz-wait-pretty.js` + j0k3r config keys, installer/doctor/capability retarget, prompt updates, remove legacy dead deploy funcs | Doctor PASS with own runtime; j0k3r uninstalled from settings; delegation doc matches tool names |
| S4 — Crash-class regression harness | JS width-gate tests (fixtures: ✅ rows, CJK, Hangul, ZWJ, ANSI/OSC; property `measure(line) >= pi-tui.visibleWidth(line)`; `wrap` output ≤ width) | `node --test` green in CI; fixture reproducing the original w=190>188 shape passes |

---

## Risks

1. **pi RPC/extension API churn** — biggz floats pi itself (`0.85.1`); pin strategy for pi-tui types and defensive parsing are required. gentle pins pi-tui exactly (`0.85.1`) for this reason.
2. **Child interactivity parity** — SDD agents carry `ask_user_question`; `--mode json` children cannot answer. Either use `--mode rpc` (interaction bridging complexity) or remove the tool from child agents (behavior change to SDD phases).
3. **Transition collision** — dual registration of `subagent_*` if j0k3r remains installed while own runtime registers; needs the gentle-style guard (register only when j0k3r absent) or an explicit flag.
4. **Capability-probe false negatives** — `ResolveBackgroundSubagentsCapability` and doctor check key off package presence; forgetting them leaves background policy permanently `off`/warning.
5. **Feature expectations in prompts** — `FleetView`/`subagent_wait`/`subagent` phrasing is embedded in prompts and specs; leaving it stale produces prompt/tool mismatch (already the case today, E-drift).
6. **Windows specifics** — process-tree kill, shell resolution (`cmd /c` vs bash), path/env propagation; gentle needed a dedicated Windows transport (out of v1, but child stdio must still behave on win32).
7. **MIT/trademark obligations** — adapting gentle-shell code requires preserving the LICENSE notice and renaming marks (TRADEMARKS.md); prefer independent implementation of patterns, not copy-paste.
8. **Over-ownership creep** — minimal runtime can silently grow into full parity; the out-of-scope list above is the guard.
9. **Verification gap upstream** — no width tests exist in the plugin; our regression harness must not depend on upstream test infrastructure.

---

## Open Questions

1. Tool naming: register a `subagent` alias (official-example name) alongside `subagent_run`, or normalize prompts to `subagent_run` only? (Affects delegation doc, settings permissions, and any user muscle memory.)
2. Child protocol: `--mode json` (simpler, no dialog channel) vs `--mode rpc` (steering + child `ask_user_question` parity)? Which SDD phases actually ask questions in children today?
3. Background verification semantics: is "completion notification only, no polling" preserved with a custom message type, and does `biggz-session-guard`/synthesis gate interplay stay intact?
4. Model selection per SDD agent: keep frontmatter `model:` inheritance (pi-native) and drop j0k3r's model-profiles UI, or replicate minimal profiles?
5. Pin policy: exact `@1.6.1` vs `~1.6.x` for Phase 0, and what is the removal criterion (upstream fix released + one-release soak)?
6. Where does the crash-class width harness live (mirroring `internal/assets/pi/*.test.mjs` convention) and is it wired into CI?
7. FleetView minimum viable substitute: status widget only, or also a `/biggz-agents` panel? (Skip panels in v1 unless the user requires them.)

---

## Ready for Proposal

**Yes.** The orchestrator should tell the user: exploration is done; the crash has a verified root cause (code-point vs cell width in `pi-subagents-j0k3r@1.6.1`, upstream issue #25 open since 2026-09-09, unfixed in latest 1.6.1); the recommendation is **GO** on a minimal own pi subagent runtime (Approach B) with an immediate pin slice (Approach C1 as Phase 0), plus explicit v1 scope boundaries (no overlay/history/chains/session-transport) and a 5-slice MVP plan; two decisions are needed before `sdd-propose`: the tool-naming strategy (Q1) and the child protocol (Q2).
