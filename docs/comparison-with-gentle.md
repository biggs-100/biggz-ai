# biggz-ai vs gentle-ai

## By the Numbers

| Metric | gentle-ai | biggz-ai | Reduction |
|---|---|---|---|
| Production Go lines (no tests) | 112,037 | 36,307 | **68%** |
| Test Go lines | 200,631 | 24,060 | **88%** |
| Production Go files | 424 | 186 | **56%** |
| Test files | 606 | 105 | **83%** |
| Agent adapters | 16 (17 dirs incl. manifest) | 16 (ported, same set) | — |
| Internal packages | 33 | 28 | — |

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
| **Lenses** | Constants in type system (R1-R4 prefix maps, counter fields) | ✅ Same model — lens names as constants; embedded static-analysis lenses removed |
| **Orchestrator** | 70 lines, underdeveloped | Balanceado con review system + DAG graph |

## Agent Integration

| Aspect | gentle-ai | biggz-ai |
|---|---|---|
| **Agent adapters** | 16 with capability manifests, contract IDs, digests | 16 ported (single-file full adapters, same set) |
| **Detection** | Complex: binary + config file checks | Simple: `exec.LookPath()` |
| **Install** | Multi-step: download, extract, configure | Single command: `biggz install` |

## Review System

| Component | gentle-ai (73 files) | biggz-ai (18 files) |
|---|---|---|
| State machine | transaction.go (1785 lines) | model/review.go + fsm.go |
| Evidence chain | Scattered hashes | model/hash.go |
| Lenses | Hardcoded in transaction types | ✅ Hardcoded lens names; external reviewer CLIs execute them |
| Pipeline | Sequential stages | ✅ Sequential stages (DAG removed with legacy engine) |
| Judgment Day | rdd_mode.go | internal/review/judgment.go |
| Ledger | ledger.go | internal/review/ledger.go |
| Snapshots | snapshot.go | internal/review/snapshot.go |
| Authority | rar_*.go | internal/review/authority.go |

## SDD System

| Feature | gentle-ai | biggz-ai |
|---|---|---|
| **Native commands** | sdd-status, sdd-verify-validate, sdd-attempt, sdd-continue | ✅ Same + `sdd-apply` guard (7 verbs: `acquire`/`settle` added — 3d1dd53) |
| **Skills** | 29 unique (38 SKILL.md files) | 27: 8 verbatim, 10 thin-equivalent, 1 adapted, judgment-day 1.7, rdd-defect-workflow ported + `sdd-research` lane (3f072ca) |
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
| **Storage** | `~/.BigMem/` (SQLite) | `~/.biggz/bigmem/` (SQLite) |
| **Protocol** | Full with proactive saves, session summaries | ✅ Full |
| **MCP server** | `BigMem mcp` (external binary) | `biggz-mcp` (native Go) |

## What gentle-ai has that biggz-ai doesn't (verified 2026-08-23)

| Feature | Status in biggz-ai |
|---|---|
| Advisory review transport (`advisoryreview`) | Absent — no third runtime gets advisory-only verdicts today |
| Engram Cloud sync (tokens, autosync, self-hosted) | Absent — BigMem is local-only (SQLite at `~/.biggz/bigmem`) |
| GGA (Guardian Angel git-hook guardian) | Discarded — RDD kill switch + native gates (`review gate`) cover the same need; no git hooks needed |
| Pi runtime integration (answer-consent, organic routing) | Absent — pi adapter only |
| CodeGraph integration + `codegraph` verb | ✅ `biggz codegraph init --cwd` + guidance (safe-root validation + `change-intent` hint — ca97594) |
| Bench journeys (`bench/` + `gentle-ai bench`) | Absent |
| Package-manager installers (apt/pacman/dnf/zypper/scoop) + Android/Termux | Absent — brew formula + goreleaser only |
| `update`/`upgrade` split + beta/stable channels + `--all` ecosystem | ✅ Split `update` (check) / `upgrade` (execute + snapshot + verify hardening) + Windows async `.bat` mover (`replace_windows.go` — aabbe45) |
| Cooperative file lock (`filecoord` + `store_lock` hardening) | ✅ Minimal `BusyError` + stale detection (PID + mtime 5m) + `AcquireWithTimeout` + `LockPathFor` SHA256 (245286f) |
| Base-equivalent correction budget (`floor_two` #3583) | ✅ `DeriveCorrectionBudget` floor 2: `min(200, max(2, ceil(lines/2)))` (3c821de) |
| SDD archive collision handling | ✅ `ArchiveChange` with UTC timestamp suffix `20060102-150405`, no-clobber (ca97594, 56fdd57c) |
| SDD research lane (`sdd-research` + `research-lifecycle`) | ✅ Skill + prompt (dual tier) + opencode command + orchestrator gate (3f072ca) |
| Compact `acquire`/`settle` + admission probe (`runtimeReadiness`) | ✅ Token continuation + `BlockedReason`/`SettleObligation` freeing `verify`/`archive` from stale `decision-required` (3d1dd53) |
| Truthful `failed`/`interrupted` remediation settle (#3422) | ✅ `interrupted` vs `failed` distinct in `RemediationState.Reason` — original vs new head (0672bc0) |
| Full management TUI (model picker, Configure Models, agent set editing) | Agent-builder TUI + 4-mode model picker (19 agents, variants cache) — 2026-08-10 |
| `internal/opencode` Go model picker | Ported (2026-08-10) — reads the model-variants cache, JSONC-safe assignment read/write |
| Release policy attestation (`releasepolicy` run-marker) | ✅ Minimal `Validate` + `directoryContains` + `validateSnapshotFile` (full YAML/artifact pin omitted) |
| Legacy v1 authority compatibility (`review-*` commands) | Deliberate — no legacy debt |
| Path identity packages (`pathidentity`/`pathquote`) | ✅ `pathidentity.Contains` + `pathquote.Quote` (3× `quotePath` deduped, `ShellWord` omitted) |
| `consentenvelope` standalone package | Logic embedded in sdd/review + schemas in contracts/ |

## What biggz-ai has that gentle-ai doesn't

| Feature | Why better |
|---|---|
| Human-in-the-loop by design | Not optional — always requires human decision |
| No legacy flags | Zero `legacy*` fields |
| Property-based testing | FSM invariants via `rapid` |
| Atomic config merge | `filemerge.WriteFile` (temp → rename) |
| MCP server in Go | No external binary dependency |
| Contracts-dir walk test | gentle validates schemas ad hoc in CLI tests; biggz compiles EVERY schema and validates EVERY fixture (`internal/contracts/walk_test.go`) |
| Native `sdd-apply` edit-authority guard | CLI verb consuming `granted_roots`, wired into the apply phase assets |
| Surgical `uninstall` | Per-op failure collection, JSONC key deletion (`RemoveKeysJSONC`), keeps memory/backups unless `--purge` |
| `update` with automatic asset reconcile | Re-deploys skills/prompts/commands/plugins/config/MCP after binary swap (`--no-reconcile`) |
| CLI verbs gentle lacks | `backup create/list`, `release status/tag/verify`, `pr create`, `recovery` ledger, `mcp` server, `bigmem`, `rdd`, `sdd-apply`, `sdd-new` wizard |
| 10 phase prompts | `internal/assets/prompts/sdd/*` — gentle has no prompts/ dir |
| Manifest-freeze excludes untracked files | Credential-disclosure class designed out (gate.go:834): untracked paths are outside the candidate |
| Multi-OS e2e CI matrix | ubuntu/windows/macos green lane (2026-08-10); gentle shipped 19-27 Windows failures unseen |

## Risk profile (2026-08-10)

Of gentle's 20 documented root causes (~250 open issues, meta #2471), biggz-ai
eliminates ~9 by architecture (single state machine, manifest-freeze,
per-repo CAS, no transport matrix), mitigates ~7 by discipline (real e2e,
read-backs, per-op uninstall), and shares ~4 as standing watch items: prose
contracts, agent enums, update post-condition verification, and gate identity
equality.

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
