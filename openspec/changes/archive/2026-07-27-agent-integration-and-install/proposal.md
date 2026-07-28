# Proposal: Agent Integration and Install

## Intent

Detect installed AI coding agents and deploy biggz-ai's SDD skills + config. `AgentAdapter` exists but lacks implementations, path methods, assets, file merge, and install command. This delivers a working `biggz install` for OpenCode.

## Scope

### In Scope
- Extend `plugin.AgentAdapter` with config path methods (GlobalConfigDir, SkillsDir, SettingsPath)
- OpenCode adapter in `internal/agents/opencode/` — detection via `exec.LookPath`, config via `os.UserHomeDir`
- Asset embedding in `internal/assets/` — `//go:embed` for 12 SDD skills + opencode overlay
- File merge in `internal/filemerge/` — atomic write, JSON/JSONC merge, markdown section injection
- `biggz install` subcommand with `--dry-run` flag
- Extend `plugintest.FakeAgent` with temp dir support
- All tests via temp dirs, no real agent touched

### Out of Scope
- Multi-agent (Claude, Cursor, etc.)
- Uninstall, upgrade, backup
- Multi-profile or multi-mode install

## Capabilities

### New
- `agent-install`: detect agents and deploy biggz-ai SDD skills/config

### Modified
- `plugin-system`: extend `AgentAdapter` interface with config path methods (additive)

## Approach

**Thin Mirror** — gentle-ai's architecture at simplified scale:

| Layer | Package | Key Detail |
|-------|---------|------------|
| Adapter interface | `plugin/` (extend) | Add GlobalConfigDir, SkillsDir, SettingsPath |
| OpenCode adapter | `internal/agents/opencode/` | `exec.LookPath`, `os.UserHomeDir()/.config/opencode/` |
| Assets | `internal/assets/` | `//go:embed all:assets/` — 12 SDD skills + overlay |
| File merge | `internal/filemerge/` | Atomic writer, JSON/JSONC merge, markdown inject |
| Install | `internal/install/` | Detection → deploy, `--dry-run` |
| CLI | `cmd/biggz/main.go` | `install` subcommand |
| Testing | `plugintest/` | FakeAgent with `TempDir` support |

Interface stays in `plugin/`. Only OpenCode. No manifest digest machinery.

## Risks

| Risk | Mitigation |
|------|------------|
| Feature creep (multi-agent) | Gate: OpenCode only, no profiles |
| File merge data loss | Port tests alongside code; handle Windows NTFS |
| JSONC schema drift | Strip comments + trailing commas pre-merge |
| Platform path issues | `os.UserHomeDir()` + `filepath.Join` everywhere |

## Rollback Plan

Greenfield `internal/` packages — safe to `git revert <commit>`. Interface changes to `plugin.AgentAdapter` are additive (new methods only) — existing code compiles. If interface change breaks build, revert the commit and update any callers in the same revert.

## Success Criteria

- [ ] `biggz install` detects OpenCode when installed; `--dry-run` reports without writing
- [ ] SDD skills deployed to agent skills dir, JSONC config merged with overlay
- [ ] All tests pass via temp dirs, no real agent touched
- [ ] `plugintest.FakeAgent` supports temp dirs for filesystem tests
- [ ] Existing `AgentAdapter` callers compile unchanged (additive methods only)
