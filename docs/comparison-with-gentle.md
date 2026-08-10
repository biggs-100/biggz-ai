# biggz-ai vs gentle-ai

## By the Numbers

| Metric | gentle-ai | biggz-ai | Reduction |
|---|---|---|---|
| Production Go lines (no tests) | 112,037 | 36,307 | **68%** |
| Test Go lines | 200,631 | 24,060 | **88%** |
| Production Go files | 424 | 186 | **56%** |
| Test files | 606 | 105 | **83%** |
| Agent adapters | 17 | 3 | **82%** |
| Internal packages | ~29 | 28 | — |

Measured 2026-08-10. gentle-ai counts are from a filtered clone of `cmd/` and
`internal/` (no testdata); biggz-ai counts are the full module.

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
| **Agent builder** | `internal/agentbuilder` + TUI flow | ✅ Ported: same engines, parser, installer, registry (`~/.config/biggz/custom-agents.json`), SDD phase support |
| **SDD phase hooks** | SDD task-result failures → terminal `GENTLE_AI_SDD_FAILURE` handoff | ✅ Ported (schema `biggz-ai.sdd-task-result-failure/v1`, continuation `biggz sdd-status --cwd <dir> --json`) |
| **Apply edit-authority guard** | Apply gates + consent relay (S6 of #2540) | ✅ Native `biggz sdd-apply <change>` guard consuming `granted_roots`, wired into the `sdd-apply` prompt and skill (2026-08-10) |

## OpenCode Plugins (3/3 parity)

| Plugin | gentle-ai | biggz-ai |
|---|---|---|
| `review-result-artifacts.ts` | Reviewer transport + SDD hooks, schema `gentle-ai.sdd-task-result-failure/v1` | ✅ Same + biggz schema; keeps deliberate quarantine-to-file divergence (raw payload → `.git/biggz/preserved-results/`) and scrubbed native causes (env/email/abs-path redaction, 512-char cap) |
| `skill-registry.ts` | `gentle-ai skill-registry refresh` at startup | ✅ `biggz skill-registry refresh --quiet --no-gitignore --cwd <dir>` (fire-and-forget, 30s timeout) |
| `model-variants.ts` | Writes `~/.gentle-ai/cache/model-variants.json` | ✅ Writes `~/.biggz/cache/model-variants.json` (same atomic tmp+rename contract) |

Deferred from gentle-ai: the `internal/opencode` Go package (biggz's model
picker is still a static stub; the plugin cache is written but not yet read
from Go) and the SDDNewPhase agent-builder mode (standalone + phase-support
are wired).

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
| `internal/opencode` Go package | Deferred — the model-variants plugin cache is written but the model picker is still a static stub |
| SDDNewPhase agent-builder mode | Deferred — standalone + phase-support SDD modes are wired; new-phase graph wiring is not |
| Multi-agent install targets (claude/gemini/codex skills dirs in agent builder) | Deferred — agent builder installs to the available generation engines' skills dirs |
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
| Contracts-dir walk test | gentle validates schemas ad hoc in CLI tests; biggz compiles EVERY schema and validates EVERY fixture (`internal/contracts/walk_test.go`) |

## Wire-Envelope Formalization

| Dimension | gentle-ai | biggz-ai |
|---|---|---|
| Schema home | `contracts/review-integration/{v1,v2}/` + `contracts/sdd-integration/v1/` | `contracts/review-integration/v1/` + `contracts/sdd-integration/v1/` |
| Engine | Inline `jsonschema` usage inside CLI tests only | `internal/contracts` package: embedded FS compiler, no network, cached |
| Validation stance | Test-only conformance of emitted bytes | Same, inherited — test-only + opt-in emission checks, never runtime |
| Directory walk test | Absent | Added: every schema compiles with its `$id`; every fixture validates 1:1; negative cases mutate fixtures programmatically |
| `$id` host | `https://gentle-ai.dev/contracts/...` | `https://biggz-ai.dev/contracts/...` |
| Ledger additivity proof | Implicit | `internal/review/ledger_regression_test.go` bakes a pre-layer chain and proves no ledger byte changes |

biggz's v1 contract dirs start UNFROZEN (no FREEZE.md until a first release
consumer pins them); gentle's v1 is frozen (see its FREEZE.md wording). The
versioning policy — new version = new `v<N+1>/` directory, freeze recorded
in FREEZE.md — is shared.
