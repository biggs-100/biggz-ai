# Delta for rdd

## Purpose

Require RDD review gate (`biggz review` + receipt) before `verify` when RDD is enabled. `verify` MUST be blocked without a valid review receipt; disabled RDD reports unmanaged without fabricated PASS. Ensures review-driven evidence precedes verification.

## Requirements

### Requirement: REQ-RDD-001 — RDD Review Gate Before Verify

The system MUST enforce an RDD gate in verify preflight: when `RDDStatus` is `enabled`, `sdd-verify` and the native verify dispatcher MUST require a valid review lineage and persisted receipt (`biggz review --receipt` valid, binding hash matches, `receiptValid==true`, chain valid). The gate MUST run before any verify remediation and MUST use `biggz review` as source of truth, not a fabricated status.

#### Scenario: Enabled with valid receipt allows verify

- GIVEN `RDD effective==enabled`, lineage `fix-sdd-orchestrator-discipline` has `receiptValid==true` and chain valid
- WHEN `biggz sdd-verify` preflight checks RDD gate
- THEN gate MUST pass and verify MAY proceed

#### Scenario: Enabled without lineage blocks verify

- GIVEN `RDD enabled` and no review lineage exists for the change
- WHEN verify preflight runs
- THEN gate MUST block with hint to run `biggz review` and receipt flow

#### Scenario: Disabled RDD bypasses receipt check

- GIVEN `RDD effective==disabled`
- WHEN verify preflight runs
- THEN gate MUST pass regardless of receipt and MUST report `delivery==disabled/unmanaged`

### Requirement: REQ-RDD-002 — Verify Blocked Without Valid Receipt When Enabled

When RDD is `enabled`, the system MUST block `verify` if receipt is missing, invalid, tampered, or `allowed==false` (unmanaged). The gate MUST NOT fabricate `PASS` when `allowed==false`. Blocked verify MUST set `blockedReasons` containing `rdd_receipt_missing` or `rdd_unmanaged` and `NextRecommended==resolve-blockers` or `verify` blocked. Pre-push that reports `unmanaged` MUST NOT synthesize a passing receipt.

#### Scenario: Invalid receipt blocks verify

- GIVEN `RDD enabled`, receipt exists but `BindingHash` mismatch after chain tamper
- WHEN `biggz review gate` and verify preflight evaluate
- THEN verify MUST be blocked, `allowed==false`, and MUST NOT report `PASS`

#### Scenario: Unmanaged does not fabricate PASS

- GIVEN `RDD enabled`, `allowed==false`, no valid receipt
- WHEN gate evaluates
- THEN result MUST be `pass==false` and MUST NOT contain fabricated approval

#### Scenario: Valid receipt with all deterministic findings resolved allows verify

- GIVEN `RDD enabled`, receipt valid, zero unresolved deterministic findings
- WHEN verify runs
- THEN it MUST proceed and `VerifyReport` MAY be `PASS`

### Requirement: REQ-RDD-003 — RDD Status Source of Truth and Disabled Reporting

The system MUST derive `RDDStatus` via `biggz rdd status` (or `internal/rdd` `RDDStatus` reading `gen-%010d.json` with `LOCK` and CAS `Revision`). When `RDD disabled`, verify and gates MUST report `delivery` as `disabled` or `unmanaged` and MUST NOT block on missing receipt. Re-enabling applies only to future candidates; `archive` MUST NOT auto-disable.

#### Scenario: Fresh repo defaults to enabled and requires receipt

- GIVEN no `rdd-mode.json`/`gen-*.json` (fresh repo default `enabled`)
- WHEN `RDDStatus` called then verify attempted without receipt
- THEN `effective==enabled` and verify MUST be blocked until receipt exists

#### Scenario: Explicit global disable allows verify without receipt

- GIVEN global `mode==disabled` via `biggz rdd disable --scope=global`
- WHEN verify runs without receipt
- THEN `effective==disabled`, gate MUST allow, and report MUST contain `disabled`

#### Scenario: Archive preserves enabled mode

- GIVEN `RDD enabled` before `sdd-archive`
- WHEN `ArchiveChange` completes
- THEN `rdd status` MUST still be `enabled` and MUST NOT have written `.git/biggz/rdd-mode` disable
