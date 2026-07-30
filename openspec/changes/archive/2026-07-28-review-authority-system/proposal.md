# Proposal: Review Authority System

## Intent

Replace the current 5-state in-memory FSM with a full content-addressed event store, 13-state FSM with role-based guards, chain validation, receipt binding to complete chain, publication gates, correction budget enforcement, and inventory/status commands. Current system has no audit trail, no role enforcement, and no gate preconditions — a review is not cryptographically binding.

## Scope

### In Scope
- 13-state FSM (unreviewed → approved/escalated/invalidated) with role-based transition guards
- Content-addressed event store (SHA-256 chain) under `.git/biggz/review-transactions/`
- Receipt binding to complete chain (genesis → head), not just current state
- Lineage concept for traceable review history
- Publication gates: pre-PR and pre-push with scope change detection
- Correction budget enforcement inside the FSM (counters, cycles)
- Inventory/status commands over review lineages
- Role/actor tracking on every transition
- Policy evaluation as formal guard criteria

### Out of Scope
- Historical v1/v2 compatibility hacks (greenfield)
- CompactState dual-machine complexity from gentle-ai
- Heavy Git boundary resolution (scope detection = light)
- VCS hooks installation (gates are CLI commands, not hooks)

## Capabilities

### New Capabilities
- `review-authority`: Content-addressed event store, chain validation, role-based guards, receipt<->chain binding, budget enforcement, lineage inventory/status commands
- `review-gates`: Pre-PR and pre-push publication gates with scope change detection, gate result reporting

### Modified Capabilities
- `core-review`: FSM expands from 5 to 13 states; transition validation adds role guards, preconditions, and correction budget counters

## Approach

Greenfield design referencing gentle-ai's `internal/reviewtransaction/` for state model patterns, but built for biggz-ai from scratch:

1. **FSM**: 13 states with guard table (role × state × action), budget counters (fix rounds, scoped validations)
2. **Store**: Content-addressed event log at `.git/biggz/review-transactions/<lineage>/`, each file = SHA-256 named event with predecessor hash
3. **Receipt**: Bind to genesis + head hashes + full event count — not just MerkleRoot at completion
4. **Gates**: CLI commands (`biggz review gate pre-pr`, `biggz review gate pre-push`) that evaluate state + receipt + scope delta against policy
5. **Inventory**: CLI `biggz review list` and `biggz review status <id>` querying the event store

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `model/fsm.go` | Modified | 5→13 states, guard table, budget counters |
| `model/review.go` | Modified | Add Role, LineageID, Counters fields |
| `internal/review/authority.go` | New | Event store, chain validation, inventory |
| `internal/review/gate.go` | New | Pre-PR/pre-push gate enforcement |
| `internal/review/receipt.go` | Modified | Bind to full chain, not single state |
| `internal/review/correction.go` | Modified | Budget enforcement via FSM guards |
| `internal/review/*` | Modified | Adapt ledger, snapshot, judgment, lock |
| `.git/biggz/review-transactions/` | New | Event store location (inside git dir) |
| `cmd/biggz/main.go` | Modified | Add review gate/inventory subcommands |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Over-engineering from gentle-ai reference | Med | Explicit non-goals list; skip CompactState, v1/v2 hacks |
| SHA-256 file store perf on large reviews | Low | Single lineage = bounded events; test with 500+ event workloads |
| Gate misconfiguration blocks dev workflow | Med | Gate dry-run mode, clear error messages, per-repo opt-out config |

## Rollback Plan

- Revert `model/fsm.go` + `model/review.go` to current 5-state version
- Remove `.git/biggz/review-transactions/` directory
- Revert receipt/gate/authority to current implementations
- Full rollback = git revert on all changed files

## Dependencies

- Go stdlib `crypto/sha256` for content addressing
- `git rev-parse --git-dir` for store location resolution
- No external dependencies

## Success Criteria

- [ ] 13 states with guard table reject invalid role-based transitions
- [ ] Event store persists every transition with SHA-256 chain integrity
- [ ] Receipt verification fails if any event in chain is tampered
- [ ] Pre-PR gate blocks when findings exist or receipt invalid
- [ ] Pre-push gate blocks when scope change unacknowledged
- [ ] `biggz review list` enumerates all lineages from event store
- [ ] Correction budget enforcement prevents infinite fix cycles
