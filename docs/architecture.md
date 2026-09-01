# biggz-ai Architecture

## Overview

biggz-ai is an **AI agent harness** — it runs inside AI coding agents (OpenCode, Claude Code, Qwen) and provides structured workflows for code review and SDD (Spec-Driven Development). The human is always in the loop: the orchestrator proposes, the human decides.

## Core Principles

1. **Harness, not an agent** — biggz-ai doesn't call AI APIs. The agent IS the AI.
2. **Human-in-the-loop** — the orchestrator delegates, the human decides every phase transition.
3. **Protocol-first** — designed with a single data model from day one (unlike gentle-ai's evolved architecture).
4. **Minimal surface area** — ~40K filtered production Go lines (cmd/ + internal/) vs ~60K full-module prod (59,725 prod, 103,782 total) vs gentle-ai ~313K total (measured 2026-08-28, filtered ~58.4K prod) — see docs/comparison-with-gentle.md for method.
5. **Under-demand loading** — skills, tools, and memory are loaded only when needed.

## Package Map

34 internal packages (see docs/comparison-with-gentle.md 2026-08-28):

```
cmd/
├── biggz/           — CLI entry point (SDD, review, RDD, memory, backup, release verbs)
├── biggz-mcp/       — MCP server for BigMem protocol

internal/
├── agents/          — Agent adapters (opencode, claude, qwen)
├── agentbuilder/    — Custom SDD agent generation (engines, parser, installer, registry)
├── assets/          — Embedded skills + configs + OpenCode plugins (opencode plugins + biggz-orchestrator.md + biggz-synthesis-gate.js)
├── backup/          — tar.gz snapshot/restore
├── bigmem/          — Persistent observation store + MCP tools (22 tools, SQLite)
├── codegraph/       — CodeGraph index + change-intent report (safe-root validation)
├── contracts/       — Wire-envelope formalization engine (test-only validation, draft 2020-12)
├── doctor/          — System health checks (BigMem FTS, stale locks, skill registry)
├── filemerge/       — Atomic write, JSONC merge, section injection
├── install/         — Agent detection + deploy
├── lens/            — Legacy lens infra — review now uses content-addressed event store (risk, readability, reliability, resilience kept for parity)
│   ├── gitdiff/     — Shared git diff parsing
│   ├── risk/        — R1: static analysis
│   ├── readability/ — R2: code quality heuristics
│   ├── reliability/ — R3: test coverage, error handling
│   └── resilience/  — R4: timeouts, context, concurrency
├── review/          — Review lifecycle + RDD (store, lock, finalize, receipt, snapshot, hash, artifact)
│   ├── store.go     — GitCommonDir → <commonDir>/biggz/review-transactions/<lineage>/v1/events/<sha256>, publishImmutable, dual-read legacy flat
│   ├── lock.go      — flock(LOCK_EX|LOCK_NB) on .lock, BusyError, stale 5m PID+mtime
│   ├── finalize.go  — Finalize + FixDeltaHashForSnapshot + BurnEnabled + burned.json
│   ├── receipt.go   — Receipt binding via domainHash("biggz-ai.review-receipt-binding/v1")
│   ├── snapshot.go  — Snapshot chain via domainHash("biggz-ai.review-snapshot/v1")
│   └── artifact.go  — Artifact subject / manifest / admission (domainHash + writeLengthPrefixed)
├── sdd/             — SDD native commands (synthesis, pending, question, status_v2)
│   ├── synthesis.go — RenderSynthesis 4+6 markers, ReadLoop >50KB paginated
│   ├── pending.go   — Pending dual-write BigMem + state.yaml (biggz-ai.pending-question/v1)
│   ├── question.go  — ValidateQuestionEnvelope 16/60/4/2-4, IsCheckpointEnvelope
│   └── status_v2.go — biggz-ai.sdd-status/v2 authority-free projection
├── sddattempt/      — SDD runtime attempt ledger (acquire/settle, CAS, tokens, grants)
│   ├── sddattempt.go — Acquire/Settle/Begin/Finish/Grant/Rescope, BlockedError, SettleObligation
│   └── cas_store.go  — GitCommonDir sdd-runtime/v1/<change>/ record-<sha>.json + HEAD + LOCK
├── platform/        — OS/arch detection + shell quoting (pathidentity, pathquote)
├── policy/          — PolicyEvaluator interface + guardrails
├── project/         — Project detection (git remote → BigMem project pinning)
├── tui/             — Terminal UI (theme, gallery, status-line, ask dialog)
└── skillregistry/   — Skill registry scan + generation

model/               — Core types (ReviewState, FSM 13-state, hash domainHash+writeLengthPrefixed, ReviewStatus, Role)
├── hash.go          — domainHash(domain+"\x00"+payload) + writeLengthPrefixed(u32 BE), EvidenceDomain/MerkleDomain
├── review.go        — MaxFixRounds=1, MaxScopedValidations=1, ReviewStatus 13 values, BudgetCounters
└── fsm.go           — Guard table 13-state + Any→* wildcard + BudgetCheck (<1) verbatim errors
contracts/           — Frozen JSON Schemas (review-integration/v1 21 schemas + sdd-integration/v1 2 schemas)
model/hash.go        — domainHash(domain+"\x00"+payload) + writeLengthPrefixed helpers (EvidenceDomain/MerkleDomain)
plugin/              — LensPlugin + AgentAdapter interfaces
registry/            — Build-time plugin registry
pipeline/            — Sequential + DAG graph (legacy — superseded by capture-result per-slot finalize, kept for compat)
```

## Data Flow

### Review Pipeline (content-addressed)

```
biggz review start --subject <file> [--lineage <id>] [--lenses risk,readability,...]
  → Store.Open via GitCommonDir: <commonDir>/biggz/review-transactions/<lineage>/v1/events/<sha256>
  → Append genesis (start_review) with CorrectionBudget = min(200, max(2, ceil(changedLines/2))), frozen lens plan
  → HEAD = genesis

biggz review capture-result --lineage <id> --lens <name> --order <n> \
    --expected-revision <head> --input <reviewer-json>
  → Admit: verify subjectHash echo, inspection covers full manifest, findings canonicalized
  → Append lens_result event via publishImmutable (dual-read legacy fallback)
  → Repeat per selected lens slot (selected order, lens)

biggz review finalize <lineage>
  → Under flock(LOCK_EX|LOCK_NB) on .lock: LoadChain + Validate + deriveFinalizeData
  → Compute FixDeltaHashForSnapshot(baseTree, candidateTree, pathsDigest, cumulative, ledgerIDs) via domainHash("biggz-ai.fix-delta/v1\x00"+writeLengthPrefixed(...)) (legacy "fix-delta/v1" kept for compat)
  → Build PersistedReceipt (domainHash("biggz-ai.review-receipt-binding/v1")) → receipts/<sha256>.json via publishNoReplace
  → Append complete_review event (receipt_path + receipt_hash)
  → If BurnEnabled (default true): append burn_review, write burned.json tombstone, delete receipt file (ephemeral); else preserve receipt

biggz review gate <pre-pr|pre-push|post-apply|release> <lineage> [--json]
  → Validate chain + receipt binding + burn check → DeliveryBurned if burned, otherwise policy verdict
```

### SDD Workflow

```
User: "quiero implementar X"
  → Orquestador: ¿RDD enabled? Sí → SDD
  → sdd-continue: check artifacts → next phase
  → Delegates to sdd-propose sub-agent
    → Reads sdd-propose/SKILL.md
    → Writes proposal.md
    → Returns result
  → Orquestador: muestra resultado, pregunta
  → User: "continua"
  → sdd-continue: check → next: spec
  → Delegates to sdd-spec sub-agent
  → ... etc hasta archive
```

**SDD Agent Authority (MANDATORY):** SDD phases MUST be delegated to `sdd-<phase>` agents (`sdd-propose`, `sdd-spec`, `sdd-design`, `sdd-tasks`, `sdd-apply`, `sdd-verify`, `sdd-archive`, `sdd-explore`, `sdd-research`); do NOT use `general`/`explore` for SDD artifacts. `general` is for non-SDD direct work only. SDD artifact creation via `general` is FORBIDDEN — it bypasses the SDD contract, BigMem topic keys, and workload guard.

## State Machine

13 states with role-based guard table and budget counters (model/fsm.go, model/review.go). Self-transitions (current == target) are always valid; wildcard `Any` (`From == ""`) covers cross-cutting terminals.

| From | To | Permitted Roles | Precondition | Budget Check |
|------|----|-----------------|--------------|---------------|
| unreviewed | in_review | Reviewer, Lead | evidence-non-empty | — |
| in_review | needs_changes | Reviewer, Lead | — | — |
| needs_changes | changes_submitted | Author | — | fix-rounds (<1, MaxFixRounds=1) verbatim `"budget exceeded: fix rounds exhausted (1/1)"` |
| changes_submitted | re_review | Reviewer, Lead | — | scoped-validations (<1, MaxScopedValidations=1) verbatim `"budget exceeded: scoped validations exhausted (1/1)"` |
| in_review | approved | Reviewer, Lead, Admin | all-policies-pass | — |
| in_review | escalated | Lead, Admin | escalation-reason-provided | — |
| Any ("") | invalidated | Admin | scope-change-detected | — |
| Any ("") | blocked | Lead, Admin | policy-violation | — |
| Any ("") | withdrawn | Author | — | — |
| approved | superseded | Lead, Admin | superseding-review-exists | — |
| Any ("") | completed | Lead, Admin | all-policies-pass-receipt-valid | — |
| completed | archived | Lead, Admin | 30-day-minimum | — |

Canonical `ReviewStatus` values: `unreviewed`, `in_review`, `needs_changes`, `changes_submitted`, `re_review`, `approved`, `escalated`, `invalidated`, `blocked`, `withdrawn`, `superseded`, `completed`, `archived` (plus legacy `pending`/`in_progress` aliases). `Invalidated`/`Withdrawn` lineages are terminal and MUST NOT be finalizable — `Finalize` returns `Lineage is invalidated/withdrawn` under lock.

Budget enforcement (`checkBudget`) is verbatim gentle parity: `FixRounds >= 1` and `ScopedValidations >= 1` reject with `"budget exceeded: ... (1/1)"`.

```
unreviewed → in_review → needs_changes → changes_submitted → re_review
                │  │  │          │                      ↘
                │  │  ├─→ approved → superseded          ╰─→ (budget exhausted blocks correction cycle)
                │  │  └─→ escalated
                │  └─→ blocked / invalidated / withdrawn (Any→*)
                └─→ completed (Any→completed) → archived
```

Transitions are validated by `FSM.Transition(current, target, role, counters)` via `findGuardEntry` (exact then wildcard). Business rules (policy checks, evidence requirements) are the caller's responsibility, not the FSM's.

## BigMem Memory Protocol

22 MCP tools exposed by `biggz-mcp` plus Session Boot Recall hard gate:

**Session Boot Recall (HARD GATE):** before SDD Session Preflight the orchestrator MUST run `biggz_mem_context(limit=5)` + `biggz_mem_search(query:"sdd {project}" limit=10)` + `biggz_mem_search(query:"session_summary" limit=5)`, inject a short recap, emit `## Session Recall` markdown, and fallback to `biggz sdd-status --json --instructions` when BigMem is empty. The gate is blocking like preflight (see `internal/assets/biggz/biggz-orchestrator.md` Session Boot Recall). The synthesis gate has a narrow same-turn exception for `## Session Recall` → preflight, but does not weaken the `## Sub-agent Result` block after delegation.

22 MCP tools exposed by `biggz-mcp`:

- **Save**: `mem_save` (with what/why/where/learned format)
- **Search**: `mem_search` (full-text across title/content/topic_key)
- **Get**: `mem_get_observation` (full content)
- **Update**: `mem_update` (by ID)
- **Delete**: `mem_delete`
- **Session**: `mem_context`, `mem_session_summary`, `mem_session_start/end`
- **Prompt**: `mem_save_prompt`
- **Project**: `mem_current_project`
- **Suggest**: `mem_suggest_topic_key`
- **Timeline**: `mem_timeline`
- **Stats**: `mem_stats`
- **Pin**: `mem_pin`, `mem_unpin`
- **Doctor**: `mem_doctor`
- **Compare**: `mem_compare`
- **Judge**: `mem_judge`
- **Capture**: `mem_capture_passive`
- **Merge**: `mem_merge_projects`
- **Review**: `mem_review`

Proactive save triggers: architecture decisions, bug fixes, discoveries, config changes, patterns, user preferences.

## SGH Graph Execution

The pipeline implements SGH (Structured Graph Harness) principles:

- **Multi-ready-unit scheduling**: independent nodes run concurrently
- **Immutable plan**: execution plan is fixed for a plan version
- **Three-layer separation**: planning (orchestrator), execution (graph), recovery (human)
- **Cancel on failure**: if one node fails, running nodes are cancelled, completed nodes rollback

## RDD (Review-Driven Development)

Kill switch stored in:

- **Global**: `~/.biggz/rdd-mode.json`
- **Clone-local**: `.git/rdd-mode.json` (can only disable)

Any "off" wins. Status is read-only. Re-enabling applies to future candidates only.

### Synthesis Gate (3-layer defense) + Session Recall

Guarantees the human sees audited `## Sub-agent Result` before deciding. Gate `b0d2fc1` enforces the Post-Delegation Human Checkpoint. Session Boot Recall (`## Session Recall`) is a preceding hard gate that restores prior session context via BigMem before preflight. Synthesis rendering uses `internal/sdd/synthesis.go` `RenderSynthesis` with 4 required markers + 6 optional (Preview/Diff/Decisions/Commands/Validation/Failure), omit-empty, >50KB `ReadLoop` paginated `ReadAt` with verify-retry, `ValidateQuestionEnvelope` 16/60/4/2-4, `pending` dual-write BigMem + `state.yaml` (`biggz-ai.pending-question/v1`), `PI_SUBAGENT_CHILD=1` bypass, and `BIGGZ_ADVISE=1` thin-advise.

**Layer 1 — Prompt (machine-verifiable invariant):** `internal/assets/biggz/biggz-orchestrator.md` contains a copy-pasteable block with 4 markers (`## Sub-agent Result: {phase/agent}`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**`) and states `INVALID and will be blocked` for any **checkpoint** `ask_user_question`/`question` (those presenting `proceed`/`adjust`/`stop` or `continue`/`correct` after a delegated sub-agent) without immediately preceding markdown. General clarification questions that are NOT a checkpoint do NOT require synthesis and MUST NOT be blocked — e.g. '¿por dónde empezamos?', preflight, or other orchestration clarifications not presenting a delegated result use the question tool directly without synthesis. Every checkpoint `ask` reference is followed by `REMINDER: synthesis markdown is separate chat markdown emitted FIRST...` (12× convergence) so prompt and gate cannot drift.

**Layer 2 — Pi gate (blocking + thin advise):** `internal/assets/pi/biggz-synthesis-gate.js` wraps `pi.registerTool` for future `ask_user_question`/`question` AND sweeps pre-registered tools (`pi.tools` / `pi._tools` / `pi.getAllTools`→`getToolDefinition` if available) — load-order safe — and hooks `pi.on("tool_call")` as a secondary guard that **actually blocks** (`{block:true, reason}`), not just warns.

- **Source priority (blocking STRICT):** **only** `currentTurnMarkdown` satisfies the block — `ctx.history`/`lastAssistant` are history (stale) and are deliberately NOT checked for blocking (they only feed the non-blocking advise path via `getCurrentTurnSynthesis` fallback). The same-turn buffer (`recordText` via `pi.on("message_end")`/`message_update`/`message_start` plus legacy `assistant_message` fallback, reset at `turn_start`/`agent_start` and after each successful question) fixes the streaming race where markdown is emitted milliseconds before the tool call and has not yet landed in `ctx.history` (120 s window).
- **Blocking (default, checkpoint-only):** when the 4-marker check fails **and** `isCheckpointAsk(params)` is true, the gate **blocks** in BOTH paths: (a) wrapped `execute` returns `{content:[{type:"text", text:"Please synthesize before asking — missing ## Sub-agent Result block..."}], isError:true}` and does **not** call `original()`, (b) `tool_call` handler returns `{block:true, reason}` — either path prevents the question reaching the user. General asks (`isCheckpointAsk` false) bypass blocking entirely (allowed directly). Both blocked paths notify via `pi.notify`/`ctx.ui.notify`. Synthesis inside the tool's `question` param does not satisfy the check. **Preflight allowance:** if no synthesis has ever existed in the session (`currentTurn` + `history` + `last` all empty of markers), the gate **allows** the ask without blocking — this unblocks the SDD Session Preflight (first asks before any delegation) while still enforcing strict same-turn after at least one synthesis exists (for checkpoint asks).
- **Thin advise (opt-in):** when markers are present but `Artifacts/Paths` is thin (`countPaths <2 || len <50` via `extractArtifactsSection` cut at `Risks`/`Next`/`## `) the gate does **not** block. With `BIGGZ_ADVISE=1` (or settings advise flag) it emits a non-blocking warning `concern: synthesis is thin (Artifacts/Paths count=N, len=M)` via `pi.notify` (warning level) and allows the call. Without the flag the thin case passes silently. The heuristic never auto-fixes and never calls a model.
- **Session Recall exception (narrow):** before the first synthesis, a same-turn `## Session Recall` block satisfies the gate for the subsequent preflight ask. Only `currentTurnMarkdown` containing `## Session Recall` counts; history does not. After the first `## Sub-agent Result`, strict same-turn synthesis is required and Session Recall no longer bypasses.
- **Bypass:** only `PI_SUBAGENT_CHILD=1` bypasses both modes. There is no orchestrator bypass (`BIGGZ_ORCHESTRATOR` is not honored).
- **Checkpoint vs general:** The synthesis gate checkpoint applies ONLY to Post-Delegation Human Checkpoint questions (`proceed`/`adjust`/`stop` or `continue`/`correct` after a delegated sub-agent). The JS gate detects this via `isCheckpointAsk(params)` (case-insensitive scan of `params.questions[].options[].label` and deep `label`/`value`/`id`/`name` fields). General clarification questions that are NOT a checkpoint (no such labels, null/undefined params, or unrelated options) do NOT require synthesis and MUST NOT be blocked — both `wrapSingleTool` and `tool_call` guards `return` early allowing the call directly (optionally advise if thin but never block), matching prompt authority. Preflight and Session Recall allowances remain but are now secondary to this checkpoint filter.
- **Helpers exposed for testing:** `pi._biggzSynthesisGate` exposes `hasSynthesis`, `hasSessionRecall`, `extractArtifactsSection`, `countPaths`, `getArtifactsMetrics`, `isThinSynthesis`, `isAdviseEnabled`, `checkSynthesisPrecondition`, `checkSessionRecallInCurrentTurn`, `hasSessionRecallInHistory`, `isCheckpointAsk`, `extractParamsFromToolCall`, and `_test` helpers. After a successful `original()` the current-turn buffer is reset for the next turn.

**Layer 3 — Tests / CI:**

- **Unit:** `internal/assets/pi/biggz-synthesis-gate.test.mjs` (`node --test`) covers checkpoint-gated blocking (checkpoint missing→`isError:true` not-called, checkpoint rich→pass, general ask→pass without synthesis), thin advise (`BIGGZ_ADVISE=1`→warn pass vs silent), child bypass, same-turn race, strict history-regression (old history must not satisfy), reset-after-success, **load-order race** (pre-registered tool still blocked), **secondary `tool_call` blocking** (`{block:true}` not just warn), `message_end`/`turn_start` tracking, **preflight allowance** (first ask with no prior synthesis ever must NOT block), **Session Recall narrow exception** (same-turn `## Session Recall` → preflight allowed without synthesis, history-only recall must not bypass after delegation), **checkpoint vs general** (`isCheckpointAsk` detect `proceed`/`adjust`/`stop`/`continue`/`correct` case-insensitive, general allows without synthesis), plus helper checks (`isThinSynthesis`, `hasSynthesis`, `hasSessionRecall`, `isCheckpointAsk`, metrics).
- **Integration:** `internal/assets/biggz/orchestrator_test.go` (`go test ./internal/assets/biggz`) reads the embedded `biggz-orchestrator.md` via `assets.FS` and asserts the 4 synthesis markers, `INVALID and will be blocked`, 12× `REMINDER`, and the `## Session Recall` hard gate with `biggz_mem_context`/`biggz_mem_search`/`sdd-status` fallback, failing on drift.
- **CI:** `go vet ./...`, `go test ./...`, `node --check internal/assets/pi/biggz-synthesis-gate.js`, `node --test internal/assets/pi/biggz-synthesis-gate.test.mjs` must be green; `synthesis-gate-status` command reports `✓`/`⚠`/`✗` in the TUI.

Rollback: `git revert` the 5-file commit; no migration.

## OpenCode Plugins

biggz ships 3 OpenCode plugins (full parity with gentle-ai), all embedded
under `internal/assets/opencode/plugins/` and auto-deployed to
`~/.config/opencode/plugins/` by `install.DeployPlugins` (OpenCode auto-loads
local plugin files; no `plugin: []` registration needed):

| Plugin | Job |
|---|---|
| `review-result-artifacts.ts` | Reviewer transport (`biggz review capture-result`) + SDD phase task-result failure handoffs (`biggz-ai.sdd-task-result-failure/v1`) |
| `skill-registry.ts` | On startup runs `biggz skill-registry refresh --quiet --no-gitignore --cwd <dir>` (fingerprint-cached, fire-and-forget) |
| `model-variants.ts` | Writes `~/.biggz/cache/model-variants.json` (atomic tmp+rename) for the effort-level picker |

## Agent Builder

`internal/agentbuilder/` + the TUI `[A]gent builder` flow generate custom
sub-agent SKILL.md files with an AI CLI engine (claude / opencode / gemini /
codex), parse them into kebab-case agents, install them to the engine skills
dirs with rollback, and persist entries to
`~/.config/biggz/custom-agents.json`. SDD-integrated agents additionally get
a `<!-- biggz:custom-agent:<name> -->` reference block injected into the
target agent's system prompt via the same `InjectByMarker` mechanism the
install pipeline uses. Standalone and phase-support SDD modes are wired;
SDDNewPhase mode and the `internal/opencode` Go package (model picker reads
the cache from Go) are deferred.

## Contracts Formalization Layer

The repo-root `contracts/` tree holds frozen JSON Schemas (draft 2020-12)
plus one positive fixture per schema for every wire envelope biggz emits:
`review-integration/v1` (21 schemas) and `sdd-integration/v1` (2 schemas).
The tree is embedded via `contracts/embed.go` (go:embed cannot reach parent
directories from `internal/`), and `internal/contracts` builds the
validation engine on top of it: a compiler that resolves ONLY from the
embedded FS (never the network), an `AddEmbedded` walk that registers every
schema by its declared `$id`, and cached `Schema`/`ValidateJSON`/
`ValidateEnvelope` entry points.

Validation stance (inherited from gentle-ai): the schemas are CI-time
conformance of emitted bytes and test-only helpers — NEVER a runtime path of
the engine. The engine's own strict decoders and content-addressed integrity
checks remain the runtime authority. The layer is additive-only: it cannot
change a ledger byte, proven by `internal/review/ledger_regression_test.go`,
which loads a frozen pre-layer chain and asserts LoadChain, IntegrityVerdict,
PersistedReceipt.Validate, and receiptArtifactOf behave identically with the
layer present. See `contracts/README.md` for the const-vs-`$id` split, the
excluded formats, and the versioning policy.
