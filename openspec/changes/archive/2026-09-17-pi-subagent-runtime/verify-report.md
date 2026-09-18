```yaml
schema: biggz-ai.verify-result/v1
evidence_revision: sha256:ae7fb693dc17ef06a1f63c81f684a6216944d2362e589dafd9373328108bdf09
verdict: pass_with_warnings
blockers: 0
critical_findings: 0
requirements: 19/19
scenarios: 48/48
test_command: go test ./internal/install/... ./internal/agents/pi ./internal/sdd ./internal/doctor -count=1 && go test ./internal/assets/biggz -count=1 && node --test internal/assets/pi/biggz-subagent-runtime.test.mjs && node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs && node --test internal/assets/pi/biggz-subagent-width.test.mjs
test_exit_code: 0
test_output_hash: sha256:ae7fb693dc17ef06a1f63c81f684a6216944d2362e589dafd9373328108bdf09
build_command: go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: pi-subagent-runtime (re-verify after remediation; work tree at HEAD `8c2b2d19` + uncommitted change files)
**Version**: delta specs — 19 requirements / 48 scenarios across 6 domains (pi-subagent-runtime 7/16, runtime 2/9, pi-integration 2/0, orchestrator 2/2, pi-deploy-list 3/9, agent-install 3/12)
**Mode**: Standard (Strict TDD not active — `strict_tdd: false`, no `STRICT TDD MODE` signal; `strict-tdd-verify.md` not loaded)

### Re-run context (fresh from a clean change dir)

The prior change-level verify was BLOCKED on exactly one scenario: `runtime` → `Background Capability Probe and Disabled Reporting` → **Disabled reporting when policy off** (failed evidence `sha256:6c9c57cf…f605f806`). The human chose to IMPLEMENT the notice; remediation landed in `internal/sdd/background.go` (`backgroundSubagentsDisabled` predicate, `disabled/unmanaged` notice, `type: warning`), `internal/agents/pi/adapter.go` (twin converted to a thin delegate: type aliases + delegation) and three new tests. This run re-executes **everything fresh**; the apply/remediation reports were used for orientation only.

Previously-blocked scenario, re-proven with fresh execution (overlay probe, exit 0, `probe-sdd.txt` `sha256:72778cc7ba5bbba566c9b27e71fc661a065d61ef6357c7900651a80780c647e9`):

```text
VERIFY-PROBE-RENDER>>>
background subagents: off (decided by built-in default; capability: absent)
Background subagents are disabled/unmanaged (policy: off, capability: absent): background launches stay inert until the runtime is deployed (biggz install --agent pi) and the policy is turned on.
Resolution order (first hit wins): project file, global file, BIGGZ_BACKGROUND_SUBAGENTS, built-in default off.
<<<
VERIFY-PROBE-CHECKS>>> policy_off_literal=true capability_absent_literal=true disabled_unmanaged_phrase=true type=warning
VERIFY-PROBE-CONTROL>>> notice_present=false type=info
```

Twin entry point (`internal/agents/pi`), exit 0, `probe-pi.txt` `sha256:1150ac095b32bf6a9877257b56a46af38a0c0156be19b901b27fd1c58242f9ff`:

```text
VERIFY-PROBE-TWIN-CHECKS>>> policy_off_literal=true capability_absent_literal=true disabled_unmanaged_phrase=true type=warning
```

The probe files were added only through `go test -overlay` (temp paths); nothing was written into the repository (`git status --porcelain` shows no probe/zz paths).

### Completeness

| Metric | Value |
|--------|-------|
| Tasks total | 38 |
| Tasks complete | 38 |
| Tasks incomplete | 0 |
| Delivery re-cut note | 10 stacked PRs (S0, 2a1, 2a2, 3a, 3b, 4a, 4b, 5a, 5b, S4) — recorded in `tasks.md` lines 109 |

All 38 checkboxes are `[x]` in `openspec/changes/pi-subagent-runtime/tasks.md` (S0 4/4, S1a 6/6, S1b 7/7, S2 8/8, S3 10/10, S4 3/3). No pending task; full verification is unblocked.

### Build & Tests Execution

**Build**: ✅ Passed — `go build ./...` exit 0, empty output (`build_output_hash sha256:e3b0c442…b785`)

**Tests**: ✅ all suites exit 0 (canonical combined output `canonical-test-output.txt` = the five captured suite outputs concatenated in the order below, `sha256:ae7fb693dc17ef06a1f63c81f684a6216944d2362e589dafd9373328108bdf09`)

```text
go test ./internal/install/... ./internal/agents/pi ./internal/sdd ./internal/doctor -count=1   (go-test.txt, sha256:ed7bf9d0…3137a)
ok  	github.com/biggs-100/biggz-ai/internal/install	15.790s
ok  	github.com/biggs-100/biggz-ai/internal/install/steps	7.736s
ok  	github.com/biggs-100/biggz-ai/internal/agents/pi	1.822s
ok  	github.com/biggs-100/biggz-ai/internal/sdd	41.766s
ok  	github.com/biggs-100/biggz-ai/internal/doctor	2.930s

go test ./internal/assets/biggz -count=1                                                         (go-orchestrator-doc.txt, sha256:ef130554…5015d)
ok  	github.com/biggs-100/biggz-ai/internal/assets/biggz	0.632s

node --test internal/assets/pi/biggz-subagent-runtime.test.mjs                                   (node-runtime.txt, sha256:a8b8ef91…4491c)
ℹ tests 30 / pass 30 / fail 0   (11 suites)

node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs                              (node-factory.txt, sha256:3556f79f…20b1b)
ℹ tests 14 / pass 14 / fail 0   (1 suite)

node --test internal/assets/pi/biggz-subagent-width.test.mjs                                     (node-width.txt, sha256:97802fc6…c821b)
ℹ tests 11 / pass 11 / fail 0   (4 suites)
```

**Focused re-runs** (fresh, same session; `-run "TestZZVerifyProbe|TestRenderBackgroundSubagentsReport" -count=1 -v -overlay …`): exit 0, `internal/sdd` — `TestRenderBackgroundSubagentsReport_Disabled` PASS, `..._DisabledStateMatrix` 4/4 subtests PASS, `TestZZVerifyProbe_DisabledRendering` PASS; `internal/agents/pi` — `TestRenderBackgroundSubagentsReport_Malformed` PASS, `..._DisabledUnmanagedTwin` PASS, `TestZZVerifyProbe_DisabledUnmanagedTwin` PASS.

**Static gates**: `go vet ./internal/install/... ./internal/agents/pi ./internal/sdd ./internal/doctor` exit 0, empty output (`sha256:e3b0c442…b785`); `gofmt -l` over the 12 changed Go files exit 0, empty output.

**Modern Go check**: `use-modern-go` CLI `list` consulted fresh for `internal/sdd/background.go` (exit 0; list captured at `modern-go-list.txt` `sha256:4e4f3ac0db3c1325b0c1b0d45a1129226cf28952381758a795c576945a6ba553`). None of the returned guidelines (slices/maps/min-max/range-over-int/omitzero/…) applies to the added predicate, notice, or alias block — no WARNING.

**Coverage**: ➖ Not configured as a gate for this change (no threshold in `openspec/config.yaml`); not measured.

**Scope limitation (declared)**: full `go test ./...` was NOT re-run (Windows budget; CI owns the full matrix). The remediation's full local pass recorded one unrelated failure `internal/review` (`panic: test timed out after 5m0s`, 300.281s) with zero references to `internal/sdd`/`internal/agents/pi`; it was not re-observed here because the full matrix was out of scope.

### Spec Compliance Matrix

| # | Requirement | Scenario | Test | Result |
|---|-------------|----------|------|--------|
| 1 | pi-subagent-runtime — Tool Surface and Registration Gate | Contract tools registered | `biggz-subagent-runtime.test.mjs` > "registers the six tools + completion renderer when j0k3r is absent" (+ tool-surface list assertion) | ✅ COMPLIANT |
| 2 | pi-subagent-runtime | No dual registration | same > "refuses when getAllTools() lists subagent_run / subagent_list_*" + "refuses when settings packages carry pi-subagents-j0k3r" (both assert `pi.registered.length === 0`) | ✅ COMPLIANT |
| 3 | pi-subagent-runtime | Agents listed with bounded error | same > "bounds unknown-agent errors, refuses nested spawn…" + `subagent_agents` assertion in "runs one fake RPC child per task…" | ✅ COMPLIANT |
| 4 | pi-subagent-runtime — Foreground Task Mode and Child Protocol | Foreground run returns result | same > "completes on agent_settled; child env PI_SUBAGENT_CHILD=1 is set on spawn only" + "runs one fake RPC child per task…" | ✅ COMPLIANT |
| 5 | pi-subagent-runtime | Malformed JSONL tolerated | same > "skips malformed lines with a warning and keeps the stream alive" | ✅ COMPLIANT |
| 6 | pi-subagent-runtime — Child Interactivity Parity | Dialog relay round-trip | same > "relays a child extension_ui_request, returns the answer, and the run continues" | ✅ COMPLIANT |
| 7 | pi-subagent-runtime | Steering forwarded mid-run | same > "steers a running child mid-run" | ✅ COMPLIANT |
| 8 | pi-subagent-runtime — Stall Watchdog, Kill, and Cancel | Idle stall killed | same > "idle watchdog stalls + kills a silent child; the runtime survives it" | ✅ COMPLIANT |
| 9 | pi-subagent-runtime | Cancel kills the tree | same > "cancel kills the child + grandchild tree and marks the task cancelled" (+ abort→SIGTERM→SIGKILL / `taskkill /T /F` escalation test) | ✅ COMPLIANT |
| 10 | pi-subagent-runtime — Background Mode, Completion Delivery, Cap | Background ids then completion notice | same > "returns background ids immediately and delivers each completion without polling" | ✅ COMPLIANT |
| 11 | pi-subagent-runtime | Wait headline ≤2 lines | same > "renders the exact wait headline ≤2 lines and never a run-list dump" + "subagent_wait waits for the given runs…" | ✅ COMPLIANT |
| 12 | pi-subagent-runtime | Cap queues excess runs | same > "caps at 1, queues FIFO, auto-starts on a free slot, and cancels queued runs pre-spawn" + "resolves the cap from BIGGZ_BACKGROUND_SUBAGENTS" | ✅ COMPLIANT |
| 13 | pi-subagent-runtime — Safe Rendering by Construction | Emoji card does not overflow | same > "renders one bounded line across widths 20-200; the w=190>188 two-✅ shape stays ≤ width" | ✅ COMPLIANT |
| 14 | pi-subagent-runtime | No foreign width math | `biggz-subagent-width.test.mjs` > "S4 source scan: width measurement only via pi-tui imports" (+ runtime positive/negative clause) | ✅ COMPLIANT |
| 15 | pi-subagent-runtime — Width Regression Harness | Original crash shape passes | `biggz-subagent-width.test.mjs` > "reconstructs the crash shape deterministically…" + "renders the crash shape ≤ width for EVERY width 20–200" | ✅ COMPLIANT |
| 16 | pi-subagent-runtime | Property holds for all fixtures | same > "moduleMeasure(line) >= piTui.visibleWidth(line) for every fixture…" + "renders every fixture family ≤ width…" | ✅ COMPLIANT |
| 17 | runtime — Background Subagents 4-Source Policy Resolution | Project overrides global and env | `internal/agents/pi/adapter_test.go:39 TestResolveBackgroundSubagentsPolicy_ProjectOverrides` (project on over global off + env on → `project_file`/on/false); owner `internal/sdd/background.go:151-223` inspected (`.biggz` primary + legacy `.pi/gentle-ai` path, first-hit-wins) | ✅ COMPLIANT |
| 18 | runtime | Strict 2-key extra fails closed without fallback | `adapter_test.go:24 TestParseBackgroundSubagentsPolicyFile` (extra key rejected) + `:53 TestResolveBackgroundSubagentsPolicy_MalformedFailsClosed` (no fallback) + owner `parseBackgroundSubagentsPolicyFile` strict `len != 2` inspection | ✅ COMPLIANT |
| 19 | runtime | Malformed JSON fails closed | `adapter_test.go:53` (`{ malformed` → off/malformed/project_file, global on ignored) | ✅ COMPLIANT |
| 20 | runtime | Global beats env when project absent | `adapter_test.go:73 TestResolveBackgroundSubagentsPolicy_GlobalOverridesEnv` | ✅ COMPLIANT |
| 21 | runtime | Env fallback and default | `adapter_test.go:83 TestResolveBackgroundSubagentsPolicy_EnvFallbackAndDefault` (+ `lookupBackgroundEnv` BIGGZ > GENTLE_PI precedence) | ✅ COMPLIANT |
| 22 | runtime — Background Capability Probe and Disabled Reporting | Capability ready when runtime marker present | `internal/sdd/background_test.go:50 TestSubagentRuntimeCapability_MarkerReady` + `internal/agents/pi/subagents_fork_test.go:236 …DelegatesToMarkerOwner` | ✅ COMPLIANT |
| 23 | runtime | Capability absent without runtime marker | `background_test.go:63 TestSubagentRuntimeCapability_AbsentWithoutMarker` + `:82` (marker + j0k3r ⇒ not ready) + `:92` (corrupt settings fail closed) | ✅ COMPLIANT |
| 24 | runtime | Legacy package alone is not ready | `background_test.go:72 TestSubagentRuntimeCapability_J0k3rAloneNotReady` | ✅ COMPLIANT |
| 25 | runtime | **Disabled reporting when policy off** (previously blocked) | `background_test.go:139 TestRenderBackgroundSubagentsReport_Disabled` + `:158 …_DisabledStateMatrix` (4 subtests) + `adapter_test.go:109 …_DisabledUnmanagedTwin` + fresh overlay probe (`type=warning`, both entry points) | ✅ COMPLIANT |
| 26 | orchestrator — Delegation Tool Naming Contract | Delegation doc cites real tools | Fresh source scan of `internal/assets/biggz/biggz-orchestrator-delegation.md` (lines 37, 46, 145 cite `subagent`/`subagent_wait`; repo-wide grep: no `subagent_run` outside the runtime's guarded matcher/tests; no native `task` fallback) — static-only, see S4 | ✅ COMPLIANT |
| 27 | orchestrator | Retired affordances absent | Fresh source scan of the same file: no `FleetView`/`Fleet`/`formatAsyncRunList` occurrences (repo-wide grep confirms only pre-existing comments in `adapter.go`, `biggz-pi-pretty.js`) | ✅ COMPLIANT |
| 28 | pi-deploy-list — Guard Registered for Deploy | Deploy list contains the guard | `internal/install/steps/pi_extensions.go:81` entry + `pi_extensions_guard_test.go:31` mirror + `TestPiExtensionsGuard_FactoryExport` | ✅ COMPLIANT |
| 29 | pi-deploy-list | Deploy list keeps memory-chrome and excludes retired files | `pi_extensions.go:78` (memory-chrome kept), no `synthesis-gate`/`wait-pretty` entries; guard test count 13; `TestPiExtensionsStep_RemovesRetiredWaitPretty` | ✅ COMPLIANT |
| 30 | pi-deploy-list | Runtime pack entry present | `pi_extensions.go:84` (`pi/biggz-subagent-runtime.js`) mirrored in factory/guard lists; `…RemovesRetiredWaitPretty` asserts the deployed runtime file | ✅ COMPLIANT |
| 31 | pi-deploy-list | Retired wait shim excluded | Deploy list + factory mirror contain no `biggz-wait-pretty.js`; guard count stays 13 via swap; stale removal covered | ✅ COMPLIANT |
| 32 | pi-deploy-list | Build passes and deployed imports resolve | `go build ./...` exit 0 + `TestPiExtensionsStep_DropsUnportableExtensions` (deployed dir keeps the portable set) + `…RemovesRetiredWaitPretty` (runtime in, synthesis-gate/wait-pretty out; session-guard entry list-validated by the factory guard) | ✅ COMPLIANT |
| 33 | pi-deploy-list — Stale Wrapper Self-Heal on Upgrade | Stale copies removed on upgrade | `TestPiExtensionsStep_RemovesRetiredWaitPretty` (real `Apply`, stale wait-pretty + synthesis-gate removed, install succeeds) | ✅ COMPLIANT |
| 34 | pi-deploy-list | Missing stale files are no-op | same test, second `Apply` with no stale files → silent success | ✅ COMPLIANT |
| 35 | pi-deploy-list — Factory Test Mirror Updated | Count guard matches list | `biggz-pi-extensions-factory.test.mjs` > "deploy list covers all expected js extensions" (13) + `TestPiExtensionsGuard_FactoryExport` (13, helper drift check) | ✅ COMPLIANT |
| 36 | pi-deploy-list | Factory shape still valid | factory test per-asset `export default function <name>(pi)` assertions (13 JS entries) | ✅ COMPLIANT |
| 37 | agent-install — Pi MCP Adapter in InstallCommand | Includes adapter in order | `adapter_test.go:134 TestInstallCommand_ContainsAdapterBeforeSubagents` (idx adapter < idx j0k3r, `@^2`, dedup 1) | ✅ COMPLIANT |
| 38 | agent-install | Pin is exact | `subagents_fork_test.go:15 TestInstallCommand_UsesJ0k3rFork` (exact `@1.6.1`, fails on unpinned) + `:154 TestSettingsReconcile_PinsJ0k3rFork` + `:124 TestFilterPiPackages_DropsPredecessor` | ✅ COMPLIANT |
| 39 | agent-install | Idempotent second run | `subagents_fork_test.go:175-183` (second `ProvisionBigMemMCP`, stable count) + `adapter_test.go:167-170` (second `InstallCommand`, same length) | ✅ COMPLIANT |
| 40 | agent-install | Offline harmless | ⚠️ MCP-JSON half: `adapter_test.go:316 TestProvisionBigMemMCP_MergePreservesOthersAtomically` + `:415 …_Idempotent`; the "error MUST hint doc fallback" half is documented only (`adapter.go:140`) — no npm-unreachable test. See W3 | ⚠️ PARTIAL |
| 41 | agent-install — REQ-INST-001 Pi Web Search Extension Deployment | Atomic deploy creates extension | `internal/install/pi_web_search_test.go:15 TestDeployPiWebSearch` (embedded bytes equal, atomic writer); `Result.PiWebSearch` conjunct is a dead field (`install.go:55`, never assigned — pre-existing). See W2 | ⚠️ PARTIAL |
| 42 | agent-install | Idempotent second deploy | `pi_web_search_test.go:41 TestDeployPiWebSearch_Idempotent` (`!res.Created && !res.Changed`, bytes unchanged) | ✅ COMPLIANT |
| 43 | agent-install | Deploy via Run() | `TestPiExtensionsStep_DropsUnportableExtensions` (asserts `biggz-web-search.js` in the deployed extensions dir) + `TestPiExtensionsStep_DeploysSubAgents` (+ embedded-FS variant) + zero remaining call sites for the removed helpers (repo grep) | ✅ COMPLIANT |
| 44 | agent-install | TempDir isolation for tests | `pi_web_search_test.go:61 TestDeployPiWebSearch_TempDir` (writes under TempDir, no file outside) | ✅ COMPLIANT |
| 45 | agent-install | Self-heal removes legacy if present | `pi_web_search_test.go:80 …_LegacyCleanup` (removes the deprecated web-search variant); `pi_extensions.go:245` also removes legacy `biggz-pi-pretty.js` copies | ✅ COMPLIANT |
| 46 | agent-install — j0k3r Retirement and Cutover Reconcile | Cutover removes j0k3r | `subagents_fork_test.go:43 TestInstallCommand_CutoverDropsJ0k3r` + `:191 TestSettingsReconcile_CutoverDropsJ0k3r` (settings purge, pretty preserved) | ✅ COMPLIANT |
| 47 | agent-install | No dual registration after cutover | JS gate tests (j0k3r present ⇒ zero registrations; settings marker ⇒ refuse) + Go cutover purge; composition proven at both layers | ✅ COMPLIANT |
| 48 | agent-install | Rollback restores pinned j0k3r | `subagents_fork_test.go:223-230` (marker removed + re-run ⇒ exact `@1.6.1` restored) | ✅ COMPLIANT |

**Compliance summary**: 46/48 scenarios fully compliant via fresh passing tests; 2/48 PARTIAL (rows 40, 41 — pre-existing, WARNING-level, orthogonal to this change's diffs); 0 UNTESTED; 0 FAILING. Requirements: 19/19 implemented.

### Correctness (Static Evidence)

| Requirement | Status | Notes |
|------------|--------|-------|
| pi-subagent-runtime — Tool Surface and Registration Gate | ✅ Implemented | `biggz-subagent-runtime.js:490-561` one `typeof`-guarded chain (`getAllTools()` → `getToolDefinition("subagent_run")` → settings `packages` → fail-closed), never `pi.getTool`; six tools registered (`:718+`, factory `:927+`) |
| pi-subagent-runtime — Foreground Task Mode and Child Protocol | ✅ Implemented | `createTask` one RPC child per task; `buildChildArgs` (`--mode rpc --no-session --tools/--model`), `buildChildEnv` sets `PI_SUBAGENT_CHILD=1` without mutating the base env; `createJsonlReader` LF-only, CR strip, 1 MiB cap (`JSONL_MAX_LINE_BYTES :32`), malformed logged+skipped; no `readline` (asserted) |
| pi-subagent-runtime — Child Interactivity Parity | ✅ Implemented | `presentUiRequest` select/confirm/input normalization, `extension_ui_response` returns to child stdin; `steer()` writes the `steer` command mid-run |
| pi-subagent-runtime — Stall Watchdog, Kill, Cancel | ✅ Implemented | idle 4 min / total 30 min (`:33-34`), `killTree` escalation abort → SIGTERM group → SIGKILL, Windows `taskkill /PID /T [/F]` (`:259-267`) |
| pi-subagent-runtime — Background Mode, Completion Delivery, Cap | ✅ Implemented | background ids immediate + `deliverCompletion`; `renderWaitHeadline` (`:662`) ≤2 lines; `resolveBackgroundCap` (`:600`) default 2, numeric override clamp ≥1, `on`/`off` → 2; `queued` FIFO `pump()` |
| pi-subagent-runtime — Safe Rendering | ✅ Implemented | runtime imports `truncateToWidth` from `@earendil-works/pi-tui` (`:30`); `widgetRowsFor` (`:608`) max 2 + `… +N`, hidden idle/child/pretty-off, `setWidget("biggz-subagents", …, {placement:"belowEditor"})` (`:631`) |
| pi-subagent-runtime — Width Regression Harness | ✅ Implemented | `biggz-subagent-width.test.mjs` (181-width sweep 20–200, fixture families, ≥-property, source scan ratchet) + `test/pi-tui-resolver.mjs`/`oracle.mjs` |
| runtime — Policy Resolution | ✅ Implemented | Owner `internal/sdd/background.go:151-223` (project `.biggz` → global → `BIGGZ_BACKGROUND_SUBAGENTS` → default off; strict 2-key; ≤2 reads; fail closed). Pi twin resolver kept as a duplicate (W5) |
| runtime — Capability Probe and Disabled Reporting | ✅ Implemented (remediation) | `SubagentRuntimeCapability` (`:336-341`) = marker ∧ j0k3r-absent; `backgroundSubagentsDisabled` (`:234-236`) + notice (`:243`) + `type warning` (`:271`); pi twin delegates (`adapter.go:893-899`) — the previously-blocked THEN now holds on both entry points |
| pi-integration — POLISH-PI-01 / PI-02 (REMOVED) | ✅ Removal prepared | Delta carries `(Reason: …)` + `(Migration: …)` for both; the shim asset `biggz-wait-pretty.js` is deleted and stale deployed copies are self-healed (`pi_extensions.go:258`); headline re-homed to the runtime's `subagent_wait` |
| orchestrator — Delegation Tool Naming Contract (ADDED) | ✅ Implemented | Doc cites the registered names and the documented unavailable-fallback text; retired names/affordances absent (rows 26-27) |
| orchestrator — POLISH-ORCH-02 (REMOVED) | ✅ Removal prepared | Delta carries Reason + Migration (headline owned by `subagent_wait`) |
| pi-deploy-list — Guard Registered for Deploy | ✅ Implemented | 13 JS entries exactly (runtime in, wait shim out, memory-chrome kept, session-guard present) across `pi_extensions.go` + factory mirror + guard test |
| pi-deploy-list — Stale Self-Heal | ✅ Implemented | `pi_extensions.go:258` removal set `{biggz-synthesis-gate.js, biggz-wait-pretty.js}`, missing files silent no-op, memory-chrome explicitly NOT stale |
| pi-deploy-list — Factory Test Mirror | ✅ Implemented | Count 13 + per-entry factory shape |
| agent-install — Pi MCP Adapter in InstallCommand | ✅ Implemented | `adapter.go:158-165` order (`pi-mcp-adapter@^2` before `piSubagentsJ0k3rPinned`), exact pin const `:32`, `desiredPiPackages` `:360-362` + `dropSupersededPiPins` replaces floats; cutover delete `:169-173`; offline tolerance documented (`:140`) — hint clause untested (W3) |
| agent-install — REQ-INST-001 Web Search Deployment | ⚠️ Partial | `DeployPiWebSearch` atomic + idempotent + TempDir + legacy cleanup proven; the `Result.PiWebSearch` field is dead (W2); the live `Run()` path deploys web-search via the `PiExtensionsStep` deploy list (row 43) |
| agent-install — j0k3r Retirement and Cutover Reconcile | ✅ Implemented | Marker ⇒ settings/`InstallCommand` drop j0k3r, pretty preserved, idempotent; marker absent ⇒ exact pin (rollback proven); dead helpers (`DeployPiSubAgents`/`DeployPiWaitPretty`/`DeployPiPrettyWrapper`/`DeployPiSubagentConfig`) deleted with zero call sites (one comment reference documents the deletion) |

### Coherence (Design)

| Decision | Followed? | Notes |
|----------|-----------|-------|
| D1 RPC subset (commands `prompt`/`steer`/`abort`/`get_last_assistant_text`/`extension_ui_response`; listed events; rest ignored) | ✅ Yes | Runtime `createTask` wires exactly this subset; dialog/steer tests exercise it |
| D2 Widget (`setWidget("biggz-subagents", rows, {placement:"belowEditor"})`, row format, max 2 + `… +N`, hidden states) | ✅ Yes | `:608-660`, tests assert exact rows/options object/hide matrix |
| D3 History in-memory only (bounded ring, no disk, no BigMem) | ✅ Yes | `createTaskRegistry` + test asserts no `writeFileSync`/`appendFileSync`/`biggz_mem` in the source |
| D4 Exact pin + retirement criterion + upstream #25 | ✅ Yes | Exact pin in install/desired/reconcile; cutover drop; #25 comment published (S0 evidence), real-machine pin observed (apply evidence) |
| D5 Marker ownership in `internal/sdd` consumed by install/doctor/probe | ✅ Yes | `SubagentRuntimeTargetName`/`MarkerPath`/`Capability`; adapter probe + doctor + install all delegate |
| D6 Reconciliation (memory-chrome kept; dead funcs deleted; JS count 13 via swap) | ✅ Yes | Deploy list/guard/factory all 13 JS; four helpers + `mergeJSONCWrapper` removed with zero call sites |
| D7 Schemas (`subagent` params, `context` fresh only, `mode` task/background, bounded errors) | ✅ Yes | `createSubagentToolset` tests assert bounded errors for `fork`, unknown mode, unknown agent, unknown/expired task id |
| Registration gate chain (never `pi.getTool`; fail-closed) | ✅ Yes | `:525-561`; load-time `getAllTools()` throw tolerated (documented 0.85.1 behavior), unprovable ⇒ refuse |
| Concurrency cap (default 2, numeric override clamp ≥1, `on`/`off` → 2, FIFO queue) | ✅ Yes | `resolveBackgroundCap` + cap/FIFO test matrix |
| Completion card one bounded line via `renderCompletion` | ✅ Yes | `:892-925`; crash-shape and width-sweep tests |
| Width test oracle (resolver → real pi-tui, else oracle + fidelity) | ✅ Yes | `test/pi-tui-resolver.mjs`/`oracle.mjs`; fidelity test skips only when the real package is absent (this run: oracle mode, ≥-property still asserted) |
| Threat matrix rows (spawn auth, tree kill, child env, JSONL bounds, rendering width, dual registration) | ✅ All covered | Each row maps to a named passing test (rows 2/4/5/8/9/13) |
| Remediation deviation 1 (twin as thin delegate instead of mirrored) | ✅ Accepted | Compile-checked; report/type duplication retired; resolver duplicate deliberately left (W5); declarative equality asserted by the twin test |
| Remediation deviation 2-3 (templated notice wording; disabled state `type` info→warning) | ✅ Accepted | Required by the remediation contract; positive control `on`+`ready` stays `info`, no notice (fresh probe) |

### Ledger Binding

Orchestrated run (remediation attempt live under the orchestrator): the verifier did **not** acquire/settle any `biggz sdd-attempt` entry — the orchestrator settles after validating this report. Evidence binding: `evidence_revision sha256:ae7fb693dc17ef06a1f63c81f684a6216944d2362e589dafd9373328108bdf09` = `test_output_hash` = SHA-256 of the canonical combined suite output at `C:\Users\USER\AppData\Local\Temp\opencode\verify-pi-subagent-runtime\canonical-test-output.txt` (5 suite captures concatenated with BEGIN/END markers, in the `test_command` order). `build_output_hash` = SHA-256 of the empty `go build ./...` output.

### Issues Found

**CRITICAL**: None

**WARNING**:
1. **`{{BIGGZ_BACKGROUND_POLICY}}` dead substitution** — the raw token remains at `internal/assets/biggz/biggz-orchestrator-delegation.md:143`, while `internal/install/install.go:808-810` only substitutes the token inside `biggz/biggz-orchestrator.md`, which no longer contains it. The lazy-loaded delegation doc therefore reaches the model with an unexpanded placeholder instead of the live policy line. Pre-existing (the line is untouched by this change's diff) and not spec-violating (the `orchestrator` delta governs tool naming, which holds).
2. **`install.Result.PiWebSearch` is a dead field** — declared at `internal/install/install.go:55`, never assigned or read in production (`git grep PiWebSearch` → declaration + tests + doctor only); the last `DeployPiWebSearch(ctx, homeDir)` call in `Run()` was removed in `b70c904a` (not by this change — this change's install.go diff touches only the four dead helpers). The `agent-install` sentence "MUST integrate with `Run()` and `Result.PiWebSearch`" is stale relative to the live architecture: `Run()` deploys web-search through the `PiExtensionsStep` deploy list (test-proven). Row 41 marked PARTIAL.
3. **"Offline harmless" hint clause unexercised** — the MCP-JSON-harmless half is proven by tests; the "error MUST hint doc fallback" half is documented only (`internal/agents/pi/adapter.go:140`); no test simulates npm unreachable. Pre-existing (scenario carried unchanged; the pin modification is orthogonal to npm-failure handling). Row 40 marked PARTIAL.
4. **Real-pi activation smoke deferred (declared limitation, not falsified)** — no real `biggz install --agent pi` + pi restart + emoji-heavy delegated phase was executed; evidence is the temp-HOME reconcile harness, the fake-RPC-child suite, and the width harness.
5. **Pi duplicate resolver carried (previous warning partially retired)** — the thin-delegate conversion retired the report/type/rendering duplication (`adapter.go:729-748` aliases + `:890-899` delegation; anti-drift equality asserted by `TestRenderBackgroundSubagentsReport_DisabledUnmanagedTwin`), but `adapter.go:750-888` still duplicates `resolveBackgroundSubagentsPolicy`/`lookupBackgroundEnv`/`gentleAiConfigHome`/`parseBackgroundSubagentsPolicyFile` instead of delegating to `internal/sdd` (documented remediation deviation 1; functional parity covered by the same resolver tests).
6. **Doc-vs-validator contradiction on `fail` persistence** — `internal/assets/skills/sdd-verify/references/report-format.md:85` states a canonical `fail` "is valid and persistable", but `internal/sdd/verify.go:527-541` (`admissionCheckVerdict`, mirrored at `:400-411`) denies admission on `verdict: fail`. Under the candidate-first gate a `fail` can never be persisted as documented (fail-safe zero writes; the process doc or the validator needs reconciliation).

**SUGGESTION**:
1. `internal/assets/pi/biggz-pi-pretty.js` orphan — no deployer remains (out of the deploy list; deployed copies self-healed at `pi_extensions.go:245`), and the width allowlist pins its existence (`biggz-subagent-width.test.mjs:174`), so deletion needs a coordinated allowlist edit; recorded deletion follow-up.
2. `ResolvePackageBin` (`internal/agents/pi/model_routing.go:57`) has no production caller (test-only usage) — pre-existing dead helper outside this change's diff.
3. Width allowlist ratchet (`biggz-subagent-width.test.mjs:169-175`) asserts every exempt file still exists — keep a short "delete asset + drop allowlist entry together" checklist to avoid the stale-entry trap (see S1).
4. The `orchestrator` doc scenarios (rows 26-27) are verified by direct fresh source scan only; adding the two assertions (cite `subagent`/`subagent_wait`; no `subagent_run`/`FleetView`) to `internal/assets/biggz/orchestrator_test.go` would turn future drift into a failing test instead of a verify-time grep.
5. `internal/assets/pi/biggz-question-mouse.js:26` still references the deleted `internal/assets/pi/subagent-config.json` — stale comment.

### Verdict

**PASS WITH WARNINGS** — 38/38 tasks complete; all 48 scenarios evaluated fresh with 46/48 fully compliant and 2/48 PARTIAL pre-existing WARNING-level gaps (offline-hint clause; dead `Result.PiWebSearch` field) that do not falsify any observable behavior; the previously-blocked `runtime` "Disabled reporting when policy off" scenario is now PROVEN on both entry points (`policy: off`, `capability: absent`, `disabled/unmanaged`, `type: warning`, positive control stays `info`); build/vet/gofmt clean; Go suites 5/5 packages and node suites 30/30 + 14/14 + 11/11 pass fresh; warnings recorded for the dead placeholder, the stale web-search field, the offline-hint gap, the deferred real-pi activation, the carried resolver duplicate, and the doc-vs-validator `fail` contradiction.
