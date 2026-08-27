# Proposal: bigmem-branching (D2 — leafId + parentId)

## Intent

BigMem is linear; oh-my-pi `SessionManager` is a tree (`parentId` + active `leafId`, `SessionEntryIndex` O(1), `/branch`/`/rewind`). Without branching, BigMem cannot fork context for failed `sdd-apply` attempts. Add minimal schema/API foundation (leaf→root resolution) — defer all TUI/UX.

## Scope

### In Scope
- Schema: `sessions.parent_id TEXT`, `leaf_id TEXT`, `branch_summary TEXT` (FK self, nullable root)
- Migration via `DoctorFix`/`migrateSchema` ADD COLUMN — idempotent, `parent_id=NULL, leaf_id=self` for legacy rows
- Branch CRUD Go API: `CreateBranch`, `GetBranch`, `ListBranches`, `GetLeafPath(leafID)`, `SetLeaf`
- `Save()` optional `parentId` anchoring (nullable, no-op when omitted)
- Context: `SessionContextBranched(leafID)` iterative leaf→root (depth 100, cycle guard)
- MCP minimal: `bigmem_branch_create/list/get` (internal-only)

### Out of Scope
- `/branch`/`/rewind` TUI, `sdd-apply` auto-branch, `SessionEntryIndex` mirror (SQLite indexes suffice)
- D1 `blob:sha256` (archived), graph/FTS re-ranking, sync branch awareness

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `bigmem`: branching schema + CRUD + leaf→root resolution (delta to `openspec/specs/bigmem/spec.md`)

## Approach

1. **Migration** — `migrateSchema()` adds columns; `DoctorFix` reruns idempotently; `Doctor()` flags missing columns fixable.
2. **API** — `parentId` optional, `leafId` pointer via `SetLeaf` atomic UPDATE; `""` = legacy linear path.
3. **Context** — `SessionContext` unchanged; new `SessionContextBranched` walks `parent_id` with cycle/depth guard.
4. **Testing** — `go test ./... -count=1 -timeout 180s` (strict_tdd off); chain, cycle, self-leaf, idempotence.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/bigmem/bigmem.go` | Modified | `migrateSchema`, `Open` schema |
| `internal/bigmem/full.go` | Modified | `Session` + branch CRUD + `SessionContextBranched` |
| `internal/mcp/tools.go` | Modified | 3 minimal `bigmem_branch_*` tools |
| `cmd/biggz/cli_bigmem.go` | Modified | `doctor --fix` message only |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Large DB migration lock | Low | `ADD COLUMN` O(1) in SQLite, no rewrite |
| Cycle via parent_id | Low | Cycle guard + depth 100 + app check |
| Stale leaf_id | Med | `SetLeaf` single UPDATE under `Store.mu` |
| Scope creep to TUI | Med | Reject `/branch` code in review |

## Rollback Plan

Additive nullable columns — `git revert <sha>` leaves columns unused, no data loss. Callers omitting `parentId` see no change. Full removal via `ALTER TABLE DROP COLUMN` follow-up if needed (SQLite 3.35+).

## Dependencies

- D1 archived; `modernc.org/sqlite` FK already `ON`; no new deps.

## Success Criteria

- [ ] `DoctorFix` on legacy DB adds `parent_id`/`leaf_id`/`branch_summary`
- [ ] `GetLeafPath` returns correct leaf→root; cycle/depth guarded
- [ ] `go test ./...` passes unmodified linear tests + new branch tests
- [ ] No `/branch`/`/rewind` in TUI/CLI (grep verified)
