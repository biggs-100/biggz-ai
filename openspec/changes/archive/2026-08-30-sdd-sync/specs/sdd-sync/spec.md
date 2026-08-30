# sdd-sync Specification

## Purpose

File-backed sync of `openspec/changes/{change}/specs/` deltas to `openspec/specs/` without archiving. Keeps main specs current for stacked PRs.

## Requirements

### Requirement: Store Gate — File-Backed Only

The system MUST allow sync only when declared store is `openspec` or `hybrid`. When store is `engram`, `bigmem`, or `none`, sync MUST return `not-applicable` with zero filesystem writes.

#### Scenario: File-backed store executes sync

- GIVEN declared store is `openspec` or `hybrid` and delta specs exist
- WHEN `sdd-sync` runs
- THEN it MUST apply ADDED/MODIFIED/REMOVED to `openspec/specs/{domain}/spec.md`

#### Scenario: Engram/none returns not-applicable

- GIVEN declared store is `engram` or `none`
- WHEN `sdd-sync` runs
- THEN it MUST return `not-applicable` and `openspec/specs/` MUST be unchanged

### Requirement: Delta Semantics — ADDED/MODIFIED/REMOVED

The system MUST implement: ADDED appends blocks; MODIFIED fully replaces matching requirement; REMOVED deletes it. If main spec is legacy flat (no `### Requirement:`), sync MUST return `blocked` with hint.

#### Scenario: ADDED appends, MODIFIED replaces, REMOVED deletes

- GIVEN main spec has requirement `Foo` and delta has `ADDED Foo2`, `MODIFIED Foo`, `REMOVED Foo`
- WHEN sync applies each delta type
- THEN ADDED MUST create `Foo2`, MODIFIED MUST replace entire `Foo` block, REMOVED MUST delete `Foo`

#### Scenario: Legacy flat spec blocks

- GIVEN `openspec/specs/{domain}/spec.md` exists without `### Requirement:` headings
- WHEN sync detects legacy flat
- THEN it MUST return `blocked` and describe conversion hint

### Requirement: Destructive Guard — Explicit Approval Required

The system MUST block destructive sync when REMOVED exists or MODIFIED exceeds large-mutation threshold unless prompt contains explicit approval. With approval, sync MUST proceed.

#### Scenario: Destructive without approval blocked

- GIVEN delta contains `REMOVED` or large MODIFIED and prompt lacks explicit approval
- WHEN status derives or sync validates
- THEN it MUST set `nextRecommended: sync` with `blockedReasons` mentioning destructive/approval

#### Scenario: Destructive with approval allowed

- GIVEN same destructive delta and prompt explicitly approves destructive sync
- WHEN sync runs
- THEN it MUST apply the deletion/replace and clear `blockedReasons` for that guard

### Requirement: Collision Guard — Same-Domain Active Change

The system MUST block sync when another active change touches same `openspec/specs/{domain}/spec.md` without ordering decision; it MUST surface colliding domains in `blockedReasons`.

#### Scenario: Collision without order blocks

- GIVEN change A and active change B both delta the same domain `sdd`
- WHEN status derives without order decision
- THEN `nextRecommended` MUST be `sync` with `blockedReasons` listing colliding domain `sdd` and change B

#### Scenario: Collision with order proceeds

- GIVEN same collision but order decided (e.g., rebase/sequence acknowledged)
- WHEN sync runs after ordered resolution
- THEN it MUST apply deltas

### Requirement: RENAMED Rejection — ADDED+REMOVED Only

The system MUST reject delta containing `## RENAMED` by returning `blocked` with hint to rewrite as `ADDED`+`REMOVED`; it MUST NOT provide rename helper.

#### Scenario: RENAMED triggers blocked

- GIVEN delta spec contains `## RENAMED Requirements` section
- WHEN sync validates
- THEN it MUST return `blocked` and message MUST contain `ADDED+REMOVED` hint

#### Scenario: Rewrite as ADDED+REMOVED succeeds

- GIVEN delta replaces RENAMED with equivalent `REMOVED OldName` + `ADDED NewName`
- WHEN sync runs with proper approval if destructive
- THEN it MUST apply both operations

### Requirement: Carve-outs and Execution Invariants

The system MUST skip strict guards when `resolve-via-engram` is set; `verify` MUST be `PASS` before sync is `ready`; sync MUST respect `actionContext.mode`/`allowedEditRoots` and MUST NOT create commits or archive.

#### Scenario: Carve-out skips strict guard

- GIVEN change is marked `resolve-via-engram`
- WHEN status derives destructive/collision
- THEN those strict blocks MUST be skipped

#### Scenario: Verify must pass and no commit/archive

- GIVEN `verifyReport` is not `PASS`
- WHEN status derives sync readiness
- THEN `nextRecommended` MUST NOT be `sync:ready`
- AND WHEN sync completes successfully, `openspec/changes/{change}/` MUST remain active and git log MUST have no auto-commit from sync
