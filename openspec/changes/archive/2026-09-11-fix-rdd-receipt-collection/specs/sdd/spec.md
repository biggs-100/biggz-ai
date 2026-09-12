# Delta for sdd

## MODIFIED Requirements

### Requirement: ReviewOffer Post-Verify Wiring

System MUST emit `reviewOffer{Available:true, Invocation:"biggz review start --subject <file>"}` iff `applyState==all_done && verifyReport==done && passing && RDD enabled`; else MUST be `nil`. Passing=`pass`,0 blockers,8/8. `status.go:523`/`engram_status.go:246,342` MUST compute; `status_v2.go:48-53` MUST expose only `available,invocation`. Invocation MUST use `pathquote.Quote` and MUST NOT embed a lineage id, binding, or receipt.
(Previously: Invocation embedded `--lineage <change>-<shortsha>`, producing an offer lineage that diverged from the gate lookup.)

#### Scenario: Enabled PASS emits offer

- GIVEN `all_done`, `verify done PASS`, `RDD enabled`
- WHEN `biggz sdd-status --json` derives
- THEN `reviewOffer.available==true` with a quoted invocation carrying no lineage id

#### Scenario: Disabled or verify failing emits nil

- GIVEN `RDD disabled` OR `verify missing/fail` OR `blockers>0`
- WHEN status derives
- THEN `reviewOffer==nil`

#### Scenario: Invocation quoting

- GIVEN change `my change` shortsha `a1b2c3d`
- WHEN invocation built
- THEN MUST contain `pathquote.Quote` and MUST NOT contain a lineage id, binding or receipt

## ADDED Requirements

### Requirement: Pre-Publication Review Obligation in Phase Instructions

When RDD is enabled and the verified candidate has no valid receipt, `sdd-apply`/`sdd-verify` phase instructions MUST surface the outstanding review obligation, including the exact producer command that satisfies it, before publication. The obligation MUST NOT first appear only when archive is attempted.

#### Scenario: Verify completion surfaces obligation

- GIVEN `RDD enabled`, verify completed passing, and no receipt exists
- WHEN apply/verify phase instructions render
- THEN they MUST surface the review obligation with the exact producer command

#### Scenario: Valid receipt or disabled RDD surfaces nothing

- GIVEN a valid receipt exists OR `RDD disabled`
- WHEN phase instructions render
- THEN they MUST NOT surface a review obligation
