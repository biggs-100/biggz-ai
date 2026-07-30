# Design: Component System + Catalog + Planner + State + Agent Registry

## Technical Approach

Five independent Go packages built bottom-up: **Catalog** (pure data), **Planner** (graph algorithm), **State** (IO + merge), **Components** (deploy wrappers), **Agent Registry** (factory + discovery). The existing `plugin.AgentAdapter` interface is enriched with 3 new methods (`MCPStrategy`, `SupportsAutoInstall`, `Capabilities() → []string`), and the build-time `registry.Registry` gains `ListAll()` + renamed methods (`RegisterAdapter`/`GetAdapter`).

Two separate registries serve distinct purposes: `internal/agents/` holds a factory map for lazy adapter creation, `registry/` holds pre-wired singleton instances for the pipeline.

## Architecture Decisions

### Decision: Two registries pattern

| Option | Tradeoff |
|--------|----------|
| Combine into one registry | Mixed concerns: factory creation vs. singleton wiring |
| **Two registries (chosen)** | Clear separation; factory registry for CLI/discovery, build-time registry for pipeline |

**Rationale**: The agent-registry spec demands a lazy `Factory` map (create adapters on demand), while the existing `registry.Registry` holds pre-wired singletons for the pipeline. Merging them would couple startup wiring to install-time discovery — two different lifecycles.

### Decision: Capabilities as `[]string` (breaking change)

**Choice**: `Capabilities() []string` instead of `[]plugin.Capability`
**Rationale**: The delta spec requires this. It enables cross-package capability checks without importing `plugin`. All 3 adapters + any callers using `==` on `Capability` constants must update to string comparison.

### Decision: Catalog as hardcoded slices

**Choice**: Package-level `var` slices in `internal/catalog/`, returned by value (defensive copy)
**Rationale**: Spec requires zero runtime init. Returning by value prevents caller mutation from corrupting the originals. Three separate slices over a unified collection avoids coupling unrelated domains.

### Decision: State merge preserves unknown fields

**Choice**: `encoding/json` with `map[string]any` for unknown fields, then re-serialized alongside known struct fields
**Rationale**: Forward compat when schema evolves. JSON round-trip preserves fields the current binary doesn't understand.

## Data Flow

```
CLI / install flow
  │
  ├─→ catalog.AllAgents() / ListComponents(tier)  ──→ filtered entries
  │
  ├─→ agents.NewDefaultRegistry() → Factory("opencode") → opencode.Adapter
  │     (adapters implement enriched plugin.AgentAdapter)
  │
  ├─→ planner.Plan(targets)
  │     └─→ Graph.AddNode/AddEdge → Resolver.Resolve() → topo-sorted plan
  │     └─→ BuildReviewPayload(plan) → summary []Step
  │
  ├─→ state.ReadState(homeDir) → InstallState
  │
  └─→ components/skills.Deploy(ctx, adapter) → DeploymentResult
        └─→ install.DeploySkills(skillsDir, assets.FS, dryRun)
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/catalog/catalog.go` | Create | Types (`CatalogEntry`, `SkillEntry`, `ComponentEntry`), all hardcoded slices, lookup functions |
| `internal/planner/graph.go` | Create | `Graph` type, `AddNode`/`AddEdge`, adjacency list |
| `internal/planner/resolver.go` | Create | `Resolver.Resolve()` — topological sort + dependency injection |
| `internal/planner/planner.go` | Create | `Planner.Plan(targets)` — orchestration wrapper |
| `internal/planner/review.go` | Create | `BuildReviewPayload(plan)` → readable summary |
| `internal/state/state.go` | Create | `InstallState`, `ReadState`, `WriteState`, `MergeState` |
| `internal/components/skills.go` | Create | `Deploy(ctx, adapter) → DeploymentResult` wrapping `install.DeploySkills` |
| `internal/components/config.go` | Create | `Deploy(ctx, adapter) → DeploymentResult` wrapping `install.DeployConfig` |
| `internal/components/prompts.go` | Create | `Deploy(ctx, adapter) → DeploymentResult` wrapping `install.DeployPrompts` |
| `internal/agents/registry.go` | Create | `Factory` type, `Registry` (name→Factory), `NewDefaultRegistry()`, `Register`, `ListAll` |
| `internal/agents/discovery.go` | Create | `DetectInstalled(ctx)` — iterates factories, returns first match |
| `plugin/interfaces.go` | Modify | Add `MCPStrategy()`, `SupportsAutoInstall()`; change `Capabilities()` return to `[]string` |
| `internal/agents/opencode/adapter.go` | Modify | Add new methods, update Capabilities return type |
| `internal/agents/claude/adapter.go` | Modify | Same |
| `internal/agents/qwen/adapter.go` | Modify | Same |
| `registry/registry.go` | Modify | Rename `RegisterAgent`→`RegisterAdapter`, `GetAgent`→`GetAdapter`; add `ListAll()` |

## Interfaces / Contracts

```go
// Enriched plugin.AgentAdapter (modified)
type AgentAdapter interface {
    ID() string
    Name() string
    Detect(ctx context.Context) (string, error)
    Capabilities() []string                           // was []Capability
    SupportsAutoInstall() bool                        // new
    MCPStrategy() string                              // new
    GlobalConfigDir(homeDir string) string
    SkillsDir(homeDir string) string
    SettingsPath(homeDir string) string
    DeployConfig(ctx context.Context, cfg AgentConfig) error
}

// internal/catalog types (new)
type CatalogEntry struct {
    ID, Name, Description, Tier string
}
type SkillEntry struct {
    CatalogEntry
    Platforms  []string
    DependsOn  []string
}
type ComponentEntry struct {
    CatalogEntry
    Dependencies []string
}

// internal/state types (new)
type InstallState struct {
    AgentID     string                     `json:"agent_id"`
    Components  map[string]ComponentStatus `json:"components"`
    Skills      map[string]SkillStatus     `json:"skills"`
    LastSync    time.Time                  `json:"last_sync"`
    PendingSync int                        `json:"pending_sync"`
}

// internal/components (new)
type DeploymentResult struct {
    Changed bool
    Files   []string
}
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit (Catalog) | AllAgents returns 3 entries, slice immutability, GetAgent/ListComponents/IsSupportedAgent | Standard table-driven Go tests |
| Unit (Planner) | Graph edge rejection, linear/diamond/cycle resolution, Plan ordering, BuildReviewPayload | Port gentle-ai test cases |
| Unit (State) | JSON round-trip, missing file returns default, malformed JSON error, MergeState overwrite + preserve unknown | Temp dir per test |
| Unit (Components) | Each Deploy calls correct install function (mock adapter) | Interface mock or deploy func injection |
| Unit (Agents) | Factory produces correct adapter, duplicate registration, NewDefaultRegistry returns 3 | Direct assertions |
| Integration | State write then read round-trip with temp homeDir | Temp dir lifecycle |
| Integration | Planner Graph + Resolver with 5-node tree | Topo sort verification |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary.

## Migration / Rollout

No migration required. The `~/.biggz-ai/state.json` file is additive — old binaries that don't create it are unaffected. The AgentAdapter interface change (`[]Capability` → `[]string`, new methods) is a compile-time breaking change enforced by the Go type system; all 3 adapters and any consumers must update atomically in this change.

## Open Questions

- [ ] None — specs define clear behavior for all 5 packages.

