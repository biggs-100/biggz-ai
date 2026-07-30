# Proposal: Enrich AgentAdapter Interface

## Intent

Current `AgentAdapter` (11 methods) is too thin — missing typed `AgentID`, strategy enums, `InstallCommand`, 6 path methods, optional capability guards, capability manifest, and codegraph detector. Blocks porting the remaining 13 adapters.

## Scope

### In Scope
- Enrich `plugin.AgentAdapter` to ~22 methods (gentle-ai parity)
- Types: `model.AgentID`, `model.SupportTier`, `model.SystemPromptStrategy`, `model.MCPStrategy`
- `Detect(ctx, homeDir) (bool, string, string, bool, error)` — update all callers
- `InstallCommand(ctx) []string`
- New paths: `SystemPromptDir`, `SystemPromptFile`, `MCPConfigPath`, `CommandsDir`, `SubAgentsDir`, `OutputStyleDir`, `EmbeddedSubAgentsDir`
- Optional guards: `Supports{Skills,SystemPrompt,MCP,OutputStyles,SlashCommands,SubAgents}`
- `CapabilityManifest` + `featureClaimsByAgent` (16 entries)
- `EffectiveCodeGraphWiringDetector`
- Discovery: return ALL installed agents (not just first)
- Update opencode, claude, qwen adapters + tests

### Out of Scope
- Other 13 adapters (antigravity, codex, cursor, gemini, hermes, kilocode, kimi, kiro, openclaw, pi, trae, vscode, windsurf) — manifests only, no implementation
- DeployConfig, file merge, CLI changes

## Capabilities

### New Capabilities
- `agent-identity`: typed AgentID, SupportTier
- `agent-detect`: 5-return Detect with binary + config path
- `agent-install-command`: InstallCommand for auto-setup
- `agent-config-strategies`: SystemPromptStrategy, MCPStrategy enums
- `agent-path-discovery`: all missing path methods
- `agent-capability-manifest`: CapabilityManifest + 16-entry featureClaimsByAgent
- `agent-codegraph-detector`: EffectiveCodeGraphWiringDetector

### Modified Capabilities
- `plugin-system`: AgentAdapter enriched 11→22 methods
- `agent-registry`: Adapter interface updated to match

## Approach

Mirror gentle-ai's architecture: identity types in `model/`, enriched interface in `plugin/`, manifest + detector in `internal/agents/`. Refactor discovery to return all agents. Update 3 adapters atomically.

## Affected Areas

| Area | Impact |
|------|--------|
| `plugin/interfaces.go` | Modified — ~11 new methods, new signatures |
| `model/types.go` (new) | New — AgentID, SupportTier, strategies |
| `internal/agents/manifest.go` (new) | New — CapabilityManifest + featureClaimsByAgent |
| `internal/agents/detector.go` (new) | New — EffectiveCodeGraphWiringDetector |
| `internal/agents/discovery.go` | Modified — return all installed agents |
| `internal/agents/registry.go` | Modified — model.AgentID key |
| `internal/agents/opencode/*.go` | Modified — implement all new methods |
| `internal/agents/claude/*.go` | Modified — implement all new methods |
| `internal/agents/qwen/*.go` | Modified — implement all new methods |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Detect signature breaks callers | High | Update all callers in same atomic commit |
| Interface too large for minimal clients | Low | Guards return false, paths return "" |

## Rollback Plan

Single revert. Old 11-method interface is a compile-time contract — atomic revert restores clean state.

## Success Criteria

- [ ] All 3 adapters compile against ~22-method AgentAdapter
- [ ] model.AgentID is registry key, not raw string
- [ ] Detect 5 return values handled by all callers
- [ ] featureClaimsByAgent has exactly 16 entries
- [ ] EffectiveCodeGraphWiringDetector compiles
- [ ] All existing tests pass
