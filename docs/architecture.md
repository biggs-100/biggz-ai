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
├── assets/          — Embedded skills + configs + OpenCode plugins (opencode plugins + biggz-orchestrator.md [internal/assets/biggz/] + biggz-synthesis-gate.js [internal/assets/pi/biggz-synthesis-gate.js])
├── backup/          — tar.gz snapshot/restore
├── bigmem/          — Persistent observation store + MCP tools (22 agent-facing + 3 internal branching (25 total), SQLite) — Profiles[agent]=20, admin=3 — files: bigmem.go (Store/Search/Save+FTS5/dedup), full.go (sessions/prompts/timeline/stats/doctor), graph.go (BuildGraph/Render*), blobstore.go (PutBlob/GetBlob), sync.go (FileTransport), sync_journal.go (journal/lease), engram_import.go (compat)
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
│   ├── synthesis.go — RenderSynthesis 4+6 markers, DetectLanguage + RenderSynthesisLocalized (es|en heuristic diacritics/keywords, whitelist), ReadLoop >50KB paginated
│   ├── pending.go   — Pending dual-write BigMem + state.yaml (biggz-ai.pending-question/v1)
│   ├── question.go  — ValidateQuestionEnvelope 16/60/4/2-4, IsCheckpointEnvelope
│   └── status_v2.go — biggz-ai.sdd-status/v2 authority-free projection (hides edit_authority_missing from blockedReasons/nextRecommended; orchestrator must check raw EditAuthorityBlocked before apply, sdd-apply guard is authoritative)
├── sddattempt/      — SDD runtime attempt ledger (acquire/settle, CAS, tokens, grants)
│   ├── sddattempt.go — Acquire/Settle/Begin/Finish/Grant/Rescope, BlockedError, SettleObligation
│   └── cas_store.go  — GitCommonDir sdd-runtime/v1/<change>/ record-<sha>.json + HEAD + LOCK
├── platform/        — OS/arch detection + shell quoting (pathidentity, pathquote)
├── policy/          — PolicyEvaluator interface + guardrails
├── project/         — Project detection (git remote → BigMem project pinning)
├── tui/             — Terminal UI (theme, gallery, status-line, ask dialog)
└── skillregistry/   — Skill registry scan + generation (generated at .atl/skill-registry.md and ~/.biggz/skills/, not a static asset)

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

22 agent-facing + 3 internal branching (25 total) MCP tools exposed by `biggz-mcp` plus Session Boot Recall hard gate (Profiles[agent]=20, admin=3):

**Session Boot Recall (HARD GATE):** before SDD Session Preflight the orchestrator MUST run `biggz_mem_context(limit=5)` + `biggz recall` / `Search("", opts)` → `ORDER BY updated_at DESC` (for recency, "en que nos quedamos?") + `biggz_mem_search(query:"sdd {project}" limit=10)` (relevance), inject a short recap, emit `## Session Recall` markdown, and fallback to `git log --oneline -15` + `biggz sdd-status --json --instructions` when BigMem is empty (never use FTS term search for 'latest'). FTS rank is for relevance, not recency. See `internal/assets/biggz/biggz-orchestrator-workflow.md` Session Boot Recall.

**Rank vs Recency:**

| Query | ORDER BY | When | Example |
|-------|----------|------|---------|
| `""` (empty) | `o.updated_at DESC` @1801 | Recency — latest context | `biggz recall --limit 5 --json` or `biggz bigmem recent --limit 5 --json` or `bigmem search --query ""` |
| `"session"` (non-empty) | `rank` @1844 (BM25) | Relevance — keyword search | `bigmem search "session"` or `bigmem search --query "session"` |

> For recency use `bigmem search --query "" ORDER BY updated_at DESC` or `biggz recall`; never use FTS term search for 'latest'.
> FTS rank is for relevance, not recency.

22 agent-facing + 3 internal branching (25 total) MCP tools exposed by `biggz-mcp` (Profiles[agent]=20, admin=3):

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

**CLI notes (biggz bigmem):** save --type enum (bugfix|decision|architecture|discovery|pattern|config|preference|session_summary|etc), --scope project|personal (default project), content >50k truncated [truncated] (see `internal/bigmem/bigmem.go:truncateIfNeeded` → `maxStoredBytes=50000` + `... [truncated]` marker), topic-key hierarchy (`architecture/auth-model`, `decision/api-design`); search --match-mode all|any (default all, MCP validates, CLI forwards to `SearchOptions.MatchMode`), --all canonical alias --all-projects.

**Storage note (F7):** CLI (`biggz bigmem save`) and MCP (`mem_save`) both pre-externalize >100k/`data:image/` via `PutBlob` → `blob:sha256:…` (converged). `Store.Save` is fallback: it avoids truncate for `ShouldExternalize` payloads and stores raw >100k inline if caller didn't externalize; `DoctorFixBlobs` later migrates inline large rows to blobs — see `internal/bigmem/blobstore.go` and `bigmem.go:ShouldExternalize`. Content ≤50k is truncated with `[truncated]`; >100k is blob-externalized eagerly (CLI/MCP) or preserved raw until `DoctorFixBlobs`.

**Search limit (F10):** `Search` caps at 50 (default 20 via `defaultMaxSearchResults`) to allow `BuildGraph(limit=50)` and `biggz bigmem search --limit 50` without silent downgrade to 20 — see `internal/bigmem/bigmem.go:Search`.

**Session note (P2-7):** CLI/MCP pre-call `EnsureImplicitSession` is best-effort; authoritative ownership claim is inside `Store.Save` TX (`resolveWriteProjectTx`). Pre-call errors (especially `ErrProjectOwnershipAmbiguous`) are surfaced, not swallowed — `Store.Save` remains fallback.

**File map (P2-8, verifiable without rg):** `internal/bigmem/*.go` — `bigmem.go` (core Store/Search/Save+FTS5/dedup), `full.go` (sessions/prompts/timeline/stats/doctor), `graph.go` (BuildGraph/Render*), `blobstore.go` (PutBlob/GetBlob), `sync.go` (FileTransport/export), `sync_journal.go` (journal/lease), `engram_import.go` (compat) + `*_test.go` for audit.

**Session discipline (PR2 — `internal/sdd/session_guard.go`):** `session_guard.go` enforces `session_summary before done` — `HasSessionSummary`/`VerifySessionSummary` via `SessionContext(5)` + `Search("")` `ORDER BY updated_at DESC` (not FTS `rank`), mandatory bash `biggz bigmem save --type session_summary` when `available_tools` lacks `biggz_mem_*`, retry-once + `session-fallback.md` + `BigMem unavailable — fallback persisted` + git-log `git log --oneline -15`/`sdd-status --json` fallback when BigMem empty (anchored to `workspaceRoot`). Complementary per-task `biggz_mem_save` (dedup 15m, `PutBlob>100k` → `blob:sha256:`) + `session_summary`; gate `blocked(session_summary_missing)` in `status.go` (`deriveChangeStatus`/`deriveChangeStatusWithForcedStore`) after RDD gate. Empty `$HOME` does NOT fallback to `XDG_RUNTIME_DIR` — `BlobRoot`/`defaultBigmemRoot` return `""` and `PutBlob` errors, raw stored until `DoctorFixBlobs`. See `internal/assets/biggz/bigmem-protocol.md` SESSION CLOSE VERIFICATION and `internal/assets/biggz/biggz-orchestrator-workflow.md` Pre-Done Session Summary Hook.

## SGH Graph Execution

The pipeline implements SGH (Structured Graph Harness) principles:

- **Multi-ready-unit scheduling**: independent nodes run concurrently
- **Immutable plan**: execution plan is fixed for a plan version
- **Three-layer separation**: planning (orchestrator), execution (graph), recovery (human)
- **Cancel on failure**: if one node fails, running nodes are cancelled, completed nodes rollback

## RDD (Review-Driven Development)

Kill switch with three persistence scopes (see `internal/review/rdd.go`, `rdd_helpers.go`):

- **Global**: `~/.biggz/rdd-mode.json` (JSON `rddState` schema `biggz-ai.rdd-status/v1`, fields `mode` + `recorded_at`)
- **Clone**: `<gitCommon>/biggz/rdd-mode/gen-*.json` (CAS generation, `flock` + `O_EXCL` via `rddPublishImmutable`, not `.git/rdd-mode.json`)
- **Worktree**: `<gitDir>/biggz/rdd-mode` (private `gen-*.json` CAS, flock + O_EXCL, only when `worktreeDir != commonDir`; no mirror)
- **Mirror** (read-only fallback): `<gitCommon>/gentle-ai/rdd-mode` (pre-relocation mirror, probed for `ReachMachine` vs `ReachThisBuild`)

**Precedence**: `worktree > clone > global`, any-off-wins. Default `enabled` when no file exists (`decideRDDEffective` returns `enabled` / `SourceDefault`). Any scope `off` disables all gates (`RDDOperationStart`/`Mutate` blocked, `Gate` reports `DeliveryDisabledUnmanaged`). Re-enabling (`biggz rdd enable`) clears clone + worktree generations and writes `enabled` globally (atomic `tmp+rename` + `fsync` + `flock` via `~/.biggz/.rdd-mode.lock` for `~/.biggz/rdd-mode.json`); applies to future candidates only. Status is read-only (`biggz rdd status --json` reports `effective_mode`, `source`, `revision`, `reach`, `worktree_count`). Corrupt global mode (`~/.biggz/rdd-mode.json` invalid JSON/unknown mode) surfaces as `RDDModeUnreadableError`/`ErrRDDModeCorrupt` (not disabled); repair with `biggz rdd enable --scope=global` (doctor `review` check reports `WARNING` with that remedy; clone/worktree corrupt uses `biggz rdd disable --scope=clone/worktree`).

**Hooks**: No git hooks required; RDD gates via `biggz review gate` (`pre-pr`/`pre-push`/`post-apply`/`release`) and `sdd-apply` guard, not via `.git/hooks`. GGA discarded — RDD kill switch + native gates cover hooks; no git hooks needed (see `docs/comparison-with-gentle.md`).

**Burn vs RDD**: `BurnEnabled` (`burned.json`, `DeliveryBurned` per-lineage) is orthogonal to the RDD kill-switch (global/clone/worktree mode). One is per-lineage burn (ephemeral receipt after `finalize`); the other is the global/clone/worktree enable/disable mode. `triviallyInert` symbol is absent in biggz-ai.

### Synthesis Gate (3-layer defense) + Session Recall

Guarantees the human sees audited `## Sub-agent Result` before deciding. Gate `b0d2fc1` enforces the Post-Delegation Human Checkpoint. Session Boot Recall (`## Session Recall`) is a preceding hard gate that restores prior session context via BigMem before preflight. Synthesis rendering uses `internal/sdd/synthesis.go` `RenderSynthesis` / `RenderSynthesisLocalized` (with `DetectLanguage` heuristic) with 4 required markers + 6 optional (Preview/Diff/Decisions/Commands/Validation/Failure), omit-empty, >50KB `ReadLoop` paginated `ReadAt` with verify-retry, `ValidateQuestionEnvelope` 16/60/4/2-4, `pending` dual-write BigMem + `state.yaml` (`biggz-ai.pending-question/v1`), `PI_SUBAGENT_CHILD=1` bypass, and `BIGGZ_ADVISE=1` thin-advise.

**Language Boundary (harness vs artifact):** Harness prompts stay English; synthesis content is localized per human language (`languageHint` / `Human language: es|en — render synthesis content in that language, keep markers English, keep paths/code English`), stored via `pending_question.languageHint` dual-write (`BigMem sdd/{change}/pending-question` + `state.yaml`, `biggz-ai.pending-question/v1`) and injected into every `sdd-*` prompt with fallback `DetectLanguage(lastHumanMessage)` or `en` at render via `RenderSynthesisLocalized`. Markers (`## Sub-agent Result:`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**`, `| Topic | Decision |`) and technical identifiers (paths, `sdd/...`, `ORDER BY`, `Search`, code, branches, topic_keys) stay English — gate `b0d2fc1` (`HasSynthesis`/`isCheckpointAsk`) validates verbatim English markers; whitelist via `sanitizePlain` never translates them. `isCheckpointAsk` scans option labels only, not synthesis content, so Spanish synthesis with English markers passes; missing marker blocks regardless of language.

**Layer 1 — Prompt (machine-verifiable invariant):** `internal/assets/biggz/biggz-orchestrator.md` contains a copy-pasteable block with 4 markers (`## Sub-agent Result: {phase/agent}`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**`) and states `INVALID and will be blocked` for any **checkpoint** `ask_user_question`/`question` (those presenting `proceed`/`adjust`/`stop` or `continue`/`correct` after a delegated sub-agent) without immediately preceding markdown. General clarification questions that are NOT a checkpoint do NOT require synthesis and MUST NOT be blocked — e.g. '¿por dónde empezamos?', preflight, or other orchestration clarifications not presenting a delegated result use the question tool directly without synthesis. Every checkpoint `ask` reference is followed by `REMINDER: synthesis markdown is separate chat markdown emitted FIRST...` (12× convergence) so prompt and gate cannot drift.

> ENFORCEMENT RETIRED (2026-09-04): blocking proved unfulfillable from the
> agent side (same-turn side-channel + body-text false positives) and is now a
> passthrough in Go (`ShouldBlock`/`ShouldBlockApplyAdmission` always false)
> and JS (wrappers call through). Context-before-question is governed by the
> explicit ask contract in the orchestrator delegation doc, not by code.
> Helpers (`HasSynthesis`, `isCheckpointAsk` labels-only, `FormatFallback`)
> and their unit tests stay as living documentation. Full doc-sync pending.
>
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
