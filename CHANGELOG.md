# Changelog

All notable changes to biggz-ai are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased] — 2026-08-28

### Added
- **parity-gentle-v25** — 6 invariants fail-closed: budget 1/1 (`MaxFixRounds=1`, `MaxScopedValidations=1`), FixDelta `domainHash("fix-delta/v1\x00"+writeLengthPrefixed(...))` + `EmptyFixDeltaHash`, chain `domainHash` + `writeLengthPrefixed` (u32 BE) for `review-evidence/v1`/`review-merkle/v1`/`review-snapshot/v1`, store `GitCommonDir` (`--git-common-dir` fallback `--git-dir`) + `v1/events/<sha256>` + `publishImmutable` + dual-read, `flock(LOCK_EX|LOCK_NB)` on `.lock` + stale 5m PID+mtime, `BurnEnabled` + `burned.json` → `DeliveryBurned`, SDD V2 authority-free (`biggz-ai.sdd-status/v2` omits `granted_roots`/`missing_roots`/`edit_authority_blocked`). 3 PRs stacked `0587db9`/`4f091d4`/`87c93dd` (task refs `c72cd17`/`961ced6`/`fb27fdf`), archive `42cd08c` (specs `core-review`/`review`/`sdd-status`).
- **complete-subagent-report** — template 4 required + 6 optional (Preview/Diff/Decisions/Commands/Validation/Failure, omit-empty), `ReadLoop` >50KB paginated `ReadAt` with verify-retry, `ValidateQuestionEnvelope` 16/60/4/2-4, pending dual-write `biggz-ai.pending-question/v1` (BigMem + `state.yaml`, equality retry, compaction reload), checkpoint-only gate (`currentTurnMarkdown` strict, `BIGGZ_ADVISE=1` thin `concern`). 3 PRs stacked `f819f4e`/`48f18d0`/`e64a443` (task refs `a5b1afd`/`867d54b`/`dff57bd`), archive `3c8a247` (specs `orchestrator`/`pi-integration`).

### Fixed
- **fix-budget-accounting** — `PersistedReceipt` cumulative ledger: `CumulativeCorrectionLines` + `FixDeltaHash` hash-bound via `computeHash` (`domainHash("biggz-ai.review-receipt-binding/v1")`), `deriveNextTransition` deducts `max(0, budget - cumulative)` via `cumulativeLinesViaReceipt`, `ValidateCorrectionActual` wired, `finalizeIdempotent`/`verify_retry`/`reconcile` continuity, legacy `0`/`EmptyFixDeltaHash` compat. 805 lines code + 440 test (1283 total). Commit `e783985`, archive `2026-08-28-fix-budget-accounting`.

### Changed
- **docs P0** — `b33cf4f` aligns `docs/architecture.md`, `docs/validation-guide.md`, `openspec/specs/review-authority/spec.md` to v1 event store (`GitCommonDir`, `domainHash`, 1/1, `flock`, `Burn`, `acquire`/`settle`).

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
