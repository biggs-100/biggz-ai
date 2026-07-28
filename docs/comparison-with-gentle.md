# biggz-ai vs gentle-ai

## By the Numbers

| Metric | gentle-ai | biggz-ai | Reduction |
|---|---|---|---|
| Production lines | 95,684 | 2,993 | **97%** |
| Test lines | 158,757 | 3,329 | **98%** |
| Production files | 331 | 36 | **89%** |
| Test files | 439 | 28 | **94%** |
| Agent adapters | 17 | 3 | **82%** |
| Go packages | ~29 internal | 23 | **21%** |

## Architecture

| Dimension | gentle-ai | biggz-ai |
|---|---|---|
| **Role** | Standalone tool that installs things IN agents | Harness that runs INSIDE the agent |
| **State machine** | 2 parallel (Transaction 1884 lines + CompactState 1494 lines) | 1 (ReviewState + SchemaVersion field) |
| **Integrity** | 8+ individual SHA-256 hashes cross-validated | Evidence chain with linked hashes + MerkleRoot |
| **Business rules** | Embedded in FSM transition checks | PolicyEvaluator external interface |
| **Corrections** | 7+ fields in model (budget, cumulative, attempts...) | 1 struct (files, lines, reason, hashes) |
| **Lenses** | Constants in type system (R1-R4 prefix maps, counter fields) | LensPlugin interface (register any lens) |
| **Orchestrator** | 70 lines, underdeveloped | Balanceado con review system + DAG graph |

## Agent Integration

| Aspect | gentle-ai | biggz-ai |
|---|---|---|
| **Agent adapters** | 17 with capability manifests, contract IDs, digests | 3 (OpenCode, Claude, Qwen) ~50 lines each |
| **Detection** | Complex: binary + config file checks | Simple: `exec.LookPath()` |
| **Install** | Multi-step: download, extract, configure | Single command: `biggz install` |

## Review System

| Component | gentle-ai (73 files) | biggz-ai (18 files) |
|---|---|---|
| State machine | transaction.go (1785 lines) | model/review.go + fsm.go |
| Evidence chain | Scattered hashes | model/hash.go |
| Lenses | Hardcoded in transaction types | 4 separate packages in internal/lens/ |
| Pipeline | Sequential stages | Sequential + DAG graph (parallel lenses) |
| Judgment Day | rdd_mode.go | internal/review/judgment.go |
| Ledger | ledger.go | internal/review/ledger.go |
| Snapshots | snapshot.go | internal/review/snapshot.go |
| Authority | rar_*.go | internal/review/authority.go |

## SDD System

| Feature | gentle-ai | biggz-ai |
|---|---|---|
| **Native commands** | sdd-status, sdd-verify-validate, sdd-attempt, sdd-continue | ✅ Same 4 commands |
| **Skills** | 22+ skills | 35 skills (same content) |
| **Skill registry** | `.atl/skill-registry.md` | ✅ Same |
| **Relative paths** | Absolute (C:\Users\...) | ✅ Relative |

## Memory (BigMem)

| Feature | gentle-ai | biggz-ai |
|---|---|---|
| **MCP tools** | 22 | ✅ 22 (same) |
| **Storage** | `~/.BigMem/` (SQLite) | `~/.biggz/BigMem/` (JSON) |
| **Protocol** | Full with proactive saves, session summaries | ✅ Full |
| **MCP server** | `BigMem mcp` (external binary) | `biggz-mcp` (native Go) |

## What gentle-ai has that biggz-ai doesn't

| Feature | Why missing |
|---|---|
| 14 more agent adapters | Not needed — 3 covers the major agents |
| TUI | Not needed — UI is the agent itself |
| Self-update | Not needed — agent manages itself |
| Platform detection | Not needed — agent knows its OS |
| Store locks | Not needed — in-memory + BigMem |
| Legacy compatibility | Intentional — no legacy debt |

## What biggz-ai has that gentle-ai doesn't

| Feature | Why better |
|---|---|
| DAG graph execution | Parallel lenses (SGH multi-ready-unit) |
| Human-in-the-loop by design | Not optional — always requires human decision |
| No legacy flags | Zero `legacy*` fields |
| Property-based testing | FSM invariants via `rapid` |
| Atomic config merge | `filemerge.WriteFile` (temp → rename) |
| MCP server in Go | No external binary dependency |
