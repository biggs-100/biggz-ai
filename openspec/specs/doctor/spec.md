# Doctor Specification

## Purpose

Doctor provides read-only health checks for biggz-ai installations, including SDD asset drift detection via SHA256 manifest comparison.

## Requirements

### Requirement: SDD Asset Drift Read-Only Checks

The system MUST add `biggz doctor` RO checks `sddGlobalAssetDriftCount` and `sddLocalAgentOverrideCount` computed via `assets/managed.go:ManagedAssetHash` SHA256 against `managed-assets.json` v1, report `warn: Global SDD asset drift N` when `N>0` with status `warn` (not `fail`), expose no `--fix`, and keep Runner panic-isolated via `diagnostics/doctor.go`.

#### Scenario: Global drift warn
- GIVEN one global `sdd-*.md` hash differs from manifest
- WHEN `biggz doctor` runs
- THEN `sddGlobalAssetDriftCount` MUST be `1` and result MUST be `warn` with message containing `Global SDD asset drift 1`

#### Scenario: Local override warn
- GIVEN local agent override hash differs
- WHEN doctor runs
- THEN `sddLocalAgentOverrideCount` MUST reflect count and status `warn`

#### Scenario: No drift pass and no fix
- GIVEN all hashes match
- WHEN `biggz doctor` and `biggz doctor --json` run
- THEN both counts MUST be `0` with `pass`; CLI MUST NOT accept `--fix`

#### Scenario: Panic isolation
- GIVEN drift check panics
- WHEN `Runner.Run()` completes
- THEN drift result MUST be `warn`/`fail` with panic message and other checks unaffected

### Requirement: Pi MCP Adapter Health Check

`biggz doctor` MUST add read-only checks for `pi-mcp-adapter` presence/version `^2` and `biggz-mcp` stdio health (`mcpServers.bigmem` reachable, `tools/list` has `biggz_mem_*`, `/mcp` live). Missing/non-`^2` MUST be `warn` with hint; crash MUST be `warn`/`fail` with `BIGGZ_MCP_TIMEOUT` guidance; panic-isolated.

#### Scenario: Healthy passes
- GIVEN `pi-mcp-adapter@2.32.1` and `biggz-mcp` healthy with `biggz_mem_*` in `tools/list`
- WHEN doctor runs
- THEN check MUST be `pass` and `/mcp` healthy

#### Scenario: Missing warns with hint
- GIVEN adapter not installed but MCP JSON exists
- WHEN doctor runs
- THEN result MUST be `warn` naming adapter and `pi install npm:pi-mcp-adapter`

#### Scenario: Version drift and crash
- GIVEN adapter `3.0.0` or `biggz-mcp` crashed
- WHEN doctor runs
- THEN mismatch MUST be `warn` and crash MUST be `warn`/`fail` with timeout guidance, others unaffected
