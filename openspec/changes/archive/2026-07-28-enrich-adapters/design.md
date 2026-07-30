# Design: Enrich AgentAdapter Interface

## Technical Approach

Mirror gentle-ai's architecture: identity types in `model/`, enriched interface in `plugin/`, manifest+detector in `internal/agents/`. Add ~16 methods to `AgentAdapter` alongside existing ones without rename. Refactor discovery from first-match to all-installed. Keep changes atomic within each stacked PR.

## Architecture Decisions

| Decision | Choice | Alternative | Rationale |
|----------|--------|-------------|-----------|
| Types location | `model/types.go` new file | `plugin/types.go` | model pkg already exists, avoids plugin → model import loop |
| SupportTier type | `string` enum | `int` enum | Human-readable in JSON serialization (catalog.go Tier field stays string compat) |
| CapabilityManifest package | `internal/agents/capabilitymanifest/` | Inline in agents | Separate concern, testable in isolation, matches gentle-ai layout |
| featureClaimsByAgent size | 16 entries | 3 (implemented only) | Canonical truth for ALL known agents; 13 out-of-scope agents get claims-only entries |
| ID() return type | `model.AgentID` | string | Typed identity propagates through registry — map key safety, grepable refactors |
| Detect signature | `(ctx, homeDir) (bool, binaryPath, configPath, autoInstallCapable, error)` | Keep old | unified.homeDir param → all 5 returns enables both detection and config resolution |

## Data Flow

```
model.AgentID ──→ AgentAdapter.ID() ──→ Registry map key
                      │
plugin/interfaces.go  │
  AgentAdapter ───────┤
                      │
CapabilityManifest ───┤── ForAgent(id) ──→ AgentFeatureClaims
                      │
DetectInstalled ──────┘──→ []AgentAdapter (all, not first)
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `model/types.go` | Create | AgentID, SupportTier, SystemPromptStrategy, MCPStrategy types |
| `internal/agents/capabilitymanifest/manifest.go` | Create | AgentCapabilityManifest, AgentFeatureClaims, featureClaimsByAgent (16 entries), ForAgent, ResolveCapabilityManifest |
| `internal/agents/detector.go` | Create | EffectiveCodeGraphWiringDetector |
| `plugin/interfaces.go` | Modify | Add ~16 new methods, change ID/Detect/MCPStrategy signatures |
| `internal/agents/opencode/adapter.go` | Modify | Implement all new methods with opencode-specific values |
| `internal/agents/claude/adapter.go` | Modify | Implement all new methods with claude-specific values |
| `internal/agents/qwen/adapter.go` | Modify | Implement all new methods with qwen-specific values |
| `internal/agents/registry.go` | Modify | `map[string]Factory` → `map[model.AgentID]Factory` |
| `internal/agents/factory.go` | Modify | globalFactories key type |
| `internal/agents/discovery.go` | Modify | Return `[]plugin.AgentAdapter` (all detected) |
| `plugintest/agent.go` | Modify | FakeAgent implements new methods |
| `plugintest/agent_test.go` | Modify | New method tests |
| `internal/install/install.go` | Modify | Update Detect caller |
| `cmd/biggz/main.go` | Modify | Update Detect caller |
| `registry/registry.go` | Modify | Handle model.AgentID from ID() |
| `internal/catalog/catalog.go` | Modify | Tier field compatibility |
| Test files (6) | Modify | Update mock adapters, add capabilitymanifest tests |

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | model types | Type conversion, equality, map-key usage |
| Unit | featureClaimsByAgent | Exactly 16 entries, ForAgent lookup, ResolveCapabilityManifest cross-validation |
| Unit | 3 adapters | Every new method returns expected value per adapter |
| Unit | DiscoverInstalled | Multiple agents, empty registry, error cases |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary.

## Migration / Rollout

3 stacked PRs to main. PR 1 (foundation) merges safely. PR 2 (interface+adapters) must be atomic — old callers break until PR 3 (integration). No data migration required.

## Open Questions

None.
