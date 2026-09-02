# Delta for sdd

## Purpose

Wire SDD orchestrator gate to enforce `session_summary` before every closing `apply` batch and final `done`, with bash fallback, explicit `context(5)+search` verification, complementary save discipline, and retry-once degraded fallback. Implements PR1 (`internal/sdd/session_guard.go`, pre-done hook) and PR2 (`context/search` readback, docs).

## ADDED Requirements

### Requirement: REQ-SD-S1 — Orchestrator Gate Before Apply-Close and Final Done (Q1)

The orchestrator MUST call `session_guard` before closing any `apply` batch and before final `done`. If `HasSessionSummary` is false and bash fallback not verified, it MUST block continuation with `needs_decision` and `blockedReasons=["session_summary_missing"]`, and MUST NOT proceed to next phase or `archive`.

#### Scenario: Apply batch close blocked
- GIVEN `sdd-apply` batch finished but `biggz_mem_session_summary` not yet persisted
- WHEN orchestrator evaluates pre-close hook
- THEN `nextRecommended` MUST be `resolve-blockers` with `session_summary_missing` and no phase advance

#### Scenario: Final done blocked recovers after summary
- GIVEN gate blocked `done` for missing summary
- WHEN `session_summary` (MCP or bash) verified
- THEN gate MUST clear and `done`/`archive` MAY proceed

### Requirement: REQ-SD-S2 — Mandatory Bash Fallback Routing (Q2)

When `available_tools` lacks `biggz_mem_*`, orchestrator MUST route close via `biggz bigmem save --type session_summary` bash fallback. The fallback MUST be mandatory and use `PutBlob >100k` + `EndSession` parity (`engram/store.go` 8897L, `SessionActivity` 10m nudge, `DetectProjectFull` 5 cases).

#### Scenario: MCP missing triggers bash path
- GIVEN `available_tools` lacks `biggz_mem_session_summary`
- WHEN closing `apply` batch
- THEN orchestrator MUST exec `biggz bigmem save --type session_summary --project <proj> --json` via bash

#### Scenario: MCP present skips bash
- GIVEN MCP tool available
- WHEN closing
- THEN orchestrator MUST use MCP path and MUST NOT invoke bash fallback

### Requirement: REQ-SD-S3 — Explicit Verification context(5)+search (Q3)

Before allowing `done`, orchestrator MUST run `biggz_mem_context(5)` and `search` (empty-query `recent`/`Search("",…)` ordered `updated_at DESC`, not FTS) plus `sdd-status --json` fallback when BigMem empty. Results MUST contain the new `session_summary`; otherwise MUST be treated as verification failure.

#### Scenario: Verification passes via context
- GIVEN summary saved with `--project biggz-ai`
- WHEN `biggz_mem_context(5)` and `biggz bigmem search --query "" --limit 5` executed
- THEN output MUST list the summary; `done` MUST be allowed

#### Scenario: Empty BigMem fallback
- GIVEN BigMem `context`/`search` empty
- WHEN verification runs
- THEN `git log --oneline -15` and `biggz sdd-status --json --instructions` MUST run and fallback noted, and `done` MUST remain blocked until summary appears

### Requirement: REQ-SD-S4 — Complementary Discipline (Per-Task + Summary) (Q4)

Orchestrator MUST enforce per-task `biggz_mem_save` after every delegated sub-agent (proactive triggers + delivery guarantee) plus `session_summary` on close. Per-task saves MUST use dedup 15m, `capture_prompt` rules, and `compaction` → `EndSession` + `mem_context` recovery. Summary MUST NOT be skipped even if per-task saves exist.

#### Scenario: Delegated sdd-spec completed
- GIVEN `sdd-spec` sub-agent returned
- WHEN synthesis emitted before checkpoint
- THEN orchestrator MUST persist per-task save (or bubble sub-agent save) before next phase

#### Scenario: Summary still required
- GIVEN per-task saves verified in `biggz_mem_context`
- WHEN closing session without `session_summary`
- THEN gate MUST remain blocked per REQ-SD-S1

### Requirement: REQ-SD-S5 — Retry-Once + Degraded Fallback + Delivery Guarantee (Q5)

On save/verify failure, orchestrator MUST retry once. If still failing, it MUST deliver the user-facing answer anyway with brief failure note, write degraded fallback file (`openspec/changes/{change}/session-fallback.md`), and not block reply on memory operation. Next session MUST retry persistence.

#### Scenario: Retry succeeds
- GIVEN first `session_summary` call failed or timed out
- WHEN retried once
- THEN success MUST satisfy REQ-SD-S3 and allow `done`

#### Scenario: Degraded deliver with note
- GIVEN retry still fails
- WHEN closing
- THEN orchestrator MUST deliver complete answer with note `BigMem save failed — fallback persisted, will retry next session` and write fallback file; `review` gate MUST record persisted evidence or note
