# Proposal: Component System + Catalog + Planner + State + Agent Registry

## Intent

biggz-ai needs an organized component architecture to manage agent installation, skill deployment, and config orchestration beyond the current ad-hoc `install.Run()` approach. This change introduces a Catalog for discoverability, a Planner for dependency-aware execution, State persistence for sync tracking, and an Agent Registry factory — all ported from gentle-ai with simplified types/IDs.

## Scope

### In Scope
- **Catalog**: Hardcoded Go slices describing agents, components, and skills with metadata (ID, Name, Description, Tier)
- **Planner**: Graph + Resolver + TopologicalSort + soft ordering — port 1:1 from gentle-ai
- **Agent Registry**: Factory + registry pattern; keep 3 adapters (OpenCode, Claude, Qwen)
- **State Persistence**: JSON in `~/.biggz-ai/state.json` with merge — schema: `InstallState{AgentID, Components, Skills, LastSync, PendingSync}`
- **3 Components** (`internal/components/`): skills, config, prompts — injectable wrappers around existing `internal/install` deploy functions

### Out of Scope
- Porting other 13 agent adapters from gentle-ai
- Porting gentle-ai components (engram, sdd, persona, gga, context7, theme, permissions...)
- TUI (separate change)
- System detection / platform profiles (separate change)

## Capabilities

### New Capabilities
- `component-catalog`: metadata-driven catalog of agents, components, and skills
- `planner`: graph-based dependency resolver with topological sort and soft ordering
- `agent-registry`: factory pattern for agent adapter creation and registration
- `state-persistence`: JSON state file with read/merge/write lifecycle

### Modified Capabilities
- `plugin-system`: enrich AgentAdapter interface with factory metadata; registry gains catalog integration

## Approach

1. **Catalog** (`internal/components/catalog/`): define `CatalogEntry{ID, Name, Description, Tier string}` types. Expose `AllAgents()`, `AllComponents()`, `AllSkills()` as hardcoded slices.
2. **Planner** (`internal/components/planner/`): port Graph + Resolver from gentle-ai. Topological sort with soft ordering (warn on cycle, continue).
3. **Agent Registry** (`internal/agents/`): add `Factory` func type + `Register(name, factory)`. Adapt existing 3 adapters. Add `ListAll()` returning catalog entries.
4. **State** (`internal/components/state/`): read/write `~/.biggz-ai/state.json`. Merge strategy: incoming fields overwrite, unknown fields preserved.
5. **Components** (`internal/components/`): 3 wrappers calling `install.DeploySkills`, `install.DeployConfig`, `install.DeployPrompts` respectively.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `plugin/interfaces.go` | Modified | Enrich AgentAdapter with factory metadata |
| `internal/agents/` | Modified | Add factory + registry pattern |
| `internal/components/` | New | Catalog, planner, state, 3 wrappers |
| `registry/registry.go` | Modified | Integrate with agent registry |
| `openspec/specs/plugin-system/spec.md` | Modified | AgentAdapter requirement changes |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Planner cycle resolution diverges from gentle-ai | Low | Port tests alongside logic |
| State file format changes require migration later | Low | Keep schema minimal; merge preserves unknown fields |

## Rollback Plan

Revert the 4 new sub-packages. Restore `plugin/interfaces.go` and `registry/registry.go` from git. No data migration needed — state file is additive only.

## Dependencies

- Existing `internal/install` package (components wrap its deploy functions)
- Existing agent adapters in `internal/agents/`

## Success Criteria

- [ ] Catalog returns all 3 agents, 3 components, available skills by tier
- [ ] Planner resolves a 5-node dependency graph with correct topological order
- [ ] Agent registry creates instances via factory for all 3 adapters
- [ ] State persists and round-trips through JSON with merge
- [ ] Each component wrapper successfully calls its underlying deploy function
