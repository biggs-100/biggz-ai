# biggz-ai Architecture

## Overview

biggz-ai is an **AI agent harness** — it runs inside AI coding agents (OpenCode, Claude Code, Qwen) and provides structured workflows for code review and SDD (Spec-Driven Development). The human is always in the loop: the orchestrator proposes, the human decides.

## Core Principles

1. **Harness, not an agent** — biggz-ai doesn't call AI APIs. The agent IS the AI.
2. **Human-in-the-loop** — the orchestrator delegates, the human decides every phase transition.
3. **Protocol-first** — designed with a single data model from day one (unlike gentle-ai's evolved architecture).
4. **Minimal surface area** — ~36K production Go lines vs gentle-ai's ~112K, and ~60K vs ~313K including tests (measured 2026-08-10).
5. **Under-demand loading** — skills, tools, and memory are loaded only when needed.

## Package Map

```
cmd/
├── biggz/           — CLI entry point (SDD, review, RDD, memory, backup, release verbs)
├── biggz-mcp/       — MCP server for BigMem protocol

internal/
├── agents/          — Agent adapters (opencode, claude, qwen)
├── agentbuilder/    — Custom SDD agent generation (engines, parser, installer, registry)
├── assets/          — Embedded skills + configs + OpenCode plugins
├── backup/          — tar.gz snapshot/restore
├── BigMem/          — Persistent observation store + MCP tools
├── contracts/       — Wire-envelope formalization engine (test-only validation)
├── filemerge/       — Atomic write, JSONC merge, section injection
├── install/         — Agent detection + deploy
├── lens/            — Review lenses (risk, readability, reliability, resilience)
│   ├── gitdiff/     — Shared git diff parsing
│   ├── risk/        — R1: static analysis
│   ├── readability/ — R2: code quality heuristics
│   ├── reliability/ — R3: test coverage, error handling
│   └── resilience/  — R4: timeouts, context, concurrency
├── release/         — Version tagging + verification
├── review/          — Review lifecycle + RDD
└── sdd/             — SDD native commands
└── skillregistry/   — Skill registry scan + generation

model/               — Core types (ReviewState, FSM, evidence chain)
orchestrator/        — Review lifecycle orchestration
pipeline/            — Sequential + DAG graph execution
plugin/              — LensPlugin + AgentAdapter interfaces
plugintest/          — Test helpers
policy/              — PolicyEvaluator interface
registry/            — Build-time plugin registry
```

## Data Flow

### Review Pipeline

```
stdin (ReviewSubject JSON)
  → cmd/biggz: parse
  → orchestrator.Execute()
    → ReviewState (Pending)
    → Transition to InProgress
    → Graph.Execute()  ← parallel DAG (SGH multi-ready-unit)
      ├── RiskLens        \
      ├── ReadabilityLens  ├── all run in parallel
      ├── ReliabilityLens  │   (no dependencies)
      ├── ResilienceLens  /
      └── PolicyEvaluator ← depends on all lenses
    → Transition to Completed (or Failed on error)
    → Compute MerkleRoot
  → stdout (ReviewState JSON)
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

## State Machine

5 states with external policy evaluation:

```
Pending → InProgress → Completed → Archived
  ↓                        ↓
  └──→ Failed             └──→ InProgress (correction cycle)
```

Transitions are validated by the FSM. Business rules are evaluated by PolicyEvaluator, not embedded in transitions.

## BigMem Memory Protocol

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
