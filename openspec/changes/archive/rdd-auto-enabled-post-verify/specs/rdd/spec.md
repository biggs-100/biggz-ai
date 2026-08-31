# Delta for rdd

## ADDED Requirements

### Requirement: Default ON Invariance

`rdd.go:280-322` default with no scope files MUST be `enabled` (`Source default`). No `rdd-mode.json`/`gen-*.json` MUST yield `effective enabled`.

#### Scenario: Fresh repo returns enabled
- GIVEN no global nor `gen-*` files
- WHEN `RDDStatus` called
- THEN `effective==enabled`, `source==default`

#### Scenario: Explicit disable yields disabled
- GIVEN global `mode disabled`
- WHEN `RDDStatus` called
- THEN `effective==disabled`, `source==global`

### Requirement: Gate Blocking Semantics

`gate.go`/hook MUST block when `RDD enabled` unmanaged (`allowed:false`), allow when `RDD disabled`. `allowed:false` MUST NOT fabricate PASS.

#### Scenario: Enabled unmanaged blocks
- GIVEN `RDD enabled` and no lineage or `allowed:false`
- WHEN gate/hook runs
- THEN `allowed==false`, hook exit 1 with hint `rdd disable`

#### Scenario: Disabled allows
- GIVEN `RDD disabled`
- WHEN gate/hook runs
- THEN `delivery==disabled/unmanaged` and MUST NOT block

### Requirement: Ghost Cleanup Documentation

Proposal MUST document manual `rm -rf .../019fbb3a-*` after payload `Temp/biggz-smoke`; code MUST NOT auto-delete ghosts.

#### Scenario: Manual rm after Temp check
- GIVEN ghost payload contains `Temp/biggz-smoke`
- WHEN user runs `rm -rf 019fbb3a-*`
- THEN ghost removed, `review list` hides it

#### Scenario: No auto-delete
- GIVEN ghosts exist
- WHEN code runs
- THEN grep for `019fbb3a` rm MUST find zero deletions

### Requirement: Install Defense-in-Depth

`install.go:410-560` `ensureRDDEnabled` MUST be idempotent, clear stale `biggz/rdd-mode` gens, ensure global `enabled`, SHOULD warn when overriding explicit `disabled`.

#### Scenario: Stale clone cleared
- GIVEN stale `gen-0000000000.json disabled` and global `enabled`
- WHEN `ensureRDDEnabled` runs twice
- THEN first removes stale, second no-op, global stays `enabled`

#### Scenario: Explicit disable warns
- GIVEN global `disabled` by user
- WHEN install runs
- THEN SHOULD warn before re-enabling
