---
name: sdd-design
description: Create the SDD technical design — architecture decisions, data flow, file changes, interfaces, and threat matrix. Trigger: orchestrator launches design for a change.
license: MIT
metadata:
  author: biggz-ai
  version: '1.0'
---

# SDD Design

Translate specs into a concrete technical design. Document architecture decisions, interfaces, data flow, file changes, and a threat matrix.

## Activation Contract

1. Read specs — these are the acceptance criteria.
2. Design architecture: interfaces, data flow, file layout.
3. Document every decision with alternatives and rationale.
4. Define testing strategy per requirement.
5. Create a threat matrix (security/performance/failure risks).
6. Persist design artifact.

## Hard Rules

- Every spec requirement (REQ-N) must map to at least one design element.
- Every architecture decision must be documented with: decision, alternatives considered, rationale.
- The file change list must be exhaustive — every file that needs creation or modification.
- The threat matrix must include at least: failure mode, impact, mitigation.
- Never design outside the spec scope — anti-scope from the proposal is a hard boundary.

## Decision Gates

| Gate | Condition | Action |
|------|-----------|--------|
| New interface | Design introduces a public API or exported type | Document interface contract explicitly |
| External dep | Design requires a new library or tool | Flag for approval in the design document |
| Performance concern | Design introduces N+1, O(n^2), or blocking operations | Include performance analysis in threat matrix |
| Data migration | Design changes data schema or storage | Document migration strategy |

## Execution Steps

1. **Load shared protocol** — read `../_shared/sdd-phase-common.md`.
2. **Load proposal and specs** — read `proposal.md` (scope) and `spec.md` (requirements).
3. **Map requirements to design elements** — for each REQ-N, determine: which file(s) change, which interface(s), which data flow(s).
4. **Design architecture** — document:
   - **Component diagram**: modules, packages, files and their relationships.
   - **Data flow**: how data moves through the system (request → handler → store → response).
   - **Interface contracts**: function signatures, type definitions, error returns.
5. **Write design document** — create `openspec/changes/{change-name}/design.md`:
   ```yaml
   ---
   title: "{change-name} — Technical Design"
   status: draft | approved
   ---
   ## Architecture Decisions
   | Decision | Alternatives | Rationale |
   |----------|-------------|----------|
   | {choice} | {list} | {reason} |

   ## Data Flow
   {description of data movement}

   ## Component Design
   {per-component: responsibility, interface, files}

   ## File Changes
   - `path/to/new.go` — create — {purpose}
   - `path/to/existing.go` — modify — {what changes}

   ## Testing Strategy
   | REQ | Test Type | Approach |
   |-----|-----------|----------|
   | REQ-1 | unit | {approach} |
   | REQ-2 | integration | {approach} |

   ## Threat Matrix
   | Failure Mode | Impact | Likelihood | Mitigation |
   |-------------|--------|------------|------------|
   | {failure} | {impact} | low/med/high | {mitigation} |
   ```
6. **Review against specs** — verify every REQ-N from the spec maps to at least one design element. Flag orphans.
7. **Persist** — write design file and Engram entry. Update `_meta.yaml` with `phase: design`.
8. **Recommend next phase** — tasks.

## Output Contract

```yaml
status: success | blocked | needs-approval
executive_summary: "Designed auth middleware: 3 new files, 1 modified. 5 architecture decisions documented."
artifacts:
  - path: openspec/changes/{change-name}/design.md
    type: design-doc
    summary: "Architecture decisions, data flow, file changes, threat matrix"
next_recommended: tasks
risks:
  - description: "External dep flag: needs approval before implementation"
    severity: medium
skill_resolution: auto
```

## References

- `../_shared/sdd-phase-common.md`
- `openspec/changes/{change-name}/proposal.md`
- `openspec/changes/{change-name}/spec.md`
- `openspec/changes/{change-name}/design.md`
