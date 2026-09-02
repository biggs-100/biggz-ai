# Delta for bigmem

## Purpose

Harden BigMem close discipline: gate before `done`, bash fallback when MCP absent, `context(5)+search` verification, per-task saves + `session_summary` complementary, retry-once degraded fallback. Ports `gentle-ai/protocol.md` SESSION CLOSE PROTOCOL and `engram/store.go` SessionActivity/DetectProject/dedup/PutBlob/EndSession.

## ADDED Requirements

### Requirement: REQ-SD-B1 — Session-Close Invariant Gate (Q1)

The system MUST block `done` and every closing `apply` batch without prior `biggz_mem_session_summary` or CLI fallback `biggz bigmem save --type session_summary` in same session. `internal/sdd/session_guard.go` MUST enforce; missing MUST return `needs_decision` + `blocked(session_summary_missing)`.

#### Scenario: Final done blocked without summary
- GIVEN session `2026-09-02` has no `session_summary` observation
- WHEN orchestrator attempts final `done`
- THEN gate MUST reject with `blocked` and surface `session_summary required before done`

#### Scenario: Closing apply batch blocked
- GIVEN an `apply` batch completed but no summary persisted
- WHEN closing the batch before next continuation
- THEN gate MUST block until `session_summary` (MCP or bash) succeeds

#### Scenario: Summary present allows close
- GIVEN `session_summary` persisted via MCP or bash fallback
- WHEN `done` or batch-close evaluated
- THEN gate MUST allow

### Requirement: REQ-SD-B2 — Mandatory Bash Fallback When MCP Absent (Q2)

When `available_tools` lacks `biggz_mem_*`, the system MUST fallback to `biggz bigmem save --type session_summary` via bash. Fallback is mandatory. Payload MUST reuse `topic_key`/`type` validation and `capture_prompt:false`.

#### Scenario: MCP present uses MCP
- GIVEN `available_tools` contains `biggz_mem_session_summary`
- WHEN closing session
- THEN system MUST call MCP `biggz_mem_session_summary` (no bash fallback)

#### Scenario: MCP absent triggers bash fallback
- GIVEN `available_tools` lacks `biggz_mem_*`
- WHEN closing session
- THEN system MUST execute `biggz bigmem save --type session_summary --scope project` via bash and treat its persisted ID as satisfying REQ-SD-B1

#### Scenario: Fallback reuses schema validation
- GIVEN bash fallback invoked
- WHEN payload validated
- THEN `topic_key` pattern `sdd/{change}/...` and `type=session_summary` MUST validate same as MCP path

### Requirement: REQ-SD-B3 — Explicit Verification via context(5)+search (Q3)

Before `done`, the system MUST verify via `biggz_mem_context(5)` and `biggz_mem_search`/`biggz recall --query ""` (empty-query `ORDER BY updated_at DESC`). Verification MUST show `session_summary` in results; FTS `rank` MUST NOT be used for recency.

#### Scenario: Verification succeeds
- GIVEN `session_summary` just saved
- WHEN `biggz_mem_context(5)` then `search`/`recent --query ""` executed
- THEN results MUST contain the new `session_summary` with matching `session_id`

#### Scenario: Verification failure triggers retry (see REQ-SD-B5)
- GIVEN context/search do not return the summary within timeout
- WHEN verification evaluated
- THEN system MUST retry once before degraded fallback

### Requirement: REQ-SD-B4 — Complementary Saves (Per-Task + session_summary) (Q4)

Per-task `biggz_mem_save` (proactive triggers, SessionActivity 10m, dedup 15m, PutBlob >100k/`data:image/`, 5-case `DetectProjectFull`, `capture_prompt`) and `session_summary` on close are complementary. System MUST perform both; one MUST NOT replace the other.

#### Scenario: Task save during work
- GIVEN architecture decision made mid-session
- WHEN proactive trigger fires
- THEN `biggz_mem_save` MUST persist immediately (dedup 15m, blob >100k via `blob:sha256:`)

#### Scenario: Close requires summary even if tasks saved
- GIVEN N per-task saves persisted
- WHEN attempting `done` without `session_summary`
- THEN gate MUST still block (REQ-SD-B1)

### Requirement: REQ-SD-B5 — Retry-Once + Degraded File Fallback (Q5)

On `save`/`session_summary` failure or verification miss, the system MUST retry exactly once. If still failing, it MUST deliver user answer with brief note, persist fallback file (`openspec/changes/{change}/session-fallback.md`), and retry next session. Saving is bookkeeping, never a substitute for replying.

#### Scenario: Transient failure recovers on retry
- GIVEN first `save` times out
- WHEN retried once immediately
- THEN second success MUST satisfy gate and verification

#### Scenario: Persistent failure delivers degraded
- GIVEN retry still fails or BigMem unavailable
- WHEN closing
- THEN system MUST return full user answer with note `BigMem unavailable — fallback persisted`, write fallback file, and schedule retry next session
