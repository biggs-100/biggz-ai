# Proposal: fix-rdd-receipt-collection

## Intent

RDD is default-ON and archive requires a receipt, but Pi has no producer: collection (materialize → lens → capture) is OpenCode-only, and CLI hunk derivation is a placeholder (`cmd/biggz/cli_review.go:45-56`). Proof: merged `fix-bigmem-recall-friction` (PRs #54–#59) blocks with `rdd_receipt_missing`; offer lineage `…-c71dd60c` ≠ gate lookup (bare change); abbreviated subject SHAs never capture (`internal/review/capture.go:205`); invisible until archive; kill switch the only escape. Issue #60.

## Scope

### In Scope
- **Go materialization**: real hunk derivation (frozen trees, isolated GIT_DIR, 4 MiB refusal caps, binary-safe) + `--materialize`/`lens-context` printing the full reviewer task; non-vacuity tests.
- **Identity**: one lineage derivation for offer and gate (no embedded id); full-SHA subject canonicalization at `review start`.
- **Surfacing + parity guard**: obligation visible pre-publication; guard fails when a blocking surface lacks a producer.
- **PR 2**: OpenCode plugin aligned to the same contract.

### Out of Scope
- Pi collector extension (deferred); gate weakening (fail-closed stays); reviewer pinning; lens heuristics.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `review`: materialization/collection contract — Go-owned reviewer bytes, verbatim host transport, non-vacuity, parity guard.
- `review-authority`: single lineage-identity derivation; full-SHA subject canonicalization at start.
- `rdd`: typed, actionable `rdd_receipt_missing` refusal naming the producer; fail-closed unchanged.
- `sdd`: ReviewOffer drops the embedded lineage id; apply/verify instructions surface the obligation.
- `sdd-status`: `nextRecommended`/`blockedReasons` surface the obligation pre-publication.

## Approach

Copy the verified gentle-ai shape (@acc62463): Go prints the reviewer task (binding, context, name-status, numstat, per-path patch); hosts transport bytes verbatim; one identity derivation for offer and gate. Dogfoods its own receipt.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `cmd/biggz/cli_review.go` | Modified | Hunk derivation; `--materialize` |
| `internal/review/` | Modified | Materialization contract; non-vacuity |
| `internal/sdd/status.go`, `verify.go` | Modified | Single identity derivation |
| `internal/assets/opencode/plugins/review-result-artifacts.ts` | Modified (PR 2) | Transport-only reviewer |
| Guard/CI tests | New | Producer-parity guard |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Vacuous empty-hunk receipts | Med | Non-vacuity tests |
| Identity breaks existing lineages | Low | Read-path canonicalization; never rewrite events |
| Guard brittle across hosts | Low | Assert producer registration |

## Rollback Plan

Revert the diff; no migration. Canonicalization is read-path only, so existing events stay valid; kill switch stays.

## Dependencies

- Issue #60; shape from gentle-ai @acc62463.

## Success Criteria

- [ ] `--materialize` prints the full reviewer task with real findings on a known-bad candidate; offer and gate resolve one lineage (abbreviated SHAs rejected).
- [ ] Obligation visible pre-publication; guard fails without producer.
- [ ] Own receipt from the new surface; retro-collect `fix-bigmem-recall-friction`; archive without kill switch; comment #60 with reference mapping, close when verified.
