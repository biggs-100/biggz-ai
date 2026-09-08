## Exploration: pi-footprint-slim

### Current State
- `internal/agents/pi/adapter.go:397` `piDirectTools` hardcodes 20 tools (parity with `cmd/biggz-mcp` `ProfileAgent` 20); `mergePiMCPFileBigMem` (`:384-386`) unconditionally writes full 20 into `~/.pi/agent/mcp.json` `directTools` via `mergePiDirectTools` (`:450+`, additive/union merge — stale entries never pruned). Every `biggz install --agent pi` re-promotes all 20 to top-level, bloating model function surface → 429 pressure.
- APPEND_SYSTEM generation is dual-path: legacy `internal/install/install.go:DeployPersona` (`:758`, persona + `biggz-orchestrator.md` + `web-tools.md`) and `DeployBigMemProtocol` (`:838`, `bigmem-protocol.md`), mirrored by `internal/install/steps/overlay.go:deployPersona` (`:277`) + `deployBigMemProtocol` (`:316`). Pi target = `~/.pi/agent/APPEND_SYSTEM.md` (`adapter.go:25,177-180`). Protocol/orchestrator assets (`internal/assets/biggz/`) carry repeated REMINDER blocks (~5x) + verbose examples.
- Deployed proof (`~/.pi/agent/mcp.json` 10 tools + trimmed APPEND_SYSTEM) is manual-only; reinstall regenerates the fat 20 + verbose prompt.

### Affected Areas
- `internal/agents/pi/adapter.go` — `piDirectTools` var (`:397-419`) + `mergePiDirectTools` (`:450`) — trim to 10, decide prune-vs-union semantics
- `internal/agents/pi/adapter_test.go` — `TestProvisionBigMemMCP_*` (`:162,286,385`) assert shape/merge/idempotent; mirror tests need 10-tool update
- `cmd/biggz-mcp/main_test.go:241` — asserts agent profile = 20 tools (server-side, NOT directTools; keep or decouple)
- `internal/assets/biggz/biggz-orchestrator.md`, `bigmem-protocol.md`, `biggz-persona.md`, `web-tools.md` — trim targets (REMINDER dedupe, example trim)
- `internal/install/install.go:DeployPersona/DeployBigMemProtocol` + `internal/install/steps/overlay.go:deployPersona/deployBigMemProtocol` — generation path (no logic change needed if only asset text trimmed)
- `internal/install/pi_subagents_test.go:143-156` — asserts `biggz:bigmem-protocol` marker present (load-bearing marker, keep)
- `openspec/specs/pi-integration/spec.md:178,183,231` — spec references directTools shape (update if it pins count 20)

### Approaches
1. **Slim list + prune-on-merge + asset trim (Recommended)** — Replace `piDirectTools` with the proven 10 (`save, search, get_observation, context, session_summary, save_prompt, update, timeline, review, judge`); make merge authoritative (drop the 10 removed even if present); trim assets (REMINDER x5→x1, cut verbose examples, keep all `<!-- biggz:* -->` markers + gate template verbatim); update mirror tests + spec; reinstall + verify.
   - Pros: reinstall-preserving; 429 relief (halved function surface); prune prevents zombie tools on existing installs
   - Cons: `mergePiDirectTools` behavior change needs a test; dropping 10 tools could break flows that call them (mitigate: server still exposes 20 via `--tools=agent`, only top-level promotion shrinks)
   - Effort: Medium
2. **Slim list only, keep union merge + no asset trim** — Change `piDirectTools` to 10, leave merge additive, skip prompt trim.
   - Pros: smallest diff, lowest risk
   - Cons: existing installs keep stale 10 (no relief until manual wipe); APPEND_SYSTEM bloat untouched; reinstall does not converge to proven state
   - Effort: Low

### Recommendation
Approach 1. It is the only option that converges any install (fresh or existing) to the proven deployed state on reinstall. Keep server `ProfileAgent` at 20 (no `cmd/biggz-mcp` change) — trim is promotion-only, so rollback = revert list + reinstall.

### Risks
- Dropped 10 tools may be referenced by prompts/skills → grep before cutting; keep server-side available.
- Gate markers (`## Sub-agent Result`, 4 `**...**` markers, `<!-- biggz:orchestrator/persona/bigmem-protocol/web-tools -->`) are load-bearing — trim prose/examples only, never markers/template tokens (`{{BIGGZ_BACKGROUND_POLICY}}`).
- `mergePiDirectTools` prune semantics change: existing test expects foreign entries preserved — must preserve foreign entries while pruning only the 10 removed BigMem ones (allowlist-prune, not wipe).
- Spec `pi-integration` + archive docs pin "20 tools" in prose — update spec, leave archives untouched.

### Ready for Proposal
Yes — propose Approach 1 with the 10-tool allowlist from `_meta.yaml`, asset trim boundaries above, mirror-test updates, and reinstall+verify (`go test ./internal/agents/pi/... ./internal/install/... ./cmd/biggz-mcp/...`, `biggz install --agent pi`, `doctor pi-mcp-adapter`, diff `~/.pi/agent/mcp.json` directTools == 10 + APPEND_SYSTEM single REMINDER).
