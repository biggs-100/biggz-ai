# Database Migration Spec Template

## REQ-001: Schema Change

**Title**: [Migration name]
**Migration ID**: `[YYYYMMDD]_[description]`

### Changes

| Operation | Table | Column | Type | Default |
|-----------|-------|--------|------|---------|
| CREATE/ALTER/DROP | [table] | [column] | [type] | [default] |

### SQL

```sql
-- Up migration
ALTER TABLE [table] ADD COLUMN [column] [type] DEFAULT [default];

-- Down migration
ALTER TABLE [table] DROP COLUMN [column];
```

### Scenario: Migration applies cleanly

GIVEN the database is at version [N]
WHEN the migration runs
THEN the schema is updated to version [N+1]
AND the new column/table exists

### Scenario: Rollback

GIVEN the database is at version [N+1]
WHEN the down migration runs
THEN the schema reverts to version [N]

## REQ-002: Data Migration

**Title**: Backfill existing rows
**Description**: [How existing data is migrated]

GIVEN [N] existing rows without the new column populated
WHEN the backfill runs
THEN all rows have the column populated with [expected value]

## REQ-003: Performance

**Title**: Migration performance

GIVEN the [table] has [N] rows
WHEN the migration runs
THEN it completes within [time] seconds without locking the table for more than [time]

## REQ-004: Rollback Plan

**Title**: How to revert if the migration fails

1. Run down migration
2. Restore from backup
3. [Other steps]
