# Exploration: Agent Integration and Install

## Current State

**biggz-ai** is an early-stage Go project rebuilding gentle-ai from scratch. It currently has:
- Core domain model (`model/` — FSM, hash chain, review types)
- Orchestrator (`orchestrator/` — pipeline lifecycle with status transitions)
- Pipeline (`pipeline/` — sequential stage execution with rollback)
- Plugin interfaces (`plugin/` — `LensPlugin` and `AgentAdapter` interfaces, `Capability` types, `AgentConfig`)
- Policy evaluator (`policy/`)
- Build-time registry (`registry/` — `RegisterLens`, `RegisterAgent`, `GetLens`, `GetAgent`)
- CLI entry point (`cmd/biggz/main.go` — stdin/stdout pipeline, no install/deploy subcommand)
- SDD config (`openspec/config.yaml`)
- Specs for CLI and plugin system (`openspec/specs/cli/spec.md`, `openspec/specs/plugin-system/spec.md`)

**What does NOT exist yet:**
- No `internal/` package hierarchy — everything is at the project root
- No asset embedding (`//go:embed`) or embedded skill files
- No agent adapter implementations (the `AgentAdapter` interface exists but no concrete adapters)
- No install or deploy CLI command
- No agent capability manifest or feature claims
- No file merge component (atomic writes, JSON/JSONC merge, markdown section injection)
- No agent discovery (iterating adapters to find installed runtimes)
- No agent ID constants or support tiers in the model package
- No MCP configuration or slash command management

**gentle-ai reference patterns** (the mature implementation):
- `internal/agents/` — rich adapter interface with 20+ methods covering identity, detection, config paths, MCP, skills, sub-agents, and system prompts
- `internal/agents/opencode/adapter.go` — 210+ line adapter for OpenCode with detection via `exec.LookPath("opencode")`, config paths for `~/.config/opencode/`, capability manifest integration
- `internal/agents/capabilitymanifest/manifest.go` — canonical per-agent feature matrix with 16 agents, implementation routing facts, contract claims, digest support
- `internal/agents/discovery.go` — `DiscoverInstalled()` iterates registry adapters, calls `Detect()` on each
- `internal/assets/assets.go` — single `//go:embed all:...` FS with 15+ agent dirs, skill files, commands, overlays
- `internal/components/sdd/inject.go` — 775-line injector that writes skills, slash commands, JSON overlay, system prompts, profiles, workflows, sub-agents, post-verification
- `internal/components/filemerge/` — atomic writer, JSON/JSONC merge with sentinel support, markdown section injection with marker parsing, YAML/TOML readers
- `internal/installcmd/resolver.go` — platform-aware install command resolution (brew, npm, apt, winget)
- `internal/cli/install.go` — install CLI flags parsing (`--agent`, `--skill`, `--component`, `--dry-run`, `--scope`)

## Affected Areas

### New packages to create:

| Package | Path | Description |
|---------|------|-------------|
| Agent interfaces | `internal/agents/` | Adapter interface, capability manifest, discovery, registry |
| OpenCode adapter | `internal/agents/opencode/adapter.go` | Concrete adapter for OpenCode agent |
| Asset embedding | `internal/assets/` | `//go:embed` for SDD skills, commands, overlays, shared refs |
| File merge | `internal/filemerge/` | Atomic write, JSON merge, markdown section injection |
| Install cmd | `internal/install/` | Install/deploy orchestration (flags, resolver, execution) |
| Agent IDs | `internal/model/` (extend) | AgentID constants, SupportTier, Capability types |

### Existing packages to modify:

| Package | Change |
|---------|--------|
| `cmd/biggz/main.go` | Add `install`/`deploy` subcommand |
| `plugin/interfaces.go` | Expand `AgentAdapter` with config path methods (or migrate to `internal/agents/`) |
| `registry/` | Add agent registry support (or delegate to `internal/agents/`) |
| `go.mod` | Possibly add `embed` import (stdlib, no dep change) |

### Embedded asset directories to create:

| Path | Content |
|------|---------|
| `assets/skills/_shared/` | `SKILL.md`, `sdd-phase-common.md`, `persistence-contract.md`, `openspec-convention.md` |
| `assets/skills/sdd-init/` | Init phase skill |
| `assets/skills/sdd-explore/` | Explore phase skill |
| `assets/skills/sdd-propose/` | Propose phase skill |
| `assets/skills/sdd-spec/` | Spec phase skill |
| `assets/skills/sdd-design/` | Design phase skill |
| `assets/skills/sdd-tasks/` | Tasks phase skill |
| `assets/skills/sdd-apply/` | Apply phase skill |
| `assets/skills/sdd-verify/` | Verify phase skill |
| `assets/skills/sdd-archive/` | Archive phase skill |
| `assets/skills/sdd-onboard/` | Onboard phase skill |
| `assets/opencode/commands/` | Slash command `.md` files for /sdd-* |
| `assets/opencode/` | `sdd-overlay-single.json`, `sdd-overlay-multi.json` |

## Approaches

### Approach 1: Thin Mirror — follow gentle-ai's structure with simplification

Gently mirror the mature architecture from gentle-ai but strip what biggz-ai does not need: only OpenCode adapter (not 16 agents), only SDD skills (not general-purpose skills), only the install command (not upgrade/backup/restore).

- **Package layout**: `internal/agents/`, `internal/assets/`, `internal/filemerge/`, `internal/install/`
- **Adapter interface**: Simplified version of gentle-ai's 20-method Adapter, focused on what biggz-ai needs
- **Asset embedding**: Single `//go:embed` for skills + opencode configs, no multi-agent assets
- **Install command**: `biggz install` with `--dry-run` flag, detects OpenCode, deploys skills + configs

**Pros:**
- Proven architecture — gentle-ai has 2+ years of production use
- Clean separation of concerns (agents, assets, filemerge, install)
- Easy to add future agents (Claude Code, Cursor, etc.) without structural changes
- `filemerge` component is independently useful for future features
- Follows Go convention of `internal/` packages for encapsulation

**Cons:**
- More initial code than strictly needed for MVP
- Risk of over-engineering if OpenCode is the only agent ever targeted
- File merge component is ~800 lines of subtle filesystem logic (reuse from gentle-ai wholesale vs rewrite?)

**Effort**: High (2000-3000 lines across packages)

### Approach 2: Minimal First — only implement what's strictly needed

Collapse everything into a single `installer` package or even the `cmd/biggz/` package. Hardcode OpenCode paths, embed only the most critical skills (init, explore, propose, apply). Skip the full adapter interface, capability manifest, and multi-agent patterns.

- **Package layout**: `internal/installer/` or just `cmd/biggz/install.go`
- **Adapter**: Inline OpenCode detection (`exec.LookPath("opencode")`, `os.Stat(~/.config/opencode)`)
- **Asset embedding**: Minimal — only the skills biggz needs for its own SDD pipeline
- **Install command**: Short script-like function that writes skills and configs

**Pros:**
- Fast to implement (500-800 lines)
- No unused abstractions
- Easy to understand and debug
- Gets an MVP working quickly

**Cons:**
- Technical debt if more agents are added later
- Hardcoded paths and patterns will need refactoring
- The existing `plugin.AgentAdapter` interface was designed for extensibility — ignoring it violates the existing architecture
- No file merge means destructive writes (overwrite instead of merge opencode.json)
- No capability manifest means capability checks are ad-hoc

**Effort**: Low (500-800 lines)

### Approach 3: Adapter-Only — implement the adapter layer, defer install and assets

Focus on completing the `AgentAdapter` interface and implementing the OpenCode adapter properly. Register it in the registry, implement detection and capability manifest. Defer the install command, asset embedding, and file merge to a later change.

- **Package layout**: `internal/agents/` + `internal/agents/opencode/`
- **Adapter**: Full interface with detection, config paths, capability manifest
- **Assets**: Not embedded yet
- **Install command**: Not implemented yet

**Pros:**
- Clean, focused scope
- Foundation for everything else
- The existing `plugin.AgentAdapter` interface gets a real implementation
- Agent registry becomes functional

**Cons:**
- Incomplete — users cannot actually install anything
- Artifacts in `assets/` and `install/` need separate SDD changes
- No immediate user-visible value
- Risk of interface drift if install is implemented much later

**Effort**: Medium (800-1200 lines)

## Recommendation

**Approach 1: Thin Mirror** — with a specific simplification strategy:

1. **Adapter interface**: Start with a compact version (not the full 20-method gentle-ai interface) that covers: Identity (ID, Name), Detection (Detect), Paths (GlobalConfigDir, SkillsDir, SettingsPath), Capabilities (Capabilities), and Deploy (DeployConfig). This is closer to the existing `plugin.AgentAdapter` interface but with the path methods added.

2. **Capability manifest**: Implement a simplified version — a static map of agent ID → feature set, without the digest, validation, and contract claim machinery. Add it to the model package.

3. **Asset embedding**: Embed only SDD skills (12 skills) and the OpenCode overlay JSON. No Claude, Cursor, Windsurf, or other agent assets. At ~150KB total this is easy to maintain.

4. **File merge**: Port the core three sub-packages from gentle-ai: `writer.go` (atomic write), `json_merge.go` (JSON/JSONC merge), and `section.go` (markdown section injection). These are well-tested and independently useful.

5. **Install command**: `biggz install` that detects OpenCode, checks if skills are already deployed, writes missing ones, merges the SDD overlay into `opencode.json`, and writes slash commands. Support `--dry-run`.

6. **Code location for AgentAdapter**: The existing `plugin/interfaces.go` `AgentAdapter` is a good starting point but needs more methods. Either extend it there or migrate to `internal/agents/`. Recommendation: keep the interface in `plugin/` since that's where the spec defines it, but move the adapter implementations to `internal/agents/opencode/`.

This approach gives biggz-ai a deployable install experience in this change while keeping the door open for future agents. The file merge component pays immediate dividends for all future file-writing features.

## Risks

1. **Feature creep** — The thin mirror could expand to include more than needed (multi-mode, profiles, model assignments, runtime agents, pluggable adapters). Scope gate: implement ONLY the OpenCode adapter, ONLY single-mode SDD, and ONLY the `install` command (not `install --agent`, `install --profile`, etc.).

2. **Asset maintenance burden** — SDD skills in gentle-ai evolve. biggz-ai needs its own copy. If gentle-ai restructures its skills, biggz-ai needs to catch up. Mitigation: biggz-ai should own its skill copies and treat them as versioned documents, not symlinks to gentle-ai.

3. **File merge complexity** — The atomic writer has subtle OS-specific behavior (Windows NTFS directory sync, symlink parent detection, permission relaxation). Porting this incorrectly can cause data loss. Mitigation: port the tests alongside the implementation, and handle Windows paths correctly from day 1.

4. **OpenCode JSONC schema changes** — OpenCode's `opencode.json`/`opencode.jsonc` format could change. The JSONC comment stripping and merge logic must tolerate both formats. Mitigation: use the same normalization approach as gentle-ai (strip comments → strip trailing commas → strict JSON decode).

5. **Existing `plugin.AgentAdapter` interface** — Changing this interface breaks the existing contract defined in `openspec/specs/plugin-system/spec.md`. Any changes must be reflected there. Mitigation: do not remove existing methods, only add new ones, and update the spec in the same commit.

6. **Path handling across platforms** — `~/.config/opencode/` resolution differs on Windows (%USERPROFILE%\\.config\\opencode\\), macOS, and Linux. Must use `os.UserHomeDir()` + `filepath.Join()`. Mitigation: test on all three platforms or at minimum document the constraint.

## Ready for Proposal

**Yes** — the exploration is complete and the approach is clear.

The orchestrator should tell the user: "We explored the agent integration and install domain by studying both the existing biggz-ai codebase and the mature gentle-ai reference implementation. The recommended approach is to mirror gentle-ai's architecture at a simplified scale: create `internal/agents/`, `internal/assets/`, `internal/filemerge/`, and `internal/install/` packages, implement only the OpenCode adapter, embed only SDD skills, and deliver a working `biggz install` command. The proposal phase should scope this precisely to avoid feature creep."

Required next phases: **proposal → spec → design → tasks → apply → verify → archive**
