# Delta for pi-integration

## ADDED Requirements

### Requirement: Pi BigMem MCP Provisioning via Adapter

The system MUST provision `mcpServers.bigmem` with `command=BiggzMCPPath()`, `args=["--tools=agent","--prefix=biggz"]`, `type="local"` plus `imports:["opencode"]` and `directTools` via `ProvisionBigMemMCP` into BOTH `~/.pi/agent/settings.json` and `~/.pi/agent/mcp.json` atomically via `filemerge.WriteFileAtomic`, preserving other servers; project `.pi/mcp.json` overlays global but `bigmem` MUST stay authoritative.

#### Scenario: Fresh provision correct shape
- GIVEN no Pi MCP config exists
- WHEN `ProvisionBigMemMCP` executes
- THEN `settings.json` MUST have `mcpServers.bigmem` with `--prefix=biggz` and `mcp.json` MUST have `mcpServers.bigmem` + `imports:["opencode"]` + `directTools`

#### Scenario: Merge preserves others atomically
- GIVEN `settings.json` with `mcpServers.other`
- WHEN merge runs
- THEN `other` MUST be preserved, `bigmem` added/updated, failed write MUST leave target unchanged

#### Scenario: Global vs project precedence
- GIVEN global and project `mcp.json` exist
- WHEN adapter resolves
- THEN project MUST overlay global but `bigmem` MUST win in both files

### Requirement: Adapter-Aware Wrapper Fallback

Wrappers `biggz-memory-chrome`/`biggz-synthesis-gate` MUST gate on `!pi.getTool("biggz_mem_save")` to avoid double-render; when absent MUST fallback for one release; `PI_SUBAGENT_CHILD=1` bypass preserved.

#### Scenario: Adapter present suppresses wrapper
- GIVEN `pi.getTool("biggz_mem_save")` returns native tool
- WHEN wrapper handler fires
- THEN it MUST no-op without pill duplication or re-block

#### Scenario: Adapter absent retains wrapper
- GIVEN `pi.getTool("biggz_mem_save")` falsy
- WHEN handlers fire
- THEN wrappers MUST render pill and enforce gate as before

### Requirement: BigMem MCP Tool Annotations

`cmd/biggz-mcp:buildToolList` MUST set `readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint` per BigMem semantics for adapter filtering.

#### Scenario: Read-only marked
- GIVEN `tools/list` after build
- WHEN inspecting `biggz_mem_search`/`biggz_mem_get`
- THEN each MUST have `readOnlyHint:true`, `destructiveHint:false`

#### Scenario: Mutating not read-only
- GIVEN same list
- WHEN inspecting `biggz_mem_save`
- THEN it MUST have `readOnlyHint:false`
