# Design: bigmem-branching (D2 — leafId + parentId)

## Technical Approach

Additive branching on `sessions` via `migrateSchema`/`DoctorFix` `ADD COLUMN` O(1): `parent_id/leaf_id/branch_summary`, iterative leaf→root (depth100, cycle guard), minimal CRUD. Mirrors oh-my-pi; no TUI. Empty IDs = linear compat. REQ-B1–B8.

## Architecture Decisions

### Decision: Schema — nullable parent_id

| Option | Tradeoff | Decision |
|--------|----------|----------|
| `parent_id TEXT` nullable, `NULL`=root | Simple self-FK, `IS NULL` query, matches oh-my-pi | **Choose** |
| `NOT NULL DEFAULT ''` | Sentinel breaks FK | Reject |
| Separate `branches` table | Extra JOIN | Reject |

Rationale: Self-FK, O(1) `ADD COLUMN`, `FOREIGN KEY(parent_id) REFERENCES sessions(id) ON DELETE SET NULL`.

### Decision: Leaf tracking — column vs table

| Option | Tradeoff | Decision |
|--------|----------|----------|
| `leaf_id TEXT` on `sessions` (self for roots) | One `UPDATE` under `Store.mu`, no JOIN | **Choose** |
| `branch_heads` table | Extra table/JOIN | Reject |
| In-memory only | Lost on restart | Reject |

Rationale: `SetLeaf` = atomic `UPDATE` under `Store.mu`.

### Decision: branch_summary — TEXT vs JSON

| Option | Tradeoff | Decision |
|--------|----------|----------|
| `TEXT` optional | Sufficient, no parsing | **Choose** |
| `JSON` | Index/parse overhead | Reject |

Rationale: REQ-B3 text only; JSON YAGNI.

### Decision: Migration — DoctorFix vs CREATE TABLE

| Option | Tradeoff | Decision |
|--------|----------|----------|
| `migrateSchema`+`DoctorFix` `ADD COLUMN` + backfill `leaf_id=id` | O(1), no lock, matches directory/start_time migration | **Choose** |
| `CREATE TABLE new_sessions` copy | O(N) lock | Reject |
| New `schema.go` | Duplicates `Open()` DDL | Reject |

Rationale: Idempotent, low-risk, follows `ensureColumns` precedent.

## Data Flow

```
CreateBranch(parentID) → Store.mu.Lock → INSERT sessions (FK check) → Session
Save(obs, parentId?) ─→ dedup ─┘
SetLeaf(leafID) ──UPDATE leaf_id──┘
GetLeafPath(leafID) ──iterative SELECT parent_id (visited+depth100)──→ []Session leaf→root
SessionContextBranched(leafID) ── GetLeafPath or fallback SessionContext("") ──→ MCP
DoctorFix ── ensureColumns ── backfill leaf_id=id ── VACUUM/checkpoint
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/bigmem/bigmem.go` | Modify | `migrateSchema` for 3 cols; `Open()` FK + indexes |
| `internal/bigmem/full.go` | Modify | Extend `Session`; `SessionStart` DDL; CRUD + `GetLeafPath`/`SessionContextBranched`; `Doctor`/`DoctorFix` backfill |
| `internal/mcp/tools.go` | Modify | 3 tools `bigmem_branch_create/list/get` → `Store` |
| `cmd/biggz/cli_bigmem.go` | Modify | `doctor --fix` message only |
| `internal/bigmem/branch_test.go` | Create | Chain/cycle/depth/idempotence tests |

No `tui/`; `grep -r "/branch\|/rewind" tui/ cmd/` empty.

## Interfaces / Contracts

```go
type Session struct {
    ID string; StartTime, EndTime time.Time; Summary, Project, Directory string
    ParentID *string `json:"parent_id,omitempty"` // nil=root
    LeafID string `json:"leaf_id"`
    BranchSummary string `json:"branch_summary,omitempty"`
}
func (s *Store) CreateBranch(parentID, summary string) (*Session, error) // validates parent exists
func (s *Store) GetBranch(id string) (*Session, error)
func (s *Store) ListBranches() ([]Session, error)
func (s *Store) SetLeaf(leafID string) error
func (s *Store) GetLeafPath(leafID string) ([]Session, error) // leaf→root, depth100, cycle guard
func (s *Store) SessionContextBranched(leafID string, limit int) ([]Session, error) // ""→fallback
func (s *Store) Save(obs *Observation, parentID ...string) error
```

DDL: `parent_id REFERENCES sessions(id) ON DELETE SET NULL, leaf_id TEXT, branch_summary TEXT` +2 indexes. Traversal param `?` only.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Fresh 3 cols; root `leaf_id==id` | `PRAGMA table_info` |
| Unit | Legacy backfill | No cols → `DoctorFix` → `leaf_id=id` |
| Unit | Chain/cycle/depth100; missing parent | Injected cycle, 110-chain |
| Unit | `SetLeaf` atomic | `-race` writers |
| Integration | `Save` ±parent preserves FTS/dedup; `""` fallback | Save→Search compare |
| E2E | Linear suite still passes | `go test ./...` |

## Threat Matrix

Canonical boundaries (no routing/shell/VCS in this change):

| Boundary | Applicability | Reason |
|----------|---------------|--------|
| Documentation-like paths | N/A | No exec classification |
| Git repository selection | N/A | No `git -C` |
| Commit state | N/A | No commit |
| Push state | N/A | No push |
| PR commands | N/A | No `gh pr` |

Branch-traversal (applicable):

| Threat | Safe | Failure | RED test |
|--------|------|---------|----------|
| Cycle A↔B | visited+depth100 → partial path | No hang (iterative) | Inject cycle, len==2 |
| Depth 200 | Truncate 100 | No OOM | 110 chain → len 100 |
| Missing parent | Validate `SELECT 1` before INSERT, FK | Error, 0 rows | `CreateBranch("missing")` error |
| Stale leaf_id | `SetLeaf` single `UPDATE` under `mu` | Last-writer-wins | Parallel `-race` |
| SQL injection leafID | Param `?` only | No concat | `"' OR 1=1"` not found |

## Migration / Rollout

`migrateSchema` on `Open()` + `DoctorFix` backfill `leaf_id=id`. Additive nullable → `git revert` safe. No flag. Verify `PRAGMA table_info`.

## Open Questions

- [ ] `SetLeaf` global vs per-project leaf (D2 global per spec)
- [ ] `branch_summary` index deferred
- [ ] MCP `internal-only` gate (`tools.go` vs `context7.go`)
