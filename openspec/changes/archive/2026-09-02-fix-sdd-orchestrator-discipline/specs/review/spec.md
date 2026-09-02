# Delta for review

## Purpose

Gate SDD `verify` on review-driven evidence when RDD enabled. Review receipt (`biggz review` + `publishImmutable` binding) MUST precede verification; disabled RDD remains unmanaged without fabricated PASS.

## Requirements

### Requirement: REQ-REV-001 — Review Gate Before Verify

When `RDDStatus` is `enabled`, `sdd-verify` MUST require a valid review lineage and persisted receipt before any verification. Receipt binding (`domainHash` + `writeLengthPrefixed`, `PersistedReceipt` under `receipts/<sha256>.json`) MUST be verified via `biggz review --receipt` / `Receipt.Verify` / `PersistedReceipt.Validate`. Missing/invalid MUST block `verify` fail-closed.

#### Scenario: Enabled without receipt blocks

- GIVEN `RDD enabled`, no lineage for change
- WHEN verify preflight checks
- THEN it MUST block and hint `biggz review` flow

#### Scenario: Enabled with valid receipt allows

- GIVEN `RDD enabled`, `receiptValid==true`, chain `Valid==true`
- WHEN verify preflight runs
- THEN gate MUST pass

#### Scenario: Disabled allows without receipt

- GIVEN `RDD disabled`
- WHEN verify checks
- THEN it MUST allow and report `disabled/unmanaged`

### Requirement: REQ-REV-002 — No Fabricated PASS on Unmanaged

When `RDD enabled` and `allowed==false` (unmanaged) the gate MUST NOT fabricate `PASS`. Result MUST be `pass==false` with `allowed==false`. Pre-push unmanaged MUST report `unmanaged` not `PASS`. `verify` MUST surface `rdd_unmanaged` blocker.

#### Scenario: Unmanaged does not fabricate

- GIVEN `RDD enabled`, `allowed==false`
- WHEN `biggz review gate pre-pr` / verify gate evaluates
- THEN it MUST return non-zero/blocked and MUST NOT contain PASS

#### Scenario: Invalid binding hash blocks

- GIVEN receipt `BindingHash` mismatched after tamper
- WHEN gate validates
- THEN it MUST block with receipt validation failure

### Requirement: REQ-REV-003 — Receipt Binding Integrity

Receipt MUST bind `genesis/head/count/lineage` via `domainHash("biggz-ai.review-receipt/v1\x00" + writeLengthPrefixed(...))` and terminal receipt MUST bind `baseTree/initialReviewTree/finalCandidateTree/pathsDigest/fixDeltaHash/policyHash/evidenceHash` via `domainHash("biggz-ai.review-receipt-binding/v1\x00" + jsonPayload)` with `FixDeltaHashForSnapshot`. `verify` gate MUST validate self-hash and finding disjointness.

#### Scenario: Valid terminal receipt passes

- GIVEN `PersistedReceipt` built by `Finalize` with correct `FixDeltaHashForSnapshot`
- WHEN `Validate()` called in verify gate
- THEN it MUST succeed

#### Scenario: Zero cumulative returns empty hash

- GIVEN `cumulative==0`
- WHEN `FixDeltaHashForSnapshot` called
- THEN it MUST return `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`
