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
