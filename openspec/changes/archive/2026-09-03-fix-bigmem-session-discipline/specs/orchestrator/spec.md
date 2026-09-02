# Delta for orchestrator

## Purpose

Document and enforce BigMem session discipline in orchestrator workflow: mandatory `session_summary` gate before `done`, bash fallback when MCP absent, explicit `context(5)+search` verification, complementary per-task + summary saves, and retry-once degraded fallback. Updates `bigmem-protocol.md` and `docs/architecture.md`.

## ADDED Requirements

### Requirement: REQ-SD-O1 — Protocol Docs and Architecture Note

`internal/assets/biggz/bigmem-protocol.md` MUST document under SESSION CLOSE PROTOCOL: mandatory `biggz_mem_session_summary` before every closing `apply` batch and final `done`, bash fallback `biggz bigmem save --type session_summary` when `available_tools` lacks `biggz_mem_*`, and verification via `biggz_mem_context(5)` + `search --query "" ORDER BY updated_at DESC`. `docs/architecture.md` MUST add session-discipline note referencing `internal/sdd/session_guard.go`.

#### Scenario: Protocol contains gate table
- GIVEN `bigmem-protocol.md` rendered
- WHEN searched
- THEN it MUST contain `SESSION CLOSE PROTOCOL`, `biggz_mem_session_summary`, `biggz bigmem save --type session_summary`, and `biggz_mem_context(5)` + `search --query ""`

#### Scenario: Arch note present
- GIVEN `docs/architecture.md` read
- WHEN searched
- THEN it MUST contain `session_guard.go` and `session_summary before done`

### Requirement: REQ-SD-O2 — Workflow Gate Wiring (Q1+Q3)

Orchestrator workflow (`biggz-orchestrator-workflow.md` / `internal/sdd/*`) MUST wire pre-done hook: before `done`, verify `context(5)+search`; missing MUST block with `needs_decision`. Fallback to file (`openspec/changes/{change}/session-fallback.md`) when BigMem unavailable is allowed only after retry-once, with note.

#### Scenario: Workflow blocks done until verified
- GIVEN no verified `session_summary` in `context(5)`/`search`
- WHEN `biggz sdd-status --json --instructions` evaluated for `done`
- THEN orchestrator MUST report `blocked(session_summary_missing)` and NOT emit `done`

### Requirement: REQ-SD-O3 — Complementary + Retry Discipline Visibility (Q4+Q5)

Orchestrator MUST surface complementary discipline in synthesis: per-task saves count + `session_summary` status. On persistent BigMem failure, it MUST note `retry once failed — fallback persisted`, deliver answer, and schedule next-session retry (manual recovery `obs-1788387626730819800-1` no longer needed).

#### Scenario: Synthesis shows both layers
- GIVEN session with 3 per-task saves and 1 `session_summary`
- WHEN synthesis `## Sub-agent Result` rendered
- THEN `Artifacts/Paths` MUST list `sdd/{change}/...` topic keys and `APPLICABLE` fallback path

#### Scenario: Degraded path visible
- GIVEN retry-once still failed
- WHEN closing
- THEN synthesis `Risks` MUST contain `BigMem unavailable — degraded fallback persisted` and answer delivered
