# Delta for pi-integration

## MODIFIED Requirements

### Requirement: Pi BigMem MCP Provisioning via Adapter

The system MUST provision `mcpServers.bigmem` with `command=BiggzMCPPath()`, `args=["--tools=agent","--prefix=biggz"]`, `type="local"` plus `imports:["opencode"]` and `directTools` equal to exactly the 10-tool allowlist (`save`, `search`, `get_observation`, `context`, `session_summary`, `save_prompt`, `update`, `timeline`, `review`, `judge`) via `ProvisionBigMemMCP` into BOTH `~/.pi/agent/settings.json` and `~/.pi/agent/mcp.json` atomically via `filemerge.WriteFileAtomic`, preserving other servers; merge MUST be allowlist-prune (drop the 10 removed BigMem names when present, preserve foreign entries); project `.pi/mcp.json` overlays global but `bigmem` MUST stay authoritative; server `ProfileAgent` MUST stay at 20 tools.
(Previously: promoted full 20 tools with additive union merge, never pruning.)

#### Scenario: Fresh provision is 10 tools

- GIVEN no Pi MCP config exists
- WHEN `ProvisionBigMemMCP` executes
- THEN `directTools` MUST equal exactly the 10 allowlist in both files

#### Scenario: Reinstall prunes stale 10

- GIVEN `mcp.json` with all 20 `directTools`
- WHEN reinstall merges
- THEN the 10 removed names MUST be dropped, 10 allowlist MUST remain

#### Scenario: Foreign entries preserved atomically

- GIVEN `settings.json` with `mcpServers.other` plus foreign `directTools`
- WHEN merge runs
- THEN `other` and foreign entries MUST be preserved; failed write MUST leave target unchanged

#### Scenario: Global vs project precedence

- GIVEN global and project `mcp.json` exist
- WHEN adapter resolves
- THEN project MUST overlay global but `bigmem` MUST win in both files

#### Scenario: Server stays at 20

- GIVEN `--tools=agent` server profile
- WHEN `tools/list` runs
- THEN `ProfileAgent` MUST still expose all 20 tools

## ADDED Requirements

### Requirement: Slim APPEND_SYSTEM Generation

The system MUST generate `APPEND_SYSTEM.md` with a single REMINDER block, all `<!-- biggz:* -->` markers, gate template, and `{{BIGGZ_BACKGROUND_POLICY}}` plus other template tokens intact, with zero semantic change (prose/example trim only).

#### Scenario: Single REMINDER with markers intact

- GIVEN asset trim applied
- WHEN `APPEND_SYSTEM.md` is generated
- THEN exactly one REMINDER MUST exist and all markers/template/tokens MUST be present

#### Scenario: Reinstall does not reduplicate REMINDER

- GIVEN existing slim `APPEND_SYSTEM.md`
- WHEN reinstall regenerates
- THEN REMINDER count MUST stay one

### Requirement: Reinstall Convergence and Rollback

The system MUST converge fresh and existing installs to the 10-tool `directTools` plus slim prompt on every `biggz install --agent pi`; revert of sources plus reinstall MUST restore 20-tool promotion and full prompt with no migration.

#### Scenario: Existing install converges

- GIVEN deployed fat 20-tool `mcp.json`
- WHEN `biggz install --agent pi` re-runs
- THEN `directTools` MUST equal the 10 allowlist

#### Scenario: Rollback restores fat state

- GIVEN slim sources reverted
- WHEN `biggz install --agent pi` re-runs
- THEN 20-tool promotion and full prompt MUST return
