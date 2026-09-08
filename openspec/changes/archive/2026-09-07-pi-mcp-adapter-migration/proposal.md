# Proposal: pi-mcp-adapter-migration

## Intent

Make Pi BigMem MCP-native like opencode. Today `ProvisionBigMemMCP` writes `mcpServers.bigmem` but without `pi-mcp-adapter` no stdio client spawns `biggz-mcp`; JS wrappers simulate it. Install adapter (761K/mo, v2.32.1) so `biggz_mem_*` are native Pi tools with `/mcp` health and `imports:["opencode"]` parity.

## Scope

### In Scope
- Add `npm:pi-mcp-adapter` to `InstallCommand` (before `pi-subagents-j0k3r`, idempotent).
- Emit `mcpServers.bigmem {command, args --tools=agent --prefix=biggz, type:local}` + `imports:["opencode"]` + `directTools` via `ProvisionBigMemMCP`/`mergePiMCPFileBigMem`.
- Ensure `internal/install` order: binary -> `ProvisionBigMemMCP` -> `pi install`.
- Keep wrappers one release; gate `biggz-memory-chrome`/`synthesis-gate` on `!pi.getTool("biggz_mem_save")`.
- Add `readOnlyHint` annotations in `cmd/biggz-mcp:buildToolList`.
- Doctor check for adapter presence/version + `biggz-mcp` health.

### Out of Scope
- Full wrapper retirement (next release).
- Immediate removal of wrappers.
- Changing opencode config.

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `pi-integration`: adapter-shaped MCP config (`imports`, `directTools`).
- `agent-install`: `InstallCommand` includes adapter.
- `installer-pipeline`: deploy ordering.
- `doctor`: adapter health.

## Constraints
- Pin adapter `^2` (v2.32.1 shape may drift); offline -> doc fallback.
- Preserve merge precedence (`mcp.json` vs `settings.json`) via `WriteFileAtomic`.
- Tool names <=64 chars after sanitize.

## Approach

Phased 1->2 (exploration rec). Phase 1: `ProvisionBigMemMCP` authoritative, adapter required, wrappers as fallback adapter-aware. Validate via `/mcp` UI + respawn. Phase 2 (next change): retire wrappers after doctor green.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/agents/pi/adapter.go` | Modified | `InstallCommand`, `ProvisionBigMemMCP` merges |
| `internal/install/steps/pi_extensions.go` | Modified | Deploy list, health wiring |
| `internal/install/install.go` | Modified | `Run` ordering, verify |
| `cmd/biggz-mcp/main.go` | Modified | `buildToolList` hints |
| `internal/doctor/*` | Modified | Adapter health check |
| `internal/assets/pi/*` | Modified | Adapter-aware guards |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Version drift | Med | Pin major, idempotency test |
| Config layering | Med | Atomic merge tests |
| Name collision 64-char | Low | Keep names short, `tools/list` test |
| Stdio crash/respawn | Med | Doctor + timeout guidance |
| Gate double-block | Med | `PI_SUBAGENT_CHILD` bypass |

## Rollback Plan

Remove `npm:pi-mcp-adapter` from `InstallCommand` + `pi uninstall`; `ProvisionBigMemMCP` writes harmless without client; wrappers still work. Single-commit revert.

## Dependencies

- `pi-mcp-adapter@^2` via npm at install time.
- `biggz-mcp` at `~/.biggz/biggz-mcp`; opencode config optional.

## Success Criteria

- [ ] `pi` loads `pi-mcp-adapter` without crash
- [ ] `biggz_mem_*` appear as native Pi tools (`tools/list`)
- [ ] `/mcp` shows `biggz` healthy + live refresh
- [ ] `biggz doctor` healthy; `go test`/`node --test` green

## Proposal question round

- Q1: `directTools` unconditional or probe `>=2.32`?
- Q2: Sync both `settings.json`+`mcp.json` or `mcp.json` only?
- Q3: Doctor missing adapter = `warn` or `fail`?
