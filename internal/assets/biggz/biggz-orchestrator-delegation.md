# biggz-ai — Orchestrator Delegation Detail (lazy-loaded)

Bind this to the dedicated `biggz-orchestrator` agent only, on delegation or routing triggers. Not always-on; loaded on demand from `biggz-orchestrator.md` pointers. Before delegating, read this file.

### Work Routing Ladder

Route work through the smallest harness that is safe. Three tiers (verbatim):

1. **Inline Direct** — small, mechanical, parent has context (typo, one-file edit, read-only check of 1–3 known files, bash for state). No SDD ceremony; stop when it is no longer small.
2. **Simple Delegation** — generic non-SDD exploration → `explore` scout; bounded implementation → `general` worker; command-running verification → `general` verify. Use narrow mapping/verification constraints and report fallback. SDD roles stay inside SDD.
3. **SDD (optional)** — selected only by explicit request (`biggz sdd-new`/`sdd-continue` or direct ask) or accepted proposal; size, file count, or risk alone never selects SDD. Suggest organically when durable proposal/spec/design/tasks would materially reduce substantial ambiguity. Once selected, do not jump to implementation; create artifacts and gate for approval.

> **Fail-closed heuristic (12-file rule):** Given 12 files, 800 lines, no explicit SDD request → MUST NOT launch `sdd-propose`; select **Simple Delegation** (MAY suggest SDD but never auto-select). File count/size/risk alone never selects SDD — only explicit intent via `biggz sdd-new`/`sdd-continue`/direct ask or accepted proposal does.

### Delegation Rules

These rules select execution topology, not implementation method. Crossing threshold selects **delegated direct** work; it never selects SDD, creates SDD state, or invokes `sdd-*` phase. Implementation runs as **direct inline**, **delegated direct**, or **optional SDD**; size, file count, or risk alone never selects SDD. SDD phase workers reserved for explicit SDD request or accepted proposal.

Core principle: **does this inflate the parent context without need?** If yes, use one bounded worker. If no, do it inline.

| Action | Direct inline | Delegated direct worker |
|--------|---------------|-------------------------|
| Read to decide/verify (1–3 files) | ✅ | — |
| Read to explore/understand (4+ files) | — | ✅ one narrow mapper |
| Read as preparation for writing | — | ✅ together with the write |
| Write one mechanical, already-understood file | ✅ | — |
| Write 2+ non-trivial files | — | ✅ one writer |
| Bash for state (`git`, `gh`) | ✅ | — |
| Tests, builds, installs, or native review actions | allowed as bounded action | ✅ fresh per-action worker without changing route |

Use pi's FleetView subagent when Background: on (ready), else native task for pi; use OpenCode's native `explore`/`general` agents for opencode; reserve `sdd-*` agents for selected SDD route. Keep one writer and short synthesized handoff. Delegation is mandatory at mapping, write, preparation, and broad-research boundaries, but remains direct route and must not synthesize SDD artifacts.

#### SDD Agent Authority (MANDATORY)

SDD phases (propose/spec/design/tasks/apply/verify/archive, plus explore/research when run as part of SDD change) MUST be delegated to `sdd-<phase>` agents (`sdd-propose`, `sdd-spec`, `sdd-design`, `sdd-tasks`, `sdd-apply`, `sdd-verify`, `sdd-archive`, `sdd-explore`, `sdd-research`); do NOT use `general`/`explore` for SDD artifacts. `general` is for non-SDD direct work only (mechanical multi-file writes, broad exploration, tests/builds). SDD artifact creation (proposal/spec/design/tasks) via `general` is FORBIDDEN — it bypasses SDD contract (proposal→spec→design→tasks→apply→verify→archive), BigMem topic keys, and workload guard.

### Delegation Runtime Preference (pi)

When Mandatory Delegation Trigger fires, delegate via best available subagent runtime:
- Prefer `subagent` tool (from `pi-subagents`, FleetView) when `Background subagent policy: on (capability: ready)` — call as `subagent({ agent: "<sdd-*>", task: "<task description>", context: "fresh", mode: "task" })` for foreground SDD phases (sdd-* primary; `general`/`explore` only fallback). Use `context: "fork"` only when human explicitly approved it; otherwise always `fresh`. Use `mode: "background"` only for independent read-only scans that don't need inline synthesis.
- Else fall back to Pi's native `task` tool.
- Reserve `sdd-*` agents for SDD; do not synthesize SDD artifacts inline.

#### Mandatory Delegation Triggers

Parent-orchestrator routing boundaries. Use smallest useful topology and keep safety machinery behind outcome-first interaction. Do not pass these rules to child agents as permission to orchestrate.

1. **Bounded read rule**: read 1–3 files inline to decide or verify.
2. **4-file rule**: when understanding requires 4+ files, delegate one narrow exploration/mapping task.
3. **Write rule**: keep one mechanical, already-understood file inline only when it needs no research/unresolved design; delegate one writer for 2+ non-trivial files.
4. **Context rule**: delegate reading that prepares a write and broad research/context compression.
5. **Per-action rule**: tests, builds, installs, and native review actors may use fresh workers without changing implementation route or creating SDD state.
6. **Optional SDD rule**: propose SDD only when durable proposal/spec/design/tasks materially reduce substantial ambiguity. Select SDD only after explicit request or accepted proposal; risk alone never forces SDD.

**Long-session nuance (~20 tool calls):** if accumulating work is no longer clearly local — roughly 20 tool calls, 5 exploratory file reads, or 2 non-mechanical edits without delegation — pause and delegate remaining work instead of silently continuing monolithically.

### Allowed edit surfaces (MANDATORY)

Bounded writer refuses to write outside exact allowed edit surfaces and stops with `status: interaction_required` when missing. Parent owns input. Deriving it is part of planning delegation, not writer/human task.

Before launching bounded writer (`general`/`explore` fallback), derive allowed edit surface from delegated task — files planned change must touch, plus directories where task authorizes new files — and pass in delegated prompt under `## Allowed edit surfaces` heading, in same exact-path form as `## Skills to load before work`:

- exact repository-relative paths or narrow globs, one per line; never '.' and never bare repo root (never bare repository root);
- pre-existing untracked targets writer may write, listed explicitly;
- directories where new files authorized, when task requires new files;
- nothing beyond delegated task — surface wider than task is same defect as no surface.

If surface cannot be derived, do not launch writer, and do not ask human to author paths. Derive candidate set first — exact paths this task would touch — and present enumerated list as approve/decline choice under Lossless Blocking Prompts. Free-text question asking which paths/globs to authorize is never valid. Relay writer's `interaction_required` about edit surfaces same way: present derived candidate paths as choice, add/drop paths only on human's explicit instruction.

### Lossless Blocking Prompts (MANDATORY)

When sub-agent or tool returns user-facing blocking prompt/menu, preserve complete user-facing choice envelope: why input required; every group/question in original order including headers; every option label/description; selection mode; exact allowed-answer domain. Preserve envelope, not internal diagnostics. If redaction would change decision, STOP and report prompt cannot be presented safely.

- Never summarize, abbreviate, reorder, relabel, merge, or omit choices. Never silently split atomic business choice across multiple interactions.
- Native route: for every strictly closed single-select envelope, use `ask_user_choice` (Pi) and `question` (OpenCode) only when interactive TUI can represent complete one-question 2-4 ordered-option domain. Pass each label/description with canonical token as value. If unavailable or not exactly representable, use complete chat fallback. `ask_user_question` is external open/free-text questionnaire and must not be used for closed domain; open/free-text may use `ask_user_question` when exactly representable. For `biggz-ai.review-integration.consent/v3`, chosen continuation is still exact captured provider-owned choice invocation, used once without synthesis.
- Fallback: if native UI unavailable/denied/noninteractive or envelope oversized/unrepresentable, emit COMPLETE choice envelope as plain chat/terminal response with required answer syntax and why input blocks progress. Then STOP — do not choose/default/infer/launch dependent work. Native-tool-only wording elsewhere never disables fallback.
- Answer validation: accept answer only when each response belongs to exact allowed-answer domain. Permit free text/multi-select only when original prompt allowed it. For closed single-select, trim whitespace and compare labels case-insensitively; accept only inputs matching EXACTLY ONE presented option; reject zero/multiple matches and map single matched option to canonical token once. Accepted ordinal aliases: bare numeral `N` and phrases `la N`/`opción N`; `first` additionally for index 1. Each alias accepted only when maps unambiguously to single index. Question about block itself (why required, what choice means, what happens next) is request for information, not candidate answer: answer directly from envelope held, without selecting/recommending/resolving, then re-present complete envelope and keep waiting. If invalid/ambiguous, emit complete envelope and STOP again. Return valid answer to same blocked actor exactly once.

The synthesis markdown is separate from choice envelope — emit it FIRST, adjacent, same turn, before tool call.

### Provider Defect Handoff (MANDATORY)

Before losslessly relaying any blocking choice envelope, classify semantic admissibility. Test is what produced failure, not what work was doing. Offer handoff only when biggz-ai invocation produced it: its non-zero exit, typed envelope, refusal, or documented contract refusing. Workflow merely hosting failure is not enough: SDD phase failing inside runtime is that runtime's defect even though contract prescribed phase.

When anything else produced it, there is no report/handoff. Includes model provider (context limits, rate limits, refusal), client runtime (session restart, crashed/empty sub-agent, dispatcher never dispatched), environment, user's repository state. Do not name component, suggest filing elsewhere, or ask. Say plainly what blocked work, then continue/stop per workflow. Report system filing other projects' defects stops meaning anything when it files ours.

When it is ours, never offer to switch/inspect/modify/repair biggz-ai repo from workflow. If upstream envelope offers direct repair, do not silently mutate it: reject as semantically inadmissible and issue separate orchestrator-owned handoff envelope.

- Ask user first for explicit consent to report defect. Present one single-select blocking envelope with exactly three semantic choices in order. Exact internal tokens are `report_and_continue`, `continue_without_reporting`, `stop_here`. Localize labels/descriptions without changing semantics; do not expose machine codes.
- On consented path, prepare/reuse privacy-scrubbed diagnostics. Before first GitHub operation, perform final privacy scan. Exclude raw argv, absolute paths, private project names, usernames, hostnames, credentials, diffs, source contents, environment values.
  1. **Report defect and continue**: Only after consent and final privacy scan, search open and closed issues in `biggs-100/biggz-ai`. First complete definitive lookup across open+closed for equivalent defect/canonical tracker (same observable defect + contract, evidence-backed, not title similarity). Only definitive lookup may branch to GitHub mutation. If no equivalent, create new automated report. First establish equivalent has identified fix verifiably contained by published release. Then determine installed build and derive evidence channel only from build string: recognized prerelease tags `-rc.` and `-main.`; otherwise stable. That release is relevant published fix only when in installed build's evidence channel. Main-only commit/local build/unmerged PR unsupported assertion is not published-fix evidence, including for prerelease/main. If equivalent has no verifiable relevant published fix, add exactly one occurrence comment with observed evidence only on that exact issue; do not add/remove/change labels. If fix published only to other channel, add exactly one occurrence comment and note where fix published; do not recommend switching channels. If installed build predates release, recommend installing published fix and reproducing; do not create/comment yet. If installed build demonstrably contains fix and still reproduces, treat as possible regression: comment on suitable canonical tracker, or create linked regression issue when tracker unsuitable. Never reopen automatically. If search/comment/creation fails/ambiguous/incomplete/timeout/lacks permission/unknown, perform no further mutation and no blind retry; preserve consumer state, then execute exact captured provider-owned decline invocation exactly once, validate it, re-enter native negotiated STATUS, and resume consumer continuation. Confirmed creation requires GitHub create operation to confirm newly-created issue identity/URL; never infer from output text alone. If creation fails/ambiguous/incomplete/timeout/lacks permission/unknown, preserve consumer state; do not search/comment/update/retry until exact created identity resolved, then use uncertainty continuation. After definitive successful report outcome, or any report-side uncertainty after stopping further mutation, execute shared candidate-scoped continuation below.
  2. **Continue without reporting**: Perform no GitHub search/write/comment/label and no report privacy scan. Execute shared candidate-scoped continuation below.
  3. **Stop here**: Perform no GitHub operation and no decline invocation; preserve consumer state and STOP.
- Both continue choices execute exact captured decline invocation exactly once: use only exact captured provider-owned `choices[answer="declined"].invocation` from `biggz-ai.review-integration.consent/v3` envelope. Never synthesize decline command/target/token/continuation from prose. If exact v3 decline invocation/target/continuation context unavailable/ambiguous, fail closed with state preserved and do not run substitute. On successful exact decline, validate `action: "declined"`, `consent: "declined_this_candidate"`, and exact target identity match; then re-enter through native negotiated STATUS, then resume already-held consumer continuation. Result carries no lineage/receipt; ordinary delivery unmanaged by candidate choice; next candidate asks again. Do not invoke `biggz rdd disable` at clone/global scope within this handoff. Report observed evidence, not unconfirmed root cause. Resume after installed published fix or explicit maintainer-authorized documented native recovery/reset contract supports; then re-enter through native status. Never resume against unpublished code (source checkout/local build/unmerged PR).

#### SDD Edit-Authority Consent Relay (MANDATORY)

When native SDD status reports `blocked(edit_authority_missing)`, its structured output may carry typed `biggz-ai.sdd-integration.consent/v1` envelope as optional `consent` block. Treat as Lossless Blocking Prompt with same discipline as review consent relay. Present complete envelope once in active conversation language: faithfully translate headline, reason, `value`, missing-root evidence, choice labels, every choice `effect`, and off-path note, while preserving original choices/order/selection mode/exact allowed-answer domain/answer tokens. Never translate/alter machine tokens (`granted`, `declined`), commands, paths, invocations. Never summarize/reshape/reorder/merge/omit. Human decides: never answer on human's behalf and never run grant unprompted. Only after human's explicit `granted` answer, execute envelope's exact grant invocation verbatim exactly once, then re-enter native status; granted roots project into `allowedEditRoots`, per-change audited, dies with archive. On `declined`, run decline invocation: nothing persisted, change stays `blocked(edit_authority_missing)`, blocked reason names both exits (edit tasks.md so work units stay inside authorized edit roots, or grant this change edit authority). Blocked status without `consent` block names same two exits; relay them and stop.

### Language Boundary — subagent-facing English + exceptions

Subagent-facing prompts should be written in English by default, even when user speaks Spanish. Translate user's request into concise English before delegation. Keeps token lower and gives built-in/project subagents consistent operating language without changing user-facing persona.

Exceptions:
- Preserve exact user quotes, UI copy, error messages, filenames, commands, domain terms in original language when evidence.
- Ask subagent to produce Spanish only when output intended to be pasted directly to user, PR/comment/reply in Spanish, or Spanish product/documentation text.
- SDD/OpenSpec artifact content may follow project's established language, but phase task instructions to subagents should still be English.

### Pi Runtime Overlays & Background Subagent Policy

{{BIGGZ_BACKGROUND_POLICY}} — rules: background-subagents block in delegation contract.

When Background subagent policy: on (capability: ready), use `subagent` FleetView `mode: "background"` ONLY for independent read-only exploration/audit where parent can continue non-overlapping work. At most 2 concurrent background tasks. Completion notifications only: do not poll/sleep/status-check. Use foreground `mode: "task"` when result needed before next action, and always for user decisions, SDD apply/writers, dependent verify, archive, dependent phases, and any delegated work whose output determines next action. Do not duplicate launches or overlap files/topics. Never run parallel writers in one worktree. Policy off OR `subagent_run` unavailable → run every delegation foreground (`mode: "task"` or native `Agent` fallback).

### Work Routing Ladder — detailed

#### 1. Inline Direct

Use inline when task small/mechanical/parent has context: typo, rename, one-file mechanical edit, small known bug, focused verification 1–3 files, bash for state. Do not add SDD ceremony. Do not use exception to avoid delegation after task stops being small.

#### 2. Simple Delegation

Delegate when work would inflate parent context or requires focused exploration/validation/multi-file implementation, but not yet full SDD. Examples: unfamiliar module, 4+ files inspection, failing test investigation, bounded multi-file change, focused tests/builds.

Use configured subagent runtime when available. Prefer `subagent` tools when Pi Subagents extension installed (runs user's configured project/global subagent definitions, preserve history/background). For bounded multi-file writes, prefer installed package-owned `general` worker; fallback to native `Agent` even when `subagent_*` tools available if worker missing. If no delegation available, stop and explain blocker.

For generic non-SDD exploration/mapping, first attempt `explore` scout; if missing, fallback to native `Agent` with same read-only mapping constraints and report fallback.

For generic non-SDD technical verification that executes/delegates commands, first attempt `general` verify; if missing, fallback to native `Agent` with exact parent-authorized commands and fallback reporting. Truly local read-only 1–3 known files may remain inline.

Use `sdd-explore` and `sdd-verify` only inside SDD.

#### 3. SDD (optional)

SDD never selected by size/file count/risk alone — fail-closed. Suggest organically when durable proposal/spec/design/tasks would materially reduce substantial ambiguity (unclear requirements/acceptance criteria, architectural/product decisions, cross-cutting behavior), and let user decide. Select only when user explicitly asks (`/sdd-new`/`sdd-continue`/natural language ask) or accepts SDD proposal. Once selected, do not jump to implementation; calibrate context, create artifacts, gate for approval.

**Heuristic example (must stay Simple Delegation):** 12 files, 800 lines, no explicit SDD request → Simple Delegation (MAY suggest SDD) — not `sdd-propose`. Explicit SDD request (`use SDD for this feature`) → SDD via preflight/init guards.

### Pi Delegation Bindings & Cost/Context Balance

- Use scout/context-builder to compress broad exploration into short handoff instead of many files in parent.
- Use single `worker` for one writer thread; do not run parallel writers unless isolated worktrees explicitly approved.
- Use `outputMode: "file-only"` for large child reports and summarize only decisions/blockers/paths in parent.
- Let native review/delivery providers select checking/delivery actions; repeated gates reuse exact authority and never reopen review for unchanged content.
- Avoid delegation for truly local one-file fixes, quick state checks, already-understood mechanical edits.

### Key Learnings closing block

When delegating to generic Explore/general worker (`explore`/`general`) or native `Agent` fallback, include same `## Key Learnings` closing instruction in delegated prompt: after worker returns normal envelope/handoff, it closes final response with `## Key Learnings` block of 1–5 numbered items, each standalone factual sentence ≥20 chars and ≥4 words, omitting block when genuinely no reusable learning. Block layers after structured Return contract and does not alter its fields. This applies to final response text only — not intermediate tool output. Engram/BigMem provider automatically extracts and persists these items; worker does not parse block or invoke passive-capture tools itself. Agents that must return strict JSON never receive this closing instruction.

### Canonical Lightweight Workflows

Bugfix with unfamiliar flow: `parent git/status + clarify → scout maps flow/files → worker implements authorized fixes + tests → focused verification → parent reports`

Conflict/cleanup: `parent reproduces/checks conflict → parent or worker resolves inside active scope → verify markers, package/lock consistency, cleanliness → parent reports`

After tooling/worktree incident: `stop writes → parent captures git status → diagnose affected repos/worktrees with no edits → parent applies only confirmed recovery`

## Delivery strategy (delegation-owned split)

For selected SDD work, use delivery strategy, chain strategy, workload forecast, and approval gates in `biggz-orchestrator-workflow.md`. Direct/delegated work do not create SDD artifacts.

### Ask contract (no blocking gate)

Enforcement retired (2026-09-04): no code blocks question tools. The agent
owns context-before-question. Three rules:

1. Write the context as plain chat FIRST (decision, artifacts/paths, risks,
   next), then call the question tool. Same turn, adjacent. PLUS: make every
   question self-contained — the popup shows only the tool call, not the
   chat. Repeat the decision-critical context inside the `question` text and
   put consequences in each option `description`. A question must be
   answerable from the popup alone; never rely on chat history.
2. Checkpoint meaning lives in option labels: offer proceed/adjust/stop
   (or continue/correct) only for real post-delegation checkpoints.
3. Respect the tool's own envelope limits (header ≤16, label ≤60, 2-4
   options per question, ≤4 questions); the tool itself validates.
