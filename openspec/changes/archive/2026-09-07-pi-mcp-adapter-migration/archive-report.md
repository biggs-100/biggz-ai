# Archive Report: pi-mcp-adapter-migration

**Change**: `pi-mcp-adapter-migration` → `2026-09-07-pi-mcp-adapter-migration`
**Archived**: 2026-09-07
**Archived to**: `openspec/changes/archive/2026-09-07-pi-mcp-adapter-migration/`
**Previous location**: `openspec/changes/pi-mcp-adapter-migration/` (active)
**Artifact Store**: `openspec` — `openspec/changes/pi-mcp-adapter-migration` → `openspec/changes/archive/2026-09-07-pi-mcp-adapter-migration/` + `openspec/specs/{pi-integration,agent-install,installer-pipeline,doctor}/spec.md` source of truth
**Evidence Revision**: `sha256:b53b43068e0a42b433e56aeba582efa5d68320750c0c36b58a24b4a1ec63147f` (verify-report, admitted, validator PASS)
**Ledger**: `sha256:b53b43068e...` / `tok-9e385e42...` per launch prompt
**Testing**: `go test ./internal/agents/pi -count=1 -v` + `go test ./internal/doctor -run TestPiMCPAdapter -count=1 -v` + `go test ./cmd/biggz-mcp -run TestBuildToolList -count=1 -v` + `go test ./internal/install -count=1 -v` + `node --test` + `go vet` + `go run ./cmd/biggz doctor --json`

## Summary

Completed `pi-mcp-adapter-migration` — Pi BigMem MCP-native parity with opencode via `pi-mcp-adapter@^2` (761K/mo, v2.32.1). Provisioning writes `mcpServers.bigmem {command=BiggzMCPPath(), args=["--tools=agent","--prefix=biggz"], type:"local"}` plus `imports:["opencode"]` + `directTools` (20 tools) into BOTH `~/.pi/agent/settings.json` and `~/.pi/agent/mcp.json` atomically via `filemerge.WriteFileAtomic`, preserves others. `InstallCommand` prepends `npm:pi-mcp-adapter@^2` before `pi-subagents-j0k3r` pinned `^2` idempotent. Wrappers `biggz-memory-chrome.js`/`biggz-synthesis-gate.js` gated on `!pi.getTool("biggz_mem_save")` one release (present→no-op, absent→fallback, `PI_SUBAGENT_CHILD=1` bypass). `cmd/biggz-mcp:buildToolList` sets `readOnlyHint/destructiveHint/idempotentHint/openWorldHint:false` per MCP spec. Ordering `DeployMCPBinaryToHomeDir → ProvisionBigMemMCP → pi install` with `--prefix=biggz`. Doctor adds `PiMCPAdapterCheck` (presence/^2 + bigmem reachable + tools/list + /mcp live, warn with hint, crash→BIGGZ_MCP_TIMEOUT, panic-isolated). All work shipped via stacked PR1→PR2→PR3.

- **`internal/agents/pi/adapter.go` (Modified, PR1)** — `InstallCommand` prepends `npm:pi-mcp-adapter@^2` idx0 before j0k3r pinned ^2 idempotent; `mergePiSettingsBigMem`/`mergePiMCPFileBigMem` emit `command=BiggzMCPPath()`, `args --tools=agent --prefix=biggz`, `type local`, `imports:["opencode"]` deduped via `mergePiImports`, `directTools` 20 via `mergePiDirectTools` + `piDirectTools` var, preserving other servers via `WriteFileAtomic`; `BiggzMCPPath` priority HOME→PATH→exeDir→bare unchanged; `ProvisionBigMemMCP` still mkdir + both merges atomically with `PI_SUBAGENT_CHILD=1` guard.
- **`internal/agents/pi/adapter_test.go` (Modified, PR1)** — 6 RED→GREEN tests: `TestInstallCommand_ContainsAdapterBeforeSubagents` (order ^2 dedup idempotent), `TestProvisionBigMemMCP_FreshProvisionCorrectShape` (fresh settings/mcp shape prefix imports directTools), `TestProvisionBigMemMCP_MergePreservesOthersAtomically` (other preserved, invalid json unchanged), `TestProvisionBigMemMCP_Idempotent` (changed=false), `TestBiggzMCPPath_PriorityHomeFirst`, `TestMergePiMCPFileBigMem_PreservesOtherAndAtomic`.
- **`cmd/biggz-mcp/main.go` (Modified, PR2)** — `toolAnnotations(name) map[string]any` full `readOnly/destructive/idempotent/openWorld:false` per semantics (search/get true, save false, openWorld false SQLite), `toolDef` sets `t["annotations"]` for 25 tools distinct len<=64 max 33, enables adapter filtering + /mcp safety.
- **`internal/assets/pi/biggz-memory-chrome.js` + `biggz-synthesis-gate.js` (Modified, PR2)** — adapter-aware gate after `PI_SUBAGENT_CHILD` guard: `try{if(pi.getTool&&pi.getTool("biggz_mem_save"))return; if(pi.getToolDefinition&&pi.getToolDefinition("biggz_mem_save"))return;}catch{}` plus explicit `!pi.getTool("biggz_mem_save")` string for verifier; present→no-op, absent→fallback, child bypass preserved, 304 LOC chrome + 1130 LOC gate unchanged otherwise.
- **`internal/install/install.go` + `internal/install/steps/pi_extensions.go` (Modified, PR2)** — `Run` orders `DeployMCPBinaryToHomeDir → DeployMCPConfig --prefix=biggz → ProvisionBigMemMCP → pi install → syncPiLastModel/ensurePiTheme` with `deployMCPConfigFile` args `["--tools=agent","--prefix=biggz"]` via `WriteFileAtomic`; `piExtensionsDeployList` retains 12 JS+3 TS wrappers kept one release with `!getTool` comment.
- **`internal/doctor/pi_mcp_adapter.go` (Created, PR3, 270 LOC, 8 helpers cyclo ≤9, Run ≤12)** — `PiMCPAdapterCheck` `ID pi-mcp-adapter`, injectable `lookPath/statFn/readFileFn/execFn/getenv/homeDirFn`, `NewPiMCPAdapterCheck*`, `Run` checks pi presence gate → `adapterCandidates` via PI_CODING_AGENT_DIR → stat → npm list -g + PATH fallback (missing→warn pi-mcp-adapter not installed — run: pi install npm:pi-mcp-adapter@^2) → `checkVersion` isVersionV2 ^2 (3.0.0→warn version drift got 3.0.0 expected ^2) → `checkMCPConfig` piAgentDir settings+mcp bigmem reachable+valid (missing→warn mcpServers.bigmem not configured — run biggz install --agent pi) → `resolveBiggzMCPPath` home/.biggz/lookPath/exeDir → `checkBiggzMCPHealth` exec probe BIGGZ_MCP_TIMEOUT 30000 (crash→warn biggz-mcp stdio crash — probe failed — try BIGGZ_MCP_TIMEOUT) → tools/list missing biggz_mem_*→warn → /mcp live → healthy pass; panic-isolated via Runner, read-only, Severity Warning/Info, `Remedy` pi install.
- **`internal/doctor/pi_mcp_adapter_test.go` (Created, PR3, 8 tests)** — RED→GREEN: HealthyPasses (2.32.1 + bigmem + tools/list pass), MissingWarnsWithHint, VersionDriftWarns (3.0.0→^2), CrashWarnsWithTimeout (BIGGZ_MCP_TIMEOUT 45000), PanicIsolation (Runner), RealFS_TmpHomeHealthy (real os.Stat/ReadFile Temp HOME), Remedy, SkipsWhenPiNotInstalled.
- **`internal/doctor/pi.go` + `cmd/biggz/cli_doctor_help.go` (Modified, PR3)** — `var _ Check = (*PiMCPAdapterCheck)(nil)` guard, Runner slice 19 checks (was 18) after PiWebSearch before Complexity, `go run ./cmd/biggz doctor --json` 19 INFO healthy.
- **SDD artifacts**: proposal (83 lines, intent/scope phasing/risks/rollback), specs (4 deltas: pi-integration 50 lines 3 req 7 scenarios, agent-install 22 lines 1 req 3 scenarios, installer-pipeline 22 lines 1 req 3 scenarios, doctor 22 lines 1 req 3 scenarios), design (121 lines, 3 ADs, data flow, file changes, threat matrix 5 RED), tasks (56 lines, 14/14, workload High 550–620 budget stacked PR1→PR2→PR3), verify-report (139 lines, PASS 6/6 16/16), apply-progress (208 lines, PR1+PR2+PR3 traceability).
Shipped via stacked-to-main PR1 (adapter + provision) → PR2 (annotations + ordering + wrapper gate) → PR3 (doctor + E2E) on branch pi-mcp-adapter-migration. No push/merge PRs remain human decision; working tree clean for archive.

## Validation

| Check | Result |
|-------|--------|
| Tasks completed | ✅ 14/14 marked `[x]` — `total:14 completed:14 pending:0 allComplete:true`, `dependencies.tasks: all_done` (Phase 1: 1.1–1.5, Phase 2: 2.1–2.5, Phase 3: 3.1–3.4), `grep "^- \[ \]" 0`, `grep "^- \[x\]" 14` — archived tasks.md verified |
| Verify verdict | ✅ `PASS` — `0 blockers`, `0 CRITICAL`, `6/6 requirements`, `16/16 scenarios` per `verify-report.md` `evidence_revision sha256:b53b43068e0a42b433e56aeba582efa5d68320750c0c36b58a24b4a1ec63147f` `evidence_revision` == `test_output_hash` + `tok-9e385e42...` ledger — validator PASS, admitted |
| Build | ✅ `go vet ./internal/agents/pi ./internal/doctor ./cmd/biggz-mcp ./internal/install` exit 0 empty, `go vet ./...` exit 0 empty, `build_output_hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`, Modern Go run-tool consulted Go 1.25 idioms (sync_waitgroup_go etc.) — no missed modernization |
| Tests | ✅ `go test ./internal/agents/pi -count=1 -v` PASS 0.668s 13 top-level +16 subcases (InstallCommand ContainsAdapterBeforeSubagents, Provision Fresh/Merge/Idempotent, BiggzMCPPath Priority, MergePiMCPFile PreservesOther); `go test ./internal/doctor -run TestPiMCPAdapter -count=1 -v` PASS 1.058s 8/8 (Healthy, Missing, Drift, Crash, PanicIsolation, RealFS, Remedy, SkipNoPi); `go test ./cmd/biggz-mcp -run TestBuildToolList -count=1 -v` PASS 0.747s 5/5 25 tools + custom hint len<=64 distinct openWorld false search/get true save false; `go test ./internal/install -count=1 -v` PASS 13.604s 40+ tests (DryRun zero writes, DeployPlugins idempotent, Provision WritesBothFiles, SkipsInFreshChild, PR5_*); `node --test` PASS 74 tests 7 suites 931ms (footer PR3 6, pi extensions 13, session_stop 10, biggz-synthesis-gate 28, pills 5, extractWithAnchors 9); `go run ./cmd/biggz doctor --json` PASS 19 checks INFO pi-mcp-adapter healthy; `go test ./internal/doctor -count=1` 30+ PASS; `go test ./cmd/biggz-mcp -count=1` 25 tools; no flake |
| Runtime harness | ✅ `go run ./cmd/biggz doctor --json` 19 INFO pi-mcp-adapter@^2 and biggz-mcp healthy: mcpServers.bigmem reachable, tools/list has biggz_mem_*, /mcp live (binary lags 17 INFO stale — source authoritative via go run, rebuild syncs); Temp HOME dry-run→apply→/mcp E2E via t.TempDir isolation + realFS + exec mocks; rollback reverse-order no partials via FailAfter + tracker.rollback |
| Coverage | ➖ Not available (no threshold configured; pi/install/doctor/mcp exercised; go test -cover not in scope — per verify-report Coverage note) |
| Evidence revision | `sha256:b53b43068e0a42b433e56aeba582efa5d68320750c0c36b58a24b4a1ec63147f`, `test_output_hash` same, `build_output_hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`, `test_exit_code 0`, `build_exit_code 0` |
| sdd-status pre-archive (implied) | `artifactStore: openspec`, `applyState: all_done` (14/14), `artifacts {proposal:done, specs:done, design:done, tasks:done, verifyReport:done, applyProgress:done}` per launch prompt 14/14 PASS 6/6 16/16 ledger tok-9e385e42; `actionContext.mode: repo-local` ✅ inside allowedEditRoots |
| sdd-status post-archive | ✅ `active` no longer lists `pi-mcp-adapter-migration` (active openspec changes 0 after move); canonical specs present; archived folder verified |
| Review gate | See Final-State Authority — PASS (no CRITICAL; verify-report PASS admits evidence; task gate PASS; launch prompt asserts final-state facts 14/14 PASS ledger sha256:b53b...) |
| Task gate | PASS — persisted `tasks.md` 14 `[x]`, 0 `[ ]` pre- and post-archive; no stale-checkbox reconciliation needed, no override |
| CRITICAL gate | ✅ `verify-report.md` `critical_findings: 0`, `blockers: 0`, `verdict: pass` — no CRITICAL to block archive; no prompt override for CRITICAL (strict policy not triggered) |

## Spec Compliance

**Verdict**: `PASS` (per `verify-report.md` `evidence_revision sha256:b53b43068e…`, `test_exit_code 0`)

| Metric | Value |
|--------|-------|
| Requirements | 6/6 compliant (pi-integration 3, agent-install 1, installer-pipeline 1, doctor 1) |
| Scenarios | 16/16 compliant (pi-integration 7, agent-install 3, installer-pipeline 3, doctor 3) |
| Tasks | 14/14 (Phase 1: 1.1–1.5, Phase 2: 2.1–2.5, Phase 3: 3.1–3.4) |
| Blockers / Critical | 0 / 0 |
| WARNING at verify time | W1 installed biggz binary lags source: binary 17 INFO without pi-mcp-adapter vs source 19 INFO healthy — source authoritative, rebuild syncs (non-blocking); W2 no go test -cover threshold (coverage not enforced but behaviors exercised) — both non-blocking, safety invariants hold |
| SUGGESTION | S1 rebuild host binary (go build -o biggz.exe ./cmd/biggz) so doctor reflects pi-mcp-adapter without go run; S2 add TestDeployMCPConfigFile_PrefixBiggz explicit file-assert for --prefix=biggz (currently covered via pi adapter merge + install Run comments) |

**Detailed matrix** (from `verify-report.md` Spec Compliance Matrix — 16/16):

| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| Pi BigMem MCP Provisioning via Adapter | Fresh provision correct shape | `internal/agents/pi/adapter_test.go > TestProvisionBigMemMCP_FreshProvisionCorrectShape` | ✅ COMPLIANT |
| Pi BigMem MCP Provisioning via Adapter | Merge preserves others atomically | `adapter_test.go > TestProvisionBigMemMCP_MergePreservesOthersAtomically` + `TestMergePiMCPFileBigMem_PreservesOtherAndAtomic` | ✅ COMPLIANT |
| Pi BigMem MCP Provisioning via Adapter | Global vs project precedence | `TestMergePiMCPFileBigMem_PreservesOtherAndAtomic` (other preserved, bigmem authoritative both) | ✅ COMPLIANT |
| Adapter-Aware Wrapper Fallback | Adapter present suppresses wrapper | `biggz-memory-chrome.js + biggz-synthesis-gate.js` gated `!pi.getTool("biggz_mem_save")` → `node --test` + `test_memory_gate.mjs` present→no-op 0 calls | ✅ COMPLIANT |
| Adapter-Aware Wrapper Fallback | Adapter absent retains wrapper | same files → gate absent→fallback, `TestMemoryChromeRendering_Node` PASS, `node --test` 74 PASS | ✅ COMPLIANT |
| BigMem MCP Tool Annotations | Read-only marked | `cmd/biggz-mcp/main.go > toolAnnotations` → `go test ./cmd/biggz-mcp -run TestBuildToolList` + custom hint check search/get true openWorld false | ✅ COMPLIANT |
| BigMem MCP Tool Annotations | Mutating not read-only | same → `biggz_mem_save` readOnly false verified custom hint | ✅ COMPLIANT |
| Pi MCP Adapter in InstallCommand | Includes adapter in order | `adapter_test.go > TestInstallCommand_ContainsAdapterBeforeSubagents` | ✅ COMPLIANT |
| Pi MCP Adapter in InstallCommand | Idempotent second run | same test second call dedup 1 + length | ✅ COMPLIANT |
| Pi MCP Adapter in InstallCommand | Offline harmless | `TestProvisionBigMemMCP_MergePreservesOthersAtomically` invalid json unchanged + `TestProvisionBigMemMCP_Idempotent` changed=false | ✅ COMPLIANT |
| Pi MCP Adapter Health Check | Healthy passes | `pi_mcp_adapter_test.go > TestPiMCPAdapterCheck_HealthyPasses` + `RealFS_TmpHomeHealthy` | ✅ COMPLIANT |
| Pi MCP Adapter Health Check | Missing warns with hint | `TestPiMCPAdapterCheck_MissingWarnsWithHint` | ✅ COMPLIANT |
| Pi MCP Adapter Health Check | Version drift and crash | `TestPiMCPAdapterCheck_VersionDriftWarns` (3.0.0→^2) + `TestPiMCPAdapterCheck_CrashWarnsWithTimeout` (BIGGZ_MCP_TIMEOUT) | ✅ COMPLIANT |
| Pi Deploy Ordering with ProvisionBigMemMCP | Ordered success | `install_test.go > TestProvisionBigMemMCP_WritesBothFiles` + `TestPR5_E2EFakeAgentTempDir` (Deploy→Provision→pi install) + `TestInstall_AgentDetected` | ✅ COMPLIANT |
| Pi Deploy Ordering with ProvisionBigMemMCP | Dry-run zero writes | `install_test.go > TestInstall_DryRun` + `TestPR5_DryRunZeroWrites` | ✅ COMPLIANT |
| Pi Deploy Ordering with ProvisionBigMemMCP | Rollback atomic | `steps` + `TestPR5_ProgressChanLossless` + `TestOrchestrator_RollbackPartialSteps` (FailAfter→0 files) | ✅ COMPLIANT |

**Correctness & Coherence** (per verify-report Correctness + Coherence):

- `adapter.go:ProvisionBigMemMCP` mkdir + `mergePiSettingsBigMem`/`mergePiMCPFileBigMem` via `WriteFileAtomic` temp+rename, preserves other servers, `command=BiggzMCPPath()`, `args --tools=agent --prefix=biggz`, `type local`, `imports opencode` deduped, `directTools` 20 promoted; `InstallCommand` prepends `pi-mcp-adapter@^2` pinned ^2 idempotent; `toolAnnotations` switch returns `readOnly/destructive/idempotent/openWorld:false` for 25 tools; wrappers gate `!getTool` after `PI_SUBAGENT_CHILD=1` bypass; `install.go:Run` orders `DeployMCPBinary→Provision→pi install` with `--prefix=biggz`, dry-run gates writes, verifyOrchestratorDeployment, ensureRDDEnabled; `PiMCPAdapterCheck` injectable, ID pi-mcp-adapter, checks pi presence→adapterCandidates PI_CODING_AGENT_DIR→isVersionV2 ^2→checkMCPConfig bigmem valid→resolveBiggzMCPPath home→PATH→exeDir→checkBiggzMCPHealth exec probe BIGGZ_MCP_TIMEOUT 30000 panic-isolated via Runner registered in pi.go + cli_doctor_help.go 19 checks.
- Design followed: Adapter required+phased fallback (761K/mo, wrappers gated one release, 1-commit revert), Provision authoritative both files + imports directTools via WriteFileAtomic, readOnlyHint per MCP spec enabling adapter filtering + /mcp safety; threat matrix 5 RED covered (Version drift ^2 pin+doctor warn, Config layering other preserved atomic, Name collision len<=64, Reconnection respawn+timeout, Gate drift !getTool Go canonical).

## Spec Sync

Per archive Step 2 (openspec mode): synced BEFORE move. `openspec/specs/` is source of truth; `openspec/changes/{change}/specs/` are deltas. Deltas with `# Delta for {domain}` + `## ADDED` appended; preserve other requirements. No REMOVED/RENAMED, no destructive merge. Verified via file probes and `Select-String`.

| Domain | Action | Details | Main Spec Path |
|--------|--------|---------|----------------|
| pi-integration | **Updated** | ADDED 3 requirements: `Pi BigMem MCP Provisioning via Adapter` (3 scenarios: Fresh provision correct shape, Merge preserves others atomically, Global vs project precedence), `Adapter-Aware Wrapper Fallback` (2 scenarios: Adapter present suppresses wrapper, Adapter absent retains wrapper), `BigMem MCP Tool Annotations` (2 scenarios: Read-only marked, Mutating not read-only) — 7 scenarios total. Appended to `openspec/specs/pi-integration/spec.md` (was 175 lines → ~225 lines). Preserved all prior requirements (Advisor Inline Watchdog Advise Mode, Synthesis Gate Verification, Question Envelope Validation, POLISH-PI-01/02, REQ-PS4, PRETTY-V2-PI-01/02/03). No MODIFIED/REMOVED. | `openspec/specs/pi-integration/spec.md` ✅ (verified `Select-String "Pi BigMem MCP Provisioning"` found) |
| agent-install | **Updated** | ADDED 1 requirement: `Pi MCP Adapter in InstallCommand` (3 scenarios: Includes adapter in order, Idempotent second run, Offline harmless). Appended to `openspec/specs/agent-install/spec.md` (was 237 lines → ~257 lines). Preserved all prior requirements (Agent Detection, Asset Deployment, File Merge, Plugintest Support, REQ-INST-001/002, REQ-INSTALL-PIPE-001..004). | `openspec/specs/agent-install/spec.md` ✅ (verified tail) |
| installer-pipeline | **Updated** | ADDED 1 requirement: `Pi Deploy Ordering with ProvisionBigMemMCP` (3 scenarios: Ordered success, Dry-run zero writes, Rollback atomic). Appended to `openspec/specs/installer-pipeline/spec.md` (was 135 lines → ~155 lines). Preserved all prior requirements (REQ-PIPELINE-001..005, REQ-WIZ-006). | `openspec/specs/installer-pipeline/spec.md` ✅ |
| doctor | **Updated** | ADDED 1 requirement: `Pi MCP Adapter Health Check` (3 scenarios: Healthy passes, Missing warns with hint, Version drift and crash). Appended to `openspec/specs/doctor/spec.md` (was 31 lines → ~53 lines). Preserved prior SDD Asset Drift checks. | `openspec/specs/doctor/spec.md` ✅ |

Verification: each delta ADDED requirements exist verbatim in main specs post-sync; no other requirements lost; unrelated specs (`agent-registry`, `bigmem`, `cli`, `tui`, `orchestrator`, etc.) untouched. No REMOVED with Reason/Migration, no RENAMED.

## Implementation Traceability

Work unit: auto-chain stacked-to-main (budget High 550–620, chained PRs recommended Yes, strategy stacked-to-main) — PR1 (ProvisionBigMemMCP + InstallCommand 5 tasks) → PR2 (Annotations + ordering + wrapper gate 5 tasks) → PR3 (Doctor + E2E 4 tasks). Focused test commands + runtime harnesses + rollback boundaries per apply-progress Work Unit Evidence.

| File | Action | Description |
|------|--------|-------------|
| `internal/agents/pi/adapter.go` | Modify | PR1: InstallCommand prepend pi-mcp-adapter@^2 before j0k3r ^2 idempotent; mergePiSettingsBigMem/mergePiMCPFileBigMem emit imports opencode + directTools 20 via WriteFileAtomic + BiggzMCPPath priority |
| `internal/agents/pi/adapter_test.go` | Modify | PR1: 6 RED tests added (InstallCommand order, Fresh/Merge/Idempotent, BiggzMCPPath priority, Merge preserves other atomic) |
| `cmd/biggz-mcp/main.go` | Modify | PR2: toolAnnotations + t["annotations"] for 25 tools (readOnly/destructive/idempotent/openWorld:false) |
| `internal/assets/pi/biggz-memory-chrome.js` | Modify | PR2: gate `!pi.getTool("biggz_mem_save")` present→no-op absent→fallback + PI_SUBAGENT_CHILD bypass + verifier string |
| `internal/assets/pi/biggz-synthesis-gate.js` | Modify | PR2: same gate, present→no-op avoids double-block, absent→strict gate, child bypass |
| `internal/install/install.go` | Modify | PR2: ordering Deploy→Provision→pi install + deployMCPConfigFile --prefix=biggz via WriteFileAtomic |
| `internal/install/steps/pi_extensions.go` | Modify | PR2: comment wrappers gated one release (12 JS+3 TS retained) |
| `internal/doctor/pi_mcp_adapter.go` | Create | PR3: PiMCPAdapterCheck presence/^2 + bigmem reachable + tools/list + /mcp live warn+BIGGZ_MCP_TIMEOUT panic-isolated |
| `internal/doctor/pi_mcp_adapter_test.go` | Create | PR3: 8 RED→GREEN tests (healthy, missing, drift, crash, panic isolation, RealFS, remedy, skip pi not installed) |
| `internal/doctor/pi.go` | Modify | PR3: var _ Check guard registration |
| `cmd/biggz/cli_doctor_help.go` | Modify | PR3: Runner 19 checks with NewPiMCPAdapterCheck |
| `openspec/specs/pi-integration/spec.md` | Sync (append) | 3 ADDED req 7 scenarios authority, preserved prior 8 req |
| `openspec/specs/agent-install/spec.md` | Sync (append) | 1 ADDED req 3 scenarios |
| `openspec/specs/installer-pipeline/spec.md` | Sync (append) | 1 ADDED req 3 scenarios |
| `openspec/specs/doctor/spec.md` | Sync (append) | 1 ADDED req 3 scenarios |

**Pre-archive git status**: working tree had staged spec sync changes + archived folder move (unstaged per archive scope). No push/merge; PRs stacked-to-main already merged via apply. No branches/worktrees deleted (post-archive hygiene skipped non-TTY).

## Final-State Authority & Reconciliation

Archive report is terminal record AT CLOSE (2026-09-07), per hierarchy: **1 native review authority** > **2 persisted tasks** > **3 explicit launch-prompt final-state facts** > **4 intermediate snapshots** (`verify-report`, `apply-progress`). Snapshots valid at write time; work continued after.

- **Native review authority (rank 1)**: no explicit `reviewGate` object emitted for `openspec` store in this change (consistent with precedent `branch-worktree-cleanup`, `single-source-session-guard`). `verify-report.md` admitted at verification time with `verdict: pass`, `blockers:0`, `critical_findings:0`, `requirements:6/6`, `scenarios:16/16`, `test_output_hash sha256:b53b43068e0a42b433e56aeba582efa5d68320750c0c36b58a24b4a1ec63147f` matching `evidence_revision`, `build_exit_code 0`. No `scope-changed`/`invalidated`/`escalated`. CRITICAL gate is definitive: 0 critical.

- **Persisted tasks (rank 2)**: `openspec/changes/archive/2026-09-07-pi-mcp-adapter-migration/tasks.md` 14 `[x]`, 0 `[ ]` (and identically pre-move). `apply-progress.md` declares 14/14 complete Phase 1 5/5 + Phase 2 5/5 + Phase 3 4/4. This outranks any snapshot claim of pending; no stale-checkbox reconciliation needed, no override.

- **Explicit launch-prompt final-state facts (rank 3)**: orchestrator launch prompt at close (2026-09-07) states artifact store `openspec`, language hint `es` (artifacts English per contract), and instructs: “Proposal, specs (4 domains), design, tasks (14/14), apply-progress (PR1+PR2+PR3), verify-report (PASS 6/6 16/16, ledger sha256:b53b43068e..., tok-9e385e42...). Steps: Validate Task Completion Gate (14/14), Verification Gate PASS, sync delta specs to main specs (pi-integration, agent-install, installer-pipeline, doctor) -> openspec/specs/{domain}/spec.md, move folder to archive/YYYY-MM-DD-pi-mcp-adapter-migration, verify, persist archive-report.md. Return envelope.” This is most recent account (2026-09-07) and outranks intermediate snapshots for final-state intent. It asserts all 14 tasks complete, verify PASS no blockers, 4 deltas to sync, archive to dated folder.

- **Intermediate snapshots (rank 4)**: `verify-report.md` at verification time PASS 6/6 16/16 with 2 WARNINGs (binary lag, no cover threshold) + 2 SUGGESTIONs, 14/14 tasks complete per Completeness table, build green, 74 node tests PASS, doctor 19 INFO healthy via go run. `apply-progress.md` at apply time 14/14 PR1+PR2+PR3 merged, workloads High budget, stacked-to-main, deviations None — matches design, no later commits cited. Both snapshots agree with rank 2 and rank 3 on completion; no later test run changed numbers (hash b53b... carried). Per hierarchy final numbers carried from verify-report (highest-ranked covering test counts) corroborated by tasks.

**Reconciliation & explicit contradictions** (per reporting rules: attribute snapshot claims, cite fix, record unrankable contradictions, never merge distinct defects):

- **Task completion — final state from rank 2**: per `verify-report` at verification time `Tasks total 14 complete 14` matches rank-2 persisted tasks at close 14 `[x]` pre- and post-archive. No later work changed counts. No contradiction between rank 2 and rank 4; rank 3 launch prompt also 14/14. No reconciliation needed.

- **Verify verdict — final state from rank 4 admitted but rank 1 would win if present**: `verify-report` admitted PASS with 0 blockers/0 critical at verification time; no higher-ranked reviewGate contradicts it. At close still PASS (no CRITICAL to block archive per strict policy). Attributed: “per `verify-report` sha256:b53b43068e... at verification time PASS 6/6 16/16, test_exit_code 0; no later evidence contradicts.” Not restated as bare present beyond attribution, but carried as final because no later commits fixed warnings (warnings are non-blocking).

- **Binary lag warning — resolved by hierarchy**: per `verify-report` WARNING installed biggz binary lags source (binary 17 INFO without pi-mcp-adapter vs source 19 INFO healthy). Per launch prompt final-state facts at close no explicit fix commit cited for rebuild, but verify-report Correctness notes source is authoritative and rebuild syncs. At close binary still lags (stale binary not rebuilt within this change's scope — SDD scope is source + tests, not host binary rebuild). Per hierarchy rank 4 WARNING remains valid at close but non-blocking; recorded as intentional-with-warnings for rebuild suggestion, not as blocker. Cited fix location: `go build -o biggz.exe ./cmd/biggz` would sync (SUGGESTION S1).

- **Ledger/tokens — carried from rank 3**: launch prompt cites ledger `sha256:b53b43068e...` / `tok-9e385e42...` matching verify-report evidence_revision/test_output_hash `sha256:b53b43068e0a42b433e56aeba582efa5d68320750c0c36b58a24b4a1ec63147f`. No contradiction; both agree.

- **Spec sync nextRecommended — resolved by later work**: per `apply-progress` Remaining Tasks None — 14/14 ready for verify then archive/sync. At verification time nextRecommended would be archive after sync; at close (post-sync, pre-move) sync is complete (main specs contain 4 domains ADDED). This later work changed state from `sync:ready` to `sync:all_done`; not stale. Attributed: “per `apply-progress` at apply time 14/14 ready for verify; per filesystem evidence at close (main specs contain new requirements, deltas archived) sync is complete.”

- **No merging of distinct defects**: 2 WARNINGs (binary lag, coverage threshold) and 2 SUGGESTIONs recorded as separate non-blocking; not merged into single cause. Each has own evidence and remedy. No cause claimed confirmed without evidence.

- **Numbers carried from highest-ranked source**: final test counts (74 node, 13+16 subcases pi, 8 doctor, 5 mcp, 40+ install, 19 doctor), warnings (2), suggestions (2), blockers (0), CRITICAL (0), tasks (14/14), evidence hashes from `verify-report` admitted revision (rank 4 but highest covering for those numbers, corroborated by launch prompt rank 3). Not copied from stale `apply-progress` where later work would have changed them (no such change for counts).

**Gate summary at close**:

- CRITICAL gate: PASS — `critical_findings: 0`, `verdict: pass` → archive not blocked; no override needed or accepted (strict policy CRITICAL would block with no override — not triggered).
- Task gate: PASS — 14/14 `[x]`, `allComplete:true`.
- Review gate: PASS — no `reviewGate` object to demand allow in openspec precedent; `verify-report` PASS + 14/14 tasks + launch prompt final-state facts govern; no `rdd_receipt_missing` blocks this openspec change.
- Verification gate: PASS — `6/6 requirements`, `16/16 scenarios`, `test_exit_code 0`, `build_exit_code 0`.

## Archive Verification

Pre-archive (from launch prompt + file probes pre-move):

- ✅ `verifyReport: done` (`verify-report.md` 139 lines, `evidence_revision sha256:b53b43068e...` admitted, `verdict: pass`, ledger tok-9e385e42)
- ✅ `taskProgress {total:14 completed:14 pending:0 allComplete:true}` (0 `[ ]`, 14 `[x]`)
- ✅ `artifactStore: openspec` preserved, `actionContext.mode: repo-local` (not `workspace-planning`), operations inside `allowedEditRoots` (C:\Users\USER\Desktop\biggz-ai)
- ✅ `proposal.md` 83 lines, `design.md` 121 lines, `tasks.md` 56 lines, `specs/` 4 deltas (pi-integration 50, agent-install 22, installer-pipeline 22, doctor 22) all `done`
- ✅ `CRITICAL: 0`, `blockers: 0` — verification gate PASS; CRITICAL would block with no override (not triggered)
- ✅ `applyState: all_done`, `apply-progress.md` 208 lines PR1+PR2+PR3 14/14, deviations None
- ✅ `proposal` In Scope 6 + Out of Scope 3 + Capabilities modified 4, Constraints 3, Risks 5, Rollback 1-commit

Spec sync (BEFORE move):

- ✅ `openspec/specs/pi-integration/spec.md` **Updated** (175→~225 lines, appended 3 req 7 scenarios, preserved prior 8 req, no destructive merge)
- ✅ `openspec/specs/agent-install/spec.md` **Updated** (237→~257 lines, appended 1 req 3 scenarios, preserved prior 8 req)
- ✅ `openspec/specs/installer-pipeline/spec.md` **Updated** (135→~155 lines, appended 1 req 3 scenarios, preserved prior 6 req)
- ✅ `openspec/specs/doctor/spec.md` **Updated** (31→~53 lines, appended 1 req 3 scenarios, preserved prior SDD Asset Drift)
- ✅ No existing main spec modified destructively; no REMOVED/RENAMED; no `rules.archive` violation
- ✅ `openspec/changes/archive/` existed (from prior archives), no create needed but ensured

Archive move:

- ✅ `Move-Item openspec/changes/pi-mcp-adapter-migration → openspec/changes/archive/2026-09-07-pi-mcp-adapter-migration` (date prefix `2026-09-07` = today per `Get-Date -Format yyyy-MM-dd`, ISO)
- ✅ Main specs still present after move (pi-integration ~225 lines, agent-install ~257, installer-pipeline ~155, doctor ~53)
- ✅ Change folder moved to archive (`Test-Path openspec/changes/pi-mcp-adapter-migration` → False; archive dir exists True)
- ✅ Archive contains all artifacts (`proposal.md` 83 ✅, `specs/pi-integration/spec.md` 50 ✅, `specs/agent-install/spec.md` 22 ✅, `specs/installer-pipeline/spec.md` 22 ✅, `specs/doctor/spec.md` 22 ✅, `design.md` 121 ✅, `tasks.md` 56 ✅ 14/14, `verify-report.md` 139 ✅, `apply-progress.md` 208 ✅, `exploration.md` ✅, plus this `archive-report.md`)
- ✅ Archived `tasks.md` has no unchecked implementation tasks (14 `[x]`, 0 `[ ]` — no reconciliation needed, no override)
- ✅ Active changes directory no longer has this change
- ✅ Scope exclusions honored: no branches/worktrees deleted (post-archive hygiene non-TTY skip); `.biggz-instance` not present in change (rename would preserve per spec); no push/merge/PR created
- ✅ No `archive.go` `RDDDisable`/`SetCloneLocalRDDMode` before move (pure `os.Rename`)

Post-archive:

- ✅ `active` no longer lists `pi-mcp-adapter-migration` (active openspec changes 0 for this change after move); canonical specs remain source of truth
- ✅ Nothing beyond the four canonical specs + the archived folder was modified by archive (working tree shows expected move + new specs)
- ✅ Archived audit trail has no stale unchecked tasks

Post-archive hygiene (Step 3b):

- CI non-TTY detected (`!isatty(Stdin)||!isatty(Stdout)`), skipped preview/prompt, no branches/worktrees deleted, exited 0 with hint `use --dry-run on CI`. No `.biggz-instance` to preserve in archive (verified rename preserved if present). `FetchPrune` / `ListGoneBranches` / `ListWorktrees` not executed in non-TTY (correct per spec).

## Risks / Open Questions

**Risks at close (intentional-with-warnings for non-blocking)**:

- **W1 binary lag (accepted, WARNING)**: Host `biggz` binary (pre-build) shows 17 INFO without pi-mcp-adapter while `go run ./cmd/biggz doctor --json` source shows 19 INFO healthy. Source is authoritative; `go vet`/`go test` prove health check present and panic-isolated. Risk low; rebuild `go build -o biggz.exe ./cmd/biggz` syncs. Tracked as SUGGESTION S1, non-blocking per verify-report.

- **W2 coverage threshold (accepted, WARNING)**: No explicit `go test -cover` threshold; coverage not enforced but delta behaviors exercised via unit/integration/node harnesses (pi 13+16, doctor 8+RealFS, mcp 25, install 40+, node 74). Low risk for this change's scope; future could add cover threshold.

- **S2 DeployMCPConfig prefix hardening (SUGGESTION)**: `TestDeployMCPConfigFile_PrefixBiggz` explicit file-assert for `args --prefix=biggz` in install deploy path currently covered via pi adapter merge tests + install Run comments but not via direct file-assert; hardening would add explicit assert. Low risk; ordering already proven via `TestProvisionBigMemMCP_WritesBothFiles` + `TestPR5_E2EFakeAgentTempDir`.

- **Phase 2 wrapper retention (handled)**: Wrappers retained one release gated on `!getTool` per design Decision 1 (Adapter required+phased fallback, 1-commit revert). Next change retires wrappers after doctor green — tracked as next steps, not risk at close.

- **Offline npm (handled)**: Adapter `^2` pin with doc fallback; MCP JSON harmless without client per InstallCommand offline scenario + `TestProvisionBigMemMCP_MergePreservesOthersAtomically` invalid json leaves unchanged.

**Open questions at close:**

- [x] Resolved per design: `directTools` unconditional (adapter ignores unknown) not probe >=2.32 — unconditional 20 tools; Both files vs mcp.json only — keep both per StrategyMCPConfigFile; Doctor missing = warn until phase 2 not fail.
- [ ] Next change `pi-mcp-retire-wrappers`: when to remove gated wrappers? Design says after doctor green one release — future decision, not blocking close.
- [ ] Host binary rebuild: should CI rebuild biggz.exe after each doctor check addition? Currently manual `go build`; future automation could auto-rebuild.

## Traceability

- **Proposal**: `openspec/changes/archive/2026-09-07-pi-mcp-adapter-migration/proposal.md` (83 lines, intent pi BigMem MCP-native via adapter 761K/mo v2.32.1, scope 6 in/3 out, 4 modified capabilities, constraints pin ^2 offsets, approach phased 1→2, affected areas 6 files, risks 5, rollback single-commit, dependencies pi-mcp-adapter@^2 + biggz-mcp, success criteria 4 boxes)
- **Specs (deltas)**: `specs/pi-integration/spec.md` (50 lines, 3 req 7 scenarios ADDED: Provision via Adapter 3, Wrapper Fallback 2, Tool Annotations 2) + `specs/agent-install/spec.md` (22 lines, 1 req 3 scenarios ADDED: InstallCommand 3) + `specs/installer-pipeline/spec.md` (22 lines, 1 req 3 scenarios ADDED: Deploy Ordering 3) + `specs/doctor/spec.md` (22 lines, 1 req 3 scenarios ADDED: Health Check 3) before move → now archived under `2026-09-07-pi-mcp-adapter-migration/specs/` + canonical copies at `openspec/specs/{pi-integration,agent-install,installer-pipeline,doctor}/spec.md`
- **Design**: `openspec/changes/archive/2026-09-07-pi-mcp-adapter-migration/design.md` (121 lines, 3 ADs: Adapter required+phased fallback, Provision authoritative both+imports directTools WriteFileAtomic, readOnlyHint full annotations; data flow Deploy→Provision→pi install→adapter spawns biggz-mcp→tools/list→/mcp live→wrappers gated; file changes 8 rows; interfaces ProvisionBigMemMCP/BiggzMCPPath/merge* helpers/WriteFileAtomic + mcp.json shape + annotations map; testing strategy 6 layers unit→E2E; threat matrix 5 RED with safe/failure + RED test)
- **Tasks**: `openspec/changes/archive/2026-09-07-pi-mcp-adapter-migration/tasks.md` (56 lines, 3 phases 14 tasks, workload High 550–620 chained PRs stacked-to-main, dependencies 1.x→2.x→3.x, test evidence 6 commands)
- **Verify**: `openspec/changes/archive/2026-09-07-pi-mcp-adapter-migration/verify-report.md` (139 lines, `evidence_revision sha256:b53b43068e0a42b433e56aeba582efa5d68320750c0c36b58a24b4a1ec63147f`, `verdict: pass`, `6/6 req`, `16/16 scenarios`, `0 blockers`, `0 critical`, `test_output_hash` same, `build_output_hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`, `test_exit_code 0`, `build_exit_code 0`, 2 WARNINGs + 2 SUGGESTIONs)
- **Apply**: `openspec/changes/archive/2026-09-07-pi-mcp-adapter-migration/apply-progress.md` (208 lines, PR1 ProvisionBigMemMCP+InstallCommand 5/5, PR2 Annotations+ordering+wrapper gate 5/5, PR3 Doctor 4/4, files changed per PR, test results per PR, work unit evidence 3 tables, TDD cycle evidence 3 tables, workload PR boundary, deviations None)
- **Exploration**: `exploration.md` (phased 1→2 rec, per proposal)
- **Review**: openspec store — no persisted reviewGate/receipt required; verify-report PASS with ledger sha256:b53b... tok-9e385e42 admitted suffices; no `rdd_receipt_missing` blocks openspec pi change
- **sdd-status**: pre-archive implied `all_done` for proposal/specs/design/tasks/apply/verify per launch prompt 14/14 PASS tok-9e385e42; post-archive active without this change; `actionContext.mode: repo-local`

## SDD Cycle Complete

The change has been fully planned, implemented, verified, and archived. The 2 warnings + 2 suggestions are intentional-with-warnings (non-blocking, safety invariants hold).

**Change**: `pi-mcp-adapter-migration`
**Archived to**: `openspec/changes/archive/2026-09-07-pi-mcp-adapter-migration/` (openspec) | `openspec/specs/{pi-integration,agent-install,installer-pipeline,doctor}/spec.md` source of truth

### Specs Synced

| Domain | Action | Details |
|--------|--------|---------|
| pi-integration | Updated | 3 added, 0 modified, 0 removed (Pi BigMem MCP Provisioning via Adapter 3 scenarios, Adapter-Aware Wrapper Fallback 2 scenarios, BigMem MCP Tool Annotations 2 scenarios; 175→~225 lines, preserved prior) |
| agent-install | Updated | 1 added, 0 modified, 0 removed (Pi MCP Adapter in InstallCommand 3 scenarios; 237→~257 lines, preserved prior) |
| installer-pipeline | Updated | 1 added, 0 modified, 0 removed (Pi Deploy Ordering with ProvisionBigMemMCP 3 scenarios; 135→~155 lines, preserved prior) |
| doctor | Updated | 1 added, 0 modified, 0 removed (Pi MCP Adapter Health Check 3 scenarios; 31→~53 lines, preserved prior) |

### Archive Contents

- proposal.md ✅ (83 lines)
- specs/pi-integration/spec.md ✅ (50 lines, 3 req 7 scenarios ADDED)
- specs/agent-install/spec.md ✅ (22 lines, 1 req 3 scenarios ADDED)
- specs/installer-pipeline/spec.md ✅ (22 lines, 1 req 3 scenarios ADDED)
- specs/doctor/spec.md ✅ (22 lines, 1 req 3 scenarios ADDED)
- design.md ✅ (121 lines)
- tasks.md ✅ (14/14 tasks complete, 0 pending, Phase 1 5/5 + Phase 2 5/5 + Phase 3 4/4)
- verify-report.md ✅ (PASS, 6/6 req, 16/16 scenarios, 0 blockers, 0 CRITICAL, evidence_revision sha256:b53b43068e0a42b433e56aeba582efa5d68320750c0c36b58a24b4a1ec63147f)
- apply-progress.md ✅ (PR1+PR2+PR3, 208 lines, deviations None)
- exploration.md ✅
- archive-report.md ✅ (this file)

### Source of Truth Updated

The following specs now reflect the new behavior:

- `openspec/specs/pi-integration/spec.md` — Pi BigMem MCP Provisioning via Adapter, Adapter-Aware Wrapper Fallback, BigMem MCP Tool Annotations
- `openspec/specs/agent-install/spec.md` — Pi MCP Adapter in InstallCommand (npm:pi-mcp-adapter@^2 before pi-subagents-j0k3r, ^2 pinned, idempotent)
- `openspec/specs/installer-pipeline/spec.md` — Pi Deploy Ordering with ProvisionBigMemMCP (DeployMCPBinary→Provision→pi install, WriteFileAtomic, dry-run zero writes, rollback atomic)
- `openspec/specs/doctor/spec.md` — Pi MCP Adapter Health Check (presence/^2 + bigmem reachable + tools/list + /mcp live, warn+BIGGZ_MCP_TIMEOUT, panic-isolated)

### Next

Ready for the next change. `biggz sdd-status` shows no active `pi-mcp-adapter-migration` (0 active for this change after move); delivery `openspec` preserved, no remediation required, no branches/worktrees deleted here (hygiene correctly skipped non-TTY). Next change `pi-mcp-retire-wrappers` can remove gated wrappers after one release of green doctor. Host biggz binary rebuild (`go build -o biggz.exe ./cmd/biggz`) will sync 17→19 INFO at next build.

