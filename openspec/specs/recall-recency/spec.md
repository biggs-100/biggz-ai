# recall-recency Specification

## Purpose

Recency-only latest-context path via `ORDER BY updated_at DESC` (empty query @1801); FTS `ORDER BY rank` (@1844) unchanged for relevance.

## Requirements

### Requirement: REQ-RR1 — Recency Helper Command

System MUST provide `biggz recall` / `biggz bigmem recent` via `Search("", opts)` ordered `updated_at DESC`; limit caps 50; forwards filters.

#### Scenario: Empty query recency

- GIVEN obs `2026-08-27` and `2026-09-01`
- WHEN `biggz recall --json --limit 5`
- THEN `2026-09-01` first

#### Scenario: Type filter

- GIVEN `session_summary` + `decision`
- WHEN `recent --type session_summary --json`
- THEN only `session_summary` by `updated_at DESC`

#### Scenario: Project filter

- GIVEN projects `biggz-ai` / `other`
- WHEN `recall --project biggz-ai --json`
- THEN only `biggz-ai`

#### Scenario: Limit cap 50

- GIVEN `--limit 100`
- WHEN run
- THEN at most 50 returned

#### Scenario: JSON vs human

- GIVEN with/without `--json`
- WHEN run
- THEN human shows lines, JSON valid array with `updated_at`

### Requirement: REQ-RR2 — No Regression on FTS Relevance

Non-empty query MUST use `ORDER BY rank` (BM25 @1844); empty MUST use `updated_at DESC`.

#### Scenario: Rank ordered

- GIVEN query `session`
- WHEN `Search("session", opts)`
- THEN ordered by `rank`

#### Scenario: Empty recency

- GIVEN query `""`
- WHEN `Search("", opts)`
- THEN ordered by `updated_at DESC`

## Non-Goals

- No `--order` flag; no FTS algorithm change
