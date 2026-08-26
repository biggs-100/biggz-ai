# Tasks: Component System

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~900 across 22 files |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1: Foundation (~430 lines) → PR 2: Logic (~470 lines) |
| Delivery strategy | force-chained |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Plugin interface enrichment, catalog, agent registry, adapter updates, registry rename | PR 1 | N/A (no test runner) | N/A (library packages) | `git revert` of `plugin/`, `internal/catalog/`, `internal/agents/`, `registry/` |
| 2 | Planner, state, component deploy wrappers | PR 2 | N/A (no test runner) | N/A (library packages) | `git revert` of `internal/planner/`, `internal/state/`, `internal/components/` |

**Note**: `CatalogEntry` must live in `plugin/` (not `internal/catalog/`) so it's importable by both `internal/agents/` and `registry/`. The `internal/catalog/` package can embed it.

## Phase 1: Core Interface & Catalog (PR 1)

- [x] 1.1 Add `CatalogEntry` struct to `plugin/types.go` — shared type consumed by both registries and catalog
- [x] 1.2 Enrich `plugin/interfaces.go`: add `SupportsAutoInstall() bool` + `MCPStrategy() string`; change `Capabilities() []Capability` → `[]string`; keep Capability string consts as `[]string` values
- [x] 1.3 Create `internal/catalog/catalog.go`: `SkillEntry` / `ComponentEntry` (embed `plugin.CatalogEntry`); `AllAgents()`, `AllComponents()`, `AllSkills()` returning hardcoded slices by value; lookup helpers (`GetAgent`, `ListComponents`, `IsSupportedAgent`)
- [x] 1.4 Update 3 adapters (`opencode`, `claude`, `qwen`): implement `SupportsAutoInstall()`, `MCPStrategy()`; convert `Capabilities()` return to `[]string`
- [x] 1.5 Create `internal/agents/registry.go`: `Factory` func type; `Registry` with `Register(name, Factory)` + `ListAll() []plugin.CatalogEntry`; `NewDefaultRegistry()` pre-wiring 3 adapters
- [x] 1.6 Create `internal/agents/discovery.go`: `DetectInstalled(ctx, *Registry)` — iterates factories, returns first adapter whose `Detect(ctx)` succeeds
- [x] 1.7 Modify `registry/registry.go`: rename `RegisterAgent`→`RegisterAdapter`, `GetAgent`→`GetAdapter`; add `ListAll() []plugin.CatalogEntry` building entries from registered adapters
- [x] 1.8 Tests: catalog immutability + lookups; agent registry register/dup/list; discovery detect; adapter new methods; registry rename + ListAll

## Phase 2: Planner, State & Components (PR 2)

- [x] 2.1 Create `internal/planner/graph.go`: `Graph` with `AddNode(id)`, `AddEdge(from, to)` adjacency list; Kahn's topological sort with cycle detection
- [x] 2.2 Create `internal/planner/resolver.go`: `Resolver.Resolve(selection)` — topological sort + auto-dependency resolution
- [x] 2.3 Create `internal/planner/types.go`: `Plan`, `Selection`, `ComponentNode` types (replaced `planner.go` per PR 2 spec)
- [x] 2.4 Create `internal/planner/review.go`: `BuildReviewPayload(plan) → []Step` readable summary _(implemented in types.go)_
- [x] 2.5 Create `internal/state/state.go`: `InstallState` (`[]string` fields) with `Read(path)`, `Write(path)`, `Merge(base, overlay)`; JSON round-trip with unknown field preservation via `map[string]json.RawMessage`
- [x] 2.6 Create `internal/components/skills.go`: `SkillsComponent` implementing `Component` interface, wrapping `install.DeploySkills`
- [x] 2.7 Create `internal/components/config.go`: `ConfigComponent` implementing `Component` interface, wrapping `install.DeployConfig`
- [x] 2.8 Create `internal/components/prompts.go`: `PromptsComponent` implementing `Component` interface, wrapping `install.DeployPrompts`
- [x] 2.9 Tests: planner (17 tests: topological sort, cycle detection, linear/diamond/partial-cycle, dependencies, resolver with deps/unknown/cycle/skills); state (14 tests: read/missing/malformed/round-trip/write/merge variants/unknown fields); components (11 tests: skills/config/prompts deploy with files/empty/overlay, file list, ID)

## Total: 17 tasks across 2 phases / 2 stacked PRs
