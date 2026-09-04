# Changelog

All notable changes to biggz-ai are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased] — 2026-08-31

### Fixed
- **install-pi-packages-web-btw** (2026-09-04) — `biggz install --agent pi` now installs `npm:pi-web-access` (multi-provider search, URL/PDF extraction, video understanding, GitHub cloning) and `npm:pi-btw` (`/btw` parallel side conversations). Same InstallCommand pattern as `rpiv-todo`. Test: `TestInstallCommand_IncludesWebAndBtw`.
- **question-visible-context** (2026-09-04) — blocking questions must carry visible decision context: `ValidateQuestionEnvelope` now rejects options with empty descriptions fail-closed, `QuestionOption` gains `preview` (side-by-side detail, persisted verbatim by `FormatFallback`), and the orchestrator workflow mandates context-first in the human language before every question. The questionnaire modal shows only the envelope, so bare-label choices are no longer a representable state. Tests: missing-description (per-question + top-level), preview-in-fallback; existing fixtures migrated.
- **install-pi-todo-overlay** (2026-09-04) — `biggz install --agent pi` now installs `npm:rpiv-todo` (visual task-tracking overlay in pi's TUI, gentle-pi parity). Same pattern as `rpiv-ask-user-question`: via `InstallCommand` only. Test: `TestInstallCommand_IncludesTodoOverlay`.
- **install-pi-drop-startup-banner** (2026-09-04) — removed `startup-banner.ts` (animated gentle-pi banner) from `internal/assets/pi` and the pi deploy list per user request; install self-heals stale copies. Purely visual extension, no backend impact.
- **install-pi-subagents-j0k3r** (2026-09-04) — pi subagent dispatcher switched from `npm:pi-subagents` (nicobailon) to the maintained fork `npm:pi-subagents-j0k3r` (v1.5.13, j0k3r-dev-rgl). `InstallCommand`, `desiredPiPackages` reconciliation (predecessor entry now filtered as legacy) and `doctor` check/Remedy retargeted; install removes the predecessor package best-effort so both never load at once (duplicate `subagent_*` tools). Doctor warns with a migrate hint when only the legacy package is present. Tests: `TestInstallCommand_UsesJ0k3rFork`, `TestFilterPiPackages_DropsPredecessor`, `TestPiSubagentsCheck_{ForkPasses,LegacyWarnsMigrate,MissingWarns}`.
- **install-pi-unportable-extensions** (2026-09-04) — `biggz install --agent pi` no longer deploys `gentle-ai.ts`/`quiet-tools.ts`/`sdd-init.ts`: they import from `../lib/*`, a tree never ported into `internal/assets/pi`, and crashed pi on startup (`Cannot find module '../lib/sdd-preflight.ts'`). Install self-heals stale copies from existing `~/.pi/agent/extensions/`, and the dead sources (~344KB) were deleted from `internal/assets/pi`. Covered features remain via `biggz-synthesis-gate.js`, `biggz-tool-pills.js`, `skill-registry.ts` and the biggz review/SDD CLI. Test: `TestPiExtensionsStep_DropsUnportableExtensions`.
- **install-path-dedup** (2026-09-04) — `biggz install` no longer re-adds `~/.biggz` to the User PATH when the binary already resolves via PATH (e.g. GOBIN after `go install`), eliminating the recurring `doctor` duplicate-binary warning. New exported `install.BinaryOnPath()` scans PATH directly (SplitList + Stat, mirroring doctor's PathCheck) instead of `exec.LookPath`, which returns ErrDot without scanning PATH when `./biggz.exe` exists in the working directory (go.dev/issue/53536). The `~/.biggz` binary copy is still deployed (agent configs reference it by absolute path). Tests: `TestBinaryOnPath_{Found,Missing,IgnoresWorkingDirectory}`.

### Added
- **bigmem-rescue-ownership** (2026-08-28) — orphan BigMem session rescue with DryRun/scope handling (15 tasks, archived).
- **bigmem-sync-v2** (2026-08-28) — BigMem sync v2 parity with Engram (21 tasks, archived).
- **parity-gentle-69** PR1-3 (2026-08-28/29) — ledger verify hardening slice, `internal/review` budget/ledger fixes (9fab44e/062006d/6c8a969).
- **ola1-gentle-hardening** (2026-08-29) — 5-case detection + Batch S/M redaction, quarantine + lease, rescue-ownership journal
- **ola2-guardrails-preflight-synthesis** + **ola3-gentle-final-hardening** (2026-08-29) — guardrails preflight + synthesis gate hardening
- **gentle-safety-sealed-explorers** (2026-08-30) — 6/8/5 sealed explorers, archive ola1/ola2
- **gentle-model-bg-verify** (2026-08-30) — model routing BG verify + gentle-model parity (THINKING_LEVELS `biggz-ai.agent_model_routing/v1`)
- **sdd-sync** (2026-08-30) — SDD sync executor + deltas + RENAMED fix + skill registry watcher (fsnotify)
- **help-backup-tui** (2026-08-30) — help backup TUI + orchestrator synthesis scannable
- **codegraph-change-intent-full** (2026-08-30) — CodeGraph change-intent full graph (JSON + Markdown)
- **fix-orchestrator-checkpoint-synthesis** (2026-08-30) + **polish-wait-visuals** (2026-08-31) — checkpoint synthesis + wait visuals
- **rdd-cas-reach-parity** (2026-08-31) — RDD CAS reach parity (archived)
- **refactors** (2026-08-30/31) — extract helpers: `renderTable`, `ValidateSymlinkTarget`, `writeRDDModeCAS`, `status`/`synthesis`/`ParseDeltaSpec`/`ApplyDeltas`/`foreignRuntimeTopologyRoots`/`RDDStatus`/`SetWorktreeRDDMode`/`SetCloneLocalRDDMode`/`deriveSyncState`/`sync`
- **ui-port** (2026-09-01) — BigMem implicit session fix (`MostRecentActiveSession`/`EnsureImplicitSession`), 6 Engram parity prompts, orchestrator thin 792→68L + `biggz-synthesis-gate.js`, TUI theme engine + gallery + Pi pretty (tool-pills, powerline footer, Ask TabBar)

## [Unreleased] — 2026-08-28 (archived)

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
