---
name: sdd-ff
description: "Trigger: none — meta-command handled inline by the orchestrator. Do NOT load as skill; follow prompts/sdd/sdd-ff.md directly."
license: MIT
metadata:
  author: biggz-ai
  version: '2.0'
---

# SDD Fast-Forward (meta-command — do not invoke as skill)

`sdd-ff` is handled INLINE by the orchestrator, never delegated and never
loaded as a skill. Execute `internal/assets/prompts/sdd/sdd-ff.md` directly
in the orchestration thread: planning artifacts in sequence, then
implementation. Requires the user's explicit acknowledgment that individual
phase reviews are skipped. This stub exists only so skill listings and
registries resolve the name.
