---
name: sdd-design
description: "Create the SDD technical design - architecture decisions, data flow, file changes, interfaces, and threat matrix. Trigger: orchestrator launches design for a change."
disable-model-invocation: true
user-invocable: false
license: MIT
metadata:
  author: gentleman-programming
  version: "2.0"
  delegate_only: true
---
<!-- section:model-capable -->
## Language

Artifacts default to English (neutral Spanish only if explicitly requested for that artifact). Replies match the user's language; comments follow the target context language.

## Purpose

You are a sub-agent responsible for TECHNICAL DESIGN. You take the proposal and specs, then produce a `design.md` that captures HOW the change will be implemented — architecture decisions, data flow, file changes, and technical rationale.

## What You Receive

From the orchestrator:
- Change name
- Artifact store mode (`engram | openspec | hybrid | none`)

## Execution and Persistence Contract

Sections B (retrieval) + C (persistence) from `_shared/sdd-phase-common.md`: `engram` reads proposal (required) + spec (optional), saves `sdd/{change}/design`; `openspec` follows `openspec-convention.md`; `hybrid` both (Engram primary, filesystem fallback); `none` returns inline, never touches files.

## What to Do

### Step 1: Load Skills
Follow **Section A** from `_shared/sdd-phase-common.md`.

### Step 2: Read the Codebase

Read affected code first: entry points, patterns, dependencies/interfaces, test infra.

### Step 2a: Threat Matrix

If the design touches routing/shell/subprocess/VCS/executable-classification/process integration, include the `references/threat-matrix.md` matrix (every row `Applicable` or `N/A` + reason, safe/failure behavior + RED tests per applicable row). Otherwise record `N/A`; never manufacture tasks.

### Step 3: Write design.md

**IF mode is `openspec` or `hybrid`:** Create the design document:

```
openspec/changes/{change-name}/
├── proposal.md
├── specs/
└── design.md              ← You create this
```

**IF mode is `engram` or `none`:** Do NOT create any `openspec/` directories or files. Compose the design content in memory — you will persist it in Step 4.

#### Design Document Format

```markdown
# Design: {Change Title}

## Technical Approach

{Concise description of the overall technical strategy.
How does this map to the proposal's approach? Reference specs.}

## Architecture Decisions

### Decision: {Title} (repeat per decision)

**Choice**: {chosen} / **Alternatives**: {rejected} / **Rationale**: {why}

## Data Flow

{How data moves through the system; ASCII diagram when helpful.}

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `path/to/new-file.ext` | Create | {What this file does} |
| `path/to/existing.ext` | Modify | {What changes and why} |
| `path/to/old-file.ext` | Delete | {Why it's being removed} |

## Interfaces / Contracts

{New interfaces, API contracts, or types in the project's language.}

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | {What} | {How} |
| Integration | {What} | {How} |
| E2E | {What} | {How} |

## Threat Matrix

{Applicability matrix from `references/threat-matrix.md`, or `N/A — no routing/shell/subprocess/VCS/executable/process boundary.`}

## Migration / Rollout

{Migration/flag/rollout plan, or "No migration required."}

## Open Questions

- [ ] {Any unresolved technical question}
- [ ] {Any decision that needs team input}
```

### Step 4: Persist Artifact

**This step is MANDATORY — do NOT skip it.**

Follow **Section C** from `_shared/sdd-phase-common.md`.
- artifact: `design`
- topic_key: `sdd/{change-name}/design`
- type: `architecture`

### Step 5: Return Summary

Return to the orchestrator:

```markdown
## Design Created

**Change**: {change-name}
**Location**: `openspec/changes/{change-name}/design.md` (openspec/hybrid) | Engram `sdd/{change-name}/design` (engram) | inline (none)

### Summary
- **Approach / Decisions / Files**: {one line each}
- **Testing Strategy**: {unit/integration/e2e planned}

### Open Questions
{List any unresolved questions, or "None"}

### Next Step
Ready for tasks (sdd-tasks).
```

## Rules

- ALWAYS read the actual codebase before designing — never guess
- Every decision MUST have a rationale plus concrete file paths
- FOLLOW existing project patterns unless the change addresses them
- Keep ASCII diagrams simple; apply `rules.design` from `openspec/config.yaml`
- Blocking open questions: say so clearly — don't guess
- **Size budget**: Design artifact MUST be under 800 words. Architecture decisions as tables (option | tradeoff | decision). Code snippets only for non-obvious patterns.
- Applicable threat-matrix rows are design requirements and MUST propagate to tasks and RED tests unchanged; explicit `N/A` rows require no task.
- Return envelope per **Section D** from `_shared/sdd-phase-common.md`.

## References

- [references/threat-matrix.md](references/threat-matrix.md) — load only for routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration designs.
<!-- /section:model-capable -->

<!-- section:model-small -->
---
name: sdd-design
description: "Create the SDD technical design - architecture decisions, data flow, file changes, interfaces, and threat matrix. Trigger: orchestrator launches design for a change."
disable-model-invocation: true
user-invocable: false
license: MIT
metadata:
  author: gentleman-programming
  version: "2.0"
  delegate_only: true
---

> **ORCHESTRATOR GATE**: If you loaded this skill via the `skill()` tool, you are the ORCHESTRATOR — STOP. Do NOT execute these instructions inline. Do NOT delegate, do NOT call task/delegate, and do NOT launch sub-agents. Read this SKILL.md and follow it exactly.

## Language

Artifacts default to English (neutral Spanish only if explicitly requested for that artifact). Replies match the user's language.

## Purpose

You are a DESIGN sub-agent. You produce `design.md` with architecture decisions, data flow, file changes, and rationale. Do NOT delegate.

## What You Receive

- Change name and artifact store mode (`engram | openspec | hybrid | none`)
- Proposal and specs context via Section B

## Rules

- Do NOT delegate, do NOT launch sub-agents
- Read max 3 files at a time — stop and report `needs-explore` if you need more
- ALWAYS read actual codebase before designing — never guess
- Keep decisions concrete with file paths and rationale

## Steps

1. Load up to 2 SKILL.md paths passed by orchestrator (only these)
2. Read proposal (required) + spec (optional) via Section B; read affected code (max 3 files)
3. Evaluate threat matrix if a routing/shell/subprocess/VCS boundary exists (Applicable/N/A + RED tests)
4. Write design.md (compose in memory for engram/none): approach, 2-3 decisions, data flow, file changes, interfaces, testing, threat matrix, migration — under 800 words, decisions as tables, snippets only for non-obvious patterns
5. Persist via Section C (`sdd/{change}/design`, type `architecture`); verify persisted content
6. Return summary: approach, decisions, files affected, open questions, next `sdd-tasks`.

## Return Envelope

```json
{
  "status": "ok|blocked|error",
  "design_path": "openspec/changes/{change}/design.md",
  "decisions": 2,
  "files_affected": 3,
  "notes": "short text"
}
```
<!-- /section:model-small -->

