# Delta for sdd

## ADDED Requirements

### Requirement: ReviewOffer Post-Verify Wiring

System MUST emit `reviewOffer{Available:true, Invocation:"biggz review start --lineage <change>-<shortsha>"}` iff `applyState==all_done && verifyReport==done && passing && RDD enabled`; else MUST be `nil`. Passing=`pass`,0 blockers,8/8. `status.go:523`/`engram_status.go:246,342` MUST compute; `status_v2.go:48-53` MUST expose only `available,invocation`. Invocation MUST use `pathquote.Quote`, MUST NOT embed lineage/binding/receipt.

#### Scenario: Enabled PASS emits offer
- GIVEN `all_done`, `verify done PASS`, `RDD enabled`
- WHEN `biggz sdd-status --json` derives
- THEN `reviewOffer.available==true` and `invocation` quoted

#### Scenario: Disabled or verify failing emits nil
- GIVEN `RDD disabled` OR `verify missing/fail` OR `blockers>0`
- WHEN status derives
- THEN `reviewOffer==nil`

#### Scenario: Invocation quoting
- GIVEN change `my change` shortsha `a1b2c3d`
- WHEN invocation built
- THEN MUST contain `pathquote.Quote` and MUST NOT contain persisted lineage

### Requirement: Hook Lineage-Aware Selection

Hook `pre-push:8-28` MUST select via `ls -t` newest-first filtered by `git merge-base --is-ancestor <commit> HEAD`; MUST NOT use alphabetical first. Ghost `019fbb3a-*` only if ancestor. Fallback newest `ls -t`.

#### Scenario: Ghost ignored when not ancestor
- GIVEN ghost `019fbb3a-*` not ancestor and `my-change-abc123` ancestor of `HEAD`
- WHEN hook selects `lineage`
- THEN `lineage==my-change-abc123`

#### Scenario: Fallback newest
- GIVEN `merge-base` unavailable
- WHEN hook runs
- THEN MUST pick newest `ls -t` lineage

### Requirement: Hook Space-Tolerant Grep

Hook MUST grep with `[[:space:]]*` for `delivery` disabled and `allowed` false.

#### Scenario: JSON with spaces routed
- GIVEN JSON `{"delivery": "disabled"}` and `{"allowed": false}` with spaces
- WHEN hook greps `output`
- THEN delivery grep allows push, allowed-false blocks

### Requirement: Archive Never Auto-Disable

`archive.go:ArchiveChange` MUST NOT call `RDDDisable`/`SetCloneLocalRDDMode`/`RDDEnable` nor write `.git/biggz/rdd-mode`; only `os.Rename`.

#### Scenario: Archive preserves enabled and mtime
- GIVEN `RDD enabled` and mtime T0 before archive
- WHEN `ArchiveChange` moves PASS change
- THEN `rdd status` still `enabled`, mtime==T0, grep finds zero RDD calls

### Requirement: Orchestrator Auto-Run on Block Only

Orchestrator MUST auto-run `reviewOffer.invocation` only when `allowed==false && auto-chain && offer available`; else surface offer only.

#### Scenario: Auto-chain blocked auto-runs
- GIVEN `auto-chain`, `allowed:false`, offer available
- WHEN orchestrator handles
- THEN MUST execute invocation

#### Scenario: Ask-on-risk offers only
- GIVEN `ask-on-risk` interactive, same `allowed:false`
- WHEN orchestrator handles
- THEN MUST NOT auto-run, MUST print offer
