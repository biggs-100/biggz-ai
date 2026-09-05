# biggz-ai — SDD Orchestrator Workflow (lazy-loaded)

This is the lazy-loaded SDD workflow surface for biggz-ai. Read this file before handling `/sdd-*`, natural-language SDD requests, SDD continuation/routing, apply/verify/sync/archive work, or SDD/Judgment-Day phase delegation. The thin `biggz-orchestrator.md` points here; do not duplicate this workflow inline.

## SDD Workflow

SDD phases:

```text
init → explore → research (optional) → proposal → spec → design → tasks → apply → verify → archive
```

Dependency graph:

```text
proposal -> specs --> tasks -> apply -> verify -> archive
             ^
             |
           design
```

`biggz sdd-status [change]` is the read-only status action for resolving active change, artifact paths, task progress, dependency readiness, and action context before apply/verify/archive. Uses native hybrid merge (`internal/sdd/engram_status.go`, port of `resolveEngramStatus`): scans `openspec/changes/` and merges BigMem `sdd/{change}/{proposal,spec,design,tasks,apply-progress,verify-report,archive-report}` with filesystem winning on conflict.

## Native SDD Dispatcher Guard

Before routing, continuing, applying, verifying, or archiving, determine artifact store from cached Session Preflight (check BigMem via `biggz_mem_search` if not established). The native dispatcher (`biggz sdd-status --json --instructions` and `biggz sdd-continue <change>`) is authoritative for `openspec`, `BigMem`, and `hybrid` via native hybrid merge. Invoke dispatcher for all stores and treat `nextRecommended` + dependency states as single authority. Route only by `nextRecommended` and dependency states; never infer from free text.

- If `blockedReasons` non-empty, do not proceed to apply/archive/terminal work.
> Note: SDD V2 projection (biggz-ai.sdd-status/v2) intentionally hides edit_authority_missing from blockedReasons/nextRecommended to keep status authority-free. The orchestrator MUST check raw EditAuthorityBlocked (via non-projected status or sdd-apply guard) before launching apply, or will waste an acquire token on blocked(edit_authority_missing) then surface consent. The sdd-apply guard is authoritative for edit_authority.
- If `nextRecommended` is `verify`, verification/remediation may run only to refresh evidence.
- If `nextRecommended` is `resolve-blockers`, report `blockedReasons` and stop.
- If `nextRecommended` is planning token (`propose`, `spec`, `design`, `tasks`), launch that planning phase.
- If binary unavailable, fall back to manual BigMem schema (`biggz_mem_search` + `biggz_mem_get_observation` on `sdd/{change}/...` topic keys).

## Mandatory Pre-Delegation Reads (LAZY, on-demand)

Before routing, continuing, or delegating an SDD request, read lazily on demand: `biggz-orchestrator-workflow.md` when handling `/sdd-*`/SDD continuation, `biggz-orchestrator-delegation.md` on routing/delegation triggers. Evidence reads in the launch prompt (`## Skills to load before work` + workflow/delegation context). If unreadable, warn and continue with `biggz sdd-status --json --instructions` as authority — do not infer SDD intent from free text and do not launch `sdd-*` without the relevant doc.

## Session Boot Recall (HARD GATE — best-effort in `auto`)

Before SDD Session Preflight, perform Session Recall to restore context. This gate is HARD GATE by default; in `auto` it is best-effort with fallback and never blocks the chain.

> For recency use `bigmem search --query "" ORDER BY updated_at DESC` or `biggz recall`; never use FTS term search for 'latest'.
> FTS rank is for relevance, not recency.

Steps (all mandatory):
1. `biggz_mem_context(limit=5)` — recent sessions (or `biggz recall --limit 5 --json` / `Search("", opts)` → `ORDER BY updated_at DESC` @1801)
2. `biggz recall` / `biggz bigmem recent` / `Search("", opts)` — latest observations ordered by `updated_at DESC` for "en que nos quedamos?" — MUST NOT use FTS `search --query "session"` or `ORDER BY rank` (@1844) for latest
3. `biggz_mem_search(query:"sdd {project}" limit=10)` — SDD artifacts for project (relevance, not recency)
4. Inject summary: synthesize top observations/sessions into short recap (fresh `2026-09-01` before stale `2026-08-27`)
5. Fallback: if BigMem empty/unavailable, run `git log --oneline -15` + `biggz sdd-status --json --instructions` and note fallback (ban FTS for latest even in fallback)

Required markdown after the three calls, before preflight:

```markdown
## Session Recall
**Context Loaded:** {count} observations, {count} sessions
**Project:** {project}
**Recent Summaries:** {summaries or "none"}
**Fallback Used:** {yes/no — if yes, why}
```

REMINDER: Session Recall markdown is separate chat markdown emitted FIRST, adjacent, same turn, before preflight question.

## Pre-Done Session Summary Hook (REQ-SD-S1/S2/S3/S5 — PR2 `internal/sdd/session_guard.go`)

Before closing any `apply` batch and before final `done`/`archive`, the orchestrator MUST call the session guard:

1. **Gate**: `IsSessionSummaryBlocked(ctx, workspaceRoot, change)` / `HasSessionSummary(ctx, project, sessionID)` — if no `session_summary` (MCP `biggz_mem_session_summary` OR bash `biggz bigmem save --type session_summary`) verified, MUST block with `needs_decision` + `blockedReasons=["blocked(session_summary_missing)"]` (`SessionSummaryMissingReason`) and MUST NOT advance to `verify`/`archive`/`done`. Fallback file `openspec/changes/{change}/session-fallback.md` (`FallbackPath`/`FallbackFilePath`) satisfies the gate for next session.
2. **Bash fallback (mandatory when MCP absent)**: when `available_tools` lacks `biggz_mem_*`, route close via `biggz bigmem save --type session_summary` via bash (`saveViaBash` anchored to `workspaceRoot`, 5-case `DetectProjectFull` for `project`, `PutBlob>100k`/`data:image/` → `blob:sha256:` before save). MCP present → `tryMCPSave` via `Store.Save`/`SessionEnd` (no bash).
3. **Verify**: `VerifySessionSummary` / `VerifySessionSummaryWithWorkspace` runs `biggz_mem_context(5)` (`SessionContext(5)`) + `biggz bigmem search --query ""` (`Search("", {Type: session_summary, Limit:5})` `ORDER BY updated_at DESC` not FTS `rank`) — result MUST contain the new `session_summary`; otherwise retry once → `git log --oneline -15` + `biggz sdd-status --json --instructions` anchored to `workspaceRoot` when BigMem empty (does NOT clear gate, observability only).
4. **Complementary**: per-task `biggz_mem_save` after every delegated sub-agent (dedup 15m, `capture_prompt:false` for SDD artifacts, `ShouldExternalize`→`PutBlob`) PLUS `session_summary` on close — per-task alone does NOT satisfy `blocked(session_summary_missing)`.
5. **Retry + degraded**: on save/verify failure retry once; persistent fail → write `session-fallback.md` + deliver answer with note `BigMem unavailable — fallback persisted` (saving≠replying) and retry next session.

Wired in `internal/sdd/status.go` (`deriveChangeStatus` + `deriveChangeStatusWithForcedStore`) after the RDD gate: when `applyState==all_done && coreReady` and `IsSessionSummaryBlocked` true, `Verify`/`Archive` → `DependencyBlocked` + `resolve-blockers` / `nextRecommended: verify` blocked.

## Human Language Detection — `languageHint` (MANDATORY before synthesis)

Detect human language before every synthesis and before injecting `sdd-*` prompts. Use `internal/sdd/synthesis.go:DetectLanguage` heuristic: diacritics `á/é/í/ó/ú/ñ/¿/¡` → `es`; keywords `que/en/por/con/para/continua/dale/procede` vs `hello/continue/proceed/adjust/stop`; short `hi/ok/go/dale` → `en` default (ambiguous → `en`, fallback `DetectLanguage(lastHumanMessage)` or `en`). Store as `languageHint` in session and persist dual-write `pending_question.languageHint` (`BigMem sdd/{change}/pending-question` + `openspec/changes/{change}/state.yaml pending_question`) via `SavePendingDualWrite` / `pending.go` (`biggz-ai.pending-question/v1`). Inject `Human language: es|en — render synthesis content in that language, keep markers English, keep paths/code English` into every `sdd-*` prompt (`sdd-propose`/`sdd-spec`/`sdd-design`/`sdd-tasks`/`sdd-apply`/`sdd-verify`/`sdd-archive`). Synthesis content follows `languageHint`; markers (`## Sub-agent Result:`, `**Artifacts/Paths:**`, `**Risks / Open Questions:**`, `**Next Recommended:**`, `| Topic | Decision |`) and technical identifiers (paths, `sdd/...`, `ORDER BY`, `Search`, code, branches, topic_keys) stay English — helpers (`HasSynthesis`/`isCheckpointAsk`) check verbatim English markers as advise (enforcement retired 2026-09-04); whitelist via `sanitizePlain` never translates. Fallback at render: `RenderSynthesisLocalized(r, languageHint)` or `DetectLanguage(lastHumanMessage)` if hint empty, else `en`.

## SDD Session Preflight (HARD GATE)

Before ANY SDD command or natural-language SDD request, ensure session has explicit `SDD Session Preflight` decision block. `openspec/config.yaml`, existing SDD artifacts, or previous `sdd-init` results do NOT satisfy preflight.

Four required choices (ask once via one grouped `question`/`ask_user_question` tool call):
1. Pace: Interactive, Automatic → `interactive` / `auto`
2. Artifacts: OpenSpec, BigMem, Both → `openspec` / `BigMem` / `both`
3. PRs: Ask me, Single PR, Auto → `ask-on-risk` / `single-pr` / `auto-chain`
4. Review: 400 lines, 800 lines, Other → `review_budget_lines: 400/800/N`

`exception-ok` is reachable only when user explicitly accepts `size:exception`.

Cache choices for session and include in later phase prompts. Interactive vs auto determines gatekeeper vs checkpoint pause.

## SDD Entry Routing

For new product/code change saying "use SDD", start at preflight → init guard → explore/proposal. Never launch `sdd-apply` just because user asked to implement a feature. If intent unclear, run `sdd-research` before `sdd-propose`; its denial/partial blocks proposal.

Only launch `sdd-apply` when: preflight complete, spec+design+tasks artifacts exist, and user explicitly asked to apply/continue or prior planning completed and workload guard passed. Otherwise STOP and propose new change.

## Public Implementation States

The orchestrator MUST expose exactly four public states. These are the only user-facing lifecycle indicators.

| State | Meaning |
|---|---|
| **Working** | Implementation can still change |
| **Checking** | Functional proof and bounded review in progress |
| **Ready** | Exact candidate has sufficient evidence for the delivery route |
| **Needs your decision** | Safe convergence impossible; orchestrator presents cause, impact, and choices |

State transitions:
- `Working` → `Checking` (tests/review launched)
- `Checking` → `Ready` (evidence sufficient)
- `Checking` → `Needs your decision` (ambiguity/failure)
- `Ready` → delivery (commit/PR)
- `Needs your decision` → `Working` (human decides, work resumes)

Public states replace old synthesis lifecycle markers (`◆ phase·status·next`). Synthesis markdown (`## Sub-agent Result`, `**What was done:**`, `| Topic | Decision |`, etc.) remains as record-keeping but is NOT the decision surface — the `question` tool envelope is.

## SDD Init Guard

After preflight and before ANY SDD command, check `sdd-init`:

1. `biggz_mem_search(query: "sdd-init/{project}", project: "{project}")`
2. If found → proceed
3. If NOT found → run `sdd-init` via `sdd-init` sub-agent first

Ensures testing capabilities cached, Strict TDD activated when supported, and project context available.

## Execution Mode

Collected via Session Preflight. If missing, enforce hard gate.

- `auto`: phases run back-to-back; gatekeeper validates autonomously before next phase. User sees interruption only on gate failure.
- `interactive`: after each phase, show concise summary and present `proceed/adjust/stop` via lossless blocking-prompt route. Pause for human confirmation.

Interactive is phase-scoped: "continue"/"dale"/"go on" approves only immediate next phase.

Before `sdd-propose` in interactive, offer proposal question round: 3–5 concrete business/product questions (problem, users, rules, outcome, gap, impact, edge cases, scope boundaries, non-goals, constraints, tradeoffs). Summarize assumptions and ask correct/second-round/continue via lossless route. Do not ask about test commands/PR shape/budget unless user asks.

## Visible Context Before Every Question (MANDATORY)

Never emit a blocking question the human must decide blind. The questionnaire modal shows only the envelope — chat text is not visible at decision time — so the decision context must travel INSIDE the question:

- Context message first, in the human language (`languageHint`), then the question call. Never both without the other.
- Every option carries a substantive `description` explaining what choosing it means (tradeoffs included). Bare labels are rejected fail-closed by `internal/sdd/question.go:ValidateQuestionEnvelope`.
- When options need richer comparison (mockups, snippets, diffs, configs), attach `preview` per option (side-by-side layout, single-select only); it is persisted verbatim by `FormatFallback`.
- Shortcuts (2–4 options, ≤60-char labels, ≤16-char headers) still apply; synthesis markdown stays FIRST and adjacent before any checkpoint ask.

## Automatic Mode Gatekeeper (MANDATORY)

In `auto`, orchestrator is quality gate after each phase before next. Autonomous validation — do not ask user on happy path; stop and report only on problem.

Checks every phase against Result Contract:
- **Contract conformance:** `status`, `executive_summary`, `artifacts`, `next_recommended`, `risks`, `skill_resolution` present and `status` indicates success.
- **Artifact existence:** declared artifact readable in backend (BigMem search+get or file read). Missing → FAIL.
- **No hallucination:** spot-check file paths/symbols/commands/artifacts claimed; unresolved → FAIL.
- **No drift from inputs:** spec within proposal, design answers proposal, tasks cover spec+design, apply implements tasks.
- **Routing coherence:** `next_recommended` follows Dependency Graph and no unaddressed CRITICAL.

Cost-aware validation:
- **Inline for low-risk** (`sdd-explore`, `sdd-spec`, `sdd-tasks`, `sdd-archive`): orchestrator checks inline.
- **Fresh-context validator** (`sdd-design`, `sdd-apply`): validate artifact against inputs only; not adversarial review, no ledger/review transaction.
- **Escalation on smell:** missing artifact/status mismatch/unresolved path/drift → escalate to fresh-context delegated review before deciding.

On PASS: continue to next phase. On FAIL: re-run same phase once with corrective feedback naming specific failures. Re-validate; if fails again, STOP chain and report phase, both attempts, recommended fix. Never advance on failed gate.

One-rerun quality rule is subordinate to Native Runtime Attempt Authority: every rerun still requires fresh `sdd-attempt acquire`; if provider returns `blocked`/`complete`, stop.

A terminal `sdd_task_result_empty`/`sdd_task_result_malformed` (`BIGGZ_AI_SDD_FAILURE` + `biggz-ai.sdd-task-result-failure/v1` JSON) is transport failure, not gate failure: preserve JSON, run `continuation` once, surface typed failure and wait for user decision. Later launch receives `sdd_task_dispatch_latched` — never dispatched; start new session.

## Native Runtime Attempt Authority (MANDATORY) — ledger-bound

Provider-owned Git-common-dir runtime ledger is single attempt/budget authority for `openspec` and `BigMem`; never persist caller-authored counters in artifacts/prompts/Pi state.

1. Before `sdd-apply`/`sdd-verify`/remediation harness: `biggz sdd-attempt acquire --cwd <repo> --change <change> --request-id <id> --work-unit <label> --evidence-goal <goal> --max-attempts <count> --max-changed-lines <count>`.
2. Launch only when acquire returns `state: proceed`; retain opaque `token`. `blocked`/`complete` stops launch.
3. After run: `biggz sdd-attempt settle --cwd <repo> --change <change> --token <token> --request-id <settle-id> --outcome <failed|interrupted|passed> --evidence-revision <sha256:...> --diagnosis <text> --harness-disposition <reused|invalidated> --cleanup-evidence <text> --process-evidence <text>` with distinct request-id per operation. `evidence_revision` (sha256) is ledger-bound and never `none`; settle derives binding/remediation inputs. Pass `--successor-lineage` only for distinct approved successor.
4. Route only from settle's `proceed`/`blocked`/`complete`. `status|begin|finish|reset` are diagnostic; `reset` requires explicit maintainer decision and is never automatic.

## Authority Boundary (native vs external gentle-ai)

`biggz sdd-attempt` is the sole authority inside biggz-ai. Do NOT mix runtimes in one change: never combine biggz ledger tokens with a `gentle-ai` binary ledger, never mirror counters/tokens in artifacts/prompts/Pi state. If a `gentle-ai` binary is present on PATH, treat it as a separate runtime — pick one per change (default: biggz native) and stay on it through `archive`. Interop is file-level only (`openspec/changes/`, BigMem topic keys), never token-level.

## Artifact Store Policy

- `openspec` → file-based `openspec/changes/{change}/`
- `BigMem` (alias `engram`) → persistent memory via `biggz bigmem`
- `hybrid` → both; cross-session recovery + local files
- `none` → inline only; recommend enabling openspec or BigMem

Alias invariant: `engram` is alias for `bigmem` — both refer to same store. Filesystem wins on conflict in hybrid.

## Delivery Strategy

Cached from preflight:
- `ask-on-risk` (default): ask only when tasks forecast detects review-budget risk
- `auto-chain`: automatically split into chained/stacked PR slices when needed
- `single-pr`: proceed as one PR only if within budget
- `exception-ok`: user accepts `size:exception` when over budget — preflight menu cannot select this; reached only via explicit `size:exception` acceptance

Pass `delivery_strategy` to `sdd-tasks` and `sdd-apply`.

## Chain Strategy

When delivery planning yields chained PRs, ask once for chain strategy and cache it:
- `stacked-to-main`: each PR targets previous PR branch or main in sequence
- `feature-branch-chain`: PR #1 targets tracker branch; child PRs target immediate previous PR branch; only tracker merges to main

When chained PRs selected, treat `chained-pr` registry skill as required: resolve by registry path and forward to `sdd-tasks`/`sdd-apply`; do not hardcode path. Includes `bigmem_branch_*` handling for hybrid stores.

Pass as `chain_strategy` alongside `delivery_strategy`.

## SDD Phases (dependency order)

```
proposal → specs → design → tasks → apply → verify → archive
              ↑
            design
```

1. `explore` → exploration.md
2. `propose` → proposal.md (intent, scope, approach, rollback, success criteria)
3. `spec` → openspec/specs/{domain}/spec.md (Given/When/Then)
4. `design` → design.md (architecture, data flow, interfaces, file changes)
5. `tasks` → tasks.md (checklist, workload forecast, work units)
6. `apply` → implement, run tests (`go test ./...`), update `apply-progress` (ledger-bound `evidence_revision`)
7. `verify` → verify-report.md (validate via `biggz sdd-verify-validate`)
8. `archive` → move to `openspec/changes/archive/`

## Research and Pre-Proposal Gate (MANDATORY)

Offer `sdd-research` after `sdd-explore`; selection makes completion mandatory. Before `propose`, invoke only when research `done` or unselected, product decisions `confirmed`, evidence references valid, and artifact-store state ready. Orchestrator owns product discovery. Automatic unresolved choices require one lossless grouped prompt with all context/options/consequences/allowed answers/exact tokens; persist pending pre-proposal state before prompting, then STOP without invoking `sdd-propose`. Proposer receives confirmed handoff and MUST NOT interview/infer consent. Native `biggz-ai.sdd-status/v2` is sole contract; `v1` retired. See `skills/_shared/research-lifecycle.md` for `biggz-ai.sdd-research/v1` and `biggz-ai.sdd-preproposal/v1`.

## Dependency Graph

```
proposal -> specs --> tasks -> apply -> verify -> archive
             ^
             |
           design
```

## Result Contract

Each phase returns: `status`, `executive_summary`, `artifacts`, `next_recommended`, `risks`, `skill_resolution`, `detailed_report` (first 300 chars).

## Review Workload Guard (MANDATORY)

After `sdd-tasks` and before `sdd-apply`, inspect `Review Workload Forecast`.

If `Chained PRs recommended: Yes`, `400-line budget risk: High`, estimated changed lines >400, or `Decision needed before apply: Yes`, apply cached `delivery_strategy`:

- `ask-on-risk`: STOP and ask split vs `size:exception` via lossless route; if chained chosen and `chain_strategy` missing, ask stacked-to-main vs feature-branch-chain
- `auto-chain`: do not ask splitting; if `chain_strategy` missing, ask once, then implement only next autonomous slice
- `single-pr`: STOP and require/record `size:exception` before apply
- `exception-ok`: continue, tell `sdd-apply` this run uses `size:exception`

Invalid `delivery_strategy` → STOP, report unrecognised value, re-collect before `sdd-apply`. Even in `auto`, do not override reviewer burnout protection. Always pass resolved `delivery_strategy`/`chain_strategy`/PR boundary/exception to `sdd-apply`.

## Model Assignments

Read configured models from `opencode.json` at session start and cache.

- `agent.biggz-orchestrator.model` authoritative when set
- `agent.sdd-<phase>.model` authoritative when set
- Otherwise use default runtime model

## Sub-Agent Launch Deduplication (MANDATORY)

Maintain session-scoped `(phase, task-fingerprint)` list; if same pair already launched, do NOT launch again. Emit exactly one launch per distinct task.

## Sub-Agent Launch Pattern & Skill Resolution

Orchestrator resolves skills from registry ONCE per session and caches index (`skill-registry.md` or BigMem `skill-registry`). For each sub-agent, copy matching `SKILL.md` paths under `## Skills to load before work` and instruct to read them BEFORE work. Never invent skill paths.

After delegation, check `skill_resolution`: `paths-injected` ok; `fallback-registry`/`fallback-path`/`none` → re-read registry before next delegation.

## Sub-Agent Context Protocol

Sub-agents get fresh context with NO memory; orchestrator controls context.

| Phase         | Reads                                                   | Writes           |
| ------------- | ------------------------------------------------------- | ---------------- |
| `sdd-explore` | nothing                                                 | `explore`        |
| `sdd-propose` | exploration (optional)                                  | `proposal`       |
| `sdd-spec`    | proposal (required)                                     | `spec`           |
| `sdd-design`  | proposal (required)                                     | `design`         |
| `sdd-tasks`   | spec + design (required)                                | `tasks`          |
| `sdd-apply`   | tasks + spec + design + `apply-progress` (if exists)    | `apply-progress` |
| `sdd-verify`  | spec + tasks + `apply-progress`                         | `verify-report`  |
| `sdd-archive` | all artifacts                                           | `archive-report` |

For required dependencies, sub-agents read directly from backend — orchestrator passes topic keys/file paths, NOT content. Always re-read via `biggz_mem_get_observation`/file before synthesizing.

Non-SDD: orchestrator searches BigMem for prior context and passes in prompt; sub-agent saves discoveries via `biggz_mem_save` before returning.

## Strict TDD Forwarding (MANDATORY)

When launching `sdd-apply`/`sdd-verify`, search `sdd-init/{project}` for `strict_tdd: true`. If set, add: `"STRICT TDD MODE IS ACTIVE. Test runner: {test_command}. You MUST follow strict-tdd.md. Do NOT fall back to Standard Mode."`

## Apply-Progress Continuity (MANDATORY)

When launching `sdd-apply` for continuation: `biggz_mem_search(query: "sdd/{change}/apply-progress")`. If found, add merge instruction: read existing progress via `biggz_mem_get_observation`, merge new progress, save combined — do NOT overwrite.

## Archive Final-State Handoff (MANDATORY)

When launching `sdd-archive`, forward final-state facts for work completed after `apply-progress`/`verify-report` persisted — warnings fixed, blockers resolved, tasks finished, updated counts — with commit/evidence refs. Those artifacts are intermediate snapshots; explicit facts in launch prompt outrank stale claims.

## BigMem Topic Key Format

| Artifact        | Topic Key                          |
| --------------- | ---------------------------------- |
| Project context | `sdd-init/{project}`               |
| Exploration     | `sdd/{change}/explore`             |
| Proposal        | `sdd/{change}/proposal`            |
| Spec            | `sdd/{change}/spec`                |
| Design          | `sdd/{change}/design`              |
| Tasks           | `sdd/{change}/tasks`               |
| Apply progress  | `sdd/{change}/apply-progress`      |
| Verify report   | `sdd/{change}/verify-report`       |
| Archive report  | `sdd/{change}/archive-report`      |

## Output Contract

Every phase returns: `status` (`success`/`partial`/`blocked`), `executive_summary` (1-3 sentences), `artifacts`, `next_recommended`, `risks`, `skill_resolution`, `detailed_report` (first 300 chars).

## Hard Rules

- Never skip phases — follow dependency graph
- Every spec requirement MUST have at least one Given/When/Then scenario
- Every task MUST be specific, actionable, verifiable
- Before apply, run workload forecast; if >400 lines, split into chained PRs
- Verify reports MUST be validated with `biggz sdd-verify-validate`
- Archive only after verify passes
- Phase contracts live in `prompts/sdd/` and matching `skills/sdd-*`

## Review-Driven Development (RDD)

User-owned kill switch: `biggz rdd enable|disable|status`.

- `status` read-only; `disable` stops review-driven development immediately — do not work around it
- While disabled, implement organically without reviews; never invent PASS
- Delivery under disabled switch reports `disabled/unmanaged`, never fabricated approval
- Re-enabling applies to future candidates only

## BigMem Persistent Memory

Proactive save triggers: architecture/decision, team convention, workflow change, library choice with tradeoffs, bug fix (root cause), non-obvious feature, significant artifact creation, config change, discovery/gotcha, pattern, user preference.

Self-check after EVERY task: "Did I make decision/fix/learn convention? If yes, call `biggz_mem_save` NOW."

**DELIVERY GUARANTEE — saving is not replying:** Saving to memory is internal bookkeeping and NEVER counts as answering user. End every turn with complete user-facing answer as final message with NO tool calls after it. Save BEFORE final answer, not after. Memory calls never block/truncate reply — deliver complete answer even if memory fails.

When to search: on "remember"/"recall"/past-work references, call `biggz_mem_context` then `biggz_mem_search` then `biggz_mem_get_observation`. Also proactively when starting work that might have been done before.

Session close: call `biggz_mem_session_summary` with Goal/Instructions/Discoveries/Accomplished/Next Steps/Relevant Files.

Passive capture: close reports with `## Key Learnings` numbered list (1–5 factual sentences ≥20 chars, ≥4 words) for automatic extraction. Sub-agents do same via injected instruction.

After compaction: call `biggz_mem_session_summary` with compacted summary, then `biggz_mem_context`, then continue.

## Recovery

- `BigMem` → resolve state via memory search/get on `sdd/{change}/...` topic keys
- `openspec` → read `openspec/changes/<change>/` artifacts and re-derive via native status engine
- `none` → state not persisted; explain limitation

## Provider Defect Handoff

Defect handoff owned by `biggz-orchestrator-delegation.md`. This workflow provides no summary, alternate report route, or RDD lifecycle instruction.
