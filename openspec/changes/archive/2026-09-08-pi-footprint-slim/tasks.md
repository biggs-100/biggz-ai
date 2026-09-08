# Tasks: pi-footprint-slim

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 220–300 |
| 400-line budget risk | Medium |
| Chained PRs recommended | No |
| Suggested split | Single PR (units land together) |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: stacked-to-main
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Adapter 10-tool allowlist + allowlist-prune merge + mirror tests | PR 1 | `go test ./internal/agents/pi/...` | N/A — no runtime effect without reinstall; unit tests prove merge | Revert `adapter.go` + `adapter_test.go`, reinstall |
| 2 | Asset trim + spec sync + reinstall/verify | PR 1 | `go test ./internal/install/...` + `grep -c REMINDER APPEND_SYSTEM.md` | `biggz install --agent pi` + `doctor pi-mcp-adapter` + diff `mcp.json` == 10 tools | Revert assets + spec, reinstall |

## Phase 1: Core Implementation

- [x] 1.1 Trim `piDirectTools` in `internal/agents/pi/adapter.go` from 20 to the 10-tool allowlist
- [x] 1.2 Switch `mergePiDirectTools` to allowlist-prune: drop the 10 removed `biggz_mem_*` names, preserve foreign entries, keep atomic write

## Phase 2: Assets and Spec

- [x] 2.1 Trim `internal/assets/biggz/biggz-orchestrator.md`, `biggz-orchestrator-workflow.md`, `biggz-orchestrator-delegation.md`: REMINDER dupes → x1, cut verbose examples; markers/template/tokens verbatim
- [x] 2.2 Trim `biggz-persona.md`, `bigmem-protocol.md`, `web-tools.md` only where REMINDER/dupes exist; markers/tokens verbatim
- [x] 2.3 Sync delta into `openspec/specs/pi-integration/spec.md` (10-tool + prune + single-REMINDER requirements)

## Phase 3: Testing and Verification

- [x] 3.1 Update `internal/agents/pi/adapter_test.go`: fresh == 10, reinstall prunes stale 10, foreign preserved, idempotent
- [x] 3.2 Run `go test ./internal/agents/pi/... ./internal/install/... ./cmd/biggz-mcp/...` green; server profile stays 20
- [x] 3.3 Reinstall `biggz install --agent pi`, verify `mcp.json` `directTools` == 10, single REMINDER, `doctor pi-mcp-adapter` clean
