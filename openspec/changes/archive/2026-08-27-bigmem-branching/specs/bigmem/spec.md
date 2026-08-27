# Delta for bigmem — Branching (D2)

## ADDED Requirements

### Requirement: REQ-B1 — Branching Schema

The system MUST add to `sessions`: `parent_id TEXT` (self-FK, nullable), `leaf_id TEXT`, `branch_summary TEXT`. Roots MUST have `parent_id IS NULL`.

#### Scenario: Fresh DB schema

- GIVEN fresh DB
- WHEN `Open()` completes
- THEN `PRAGMA table_info(sessions)` MUST include `parent_id`, `leaf_id`, `branch_summary`

#### Scenario: Root creation

- GIVEN `CreateBranch(parentID="")`
- WHEN inserted
- THEN `parent_id` MUST be `NULL` and `leaf_id` MUST equal `id`

### Requirement: REQ-B2 — Legacy Migration (DoctorFix)

`migrateSchema()` MUST `ADD COLUMN` idempotently (O(1)). Legacy rows MUST become `parent_id=NULL, leaf_id=self`. `Doctor()` MUST flag missing columns fixable; `DoctorFix()` MUST be idempotent.

#### Scenario: Legacy migration

- GIVEN DB with 2 pre-D2 sessions
- WHEN `DoctorFix()` runs
- THEN rows MUST have `leaf_id=id` and `parent_id IS NULL`

#### Scenario: Idempotent rerun

- GIVEN migrated DB
- WHEN `DoctorFix()` runs twice
- THEN second run MUST succeed with unchanged row count

### Requirement: REQ-B3 — Branch CRUD

The system MUST provide `CreateBranch(parentID, summary)`, `GetBranch(id)`, `ListBranches()`. `CreateBranch` with non-empty `parentID` MUST validate parent exists. `branch_summary` is optional text.

#### Scenario: Create child

- GIVEN root `A`
- WHEN `CreateBranch(parentID=A.id, summary="fix")`
- THEN child MUST have `parent_id=A.id` and `branch_summary="fix"`

#### Scenario: List/Get

- GIVEN chain `A->B->C`
- WHEN `ListBranches()` / `GetBranch(B.id)` called
- THEN list has 3 and `Get` returns `B` with correct parent

#### Scenario: Missing parent error

- GIVEN parent `missing` absent
- WHEN `CreateBranch(parentID="missing")`
- THEN MUST return error, 0 rows inserted

### Requirement: REQ-B4 — Leaf→Root Resolution

The system MUST provide `GetLeafPath(leafID)` and `SessionContextBranched(leafID)` walking `parent_id` iteratively leaf→root, depth limit 100, cycle guard, ordered leaf→root. `""` leafID MUST fallback to linear `SessionContext`.

#### Scenario: Chain resolution

- GIVEN `R->B->L`
- WHEN `GetLeafPath(L.id)` called
- THEN result MUST be `[L, B, R]`

#### Scenario: Cycle and depth guard

- GIVEN `A.parent_id=B, B.parent_id=A`
- WHEN `GetLeafPath(A.id)` called
- THEN traversal MUST terminate without loop (depth/cycle guard)

### Requirement: REQ-B5 — Save Anchoring & SetLeaf

`Save()` MAY accept optional `parentId`; when omitted MUST be no-op. `SetLeaf(leafID)` MUST atomically UPDATE `leaf_id` under `Store.mu`.

#### Scenario: Save with anchoring

- GIVEN active leaf `L`
- WHEN `Save(obs, parentId=L.id)` called
- THEN association MUST persist without breaking legacy dedup

#### Scenario: Save without parent unchanged

- GIVEN `Save(obs)` with no parentId
- WHEN executed
- THEN FTS/dedup behavior MUST match pre-D2

#### Scenario: SetLeaf atomic

- GIVEN concurrent `SetLeaf` calls
- WHEN both complete
- THEN final leaf MUST be one of the values (single UPDATE)

### Requirement: REQ-B6 — Backward Compatibility

`Get`/`Search` MUST work for legacy and branched rows. Existing linear tests MUST pass unchanged.

#### Scenario: Legacy Get/Search

- GIVEN legacy row `parent_id=NULL`
- WHEN `Get(id)` or `Search(q)` called
- THEN results MUST return normally, independent of branching columns

### Requirement: REQ-B7 — No Automatic GC

The system MUST NOT auto-delete branches; retention is indefinite until explicit delete.

#### Scenario: Retention

- GIVEN `R->B->C`
- WHEN no explicit delete issued
- THEN all three MUST remain queryable

### Requirement: REQ-B8 — Minimal MCP & Scope Bound

MCP MUST expose only `bigmem_branch_create/list/get` (internal-only) delegating to Go API. TUI ` /branch`/`/rewind`, `sdd-apply` auto-branch, `SessionEntryIndex` mirror, D1 blob, graph/FTS re-rank, sync branch awareness MUST NOT be implemented.

#### Scenario: MCP minimal

- GIVEN MCP server running
- WHEN tools listed
- THEN `bigmem_branch_create/list/get` MUST exist and create MUST call `CreateBranch`

#### Scenario: No TUI branching

- GIVEN change applied
- WHEN `grep -r "/branch\|/rewind" tui/ cmd/` runs
- THEN output MUST be empty
