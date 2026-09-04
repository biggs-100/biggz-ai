# Delta for bigmem

> Decision: `ListRelationsByIDs(ids)` chosen over `GetBatch`/`MGet`. Reason: the N+1 has two halves — unscoped `ListRelations("")` (full-table scan, LIMIT 50 truncation) plus per-endpoint `GetCtx`. `GetBatch` fixes only the second half; the unscoped scan remains. `ListRelationsByIDs` pushes `WHERE source_id IN (...) OR target_id IN (...)` into SQL, fixing discovery and hydration in one scoped query. Missing-counterpart titles resolve from the in-memory result map with `"deleted"` fallback — zero hot-path Gets.

## ADDED Requirements

### Requirement: Scoped relation lookup

The Store MUST provide additive `ListRelationsByIDs(ids []string)` returning relations where source or target is in `ids`. Existing `GetCtx`/`ListRelations` MUST remain unchanged.

#### Scenario: Scoped lookup returns both endpoints

- GIVEN observations A, B with relation A supersedes B
- WHEN `ListRelationsByIDs([A])` runs
- THEN the A→B relation MUST be returned

#### Scenario: Empty input queries nothing

- GIVEN an empty ID list
- WHEN `ListRelationsByIDs([])` runs
- THEN it MUST return empty without querying the relations table

### Requirement: mem_search annotation query bound

`mem_search` MUST annotate relations with at most 2 Store queries regardless of result count N, MUST scope discovery to the union of result IDs on both endpoints, and MUST NOT call `GetCtx` per relation on the hot path.

#### Scenario: Large result set stays bounded

- GIVEN a search returning 50 results
- WHEN relation annotation runs
- THEN at most 2 Store queries MUST occur (1 scoped lookup + ≤1 title batch or zero)

#### Scenario: Cross-ID relations not missed

- GIVEN relation A→Z where only A is in results
- WHEN annotation runs with scope = union of result IDs
- THEN the annotation MUST appear, title from in-memory map or `"deleted"` fallback

### Requirement: Explicit search-limit semantics

`SearchCtx` MUST keep the 50-row cap and default 20. `mem_search` MUST validate `limit` (non-numeric/≤0 → default 20; >50 → clamp to 50) and MUST explicitly signal the effective limit instead of silently clamping.

#### Scenario: Oversize limit clamped visibly

- GIVEN `mem_search` with `limit=100000`
- WHEN search executes
- THEN at most 50 rows MUST return AND the effective limit MUST be signaled (response field or stderr)

#### Scenario: Invalid limit defaults

- GIVEN `mem_search` with `limit=0` or negative
- WHEN search executes
- THEN default 20 MUST apply
