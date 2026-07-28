# Design: Agent Integration and Install

## Technical Approach

**Thin Mirror of gentle-ai**: mirror the proven architecture at simplified scale — OpenCode only, SDD skills only, single install command. Keep `AgentAdapter` interface in `plugin/` (existing spec location), put implementations in `internal/`. The `homeDir` parameter on path methods enables filesystem testing without touching real user configs.

Six deliverable units: (1) extend interface, (2) OpenCode adapter, (3) embedded assets, (4) file merge library, (5) install orchestrator, (6) CLI subcommand.

## Architecture Decisions

| Decision | Choice | Alternatives | Rationale |
|----------|--------|-------------|-----------|
| Interface location | `plugin/` (extend) | Move to `internal/agents/` | Existing spec defines it in `plugin/`; additive methods don't break callers |
| Path method signature | `(homeDir string) string` | Implicit `os.UserHomeDir()` | Explicit parameter makes tests deterministic via `t.TempDir()` |
| Asset strategy | Single `//go:embed` FS | Per-agent dirs | Only OpenCode exists; one FS avoids N embed calls |
| JSONC merge | Strip comments→decode→merge→re-encode | Preserve comments | Preserving comments requires AST-level parsing; MVP doesn't need it |
| Install in `main.go` | Simple `os.Args` gate | Cobra/flags lib | Zero new dependencies; `--dry-run` is the only flag |

## Data Flow

```
biggz install [--dry-run]
    │
    ├─ os.Args[1] == "install"? ──→ installRun()
    │
    ├─ 1. Detect: exec.LookPath("opencode")
    │     ├─ not found → print error, exit 1
    │     └─ found → save binary path
    │
    ├─ 2. Create opencode.Adapter
    │
    ├─ 3. install.Run(ctx, adapter, config)
    │     ├─ GlobalConfigDir(homeDir)  → ~/.config/opencode/
    │     ├─ SkillsDir(homeDir)        → ~/.config/opencode/skills/
    │     ├─ SettingsPath(homeDir)     → ~/.config/opencode/opencode.jsonc
    │     ├─ Deploy skills: copy embedded FS → skills dir
    │     ├─ Merge JSONC: read → merge overlay → atomic write
    │     └─ Dry-run: print plan, write nothing
    │
    └─ 4. Print Result, exit 0
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `plugin/interfaces.go` | Modify | Add `GlobalConfigDir`, `SkillsDir`, `SettingsPath` to `AgentAdapter` |
| `internal/agents/opencode/adapter.go` | Create | OpenCode adapter (detect, caps, paths, deploy) |
| `internal/agents/opencode/adapter_test.go` | Create | Tests via `FakeAgent` + temp dirs |
| `internal/assets/embed.go` | Create | `//go:embed all:skills all:opencode` |
| `internal/assets/skills/` | Create | 12 SDD skill stubs + `_shared/` refs |
| `internal/assets/opencode/` | Create | `sdd-overlay-single.json`, `sdd-overlay-multi.json`, `commands/` |
| `internal/filemerge/writer.go` | Create | Atomic write: temp → rename |
| `internal/filemerge/writer_test.go` | Create | Concurrent safety, partial write recovery |
| `internal/filemerge/json_merge.go` | Create | JSONC strip → decode → merge → encode |
| `internal/filemerge/json_merge_test.go` | Create | Merge, replace, edge cases |
| `internal/filemerge/section.go` | Create | Marker-based markdown injection |
| `internal/filemerge/section_test.go` | Create | Insert, replace, noop cases |
| `internal/install/install.go` | Create | `Run(ctx, adapter, Config) → Result` |
| `internal/install/install_test.go` | Create | Dry-run, deploy, idempotent via FakeAgent + TempDir |
| `plugintest/agent.go` | Modify | Add `SetTempDir`, `tempDir` field, path methods |
| `plugintest/agent_test.go` | Create | Verify paths resolve under TempDir |
| `cmd/biggz/main.go` | Modify | Add `install` subcommand gate |

## Interfaces / Contracts

**Non-obvious pattern — atomic write** (all writes safe under crash/failure):

```go
func WriteFile(path string, content []byte, perm os.FileMode) error {
    tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
    if err != nil { return err }
    defer os.Remove(tmp.Name())

    if _, err := tmp.Write(content); err != nil { return err }
    if err := tmp.Close(); err != nil { return err }

    return os.Rename(tmp.Name(), path)  // atomic on same filesystem
}
```

**Non-obvious pattern — JSONC merge** (strip non-standard syntax, merge top-level keys):

```go
func MergeJSONC(existing, overlay []byte) ([]byte, error) {
    existing = stripComments(stripTrailingCommas(existing))
    overlay  = stripComments(stripTrailingCommas(overlay))

    var base, patch map[string]any
    json.Unmarshal(existing, &base)  // err handled
    json.Unmarshal(overlay, &patch)

    for k, v := range patch {
        base[k] = v  // top-level key replacement
    }
    return json.MarshalIndent(base, "", "  ")
}
```

## Testing Strategy

| Layer | What | How |
|-------|------|-----|
| Unit — filemerge | Atomic write, JSON merge, section injection | Temp dirs, table-driven, edge cases (empty, nil, concurrent) |
| Unit — adapter | Detection, paths, caps | `plugintest.FakeAgent` with `SetTempDir(t.TempDir())` |
| Unit — install | Dry-run, deploy, idempotent | FakeAgent + real filesystem under `t.TempDir()` |
| Integration | `cmd/biggz` install path | Build binary, run with `--dry-run` |

## Threat Matrix

N/A — no shell execution beyond `exec.LookPath`, no subprocess management, no VCS/PR automation, no executable-file classification, no process-integration boundary. `exec.LookPath` is a read-only PATH search, documented safe by Go stdlib.

## Migration / Rollout

No migration required. All `internal/` packages are greenfield. Interface changes are additive — existing code compiles unchanged. The `install` subcommand is opt-in (only triggers when `os.Args[1] == "install"`).

## Open Questions

- [ ] Should the skill stubs contain full skill content or just placeholder headers? Decision: MVP stubs with headers only; refine later.
- [ ] Does `opencode.jsonc` use trailing commas in practice? Verified yes — JSONC merge must strip them.
