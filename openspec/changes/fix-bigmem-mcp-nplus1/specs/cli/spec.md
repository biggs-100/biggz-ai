# Delta for cli

## ADDED Requirements

### Requirement: Paged export with explicit cap

`biggz bigmem export` MUST page through `Search` in chunks (respecting the 50-row Store cap) instead of a single `Limit:100000` call, MUST accept `--limit N` (explicit row cap) and `--project P` flags, and MUST write the complete capped set to the file stream.

#### Scenario: Export beyond 50 rows completes

- GIVEN a store with 120 observations
- WHEN `biggz bigmem export out.json` runs
- THEN `out.json` MUST contain all 120 observations

#### Scenario: Explicit cap honored

- GIVEN a store with 120 observations
- WHEN `biggz bigmem export out.json --limit 60` runs
- THEN `out.json` MUST contain exactly 60 observations

#### Scenario: Project filter forwarded

- GIVEN observations in projects P1 and P2
- WHEN `biggz bigmem export out.json --project P1` runs
- THEN only P1 observations MUST be exported

### Requirement: Export shape and conflicts preservation

Export output MUST keep the current JSON array-of-observations shape so `biggz bigmem import` round-trips without change. The `conflicts` CLI MUST keep calling unscoped `ListRelations("")` with byte-identical output format.

#### Scenario: Import round-trip

- GIVEN `out.json` produced by paged export
- WHEN `biggz bigmem import out.json` runs
- THEN all exported observations MUST re-import with zero parse errors

#### Scenario: Conflicts output unchanged

- GIVEN pending relations
- WHEN `biggz bigmem conflicts list` runs before and after the change
- THEN output format and row content MUST be identical
