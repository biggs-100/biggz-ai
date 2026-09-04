# Proposal: Fix BigMem Status Bypass

## Intent

`sdd-status` hot path bypasses Store API with raw `sql.Open` + full-content `SELECT`, filtering in Go, no ctx, silent `(nil,nil,nil)` failures — slow, uncancelable, hides DB errors.

## Scope

### In Scope
- Replace `collectBigMemChangesWithArchive` raw SQL with Store `*Ctx` API (`SearchCtx`/equivalent)
- Filter project/scope in SQL; fetch `content` only for visible changes
- Thread caller ctx + timeout through `openBigMemDB`/`queryBigMemRows`/`scanBigMemTopics`; fix `status.go:~453/~723` `context.Background`
- Visible failures: log + wrapped error; degraded path only with explicit logged warning

### Out of Scope
- Doctor `RootDir()+"/bigmem.db"` concat (SDD4-docs or later)
- Blob/DOCS (SDD4), MCP N+1 (SDD3)
- New Store query methods unless minimal filter suffices

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `sdd-status`: BigMem derivation MUST use Store `*Ctx` API with SQL-side filtering, minimal hydration, ctx timeout, visible errors (not raw `db.Query` + Go filter + silent nil)

## Approach

Open Store via existing constructor (no `sql.Open` in `sdd`), query with project/scope predicates + `deleted_at IS NULL` + `topic_key LIKE 'sdd/%'` in SQL; hydrate full `content` only for surviving rows; propagate caller ctx with timeout; return/log wrapped errors; keep filesystem-wins merge unchanged. Remove `modernc.org/sqlite` import from `engram_status.go` if unused.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/sdd/engram_status.go` | Modified | Remove `openBigMemDB`/`queryBigMemRows`/`scanBigMemTopics` raw SQL; Store `*Ctx` path |
| `internal/sdd/status.go` | Modified | Replace `context.Background` at `IsSessionSummaryBlocked` ~453/~723 with caller ctx |
| `internal/bigmem/` | None | Consume existing `SearchCtx`/`SessionContextCtx`/`TimelineCtx`; no change expected |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Store API can't express filter efficiently | Low | Add minimal predicate option; keep SQL-side filter |
| Behavior drift (project/scope visibility) | Med | Parity tests: personal excluded, inferred-project match, override disables filter |
| Perf regression on large stores | Low | Select keys first, hydrate visible only; benchmark |

## Rollback Plan

Revert commit restoring raw-SQL collector; `sdd-status` falls back to filesystem-only on DB absence as before. No migration — read-only change.

## Dependencies

- SDD1 archived Store `*Ctx` API (`SearchCtx`, `SessionContextCtx`, `TimelineCtx`)

## Success Criteria

- [ ] No `sql.Open`/`db.Query` (non-Ctx) in `internal/sdd/engram_status.go`
- [ ] `sdd-status` filters project/scope in SQL, hydrates content only for visible changes
- [ ] Hot path carries caller ctx with timeout; no `Background` at ~453/~723
- [ ] DB errors logged + wrapped, never silent `(nil,nil,nil)`; `go test ./internal/sdd/... ./internal/bigmem/...` pass
