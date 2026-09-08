## Exploration: pi-wrapper-removal

### Current State
- `internal/assets/pi/biggz-memory-chrome.js` (~306 LOC): pure UI chrome for BigMem MCP tools. `TOOL_LABELS` + `humanToolName` + `compactToolArg` + `compactResultStatus` + `renderCallText`/`renderResultText` + `framedBlock`/`renderStatusLine` turn collapsed `biggz_mem_save` + hidden JSON into `🧠 save "title" → ✓ saved #id`. No DB semantics. Factory `export default function biggzMemoryChrome(pi)` with `PI_SUBAGENT_CHILD=1` bypass and adapter-aware gate: `if (pi.getTool("biggz_mem_save")) return` — present→no-op, absent→fallback render. Verifier string `!pi.getTool("biggz_mem_save")`.
- `internal/assets/pi/biggz-synthesis-gate.js` (~1170 LOC, 53KB): Pi-harness fail-closed checkpoint. Wraps `ask_user_choice` / `ask_user_question` / `question` via `pi.registerTool` interception + pre-registered sweep + `pi.on("tool_call")` secondary guard (`{block:true}`) + `tool_execution_end` reset. Strict same-turn `currentTurnMarkdown` (via `message_end`/`message_update` buffer, `turn_start` reset) must contain 4 markers; `ctx.history` advise-only. `IsCheckpointAsk` bilingual label/value/id/name/title scan only; `HasOptions` alone never blocks. `BIGGZ_ADVISE=1` thin concern only, `PI_SUBAGENT_CHILD=1` full bypass. Same adapter-aware fallback gate as chrome (`!pi.getTool("biggz_mem_save")`).
- Loader: `internal/install/steps/pi_extensions.go` `piExtensionsDeployList()` (12 JS + 3 TS) → `~/.pi/agent/extensions/` via `Apply()` + `validatePiExtensionsFactory` (`export default function` required or pi crashes). Self-heals stale `biggz-pi-pretty.js` / `gentle-ai.ts` / `quiet-tools.ts` / `sdd-init.ts`. Mirror test `internal/assets/pi/biggz-pi-extensions-factory.test.mjs` asserts deploy list + factory shape.
- Native path: `internal/agents/pi/adapter.go` `InstallCommand` → `pi install npm:pi-mcp-adapter@^2`; `ProvisionBigMemMCP` atomically merges `mcpServers.bigmem` (`command=BiggzMCPPath()`, `args=["--tools=agent","--prefix=biggz"]`, `type="local"`) + `imports:["opencode"]` + `directTools` (20 `biggz_mem_*`) into BOTH `~/.pi/agent/settings.json` and `mcp.json` via `filemerge.WriteFileAtomic`. `internal/doctor/pi_mcp_adapter.go` `PiMCPAdapterCheck`: adapter dir present, `package.json` version `^2`, `mcpServers.bigmem` valid, `biggz-mcp --help` probe OK. Spec contract: `openspec/specs/pi-integration/spec.md` `Adapter-Aware Wrapper Fallback` + `Pi BigMem MCP Provisioning`.
- Constraints to preserve: Go `internal/sdd/synthesis_gate.go` `ShouldBlock`/`HasSynthesis`/`IsCheckpointAsk` canonical (REQ-DG-1, 120s window, 4 English markers, Session Recall bypass, `ShouldBlockApplyAdmission` without child bypass); `internal/assets/pi/biggz-session-guard.js` factory (`export default function(pi)` + `checkSessionStop` shared with `biggz-extension-api.js`, 1000ms timeout, exit-1 + `blocked(session_summary_missing)` token only, `BIGGZ_PRETTY=0`/`PI_SUBAGENT_CHILD=1` bypass) — pi crashes with `Extension does not export a valid factory function` if broken.

### Affected Areas
- `internal/assets/pi/biggz-memory-chrome.js` — delete candidate (pure UI, no-op when native present).
- `internal/assets/pi/biggz-synthesis-gate.js` — delete candidate (only Pi in-harness gate; Go gate does not wrap Pi asks).
- `internal/install/steps/pi_extensions.go` — remove 2 entries from `piExtensionsDeployList()`, add stale self-heal `os.Remove`, update PR2 comment.
- `internal/assets/pi/biggz-pi-extensions-factory.test.mjs` — drop 2 entries, update `jsCount` guard.
- `internal/assets/pi/biggz-synthesis-gate.test.mjs` — delete or repurpose; CI references it.
- `openspec/specs/pi-integration/spec.md` — retire `Adapter-Aware Wrapper Fallback` requirement, keep provisioning/annotations.
- `internal/sdd/synthesis_gate.go` — NOT touched (canonical, must stay).
- `internal/assets/pi/biggz-session-guard.js` + `biggz-extension-api.js` + `biggz-tool-interception.js` — NOT touched (factory + session-close guard intact).
- `internal/doctor/pi_mcp_adapter.go` + `internal/agents/pi/adapter.go` — stability evidence source, no change expected.
- `docs/architecture.md`, `docs/validation-guide.md`, `CHANGELOG.md` — doc/CI references to gate JS need update.
- Deployed `~/.pi/agent/extensions/biggz-{memory-chrome,synthesis-gate}.js` — stale copies need self-heal removal on upgrade.

### Approaches
1. **Hard removal in one change** — delete both JS assets + deploy entries + tests + spec in a single commit.
   - Pros: clean, no dead code, single review.
   - Cons: no soak; if native flaps, no fallback; harder to bisect pretty vs gate regressions.
   - Effort: Medium
2. **Phased removal (deploy-list exclusion first, sources one release later)** — step 1: drop from deploy list + self-heal stale + spec/test updates, keep source files; step 2 (next release): delete sources after stability soak.
   - Pros: revertible in one line; stale cleanup verified; fallback still available via manual copy; matches existing one-release-fallback intent.
   - Cons: temporary dead sources; two reviews.
   - Effort: Medium
3. **Keep thin shims permanently** — replace bodies with documented no-op factories that only warn when native absent.
   - Pros: zero risk of pi loader crash; explicit fallback message.
   - Cons: permanent maintenance; contradicts removal goal; spec must keep fallback requirement.
   - Effort: Low

### Recommendation
Option 2 (phased). Wrappers are already no-ops when `pi.getTool("biggz_mem_save")` is truthy, so step 1 only makes the no-op permanent for fresh installs while proving native stability over one release. Gate removal on stability criteria below, with Go gate + session-guard untouched. Delete sources only after soak passes.

Native stability criteria (ALL must pass before source deletion):
- `biggz doctor` → `pi-mcp-adapter` PASS (not warn): adapter dir found, version `2.x`, `mcpServers.bigmem` valid in `settings.json`+`mcp.json`, `biggz-mcp --help` exit 0.
- `pi list` / `/mcp` live shows native `biggz_mem_*` (at least `biggz_mem_save`); inside Pi `pi.getTool("biggz_mem_save")` truthy (the exact gate condition).
- `settings.json`+`mcp.json` contain `command=<BiggzMCPPath>`, `args=["--tools=agent","--prefix=biggz"]`, `type="local"`, `imports` includes `opencode`, `directTools` includes 20 `biggz_mem_*`.
- `go vet ./...` + `go test ./...` + `node --check` on remaining extensions green after deploy-list edit.
- Soak: one release with wrappers deployed-but-no-op and zero fallback-firing reports (no pill duplication, no re-block).

### Risks
- Fail-closed hole: removing JS gate leaves Pi `ask_user_choice` unwrapped; Go `ShouldBlock` does not run inside Pi harness — checkpoint bypassable until native gate replacement exists.
- Pretty loss: collapsed lines revert to `biggz_mem_save` + hidden JSON; acceptable but user-visible.
- Stale deployed copies: without `os.Remove` self-heal, old `~/.pi/agent/extensions/` copies keep enforcing/duplicating after removal.
- Factory/test drift: forgetting factory test or `validatePiExtensionsFactory` sync breaks CI or crashes pi.
- Version drift: `pi-mcp-adapter` major bump off `^2` or `biggz-mcp` binary missing re-triggers the exact conditions wrappers papered over.

### Ready for Proposal
Yes — propose phased removal with stability gate above, explicit non-goals (Go gate + session-guard untouched), and rollback inputs below. Orchestrator should persist this file verbatim to `openspec/changes/pi-wrapper-removal/exploration.md` (no write tool in this runtime) then run read-only `biggz doctor` + `pi list` evidence before proposing.
Rollback inputs: single-commit revert restores deploy list + assets; `biggz install --agent pi` re-deploys; `doctor PiMCPAdapterCheck` Remedy (`pi install npm:pi-mcp-adapter@^2`) re-provisions.
