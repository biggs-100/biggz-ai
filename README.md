# biggz-ai

**AI Agent Harness — Review-Driven Development with Human-in-the-Loop**

biggz-ai is a lightweight, self-contained harness for AI coding agents (OpenCode, Claude Code, Qwen). It provides SDD (Spec-Driven Development) workflow orchestration, code review pipelines with 4 lenses (R1-R4), persistent memory (BigMem), and full lifecycle management — all with the human always in control.

Inspired by gentle-ai, but rebuilt from scratch with 95% less code, a cleaner architecture, and no legacy debt.

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
| `biggz sdd-status` | Show active/archived SDD changes |
| `biggz sdd-verify-validate` | Validate verify reports |
| `biggz sdd-attempt` | Manage attempt budgets |
| `biggz sdd-continue <change>` | Determine next SDD phase |
| `biggz BigMem save|search|get` | Persistent memory |
| `biggz backup create|list|restore` | Snapshot/restore state |
| `biggz release status|tag|verify` | Version management |
| `biggz skill-registry refresh` | Regenerate skill registry |
| `biggz rdd enable|disable|status` | Review-Driven Development kill switch |
| `biggz-mcp` | MCP server for agent memory tools |

## Architecture

```
CLI (cmd/biggz)
  ├── Orchestrator ──► Pipeline/DAG ──► Lenses (R1-R4)
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
- **5 coarse FSM states**: Policy evaluation is external (PolicyEvaluator interface)
- **Lenses as plugins**: LensPlugin interface, register via Registry
- **Agent adapters as interfaces**: AgentAdapter for detection + deploy
- **Human-in-the-loop**: Orchestrator delegates, human decides

## SDD Workflow

```
proposal → spec → design → tasks → apply → verify → archive
                                              ↑
                                            design
```

Each phase is a skill (`/sdd-propose`, `/sdd-spec`, etc.) that the orchestrator delegates to sub-agents. Lenses run in parallel via the DAG graph executor (SGH-inspired multi-ready-unit scheduling).

## Review Pipeline

```
Input (ReviewSubject)
  → Graph.Execute()
    ├── Risk Lens (R1)     — static analysis, git diff
    ├── Readability (R2)   — file length, naming heuristics
    ├── Reliability (R3)   — test coverage, error handling
    ├── Resilience (R4)    — timeouts, context, concurrency
    └── Policy Evaluator   — business rules (depends on all lenses)
  → ReviewState with evidence chain + MerkleRoot
```

All 4 lenses run in parallel — they have no dependencies on each other.

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

## Comparison with gentle-ai

| Dimension | gentle-ai | biggz-ai |
|---|---|---|
| Lines of code | ~254K | ~6.3K |
| Files | 770 | 64 |
| State machine | 2 parallel (Transaction + CompactState) | 1 (ReviewState + SchemaVersion) |
| Integrity | 8+ hashes | Evidence chain + MerkleRoot |
| Business rules | Embedded in FSM | PolicyEvaluator interface |
| Lenses | Constants in type system | LensPlugin interface |
| Agent adapters | 17 with manifests | 3 simple adapters |
| Testing | Golden files (fragile) | Property-based (rapid) |
| Human in loop | Optional | Always |

## Documentation

- [Architecture](docs/architecture.md)
- [Comparison with gentle-ai](docs/comparison-with-gentle.md)
- [Validation Guide](docs/validation-guide.md)
- [Contributing](CONTRIBUTING.md)

## License

MIT — see [LICENSE](LICENSE).
