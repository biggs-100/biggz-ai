# Native Bounded Review Orchestration

Parent orchestrator and native CLI only. Never pass this contract to a reviewer, refuter, judge, correction actor, or validator. Those roles receive only scope, candidate-causal admission, severity, evidence requirements, and output shape.

## Route

Begin a review with `biggz review start --subject <file>` (optionally `--lineage <id>` for an explicit UUIDv7 lineage). The native facade discovers the repository scope, derives the immutable subject target, selects the lens plan, and freezes the correction budget; correction and compatible base advance never recalculate risk or reopen review. Check `biggz review list [--json]` before starting to detect an in-flight lineage for the same subject.

A canonical four-lens selection is long work: before the first lens runs, give the one cost/side-effect forecast — four reviewer model runs over the frozen candidate, the frozen correction budget, and the at-most-one bounded correction it implies — once per candidate, never per lens.

## State

The provider-owned lineage is the single source of truth. Query `biggz review status <lineage> --json`; it returns `lineage_id`, `head_hash`, `event_count`, `chain_valid`, `receipt`, `integrity_verdict`, and `budget_counters`. Route only from the returned state: an in-review lineage resumes with `biggz review resume <lineage>` (Blocked/NeedsChanges -> InReview); `biggz review validate <lineage>` verifies chain integrity; a `receipt` present means approved. Never substitute direct START, and the apply executor never launches review.

Reviewer findings enter the lineage through the native chain; the orchestrator never constructs canonical bytes or hashes. Only candidate-caused severe findings block: `pre-existing` and `base-only` become follow-ups, `unknown` escalates, WARNING/SUGGESTION remain info. Deterministic blockers need no refuter; inferential blockers share one read-only refuter batch. Judgment Day uses two independent judges.

## Correction

Ordinary review permits one correction transaction, tracked in `budget_counters`. After the bounded edit, run one read-only scoped fix validator; the facade maps correction only to corroborated frozen IDs and genesis paths and rejects over-budget repository evidence. Later observations are follow-ups, not another correction. Judgment Day alone keeps its existing two-round rule. SDD then runs one independent requirements/runtime verification. Failure escalates and never starts another reviewer, refuter, correction, or validator.

<!-- authority-first-terminal-procedure:start -->
### Authority-First Terminal Procedure

Use only the native facade; it appends and reads back native authority (`.git/biggz/review-transactions/`) before materializing existing compatibility artifacts.

| Order | Operation | Required result | Terminal mirrors |
|---|---|---|---|
| 01 | `biggz review list`; `biggz review status <lineage> --json` (lineage resolved from the change's runtime binding) | one provider-owned lineage returned with `chain_valid: true` and a receipt or valid `integrity_verdict`; `receipt` present means approved | blocked |
| 02 | provider-returned transition (`biggz review start --subject <file>` / `biggz review resume <lineage>` / `biggz review validate <lineage>`) | exact operation/arguments completed or `collect` inputs satisfied; `stop` halts without a lifecycle operation | blocked |
| 03 | repeat 01–02 | `biggz review gate pre-pr\|pre-push <lineage> --json` returns `passed: true` against the same receipt; exit 1 with reasons blocks the terminal gate | blocked |
| 04 | `biggz review bind-sdd <change> <lineage> <revision>`; refresh BigMem `sdd/{change}/review/gate-context` | existing mirrors reconciled to the native store | allowed |

After ambiguous output, query STATUS again; native discovery reports the committed authority and its next transition without another budget. Malformed or ambiguous lineage remains invalid.
<!-- authority-first-terminal-procedure:end -->

## Delivery

Repository Git common-dir CAS remains authoritative. Existing transaction, policy, ledger, receipt, bundle, and gate-context schemas, prerequisites, and compatibility behavior remain unchanged in this work unit. Reconcile mirrors only after native allow. Supported lifecycle CLI gates are `pre-pr` and `pre-push` via `biggz review gate`; they discover and validate the same receipt and never launch reviewers or create a budget. Archive still requires structured status with `reviewGate.result: allow` and its approved receipt, or `reviewGate.delivery: disabled/unmanaged` while the kill switch is off. Model/provider/profile selection remains user-owned.

Before commit, stage all reviewed paths without content/mode changes, then validate the gate. Frozen intended-untracked paths must remain all untracked or all move to an index whose complete tree and paths match the receipt.
