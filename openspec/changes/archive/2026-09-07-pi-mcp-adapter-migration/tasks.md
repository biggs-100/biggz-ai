# Tasks: pi-mcp-adapter-migration

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 550–620 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1 → PR2 → PR3 stacked |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | ProvisionBigMemMCP + InstallCommand | PR1 | `go test ./internal/agents/pi -run TestProvision -count=1` | `biggz install --agent pi --dry-run` TempDir | Revert `internal/agents/pi/adapter.go` only |
| 2 | Annotations + ordering + wrapper gate | PR2 | `go test ./cmd/biggz-mcp -run TestBuildToolList && go test ./internal/install -run TestRun && node --test` | Temp HOME install verify Deploy→Provision→pi install | Revert `cmd/biggz-mcp/main.go`, `internal/install/*`, `internal/assets/pi/*.js` |
| 3 | Doctor health check | PR3 | `go test ./internal/doctor -run TestPiMCPAdapter -count=1` | `biggz doctor --json` + `/mcp` live | Delete `internal/doctor/pi_mcp_adapter.go` + unregister |

## Phase 1: Foundation — MCP Provisioning

- [x] 1.1 RED: Test `InstallCommand()` contains `npm:pi-mcp-adapter@^2` before `pi-subagents-j0k3r` in `internal/agents/pi/adapter_test.go` (version drift)
- [x] 1.2 Modify `internal/agents/pi/adapter.go` `InstallCommand()` to prepend `npm:pi-mcp-adapter@^2` pinned `^2`, idempotent
- [x] 1.3 RED: Test `mergePiMCPFileBigMem` preserves `mcpServers.other` and `WriteFileAtomic` leaves file unchanged on failure (config layering)
- [x] 1.4 Modify `internal/agents/pi/adapter.go` `mergePiSettingsBigMem`/`mergePiMCPFileBigMem` to emit `command=BiggzMCPPath()`, `args[--tools=agent,--prefix=biggz]`, `type:local`, `imports:["opencode"]`, `directTools` via `WriteFileAtomic`
- [x] 1.5 Verify `go test ./internal/agents/pi -run TestProvision` fresh/merge/idempotent (pi-integration spec)

## Phase 2: Core — Annotations, Ordering, Fallback

- [x] 2.1 RED: Test `buildToolList` in `cmd/biggz-mcp/main_test.go` has `len<=64`, distinct `tools/list`, correct hints (name collision)
- [x] 2.2 Modify `cmd/biggz-mcp/main.go` `buildToolList` to set `readOnlyHint/destructiveHint/idempotentHint/openWorldHint:false` (search/get true, save false)
- [x] 2.3 RED: `node --test` wrapper gate on `!pi.getTool("biggz_mem_save")` present→no-op, absent→fallback, `PI_SUBAGENT_CHILD=1` bypass (gate drift + reconnection)
- [x] 2.4 Modify `internal/assets/pi/biggz-memory-chrome.js` + `biggz-synthesis-gate.js` to gate on `!pi.getTool("biggz_mem_save")`
- [x] 2.5 Modify `internal/install/install.go` `Run` + `internal/install/steps/pi_extensions.go` to order `DeployMCPBinary→ProvisionBigMemMCP→pi install` and add `--prefix=biggz`

## Phase 3: Integration — Doctor & Verification

- [x] 3.1 RED: Test `PiMCPAdapterCheck` in `internal/doctor/pi_mcp_adapter_test.go` missing→warn, `3.0.0`→warn, crash→warn+`BIGGZ_MCP_TIMEOUT` (version drift + reconnection)
- [x] 3.2 Create `internal/doctor/pi_mcp_adapter.go` `PiMCPAdapterCheck` (presence/^2 + bigmem reachable + `tools/list` + `/mcp` live) and register in `internal/doctor/pi.go`
- [x] 3.3 Verify `go test ./internal/install -run TestRun` dry-run zero writes outside TempDir and rollback reverse-order no partials (installer-pipeline spec)
- [x] 3.4 E2E Temp HOME dry-run→apply→`/mcp` healthy + `go vet && go test ./... && node --test` green

## Dependencies

1.x → 2.x → 3.x; 1.4 → 2.5; 2.4 → 3.1; All → 3.4 (E2E)

## Test Evidence

`go test ./internal/agents/pi`, `go test ./cmd/biggz-mcp`, `node --test`, `go test ./internal/install`, `go test ./internal/doctor`, `go vet && go test ./... && node --test` + Temp HOME `/mcp`
