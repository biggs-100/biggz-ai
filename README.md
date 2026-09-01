# biggz-ai

**AI Agent Harness — Review-Driven Development with Human-in-the-Loop**

biggz-ai is a lightweight, self-contained harness for AI coding agents (OpenCode, Claude Code, Qwen). It provides SDD (Spec-Driven Development) workflow orchestration, code review pipelines with 6 lenses (R1-R4 + performance, dependencies), persistent memory (BigMem), and full lifecycle management — all with the human always in control.

Inspired by gentle-ai, but rebuilt from scratch with ~40K filtered production Go lines (cmd/ + internal/) vs ~60K full-module prod (59,725 prod, 103,782 total) vs gentle-ai ~313K total (measured 2026-08-28, filtered ~58.4K prod) — see docs/comparison-with-gentle.md for method, a cleaner architecture, and no legacy debt.

## Quick Start

```bash
# Install in your AI agent
biggz install

# Check SDD status
biggz sdd-status

# Run a code review (content-addressed)
biggz review start --subject subject.json --lineage demo-001
# biggz review capture-result --lineage demo-001 --target <subject-id> --expected-revision <sha> --lens risk --order 1
# biggz review finalize --lineage demo-001
# biggz review gate --lineage demo-001
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `biggz` | Run review pipeline (content-addressed) |
| `biggz install` | Install skills + config in agent |
| `biggz uninstall` | Remove managed assets (keeps memory data unless `--purge`) |
| `biggz sdd-status [--cwd <dir>] [--json] [--instructions] [--watch] [--contract biggz-ai.sdd-status/v2]` | Show active/archived SDD changes (V2 authority-free) |
| `biggz sdd-verify-validate --input <path\|->` | Validate verify reports (`--requirements`/`--scenarios`, `--json`) |
| `biggz sdd-attempt acquire\|settle\|status\|grant\|begin\|finish\|reset` | Manage attempt budgets (CAS, tokens, grants) |
| `biggz sdd-continue [change]` | Determine next SDD phase (picker when omitted) |
| `biggz sdd-apply <change>` | Validate edit authority for the apply phase (guard, warn `blocked(edit_authority_missing)`) |
| `biggz sdd-new [change]` | Interactive SDD change wizard |
| `biggz sdd-profile list\|apply\|remove` | Manage SDD model profiles (default/balanced/quality/cheap) |
| `biggz sdd-remediate <change> [--verify-report <path>]` | Validate remediation result for a verify failure |
| `biggz bigmem save <title> <msg> [--type T] [--project P] [--scope S] [--topic-key K]` | Persistent memory core (save/search/get/delete/update/timeline/context/stats/doctor/export/import/rescue-ownership + graph, compare, conflicts, projects, sync, version, help) |
| `biggz bigmem graph [--project P] [--format dot\|ascii\|json] [--limit N] [--scope project\|all]` | Topic hierarchy and relations |
| `biggz bigmem compare <id-a> <id-b>` | Compare two observations |
| `biggz bigmem conflicts list\|show\|judge\|scan\|stats\|deferred` | Pending memory conflicts |
| `biggz bigmem projects list\|consolidate` | Project index and dedup |
| `biggz bigmem sync [--import] [--status] [--project P] [--all] [--from-engram] [--engram-dir PATH]` | Sync observations via `.bigmem/` |
| `biggz backup create\|list\|restore` | Snapshot/restore state |
| `biggz release status\|tag\|verify` | Version management |
| `biggz skill-registry refresh [--force] [--quiet]` | Regenerate skill registry |
| `biggz sync` | Deploy skills, config, prompts, and commands |
| `biggz update` | Check for updates (stable/beta channel) |
| `biggz upgrade` | Upgrade binary with signature verification |
| `biggz doctor [--json] [--fix]` | Run system health checks and auto-fix |
| `biggz tdd enable\|disable\|status` | Strict TDD mode (AGENTS.md marker) |
| `biggz plugin list\|install\|uninstall` | Manage OpenCode community plugins |
| `biggz mcp [--tools] [--prefix <name>]` | Start MCP server for agent memory tools (22 tools) — binary also available as `biggz-mcp` |
| `biggz pr create <change> [--with-evidence]` | Auto-generate branch and PR from SDD apply |
| `biggz codegraph init --cwd <dir>` | Initialize CodeGraph index (safe-root validation) |
| `biggz hooks init` | Create default `.biggz/hooks.yaml` |
| `biggz export changelog [--format json\|txt] [--since DATE]` | Export changelog from git log |
| `biggz recovery list\|show\|generate\|validate\|export\|import\|delete` | Recovery trace ledger |
| `biggz review start\|capture-result\|finalize\|gate\|...` | Content-addressed review pipeline (GitCommonDir, flock, Burn) |
| `biggz rdd enable\|disable\|status` | Review-Driven Development kill switch |

## Architecture

```
CLI (cmd/biggz)
  ├── Install ──► Agent Detection ──► Skill Deploy ──► Config Merge
  ├── SDD ──► Status, Verify, Attempt, Continue, Profile, Remediate
  ├── BigMem ──► MCP Server ──► 22 memory tools (graph, conflicts, projects, sync)
  ├── Review ──► Lifecycle, Findings, Corrections, Receipt, Gates, Ledger
  ├── Judgment Day ──► Dual-blind adversarial review
  ├── Backup/Restore ──► tar.gz snapshots
  ├── TDD / Plugin / MCP ──► Strict TDD, community plugins, MCP server
  └── Release ──► Version tagging and verification
```
> Full package map with 34 internal packages (see docs/comparison-with-gentle.md 2026-08-28) lives in [docs/architecture.md](docs/architecture.md).

### Key Design Decisions

- **Single ReviewState**: No parallel state machines (unlike gentle-ai's Transaction + CompactState)
- **Merkle root over evidence chain**: Single integrity check instead of 8+ individual hashes
- **13-state FSM with role guards and budget counters**: transition legality, roles, and budget enforcement validated by a single stateless FSM
- **One review engine**: receipt-driven RDD workflow; reviewers run as external provider CLIs (no embedded static-analysis engine)
- **Agent adapters as interfaces**: AgentAdapter for detection + deploy
- **Human-in-the-loop**: the tool surfaces consent envelopes and gates; the human decides

## SDD Workflow

```
proposal → spec → design → tasks → apply → verify → archive
                                              ↑
                                            design
```
Full graph: init → explore → research (optional) → proposal → spec → design → tasks → apply → verify → archive — see internal/assets/biggz/biggz-orchestrator-workflow.md

Each phase is a skill (`/sdd-propose`, `/sdd-spec`, etc.) that the orchestrator delegates to sub-agents. Review lenses are captured per-slot via `biggz review capture-result` (content-addressed, strict admission) and committed once via `biggz review finalize` — no DAG; `Graph.Execute()` is legacy (see `docs/architecture.md` for the `review start → capture → finalize → gate` pipeline).

### Edit authority for multi-repository changes

> **SDD V2 authority-free:** `biggz sdd-status --json --contract biggz-ai.sdd-status/v2` never blocks nor emits `granted_roots`/`missing_roots`/`edit_authority_blocked`. Status is a pure projection (`blockedReasons: []`, `nextRecommended` never `resolve-blockers` on authority). Enforcement lives only in `biggz sdd-apply <change>`, which warns `blocked(edit_authority_missing)` with both exits — edit `tasks.md` so every work unit stays inside the authorized edit roots, or grant via `biggz sdd-attempt grant <change> --root <path>... --change-instance <token> --request-id <id> --actor <name> --reason <text>`. The grant is recorded in the change's runtime ledger, scoped to the change-instance identity persisted in the change's own directory (`.biggz-instance`), and dies with archive.

Apply-side enforcement is native: `biggz sdd-apply <change>` is a guard that validates the workspace root plus the runtime-ledger granted roots against the repository roots the task plan targets. It exits 0 with the allowed roots when every `tasks.md` target stays inside them, or prints the same `blocked(edit_authority_missing)` reason plus the consent envelope's grant invocation and exits 1 when it does not. The apply phase assets (`sdd-apply` prompt and skill) run this guard before any edit and relay the consent envelope when it blocks.

## Review Pipeline (content-addressed — review start → capture → finalize → gate)

```
biggz review start --subject <file> [--lineage <id>] [--lenses <list>]
  → Store.Open via GitCommonDir → <commonDir>/biggz/review-transactions/<lineage>/v1/events/<sha256>
    (publishImmutable, dual-read legacy flat) → append genesis (start_review)
    with CorrectionBudget = min(200, max(2, ceil(changedLines/2))), frozen lens plan

biggz review capture-result --lineage <id> --lens <name> --order <n> --expected-revision <sha> --input <reviewer-json>
  → Admit (subjectHash echo, full-manifest inspection, findings canonicalized)
    → append lens_result via flock(LOCK_EX|LOCK_NB) on .lock

biggz review finalize <lineage>
  → Under flock: LoadChain + Validate + FixDeltaHashForSnapshot(baseTree, candidate, pathsDigest, cumulative, ledgerIDs)
    via domainHash("fix-delta/v1\x00"+writeLengthPrefixed(...))
  → Build PersistedReceipt (domainHash("biggz-ai.review-receipt-binding/v1") — legacy FixDelta domain "fix-delta/v1" kept for compat)
    → receipts/<sha256>.json → append complete_review (receipt_path + receipt_hash)
    → if BurnEnabled write burned.json tombstone, delete receipt (ephemeral)

biggz review gate <pre-pr|pre-push|post-apply|release> <lineage> [--json]
  → Validate chain + receipt binding + burn check → DeliveryBurned if burned, otherwise policy verdict
    (1/1 budget: FixRounds/ScopedValidations, domainHash + writeLengthPrefixed integrity)
```

Per-lens capture is strict and per-slot; `finalize` is the single commit. See `docs/architecture.md` and `docs/validation-guide.md` for the full flow and `domainHash`/`writeLengthPrefixed` integrity details.

## BigMem Memory

biggz-ai includes a full MCP server (`biggz-mcp`) exposing 22 memory tools:

`mem_save`, `mem_search`, `mem_get_observation`, `mem_update`, `mem_delete`, `mem_context`, `mem_session_summary`, `mem_session_start`, `mem_session_end`, `mem_save_prompt`, `mem_current_project`, `mem_suggest_topic_key`, `mem_timeline`, `mem_stats`, `mem_pin`, `mem_unpin`, `mem_doctor`, `mem_compare`, `mem_judge`, `mem_capture_passive`, `mem_merge_projects`, `mem_review`

100% compatible with gentle-ai's BigMem protocol.

## RDD (Review-Driven Development)

User-owned kill switch:

```bash
biggz rdd status     # show current mode
biggz rdd disable    # stop reviews
biggz rdd enable     # re-enable reviews
```

Any "off" wins: clone-local override beats global enable.

## Agent Adapters

| Agent | Detection | Config | Status |
|-------|-----------|--------|--------|
| OpenCode | `exec.LookPath("opencode")` | `~/.config/opencode/` | ✅ |
| Claude Code | `exec.LookPath("claude")` | `~/.claude/` | ✅ |
| Qwen | `exec.LookPath("qwen")` | `~/.qwen/` | ✅ |

Plus 13 more adapters ported for parity (cursor, windsurf, gemini, codex, pi,
vscode, kiro, antigravity, hermes, kimi, kilocode, trae, openclaw) — 16 core adapters.
Full capability manifest lists 27 entries including community adapters (see `internal/agents/capabilitymanifest`).

## Comparison with gentle-ai

| Dimension | gentle-ai | biggz-ai |
|---|---|---|
| Lines of code (Go, measured) | ~313K | ~60K (refreshed 2026-08-28, see docs/comparison-with-gentle.md) |
| Files (Go, measured) | 1,030 | 291 (refreshed 2026-08-28) |
| State machine | 2 parallel (Transaction + CompactState) | 1 (ReviewState + SchemaVersion) |
| Integrity | 8+ hashes | Evidence chain + MerkleRoot |
| Business rules | Embedded in FSM | PolicyEvaluator interface |
| Lenses | Constants in type system | LensPlugin interface |
| Agent adapters | 16 with manifests | 16 (ported, same set) |
| Testing | Golden files (fragile) | Property-based (rapid) |
| Human in loop | Optional | Always |

## Documentation

- [Architecture](docs/architecture.md)
- [Comparison with gentle-ai](docs/comparison-with-gentle.md)
- [Validation Guide](docs/validation-guide.md)
- [Contributing](CONTRIBUTING.md)
- [Security](SECURITY.md)
- [Support](SUPPORT.md)

## Code of Conduct

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md).

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for release history.

## License

MIT — see [LICENSE](LICENSE).
