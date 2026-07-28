# Apply Progress: Agent Integration and Install — PR #2

**Batch**: File Merge Library (Phase 3)
**Mode**: Standard (strict_tdd: false)
**Delivery**: auto-chain, stacked-to-main, PR #2 of 3
**Previous**: PR #1 — Interface + Adapter + Assets (completed)

## Completed Tasks

### Phase 1: Interface & Adapter

- [x] 1.1 Extended `plugin/interfaces.go` — added `GlobalConfigDir`, `SkillsDir`, `SettingsPath(homeDir string) string` to `AgentAdapter` interface
- [x] 1.2 Created `internal/agents/opencode/adapter.go` — OpenCode adapter with `exec.LookPath` detection, 5 capabilities, path methods, and MVP `DeployConfig` no-op
- [x] 1.3 Created `internal/agents/opencode/adapter_test.go` — 8 tests covering detection (found/not-found), capabilities, all 3 path methods, and deploy config
- [x] 1.4 Extended `plugintest/agent.go` — added `tempDir` field, `SetTempDir(dir)`, and path methods that route under tempDir when set
- [x] 1.5 Created `plugintest/agent_test.go` — 10 tests covering defaults, detect, all 3 path methods with/without tempDir, set-temp-dir-twice

### Phase 2: Assets

- [x] 2.1 Created `internal/assets/embed.go` — `//go:embed all:skills all:opencode` with embed.FS export
- [x] 2.2 Created 12 SDD skill stubs under `internal/assets/skills/`:
  - `sdd-init`, `sdd-explore`, `sdd-propose`, `sdd-spec`, `sdd-design`, `sdd-tasks`,
  - `sdd-apply`, `sdd-verify`, `sdd-archive`, `sdd-onboard`, `sdd-new`, `sdd-ff`
- [x] 2.3 Created `internal/assets/skills/_shared/SKILL.md` and `_shared/sdd-phase-common.md`
- [x] 2.4 Created `internal/assets/opencode/sdd-overlay-single.json` and `sdd-overlay-multi.json`
- [x] 2.5 Created `internal/assets/opencode/commands/` — 9 slash command .md files for `/sdd-*`

### Phase 3: File Merge

- [x] 3.1 Created `internal/filemerge/writer.go` — `WriteFile(path, content, perm)` with atomic temp → rename pattern, `os.Chmod` for permissions
- [x] 3.2 Created `internal/filemerge/writer_test.go` — 5 tests: basic write, non-existent dir error, concurrent writes, permissions (skipped on Windows), failed write preserves original
- [x] 3.3 Created `internal/filemerge/json_merge.go` — `MergeJSONC(existing, overlay)`, `stripComments()` (handles `//`, `/* */`, escapes inside strings), `stripTrailingCommas()`
- [x] 3.4 Created `internal/filemerge/json_merge_test.go` — 8 tests: basic merge, overlay replaces keys, strip comments, strip trailing commas, comments in strings preserved, empty overlay, invalid JSON cases, full JSONC round-trip
- [x] 3.5 Created `internal/filemerge/section.go` — `InjectSection(content, name, newSection)` appends marker-delimited section; `ReplaceSection(content, name, newSection)` replaces content between `<!-- section:name -->` and `<!-- /section -->` markers
- [x] 3.6 Created `internal/filemerge/section_test.go` — 6 tests: inject into empty, inject with existing sections, replace existing, missing section error, missing closing marker error, content without trailing newline

### Side Fix

- [x] Extended `registry/registry_test.go` — added `GlobalConfigDir`, `SkillsDir`, `SettingsPath` to `mockAgent` to satisfy updated interface

## Work Unit Evidence — PR #2

| Evidence | Required value | Result |
|----------|----------------|--------|
| Focused test command | `go test ./internal/filemerge/...` | PASS — 16 tests (15 PASS + 1 SKIP on Windows for permissions) |
| Runtime harness | N/A — library only; no runtime boundary | N/A |
| Rollback boundary | `internal/filemerge/` directory | All files are new; simple directory delete |

## Full Test Suite (PR #2 final state)

`go test ./...` — ALL PASS across all 10 packages

`go vet ./...` — CLEAN

## Deviations from Design

- Added `os.Chmod(path, perm)` to `WriteFile` after rename — design signature includes `perm` but pseudo-code omitted its usage; needed for the permissions test
- Added escape-sequence handling in `stripComments` — design didn't detail it, but required to correctly preserve `\"` and `\\` inside JSON strings (caught by `TestMergeJSONC_CommentsInStrings`)
- `InjectSection` uses `string` for content and `[]byte` for section content; design signature was ambiguous on types

## Issues Found

- Windows does not support Unix file permissions (`os.Chmod` is a no-op on Windows); the `TestWriteFile_Permissions` test is conditionally skipped on `runtime.GOOS == "windows"`
- Concurrent writes to the same file are safe under atomic rename (some goroutines' `os.Rename` calls will fail, but the temp file cleanup via `defer os.Remove` handles that; at least one writer always succeeds)

## Files Changed (PR #2 only)

| File | Action | What Was Done |
|------|--------|---------------|
| `internal/filemerge/writer.go` | Created | Atomic file write via temp → rename |
| `internal/filemerge/writer_test.go` | Created | 5 tests for writer |
| `internal/filemerge/json_merge.go` | Created | JSONC merge with comment/commma stripping |
| `internal/filemerge/json_merge_test.go` | Created | 8 tests for JSONC merge |
| `internal/filemerge/section.go` | Created | Marker-based markdown section injection |
| `internal/filemerge/section_test.go` | Created | 6 tests for section operations |
| `openspec/changes/agent-integration-and-install/tasks.md` | Modified | Marked Phase 3 tasks 3.1–3.6 as complete |

## Remaining Tasks (for PR #3)

- Phase 4: Install command + wiring (4.1–4.3)
- Phase 5: Verify (5.1–5.2)

## Risks

- None specific to PR #2. All changes are new files under `internal/filemerge/`. No existing code was modified (only `tasks.md` was updated). Rollback is a simple directory delete.
