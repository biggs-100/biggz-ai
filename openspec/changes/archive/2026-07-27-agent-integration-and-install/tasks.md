# Tasks: Agent Integration and Install

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~1500-2000 |
| 400-line budget risk | Medium |
| Chained PRs recommended | Yes |
| Suggested split | PR #1: Interface + Adapter + Assets → PR #2: File Merge → PR #3: Install + CLI |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Interface + adapter + assets | PR #1 | `go test ./internal/agents/opencode/... ./plugintest/...` | N/A — unit-tested only | plugin/ + internal/agents/ + internal/assets/ |
| 2 | File merge library | PR #2 | `go test ./internal/filemerge/...` | N/A — library only | internal/filemerge/ |
| 3 | Install command + wiring | PR #3 | `go test ./internal/install/...` | N/A — unit-tested via FakeAgent | internal/install/ + cmd/biggz/main.go |

## Phase 1: Interface & Adapter

- [x] 1.1 Extend `plugin/interfaces.go` — add GlobalConfigDir, SkillsDir, SettingsPath(homeDir string) string to AgentAdapter
- [x] 1.2 Create `internal/agents/opencode/adapter.go` — OpenCode adapter: Detect via exec.LookPath, paths via os.UserHomeDir + filepath.Join, capabilities
- [x] 1.3 Create `internal/agents/opencode/adapter_test.go` — detection tests with LookPath mock, path resolution under temp dir
- [x] 1.4 Extend `plugintest/agent.go` — add tempDir field, SetTempDir(path), GlobalConfigDir/SkillsDir/SettingsPath methods
- [x] 1.5 Create `plugintest/agent_test.go` — verify all path methods resolve under TempDir

## Phase 2: Assets

- [x] 2.1 Create `internal/assets/embed.go` — `//go:embed all:skills all:opencode` with embed.FS export
- [x] 2.2 Create 12 SDD skill stubs under `internal/assets/skills/sdd-{init,explore,propose,spec,design,tasks,apply,verify,archive,onboard,new,ff}/*/SKILL.md`
- [x] 2.3 Create `internal/assets/skills/_shared/` — sdd-phase-common.md, SKILL.md
- [x] 2.4 Create `internal/assets/opencode/sdd-overlay-single.json`, `sdd-overlay-multi.json`
- [x] 2.5 Create `internal/assets/opencode/commands/` — slash command .md files for /sdd-*

## Phase 3: File Merge

- [x] 3.1 Create `internal/filemerge/writer.go` — WriteFile: CreateTemp → close → os.Rename (atomic on same filesystem)
- [x] 3.2 Create `internal/filemerge/writer_test.go` — concurrent writes, partial write recovery, permissions
- [x] 3.3 Create `internal/filemerge/json_merge.go` — MergeJSONC: stripComments → stripTrailingCommas → json.Unmarshal → merge top-level keys → MarshalIndent
- [x] 3.4 Create `internal/filemerge/json_merge_test.go` — section add, section replace, empty/nil input edge cases
- [x] 3.5 Create `internal/filemerge/section.go` — InsertSection, ReplaceSection with marker-based markdown injection
- [x] 3.6 Create `internal/filemerge/section_test.go` — insert, replace, noop, missing marker edge cases

## Phase 4: Install Command

- [x] 4.1 Create `internal/install/install.go` — Run(ctx, adapter, Config) → Result: detect → deploy skills → merge JSONC overlay
- [x] 4.2 Create `internal/install/install_test.go` — dry-run reports plan, actual deploy writes files, idempotent re-deploy via FakeAgent + TempDir
- [x] 4.3 Modify `cmd/biggz/main.go` — add `os.Args[1] == "install"` gate before existing stdin pipeline, route to installRun()

## Phase 5: Verify

- [x] 5.1 Run `go test ./...` — all tests pass
- [x] 5.2 Run `go vet ./...` — clean
