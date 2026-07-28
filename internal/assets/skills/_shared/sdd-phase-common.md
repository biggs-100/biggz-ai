---
name: sdd-phase-common
description: Common protocol loaded by every SDD phase skill. Provides skill loading, artifact retrieval, persistence, and return envelope conventions.
---

# SDD Phase — Common Protocol

Boilerplate identical across all SDD phase skills.

## A. Skill Loading

1. Check if orchestrator injected a `## Skills to load before work` block. If yes, read those exact SKILL.md files.
2. If none provided, search the skill registry as fallback.
3. If no registry, proceed with phase skill only.

## B. Artifact Retrieval

`mem_search` returns 300-char previews — always call `mem_get_observation(id)` for full content. Run searches in parallel, then retrievals in parallel.

## C. Artifact Persistence

### Engram: `mem_save(title, topic_key, type: "architecture", capture_prompt: false, content: ...)`

### OpenSpec: File already written during phase main step.

### Hybrid: Do both.

### None: Return result inline only.

## D. Return Envelope

`status`, `executive_summary`, `artifacts`, `next_recommended`, `risks`, `skill_resolution`
