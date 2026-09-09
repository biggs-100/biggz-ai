---
name: sdd-new
description: "Trigger: none — meta-command handled inline by the orchestrator. Do NOT load as skill; follow prompts/sdd/sdd-new.md directly."
license: MIT
metadata:
  author: biggz-ai
  version: '2.0'
---

# SDD New (meta-command — do not invoke as skill)

`sdd-new` is handled INLINE by the orchestrator, never delegated and never
loaded as a skill. Execute `internal/assets/prompts/sdd/sdd-new.md` directly
in the orchestration thread: verify SDD init, scaffold the change directory,
and route to explore or propose. This stub exists only so skill listings and
registries resolve the name.
