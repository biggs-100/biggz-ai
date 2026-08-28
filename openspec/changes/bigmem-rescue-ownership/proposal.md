# Proposal: bigmem-rescue-ownership — Port Engram Rescue-Ownership to BigMem

## Intent

Legacy sessions with `project` NULL/blank block writes (`ErrProjectRequired` or split). BigMem only has `inferBigMemProject` basename fallback — no `resolveWriteProjectTx` or `RescueNullProjectOwnership`. Port Engram's atomic session adoption so requested project can claim orphans in same TX and bulk-rescue.

## Scope

### In Scope
- `internal/bigmem/bigmem.go`: `ErrProjectRequired`, `ErrProjectOwnershipAmbiguous`, `foreignRecordOwnerTx`, `resolveWriteProjectTx(sessionID, requestedProject)`, `adoptSessionOwnershipTx`, `RescueNullProjectOwnership`+`Plan`/`Result`, sync enqueue if `sync_mutations` exists
- `Save` calls `resolveWriteProjectTx` before dedup (BEGIN IMMEDIATE TX)
- `cmd/biggz/cli_bigmem.go`: `biggz bigmem rescue-ownership --project X [--session Y] [--dry-run] [--json]`
- Tests: adoption, ambiguous fail, bulk rescue

### Out of Scope
- Sync journal/cloud, TUI `/branch`, graph/FTS, blobstore
- Auto-adoption on read; non-session observations

## Capabilities

### New Capabilities
- `bigmem-rescue-ownership`: null-project session adoption + bulk rescue (2-phase Plan, dry-run, JSON)

### Modified Capabilities
- `bigmem`: `Save` resolves via `resolveWriteProjectTx`; ambiguous parent fails loud with hint

## Approach

- **P1 Per-write:** `Save` → `resolveWriteProjectTx` (TX). If `sessions.project IS NULL/''` and no `observations.project != requested` → `adoptSessionOwnershipTx` (`UPDATE`+enqueue `sync_mutations` if exists). Else `ErrProjectOwnershipAmbiguous` hint `biggz bigmem rescue-ownership --project X --session Y`.
- **P2 Bulk:** `RescueNullProjectOwnership(project)` lists `WHERE project IS NULL OR trim(project)=''`, 2-phase `Plan` (counts/IDs/ambiguous), then adopts per-session TX. Adapt Engram schema + `inferBigMemProject`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/bigmem/bigmem.go` | Modified | Ownership errors, resolve/adopt/rescue, Plan |
| `cmd/biggz/cli_bigmem.go` | Modified | `rescue-ownership` command |
| `internal/bigmem/*_test.go` | Modified | Adoption + rescue tests |
| `internal/project/detect.go` | Referenced | 5-case reuse, no change |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Race concurrent Save | Medium | Single TX + `Store.mu`, idempotent UPDATE |
| Ambiguous silently adopted | Medium | `foreignRecordOwnerTx` checks `observations`; fail loud with hint |
| Sync divergence if not enqueued | Low | Probe `sqlite_master` for `sync_mutations`; enqueue if exists |

## Rollback Plan

`git revert <sha>` — additive `UPDATE` only, no DDL. Backup `~/.biggz/bigmem/backup-*.db` or `export` reversible; orphans stay NULL.

## Dependencies

- Batches S `a09e872`, M `12b751b`, 5-case `53a98cf` on main; `modernc.org/sqlite`; Engram ref adapted

## Success Criteria

- [ ] Legacy NULL session saved after `resolveWriteProjectTx` adoption in same TX
- [ ] Ambiguous parent fails `ErrProjectOwnershipAmbiguous` with rescue hint
- [ ] `RescueNullProjectOwnership` adopts N orphans; Plan dry-run matches apply
- [ ] `go vet ./...` + `go test ./internal/bigmem -count=1 -timeout 180s` green

## Proposal question round

Interactive (answer/skip/correct): 1) bulk skip or fail? (skip) 2) per-Save implicit? (yes) 3) sync per session? (yes) 4) `unknown` rescuable? (no, only NULL/'' )
