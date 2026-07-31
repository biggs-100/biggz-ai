---
description: Create or update an OpenCode skill using the bundled skill-creator workflow
agent: biggz-orchestrator
subtask: true
---

Load `skill-creator` first, then use it to create or update an OpenCode skill from the user's request.

HARD GATE:
If the request is ambiguous, ask one focused clarification before editing files. Do not guess the skill's purpose, description, or frontmatter.
