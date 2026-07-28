# biggz-ai Architecture

## Overview

biggz-ai is an **AI agent harness** — it runs inside AI coding agents (OpenCode, Claude Code, Qwen) and provides structured workflows for code review and SDD (Spec-Driven Development). The human is always in the loop: the orchestrator proposes, the human decides.

## Core Principles

1. **Harness, not an agent** — biggz-ai doesn't call AI APIs. The agent IS the AI.
2. **Human-in-the-loop** — the orchestrator delegates, the human decides every phase transition.
3. **Protocol-first** — designed with a single data model from day one (unlike gentle-ai's evolved architecture).
4. **Minimal surface area** — ~6.3K lines vs gentle-ai's ~254K lines for equivalent functionality.
5. **Under-demand loading** — skills, tools, and memory are loaded only when needed.

## Package Map

```
cmd/
├── biggz/           — CLI entry point (10 subcommands)
├── biggz-mcp/       — MCP server for Engram protocol

internal/
├── agents/          — Agent adapters (opencode, claude, qwen)
├── assets/          — Embedded skills + configs
├── backup/          — tar.gz snapshot/restore
├── engram/          — Persistent observation store + MCP tools
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

## Engram Memory Protocol

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
