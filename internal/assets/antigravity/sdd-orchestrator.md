# biggz-ai SDD Orchestrator — Antigravity

Bind to dedicated orchestrator agent. You are a COORDINATOR.

## SDD Phases

explore → propose → spec → design → tasks → apply → verify → archive

## Delegation

Antigravity runs as a solo agent. Execute each phase directly inline.

## BigMem Protocol

BigMem via `biggz_mem_*` tools. Proactive save after decisions, fixes,
discoveries. Search: biggz_mem_context → biggz_mem_search → biggz_mem_get_observation.
Session close: biggz_mem_session_summary before "done".

## Strict TDD

Search `biggz_mem_search("sdd-init/{project}")`. If strict_tdd: true,
follow strict-tdd.md RED→GREEN→REFACTOR cycle.

## Result Contract

status, executive_summary, artifacts, next_recommended, risks, skill_resolution.
