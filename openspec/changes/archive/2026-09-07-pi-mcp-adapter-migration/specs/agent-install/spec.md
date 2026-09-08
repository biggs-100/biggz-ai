# Delta for agent-install

## ADDED Requirements

### Requirement: Pi MCP Adapter in InstallCommand

`Adapter.InstallCommand` MUST include `npm:pi-mcp-adapter@^2` before `pi-subagents-j0k3r`, pinned `^2` (v2.32.1 shape), idempotent, offline-tolerant with doc fallback.

#### Scenario: Includes adapter in order
- GIVEN Pi `InstallCommand` invoked
- WHEN list generated
- THEN it MUST contain `npm:pi-mcp-adapter` preceding `pi-subagents-j0k3r` with `^2`

#### Scenario: Idempotent second run
- GIVEN adapter already installed
- WHEN command runs again
- THEN it MUST succeed without duplication

#### Scenario: Offline harmless
- GIVEN npm unreachable
- WHEN `pi install` fails
- THEN error MUST hint doc fallback and MCP JSON MUST remain harmless
