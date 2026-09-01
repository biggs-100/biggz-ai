# Delta for orchestrator

## ADDED Requirements

### Requirement: REQ-RR3 — Session Recall Gate Hardening

Session Boot Recall MUST use empty-query recency (`biggz_mem_context(5)`/`biggz recall`/`Search("",…)`) for "en que nos quedamos?" plus `git log --oneline -15` and `sdd-status --json` fallback; MUST NOT use FTS.

#### Scenario: Recent wins

- GIVEN `2026-09-01` summary exists
- WHEN gate runs
- THEN synthesis includes `2026-09-01`, not stale `2026-08-27`

#### Scenario: Fallback

- GIVEN BigMem empty
- WHEN gate runs
- THEN `git log --oneline -15` and `sdd-status --json` run, fallback noted

#### Scenario: No FTS for latest

- GIVEN "en que nos quedamos?"
- WHEN resolving latest
- THEN helper used, never `search --query "session"`

### Requirement: REQ-RR4 — Agent Prompt Guardrail

Prompt (`APPEND_SYSTEM.md` via `install.go` + `bigmem-protocol.md`) MUST contain literal string: For recency use `bigmem search --query "" ORDER BY updated_at DESC` or `biggz recall`; never use FTS term search for 'latest'.

#### Scenario: Prompt contains

- GIVEN prompt file read
- WHEN searched
- THEN literal guardrail MUST be present

#### Scenario: Install preserves

- GIVEN `biggz install`
- WHEN `APPEND_SYSTEM.md` regenerated
- THEN guardrail stays inside `<!-- biggz:bigmem-protocol -->`

#### Scenario: TUI visible

- GIVEN TUI help/protocol view
- WHEN rendered
- THEN guardrail visible
