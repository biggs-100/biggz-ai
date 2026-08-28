# Delta for bigmem

## ADDED Requirements

### Requirement: REQ-GW1 — Stale Ghost Detection (>5 min)

The system MUST classify `bigmem.db-wal`/`-shm` as stale ghost only when `wal size == 0` AND `shm size > 0` AND `time.Since(shm ModTime) > 5 min`. All three MUST hold. Fresh (<5 min) or other sizes MUST NOT be stale.

#### Scenario: Stale ghost detected
- GIVEN `bigmem.db-wal` 0 B and `bigmem.db-shm` >0 B with `ModTime` 6 min ago
- WHEN `isGhostWAL` / `Open` pre-check evaluates
- THEN result MUST be stale ghost

#### Scenario: Fresh ghost not stale
- GIVEN same sizes but `ModTime` 30 s ago
- WHEN evaluation runs
- THEN result MUST be NOT stale

#### Scenario: Non-ghost sizes not stale
- GIVEN `wal` >0 B or `shm` ==0 B (any mtime)
- WHEN evaluation runs
- THEN result MUST be NOT stale

### Requirement: REQ-GW2 — Stale Reclaim in Open (O_EXCL + Remove + TRUNCATE)

When REQ-GW1 is stale, `Open`/`ResolveDBPath` MUST probe liveness via `os.OpenFile(O_CREATE|O_EXCL)`; on success MUST `os.Remove` wal and shm, then execute `PRAGMA wal_checkpoint(TRUNCATE)` before `sql.Open`, and MUST open primary DB with no fallback to `bigmem_recovered`.

#### Scenario: Stale reclaimed, primary used
- GIVEN stale ghost and `O_EXCL` probe succeeds
- WHEN `Open` is called
- THEN wal/shm MUST be removed, `wal_checkpoint(TRUNCATE)` MUST run, and primary `bigmem.db` MUST be opened (no recovered warning)

#### Scenario: Checkpoint best-effort
- GIVEN stale ghost reclaim succeeds but checkpoint returns error
- WHEN `Open` continues
- THEN `Open` MUST still succeed and return primary (checkpoint MUST NOT fail open)

### Requirement: REQ-GW3 — Fresh/Busy Preservation and Race Fallback

If ghost is fresh (<5 min) OR `O_EXCL` probe fails (file exists / locked / race), the system MUST NOT remove wal/shm and MUST preserve existing recovered fallback behavior (`warning` + `bigmem_recovered` merge/promote).

#### Scenario: Fresh ghost preserved with fallback
- GIVEN `wal` 0 B / `shm` >0 B but `ModTime` <5 min
- WHEN `Open` is called
- THEN wal/shm MUST remain and `ResolveDBPath` MUST follow recovered fallback path if blocked

#### Scenario: Busy O_EXCL preserves fallback
- GIVEN stale sizes but `O_EXCL` fails (concurrent holder / Windows lock)
- WHEN `Open` is called
- THEN wal/shm MUST NOT be removed and fallback path MUST remain intact

#### Scenario: No stale → no removal side-effect
- GIVEN no ghost (e.g., `wal` >0)
- WHEN `Open` is called
- THEN no `Remove` on wal/shm MUST occur

### Requirement: REQ-GW4 — Deferred WAL Checkpoint in Save/Search

After successful `Save` or `Search` (rows closed), the system MUST defer `PRAGMA wal_checkpoint(TRUNCATE)` as best-effort, non-blocking cleanup to bound WAL after bursts. Failure MUST NOT propagate to caller. Checkpoint MUST NOT run on error paths.

#### Scenario: Save defers checkpoint
- GIVEN `Save` succeeds (insert/update)
- WHEN `Save` returns
- THEN a deferred `wal_checkpoint(TRUNCATE)` MUST have been attempted (WAL bounded)

#### Scenario: Search defers checkpoint after rows close
- GIVEN `Search` succeeds and rows are consumed/closed
- WHEN `Search` returns results
- THEN deferred `wal_checkpoint(TRUNCATE)` MUST have been attempted

#### Scenario: Checkpoint failure does not fail operation
- GIVEN `Save` succeeded but checkpoint returns `busy`/`locked`
- WHEN caller receives result
- THEN `Save` MUST still report success

#### Scenario: No checkpoint on Save error
- GIVEN `Save` fails (e.g., constraint error)
- WHEN error is returned
- THEN checkpoint MUST NOT be attempted

## MODIFIED Requirements
_None — existing REQ-1..REQ-8, REQ-B1..B8 unchanged._

## REMOVED Requirements
_None._

## RENAMED Requirements
_None._
