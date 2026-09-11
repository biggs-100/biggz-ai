# Delta for orchestrator

## MODIFIED Requirements

### Requirement: REQ-RR3 — Session Recall Gate Hardening

Session Boot Recall MUST start with `biggz_mem_context(5)` and MUST answer after at most ONE additional recency call (`biggz recall`/`Search("",…)`, empty query, `ORDER BY updated_at DESC`); total recall reads MUST NOT exceed two calls before the recap is emitted, and recall SHOULD stay within the 1–2 call / ~5k-token KPI. "Where were we?" MUST NEVER trigger chained FTS/keyword searches (`search --query "session"`, token chains) to reconstruct session state; FTS `rank` MUST NOT be used for latest. Fallback when BigMem is empty: `git log --oneline -15` + `sdd-status --json`, fallback noted. `internal/assets/biggz/biggz-orchestrator-workflow.md` MUST document this Recall discipline (`mem_context` + ≤1 recency call → answer; never FTS chains).
(Previously: empty-query recency + FTS ban, without a call budget, stop condition, or workflow-asset clause.)

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

#### Scenario: Bounded recall answers in ≤2 reads
- GIVEN clean session and "en que nos quedamos?"
- WHEN the recall path runs
- THEN `mem_context` + ≤1 recency call MUST identify the latest summary
- AND the recap MUST be emitted without further "where were we" reads

#### Scenario: No chained FTS
- GIVEN the latest summary is already identified
- WHEN the orchestrator considers searching further
- THEN it MUST answer and MUST NOT issue FTS/keyword chains

#### Scenario: Workflow asset documents discipline
- GIVEN `biggz-orchestrator-workflow.md` read
- WHEN searched
- THEN the Recall discipline (`mem_context` + ≤1 recency call → answer; never FTS chains) MUST be present
