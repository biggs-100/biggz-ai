# Changelog

All notable changes to biggz-ai are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [1.0.0] — 2026-07-29

### Added
- Recovery trace system: CLI (`biggz recovery list/show/generate/validate/export/import/delete`) + SQLite store + TUI screen
- Dashboard TUI screen with memory stats, project list, and quick actions
- Session detail TUI view showing prompts, metadata, and observations
- BigMem CLI: engram-compatible commands (`delete session/prompt/project`, `conflicts stats/deferred`, `projects consolidate`, `doctor --json`, `version`, `help`)
- BigMem sync: flag-based interface (--import/--status/--project/--all), export to `.bigmem/`, auto-detect project from git root
- BigMem save/search: engram-compatible flags (--type, --project, --scope, --limit)
- Install: auto-add `~/.biggz/` to USER PATH on Windows
- Conflict prevention: shared skills not deployed to agent dir to avoid gentle-ai collision
- LICENSE (MIT) and CONTRIBUTING.md

### Fixed
- MCP server: handle `initialize` method (required by MCP standard handshake)
- MCP server: normalize `required: null` to `[]` in tool schemas
- Skill deployment: always copy to agent's skills directory for OpenCode discovery
- Skill overlay: remove invalid `"skills"` top-level key from config JSON
- Skill registry: add `~/.biggz/skills/` to scan paths
- Frontmatter: add license + metadata to all 24 SKILL.md files

## [dev] — 2026-07-28

### Added
- Initial project scaffold
- Bubble Tea TUI with 12 screens (welcome, install, config, status, memory, backup, profile, upgrade, uninstall, strict TDD, review, sessions)
- CLI: install, sdd-status, sdd-attempt, sdd-continue, sdd-verify-validate, bigmem (save/search/get), backup, release, skill-registry, review, doctor, update, sync, rdd, mcp
- SDD workflow: proposal, spec, design, tasks, apply, verify, archive phases
- Review pipeline: 4 lenses (R1 Risk, R2 Readability, R3 Reliability, R4 Resilience) with DAG parallel execution
- Judgment Day: dual-blind adversarial review with refuter
- BigMem persistent memory: SQLite + FTS5 with 22 MCP tools
- Agent adapters: OpenCode, Claude Code, Qwen with automatic detection
- Install system: skill/config/command/persona/MCP deployment
- RDD kill switch (global/clone/worktree scope)
- Content-addressed review event store with SHA-256 evidence chain
- Filemerge package: JSONC-aware deep merge with atomic writes
- Doctor health check system with 11 checks
- Self-update mechanism with minisign verification
- E2E tests and validation guide
- GoReleaser cross-platform build pipeline
