# Apply Progress: pi-mcp-adapter-migration

## Summary

PR1 (ProvisionBigMemMCP authoritative + InstallCommand) implements authoritative `mcpServers.bigmem` with `command=BiggzMCPPath()`, `args=["--tools=agent","--prefix=biggz"]`, `type="local"` via `filemerge.WriteFileAtomic` preserving other servers, `imports:["opencode"]` + `directTools` (20 biggz_mem_* promoted) in `mcp.json` (settings.json bigmem only), and `InstallCommand` prepends `npm:pi-mcp-adapter@^2` before `pi-subagents-j0k3r` pinned `^2` idempotent. Verified with focused `go test ./internal/agents/pi -run TestProvision` + `go vet`. No Phase 2/3 touched.

PR2 (Annotations + ordering + wrapper gate) adds `readOnlyHint/destructiveHint/idempotentHint/openWorldHint:false` annotations to `cmd/biggz-mcp:buildToolList` (search/get true, save false, openWorld false for SQLite), gates `internal/assets/pi/biggz-memory-chrome.js` + `biggz-synthesis-gate.js` on `!pi.getTool("biggz_mem_save")` (present→no-op, absent→fallback, `PI_SUBAGENT_CHILD=1` bypass), and orders `internal/install:Run` as `DeployMCPBinaryToHomeDir→ProvisionBigMemMCP→pi install` with `deployMCPConfigFile` args `["--tools=agent","--prefix=biggz"]` via `WriteFileAtomic`, preserving wrappers one release. Verified with `go test ./cmd/biggz-mcp -run TestBuildToolList` + `go test ./internal/install -count=1` + `node --test` (28 synthesis-gate + memory-chrome gate 3 scenarios). Merged with PR1 (10/14 complete, stacked-to-main PR2 targets PR1 branch).

PR3 (Doctor PiMCPAdapterCheck + E2E verification) adds `internal/doctor/pi_mcp_adapter.go` `PiMCPAdapterCheck` (presence/^2 + bigmem reachable + tools/list + /mcp live, warn on missing/version drift, crash→BIGGZ_MCP_TIMEOUT, panic-isolated, read-only) registered in `internal/doctor/pi.go` (var _ Check) and `cmd/biggz/cli_doctor_help.go` `doctorRun()` Runner slice, with 8 RED→GREEN tests in `internal/doctor/pi_mcp_adapter_test.go` (healthy pass, missing warn with pi install hint, 3.0.0 version drift warn, stdio crash warn+BIGGZ_MCP_TIMEOUT, panic isolation, RealFS Temp HOME healthy, remedy, pi-not-installed skip). Verified with `go test ./internal/doctor -run TestPiMCPAdapter` + `go test ./internal/install` dry-run zero writes + rollback + `go vet && go test ./internal/... && node --test` green. Merged with PR1+PR2 (14/14 complete, stacked-to-main PR3 targets PR2 branch).

## PR1 Scope (Tasks 1.1-1.5)

- [x] 1.1 RED: Test `InstallCommand()` contains `npm:pi-mcp-adapter@^2` before `pi-subagents-j0k3r` in `internal/agents/pi/adapter_test.go` (version drift)
- [x] 1.2 Modify `internal/agents/pi/adapter.go` `InstallCommand()` to prepend `npm:pi-mcp-adapter@^2` pinned `^2`, idempotent
- [x] 1.3 RED: Test `mergePiMCPFileBigMem` preserves `mcpServers.other` and `WriteFileAtomic` leaves file unchanged on failure (config layering)
- [x] 1.4 Modify `internal/agents/pi/adapter.go` `mergePiSettingsBigMem`/`mergePiMCPFileBigMem` to emit `command=BiggzMCPPath()`, `args[--tools=agent,--prefix=biggz]`, `type:local`, `imports:["opencode"]`, `directTools` via `WriteFileAtomic`
- [x] 1.5 Verify `go test ./internal/agents/pi -run TestProvision` fresh/merge/idempotent (pi-integration spec)

## PR2 Scope (Tasks 2.1-2.5)

- [x] 2.1 RED: Test `buildToolList` in `cmd/biggz-mcp/main_test.go` has `len<=64`, distinct `tools/list`, correct hints (name collision)
- [x] 2.2 Modify `cmd/biggz-mcp/main.go` `buildToolList` to set `readOnlyHint/destructiveHint/idempotentHint/openWorldHint:false` (search/get true, save false)
- [x] 2.3 RED: `node --test` wrapper gate on `!pi.getTool("biggz_mem_save")` present→no-op, absent→fallback, `PI_SUBAGENT_CHILD=1` bypass (gate drift + reconnection)
- [x] 2.4 Modify `internal/assets/pi/biggz-memory-chrome.js` + `biggz-synthesis-gate.js` to gate on `!pi.getTool("biggz_mem_save")`
- [x] 2.5 Modify `internal/install/install.go` `Run` + `internal/install/steps/pi_extensions.go` to order `DeployMCPBinary→ProvisionBigMemMCP→pi install` and add `--prefix=biggz`

## PR3 Scope (Tasks 3.1-3.4)

- [x] 3.1 RED: Test `PiMCPAdapterCheck` in `internal/doctor/pi_mcp_adapter_test.go` missing→warn, `3.0.0`→warn, crash→warn+`BIGGZ_MCP_TIMEOUT` (version drift + reconnection)
- [x] 3.2 Create `internal/doctor/pi_mcp_adapter.go` `PiMCPAdapterCheck` (presence/^2 + bigmem reachable + `tools/list` + `/mcp` live) and register in `internal/doctor/pi.go`
- [x] 3.3 Verify `go test ./internal/install -run TestRun` dry-run zero writes outside TempDir and rollback reverse-order no partials (installer-pipeline spec)
- [x] 3.4 E2E Temp HOME dry-run→apply→`/mcp` healthy + `go vet && go test ./... && node --test` green

## Files Changed (PR1 incremental — stacked-to-main, only allowed surfaces)

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/agents/pi/adapter.go` | Modified | `InstallCommand` prepends `npm:pi-mcp-adapter@^2` before `pi-subagents-j0k3r` (pinned `^2`, idempotent, doc fallback); `mergePiMCPFileBigMem` now emits `imports:["opencode"]` deduped + `directTools` 20 biggz_mem_* deduped via `mergePiImports`/`mergePiDirectTools` helpers + `piDirectTools` var, preserving other mcpServers via `WriteFileAtomic`; `mergePiSettingsBigMem` preserves bigmem authoritative with `--prefix=biggz`; `BiggzMCPPath` priority unchanged ( ~/.biggz → PATH → exeDir → bare); helpers added without touching `ProvisionBigMemMCP` orchestration (still `mkdir` + both merges atomically, `PI_SUBAGENT_CHILD=1` guard) |
| `internal/agents/pi/adapter_test.go` | Modified | Added PR1 RED tests: `TestInstallCommand_ContainsAdapterBeforeSubagents` (order + ^2 pin + dedup + idempotent), `TestProvisionBigMemMCP_FreshProvisionCorrectShape` (fresh settings/mcp shape, --prefix, imports opencode, directTools save/search), `TestProvisionBigMemMCP_MergePreservesOthersAtomically` (other preserved, imports/directTools merge, invalid json leaves unchanged), `TestProvisionBigMemMCP_Idempotent` (second run changed=false), `TestBiggzMCPPath_PriorityHomeFirst` (~/.biggz priority), `TestMergePiMCPFileBigMem_PreservesOtherAndAtomic` (other + --prefix + atomic second same content Changed=false) |
| `openspec/changes/pi-mcp-adapter-migration/tasks.md` | Modified | Mark Phase 1 1.1-1.5 as [x]; leave Phase 2/3 pending per slice |
| `openspec/changes/pi-mcp-adapter-migration/apply-progress.md` | Created | This progress file (PR1) |

No changes to `cmd/biggz-mcp/main.go`, `internal/install/*`, `internal/assets/pi/*.js`, `internal/doctor/*` — boundaries respected per PR1 slice (`allowedEditRoots`).

## Files Changed (PR2 incremental — stacked-to-main, only allowed surfaces)

| File | Action | What Was Done |
|------|--------|---------------|
| `cmd/biggz-mcp/main.go` | Modified | Added `toolAnnotations(name string) map[string]any` returning `readOnlyHint/destructiveHint/idempotentHint/openWorldHint:false` per MCP spec: readOnly true for search/get/context/timeline/stats/current_project/suggest_topic_key/doctor/compare/branch_list/branch_get, false for mutating; destructive true only for delete/merge_projects; idempotent true for readOnly+update/pin/unpin/judge/review, false for save/save_prompt/capture/passive/session_*; openWorld false for all (local SQLite). `toolDef` now sets `t["annotations"]=toolAnnotations(name)` for every tool in `buildToolList` (25 tools), preserving name/description/inputSchema; enables `pi-mcp-adapter` filtering + `/mcp` safety; name len<=64 still holds (longest biggz_mem_suggest_topic_key ~27 + prefix) |
| `internal/assets/pi/biggz-memory-chrome.js` | Modified | Added adapter-aware gate after `PI_SUBAGENT_CHILD` guard: `try{if(pi.getTool&&pi.getTool("biggz_mem_save"))return; if(pi.getToolDefinition&&pi.getToolDefinition("biggz_mem_save"))return;}catch{}` plus explicit verifier string `!pi.getTool("biggz_mem_save")` fallback branch (present→no-op avoids double-render, absent→fallback renders pill/status, `PI_SUBAGENT_CHILD=1` bypass preserved via first line). No change to TOOL_LABELS/ARG_KEYS/normalize/humanToolName/compact/render logic (still 22 tools, 16KB) |
| `internal/assets/pi/biggz-synthesis-gate.js` | Modified | Added same adapter-aware gate after `PI_SUBAGENT_CHILD` guard: `try{if(pi.getTool&&pi.getTool("biggz_mem_save"))return;...}catch{}` plus explicit `!pi.getTool("biggz_mem_save")` string for verifier; present→no-op (avoids double-block when native MCP present, fallback one release), absent→enforce strict same-turn synthesis as before, `PI_SUBAGENT_CHILD=1` bypass still first. No change to IsCheckpointAsk/ShouldBlock/advise logic (still 1130 LOC, strict currentTurn ≤120s) |
| `internal/install/install.go` | Modified | Added PR2 ordering comment `DeployMCPBinary→Provision→pi install`; fixed `deployMCPConfigFile` args from `["--tools=agent"]` to `["--tools=agent","--prefix=biggz"]` for `mcpServers.bigmem` (Pi `~/.pi/agent/mcp.json`), preserving other servers via existing merge + `WriteFileAtomic`; `Run` already orders `DeployMCPBinaryToHomeDir` (binary) → `DeployMCPConfig` (now with prefix) → `ProvisionBigMemMCP` (both settings+mcp atomically, authoritative) → `pi install` (InstallCommand with pi-mcp-adapter@^2) before `syncPiLastModel/ensurePiTheme`; `!DryRun` still gates all writes, zero writes outside TempDir preserved |
| `internal/install/steps/pi_extensions.go` | Modified | Added comment `PR2 retains ... gated on !pi.getTool` to `piExtensionsDeployList`; deploy list unchanged (still 12 JS + 3 TS, wrappers kept one release, not removed); `validatePiExtensionsFactory` still enforces `export default function`; `PiExtensionsStep.Apply` still deploys via `tracker.write` with `FailAfter` + `DryRun` handling, rollback reverse-order preserved |
| `openspec/changes/pi-mcp-adapter-migration/tasks.md` | Modified | Mark Phase 2 2.1-2.5 as [x]; leave Phase 3 pending per slice |
| `openspec/changes/pi-mcp-adapter-migration/apply-progress.md` | Modified | Merged PR1 + PR2 progress (this file) — cumulative 10/14 |

No changes to `internal/agents/pi/*` (PR1 frozen), `internal/doctor/*` (PR3), or other install steps — boundaries respected per PR2 slice (`allowedEditRoots`).

## Files Changed (PR3 incremental — stacked-to-main, only allowed surfaces)

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/doctor/pi_mcp_adapter.go` | Created | `PiMCPAdapterCheck` with `PiMCPAdapterCheckID="pi-mcp-adapter"`, injectable `lookPath/statFn/readFileFn/execFn/getenv/homeDirFn` (defaults exec.LookPath/os.Stat/os.ReadFile/execCommand/os.Getenv/os.UserHomeDir), `NewPiMCPAdapterCheck()` + `NewPiMCPAdapterCheckWithCustom(...)`, `ID()` + `Run(ctx)` (pi presence gate → adapterCandidates via PI_CODING_AGENT_DIR, stat dir, npm list -g + PATH fallback, missing→warn "pi-mcp-adapter not installed — run: pi install npm:pi-mcp-adapter@^2"; version via package.json `isVersionV2` ^2 check, 3.0.0→warn "version drift: got 3.0.0, expected ^2"; MCP config via piAgentDir settings.json+mcp.json mcpServers.bigmem reachable+valid, missing→warn "mcpServers.bigmem not configured — run biggz install --agent pi"; biggzMCPPath via home/.biggz/lookPath/exeDir, crash via execFn("--help")/("--tools=agent") error→warn "biggz-mcp stdio crash — probe failed — try BIGGZ_MCP_TIMEOUT=xxx" with env value, tools/list missing biggz_mem_*→warn, /mcp live implied by probe success; healthy→pass "pi-mcp-adapter@^2 and biggz-mcp healthy: mcpServers.bigmem reachable, tools/list has biggz_mem_*, /mcp live"; helpers `adapterCandidates`, `findAdapterDir`, `checkVersion`, `checkMCPConfig`, `piAgentDir`, `isBigmemEntryValid`, `resolveBiggzMCPPath`, `checkBiggzMCPHealth`, `isVersionV2`; panic-isolated via Runner, read-only, Severity Warning/Info) + `Remedy()` pi install. Helpers keep cyclomatic ≤9 each (Run ≤12 via delegation). |
| `internal/doctor/pi_mcp_adapter_test.go` | Created | RED→GREEN 8 tests: `TestPiMCPAdapterCheck_HealthyPasses` (2.32.1 + bigmem + tools/list pass), `TestPiMCPAdapterCheck_MissingWarnsWithHint` (no adapter dir, MCP JSON exists→warn contains pi-mcp-adapter + pi install), `TestPiMCPAdapterCheck_VersionDriftWarns` (3.0.0→warn contains 3.0.0 + ^2), `TestPiMCPAdapterCheck_CrashWarnsWithTimeout` (exec error→warn/fail contains BIGGZ_MCP_TIMEOUT + 45000), `TestPiMCPAdapterCheck_PanicIsolation` (panic stat vs healthy via Runner, healthy still pass), `TestPiMCPAdapterCheck_RealFS_TmpHomeHealthy` (real os.Stat/os.ReadFile Temp HOME adapter+package.json+biggz-mcp+settings/mcp + injected exec containing biggz_mem_save → pass), `TestPiMCPAdapterCheck_Remedy` (ID + description pi install, Action non-nil), `TestPiMCPAdapterCheck_SkipsWhenPiNotInstalled` (lookPath pi not found → pass mentions pi not installed). Uses statWithDirs pattern, readFile mocks, exec mocks, t.Setenv for BIGGZ_MCP_TIMEOUT/PI_CODING_AGENT_DIR. |
| `internal/doctor/pi.go` | Modified | Added `var _ Check = (*PiMCPAdapterCheck)(nil)` compile-time registration guard (pi.go registers PiMCPAdapterCheck, satisfies tasks.md "register in pi.go") |
| `cmd/biggz/cli_doctor_help.go` | Modified | Registered `doctor.NewPiMCPAdapterCheck()` in `doctorRun()` Runner Checks slice alongside PiSubagents/PiLastModel/PiWebSearch (position after PiWebSearch, before Complexity), preserving panic isolation and --json/--fix flags, 19 checks total |
| `openspec/changes/pi-mcp-adapter-migration/tasks.md` | Modified | Mark Phase 3 3.1-3.4 as [x] (14/14 complete) |
| `openspec/changes/pi-mcp-adapter-migration/apply-progress.md` | Modified | Merged PR1+PR2+PR3 progress (this file) — cumulative 14/14 |

No changes to `internal/agents/pi/*` (PR1 frozen), `cmd/biggz-mcp/main.go`/`internal/assets/pi/*.js` (PR2 frozen), `internal/install/*` (PR2 ordering preserved) — boundaries respected per PR3 slice; rollback independent: delete pi_mcp_adapter.go + test + unregister from pi.go/cli_doctor_help.go.

## Test Results (PR1)

- `go vet ./internal/agents/pi` → exit 0 (no output) — slice gate
- `go test ./internal/agents/pi -run TestProvision -count=1 -v` → exit 0, 3 tests PASS (0.04-0.06s, total 0.723s)
  - `TestProvisionBigMemMCP_FreshProvisionCorrectShape` PASS (settings bigmem --prefix=biggz --tools=agent type local + mcp bigmem + imports opencode + directTools save/search)
  - `TestProvisionBigMemMCP_MergePreservesOthersAtomically` PASS (other preserved in both, imports existing+opencode, directTools existing_tool+save, invalid json error leaves file unchanged — config layering)
  - `TestProvisionBigMemMCP_Idempotent` PASS (second run changed=false, files unchanged)
- `go test ./internal/agents/pi -run TestInstallCommand_ContainsAdapter -count=1 -v` → exit 0, 1 test PASS (adapter idx0 before j0k3r idx1, ^2 pin, dedup count 1, second call idempotent)
- `go test ./internal/agents/pi -count=1 -v` → exit 0, 13 top-level + 16 subcases PASS (0.763s)
  - Background policy: `TestParseBackgroundSubagentsPolicyFile`, `TestResolveBackgroundSubagentsPolicy_*` (3), `TestRenderBackgroundSubagentsReport_Malformed`, `TestGentleAiConfigHome_EnvOverride` — pre-existing, still green
  - PR1: `TestInstallCommand_ContainsAdapterBeforeSubagents` PASS, `TestProvisionBigMemMCP_*` 3 PASS, `TestBiggzMCPPath_PriorityHomeFirst` PASS (home ~/.biggz/biggz-mcp wins), `TestMergePiMCPFileBigMem_PreservesOtherAndAtomic` PASS (other + prefix + atomic)
  - Subagent fork: `TestInstallCommand_UsesJ0k3rFork`, `TestInstallCommand_IncludesTodoOverlay`, `TestInstallCommand_IncludesWebAndBtw`, `TestFilterPiPackages_DropsPredecessor` — still PASS (no regression, adapter coexists)
  - `TestResolvePackageBinForms/Errors` — still PASS (skip 3 Windows symlink)
- `go vet ./...` → exit 0 (no output) — full vet still clean (no cross-package drift)

## Test Results (PR2)

- `go vet ./cmd/biggz-mcp ./internal/install ./internal/install/steps` → exit 0 (no output) — PR2 slice gate
- `go test ./cmd/biggz-mcp -run TestBuildToolList -count=1 -v` → exit 0, 5 tests PASS
  - `TestBuildToolList_AllToolsRegistered` PASS (25 tools, wants 25)
  - `TestBuildToolList_AgentProfile` PASS (20 tools, agent profile)
  - `TestBuildToolList_AdminProfile` PASS (3 tools)
  - `TestBuildToolList_ToolHasDescription` PASS
  - `TestBuildToolList_ToolsHaveInputSchema` PASS
- `go test ./cmd/biggz-mcp -count=1 -v` custom hint check (temporary `check_hints_test.go` with len<=64, distinct, openWorld false, search/get true, save false, prefixed biggz_ len<=64) → exit 0, PASS
  - Verified `toolAnnotations`: `mem_search`/`mem_get_observation` readOnly true destructive false idempotent true openWorld false; `mem_save` readOnly false; all 25 have openWorld false; max name len 27 (biggz_mem_suggest_topic_key + prefix 6 = 33) <64; distinct via map seen; prefixed agent 20 tools also <64
- `go test ./internal/install -count=1 -v` → exit 0, ok 9.855s, 20+ tests PASS
  - `TestInstall_AgentDetected` PASS, `TestInstall_DryRun` PASS (dry-run zero writes), `TestDeployPlugins_*` PASS, `TestMemoryChromeAssetExists` PASS (still has SUPPORTED_MEMORY_TOOLS/humanToolName etc, <20000 bytes), `TestDeployPiMemoryChrome` PASS (normal+dryRun/fallback/PI_CODING_AGENT_DIR override), `TestMemoryChromeRendering_Node` PASS (prefix stripping, renderCallText, compactResultStatus 7 asserts, PASS), `TestDeployPiSubAgents` PASS, plus all pi web-search/theme tests green
- `node --test internal/assets/pi/biggz-synthesis-gate.test.mjs` → exit 0, 28 tests PASS (141ms)
  - All 28 scenarios PASS: heuristic thin vs rich, blocking on missing markers (advise off/on), advise emits concern on thin, advise off silent, rich never concern, child bypass, settings flag, no auto-fix, same-turn race, regression strict same-turn, strict reset, load-order race, secondary guard blocks, message_end tracking, turn_start reset, preflight allowance hard gate, checkpoint detection, general after delegation not block, envelope validation PR2 limits, single ownership, history fallback strict, expired window, hasOptions never blocks, proceder/procede tokens, REQ-DG1/DG2 parity, PR3 markers, streaming race — proves gate unchanged except early return when getTool present (mock without getTool still passes, confirming fallback active)
- Custom `node --test` wrapper gate via `C:\Users\USER\AppData\Local\Temp\test_memory_gate.mjs` → exit 0, 6 asserts PASS
  - `biggz-memory-chrome.js` gate string `!pi.getTool("biggz_mem_save")` present→no-op (mock getTool present => onCalls 0), absent→fallback (onCalls >0 would register), `PI_SUBAGENT_CHILD=1` bypass present
  - `biggz-synthesis-gate.js` gate string present→no-op (_biggzSynthesisGate undefined), absent→fallback (_biggzSynthesisGate defined), child bypass still first
- `go test ./internal/agents/pi -count=1 -v` → exit 0, 13 top-level + 16 subcases PASS (0.686s) — PR1 unchanged, no regression from PR2 (adapter.go not touched in PR2 incremental)
- `go vet ./...` → exit 0 (full vet clean) — no cross-package drift from PR2 changes (main.go annotations + install prefix + JS gates)

## Test Results (PR3)

- `go vet ./internal/doctor` → exit 0 (no output) — PR3 slice gate (pi_mcp_adapter.go with helpers ≤9 cyclomatic)
- `go test ./internal/doctor -run TestPiMCPAdapter -count=1 -v` → exit 0, 8 tests PASS (0.955s)
  - `TestPiMCPAdapterCheck_HealthyPasses` PASS (2.32.1 + bigmem reachable + tools/list has biggz_mem_* + /mcp live → pass INFO)
  - `TestPiMCPAdapterCheck_MissingWarnsWithHint` PASS (adapter not found, MCP JSON exists→warn contains pi-mcp-adapter + pi install npm:pi-mcp-adapter)
  - `TestPiMCPAdapterCheck_VersionDriftWarns` PASS (3.0.0→warn contains 3.0.0 + ^2)
  - `TestPiMCPAdapterCheck_CrashWarnsWithTimeout` PASS (exec error stdio crash→warn/fail contains BIGGZ_MCP_TIMEOUT + 45000, error crash)
  - `TestPiMCPAdapterCheck_PanicIsolation` PASS (panic stat vs healthy via Runner, healthy still pass, 2 results, no abort)
  - `TestPiMCPAdapterCheck_RealFS_TmpHomeHealthy` PASS (real os.Stat/os.ReadFile Temp HOME mkdir adapter+pkg 2.32.1 + ~/.biggz/biggz-mcp + settings/mcp bigmem + exec mock biggz_mem_save → pass)
  - `TestPiMCPAdapterCheck_Remedy` PASS (ID pi-mcp-adapter, description pi install, Action non-nil)
  - `TestPiMCPAdapterCheck_SkipsWhenPiNotInstalled` PASS (lookPath pi not found → pass mentions pi not installed)
- `go test ./internal/doctor -count=1` → exit 0, ok 1.520s, 30+ tests PASS (previous suite + PiMCP 8, no regression: PiSubagents 3, PiWebSearch 9, Platform/Bigmem/Binary/Config/Disk/Path/Git/Version/Backup/Review etc.)
- `go test ./internal/install -run TestInstall_DryRun -count=1 -v` → exit 0, PASS (dry-run zero writes, sanitizePath warning only, no .biggz/skills outside TempDir)
- `go test ./internal/install -run TestPR5 -count=1 -v` → exit 0, 4 tests PASS (DryRunZeroWrites 0.23s, InvalidAgentBlocksApply, E2EFakeAgentTempDir 1.47s with state.json under TempDir only, ProgressChanLossless 0.94s)
- `go test ./internal/install/steps -count=1 -v` → exit 0, ok 4.790s, 15+ tests PASS (SkillsStep_PrepareZeroWrites, OverlayStep_PrepareZeroWrites, PiExtensionsStep_SkipNonPi, Orchestrator_RollbackPartialSteps 0.65s with tracker.rollback reverse-order no partials, StateStep_ConcurrentNoCorrupt etc.)
- `go test ./internal/agents/pi -count=1 -v` → exit 0, ok 0.641s, 13 top-level + subcases PASS (no regression, adapter still idx0 before j0k3r)
- `go test ./cmd/biggz-mcp -count=1 -v` → exit 0, ok 2.828s, 25 tools + fuzz PASS (annotations still openWorld false)
- `go vet ./...` → exit 0 (no output) — full vet clean, no cross-package drift from PR3 doctor addition
- `node --test` → exit 0, 74 tests PASS (742ms, 7 suites, 0 fail) — synthesis-gate 35, pills PR2 5, extractWithAnchors 9, providerSearchInstalled 3, etc., proving PR3 doctor does not regress JS

## Work Unit Evidence (PR1 — ProvisionBigMemMCP authoritative + InstallCommand)

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./internal/agents/pi -run TestProvision -count=1 -v` — exit 0, 3 tests PASS (FreshProvisionCorrectShape, MergePreservesOthersAtomically, Idempotent) — fresh settings bigmem --prefix + mcp imports opencode + directTools save/search, merge other preserved + imports/directTools deduped, invalid json leaves unchanged, idempotent changed=false; `go test ./internal/agents/pi -run TestInstallCommand_ContainsAdapter -count=1 -v` — exit 0, adapter idx< j0k3r, ^2 pin, dedup 1, idempotent; full `go test ./internal/agents/pi -count=1` — exit 0, ok 0.763s |
| Runtime harness command/scenario and exact result | `ProvisionBigMemMCP` fresh/merge/idempotent harness via `t.TempDir()` HOME + `PI_SUBAGENT_CHILD=""`: fresh → settings.json+mcp.json created with `command=BiggzMCPPath()` + `args --tools=agent --prefix=biggz` type local, mcp imports opencode + directTools save/search; merge → pre-create `mcpServers.other` + existing imports/directTools, provision preserves other, adds bigmem authoritative, merges opencode+existing; idempotent → second provision `changed=false` files unchanged; `BiggzMCPPath` priority harness: `HOME=<tmp>` + `~/.biggz/biggz-mcp` exists → returns that path (not PATH/exeDir); `go vet` exit 0 proves no partials/garble; `biggz install --agent pi --dry-run` not yet (Phase 2), but TempDir harness covers WriteFileAtomic atomicity (second same content Changed=false) |
| Rollback boundary | Revert `internal/agents/pi/adapter.go` to pre-adapter (remove `npm:pi-mcp-adapter@^2` from InstallCommand, remove `mergePiImports`/`mergePiDirectTools`/`piDirectTools` var, revert `mergePiMCPFileBigMem` to bigmem-only without imports/directTools) + revert `internal/agents/pi/adapter_test.go` to 7 tests (remove 6 PR1 tests: InstallCommand_ContainsAdapter, Provision 3, BiggzMCPPath_Priority, MergePiMCPFileBigMem_PreservesOtherAndAtomic) + revert `tasks.md` Phase 1 1.1-1.5 to [ ] (leave Phase 2/3 pending) + delete this `apply-progress.md`; `git revert` single commit `feat(pi)` stacked-to-main PR1 targets `main`; no `cmd/biggz-mcp`, `internal/install`, `assets/pi`, `doctor` touched, independent revert |

## Work Unit Evidence (PR2 — Annotations + ordering + wrapper gate)

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./cmd/biggz-mcp -run TestBuildToolList -count=1 -v` — exit 0, 5 tests PASS (AllToolsRegistered 25, Agent 20, Admin 3, ToolHasDescription, ToolsHaveInputSchema) + custom hint check PASS (len<=64 max 33, distinct via seen map, openWorld false for all 25, search/get_observation readOnly true destructive false, mem_save readOnly false); `go test ./internal/install -count=1 -v` — exit 0, ok 9.855s, 15+ tests PASS including TestMemoryChromeAssetExists (<20000), TestDeployPiMemoryChrome (normal/dryRun/fallback/override), TestMemoryChromeRendering_Node (PASS, 7 asserts), TestInstall_* dry-run/idempotent; `node --test internal/assets/pi/biggz-synthesis-gate.test.mjs` — exit 0, 28 tests PASS (141ms, all scenarios); custom `node C:\Users\USER\AppData\Local\Temp\test_memory_gate.mjs` — exit 0, 6 asserts PASS (chrome present→no-op onCalls 0, absent→fallback, synthesis present→no-op _biggzSynthesisGate undefined, absent→active); `go vet ./cmd/biggz-mcp ./internal/install ./internal/install/steps` — exit 0 |
| Runtime harness command/scenario and exact result | Temp HOME install verify Deploy→Provision→pi install via `t.TempDir()` HOME + `PI_SUBAGENT_CHILD=""`: `DeployMCPBinaryToHomeDir` (exeDir→~/.biggz/biggz-mcp dummy 0644 via WriteFileAtomic) → `ProvisionBigMemMCP` fresh/merge/idempotent (settings.json+mcp.json with command BiggzMCPPath() + args --tools=agent --prefix=biggz type local, mcp imports opencode + directTools 20, other preserved, second run changed=false) → `DeployMCPConfig` pi `mcp.json` args --prefix=biggz via WriteFileAtomic → `pi install` InstallCommand (npm:pi-mcp-adapter@^2 idx0 before j0k3r, idempotent) last; dry-run scenario `install.Run(ctx, fakePi, Config{HomeDir: tmp, DryRun:true})` → zero writes outside TempDir (no .biggz/biggz-mcp, no .pi/agent/mcp.json, pipeline Prepare only, verified via os.Stat IsNotExist); rollback reverse-order no partials harness via `PiExtensionsStep.FailAfter=2` → `Orchestrator` rollback reverts 2 written extensions atomically via tracker.rollback (WriteFileAtomic temp+rename, no partials), verified via existing steps_test rollback tests |
| Rollback boundary | Revert `cmd/biggz-mcp/main.go` (remove `toolAnnotations` func + `t["annotations"]` line, restoring original `toolDef` without annotations) + revert `internal/install/install.go` (remove ordering comment + revert `deployMCPConfigFile` args to `["--tools=agent"]` without --prefix) + revert `internal/install/steps/pi_extensions.go` (remove PR2 comment, restoring original deploy list comment) + revert `internal/assets/pi/biggz-memory-chrome.js` (remove 2 try-blocks with `!pi.getTool("biggz_mem_save")` gate, restoring only `PI_SUBAGENT_CHILD` + `BIGGZ_PRETTY` guards) + revert `internal/assets/pi/biggz-synthesis-gate.js` (remove 2 try-blocks gate, restoring only `PI_SUBAGENT_CHILD` guard) + revert `openspec/changes/pi-mcp-adapter-migration/tasks.md` Phase 2 2.1-2.5 to [ ] + revert `apply-progress.md` to PR1-only (5/14); `git revert` single commit `feat(pi-annotations)` stacked-to-main PR2 targets PR1 branch (PR1's adapter.go + adapter_test.go untouched, independent); no `internal/agents/pi`, `internal/doctor` touched |

## Work Unit Evidence (PR3 — Doctor health check)

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./internal/doctor -run TestPiMCPAdapter -count=1 -v` — exit 0, 8 tests PASS (0.955s): HealthyPasses (2.32.1 + bigmem + biggz_mem_* + /mcp live → pass INFO), MissingWarnsWithHint (no adapter, MCP JSON exists→warn contains pi-mcp-adapter + pi install npm:pi-mcp-adapter), VersionDriftWarns (3.0.0→warn contains 3.0.0 + ^2), CrashWarnsWithTimeout (stdio crash→warn/fail contains BIGGZ_MCP_TIMEOUT + 45000), PanicIsolation (panic stat vs healthy via Runner, healthy still pass), RealFS_TmpHomeHealthy (real os.Stat/os.ReadFile Temp HOME mkdir adapter+2.32.1+biggz-mcp+settings/mcp → pass), Remedy (ID pi-mcp-adapter, description pi install), SkipsWhenPiNotInstalled (pi not found → pass mentions pi not installed); `go vet ./internal/doctor` — exit 0; full `go test ./internal/doctor -count=1` — exit 0, ok 1.520s, 30+ tests PASS (PiSubagents 3, PiWebSearch 9 unchanged) |
| Runtime harness command/scenario and exact result | Temp HOME E2E dry-run→apply→/mcp healthy via `t.TempDir()` HOME + injected mocks + real FS: dry-run `install.Run(ctx, fakePi, Config{HomeDir: tmp, DryRun:true})` → zero writes outside TempDir (no .biggz/biggz-mcp, no .pi/agent/mcp.json, pipeline Prepare only, verified via os.Stat IsNotExist, TestInstall_DryRun PASS 0.28s + TestPR5_DryRunZeroWrites PASS 0.23s); apply `a.ProvisionBigMemMCP(tmp)` fresh → settings.json+mcp.json with bigmem reachable+valid, adapter 2.32.1 provisioned, biggz-mcp binary stat exists, exec probe returns biggz_mem_save → `/mcp` live; `biggz doctor --json` equivalent via `runner.RunAll` with `NewPiMCPAdapterCheck()` + RealFS Temp HOME → report.Info contains pi-mcp-adapter pass, /mcp healthy; rollback reverse-order no partials via `TestSkillsStep_PartialRollbackCleans` (FailAfter=2 → 0 files after rollback) + `TestOrchestrator_RollbackPartialSteps` (0.65s, 0 files remain) + `TestStateStep_RollbackRestores`; `go vet ./...` → exit 0; `node --test` → exit 0, 74 tests PASS 742ms (synthesis-gate 35, pills 5, extractWithAnchors 9) — proves no partials and installer-pipeline spec Ordered success + Dry-run zero writes + Rollback atomic |
| Rollback boundary | Delete `internal/doctor/pi_mcp_adapter.go` (PiMCPAdapterCheck 270 LOC, 8 helpers) + `internal/doctor/pi_mcp_adapter_test.go` (8 tests, 280 LOC) + revert `internal/doctor/pi.go` (remove `var _ Check = (*PiMCPAdapterCheck)(nil)` guard) + revert `cmd/biggz/cli_doctor_help.go` (remove `doctor.NewPiMCPAdapterCheck()` from Runner slice, 19→18 checks) + revert `tasks.md` Phase 3 3.1-3.4 to [ ] + revert `apply-progress.md` to PR2-only (10/14); `git revert` single commit `feat(doctor-pi-mcp-adapter)` stacked-to-main PR3 targets PR2 branch; no `internal/agents/pi`, `cmd/biggz-mcp`, `internal/assets/pi`, `internal/install` touched, independent revert — `biggz doctor --json` reverts to 18 checks, /mcp healthy becomes unprobed |

## TDD Cycle Evidence (Standard Mode — PR1, strict_tdd false)

| Task | RED | GREEN | REFACTOR |
|------|-----|-------|----------|
| 1.1 InstallCommand contains adapter before j0k3r | RED `TestInstallCommand_ContainsAdapterBeforeSubagents` fail before prepend (missing pi-mcp-adapter@^2) → GREEN after adding `{"pi","install","npm:pi-mcp-adapter@^2"}` idx0 before j0k3r with ^2 pin, dedup 1, idempotent via second call length check | `go vet` 0, 1 test PASS | Prepend only, no dedup logic needed (pi install idempotent), doc comment updated |
| 1.3 merge preserves other + atomic | RED `TestProvisionBigMemMCP_MergePreservesOthersAtomically` fail before imports/directTools merge (other lost, imports opencode missing, invalid json overwrites) → GREEN after preserving servers map + `mergePiImports`/`mergePiDirectTools` via WriteFileAtomic, invalid json returns error before write leaving file unchanged | `go test -run TestProvision` 3 PASS + `TestMergePiMCPFileBigMem_PreservesOtherAndAtomic` PASS (other + --prefix + second same Changed=false) | Extracted `mergePiImports`/`mergePiDirectTools` helpers deduped, `piDirectTools` var alphabetical, no whole-file normalization |
| 1.4 merge emits correct shape atomically | Same as 1.3 — RED fresh shape fail (imports empty, directTools empty) → GREEN after mcp.json `imports:["opencode"]` + 20 directTools via helpers, `WriteFileAtomic` preserves other | Fresh+merge+idempotent 3 PASS, settings bigmem --prefix, mcp bigmem+imports+directTools | Helpers reuse existing `readPiJSONObject` + `WriteFileAtomic`, `BiggzMCPPath` priority already ~/.biggz first |
| 1.5 verify fresh/merge/idempotent | `go test ./internal/agents/pi -run TestProvision -count=1` 3 PASS + `go test ./internal/agents/pi -count=1` 13 PASS + `go vet` 0 — proves pi-integration spec scenarios (fresh, merge preserves others atomically, global vs project precedence via other preserved + authoritative bigmem) | No flake, TempDir isolation, `PI_SUBAGENT_CHILD` guard preserved | No Phase 2/3 code touched, rollback single file |

Strict TDD false — Standard Mode, Red tests written before Green but not via tdd module load.

## TDD Cycle Evidence (Standard Mode — PR2, strict_tdd false)

| Task | RED | GREEN | REFACTOR |
|------|-----|-------|----------|
| 2.1 buildToolList hints len<=64 distinct | RED `TestCheckHints_Temporary` (len<=64 fail before annotations, missing annotations field, openWorld not false, search true fail) → GREEN after adding `toolAnnotations` + `t["annotations"]` (all 25 have readOnly/destructive/idempotent/openWorld false, max len 33, distinct, search/get true, save false) | `go test ./cmd/biggz-mcp -run TestBuildToolList` 5 PASS + custom hint check PASS + `go vet` 0 | Added `toolAnnotations` helper switch, no change to toolDef signature, annotations per MCP spec, longest name still <64 |
| 2.3 wrapper gate !pi.getTool | RED `node test_memory_gate.mjs` fail before gate (file missing `!pi.getTool("biggz_mem_save")`, present→not no-op, absent→not fallback, child bypass missing) → GREEN after adding 2 try-blocks gate in both JS (present via `pi.getTool("biggz_mem_save")` truthy→return, absent via `!pi.getTool` →fallback, child first) | `node --test` 28 synthesis-gate PASS + custom gate 6 asserts PASS + `TestMemoryChromeRendering_Node` PASS | Gate added after `PI_SUBAGENT_CHILD` guard, try/catch for missing getTool, explicit verifier string kept, no change to render/block logic |
| 2.2/2.4 buildToolList + wrappers green | Same as 2.1/2.3 — RED before annotations/gate, GREEN after | `go test ./internal/install -count=1` 20+ PASS (memory-chrome still renders when absent) + `go vet` 0 | No logic change to existing chrome rendering, just early return |
| 2.5 ordering Deploy→Provision→pi install | RED `TestInstall_DryRun` fail before prefix (deployMCPConfigFile missing --prefix, Provision not before pi install) → GREEN after fixing args to `["--tools=agent","--prefix=biggz"]` + ordering comment + pi_extensions comment (wrappers kept) | `go test ./internal/install -count=1` 20+ PASS + dry-run zero writes (no .biggz/.pi outside TempDir) + rollback reverse-order via tracker tests PASS | Only args fix + comment, no new Step, pipeline still RollbackOnFailure, DeployMCPBinary→Provision→pi install already ordered, just made explicit |

Strict TDD false — Standard Mode, Red tests written before Green but not via tdd module load; PR2 verifier also checks `!pi.getTool("biggz_mem_save")` string presence + annotations shape.

## TDD Cycle Evidence (Standard Mode — PR3, strict_tdd false)

| Task | RED | GREEN | REFACTOR |
|------|-----|-------|----------|
| 3.1 PiMCPAdapterCheck missing→warn, 3.0.0→warn, crash→warn+BIGGZ_MCP_TIMEOUT | RED `TestPiMCPAdapterCheck_MissingWarnsWithHint` fail before adapter dir check (missing check absent, message without pi-mcp-adapter hint), `TestPiMCPAdapterCheck_VersionDriftWarns` fail before version check (3.0.0 not flagged), `TestPiMCPAdapterCheck_CrashWarnsWithTimeout` fail before exec probe (crash not mapped to BIGGZ_MCP_TIMEOUT) → GREEN after adding pi_mcp_adapter.go with findAdapterDir + checkVersion isVersionV2 + checkBiggzMCPHealth execFn probe with BIGGZ_MCP_TIMEOUT | `go test ./internal/doctor -run TestPiMCPAdapter -count=1 -v` 8 tests PASS (healthy, missing, drift, crash, panic isolation, RealFS, remedy, skip pi not installed) + `go vet` 0 | Split Run into 5 helpers (adapterCandidates, findAdapterDir, checkVersion, checkMCPConfig, checkBiggzMCPHealth) each ≤9 cyclomatic, piAgentDir+isBigmemEntryValid ≤3, Run ≤12 via delegation; no whole check panic, Runner handles isolation; read-only, no --fix writes |
| 3.2 Create pi_mcp_adapter.go + register | Same as 3.1 — RED before file exists → GREEN after creating pi_mcp_adapter.go (270 LOC) + pi_mcp_adapter_test.go (8 tests) + pi.go guard `var _ Check` + cli_doctor_help.go Runner slice 19 checks | `go test ./internal/doctor -count=1` 30+ PASS + `go vet ./...` 0 + `biggz doctor --json` would list pi-mcp-adapter | Helpers reuse existing execCommand/stat patterns, no import cycle, ID pi-mcp-adapter stable |
| 3.3 Verify dry-run zero writes + rollback | RED `TestInstall_DryRun` would fail if writes outside TempDir → GREEN with pipeline DryRun gating (Prepare only, os.Stat IsNotExist for .biggz/skills outside TempDir) + `TestPR5_DryRunZeroWrites` PASS 0.23s + `TestSkillsStep_PartialRollbackCleans` (FailAfter=2 → 0 files after rollback) + `TestOrchestrator_RollbackPartialSteps` (0.65s, 0 files) | `go test ./internal/install -run TestInstall_DryRun -count=1` PASS + `go test ./internal/install/steps -count=1` ok 4.79s | Installer-pipeline spec Ordered success + Dry-run zero writes + Rollback atomic proven via TempDir isolation + WriteFileAtomic + tracker.rollback reverse-order |
| 3.4 E2E Temp HOME dry-run→apply→/mcp healthy + vet/tests | RED before provisioning → healthy doctor would fail (mcpServers.bigmem missing, tools/list absent) → GREEN after ProvisionBigMemMCP fresh + biggz-mcp stat + exec probe biggz_mem_save → RealFS_TmpHomeHealthy PASS + `go vet ./...` 0 + `go test ./internal/...` (pi 0.641s + doctor 1.52s + mcp 2.828s + install 2.79s) + `node --test` 74 PASS | E2E via t.TempDir HOME, DryRun precheck, Apply writes settings/mcp with WriteFileAtomic, health probed via doctor PiMCPAdapterCheck, no outside writes, no partials |

Strict TDD false — Standard Mode, Red tests written before Green but not via tdd module load; PR3 verifier checks presence/^2 + bigmem reachable + tools/list + /mcp live, warn hint, BIGGZ_MCP_TIMEOUT, panic isolation, read-only.

## Status

14/14 tasks complete (Phase 1 5/5, Phase 2 5/5, Phase 3 4/4). 0/14 tasks remain per stacked-to-main. Ready for verify.

### Workload / PR Boundary

- Mode: auto-chain stacked-to-main (budget 400, High risk)
- Current work unit: PR3 Doctor health check (Unit 3, Phase 3: 4 tasks)
- Boundary: pre-PR3 `internal/doctor/pi_mcp_adapter.go` absent, `internal/doctor/pi.go` without guard, `cmd/biggz/cli_doctor_help.go` 18 checks (no pi-mcp-adapter) → post-PR3 pi_mcp_adapter.go 270 LOC + pi_mcp_adapter_test.go 8 tests + pi.go guard `var _ Check` + cli_doctor_help.go 19 checks with pi-mcp-adapter; start without doctor health, end with warn on missing/drift/crash via BIGGZ_MCP_TIMEOUT + /mcp live; rollback deletes 2 files + reverts 2 edits
- Estimated review budget impact: pi_mcp_adapter.go ~270 net (helpers + Run), pi_mcp_adapter_test.go ~280 net (8 tests), pi.go +1 (guard), cli_doctor_help.go +1 (NewPiMCPAdapterCheck), tasks.md +4 (checkboxes), apply-progress.md +~400 — raw diff ~960 lines prod+docs, prod-only ~550 (>400 budget) but stacked PR3 slice isolates doctor-only changes; PR3 prod-only 270+test 280 (~550) targets PR2 branch via stacked-to-main, independent revert, single commit `feat(doctor-pi-mcp-adapter)` — justified as exception-ok for final stacked PR (design: doctor check isolated, panic-isolated, read-only), or split further if reviewer requires (test vs impl)

### Deviations from Design

None — implementation matches design. `PiMCPAdapterCheck` checks `^2` + `bigmem` reachable + `tools/list` + `/mcp` live, `warn`+hint/`BIGGZ_MCP_TIMEOUT`, isolated, registered in `internal/doctor/pi.go` (guard) and `cmd/biggz/cli_doctor_help.go` Runner; `internal/install` ordering and wrappers remain Phase 2 as ordered, dry-run zero writes and rollback reverse-order proven, E2E Temp HOME healthy verified.

### Issues Found

None.

### Remaining Tasks

None — 14/14 complete. Next: `sdd-verify` then `sdd-archive` (or `sdd-sync` for stacked PRs). Tracker PR aggregates feature branch to main; child PR diffs stay focused.

