# Tasks: Enrich AgentAdapter Interface

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 850–1000 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Foundation) → PR 2 (Interface + Adapters) → PR 3 (Integration) |
| Delivery strategy | force-chained |
| Chain strategy | stacked-to-main |

```
Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High
```

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | model types + capabilitymanifest + detector | PR 1 | `go test ./model/ ./internal/agents/capabilitymanifest/` | `go build ./...` | Revert `model/types.go`, `internal/agents/capabilitymanifest/`, `internal/agents/detector.go` |
| 2 | interface enrichment + 3 adapters + FakeAgent | PR 2 | `go build ./plugin/ ./internal/agents/opencode/ ./internal/agents/claude/ ./internal/agents/qwen/ ./plugintest/` | `go build ./...` | Revert `plugin/interfaces.go` + 3 adapter dirs + `plugintest/agent.go` |
| 3 | registry + discovery + callers (main, install) | PR 3 | `go test ./internal/agents/ ./registry/ ./internal/install/` | `go run .` | Revert `internal/agents/registry.go`, `discovery.go`, `factory.go`, `registry/registry.go`, `cmd/`, `internal/install/` |

## Phase 1: Foundation Types (PR 1)

- [x] 1.1 Create `internal/agents/types.go` — `AgentID` (typed string, 16 constants), `SupportTier` (typed string, `TierFull`)
- [x] 1.2 Create `internal/agents/systemprompt.go` — `SystemPromptStrategy` (int enum, 6 constants + String())
- [x] 1.3 Create `internal/agents/mcpstrategy.go` — `MCPStrategy` (int enum, 5 constants + String())
- [x] 1.4 Write tests for all new types (AgentID, SupportTier, SystemPromptStrategy, MCPStrategy)
- [x] 1.5 Create `internal/agents/capabilitymanifest/manifest.go` with `AgentCapabilityManifest`, `AgentFeatureClaims` (8 bool fields), `featureClaimsByAgent` (16 entries), `ForAgent(agentID)`, `Validate()`
- [x] 1.6 Create `internal/agents/detector.go` with `EffectiveCodeGraphWiringDetector`
- [x] 1.7 Write tests for capabilitymanifest (16 entries, ForAgent, Validate match/mismatch)

## Phase 2: Interface + Adapters (PR 2)

- [x] 2.1 Enrich `plugin/interfaces.go` AgentAdapter: change `ID() string` to `ID() model.AgentID`, `Detect(ctx)` to `Detect(ctx, homeDir) (bool, string, string, bool, error)`, `MCPStrategy() string` to `MCPStrategy() model.MCPStrategy`; add `Tier()`, `InstallCommand(profile) [][]string, error`, 6x `Supports*()` guards, 7x path methods, `SystemPromptStrategy()`
- [x] 2.2 Update `internal/agents/opencode/adapter.go` — implement all new methods with opencode-specific values
- [x] 2.3 Update `internal/agents/claude/adapter.go` — implement all new methods with claude-specific values
- [x] 2.4 Update `internal/agents/qwen/adapter.go` — implement all new methods with qwen-specific values
- [x] 2.5 Update `plugintest/agent.go` FakeAgent — implement all new methods
- [x] 2.6 Update 3 adapter test files + plugintest tests for new signatures and methods

## Phase 3: Integration (PR 3) — Manifest + Discovery + Callers

- [x] 3.1 Change `internal/agents/registry.go` — `map[string]Factory` → `map[model.AgentID]Factory`, add manifest validation on Register, update Register/Get/ListAll/NewDefaultRegistry signatures
- [x] 3.2 Change `internal/agents/factory.go` — globalFactories key to model.AgentID
- [x] 3.3 Refactor `internal/agents/discovery.go` — `DetectInstalled` returns `[]InstalledAgent` (all installed), check GlobalConfigDir on disk
- [x] 3.4 Update `registry/registry.go` — handle model.AgentID from `a.ID()` (already compatible)
- [x] 3.5 Update `cmd/biggz/main.go` — already compatible with new Detect signature
- [x] 3.6 Update `internal/install/install.go` — already compatible with new Detect signature
- [x] 3.7 Update `internal/catalog/catalog.go` — use AgentID typed constants from `internal/agents`
- [x] 3.8 Update `internal/agents/registry_test.go` — use valid AgentIDs from manifest, handle new signatures
