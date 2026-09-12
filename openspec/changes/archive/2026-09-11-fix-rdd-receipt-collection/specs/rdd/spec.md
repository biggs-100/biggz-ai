# Delta for rdd

## MODIFIED Requirements

### Requirement: Gate Blocking Semantics

`gate.go`/hook MUST block when `RDD enabled` unmanaged (`allowed:false`), allow when `RDD disabled`. `allowed:false` MUST NOT fabricate PASS. The refusal MUST be typed and actionable: it MUST name the exact producer command(s) that satisfy the gate and MUST NOT be a dead end.
(Previously: the refusal blocked correctly but named no producer command, leaving the kill switch as the only escape.)

#### Scenario: Enabled unmanaged blocks

- GIVEN `RDD enabled` and no lineage or `allowed:false`
- WHEN gate/hook runs
- THEN `allowed==false`, hook exit 1, and the refusal MUST name the exact producer command that satisfies the gate

#### Scenario: Disabled allows

- GIVEN `RDD disabled`
- WHEN gate/hook runs
- THEN `delivery==disabled/unmanaged` and MUST NOT block

### Requirement: REQ-RDD-002 — Verify Blocked Without Valid Receipt When Enabled

When RDD is `enabled`, the system MUST block `verify` if receipt is missing, invalid, tampered, or `allowed==false` (unmanaged). The gate MUST NOT fabricate `PASS` when `allowed==false`. Blocked verify MUST set `blockedReasons` containing `rdd_receipt_missing` or `rdd_unmanaged` and `NextRecommended==resolve-blockers` or `verify` blocked. Pre-push that reports `unmanaged` MUST NOT synthesize a passing receipt. The refusal MUST be typed and actionable, naming the exact producer command that satisfies it. A degraded or unproducible receipt MUST be reported honestly — no fabricated PASS and no silent unmanaged delivery.
(Previously: blocked correctly but without an actionable producer command or an explicit rule for degraded/unproducible receipts.)

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

#### Scenario: Missing-receipt refusal names the producer

- GIVEN `RDD enabled`, no receipt, and a producer available for the host
- WHEN the refusal is rendered
- THEN `blockedReasons` MUST contain `rdd_receipt_missing` and the refusal MUST name the exact producer command(s) satisfying it

#### Scenario: Unproducible receipt reported honestly

- GIVEN `RDD enabled` and no producer can satisfy the receipt (degraded surface)
- WHEN the gate evaluates
- THEN it MUST report degraded/unproducible explicitly and MUST NOT fabricate PASS or deliver as unmanaged
