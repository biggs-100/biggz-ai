# Tasks: bigmem-branching (D2 — leafId + parentId)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 550-700 |
| 400-line budget risk | Medium |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → PR 2 |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Schema+migration+CRUD+SetLeaf | PR 1 | `go test ./internal/bigmem -run TestBranch -count=1` | `go test ./... -count=1 -timeout 180s` + `go vet ./internal/bigmem/...` | `bigmem.go`, `full.go`, `branch_test.go` — revert leaves cols unused |
| 2 | Traversal+context+MCP+compat | PR 2 | `go test ./internal/bigmem -run TestGetLeafPath -count=1` | `biggz bigmem doctor --fix` + `grep -r "/branch\|/rewind" tui/ cmd/` | `full.go` traversal, `mcp/context7.go`, `cli_bigmem.go` — revert restores linear |

## Phase 1: Foundation

- [x] 1.1 `bigmem.go` `migrateSchema` via `ensureColumns` ADD `parent_id FK`, `leaf_id`, `branch_summary`. PRAGMA table_info has 3 cols.
- [x] 1.2 `bigmem.go` indexes `idx_sessions_parent_id/_leaf_id` + update `Open()` DDL. sqlite_master shows FK/indexes.
- [x] 1.3 `full.go` extend `Session` with `ParentID *string`, `LeafID`, `BranchSummary`; roots `leaf_id=id`. Root ParentID nil && LeafID==ID.
- [x] 1.4 `full.go` `Doctor()` flags missing branching cols fixable. Legacy DB reports fixable.
- [x] 1.5 `full.go` `DoctorFix()` backfill `leaf_id=id` + checkpoint, idempotent. 2 rows backfilled, rerun unchanged.

## Phase 2: Core API

- [x] 2.1 RED `branch_test.go`: `CreateBranch("missing")` error 0 rows (missing parent). Fails before 2.2, passes after.
- [x] 2.2 `full.go` `CreateBranch(parentID,summary)` validate SELECT 1, mu.Lock INSERT. Child parent_id==A.id.
- [x] 2.3 `full.go` `GetBranch`/`ListBranches`. A->B->C list 3, GetBranch(B) correct.
- [x] 2.4 RED `branch_test.go`: `SetLeaf` race `-race` last-writer-wins (stale leaf_id). Fails before 2.5, passes after.
- [x] 2.5 `full.go` `SetLeaf(leafID)` atomic UPDATE under `Store.mu`. Parallel converges; vet clean.

## Phase 3: Integration

- [x] 3.1 RED `branch_test.go`: cycle A↔B len2, depth 110→100, SQLi `"' OR 1=1"` before GetLeafPath. 3 fail before guard.
- [x] 3.2 `full.go` `GetLeafPath(leafID)` iterative visited+depth100 param `?`. R->B->L=[L,B,R]; 3.1 passes.
- [x] 3.3 `full.go` `SessionContextBranched(leafID,limit)` fallback `""`→SessionContext. "" linear, leaf leaf→root.
- [x] 3.4 `full.go` `Save(obs, parentID ...string)` optional anchoring, omitted no-op. FTS/dedup preserved.
- [x] 3.5 `mcp/context7.go` add `bigmem_branch_create/list/get` → Store. Tools listed.
- [x] 3.6 `cli_bigmem.go` doctor --fix message; `grep -r "/branch\|/rewind" tui/ cmd/` empty. Grep empty, vet passes.

## Phase 4: Testing

- [x] 4.1 REQ-B1/B2: fresh schema + legacy backfill idempotence. `go test -run TestMigration` passes.
- [x] 4.2 REQ-B4/B5: chain/cycle/depth Threat Matrix. `go test -race ./internal/bigmem` passes.
- [x] 4.3 REQ-B6/B7/B8: legacy Get/Search, retention no GC, FTS. Linear suite passes.
- [x] 4.4 E2E: `go test ./... -count=1 -timeout 180s` + `go vet ./...` + `gofmt -l` clean. Zero failures.

## Phase 5: Cleanup

- [x] 5.1 Remove temp helpers, ensure no TUI/auto-branch. Diff 4 files + branch_test.go only.
- [x] 5.2 Collect logs (go test/vet, PRAGMA table_info) for verify. Logs attached.
