# Tasks: sync-config

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~315 |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Delivery strategy | force-chained (stacked-to-main) |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: stacked-to-main
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | filemerge: WriteFileAtomic + deep merge | PR 1 | `go test ./internal/filemerge/...` | `go build ./...` | revert `internal/filemerge/` |
| 2 | install export + CLI sync | PR 2 | `go build ./cmd/biggz/...` | `biggz sync --dry-run` | revert `internal/install/` + `cmd/biggz/main.go` |

## Phase 1: filemerge Upgrades

- [x] 1.1 Add `WriteResult` struct + `WriteFileAtomic` to `internal/filemerge/writer.go` — content comparison, temp+rename, no parent-dir creation
- [x] 1.2 Write tests in `internal/filemerge/writer_test.go` for content-skip, new file, overwrite, non-existent parent
- [x] 1.3 Upgrade `MergeJSONC` in `internal/filemerge/json_merge.go` — recursive deep merge into maps, array full-replace, `__replace__` sentinel
- [x] 1.4 Write tests in `internal/filemerge/json_merge_test.go` for deep merge, array replacement, `__replace__`, empty overlay, invalid JSON, comments

## Phase 2: Install Functions Export

- [x] 2.1 Export `DeploySkills`, `DeployConfig`, `DeployCommands`, `DeployPrompts` in `internal/install/install.go`
- [x] 2.2 Replace 4 `WriteFile` calls in install.go with `WriteFileAtomic`
- [x] 2.3 Update `internal/install/install_test.go` for any changed caller references

## Phase 3: CLI sync Command

- [x] 3.1 Add `case "sync"` to switch in `cmd/biggz/main.go` + dispatch to `syncRun()`
- [x] 3.2 Implement `syncRun()` — parse `--skills`, `--config`, `--prompts`, `--commands`, `--all`, `--dry-run` flags; resolve adapter matching installRun (opencode → claude → qwen priority, `--agent` override)
- [x] 3.3 Delegate to exported install functions per selected categories; print plan on dry-run, exit 0

## Phase 4: Integration Tests

- [x] 4.1 End-to-end sync test using `plugintest.FakeAgent` — verify each category flag calls the correct deploy function
- [x] 4.2 Verify dry-run mode reports counts without filesystem writes
