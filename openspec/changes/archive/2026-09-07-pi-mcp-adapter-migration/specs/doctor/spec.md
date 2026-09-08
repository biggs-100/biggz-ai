# Delta for doctor

## ADDED Requirements

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
