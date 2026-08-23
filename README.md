# biggz-ai

**AI Agent Harness — Review-Driven Development with Human-in-the-Loop**

biggz-ai is a lightweight, self-contained harness for AI coding agents (OpenCode, Claude Code, Qwen). It provides SDD (Spec-Driven Development) workflow orchestration, code review pipelines with 6 lenses (R1-R4 + performance, dependencies), persistent memory (BigMem), and full lifecycle management — all with the human always in control.

Inspired by gentle-ai, but rebuilt from scratch with roughly 80% less code (measured 2026-08-10: ~60K vs ~313K Go lines including tests), a cleaner architecture, and no legacy debt.

## Quick Start

```bash
# Install in your AI agent
biggz install

# Check SDD status
biggz sdd-status

# Run a code review pipeline
echo '{"repository":"my/repo","commit_sha":"abc123"}' | biggz
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `biggz` | Run review pipeline (stdin → JSON) |
| `biggz install` | Install skills + config in agent |
| `biggz uninstall` | Remove managed assets (keeps memory data unless `--purge`) |
| `biggz sdd-status` | Show active/archived SDD changes |
| `biggz sdd-verify-validate` | Validate verify reports |
| `biggz sdd-attempt` | Manage attempt budgets |
| `biggz sdd-continue <change>` | Determine next SDD phase |
| `biggz sdd-apply <change>` | Validate edit authority for the apply phase (guard) |
| `biggz BigMem save|search|get` | Persistent memory |
| `biggz backup create|list|restore` | Snapshot/restore state |
| `biggz release status|tag|verify` | Version management |
| `biggz skill-registry refresh` | Regenerate skill registry |
| `biggz sync` | Deploy skills, config, prompts, and commands |
| `biggz update` | Update the binary and reconcile managed agent assets (`--no-reconcile` to skip) |
| `biggz doctor` | Run system health checks (`--json`, `--fix`) |
| `biggz pr create <change>` | Auto-generate branch and PR from SDD apply |
| `biggz recovery list\|show\|generate` | Recovery trace ledger |
| `biggz rdd enable\|disable\|status` | Review-Driven Development kill switch |
| `biggz-mcp` | MCP server for agent memory tools |

## Architecture

```
CLI (cmd/biggz)
  ├── Install ──► Agent Detection ──► Skill Deploy ──► Config Merge
  ├── SDD ──► Status, Verify, Attempt, Continue
  ├── BigMem ──► MCP Server ──► 22 memory tools
  ├── Review ──► Lifecycle, Findings, Corrections, Receipt, Gates, Ledger
  ├── Judgment Day ──► Dual-blind adversarial review
  ├── Backup/Restore ──► tar.gz snapshots
  └── Release ──► Version tagging and verification
```

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

Each phase is a skill (`/sdd-propose`, `/sdd-spec`, etc.) that the orchestrator delegates to sub-agents. Lenses run in parallel via the DAG graph executor (SGH-inspired multi-ready-unit scheduling).

### Edit authority for multi-repository changes

A change whose `tasks.md` targets repositories outside the planning repository reports
`blocked(edit_authority_missing)` from `biggz sdd-status` and carries a typed consent envelope
(`biggz-ai.edit-authority-consent/v1`) whose granted choice is the exact runnable
`biggz sdd-attempt grant <change> --root <path>... --change-instance <token>` invocation. The
grant is recorded in the change's runtime ledger, scoped to the change-instance identity
persisted in the change's own directory (`.biggz-instance`), and dies with archive.

Apply-side enforcement is native: `biggz sdd-apply <change>` is a guard that consumes
`granted_roots` from the change's runtime ledger and exits 0 with the allowed roots when
every `tasks.md` target stays inside them, or prints the same `blocked(edit_authority_missing)`
reason plus the consent envelope's grant invocation and exits 1 when it does not. The apply
phase assets (`sdd-apply` prompt and skill) run this guard before any edit and relay the
consent envelope when it blocks.

## Review Pipeline

```
Input (ReviewSubject)
  → Graph.Execute()
    ├── Risk Lens (R1)     — static analysis, git diff
    ├── Readability (R2)   — file length, naming heuristics
    ├── Reliability (R3)   — test coverage, error handling
    ├── Resilience (R4)    — timeouts, context, concurrency
    ├── Performance        — hotspots, complexity, resource usage
    ├── Dependencies       — imports, transitive risk
    └── Policy Evaluator   — business rules (depends on all lenses)
  → ReviewState with evidence chain + MerkleRoot
```

All 6 lenses run in parallel — they have no dependencies on each other.

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
vscode, kiro, antigravity, hermes, kimi, kilocode, trae, openclaw).

## Comparison with gentle-ai

| Dimension | gentle-ai | biggz-ai |
|---|---|---|
| Lines of code (Go, measured) | ~313K | ~60K |
| Files (Go, measured) | 1,030 | 291 |
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
