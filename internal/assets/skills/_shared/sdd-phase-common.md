---
name: sdd-phase-common
description: Common protocol loaded by every SDD phase skill. Provides skill loading, artifact retrieval, persistence, and return envelope conventions.
---

# SDD Phase — Common Protocol

These sections are shared across every SDD phase skill. Load this file before
reading any phase-specific SKILL.md.

## A. Skill Loading

1. If the orchestrator injected a `## Skills to load before work` block, read
   those exact SKILL.md files first.
2. If none provided, search the skill registry for the phase key as fallback.
3. If no registry exists, proceed with the phase skill only — the orchestrator
   already selected the correct phase.

## B. Artifact Retrieval

1. `biggz_mem_search` returns 300-char previews. Always call `biggz_mem_get_observation(id)`
   for full, untruncated content.
2. Run searches in parallel, then retrieve the matching observations in parallel.
3. For file-based artifacts, read from `openspec/` paths directly.
4. When both Engram and file-system copies exist, prefer the file-system copy
   as the source of truth.

## C. Artifact Persistence

| Backend | Mechanism |
|---------|-----------|
| **Engram** | `biggz_mem_save(title, topic_key, type: "architecture", capture_prompt: false, content: ...)` |
| **OpenSpec** | Write to `openspec/changes/{change-name}/{phase}.md` during phase execution |
| **Hybrid** | Do both — write the file for the orchestrator, save to Engram for cross-session memory |
| **None** | Return result inline in the return envelope only |

Default mode is **Hybrid** unless the phase specifies otherwise.

## D. Return Envelope

Every phase skill returns this envelope on completion:

```yaml
status: success | blocked | failed
executive_summary: "1-2 sentence summary of what happened"
artifacts:
  - path: openspec/changes/{change-name}/{phase}.md
    type: phase-artifact
    summary: "What this artifact contains"
next_recommended: propose | spec | design | tasks | apply | verify | archive | done
risks:
  - description: "Thing that could block or invalidate later phases"
    severity: low | medium | high
skill_resolution: how this skill was resolved (auto | user_input | orchestrator)
```

## E. Review Workload Guard

Before producing tasks or entering apply, estimate the review surface:

1. **Calculate workload**: count new files + modified files + test files.
2. **Apply threshold**: if total > 400 lines across files, recommend splitting.
3. **PR threshold**: if total > 400 lines, recommend chained PRs.
4. **Record estimate**: store in the task or apply artifact as `review_workload_estimate`.
