# Proposal: bigmem-sync-v2 — Ordered Sync Journal for BigMem

## Intent

BigMem sync is only `sync_chunks` with no journal, ordered log, ack, backoff, lease, quarantine or deferred. Port Engram's project-scoped journal (`sync_mutations`, `sync_state`, `sync_apply_deferred`) as local foundation for reliable cloud sync. BigMem stays local-only. Follows `rescue-ownership` dd5f731.

## Scope

### In Scope
- `internal/bigmem/bigmem.go`: `sync_mutations` (seq, project, entity, entity_key, op, payload, source, disposition), `sync_state` (target_key, lifecycle, last_enqueued/acked/pulled, failures, backoff_until, lease_owner/until, reason_code), `sync_enrolled_projects`, `sync_apply_deferred` (5→dead), optional `cloud_upgrade_state`
- API: `enqueueSyncMutationTx`, `ListPending`/`Ack`, `ApplyPulledMutation`, `ReplayDeferredForScope`, `QuarantineIrreparable`, `Acquire/ReleaseSyncLease`, `MarkSync*`, `SyncMutationDisposition`
- Tests + CLI `biggz bigmem sync --status` (lifecycle per target)

### Out of Scope
- Cloud server `pgx`, dashboard, TUI `/branch`, autosync Manager goroutine, blob/graph/FTS

## Capabilities

### New Capabilities
- `bigmem-sync-journal`: ordered project journal, enrolled projects, deferred 5→dead + dead-letter hash, quarantine, lease/backoff

### Modified Capabilities
- `bigmem`: coexists with `sync_chunks`; enriches `sync --status`; no DDL on `observations`

## Approach

4 PRs stacked-to-main, <400 lines each:
- **PR1** `sync_mutations`/`sync_state`/`enrolled` DDL + `enqueue` + `ListPending`/`Ack`
- **PR2** `sync_apply_deferred` + `ApplyPulled` + `ReplayDeferred` (5→dead + hash)
- **PR3** `Quarantine` (deterministic, cursor advances, `degraded`+`reason_code`) + `Acquire/ReleaseSyncLease` (TTL 1m) + `MarkSync*`
- **PR4** `cloud_upgrade_state` only if needed else omitted; each PR `go vet` + `go test ./internal/bigmem` green

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/bigmem/bigmem.go` | Modified | Journal DDL, migrate, enqueue/ack/apply/deferred/quarantine/lease |
| `internal/bigmem/sync.go` | Modified | Keep `sync_chunks` transport; add journal status |
| `internal/bigmem/*_test.go` | Modified | Journal, deferred, quarantine, lease tests |
| `cmd/biggz/cli_bigmem.go` | Modified | `sync --status` lifecycle output |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Log blocked by irreparable | High | `Quarantine` → `deferred dead`, cursor advances |
| Deferred FK cycle | Medium | `ApplyPulled`→`deferred` on `ErrFKMissing`; `ReplayDeferred` retries; 5→dead |
| Lease split-brain | Low | `Acquire` denies if `lease_until>now && owner!=req`; release only if owner matches |
| `sync_chunks` migration | Medium | No backfill; new `Save`/`Enroll` only; coexistence |

## Rollback Plan

`git revert <sha>` per PR (stack order). Journal tables additive `CREATE TABLE IF NOT EXISTS` — inert after revert. `.bigmem/` chunks untouched, `sync_chunks` still works. No DDL on `observations`.

## Dependencies

- `rescue-ownership` dd5f731 + Batch M/S + 5-case detection; `modernc.org/sqlite`; Engram `store.go.safe` ref.

## Success Criteria

- [ ] `sync_mutations` enqueues per project, `ListPending` ordered by `seq`, `Ack` advances `last_acked_seq`
- [ ] `deferred` 5→`dead` with hash; `ReplayDeferredForScope` replays after dependency
- [ ] `QuarantineIrreparable` deterministic, cursor advances, `degraded` + `reason_code`
- [ ] `Acquire/ReleaseSyncLease` TTL+owner works; `MarkSync*` sets `backoff_until`/`consecutive_failures`
- [ ] `go vet` + `go test ./internal/bigmem -count=1 -timeout 180s` green
- [ ] `sync --status` shows `lifecycle` and `last_enqueued/acked/pulled`
