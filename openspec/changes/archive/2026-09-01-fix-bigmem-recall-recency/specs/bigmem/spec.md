# Delta for bigmem

## ADDED Requirements

### Requirement: REQ-RR5 — Docs & Protocol Rank vs Recency

`docs/architecture.md` + `bigmem-protocol.md` MUST document table `ORDER BY rank` vs `ORDER BY updated_at DESC`, when to use each, examples `search --query "session"` vs `search --query ""`/`biggz recall`; help MUST warn.

#### Scenario: Table present

- GIVEN docs read
- WHEN searched
- THEN each MUST contain rank vs `updated_at DESC` table + examples

#### Scenario: Help warns

- GIVEN `biggz bigmem search --help` / `recent --help`
- WHEN rendered
- THEN it MUST note recency uses empty query `updated_at DESC`

#### Scenario: Ordering invariant

- GIVEN `bigmem.go` read
- WHEN checking 1801/1844
- THEN `1801` has `ORDER BY o.updated_at DESC`, `1844` has `ORDER BY rank`
