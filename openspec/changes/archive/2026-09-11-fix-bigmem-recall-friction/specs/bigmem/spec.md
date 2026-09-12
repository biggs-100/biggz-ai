# Delta for bigmem

## ADDED Requirements

### Requirement: REQ-FR1 — Full Summary Read (1 call)

CLI `bigmem context`/`get` and MCP `mem_context` MUST return a `session_summary` observation's full untruncated content in a single call, by id or for the latest summary. The 150-char summary cut MUST NOT truncate this read path: only the full-read path is untruncated. Search previews MUST remain sanitized and keep the current 120-char truncation; this change MUST NOT alter preview length.

#### Scenario: MCP full summary
- GIVEN a `session_summary` observation longer than 150 chars
- WHEN `mem_context` returns it
- THEN the full content MUST be returned in the same single response

#### Scenario: CLI full summary
- GIVEN the summary observation id (or none for latest)
- WHEN `biggz bigmem get <id>` / `biggz bigmem context` runs
- THEN stdout MUST contain the full content untruncated in one call

#### Scenario: Unknown id fails visibly
- GIVEN an unknown observation id
- WHEN `get` resolves it
- THEN an explicit not-found error MUST surface (non-zero exit), no silent empty

### Requirement: REQ-SC1 — Searchable Session Close (dual-write, idempotent)

`SessionEnd`/`mem_session_summary` MUST persist a `session_summary` observation (findable via empty-query recency) in addition to the `sessions` row; the write MUST be idempotent per session id. On write failure (e.g., Windows lock), the system MUST retry once and, if still failing, surface explicit failure while preserving the `sessions` row; success MUST NOT be reported while the observation is missing.

#### Scenario: Close findable via recency
- GIVEN session `S` closed via `SessionEnd`/`mem_session_summary`
- WHEN empty-query recency runs (`biggz recall` / `Search("",…)`, `updated_at DESC`)
- THEN `S`'s `session_summary` MUST appear with full content

#### Scenario: Idempotent per session id
- GIVEN a close already persisted for `S`
- WHEN close repeats for `S` (retry or CLI fallback)
- THEN exactly one `session_summary` MUST exist for `S` (update in place, no duplicate)

#### Scenario: Locked store retries once
- GIVEN the observation write fails on a locked DB (Windows)
- WHEN close executes
- THEN it MUST retry once, and if still failing surface explicit failure AND keep the `sessions` row (no silent success)

### Requirement: REQ-FTS1 — Multi-Token FTS Robustness

Multi-token search MUST sanitize each token before FTS `MATCH` so hyphens and accents hit (e.g., `gentle-pi`, `sesión`) and MUST NOT parse them as FTS operators/negation. `match_mode=any` MUST return results when any sanitized token matches. A zero-result `match_mode=all` response MUST include an explicit zero-result signal plus a retry hint. Ordering MUST NOT regress (REQ-RR2): non-empty → `ORDER BY rank` (BM25), empty → `ORDER BY updated_at DESC`; limits (default 20, cap 50) MUST remain.

#### Scenario: Hyphenated any-mode query
- GIVEN content matches `gentle-pi` and `ruido` separately
- WHEN `match_mode=any` query `gentle-pi ruido` runs
- THEN matching observations MUST return (no FTS error, no zero)

#### Scenario: Accented query
- GIVEN content contains `sesión`
- WHEN query `sesion` or `sesión` runs
- THEN the observation MUST match

#### Scenario: Zero-result signal and retry hint
- GIVEN no row matches all tokens
- WHEN `match_mode=all` returns zero
- THEN the response MUST explicitly signal zero results AND include a retry hint (e.g., `match_mode=any`)

#### Scenario: Ordering preserved
- GIVEN non-empty and empty queries
- WHEN both run
- THEN non-empty MUST stay rank-ordered, empty MUST stay `updated_at DESC` (REQ-RR2)

## MODIFIED Requirements

### Requirement: REQ-GW1 — Stale Ghost Detection (>5 min)

The system MUST classify `bigmem.db-wal`/`-shm` as stale ghost only when `wal size == 0` AND `shm size > 0` AND `time.Since(shm ModTime) > 5 min` AND the primary liveness probe proves no live holder (e.g., no running `biggz-mcp` holding `bigmem.db`). All conditions MUST hold. When a live holder is detected, the files MUST NOT be classified stale, MUST NOT emit the ghost warning, and MUST NOT trigger `bigmem_recovered` fallback: the primary DB MUST open directly so CLI and MCP read the same store. Fresh (<5 min) or other sizes MUST NOT be stale.
(Previously: shape+mtime only; a live `biggz-mcp` holder could still trigger the warning/fallback.)

#### Scenario: Stale ghost detected
- GIVEN `bigmem.db-wal` 0 B and `bigmem.db-shm` >0 B with `ModTime` 6 min ago and no live holder
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

#### Scenario: Live holder is not ghost
- GIVEN ghost-shaped stale-age wal/shm AND `biggz-mcp` is a live holder of `bigmem.db`
- WHEN `Open` pre-check evaluates
- THEN result MUST be NOT stale
- AND primary `bigmem.db` MUST open without warning and without recovered fallback

### Requirement: REQ-GW2 — Stale Reclaim in Open (O_EXCL + Remove + TRUNCATE)

When REQ-GW1 classifies stale ghost, `Open`/`ResolveDBPath` MUST reclaim only after the liveness probe proves the previous holder is dead (e.g., `os.OpenFile(O_CREATE|O_EXCL)` succeeds); on success MUST `os.Remove` wal and shm, then execute `PRAGMA wal_checkpoint(TRUNCATE)` before `sql.Open`, and MUST open primary DB with no fallback to `bigmem_recovered`. If the probe proves a live holder, reclaim MUST NOT run and wal/shm MUST remain untouched.
(Previously: probe success gated reclaim, but a proved-live holder was not explicitly excluded.)

#### Scenario: Stale reclaimed, primary used
- GIVEN stale ghost and `O_EXCL` probe succeeds
- WHEN `Open` is called
- THEN wal/shm MUST be removed, `wal_checkpoint(TRUNCATE)` MUST run, and primary `bigmem.db` MUST be opened (no recovered warning)

#### Scenario: Checkpoint best-effort
- GIVEN stale ghost reclaim succeeds but checkpoint returns error
- WHEN `Open` continues
- THEN `Open` MUST still succeed and return primary (checkpoint MUST NOT fail open)

#### Scenario: Live holder blocks reclaim
- GIVEN ghost-shaped wal/shm and a proved live holder
- WHEN `Open` is called
- THEN wal/shm MUST NOT be removed
- AND primary MUST open without recovered fallback

### Requirement: REQ-GW3 — Fresh Normal Path, Stale-Busy Fallback

A fresh (<5 min) ghost-shaped wal/shm MUST NOT be classified stale and MUST take the normal open path: no removal, no warning, no `bigmem_recovered` fallback. Only a stale-age ghost (>5 min) whose liveness probe is inconclusive (file exists / locked / race, no live holder proven) MUST NOT remove wal/shm and MUST preserve the existing recovered fallback behavior (`warning` + `bigmem_recovered` merge/promote). Exception: when the probe detects a live holder (e.g., `biggz-mcp` is the only holder), the system MUST skip the warning and the recovered fallback and MUST open the primary DB directly.
(Previously: stale+busy (inconclusive probe) used the recovered fallback; fresh was never stale and never fell back.)

#### Scenario: Fresh ghost normal open
- GIVEN `wal` 0 B / `shm` >0 B but `ModTime` <5 min
- WHEN `Open` is called
- THEN it MUST NOT be classified stale: wal/shm MUST remain untouched and `Open` MUST take the normal primary path (no warning, no recovered fallback)

#### Scenario: Inconclusive probe preserves fallback
- GIVEN stale sizes but the probe is inconclusive (race / Windows lock, no live holder proven)
- WHEN `Open` is called
- THEN wal/shm MUST NOT be removed and fallback path MUST remain intact

#### Scenario: No stale → no removal side-effect
- GIVEN no ghost (e.g., `wal` >0)
- WHEN `Open` is called
- THEN no `Remove` on wal/shm MUST occur

#### Scenario: Live holder opens primary without fallback
- GIVEN ghost-shaped files held live by `biggz-mcp`
- WHEN `Open`/`ResolveDBPath` runs
- THEN no warning and no recovered fallback MUST occur
- AND primary MUST open so CLI and MCP read the same store, with wal/shm untouched
