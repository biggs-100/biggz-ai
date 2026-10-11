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

### Requirement: Pi Native MCP Health Check

`biggz doctor` MUST add a read-only, panic-isolated check with id `pi-native-mcp` (replacing the `pi-mcp-adapter` version check; no `^2` version check remains). It MUST skip with `pass` when Pi is absent; warn when `pi-mcp-adapter` is still installed, naming the consequence (while an extension registers `/mcp`, Pi ignores `mcp.json` and its own servers) and the remedy `pi remove npm:pi-mcp-adapter`; validate `~/.pi/agent/mcp.json` — `mcpServers.bigmem` present, `command` resolvable, no non-native `type`, `exposure`/`toolExposure` sane, no legacy `imports`/`directTools`; warn when `settings.json` carries `mcpServers.bigmem` or `directTools`; and run the bounded `biggz-mcp` stdio probe plus `pi mcp list` liveness behind `BIGGZ_MCP_TIMEOUT`. Missing config MUST be `warn` with the `biggz install --agent pi` hint; a crash MUST be `warn`/`fail` with `BIGGZ_MCP_TIMEOUT` guidance.

#### Scenario: Healthy native config passes
- GIVEN no `pi-mcp-adapter`, native `mcp.json` `bigmem`, `settings.json` with no MCP state, `pi mcp list` reporting `bigmem`, and `biggz-mcp --help` exit 0
- WHEN doctor runs
- THEN check `pi-native-mcp` MUST be `pass`

#### Scenario: Missing config warns with install hint
- GIVEN no `mcp.json` or an `mcpServers.bigmem`-less one
- WHEN doctor runs
- THEN result MUST be `warn` naming the missing native config and `biggz install --agent pi`

#### Scenario: Adapter still installed warns with removal remedy
- GIVEN the `pi-mcp-adapter` npm directory or its `settings.json` packages entry is present
- WHEN doctor runs
- THEN result MUST be `warn` naming the retired adapter and `pi remove npm:pi-mcp-adapter`

#### Scenario: Legacy and non-native shape warns
- GIVEN `mcp.json` carries `imports`/`directTools`, a non-native `type`, or a bad `exposure`/`toolExposure`
- WHEN doctor runs
- THEN the first violation MUST be `warn` naming the hint `biggz install --agent pi`

#### Scenario: Crash maps to BIGGZ_MCP_TIMEOUT
- GIVEN `biggz-mcp` crashes or `pi mcp list` fails
- WHEN doctor runs
- THEN result MUST be `warn`/`fail` with `BIGGZ_MCP_TIMEOUT` guidance, others unaffected

#### Scenario: Pi absent skips
- GIVEN the `pi` binary is not on `PATH`
- WHEN doctor runs
- THEN the check MUST be `pass` with the skip message and no files read
