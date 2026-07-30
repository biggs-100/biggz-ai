# Design: sync-config

## Technical Approach

Add `WriteFileAtomic` with content comparison + `WriteResult` to `internal/filemerge/writer.go`, upgrade `MergeJSONC` in `json_merge.go` to recursive deep merge with `__replace__` sentinel, and add `biggz sync` CLI command that reuses exported install deploy functions. Sync orchestration lives in `syncRun()` inside `cmd/biggz/main.go`, matching every existing subcommand pattern. All writes go through `WriteFileAtomic`.

## Architecture Decisions

### Decision: WriteFileAtomic as new function, not replacement
| Option | Tradeoff | Decision |
|--------|----------|----------|
| New function `WriteFileAtomic` alongside `WriteFile` | Backward compat, no caller changes outside this change | ✅ **Chosen** |
| Replace `WriteFile` signature | Breaks all callers including unrelated ones | ❌ Rejected |
**Rationale**: Spec requires `(WriteResult, error)` return. Adding a new function avoids touching `section.go` and unrelated tests.

### Decision: No parent-dir auto-creation in WriteFileAtomic
**Choice**: Return error when parent dir doesn't exist.
**Rationale**: Per spec (scenario: Non-existent parent directory). All callers already do `os.MkdirAll` before writing — consistent with existing `WriteFile` contract.

### Decision: Deep merge replaces arrays, recurses into maps
**Choice**: Arrays in overlay replace existing entirely; nested maps merge recursively. `__replace__: true` inside a merged object replaces the target entirely (key stripped from output).
**Rationale**: Array-element merge has no universal semantics. Recursive map merge is the expected "deep merge" across config tools. `__replace__` provides explicit opt-out from merge depth.

### Decision: syncRun() in main.go, deploy functions exported from install
| Option | Tradeoff | Decision |
|--------|----------|----------|
| `syncRun()` in main.go, export deploy functions | Matches existing pattern (installRun, updateRun, doctorRun) | ✅ **Chosen** |
| New `internal/sync/` package | More complexity, no callers outside CLI | ❌ Rejected |
**Rationale**: All subcommand orchestration lives in `cmd/biggz/main.go`. The deploy functions (`deploySkills`, `mergeConfig`, `writeCommands`, `deployPrompts`) are already standalone — just need exported names.

### Decision: sync reuses adapter priority from install
**Choice**: `syncRun()` accepts `--agent` + `--home` flags (same as install). Falls back to opencode → claude → qwen priority. No binary detection — only path resolution.
**Rationale**: Sync deploys to an already-installed agent. Adapter provides `SkillsDir`, `GlobalConfigDir`, `SettingsPath`. Using plugintest.FakeAgent enables testability.

## Data Flow

```
biggz sync --skills --config --dry-run
       │
       ▼
   syncRun() ── flags ──► select categories & dry-run mode
       │
       ├── skills ──► install.DeploySkills(skillsDir, assets.FS, dryRun)
       │                   │
       │                   ▼  per file
       │              filemerge.WriteFileAtomic(path, data, 0644)
       │                   │
       │                   ▼
       │              WriteResult{Changed, Created} → count
       │
       └── config ──► install.DeployConfig(settingsPath, assets.FS, dryRun)
                           │
                           ▼
                      filemerge.MergeJSONC(existing, overlay)
                           │
                           ▼
                      filemerge.WriteFileAtomic(path, merged, 0644)
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/filemerge/writer.go` | Modify | Add `WriteResult` struct + `WriteFileAtomic` with content comparison |
| `internal/filemerge/writer_test.go` | Modify | Add tests for content-skip, new file, overwrite, non-existent parent |
| `internal/filemerge/json_merge.go` | Modify | Upgrade `MergeJSONC` to recursive merge + `__replace__` sentinel |
| `internal/filemerge/json_merge_test.go` | Modify | Add tests for deep merge, arrays, `__replace__` |
| `cmd/biggz/main.go` | Modify | Add `case "sync"` dispatch + `syncRun()` function |
| `internal/install/install.go` | Modify | Export deploy functions; update `WriteFile` → `WriteFileAtomic` |
| `internal/install/install_test.go` | Modify | Update test callers for exported names |

## Interfaces / Contracts

```go
// internal/filemerge/writer.go

type WriteResult struct {
    Changed bool  // content was different from existing
    Created bool  // file did not exist before
}

func WriteFileAtomic(path string, content []byte, perm os.FileMode) (WriteResult, error)
```

```go
// internal/install/install.go (exports)

func DeploySkills(skillsDir string, ffs fs.FS, dryRun bool) (int, error)
func DeployConfig(settingsPath string, ffs fs.FS, dryRun bool) (bool, error)
func DeployCommands(commandsDir string, ffs fs.FS, dryRun bool) (int, error)
func DeployPrompts(promptsDir string, ffs fs.FS, dryRun bool) error
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | WriteFileAtomic | Content-skip, new file, overwrite, non-existent parent (table-driven, follows existing `writer_test.go` style) |
| Unit | MergeJSONC deep merge | Nested merge, array replacement, `__replace__`, empty overlay, invalid JSON (table-driven) |
| Integration | syncRun | Dry-run reports counts without writes; sync calls each exported deploy function for selected flags |
| Existing | Install tests | Update to use exported names + `WriteFileAtomic`; no behavior change |

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary in this change.

## Migration / Rollout

Replace `WriteFile` calls in `install.go` with `WriteFileAtomic`. Old `WriteFile` stays in `filemerge` package — unused after this change but not removed. No migration needed.

## Open Questions

- [ ] Should `syncRun()` try adapters in priority order (silent) or require `--agent`? Decision: priority order with `--agent` override, matching install behavior.
