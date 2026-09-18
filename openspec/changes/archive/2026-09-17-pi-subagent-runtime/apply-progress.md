# Apply Progress: pi-subagent-runtime

**Work unit**: `S0-pin` (PR 1 of 6, chain: stacked-to-main)
**Mode**: Standard (store `openspec`, `strict_tdd: false`)
**Date**: 2026-09-17
**Status**: S0 complete — ready for slice verify

## Completed Tasks

| Task | Description | Status |
|------|-------------|--------|
| 1.1 | Pin `npm:pi-subagents-j0k3r@1.6.1` in `InstallCommand` + `desiredPiPackages` (`internal/agents/pi/adapter.go`) | done |
| 1.2 | `subagents_fork_test.go`: exact-match assertion updated to `pi install npm:pi-subagents-j0k3r@1.6.1`; desired-packages pin assertion added | done |
| 1.3 | `internal/doctor/pi.go` `PiSubagentsCheck.Remedy()` installs the pinned identity | done |
| 1.4 | Upstream #25 engagement comment drafted below (crash evidence + root cause + patch offer) — **NOT posted**, posting decided at checkpoint | done (draft) |

`tasks.md` checkboxes intentionally untouched — the orchestrator marks them.

## Files Changed

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/agents/pi/adapter.go` | Modified | `InstallCommand` installs `npm:pi-subagents-j0k3r@1.6.1`; `desiredPiPackages` carries the exact pin; new `dropSupersededPiPins` + `piPackageBase` helpers make reconciliation replace floating/older j0k3r entries instead of coexisting with them; InstallCommand bullet comment updated |
| `internal/agents/pi/subagents_fork_test.go` | Modified | `TestInstallCommand_UsesJ0k3rFork` asserts the exact pinned command and fails on the unpinned form; new `TestSettingsReconcile_PinsJ0k3rFork` (real `ProvisionBigMemMCP` file path, temp HOME) asserts pin replacement, pretty preservation, and idempotence |
| `internal/doctor/pi.go` | Modified | `Remedy()` description, executed command, and error wrap use `npm:pi-subagents-j0k3r@1.6.1`; missing-package and legacy migrate hints pinned; check doc comment updated |
| `internal/doctor/pi_subagents_test.go` | Modified | New `TestPiSubagentsRemedy_PinsFork` asserts the remedy names the pinned install command |
| `openspec/changes/pi-subagent-runtime/apply-progress.md` | Created | This evidence ledger |

## Work Unit Evidence

| Evidence | Value |
|----------|-------|
| Focused test command and exact result | `go test ./internal/agents/pi ./internal/doctor -count=1` → exit 0; `ok internal/agents/pi 0.669s`, `ok internal/doctor 1.336s` |
| Runtime harness command/scenario and exact result | `go test ./internal/agents/pi -run TestSettingsReconcile_PinsJ0k3rFork -count=1 -v` → PASS (0.03s). Real reconciliation code path (`ProvisionBigMemMCP` → `mergePiSettingsBigMem`) against a temp HOME: seeds `settings.json` packages with floating `npm:pi-subagents-j0k3r` + `@1.6.0` + `npm:@heyhuynhgiabuu/pi-pretty`; after run exactly one `npm:pi-subagents-j0k3r@1.6.1`, zero floats/older, pretty preserved; second run byte-stable package count. Real `pi install` re-run NOT executed (mutates the developer's pi installation; deferred to the S0 checkpoint as non-code verification) |
| Rollback boundary | Revert the four Go files (`adapter.go`, `subagents_fork_test.go`, `doctor/pi.go`, `doctor/pi_subagents_test.go`); re-running `pi install npm:pi-subagents-j0k3r` restores the floating spec. No migrations, no persisted state, no JS assets, no settings written outside tests |

## EVIDENCE

Canonical command (ledger input):

```
go test ./internal/agents/pi ./internal/doctor -count=1
```

- exit code: `0`
- captured output file: `C:\Users\USER\AppData\Local\Temp\opencode\s0-pin-evidence.txt` (stdout+stderr)
- captured output:

```
ok  	github.com/biggs-100/biggz-ai/internal/agents/pi	0.669s
ok  	github.com/biggs-100/biggz-ai/internal/doctor	1.336s
```

- `sha256:2dbce9eb643f22d6395062573f1cef2cdf21ff07e918d5cedd2e880a491cd42e`

Supporting commands (all exit 0):

- `go build ./...` → exit 0
- `go vet ./internal/agents/pi ./internal/doctor` → exit 0
- `gofmt -l` on the four changed Go files → empty output, exit 0
- `go test ./internal/agents/pi -run "TestInstallCommand_UsesJ0k3rFork|TestSettingsReconcile_PinsJ0k3rFork|TestFilterPiPackages_DropsPredecessor|TestInstallCommand_ContainsAdapterBeforeSubagents" -count=1 -v` → 4/4 PASS (task 1.1 evidence, includes the exact-pin assertion)
- `go test ./internal/doctor -run "TestPiSubagents" -count=1 -v` → 4/4 PASS including new `TestPiSubagentsRemedy_PinsFork` (task 1.3 evidence)
- `use-modern-go` CLI `list` (Go 1.25 guideline set) consulted for the edited Go files; every returned guideline evaluated against the diff, none required changes (`any` already used; no manual min/max, sort, or map-copy loops introduced)

Real-environment observation (read-only, zero mutation): `~/.pi/agent/settings.json:523` currently holds the floating `"npm:pi-subagents-j0k3r"` entry — the next `biggz install --agent pi` reconciliation replaces it with the pinned `npm:pi-subagents-j0k3r@1.6.1` (that replacement path is what `TestSettingsReconcile_PinsJ0k3rFork` exercises).

## Task 1.4 — Upstream #25 Engagement Comment (DRAFT — NOT POSTED)

> **Not posted.** Per run scope, nothing is posted to GitHub; posting is decided at the S0 checkpoint. Target: `j0k3r-dev-rgl/pi-subagents-j0k3r` issue **#25** ("TUI crash: subagent completion card pads with grapheme count, so lines with 2-cell graphemes (✅) overflow the terminal").

```markdown
Additional crash evidence, a root-cause trace, and a patch offer for #25.

**Crash evidence**
- `~/.pi/agent/pi-tui-crash.log` (2026-09-17T16:16:11.429Z) shows `Terminal width: 188` and `Line 741 visible width: 190`.
- Line 741 is a subagent result card containing exactly 2x `✅` (U+2705, 2 terminal cells each). The overflow delta (190 - 188 = 2) equals the number of 2-cell graphemes: code-point counting under-measures exactly the emoji/CJK cells.
- Impact: pi core's fail-closed width guard throws and kills the whole process; because the completion message is persisted in the session log, the session becomes un-resumable until the message is removed or the plugin is patched.

**Root cause (verified against source)**
- `src/render/text-width.ts:7-9`: `visibleWidth(text) = [...stripAnsi(text)].length` counts code points as cells, and `wrapLineToWidth` (same file, line 49) treats every non-ANSI token as width 1.
- The completion card renders through `src/render/tools/components.ts:71,108-109` (`boxedComponent(..., { wrapped: true })`), so its padding/wrapping is cell-unsafe.
- pi core is not the defect: the guard is fail-closed by design (`chunk-JVUZSMYM.js:599-601`), and its error already names the fix (`Use visibleWidth() to measure and truncateToWidth() to truncate lines`).
- `@earendil-works/pi-tui@0.85.1` already ships cell-aware `visibleWidth` / `truncateToWidth` / `wrapTextWithAnsi` (`dist/utils.js`, East-Asian-width based). The plugin implements its own naive counter and has three more duplicates on `main` (`visibleTextWidth` in `src/render/tools/components.ts`, `src/ui/subagents-history-panel.ts`, `terminalVisibleWidth` in `src/thread-view.ts`).
- Second trigger path: the running-subagent tool row (`src/render/tools/subagent-run.ts:69` + `boxedComponent`), same root cause.

**Patch offer**
We can submit a PR that (1) routes every width decision through pi-tui's `visibleWidth` / `truncateToWidth` (single implementation, naive helpers deleted) and (2) adds a regression fixture: a card line with two `✅` at width 188 must satisfy `visibleWidth(renderedLine) <= 188` across widths 20-200. Happy to open it on request; otherwise feel free to lift the approach.

Meanwhile our installer pins `npm:pi-subagents-j0k3r@1.6.1` so we stop floating onto untested builds.
```

### S0 checkpoint outcomes (recorded during S1a apply)

- **Upstream #25 comment PUBLISHED** at `https://github.com/j0k3r-dev-rgl/pi-subagents-j0k3r/issues/25#issuecomment-5719452558` (draft above, verbatim). This supersedes the `NOT POSTED` status in the task table.
- **Local machine pinned**: `biggz install --agent pi --yes` → exit 0; `~/.pi/agent/settings.json:527` now holds `"npm:pi-subagents-j0k3r@1.6.1"`; installed plugin `package.json` reports version `1.6.1` (read-only re-verification, 2026-09-17).

## Deviations from Design

1. **Exact-pin reconcile added (`dropSupersededPiPins` + `piPackageBase` in `mergePiSettingsBigMem`).** Pinning the two strings alone would let a pre-existing floating `npm:pi-subagents-j0k3r` entry coexist with the appended `@1.6.1`, so `settings.json` would still carry a float — contradicting delta REQ "`pi-subagents-j0k3r` MUST carry `@1.6.1` and MUST NOT float". The helpers drop same-base/different-spec entries so the pin replaces them; unpinned desires (`npm:@heyhuynhgiabuu/pi-pretty`) keep the previous base-dedupe behavior. Covered by `TestSettingsReconcile_PinsJ0k3rFork`.
2. **Doctor user-facing hints pinned too** (missing-package message and legacy migrate hint), not only `Remedy()` — so the printed command matches what the remedy executes.
3. Doc comments updated where they asserted the unpinned identity (`adapter.go` InstallCommand bullet, `doctor/pi.go` check doc).

## Issues Found

- `adapter_test.go` only asserts `strings.Contains(joined, "pi-subagents-j0k3r")`, which passes with the pinned spec — left untouched (no unpinned-exact assertion there).
- `internal/install/install.go` and `internal/install/steps/pi_extensions.go` still reference j0k3r package presence (FleetView probe, agent/config deploy) — by design, S3 cutover scope; S0 does not touch them.
- Real `pi install` re-run idempotence was not executed in this run (would mutate the developer's pi installation); the settings-reconciliation path is covered by the temp-HOME harness test, and the live re-run belongs to the checkpoint decision.

## Remaining Tasks (this change, other slices)

- S1a (`S1a-runner`), S1b (`S1b-tools`), S2 (`S2-background`), S3 (`S3-cutover`), S4 (`S4-harness`) — untouched in this run (tasks.md Phases 2-6).

## Workload / PR Boundary

- Mode: chained PR slice (stacked-to-main) — PR 1 of 6
- Current work unit: `S0-pin`
- Boundary: starts from the clean tree plus the orchestrator's pre-existing uncommitted worktree changes (untouched); ends at pinned install/reconcile/remedy plus tests. No runtime JS, no cutover, no deploy-list changes.
- Changed lines: `157 insertions + 16 deletions = 173` (≤400 review budget; slice-sized for a single reviewer pass)
- Rollback: revert the four Go files; reinstall restores the floating package.

## Status

4/4 S0 tasks complete (1.4 delivered as an evidence draft per scope). Ready for slice verify.

---

# S1a — Runner core (PR 2 of 6, WU `S1a-runner`)

**Work unit**: `S1a-runner` (stacked-to-main, PR 2 of 6)
**Mode**: Standard (store `openspec`, `strict_tdd: false`; RED/GREEN task pairs executed explicitly, RED captured with the module absent)
**Date**: 2026-09-17
**Status**: S1a core complete (tasks 2.1–2.6) — ready for slice verify; line-budget decision pending

## Completed tasks

| Task | Description | Status |
|------|-------------|--------|
| 2.1 | RED `biggz-subagent-runtime.test.mjs`: fake-child harness + JSONL fuzz (partial line, CR strip, U+2028-in-string, malformed skipped+logged, 1 MiB cap/oversized) | done |
| 2.2 | GREEN `biggz-subagent-runtime.js`: strict JSONL reader — LF-only split, no `readline`, bounded 1 MiB buffer, malformed/partial logged+skipped, session survives | done |
| 2.3 | RED spawn authorization: `--tools` from frontmatter, missing frontmatter → read-only `read`, `PI_SUBAGENT_CHILD=1` set on spawn only, nested spawn refused, unknown agent → bounded error naming available agents | done |
| 2.4 | GREEN agent discovery (`~/.pi/agent/agents/*.md`, frontmatter `tools`/`model`, `PI_CODING_AGENT_DIR` honored) + spawn plan `pi --mode rpc --no-session --tools …` with `PI_SUBAGENT_CHILD=1` | done |
| 2.5 | RED watchdog/kill: idle/total → `stalled`; abort → SIGTERM group → SIGKILL (Windows `taskkill /PID /T [/F]`); child + grandchild dead after cancel; session alive | done |
| 2.6 | GREEN watchdog + process-tree kill escalation | done |

`tasks.md` checkboxes intentionally untouched — the orchestrator marks them.

## Files changed

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/assets/pi/biggz-subagent-runtime.js` | Created (416 lines) | Strict JSONL reader, frontmatter/discovery, spawn authorization, one-RPC-child runner, stall watchdog, tree-kill escalation, inert pi factory |
| `internal/assets/pi/biggz-subagent-runtime.test.mjs` | Created (256 lines) | 12 `node --test` cases; fake pi RPC child embedded inline (`node -e <script>`) — no fixture file needed |

No other file touched (no Go, no deploy list, no tasks.md, no fixture dir).

## Work Unit Evidence

| Evidence | Value |
|----------|-------|
| Focused test command and exact result | `node --test internal/assets/pi/biggz-subagent-runtime.test.mjs` → exit 0; `tests 12 / pass 12 / fail 0 / duration 10446ms`; independent rerun → exit 0 |
| Runtime harness command/scenario and exact result | Real fake-child process tree over stdio JSONL (`node -e` reports its pid + grandchild pid + `PI_SUBAGENT_CHILD`): completion on `agent_settled`, idle stall 250ms, total stall 300ms, cancel of child+grandchild — tree death verified by polling `process.kill(pid, 0)`; zero stray fake children after the suite (Win32 process scan). RED captured first against the same test file with the module absent: exit 1 (`ERR_MODULE_NOT_FOUND`) |
| Rollback boundary | Delete the two new files (`internal/assets/pi/biggz-subagent-runtime.js`, `internal/assets/pi/biggz-subagent-runtime.test.mjs`). Nothing else changed; the asset ships inert (absent from the S3 deploy list), so no deployed behavior is affected |

## EVIDENCE

Canonical command (ledger input):

```
node --test internal/assets/pi/biggz-subagent-runtime.test.mjs
```

- exit code: `0`
- captured output file: `C:\Users\USER\AppData\Local\Temp\opencode\s1a-evidence-final.txt` (stdout+stderr)
- captured output:

```
▶ strict JSONL framing
  ✔ parses partial lines, several records per chunk, and CRLF (CR stripped) (1.3639ms)
  ✔ keeps U+2028/U+2029 inside JSON strings; readline is never used (0.3247ms)
  ✔ skips malformed lines with a warning and keeps the stream alive (0.3405ms)
  ✔ drops oversized lines at the 1 MiB cap (with and without LF) and resumes (2.5873ms)
✔ strict JSONL framing (5.6717ms)
▶ agent discovery + spawn authorization
  ✔ parses tools/model/body from frontmatter; missing frontmatter ⇒ read-only (5.4065ms)
  ✔ bounds unknown-agent errors, refuses nested spawn, builds argv+env without mutation (0.6386ms)
✔ agent discovery + spawn authorization (6.2486ms)
▶ RPC child runner (fake child over JSONL)
  ✔ completes on agent_settled; child env PI_SUBAGENT_CHILD=1 is set on spawn only (413.2132ms)
  ✔ idle watchdog stalls + kills a silent child; the runtime survives it (1009.8004ms)
  ✔ total watchdog stalls a chatty child (595.0271ms)
  ✔ cancel kills the child + grandchild tree and marks the task cancelled (380.4043ms)
  ✔ escalates abort → SIGTERM group → SIGKILL (posix) and taskkill /T → /T /F (win32) (0.5314ms)
✔ RPC child runner (fake child over JSONL) (2399.4068ms)
▶ extension factory (inert S1a)
  ✔ attaches internals for tests and no-ops inside a subagent child (0.3258ms)
✔ extension factory (inert S1a) (0.4258ms)
ℹ tests 12
ℹ suites 4
ℹ pass 12
ℹ fail 0
ℹ cancelled 0
ℹ skipped 0
ℹ todo 0
ℹ duration_ms 10500.8384
```

- `sha256:4e07cf5262484c6934f40eb9873f6907a3f0014bed4629bc58db2ac865cde105`

RED evidence (same test file, module removed):

```
node --test internal/assets/pi/biggz-subagent-runtime.test.mjs
```

- exit code: `1` — `Error [ERR_MODULE_NOT_FOUND]: Cannot find module '…biggz-subagent-runtime.js' imported from '…biggz-subagent-runtime.test.mjs'`
- captured output file: `C:\Users\USER\AppData\Local\Temp\opencode\s1a-red-final.txt`
- `sha256:9708e6d938066d962b0c5f3b11b052014c16602a6f29c6cd577759550a5f58d8`

Supporting commands:

- `go build ./...` → exit 0 (embed sanity for `internal/assets` `all:pi`)
- `node --check internal/assets/pi/biggz-subagent-runtime.js` → exit 0
- `node --check internal/assets/pi/biggz-subagent-runtime.test.mjs` → exit 0
- `go vet ./internal/assets/...` → exit 0

## Deviations from Design

1. **`resolvePiLaunch` seam added.** `spawn("pi")` is ENOENT on Windows npm shims (verified on this machine: bare `pi` fails, `pi.cmd` works), so the default launch resolves `BIGGZ_PI_BIN` override → `pi.cmd` + `shell:true` on win32 → `pi` elsewhere. Tool/model tokens are sanitized to `[A-Za-z0-9_.*-/:]` before they can reach a shell.
2. **Completion is settled before tree disposal.** On `agent_settled` the task settles `completed` immediately and the child tree is killed best-effort in the background, so completion latency never waits for the kill grace.
3. **Final-text extraction deferred to S2.** `get_last_assistant_text`/TaskStore are S2 scope; S1a keeps a bounded event array (max 500) plus `stderrTail` on the snapshot.
4. **Kill escalation keeps the polite step on win32** (`taskkill /PID /T`, then `/T /F` after grace) rather than only `/T /F`, so the spec's SIGTERM→SIGKILL escalation shape holds on both platforms; a failed polite step skips the grace wait and forces immediately.
5. **Test harness passes explicit empty `args`** when the fake child is injected (`node -e <script>` must not receive pi's `--mode rpc …` argv); the production argv contract is asserted directly by `buildChildArgs`.

## Issues Found

- Budget: the slice landed at **672 authored lines** (416 runtime + 256 test) versus the ~350 estimate / 400 cap. The mandated RED matrix (5 JSONL fixtures + spawn-authorization cases + real child/grandchild tree-kill assertions on Windows) plus a session-surviving runner does not compress to 350 without dropping required evidence. Proposed split if the cap is enforced: (a) JSONL + discovery + spawn authorization, (b) runner + watchdog + kill. Orchestrator decision needed.
- `spawn("pi")` on Windows is ENOENT (npm `.cmd` shims) — handled, but worth noting for S1b/S2 which reuse the launch seam.

## Remaining Tasks (this change, other slices)

- S1b (`S1b-tools`), S2 (`S2-background`), S3 (`S3-cutover`), S4 (`S4-harness`) — untouched in this run (tasks.md Phases 3-6).

## Workload / PR Boundary

- Mode: chained PR slice (stacked-to-main) — PR 2 of 6
- Current work unit: `S1a-runner`
- Boundary: starts from the S0 slice; ends at the inert runtime module + its test file. No deploy-list entry (inert until S3), no Go changes, no other assets.
- Changed lines: `416 + 256 = 672` authored additions, 0 deletions (two new files). Over the 400 review budget — see Issues Found.
- Rollback: delete the two new files.

## Status

6/6 S1a tasks complete with RED+GREEN evidence (12/12 tests, canonical sha256 recorded). Ready for slice verify; line-budget decision pending with the orchestrator.

---

# S1b — Gate, tools, card, resolver (PR 3 of 6, WU `S1b-tools`)

**Work unit**: `S1b-tools` (stacked-to-main, PR 3 of 6)
**Mode**: Standard (store `openspec`, `strict_tdd: false`; RED task labels = test hunks authored before the runtime hunks within this run)
**Date**: 2026-09-17
**Status**: S1b complete (tasks 3.1–3.7) — ready for slice verify; line-budget decision pending

## Completed tasks

| Task | Description | Status |
|------|-------------|--------|
| 3.1 | RED dual-registration threat: `getAllTools()` lists `subagent_run` → zero registrations; j0k3r in settings `packages` → zero; degraded chain → fail-closed + logged reason | done |
| 3.2 | GREEN registration gate: one `typeof`-guarded chain `getAllTools()` → `getToolDefinition("subagent_run")` → settings `packages`; fail-closed when absence is unprovable; never calls `pi.getTool` | done |
| 3.3 | RED `subagent` task-mode: result + task id; `context` default `fresh`, `fork` → bounded unsupported error; `mode` default `task`; unknown agent bounded | done |
| 3.4 | GREEN tool surface (`subagent`, `subagent_status`, `subagent_result`, `subagent_cancel`, `subagent_agents`) over a bounded in-memory ring only (no disk, no BigMem) | done |
| 3.5 | RED completion card `renderCompletion`: one bounded line, widths 20–200, `truncateToWidth`, exact two-`✅` asset whose true width is 190 at 188 code points | done |
| 3.6 | GREEN `biggz-subagent-completion` renderer built from the exported pure `renderCompletion` | done |
| 3.7 | `test/pi-tui-resolver.mjs` (`--import` specifier hook, `PI_TUI_DIR` override, install → oracle fallback) + `test/pi-tui-oracle.mjs` + fidelity test vs the real pi-tui | done |

`tasks.md` checkboxes intentionally untouched — the orchestrator marks them.

## Files changed

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/assets/pi/biggz-subagent-runtime.js` | Modified (416 → 690) | Registration gate (tool scan → `getToolDefinition` → settings packages → fail-closed), bounded in-memory task ring, five delegation tools with terse JSON-schema params, `completionLine`/`renderCompletion`/`createCompletionRenderer` over pi-tui `truncateToWidth`, factory wiring (`registerTool` × 5 + `registerMessageRenderer("biggz-subagent-completion")`), static `@earendil-works/pi-tui` import (resolved by the test resolver in `node --test`) |
| `internal/assets/pi/biggz-subagent-runtime.test.mjs` | Modified (256 → 496) | Resolver installed before the runtime is dynamically imported; 9 new cases: gate (4), ring (1), tool surface (1), card (2), resolver fidelity + oracle existence (1) |
| `internal/assets/pi/test/pi-tui-resolver.mjs` | Created (78 lines) | `registerHooks`-based specifier hook; priority `PI_TUI_DIR` → Node resolution → discovered installs (Windows npm global, `~/.pi/agent/npm`) → oracle; idempotent, self-installs on import so `--import` works |
| `internal/assets/pi/test/pi-tui-oracle.mjs` | Created (96 lines) | Test-only cell-aware fallback (`visibleWidth`, `truncateToWidth`, `stripTerminalSequences`) with a grapheme segmenter + EAW/emoji ranges; fidelity-tested against the real pi-tui |

No other file touched (no Go, no `tasks.md`, no deploy list, no specs/design).

## Work Unit Evidence

| Evidence | Value |
|----------|-------|
| Focused test command and exact result | `node --test internal/assets/pi/biggz-subagent-runtime.test.mjs` → exit 0; `tests 21 / suites 9 / pass 21 / fail 0 / duration_ms 11277.2556` |
| Runtime harness command/scenario and exact result | Fake-`ExtensionAPI` load matrix: (a) `getAllTools()` lists `subagent_run` + `subagent_list_running` → 0 registrations + warning; (b) settings `packages` carry `npm:pi-subagents-j0k3r@1.6.1` → 0 registrations; (c) `getAllTools()` throws + corrupt `settings.json` → fail-closed, 0 registrations, logged reason; (d) clean settings + load-time `getAllTools()` throw → exactly `subagent, subagent_status, subagent_result, subagent_cancel, subagent_agents` + the completion renderer. Tool-mode uses a real fake RPC child (`node -e <script>`, `FAKE_SETTLED=1`) through the ring. Card: brute-forced 188-code-point two-`✅` line (true width 190) renders ≤ 188 across widths 20–200. Resolver: fidelity vs real `@earendil-works/pi-tui@0.85.1` on 11 fixtures (emoji/CJK/Hangul/ZWJ/ANSI/combining/ASCII) × `visibleWidth` + 5 truncate widths → PASS; forced-oracle probe (scratch, empty install roots) → mode `oracle`, runtime card renders end-to-end |
| Rollback boundary | Revert the gate/ring/tools/card/factory hunks in `biggz-subagent-runtime.js` + the S1b describes/imports in the test file and delete `internal/assets/pi/test/` (two files). Nothing else changed; the asset still ships inert (absent from the S3 deploy list), so no deployed behavior is affected |

## EVIDENCE

Canonical command (ledger input):

```
node --test internal/assets/pi/biggz-subagent-runtime.test.mjs
```

- exit code: `0`
- captured output file: `C:\Users\USER\AppData\Local\Temp\opencode\s1b-evidence.txt` (stdout+stderr)
- captured summary:

```
ℹ tests 21
ℹ suites 9
ℹ pass 21
ℹ fail 0
ℹ cancelled 0
ℹ skipped 0
ℹ todo 0
ℹ duration_ms 11277.2556
```

- `sha256:1927056e4bc21416147aee596bb0cf2ba8cf9c343cb40da0d66bbf7b5451fbad`

Supporting commands (all exit 0):

- `node --import ./internal/assets/pi/test/pi-tui-resolver.mjs --test internal/assets/pi/biggz-subagent-runtime.test.mjs` → exit 0 (`tests 21 / pass 21`); captured `C:\Users\USER\AppData\Local\Temp\opencode\s1b-evidence-import.txt` — proves the resolver also works as an `--import` entry point
- Forced-oracle probe (scratch file, not a deliverable): empty `APPDATA`/`USERPROFILE`/`HOME` → resolver mode `oracle`, `resolvePiTuiEntry` → `null`, oracle `visibleWidth('✅✅') = 4`, and `renderCompletion({agent:'probe',state:'completed',elapsedMs:1000}, 20)` → `✅ probe · complete…` through the oracle-resolved pi-tui
- Fidelity (inside the canonical run): oracle vs real pi-tui 0.85.1 — `visibleWidth` equal on all 11 fixtures and truncate widths equal at widths 1/2/5/10/20
- `go build ./...` → exit 0 (embed sanity for `internal/assets` `all:pi`)
- `node --check` on `biggz-subagent-runtime.js`, `biggz-subagent-runtime.test.mjs`, `test/pi-tui-resolver.mjs`, `test/pi-tui-oracle.mjs` → exit 0 ×4

RED note (honest capture): S1b extended the untracked S1a file pair, so there is no committed baseline to diff. The test hunks were authored before the runtime hunks; the first full-suite execution of the combined file ran 18/21 with the three intended failure surfaces (tool-result format, ellipsis assertion, oracle truncate branch) before the GREEN fixes, and the final hashed capture above is 21/21. No RED artifact file was persisted for this slice.

## Deviations from Design

1. **Load-time `getAllTools()` throw retires the link instead of failing closed.** Verified against pi 0.85.1 source (`dist/core/extensions/loader.js`): every action method throws `"Extension runtime not initialized. Action methods cannot be called during extension loading."` while factories run (factories execute before `runner.bindCore`). A literal fail-closed on that throw would mean the runtime NEVER registers on a real pi and the delta scenario "Contract tools registered" could never pass. Fail-closed is therefore bound to "no link can prove j0k3r absent" (settings unreadable/corrupt), and both branches are covered by tests.
2. **Ellipsis assertion strip.** Real `truncateToWidth` wraps the appended `…` in ANSI reset codes (`\u001b[0m…\u001b[0m`), so the card test asserts the ellipsis after stripping SGR codes (width assertion unchanged).
3. **Oracle mirrors the real "clip the ellipsis" branch.** `truncateToWidth('✅', 1, '…')` returns `'…'` in pi-tui (the ellipsis is clipped to the width instead of being dropped); the fidelity test found this and the oracle now mirrors it.
4. **`mode:"background"` returns a bounded unsupported error in S1b** (`available: task`), not the background path — S2 owns background delivery; the design Interfaces row stays satisfied from S2 on.
5. **`subagent_result` returns the bounded captured tail** (stderr tail, else event-type tail, ≤ `max_lines`); final assistant-text extraction (`get_last_assistant_text`/TaskStore) remains S2 as recorded in the S1a deviations.
6. **New exported seams beyond the design table** (test/consumer seams, all pure or injectable): `subagentRegistrationGate`, `readSettingsPackages`, `resolveSettingsPath`, `hasJ0k3rPackage`, `createTaskRegistry`, `isActiveTaskState`, `TASK_RING_LIMIT`, `createSubagentToolset`, `formatTaskResult`, `completionLine`, `renderCompletion`, `defaultCompletionWidth`, `createCompletionRenderer`, `COMPLETION_MESSAGE_TYPE`, `COMPLETION_GLYPHS`, `LEGACY_TOOL_RE`, `J0K3R_PACKAGE_MARKER`.

## Issues Found

- **Budget overrun again**: this slice landed at **688 authored lines** (runtime +274, test +240, resolver 78, oracle 96) versus the ~360 estimate / 400 cap. The contract surface itself (gate chain + 5 tools + ring + card + renderer + resolver + oracle + 9 test cases with fake-pi helpers) does not compress to 360 without dropping required evidence. Proposed re-cut if the cap is enforced: **3a** gate + ring + tool surface + their tests, **3b** card + renderer + resolver/oracle + fidelity test. Orchestrator decision needed.
- `subagent_result` is a bounded stub until S2 final-text extraction lands; nothing persists on disk (by design), so results are session-only.
- The resolver discovers pi installs at fixed roots (Windows npm global, `~/.pi/agent/npm`); a pi installed elsewhere must set `PI_TUI_DIR` (documented in the resolver header).

---

# S2 — Background delivery, widget, `subagent_wait`, dialog relay, cap (PR 4 of 6, WU `S2-background`)

**Work unit**: `S2-background` (stacked-to-main, PR 4 of 6)
**Mode**: Standard (store `openspec`, `strict_tdd: false`; RED captured by running the S2 test file against three mechanically reverted runtime copies — see RED note)
**Date**: 2026-09-17
**Status**: S2 complete (tasks 4.1–4.8) — ready for slice verify; line-budget decision pending

## Completed tasks

| Task | Description | Status |
|------|-------------|--------|
| 4.1 | RED dialog relay: fake child `extension_ui_request` (`ask_user_question`-class dialog) → parent presents → `extension_ui_response` answer returns, run continues; steering reaches a running child mid-run | done |
| 4.2 | GREEN RPC subset wiring: commands `prompt`/`steer`/`abort`/`get_last_assistant_text`/`extension_ui_response`; events `agent_start`, `message_update`, `tool_execution_start/end`, `agent_settled`, `auto_retry_end`, `extension_ui_request`, `extension_error`; everything else ignored | done |
| 4.3 | RED background: ids return immediately; each completion delivered without polling; widget rows `◐ <agent> · <state> · <elapsed>s`, max 2 + `… +N`, `truncateToWidth`'d; hidden when idle / `PI_SUBAGENT_CHILD=1` / `BIGGZ_PRETTY=0` | done |
| 4.4 | GREEN `mode:"background"` + completion delivery + `ctx.ui.setWidget("biggz-subagents", rows, { placement: "belowEditor" })` (options object, never a bare string) | done |
| 4.5 | RED cap: default 2; `BIGGZ_BACKGROUND_SUBAGENTS` numeric override (1 → third queues; 4 → four run; `on`/`off` → default 2; clamp ≥1); third launch `queued` FIFO → auto-starts when a slot frees; cancel of queued removes pre-spawn | done |
| 4.6 | GREEN cap + FIFO queue (`queued` state); Go four-source policy resolution UNCHANGED (no Go edits this slice) | done |
| 4.7 | RED `subagent_wait` headline: exact `Wait 23s · 2 runs (sdd-apply running, sdd-verify queued)` + at most one dim hint, ≤2 lines, no run-list dump | done |
| 4.8 | GREEN `renderWaitHeadline` + `subagent_wait` tool | done |

`tasks.md` checkboxes intentionally untouched — the orchestrator marks them.

## Files changed

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/assets/pi/biggz-subagent-runtime.js` | Modified (690 → 982, +292) | `createTask`: `start()` seam (queued tasks spawn on demand, watchdog timers arm at spawn), `steer()`, dialog relay (`extension_ui_request` select/confirm/input/editor → `options.onUiRequest` → `extension_ui_response`; idle watchdog suspended while a dialog awaits the user), `agent_settled → get_last_assistant_text → completed` with a bounded `finalTextMs` deadline, snapshot `finalText`/`pendingUi`; `resolveBackgroundCap`; `widgetRowsFor` (max 2 + `… +N`, hidden when idle/`PI_SUBAGENT_CHILD=1`/`BIGGZ_PRETTY=0`, rows `truncateToWidth`'d); `createWidgetPublisher` (attach/publish/notify/watch with an unref'd 1s ticker that self-clears when idle, placement always an options object); `renderWaitHeadline`; `presentUiRequest`; toolset: background launch + cap/FIFO `pump()` + completion delivery + `subagent_wait`, `subagent_result` prefers `finalText`; factory wires the widget + completion delivery (`pi.sendMessage` custom card + notify) and exposes the new seams on `_biggzSubagentRuntime` |
| `internal/assets/pi/biggz-subagent-runtime.test.mjs` | Modified (496 → 715, +219) | Fake child answers `get_last_assistant_text` (`final answer`), emits `extension_ui_request` under `FAKE_UI_REQUEST`, and answers a matching `extension_ui_response` with `dialog_answer` + `agent_settled`; 6 new S2 cases (dialog relay round-trip, `presentUiRequest` normalization, steering, background ids/delivery/widget, cap+FIFO+cancel-queued+resolver, widget rows, wait headline, `subagent_wait`); two S1b assertions updated to the S2 contract (six registered tools incl. `subagent_wait`; `mode:"background"` is now supported — unknown-mode error text `available: task, background`) |

No other file touched (no Go, no `tasks.md`, no deploy list, no specs/design, no `internal/assets/pi/test/*`).

## Work Unit Evidence

| Evidence | Value |
|----------|-------|
| Focused test command and exact result | `node --test internal/assets/pi/biggz-subagent-runtime.test.mjs` → exit 0; `tests 30 / suites 11 / pass 30 / fail 0 / duration_ms 13965.7669` |
| Runtime harness command/scenario and exact result | Real fake-child process over RPC JSONL through the tool surface: (a) **dialog round-trip** — child emits `extension_ui_request` (`method:"select"`, options Allow/Block), parent `onUiRequest` answers `{value:"Allow"}`, child echoes `extension_ui_response` and emits `dialog_answer` + `agent_settled`; the run completes with `finalText:"final answer"` and `pendingUi:0`; (b) **steer** — `task.steer("change of plan")` reaches the running child's stdin mid-run (echo observed); (c) **background** — two `mode:"background"` calls return ids with `state:"running"` and **zero** deliveries at return time (no awaiting/polling), each completion arrives through the delivery callback, widget rows `['◐ probe · running · 0s','◐ probe · running · 0s']` then `setWidget(key, undefined, {placement:"belowEditor"})` when idle; (d) **cap 1** — first `running`, second/third `queued` with `pid:null` (pre-spawn), cancel of the queued third removes it, the second auto-starts when the first settles (pid null→non-null), deliveries `[cancelled, completed, completed]`; (e) **`subagent_wait`** on two settled runs renders `Wait \d+s · 2 runs (probe completed, probe completed)` (≤2 lines) |
| Rollback boundary | Revert the S2 hunks in `biggz-subagent-runtime.js` + the S2 describes/helpers in the test file (the S1a/S1b hunks stay intact). The asset still ships inert (absent from the S3 deploy list), so no deployed behavior is affected; no persisted state (ring is in-memory), no Go, no settings writes |

## EVIDENCE

Canonical command (ledger input):

```
node --test internal/assets/pi/biggz-subagent-runtime.test.mjs
```

- exit code: `0`
- captured output file: `C:\Users\USER\AppData\Local\Temp\opencode\s2-evidence.txt` (stdout+stderr)
- captured summary:

```
▶ S2 dialog relay + steering
  ✔ relays a child extension_ui_request, returns the answer, and the run continues (338.8482ms)
  ✔ presents select/confirm/input through ctx.ui and normalizes the answer (0.3918ms)
  ✔ steers a running child mid-run (345.1647ms)
✔ S2 dialog relay + steering (684.8093ms)
▶ S2 background mode, widget, cap and wait headline
  ✔ returns background ids immediately and delivers each completion without polling (693.8997ms)
  ✔ caps at 1, queues FIFO, auto-starts on a free slot, and cancels queued runs pre-spawn (799.4747ms)
  ✔ resolves the cap from BIGGZ_BACKGROUND_SUBAGENTS (numeric override, on/off → 2, clamp ≥1) (0.1747ms)
  ✔ renders max 2 widget rows + `… +N`, truncates via pi-tui, and hides when idle/child/pretty-off (0.2741ms)
  ✔ renders the exact wait headline ≤2 lines and never a run-list dump (0.3155ms)
  ✔ subagent_wait waits for the given runs and returns the bounded headline (690.4174ms)
✔ S2 background mode, widget, cap and wait headline (2184.8562ms)
ℹ tests 30
ℹ suites 11
ℹ pass 30
ℹ fail 0
ℹ cancelled 0
ℹ skipped 0
ℹ todo 0
ℹ duration_ms 13965.7669
```

- `sha256:41cdf65107aa6919ddd096fc91b50a05e539086854418643fa094e604f1006f6`

Exact-headline assertion (task 4.7, inside the canonical run):

- `renderWaitHeadline([{agent:'sdd-apply',state:'running'},{agent:'sdd-verify',state:'queued'}], 23000)` MUST equal `['Wait 23s · 2 runs (sdd-apply running, sdd-verify queued)']` → **pass** (deep-equal asserted).
- The `subagent_wait` tool returns `/^Wait \d+s · 2 runs \(probe completed, probe completed\)$/` over ≤2 lines → **pass**.

Supporting commands (all exit 0):

- `go build ./...` → exit 0 (embed sanity for `internal/assets` `all:pi`)
- `node --check internal/assets/pi/biggz-subagent-runtime.js` → exit 0
- `node --check internal/assets/pi/biggz-subagent-runtime.test.mjs` → exit 0
- `go vet ./internal/assets/...` → exit 0
- Resolver mode probe: `install:C:\Users\USER\AppData\Roaming\npm\node_modules\@earendil-works\pi-coding-agent\node_modules\@earendil-works\pi-tui` — the canonical run exercised the **real** pi-tui 0.85.1, not the oracle (S1b fidelity case still green)

### RED evidence (three mechanically reverted copies of the final runtime, same test file)

| Variant | Reverted surface | Result |
|---------|------------------|--------|
| RED-A | S2 export surface absent (`resolveBackgroundCap`, `widgetRowsFor`, `createWidgetPublisher`, `renderWaitHeadline`, `presentUiRequest` demoted from `export`) | exit 1; `tests 30 / pass 26 / fail 4` — failing: background ids/delivery, cap resolver, widget rows, wait headline (`… is not a function`) |
| RED-B | `subagent` mode branch reverted to S1b (task-only) | exit 1; `tests 30 / pass 26 / fail 4` — failing: S1b mode assertion, background ids, cap+FIFO, `subagent_wait` |
| RED-C | dialog relay to the parent UI absent + `steer()` returns false | exit 1; `tests 30 / pass 28 / fail 2` — failing: dialog relay round-trip, steering |

Captured outputs: `C:\Users\USER\AppData\Local\Temp\opencode\s2-red-a.txt`, `s2-red-b.txt`, `s2-red-c.txt` (variants built under `…\Temp\opencode\s2-red\{a,b,c}`). This is mechanical RED, not authoring-order RED — the test bytes are the final ones and each variant removes exactly one S2 surface, so every new assertion is shown failing for the expected reason. The dialog relay cannot be removed entirely (the child would never answer and the test would hang), so RED-C disables the relay-to-UI link and observes the answer degrade to `cancelled` and the assertions fail.

## Deviations from Design

1. **`start()` seam + `queued` initial state.** `createTask({ autoStart: false })` creates a task without spawning (`state:"queued"`, `pid:null`, no watchdog timers); `task.start()` spawns, switches to `running` and arms the timers. Needed for the cap/FIFO requirement ("queue until a slot frees"); the S1a caller path is unchanged (`autoStart` defaults to true).
2. **`finalTextMs` bounded deadline (default 1500 ms).** `agent_settled` triggers `get_last_assistant_text`; if no matching `response` arrives within the deadline (or the write fails) the run still settles `completed` with `finalText:null`. Without the bound, a child that ignores the request would hold the run open until the total watchdog. `0` skips the round-trip.
3. **Idle watchdog suspension during a pending dialog.** While a `select`/`confirm`/`input`/`editor` request awaits the user, the idle timer is cleared and re-armed after the response is written — otherwise a slow human decision could trip the 4-minute idle kill. Not specified in design; required for the interactivity-parity intent.
4. **Widget ticker.** `createWidgetPublisher.watch(provider)` publishes every 1 s (unref'd) so `<elapsed>s` stays live, and stops itself when the row set empties. Design decision 2 mandates the rows/hidden states, not the refresh mechanism.
5. **`queued: "◌"` glyph added** to `COMPLETION_GLYPHS` so queued rows are distinguishable; `◐` remains the running glyph (spec row text).
6. **S1b test contract updates (behavior changed by design).** The S1b `mode:"background"` → bounded-unsupported assertion was explicitly temporary (S1b deviation 4: "the design Interfaces row stays satisfied from S2 on"); it now asserts the unknown-mode error (`available: task, background`), and the registered-tool list is six (incl. `subagent_wait`, required by the spec scenario "Contract tools registered").
7. **`resolveBackgroundCap` accepts a leading-numeric string** (`Number.parseInt`) and maps `on`/`off`/non-numeric to the default 2 — the four-source policy (files + env) stays owned by Go (no Go edits this slice).
8. **`subagent_wait` semantics**: `elapsed` is the time spent in the wait call; states are rendered at return; `task_ids` may reference already-settled ring entries, while omitting `task_ids` waits on active runs only.

## Issues Found

- **Budget overrun**: this slice landed at **511 authored lines** (runtime +292, test +219) versus the ~330 estimate / 400 cap. Required evidence (dialog round-trip + steer, background delivery + widget matrix, cap/FIFO/cancel-queued, wait headline + tool, plus the S1b contract updates) does not compress to 330. Proposed re-cut if the cap is enforced: **4a** RPC subset + dialog relay/steer + `presentUiRequest` + `renderWaitHeadline`/`subagent_wait` (~250 lines), **4b** background mode + widget + cap/FIFO/delivery (~260 lines). Orchestrator decision needed.
- The factory's `pi.sendMessage`/`ctx.ui.notify` delivery path is not harness-covered (the factory spawns real `pi.cmd`, which the tests must not do); the delivery contract is covered at the toolset seam with a factory-shaped callback (`deliverCompletion`), and the widget/notify path (attach → `ctx.ui.notify`) is asserted with a fake `ExtensionContext`. Honest gap, not silently skipped.
- `subagent_wait` without `task_ids` sees active runs only (a settled run leaves `registry.active()`); callers waiting on a specific finished run must pass `task_ids` (kept per the design row `task_ids?`).
- The `agent_settled` → `get_last_assistant_text` round-trip adds one stdin write per completion; the S1a fake child answers it, so no prior assertion depended on immediate settling.

## Remaining Tasks (this change, other slices)

- S3 (`S3-cutover`), S4 (`S4-harness`) — untouched in this run (tasks.md Phases 5-6).

## Workload / PR Boundary

- Mode: chained PR slice (stacked-to-main) — PR 4 of 6
- Current work unit: `S2-background`
- Boundary: starts from the S1b slice; ends at the S2 surface in the same two untracked files (runtime + tests). No deploy-list entry (inert until S3), no Go changes, no other assets, no `tasks.md` checkboxes.
- Changed lines: `292 + 219 = 511` authored additions, 0 deletions (extended untracked files). Over the 400 review budget — see Issues Found.
- Rollback: revert the S2 hunks in the two files; nothing else references the new seams.

## Status

8/8 S2 tasks complete with RED+GREEN evidence (30/30 tests, canonical sha256 recorded; three mechanical RED variants captured). Ready for slice verify; line-budget decision pending with the orchestrator.

---

# S3 — Cutover: deploy/reconcile retarget, dead code gone, prompts, doctor/probe (PR 5 of 6, WU `S3-cutover`)

**Work unit**: `S3-cutover` (stacked-to-main, PR 5 of 6)
**Mode**: Standard (store `openspec`, `strict_tdd: false`)
**Date**: 2026-09-17
**Status**: S3 complete (tasks 5.1–5.10) — ready for slice verify; delivery re-cut required (line budget, see Issues Found)

## Completed tasks

| Task | Description | Status |
|------|-------------|--------|
| 5.1 | Deploy list: `{"pi/biggz-subagent-runtime.js", "biggz-subagent-runtime.js"}` in, `biggz-wait-pretty.js` out; `biggz-session-guard.js` + `biggz-memory-chrome.js` kept; JS count stays 13 | done |
| 5.2 | Stale self-heal: `biggz-wait-pretty.js` added to the removal set (`biggz-synthesis-gate.js` kept); missing files silent no-op | done |
| 5.3 | Deleted `internal/assets/pi/biggz-wait-pretty.js` + `internal/assets/pi/subagent-config.json`; dropped `deploySubAgentConfig` call + method and `mergeJSONCWrapper` (+ orphaned `filemerge` import) | done |
| 5.4 | Dead sweep `internal/install/install.go`: deleted `DeployPiWaitPretty`, `DeployPiPrettyWrapper`, `DeployPiSubAgents`, `DeployPiSubagentConfig` (+ orphans `piSubagentConfigDir`, `parsePiSkillFrontmatter`); coverage retargeted to `PiExtensionsStep.deploySubAgents` | done |
| 5.5 | `biggz-pi-extensions-factory.test.mjs`: wait-pretty → runtime entry swap; JS count guard stays 13 | done |
| 5.6 | `internal/sdd/background.go`: `SubagentRuntimeTargetName`, `SubagentRuntimeMarkerPath`, `SubagentRuntimeCapability` added; `ResolveBackgroundSubagentsCapability` retargeted to marker ∧ j0k3r-absent (`PI_CODING_AGENT_DIR` honored) | done |
| 5.7 | `internal/agents/pi/adapter.go`: cutover reconcile — marker present → drop j0k3r from settings `packages`, `InstallCommand`; marker absent → S0 exact pin kept; capability probe delegates to `internal/sdd` | done |
| 5.8 | `internal/doctor/pi.go`: `PiSubagentsCheck` checks the runtime marker; remedy redeploys via `biggz install --agent pi` (shared `biggzInstallPi` action) | done |
| 5.9 | `biggz-orchestrator-delegation.md` lines 37/44–48/145: cites `subagent`/`subagent_wait`; `subagent_run`, native `task` fallback, FleetView and `context:"fork"` guidance dropped; documented unavailable-fallback text added | done |
| 5.10 | Gates: `go build ./...` + focused matrix + `go vet` + `gofmt` + runtime suite + `node --check` | done |

`tasks.md` checkboxes intentionally untouched — the orchestrator marks them.

## Files changed

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/install/steps/pi_extensions.go` | Modified | Deploy list: runtime entry in (`pi/biggz-subagent-runtime.js`), `biggz-wait-pretty.js` out; stale self-heal set gains `biggz-wait-pretty.js`; `deploySubAgentConfig` call + method deleted; comments updated |
| `internal/install/steps/helpers.go` | Modified | `mergeJSONCWrapper` deleted (sole callers lived in the retired `deploySubAgentConfig`); orphaned `filemerge` import removed |
| `internal/install/steps/pi_extensions_guard_test.go` | Modified | Mirrored deploy list swapped to the runtime entry (13 JS kept); new `TestPiExtensionsStep_RemovesRetiredWaitPretty` (real `Apply`: stale wait-pretty + synthesis-gate removed, runtime deployed, second Apply silent no-op) |
| `internal/install/install.go` | Modified | Deleted `DeployPiSubagentConfig` (+ orphan `piSubagentConfigDir`), `DeployPiWaitPretty`, `DeployPiPrettyWrapper`, `DeployPiSubAgents` (+ orphan `parsePiSkillFrontmatter`) — 453 lines removed, zero remaining references |
| `internal/install/pi_subagents_test.go` | Modified | Retargeted to the live path: all four former `install.DeployPiSubAgents` tests now drive `steps.PiExtensionsStep` (`Prepare` + `Apply`) with a fake pi agent; new embedded-FS test for the general/explore BigMem protocol block |
| `internal/sdd/background.go` | Modified | `SubagentRuntimeTargetName`, `SubagentRuntimeJ0k3rMarker`, `SubagentRuntimeMarkerPath`, `SubagentRuntimeCapability`; `ResolveBackgroundSubagentsCapability` delegates to the marker probe (third-party package presence alone never `ready`) |
| `internal/sdd/background_test.go` | Created | Marker/capability matrix: marker → ready; no marker → absent; j0k3r alone → absent; marker + j0k3r → absent (dual registration); corrupt settings → absent; `PI_CODING_AGENT_DIR` override honored |
| `internal/agents/pi/adapter.go` | Modified | Cutover: `InstallCommand` drops the fork when the marker is deployed; `mergePiSettingsBigMem(path, mcpBinary, homeDir)` purges every j0k3r spec at cutover and keeps the pin until then; capability probe delegates to `internal/sdd`; dead `backgroundCapability*` consts removed |
| `internal/agents/pi/subagents_fork_test.go` | Modified | Pre-cutover pin test made hermetic (`PI_CODING_AGENT_DIR` temp dir); new cutover tests: `InstallCommand` fork-free + idempotent, settings reconcile purge + idempotent + revert-restores-pin, capability delegation |
| `internal/agents/pi/adapter_test.go` | Modified | `TestInstallCommand_ContainsAdapterBeforeSubagents` pinned to a temp `PI_CODING_AGENT_DIR` (no marker → pre-cutover list stays deterministic) |
| `internal/doctor/pi.go` | Modified | `PiSubagentsCheck` rewritten to stat the runtime marker (pass when present, warn + `biggz install --agent pi` when missing); shared `biggzInstallPi` remedy action; `execFn` probe dropped |
| `internal/doctor/pi_subagents_test.go` | Modified | Retargeted to marker semantics: marker → pass, missing → warn with install hint, pi absent → skip, `PI_CODING_AGENT_DIR` override → pass, remedy names `biggz install --agent pi` |
| `internal/assets/pi/biggz-pi-extensions-factory.test.mjs` | Modified | Deploy-list mirror swapped to the runtime entry; count guard still 13 |
| `internal/assets/pi/biggz-wait-pretty.js` | Deleted | Retired with the j0k3r fork (230 lines) |
| `internal/assets/pi/subagent-config.json` | Deleted | Retired with the j0k3r config deploy (11 lines) |
| `internal/assets/biggz/biggz-orchestrator-delegation.md` | Modified | Lines 37/44–48/145 now cite `subagent`/`subagent_wait`; retired names/affordances removed; documented unavailable-fallback text |
| `openspec/changes/pi-subagent-runtime/apply-progress.md` | Modified | This S3 section (merged — S0/S1a/S1b/S2 sections preserved) |

No other file touched (no `tasks.md`, specs, design, proposal, runtime JS/test files, pre-existing gatekeeper worktree changes).

## Work Unit Evidence

| Evidence | Value |
|----------|-------|
| Focused test command and exact result | `go test ./internal/install/steps -run TestPiExtensions -count=1` → exit 0; 6/6 PASS (`DropsUnportableExtensions`, `Guard_FactoryExport`, `Step_RemovesRetiredWaitPretty`, `Prepare_RejectsBrokenFactory`, `Guard_ValidateHelperCoversAll`, `Step_SkipNonPi`) |
| Runtime harness command/scenario and exact result | Temp-HOME reconcile harness on the real `ProvisionBigMemMCP` → `mergePiSettingsBigMem` path: settings seeded with `npm:pi-subagents-j0k3r@1.6.1` + floating `npm:pi-subagents-j0k3r` + `npm:@heyhuynhgiabuu/pi-pretty`, runtime marker deployed at `~/.pi/agent/extensions/biggz-subagent-runtime.js` → after run zero j0k3r entries and pretty preserved; second run byte-stable count; **marker removed + re-run → exact `@1.6.1` pin restored** (rollback). Companion real-`Apply` harness (`TestPiExtensionsStep_RemovesRetiredWaitPretty`): stale `biggz-wait-pretty.js` + `biggz-synthesis-gate.js` pre-seeded in a temp HOME → both removed, `biggz-subagent-runtime.js` deployed, second `Apply` silent no-op. `InstallCommand` cutover (`PI_CODING_AGENT_DIR` temp override + marker) → no j0k3r entry, rest of list intact, idempotent. Doctor: marker file → PASS; absent → WARN naming `biggz install --agent pi`. **NOT executed**: a real `biggz install --agent pi` end-to-end (code-only slice — machine activation is a checkpoint decision); the temp-HOME harness covers the reconcile path |
| Rollback boundary | Revert the S3 hunks in the ten Go files + doc + factory mjs and restore the two deleted assets; the fork pin returns on the next reinstall (proven by the revert leg of the reconcile harness). No persisted state, no settings written outside tests, no machine activation |

## EVIDENCE

Canonical combined command (ledger input; sequential, one file):

```
go test ./internal/install/... ./internal/agents/pi ./internal/sdd ./internal/doctor -count=1
node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs
```

- exit codes: `go` = `0`, `node` = `0`
- captured output file: `C:\Users\USER\AppData\Local\Temp\opencode\s3-cutover-evidence.txt` (stdout+stderr)
- captured output (head):

```
ok  	github.com/biggs-100/biggz-ai/internal/install	12.422s
ok  	github.com/biggs-100/biggz-ai/internal/install/steps	6.633s
ok  	github.com/biggs-100/biggz-ai/internal/agents/pi	1.507s
ok  	github.com/biggs-100/biggz-ai/internal/sdd	36.967s
ok  	github.com/biggs-100/biggz-ai/internal/doctor	2.361s
```

- captured output (tail — factory guard, 13 JS entries preserved):

```
✔ pi extensions must export valid factory (9.5238ms)
ℹ tests 14
ℹ suites 1
ℹ pass 14
ℹ fail 0
ℹ duration_ms 92.5458
```

- `sha256:29D75A1B0AEC58B21BA323C7A4E6A45B33251289073440089B963CA5EE860F70`

Supporting commands (all exit 0):

- `go build ./...` → exit 0 (embed sanity for `internal/assets` `all:pi`; deleted assets no longer referenced)
- `go vet ./internal/install/... ./internal/agents/pi ./internal/sdd ./internal/doctor` → exit 0
- `go vet ./...` → exit 0 (repo-wide, compiles every test package — catches any leftover reference to the deleted exports)
- `gofmt -l` on the twelve changed/created Go files → empty output, exit 0
- `node --test internal/assets/pi/biggz-subagent-runtime.test.mjs` → exit 0; `tests 30 / pass 30 / fail 0` (runtime suite stays green); captured `C:\Users\USER\AppData\Local\Temp\opencode\s3-runtime-suite.txt`, `sha256:68B3598559983CC3F092528D60B892DFF819D230EF464EC3EDF17C2B291ED6F7`
- `node --check internal/assets/pi/biggz-pi-extensions-factory.test.mjs` → exit 0 (only JS touched this slice; no production JS changed)
- Task 5.1 evidence: `go test ./internal/install/steps -run TestPiExtensions -count=1` → exit 0
- Task 5.6 evidence: `TestSubagentRuntimeCapability_*` + `TestResolveBackgroundSubagentsCapability_DelegatesToMarkerOwner` → 7/7 PASS inside the canonical run (marker → ready; no marker → absent; j0k3r alone → absent)
- Task 5.7 evidence: `TestInstallCommand_CutoverDropsJ0k3r`, `TestSettingsReconcile_CutoverDropsJ0k3r`, `TestResolveBackgroundSubagentsCapability_DelegatesToMarkerOwner` → PASS inside the canonical run
- `use-modern-go` CLI `list` (Go 1.25 guideline set) consulted for the edited Go files; `any` used for JSON maps, `slices.DeleteFunc` + `strings.Contains` for the cutover filter, no manual min/max/sort/map-copy loops introduced

## Deviations from Design

1. **Orphans deleted beyond the four named helpers.** `piSubagentConfigDir` and `parsePiSkillFrontmatter` became reference-free once `DeployPiSubagentConfig`/`DeployPiSubAgents` were removed; both were deleted (same class as the explicitly named `mergeJSONCWrapper` orphan) to satisfy the "dead code gone" goal. `backgroundCapabilityReady/Absent` consts in the pi package were also deleted with the delegated probe.
2. **`SubagentRuntimeJ0k3rMarker` added to `internal/sdd`** (fragment `pi-subagents-j0k3r`) as the single owner-side marker string used by both the capability probe and the adapter's cutover purge — design named three identifiers; this is a small fourth that keeps one source of truth.
3. **`mergePiSettingsBigMem` gained a `homeDir` parameter** (unexported, single production caller) because the cutover decision needs the marker path, which is home-derived (`PI_CODING_AGENT_DIR` honored).
4. **Doctor check rewritten, not just retargeted.** `PiSubagentsCheck` now stats the runtime marker; the j0k3r package probing (`npm list -g`, PATH, node_modules candidates) and the `execFn` injection were dropped with it. The remedy action was extracted to `biggzInstallPi` (also reused by `PiLastModelCheck.Remedy`) so the `biggz install --agent pi` invocation lives once.
5. **Capability fail-closed on corrupt settings.** Mirroring the runtime's registration gate, unreadable/corrupt `settings.json` counts as "cannot prove j0k3r absent" → `absent`; a missing `settings.json` proves nothing installed → marker alone decides.
6. **Retargeted install tests split the fallback-agent protocol assertion.** The live `PiExtensionsStep.deploySubAgents` injects the BigMem protocol into `general`/`explore` only when the FS is the embedded `assets.FS` (the sdd-* branch also falls back to a custom FS). Live behavior unchanged; the mock-FS test asserts deployment shape and a new embedded-FS test asserts the protocol block.

## Issues Found

- **Budget overrun (delivery re-cut required)**: this slice landed at **~1614 authored changed lines** (`+591 / −1023`; cumulative scoped diff minus the S0 slice) versus the ~350 estimate / 400 budget. Deletions dominate and were mandated: `install.go` −453, `biggz-wait-pretty.js` −244, j0k3r doctor probing −138, retargeted install tests −40. Proposed re-cut if the cap is enforced: **5a** cutover code + tests (`pi_extensions.go`, `helpers.go`, `background.go`(+test), `adapter.go`(+tests), `doctor/pi.go`(+test), factory mjs, guard test, delegation doc) ≈ `+591/−45`; **5b** dead sweep + retired assets + retargeted install tests (`install.go`, both deleted assets, `pi_subagents_test.go`) ≈ `+121/−1023`. Orchestrator decision needed.
- **Install ordering makes the injected policy line stale on the first post-cutover run (NOT fixed)**: `OverlayStep.deployPersona` renders `{{BIGGZ_BACKGROUND_POLICY}}` (via `RenderBackgroundSubagentsStatusLine`) **before** `PiExtensionsStep` deploys the runtime marker, so the first `biggz install --agent pi` injects `capability: absent`; a second install (or `biggz doctor --fix`) renders `capability: ready`. Everything else is deployed on the first run. Fixing this means reordering the shared pipeline plan (skills → overlay → state → pi), which affects rollback semantics for all agents — out of this slice's scope; recommend an explicit follow-up decision.
- **No end-to-end `biggz install --agent pi` executed** (prohibited: code-only slice). The S3 runtime-harness row ("temp-HOME `biggz install --agent pi` → doctor PASS") is deferred to the checkpoint/S4 smoke.
- `ResolvePackageBin` (`internal/agents/pi/model_routing.go`) lost its last production caller (the old capability probe) but stays exported with dedicated tests (`model_routing_test.go`) — left untouched, out of scope.
- `internal/assets/pi/biggz-pi-pretty.js` remains on disk although its only deployer (`DeployPiPrettyWrapper`) was deleted in this sweep — outside the mandated deletion list; candidate follow-up deletion.
- The main spec `openspec/specs/agent-install/spec.md:117` still names `DeployPiSubAgents`; the change's delta (`REQ-INST-001` MODIFIED) replaces it at archive/sync time. No spec files touched here.

## Remaining Tasks (this change, other slices)

- S4 (`S4-harness`) — untouched in this run (tasks.md Phase 6).

## Workload / PR Boundary

- Mode: chained PR slice (stacked-to-main) — PR 5 of 6
- Current work unit: `S3-cutover`
- Boundary: starts from the S2 slice (runtime inert, not deployed); ends at the runtime deployed by the install path, j0k3r retired from install/reconcile/doctor/prompt, dead code and retired assets removed. No machine activation (`biggz install` not run), no commits, no `tasks.md` checkboxes.
- Changed lines: **~1614** authored (`+591 / −1023`) — over the 400 review budget; re-cut proposal above.
- Rollback: revert the S3 hunks + restore the two assets; reinstall restores `npm:pi-subagents-j0k3r@1.6.1` (proven by the revert leg of the reconcile harness).

## Status

10/10 S3 tasks complete with the canonical sha256 recorded and the runtime suite still 30/30. Ready for slice verify; delivery re-cut decision pending with the orchestrator.

---

# S4 — Width regression harness + smoke (PR 6 of 6, WU `S4-harness`)

**Work unit**: `S4-harness` (stacked-to-main, PR 6 of 6 — final slice)
**Mode**: Standard (store `openspec`, `strict_tdd: false`)
**Date**: 2026-09-17
**Status**: S4 complete (tasks 6.1–6.3) — ready for slice verify; the real-pi activation run is DEFERRED with rationale (checklist below)

## Completed tasks

| Task | Description | Status |
|------|-------------|--------|
| 6.1 | `internal/assets/pi/biggz-subagent-width.test.mjs`: emoji/CJK/Hangul/ZWJ/ANSI/OSC fixtures; deterministic crash shape (188 cp, two `✅`, `visibleWidth === 190`) rendered ≤ width for EVERY width 20–200; property `moduleMeasure(line) >= realPiTui.visibleWidth(line)`; resolver/oracle reused | done |
| 6.2 | Source scan inside the same file: `internal/assets/pi/*.js` width math only via pi-tui imports (naive `[...str].length`-class detection + runtime positive/negative assertions); oracle fidelity vs real pi-tui 0.85.1 on all fixture families | done |
| 6.3 | Smoke (harness legs, evidence below): temp-HOME E2E — runtime deployed, wait-pretty self-healed, 3 installs idempotent; real-pi emoji-heavy run DEFERRED (activation checklist) | done (harness) + deferred (real-pi) |

`tasks.md` checkboxes intentionally untouched — the orchestrator marks them.

## Files changed

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/assets/pi/biggz-subagent-width.test.mjs` | Created (237 lines) | 11 `node --test` cases in 4 suites: (a) resolver resolution + crash shape (deterministic reconstruction from `completionLine`'s own prefix, zero hardcoded log string) + width sweep 20–200 for the crash shape and all 16 fixtures; (b) never-under-measure property (`moduleMeasure` and the oracle vs the real pi-tui); (c) oracle fidelity (equality on `visibleWidth` + `truncateToWidth`) on every fixture family, skip-with-reason when no install; (d) asset source scan (5 naive-width patterns, per-file baseline allowlist, scanner self-test, runtime import assertion, allowlist freshness) |
| `openspec/changes/pi-subagent-runtime/apply-progress.md` | Modified | This S4 section (merged — S0/S1a/S1b/S2/S3 sections preserved) |

No other file touched (no runtime JS/test, no resolver/oracle, no Go, no `tasks.md`, no specs/design/proposal, no `~/.pi` on the real machine).

## Work Unit Evidence

| Evidence | Value |
|----------|-------|
| Focused test command and exact result | `node --test internal/assets/pi/biggz-subagent-width.test.mjs` → exit 0; `tests 11 / suites 4 / pass 11 / fail 0 / skipped 0 / duration_ms 340.3284`; resolver mode `install:C:\Users\USER\AppData\Roaming\npm\node_modules\@earendil-works\pi-coding-agent\node_modules\@earendil-works\pi-tui` (the REAL pi-tui 0.85.1, not the oracle) |
| Runtime harness command/scenario and exact result | Temp-HOME binary E2E (details in the 6.3 block): `go build -o %TEMP%\opencode\biggz-s4-smoke.exe ./cmd/biggz` → exit 0, then `biggz-s4-smoke.exe install --home <tempHome> --agent pi --yes` ×3 → exit 0 each; `biggz-subagent-runtime.js` deployed (39982 bytes) and stale pre-seeded `biggz-wait-pretty.js` removed on run 1; extensions dir snapshot identical runs 1↔2; 84-file `.pi\agent` surface byte-identical after run 3; `pi` CLI shadowed by a logging no-op shim → zero real-machine `pi install`; real `~/.pi/agent/extensions` unchanged (runtime absent, wait-pretty still present — read-only check) |
| Rollback boundary | Delete `internal/assets/pi/biggz-subagent-width.test.mjs` (the only repo change). Smoke artifacts live only under `%TEMP%\opencode\s4-*` (throwaway home, shim, logs, exe) and can be deleted with them. Nothing else in the repo changed; no deploy surface touched |

## EVIDENCE

Canonical command (ledger input):

```
node --test internal/assets/pi/biggz-subagent-width.test.mjs
```

- exit code: `0`
- captured output file: `C:\Users\USER\AppData\Local\Temp\opencode\s4-width-evidence.txt` (stdout+stderr)
- captured output:

```
▶ S4 width harness: fixtures and the exact crash shape
  ✔ resolves a real pi-tui or the test-only oracle before the runtime imports it (0.8081ms)
  ✔ reconstructs the crash shape deterministically: 188 code points, two ✅, true width 190 (18.8843ms)
  ✔ renders the crash shape ≤ width for EVERY width 20–200 (19.2504ms)
  ✔ renders every fixture family ≤ width for EVERY width 20–200 (74.9124ms)
▶ S4 width property: never under-measure (measure ≥ real pi-tui)
  ✔ moduleMeasure(line) >= piTui.visibleWidth(line) for every fixture line and the crash line (1.6363ms)
  ℹ real pi-tui: C:\Users\USER\AppData\Roaming\npm\node_modules\@earendil-works\pi-coding-agent\node_modules\@earendil-works\pi-tui\dist\index.js
▶ S4 resolver fidelity against the real pi-tui
  ✔ the oracle matches the real pi-tui on every fixture family when installed (5.7306ms)
▶ S4 source scan: width measurement only via pi-tui imports
  ✔ has a non-empty asset corpus including the module under test (0.2123ms)
  ✔ flags independent width math outside the recorded baseline with a clear message (2.8905ms)
  ✔ scanner self-test: catches the #25 defect class and stays silent on the pi-tui import path (0.32ms)
  ✔ the runtime measures only through its pi-tui import (positive + negative) (0.5323ms)
  ✔ keeps the allowlist honest: every recorded exempt file still exists (0.4063ms)
ℹ tests 11
ℹ suites 4
ℹ pass 11
ℹ fail 0
ℹ cancelled 0
ℹ skipped 0
ℹ todo 0
ℹ duration_ms 340.3284
```

- `sha256:7d5e7203ebbea8e6edd7061763527ff688c655767ff94403601c340d0b06938b`

Supporting commands (all exit 0):

- `node --test internal/assets/pi/biggz-subagent-runtime.test.mjs` → `tests 30 / suites 11 / pass 30 / fail 0` (stays green); captured `C:\Users\USER\AppData\Local\Temp\opencode\s4-runtime-suite.txt`
- `node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs` → `tests 14 / suites 1 / pass 14 / fail 0`; captured `C:\Users\USER\AppData\Local\Temp\opencode\s4-factory-suite.txt`
- `node --check internal/assets/pi/biggz-subagent-width.test.mjs` → exit 0
- `go build ./...` → exit 0 (embed sanity: the new `.mjs` rides the embedded `all:pi` FS)
- `go test ./internal/install/steps -run TestPiExtensions -count=1` → `ok ... 1.674s` (guard/factory/deploy-list checks stay green with the new asset present)

### 6.3 Smoke — temp-HOME install E2E (observed values)

Exact commands (throwaway home + shim created by `%TEMP%\opencode\s4-smoke.ps1`; combined capture `s4-smoke-evidence.txt`, `sha256:a62d40eb492680b260cc514832b95bc1552471049ba53e44131ea6c608936745`):

```
go build -o %TEMP%\opencode\biggz-s4-smoke.exe ./cmd/biggz
$env:PATH = "%TEMP%\opencode\s4-smoke-bin;<rest>"      # logging no-op pi.cmd shim (see rationale)
$env:PI_CODING_AGENT_DIR = "<tempHome>\.pi\agent"
& biggz-s4-smoke.exe install --home <tempHome> --agent pi --yes    # run 1
& biggz-s4-smoke.exe install --home <tempHome> --agent pi --yes    # run 2
& biggz-s4-smoke.exe install --home <tempHome> --agent pi --yes    # run 3 (extra idempotence surface)
```

| Check | Run 1 | Run 2 | Run 3 |
|-------|-------|-------|-------|
| exit code | `0` | `0` | `0` |
| `biggz-subagent-runtime.js` in `<tempHome>\.pi\agent\extensions\` | `True` (39982 bytes, `sha256:A6394899…`) | `True` (same hash) | `True` (same hash) |
| pre-seeded stale `biggz-wait-pretty.js` | removed (`False`; self-healed) | `False` | `False` |
| extensions inventory | 13 `.js` + 3 `.ts` (= deploy list; wait-pretty/pi-pretty absent) | identical | identical |
| idempotence | — | extensions snapshot byte-identical runs 1↔2 | 84-file `.pi\agent` surface byte-identical after run 3 |
| capability line (`capability: ready|absent`) | **not present** | **not present** | not present (see deviations) |

- **Documented pipeline-order caveat — actual observed value**: the expected `capability: absent` (run 1) → `capability: ready` (run 2) transition did NOT reproduce, because **no capability line renders at all**: the `{{BIGGZ_BACKGROUND_POLICY}}` placeholder lives in `internal/assets/biggz/biggz-orchestrator-delegation.md:143`, which the pi pipeline does not deploy or inline (whole-home search: placeholder absent, `capability:` absent after every run), while the substitution in `internal/install/install.go:808-811` reads `biggz/biggz-orchestrator.md` (which no longer contains the placeholder). Recorded as a finding; not fixed (out of slice scope, code untouched).
- **`pi` shim rationale (honest note)**: `install.Run` executes `adapter.InstallCommand(null)` via `cmd /c pi install …` with inherited env (`internal/install/install.go:229-254`). To guarantee "temp HOME only — never touch the real machine", `pi` was shadowed on PATH by `%TEMP%\opencode\s4-smoke-bin\pi.cmd`, which logs its argv and exits 0. Shim log (10 lines, 5 per run — identical in runs 1–3): `install npm:pi-mcp-adapter@2`, `…rpiv-ask-user-question`, `…rpiv-todo`, `…pi-web-access`, `…pi-btw`, each with `PI_CODING_AGENT_DIR=<tempHome>\.pi\agent`. **No `pi-subagents-j0k3r` command ever ran** → the S3 cutover drops the fork at binary level once the marker is deployed (cutover observed E2E, first time outside unit tests). The shim is a no-op, so no package was installed anywhere; the machine's pi was never executed.
- Real-machine read-only verification (post-smoke): `~/.pi/agent/extensions/biggz-subagent-runtime.js` → `False` (never deployed there); `~/.pi/agent/extensions/biggz-wait-pretty.js` → `True` (pre-S4 state untouched); `~/.pi/agent/extensions/biggz-synthesis-gate.js` → `False`. Zero real-home mutation.
- Run 3 capture: `C:\Users\USER\AppData\Local\Temp\opencode\s4-smoke-run3.txt`, `sha256:15c833ccfd9f10ee661c2efd0e3ccf27a030e4e5e910e3b9fb4e7ec395c20139`.

### Deferred activation checklist (6.3 — real-pi run, NOT executed, NOT faked)

- [ ] After activating the runtime on the real machine (`biggz install --agent pi` + pi restart — a checkpoint decision, out of apply scope): run one emoji-heavy delegated SDD phase and confirm **zero** `pi-tui-crash.log` writes; confirm the two-task background notice renders (`mode:"background"` ×2 → completion notices + widget rows).
- Rationale: the check requires the runtime deployed on the real machine; this run is forbidden from touching the real `~/.pi` (temp HOME only). The temp-HOME E2E above covers every de-machineable leg (deploy, self-heal, idempotence, cutover at command level, width harness).

## Deviations from Design

1. **Source-scan baseline allowlist extends beyond the oracle/resolver.** The instruction named the oracle/resolver as the only allowed files, but the pre-existing corpus already carries independent width math that this slice is forbidden to modify: `biggz-footer.js` (codepoint `visibleWidth` fallback, deployed), `biggz-question-mouse.js` (documented ASCII-art fallback, deployed), `biggz-pi-pretty.js` (naive counter; retired asset, no deployer since S3). The scan is implemented as a **ratchet**: those three are allowlisted with per-file rationale plus oracle/resolver, any OTHER `*.js` with independent width math fails with file:line + pattern, the runtime is asserted positively (pi-tui import) and negatively (no naive patterns), and stale allowlist entries fail the suite. Follow-ups recorded in Issues Found.
2. **Crash shape reconstructed from the module's own prefix, not brute-forced.** `crashShape()` derives the `completionLine` prefix and pads the summary arithmetically to exactly 188 code points (S1b's version scanned candidate lengths). Same asserted shape (`188 cp / 2 ✅ / width 190`), no hardcoded fixture string — as tasked.
3. **Capability-line observation recorded as "not rendered"** instead of the expected absent→ready pair (see 6.3 caveat). Honest observation over expected narrative.
4. **Fidelity leg asserts equality (oracle ≡ real), the property leg asserts ≥ (`measure >= real`).** Both as specified (design "fidelity"; task 6.1 property); the ≥ direction is the safety contract, equality is the version-pinned fidelity check with skip-with-reason when no install.

## Issues Found

- **Dead policy substitution — capability line never renders (NEW, not fixed)**: `internal/install/install.go:808-811` replaces `{{BIGGZ_BACKGROUND_POLICY}}` inside `biggz/biggz-orchestrator.md`, but the placeholder only exists in `biggz/biggz-orchestrator-delegation.md:143` (never inlined/deployed by the pi pipeline). Consequence: the S3 caveat (first install `capability: absent` → second `ready`) cannot occur; the delegation doc's policy text is never materialized. Follow-up candidates: (a) deploy/render the delegation doc, or (b) move the placeholder back into the thin orchestrator; either way the substitution or the asset is dead weight today.
- **Pre-existing independent width math in deployed assets**: `biggz-footer.js:168-199` and `biggz-question-mouse.js:80-105` implement codepoint-based `visibleWidth`/`truncateToWidth` (footer prefers `Bun.stringWidth`, question-mouse documents its approximation). Same defect class as j0k3r #25, but out of this slice's edit surface; the ratchet allowlist keeps them visible. Follow-up candidates (separate change): route both through `@earendil-works/pi-tui`, delete `biggz-pi-pretty.js` (already recorded in S3).
- **Smoke needed a `pi` shim** to satisfy "temp HOME only": the install binary execs real `pi install …` commands with inherited env; without the shim those would have targeted the machine's pi/npm dirs. Shim + `PI_CODING_AGENT_DIR`+`--home` gave a fully isolated run; documented above so the evidence is not mistaken for a real package install.
- `%TEMP%\opencode\s4-*` artifacts (throwaway home, shim, exe, captured logs) are disposable and not part of the deliverable.

## Remaining Tasks (this change, other slices)

- None — S4 is the final slice. The change awaits slice verify, delivery re-cut confirmation, and archive; the deferred real-pi activation item above belongs to the activation checkpoint.

## Workload / PR Boundary

- Mode: chained PR slice (stacked-to-main) — PR 6 of 6 (**final**)
- Current work unit: `S4-harness`
- Boundary: starts from the S3 slice (runtime deployed by the install path); ends at the width regression harness + smoke evidence. One new file plus this report; zero production behavior changes, no Go, no runtime edits.
- Changed lines: `237` authored code (new harness file) + `147` report lines appended to this ledger (documentation, same file every slice merges into). Code authored this slice: `237` — within the 400-line review budget (`S4` ≈220 estimate + report).
- Rollback: delete `internal/assets/pi/biggz-subagent-width.test.mjs`; smoke artifacts live only under `%TEMP%\opencode`.

## Status

3/3 S4 tasks complete (6.1 + 6.2 fully; 6.3 harness legs a–c done and evidenced, real-pi activation item deferred with rationale). Final slice — ready for verify.

---

# Remediation — Disabled/unmanaged notice (work unit `change-verify`)

**Trigger**: change-level verify BLOCKED on the single scenario `runtime` → `Background Capability Probe and Disabled Reporting` → `Disabled reporting when policy off` (failed evidence `sha256:6c9c57cfefcea00e937b1030cc7f01ffd6d2ee11acad4e4d0852a481f605f806`). Human decision: implement the notice (option A) instead of amending the spec.
**Mode**: Standard (store `openspec`, `strict_tdd: false`).
**Date**: 2026-09-17 · **Attempt**: acquired by the orchestrator under `change-verify` (ledger untouched by this run).

## What changed

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/sdd/background.go` | Modified | Owner renderer `RenderBackgroundSubagentsReport`: new `backgroundSubagentsDisabled(policy, capability)` predicate (`off` = disabled, capability ≠ `ready` = unmanaged); when true, appends the one-line `Background subagents are disabled/unmanaged (policy: <p>, capability: <c>): background launches stay inert until the runtime is deployed (biggz install --agent pi) and the policy is turned on.` notice; the report `Type` becomes `warning` in that state. Status-line format, malformed/outranks/env lines unchanged |
| `internal/agents/pi/adapter.go` | Modified | Twin converted to a thin delegate (preferred over mirroring): `BackgroundSubagentsPolicy`/`BackgroundSubagentsSource`/`BackgroundSubagentsResolution`/`LoadBackgroundSubagentsOptions`/`BackgroundSubagentsReport` are now type aliases of the `sdd` owner types; the local `renderBackgroundSubagentsReport` body (~37 duplicate lines) and the `String()` method (illegal on an alias) and the now-dead `describeBackgroundSubagentsSource` were deleted; `renderBackgroundSubagentsReport` = `return sdd.RenderBackgroundSubagentsReport(r, capability, wrote)`. Exported wrappers and the pi-local policy resolver are unchanged |
| `internal/sdd/background_test.go` | Modified | `TestRenderBackgroundSubagentsReport_Disabled` (exact THEN: `policy: off`, `capability: absent`, `disabled/unmanaged`, `type: warning`, normative status line intact) + `TestRenderBackgroundSubagentsReport_DisabledStateMatrix` (off+absent / off+ready / on+absent → notice+warning; on+ready → no notice, `info`) |
| `internal/agents/pi/adapter_test.go` | Modified | `TestRenderBackgroundSubagentsReport_DisabledUnmanagedTwin`: token + warning assertions through the pi entry point, plus struct equality with `sdd.RenderBackgroundSubagentsReport` (anti-drift guard) |

No other file touched (no `tasks.md`, specs, design, proposal, runtime JS/JS tests, no commits; pre-existing gatekeeper worktree changes untouched).

## Work Unit Evidence

| Evidence | Value |
|----------|-------|
| Focused test command and exact result | `go test ./internal/sdd -run TestRenderBackgroundSubagentsReport -count=1 -v` → exit 0 (2 tests + 4 subtests PASS); `go test ./internal/agents/pi -run TestRenderBackgroundSubagentsReport -count=1 -v` → exit 0 (2 tests PASS, incl. the pre-existing malformed case) |
| Runtime harness command/scenario and exact result | Scenario re-probe via the verify's Go `-overlay` pattern (no repo file added): scratch tests mapped to `internal/sdd/zz_remediation_probe_test.go` + `internal/agents/pi/zz_remediation_probe_test.go`. Output: `REMEDIATION-CHECKS>>> policy_off_literal=true capability_absent=true disabled_literal=true unmanaged_literal=true disabled_unmanaged_phrase=true type=warning`; enabled control `notice_present=false type=info`; twin `REMEDIATION-TWIN-CHECKS>>> policy_off_literal=true capability_absent=true disabled_unmanaged_phrase=true type=warning` |
| Rollback boundary | Restore the two renderer implementations (owner notice + pi duplicate) and the pi local structs/method; tests revert with them. No persisted state, no settings, no JS, no machine activation |

## EVIDENCE

Canonical command (ledger input):

```
go test ./internal/sdd ./internal/agents/pi -count=1
```

- exit code: `0`
- captured output file: `C:\Users\USER\AppData\Local\Temp\opencode\remediation-pi-subagent-runtime\canonical-go-test.txt` (stdout+stderr)
- captured output:

```
ok  	github.com/biggs-100/biggz-ai/internal/sdd	35.740s
ok  	github.com/biggs-100/biggz-ai/internal/agents/pi	1.125s
```

- `sha256:E3913C5264C3214C51DE2188AB293DFA38E2D1EDDE9F543010F93F4105284FA4`

Focused verbose runs (new tests):

```
go test ./internal/sdd -run TestRenderBackgroundSubagentsReport -count=1 -v
=== RUN   TestRenderBackgroundSubagentsReport_Disabled
--- PASS: TestRenderBackgroundSubagentsReport_Disabled (0.00s)
=== RUN   TestRenderBackgroundSubagentsReport_DisabledStateMatrix
    --- PASS: .../policy_off_+_capability_absent_is_disabled_and_unmanaged (0.00s)
    --- PASS: .../policy_off_+_capability_ready_is_disabled (0.00s)
    --- PASS: .../policy_on_+_capability_absent_is_unmanaged (0.00s)
    --- PASS: .../policy_on_+_capability_ready_stays_informational (0.00s)
PASS
ok  	github.com/biggs-100/biggz-ai/internal/sdd	0.144s

go test ./internal/agents/pi -run TestRenderBackgroundSubagentsReport -count=1 -v
=== RUN   TestRenderBackgroundSubagentsReport_Malformed
--- PASS: TestRenderBackgroundSubagentsReport_Malformed (0.00s)
=== RUN   TestRenderBackgroundSubagentsReport_DisabledUnmanagedTwin
--- PASS: TestRenderBackgroundSubagentsReport_DisabledUnmanagedTwin (0.00s)
PASS
ok  	github.com/biggs-100/biggz-ai/internal/agents/pi	0.763s
```

- exit codes: `0`, `0`; captured `…\remediation-pi-subagent-runtime\focused-tests.txt`; `sha256:7F9D8AEB01D0804303E763D4A4B8E1BA58A7C9A6DD547C0F6BFDAFCCC5FBFD7A`

Fresh scenario re-probe (overlay; output verbatim):

```
REMEDIATION-RENDER>>>
background subagents: off (decided by built-in default; capability: absent)
Background subagents are disabled/unmanaged (policy: off, capability: absent): background launches stay inert until the runtime is deployed (biggz install --agent pi) and the policy is turned on.
Resolution order (first hit wins): project file, global file, BIGGZ_BACKGROUND_SUBAGENTS, built-in default off.
<<<
REMEDIATION-CHECKS>>> policy_off_literal=true capability_absent=true disabled_literal=true unmanaged_literal=true disabled_unmanaged_phrase=true type=warning
```

- probe exits: `0`; captured `…\remediation-pi-subagent-runtime\probe-evidence.txt`; `sha256:476CF225B447824EB939F20C84B17948D0BFE448D2AD8766C4440B23BDEA1315`

Supporting commands (all exit 0):

- `go build ./...` → exit 0
- `go vet ./internal/sdd ./internal/agents/pi` → exit 0
- `gofmt -l` on the four changed Go files → empty output, exit 0
- captured `…\remediation-pi-subagent-runtime\supporting.txt`; `sha256:6FBA81749E2AFFB87A955C699AA1017269369C6FE26AE9339E2CEC337D369248`
- `node --test internal/assets/pi/biggz-subagent-runtime.test.mjs` → exit 0 (`tests 30 / pass 30 / fail 0`)
- `node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs` → exit 0 (`tests 14 / pass 14 / fail 0`)
- `node --test internal/assets/pi/biggz-subagent-width.test.mjs` → exit 0 (`tests 11 / pass 11 / fail 0`)
- node exits `0,0,0`; captured `…\remediation-pi-subagent-runtime\node-suites.txt`; `sha256:2F3637C5725E21E4BA5ABDD431B31B61194E547329CDEC0CEE82D9C4DD50FDF1`
- `use-modern-go` CLI `list` consulted for `internal/sdd/background.go`; no returned guideline applies to the added predicate/notice (no loops, maps, goroutines, JSON tags)

## Diff (exact locations)

- `internal/sdd/background.go:230-236` — new `backgroundSubagentsDisabled` predicate; `:239-244` — notice appended right after the normative status line; `:271` — `tp = "warning"` when `r.Malformed || outranks || disabled`.
- `internal/agents/pi/adapter.go:729-748` — exported types become `sdd` aliases; `String()` method (pre-edit `:882`) deleted (illegal on an alias); `describeBackgroundSubagentsSource` deleted (dead); `:890-895` — renderer delegates to `sdd.RenderBackgroundSubagentsReport`.
- `internal/sdd/background_test.go:139-157` (`TestRenderBackgroundSubagentsReport_Disabled`), `:158-188` (matrix).
- `internal/agents/pi/adapter_test.go:109-129` (`TestRenderBackgroundSubagentsReport_DisabledUnmanagedTwin`).

## Deviations from Design

1. **Twin delegated instead of mirrored (choice + rationale).** The scope preferred a thin delegate "if low-risk". The alias conversion is compile-checked (`go build ./...` + `go vet` green), and both background type surfaces were already structurally identical with the same JSON tags; the codebase already uses this exact pattern (`internal/opencode/background.go` aliases/deliberately delegates to `sdd`, pi's capability probe delegates to `sdd`). Net effect: −53 lines in `adapter.go`, one rendering behavior, zero drift surface for the notice. The pi-local resolver duplicate was deliberately LEFT as-is (out of the failing scenario's surface; a resolver rewire would grow a remediation diff).
2. **Notice wording is templated from the actual state** (`policy: <p>, capability: <c>`), so one sentence covers disabled (off), unmanaged (capability absent) and both, and the literal `policy: off` token renders exactly in the scenario state.
3. **Report `type` for the default-off state changed `info` → `warning`** per the remediation contract; on+ready stays `info` (asserted by the matrix's positive control).

## Issues Found

- **Native edit-authority guard failed on infrastructure, not authority**: `biggz sdd-apply pi-subagent-runtime` exited 1 twice with `bigmem sdd-status hybrid collect: … context deadline exceeded` (BigMem primary+recovered DB merge warnings) — not a `blocked(edit_authority_missing)` consent verdict. Proceeded under the orchestrator's explicit allowed edit surfaces; recorded here for the ledger.
- **Full-repo suite has one unrelated failure**: `go test ./... -count=1 -timeout 300s` → `FAIL internal/review` with `panic: test timed out after 5m0s` (300.281s). `internal/review` has zero references to `internal/sdd`/`internal/agents/pi` (grep-confirmed), so it is independent of this remediation; all 60 other packages `ok`. Captured `…\full-go-suite.txt`; `sha256:E620BFC2EDA7067646864E5625E7BC9E6AE139F586BBE2EB834FFD682B2CB078`.
- No real-machine activation (`biggz install --agent pi` / pi restart) was executed — unchanged from the S4 deferral.

## Workload / PR Boundary

- Mode: change-level remediation (same `change-verify` work unit; not a new PR slice)
- Boundary: starts from the S3/S4 tree; ends at the owner notice + delegated twin + their tests. Rollback = revert the four Go files.
- Changed lines: `~113 additions / ~78 deletions` (`background.go +13/−1`, `adapter.go +24/−77`, `background_test.go +56`, `adapter_test.go +20`) — well under the 400-line budget.

## Status

Remediation complete; the blocked scenario's THEN now holds on both entry points with fresh probe + test evidence. Ready for the orchestrator to re-run the change-level verify.


