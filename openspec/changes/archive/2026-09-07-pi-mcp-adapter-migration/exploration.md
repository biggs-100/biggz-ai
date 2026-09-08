# Exploration: pi-mcp-adapter-migration

## Current State
Pi in biggz-ai does NOT use native MCP like opencode does. Opencode registers `biggz-mcp.exe --tools=agent --prefix=biggz` in its MCP local config (`mcp` block in `opencode.json` via `StrategyMergeIntoSettings`) and exposes `biggz_mem_*` tools natively with no JS wrappers.

Pi today simulates BigMem and guardrails via 12 JS ExtensionAPI wrappers deployed by `internal/install/steps/pi_extensions.go` (`piExtensionsDeployList()`):
`biggz-thinking-wrap`, `biggz-memory-chrome`, `biggz-tool-interception`, `biggz-extension-api`, `biggz-session-guard`, `biggz-last-model`, `biggz-synthesis-gate`, `biggz-wait-pretty`, `biggz-footer`, `biggz-tool-pills`, `biggz-web-search`, `biggz-question-mouse` (+ 3 TS assets `ask-user-choice`, `codegraph-tools`, `skill-registry`).
`validatePiExtensionsFactory()` enforces `export default function(pi)` or pi crashes. `biggz-synthesis-gate.js` (~1130 LOC) enforces `IsCheckpointAsk` strict same-turn synthesis, `biggz-extension-api.js`/`biggz-tool-interception.js` provide status-line, pill streaming, SafeToolRenderer, and session-stop guards.

Separately, `internal/agents/pi/adapter.go` already provisions `biggz-mcp` via `ProvisionBigMemMCP()` → merges `mcpServers.bigmem { command: <BiggzMCPPath>, args: ["--tools=agent","--prefix=biggz"], type: "local" }` into BOTH `~/.pi/agent/settings.json` (`mcpServers`) and `~/.pi/agent/mcp.json` (`MCPConfigFile` strategy, atomic via `filemerge.WriteFileAtomic`). `BiggzMCPPath()` prefers `~/.biggz/biggz-mcp(.exe)` → `PATH` → exe dir. `internal/install/install.go:Run()` deploys the binary (`DeployMCPBinaryToHomeDir`) + `DeployMCPConfig` + `ProvisionBigMemMCP` for `AgentPi`, then ensures `pi-subagents-j0k3r` + 4 `pi install` packages (`pi-subagents-j0k3r`, `@juicesharp/rpiv-ask-user-question`, `rpiv-todo`, `pi-web-access`).

Comment in `adapter.go:InstallCommand` explicitly says biggz-ai does **not** need `pi-mcp-adapter` ("Biggz uses BigMem native Go") — this is the drift the migration would reverse. Research context: `pi-mcp-adapter` (nicobailon, 761k/mo, v2.32.1) is now the recommended Pi MCP client (stdio/streamable-http/sse, `tools/list` auto-discovery, live refresh, `{"imports":["opencode"]}`, `/mcp` UI, `directTools` promotion). Without it, the `mcpServers.bigmem` JSON written by `ProvisionBigMemMCP` has no client to spawn `biggz-mcp` — `biggz-memory-chrome.js` only renders compact labels (`🧠 save "title" → ✓ saved #id`) and status via `pi.on("tool_call"|"tool_result")`.

`cmd/biggz-mcp/main.go` is a stdio JSON-RPC server with `--prefix`/`--tools` flags, `ProfileAgent` (20 tools) / `ProfileAdmin` (3) / `all` (25), `toolPrefix` stripping, `initialize` → `tools/list` → `tools/call`, `writeQueue` serialization, `SessionActivity` nudges, blob externalization, and `buildToolList` tool definitions. Tools are MCP-native but lack explicit `readOnlyHint`/`openWorldHint` annotations — relevant for `pi-mcp-adapter` health/safety filtering.

## Affected Areas
- `internal/install/steps/pi_extensions.go` — deploy list, factory validation, self-heal stale extensions; add/remove `pi-mcp-adapter` from install and decide wrapper retirement.
- `internal/agents/pi/adapter.go` — `InstallCommand()` (add `npm:pi-mcp-adapter`), `ProvisionBigMemMCP()` / `mergePiSettingsBigMem()` / `mergePiMCPFileBigMem()` (imports + schema for adapter), `BiggzMCPPath()` (args prefix), `MCPConfigPath()` layering (global `~/.pi/agent/mcp.json` vs project `.mcp.json`).
- `internal/install/install.go` — `Run()` pipeline: binary deploy, `DeployMCPConfig` `MCPConfigFile` branch, `ProvisionBigMemMCP` call ordering vs `pi install` npm steps; `verifyOrchestratorDeployment` (MCP presence checks).
- `internal/assets/pi/*` — 12 JS wrappers: `biggz-synthesis-gate.js` (keep vs retire), `biggz-memory-chrome.js` (adapter already renders MCP tools), `biggz-extension-api.js`/`biggz-tool-interception.js`/`biggz-session-guard.js`; new/updated config asset for adapter if needed.
- `cmd/biggz-mcp/main.go` — `buildToolList()` tool definitions, `toolDef` inputSchema, `ResolveTools` profiles, annotations (`readOnlyHint` etc.), `serverInstructions`, reconnection idempotency.
- `internal/doctor/*` and `internal/uninstall/*` — health checks for `biggz-mcp` presence + MCP JSON cleanup should include adapter config.

## Approaches
1. **Add pi-mcp-adapter as required dependency (adapter-native)** — Add `npm:pi-mcp-adapter` to `Adapter.InstallCommand()` (before `pi-subagents-j0k3r`), keep `ProvisionBigMemMCP` generating `~/.pi/agent/mcp.json` with `mcpServers.bigmem { command, args:["--tools=agent","--prefix=biggz"], type:"local" }` plus optional `{"imports":["opencode"]}` fallback that reuses `opencode.json` biggz server. Enable `directTools` for `biggz_mem_*` if adapter supports it. Keep existing JS wrappers unchanged in phase 1.
   - Pros: Pi matches opencode native MCP; auto-discovery + `tools/list` live refresh; `/mcp` UI health; imports reduce duplication; 761k/mo signals stability; `directTools` promotes BigMem tools to top-level.
   - Cons: New npm dependency (version drift 2.32.1 → breaking config shape); config layering confusion (global `~/.pi/agent/mcp.json` vs project `.pi/mcp.json` vs `.mcp.json` imports — precedence must be documented); tool name sanitization (64-char max) could collide on prefixed names; wrappers now duplicate rendering.
   - Effort: Medium (2–3 files, install idempotency + tests).

2. **Adapter-optional with imports fallback (progressive migration)** — Do NOT add to `InstallCommand` yet; update `ProvisionBigMemMCP` to write BOTH shapes: `settings.json:mcpServers.bigmem` (current) and `mcp.json: { mcpServers: { bigmem: {...} }, imports: ["opencode"] }` when `~/.config/opencode/opencode.json` exists. Document manual `pi install npm:pi-mcp-adapter`. Make wrappers adapter-aware: `biggz-memory-chrome.js` and `biggz-synthesis-gate.js` detect `pi._mcp` / `pi.getTool("biggz_mem_save")` and degrade to no-op when MCP tools already present, keeping them as fallback if adapter absent.
   - Pros: Zero breaking change; users without adapter keep working; easy rollback; validates `imports` without forcing install; wrappers stay as safety net.
   - Cons: Two code paths to maintain; synthesis gate still JS-only (not unified with Go `synthesis_gate.go`); `biggz-mcp` health must be checked in both branches; does not yet achieve "native like opencode".
   - Effort: Low (config merge + 2 JS guards + docs).

3. **Full wrapper retirement + MCP-only (clean slate)** — Install `pi-mcp-adapter` and delete `biggz-memory-chrome.js`, `biggz-tool-interception.js`, `biggz-extension-api.js` chrome portions, and `biggz-synthesis-gate.js`'s blocking logic in favor of Go `internal/sdd/synthesis_gate.go:ShouldBlock` via MCP middleware (or leave gate as thin `pi.on("tool_call")` shim aggregating `biggz_mem_*` readOnly). Rely solely on `biggz-mcp` for BigMem.
   - Pros: Single source of truth; removes ~2k LOC of JS shims; aligns Pi 1:1 with opencode architecture.
   - Cons: High risk — synthesis gate currently enforces REQ-DG-1 strict same-turn (`currentTurnMarkdown` ≤120s) and advise mode (`BIGGZ_ADVISE`); MCP-only gate would need reimplementation and no `block:true` fallback if adapter load-order races; `biggz-memory-chrome`'s collapsed pill chrome lost until adapter's renderer replaces it; breaks users on Pi < version that ships MCP natively.
   - Effort: High (gate rewrite + extensive TUI parity tests).

## Recommendation
**Go with Approach 1 phased as 1→2**: Add `pi-mcp-adapter` to `InstallCommand` and make `ProvisionBigMemMCP` authoritative for the adapter's config schema, but **retain wrappers as fallback for one release** (Approach 2's adapter-aware guards). Rationale:
- Research and download volume (761k/mo vs 11k for `pi-mcp-extension`) makes `pi-mcp-adapter` the standard; `imports: ["opencode"]` is exactly the opencode↔pi parity the task asks for.
- Current `ProvisionBigMemMCP` already writes the correct shape (`command` + `args --tools=agent --prefix=biggz` + `type:local`) that `pi-mcp-adapter` expects for stdio; gap is only package installation and `directTools`/health wiring.
- Keeping wrappers one release de-risks `biggz-synthesis-gate.js` (1000+ LOC, load-order `pi.on("tool_call")` block) and `biggz-memory-chrome` pill/status rendering while `/mcp` UI and adapter logs validate stability. Retire them in a follow-up after `doctor`/`verify` signals green.

Concrete next steps for proposal: update `pi_extensions.go:InstallCommand` + `adapter.go:mergePiMCPFileBigMem` to emit `{"mcpServers":{"bigmem":{...}},"imports":["opencode"]}` when opencode config exists + `deployMCPConfig.go:MCPConfigFile` branch to include `--prefix=biggz`; add `readOnlyHint` annotations to `cmd/biggz-mcp:buildToolList`; add `doctor` check for adapter presence + `pi-mcp-adapter` version; keep `biggz-synthesis-gate.js` but gate on `!pi.getToolDefinition("biggz_mem_save")`.

## Risks
- **Version drift of pi-mcp-adapter**: v2.32.1 config shape (`mcpServers` + `imports` + `directTools`) may change; pin major and test `pi install` idempotency, add fallback to manual doc if `pi install` fails offline.
- **Config layering confusion**: global `~/.pi/agent/mcp.json` vs project `.pi/mcp.json` vs `~/.pi/agent/settings.json:mcpServers` — adapter merges with precedence; missing merge preserves user servers but `bigmem` must win; need atomic `WriteFileAtomic` + JSONC merge tests.
- **Tool name collisions**: adapter sanitizes tool names to 64 chars; `biggz_mem_*` (prefixed) is safe, but future `biggz_*` tools near limit could collide — keep names short and test `tools/list` round-trip.
- **Reconnection/health**: stdio `biggz-mcp` crashes → adapter must respawn; `biggz-mcp`'s 1 MiB scanner buffer and write queue could stall; add `doctor` + `session_guard` health checks and `BIGGZ_MCP_TIMEOUT` guidance.
- **Synthesis gate parity drift**: Go `ShouldBlock` is canonical; JS gate must mirror `IsCheckpointAsk` tokens and `checkSynthesisPrecondition` (≤120s, `currentTurnMarkdown` only) — adapter's `tool_call` interception layer could double-block; keep `PI_SUBAGENT_CHILD=1` bypass in both.

## Ready for Proposal
Yes — scope is bounded to `internal/agents/pi`, `internal/install`, `cmd/biggz-mcp`, and `internal/assets/pi`. Propose Approach 1 phased (install adapter + imports, wrappers as fallback, annotations + doctor), with explicit rollback (remove `pi-mcp-adapter` package, retain `ProvisionBigMemMCP` writes). No spec change beyond MCP config contract and tool annotations.

