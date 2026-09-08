# Design: pi-mcp-adapter-migration

## Technical Approach

Phased 1→2 Pi→MCP-native like opencode. `ProvisionBigMemMCP` writes `mcpServers.bigmem {command,args:--tools=agent --prefix=biggz,type:local}`+`imports:["opencode"]`+`directTools` to `settings.json`+`mcp.json` atomically. `InstallCommand` adds `npm:pi-mcp-adapter@^2` before `pi-subagents-j0k3r`. Wrappers gated `!getTool` one release. `buildToolList` adds `readOnlyHint` hints. Order `Deploy→Provision→pi install`. Doctor checks adapter/health.

## Architecture Decisions

### Decision 1: Adapter required + phased fallback

| Option | Tradeoff | Decision |
|---|---|---|
| Remove wrappers immediately | Single path; high risk (gate 1k LOC load-order race) | Reject |
| Adapter optional | No breakage; never reaches parity | Reject |
| Adapter required, wrappers gated one release | Validates via `/mcp` while safe fallback; 1-commit revert | **Chosen** |

**Rationale**: 761K/mo standard; `ProvisionBigMemMCP` already emits correct stdio; fallback de-risks gate.

### Decision 2: ProvisionBigMemMCP authoritative + `imports:["opencode"]`

| Option | Tradeoff | Decision |
|---|---|---|
| Only `settings.json` | Duplicates opencode `mcp.biggez` | Reject |
| Only `mcp.json` + imports | Fails without opencode installed | Reject |
| Both files + `imports` + `directTools` in `mcp.json` | `bigmem` wins in both layers; reuses opencode when present | **Chosen** |

**Rationale**: Both writes guarantee discovery; `WriteFileAtomic` preserves others, no partials.

### Decision 3: `readOnlyHint` annotations

| Option | Tradeoff | Decision |
|---|---|---|
| No hints | Adapter cannot filter destructive tools | Reject |
| Only `readOnlyHint` | Misses idempotency | Reject |
| Full `readOnly/destructive/idempotent/openWorld` | Enables adapter filtering + `/mcp` safety; MCP spec compliant | **Chosen** |

**Rationale**: `search`/`get` true, `save` false, `openWorld` false (SQLite). Hints drive `directTools`.

## Data Flow

```
install --agent pi → DeployMCPBinaryToHomeDir (~/.biggz/biggz-mcp)
  → ProvisionBigMemMCP ─┬─→ settings.json mcpServers.bigmem
                        └─→ mcp.json mcpServers.bigmem+imports+directTools (WriteFileAtomic)
  → pi install pi-mcp-adapter@^2 + pi-subagents-j0k3r (idempotent)
  → Pi runtime: adapter spawns biggz-mcp --tools=agent --prefix=biggz (stdio)
    → tools/list (annotated) → native biggz_mem_* → /mcp healthy
    → wrappers: if getTool(biggz_mem_save) → no-op else fallback
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/agents/pi/adapter.go` | Modify | `InstallCommand` adds `pi-mcp-adapter@^2` first; `merge*` emit `imports`+`directTools`+`--prefix=biggz` |
| `internal/install/steps/pi_extensions.go` | Modify | Keep wrappers; add `!getTool` gate |
| `internal/install/install.go` | Modify | Order `Deploy→Provision→pi install`; `DeployMCPConfig` adds `--prefix=biggz` |
| `cmd/biggz-mcp/main.go` | Modify | `buildToolList` adds annotations |
| `internal/assets/pi/biggz-memory-chrome.js` | Modify | Gate `if(getTool) return` |
| `internal/assets/pi/biggz-synthesis-gate.js` | Modify | Same gate; keep `PI_SUBAGENT_CHILD=1` |
| `internal/doctor/pi_mcp_adapter.go` | Create | `PiMCPAdapterCheck`: `^2` + `bigmem` + `tools/list`; `warn`+hint/`TIMEOUT`; isolated |
| `internal/doctor/pi.go` | Modify | Register check |

## Interfaces / Contracts

```go
func (a *Adapter) ProvisionBigMemMCP(homeDir string) (bool, []string, error)
func (a *Adapter) BiggzMCPPath() string // ~/.biggz/biggz-mcp(.exe) → PATH → exeDir → bare
func (a *Adapter) mergePiSettingsBigMem(path, mcpBinary string) (filemerge.WriteResult, error)
func (a *Adapter) mergePiMCPFileBigMem(path, mcpBinary string) (filemerge.WriteResult, error)
func readPiJSONObject(path string) (map[string]any, error)
func WriteFileAtomic(path string, content []byte, perm fs.FileMode) (WriteResult, error) // temp+rename, skip if equal
```
```json
// mcp.json (settings.json has only mcpServers.bigmem)
{"mcpServers":{"bigmem":{"command":"<path>","args":["--tools=agent","--prefix=biggz"],"type":"local"}},"imports":["opencode"],"directTools":["biggz_mem_save","biggz_mem_search"]}
```
```go
// cmd/biggz-mcp annotation per toolDef
annotations: {readOnlyHint:bool, destructiveHint:bool, idempotentHint:bool, openWorldHint:false}
```

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | Provision fresh/merge/idempotent | `go test ./internal/agents/pi` TempDir; assert args |
| Unit | `buildToolList` hints | `go test ./cmd/biggz-mcp` table |
| Unit | Ordering + dry-run + rollback | `go test ./internal/install` FailAfter; no partials |
| Integration | Wrapper `!getTool` gate | `node --test` mocked `getTool` |
| Integration | Doctor health/drift/crash | `go test ./internal/doctor` injected fns |
| E2E | Dry-run + TempDir → `/mcp` | Temp HOME + `go vet && go test && node --test` |

## Threat Matrix

| Boundary | Applicable | Reason |
|----------|------------|--------|
| Documentation-like paths | N/A | JSON config only |
| Git repository selection | N/A | Targets HOME not repo |
| Commit state | N/A | No VCS |
| Push state | N/A | No VCS |
| PR commands | N/A | No PR automation |

| Threat | Safe / Failure | RED test |
|--------|---------------|----------|
| Version drift `^2` | Pin `^2`; offline→doc fallback, JSON harmless | `InstallCommand` has `@^2`; doctor mismatch→`warn` |
| Config layering | `WriteFileAtomic` preserves `other`; partial→unchanged | Merge `other` preserved; failure keeps file |
| Name collisions | Keep names ≤64; truncate→unreachable | `buildToolList` len≤64; `tools/list` distinct |
| Reconnection | Respawns; 1MiB+queue; crash→`BIGGZ_MCP_TIMEOUT` | Kill→respawn; crash fixture→warn |
| Gate drift | Gate on `!getTool`; Go canonical | `!getTool`→no-op; 4 markers still enforced |

## Migration / Rollout

Phase 1: provision authoritative + adapter required + wrappers gated. Phase 2: retire wrappers after `doctor` green. Rollback: remove from `InstallCommand` + `pi uninstall`; JSON harmless. One-commit revert.

## Open Questions

- [ ] `directTools` unconditional or `>=2.32` probe? Unconditional (adapter ignores unknown).
- [ ] Both files vs `mcp.json` only? Keep both per `StrategyMCPConfigFile`.
- [ ] Doctor missing = `warn` or `fail`? `warn` until phase 2.

