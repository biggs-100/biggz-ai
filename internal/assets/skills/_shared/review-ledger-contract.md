# Native Bounded Review Orchestration

Parent orchestrator and native CLI only. Never pass this contract to a reviewer, refuter, judge, correction actor, or validator. Those roles receive only scope, candidate-causal admission, severity, evidence requirements, and output shape.

## Route

Begin a review with `biggz review start --subject <file>` (optionally `--lineage <id>` for an explicit UUIDv7 lineage; with `--contract biggz-ai.review-integration/v1` a medium/high-risk candidate relays its typed consent envelope instead of proceeding — see Consent below). The native facade discovers the repository scope, derives the immutable subject target, selects the lens plan, and freezes the correction budget; correction and compatible base advance never recalculate risk or reopen review. Check `biggz review list [--json]` before starting to detect an in-flight lineage for the same subject.

A canonical four-lens selection is long work: before the first lens runs, give the one cost/side-effect forecast — four reviewer model runs over the frozen candidate, the frozen correction budget, and the at-most-one bounded correction it implies — once per candidate, never per lens.

**Negotiated routing (the ONLY routing authority).** Query `biggz review status <lineage> --contract biggz-ai.review-integration/v1 --next-transition`; it returns ONLY the provider-owned envelope `{schema, lineage, next_transition}` — no raw status fields, nothing to interpret. Route ONLY from `next_transition`; never from prose, raw status fields, or eligibility. The envelope names exactly one transition:

- `execute` — invoke the exact operation with the ordered argument tokens unchanged: `finalize <lineage>`, or `resume <lineage> --correction-lines <budget_remaining>` (the contract offers the max allowed forecast; the orchestrator may execute a lower one).
- `collect` — satisfy the named capture input with its exact capture operation: run `biggz review capture-result --preflight` with the input's `lineage`/`target`/`lens`/`order`/`expected_revision` (and `repository_context` when issued) to derive `subject_hash`, run the reviewer, capture with the same binding, then query STATUS again. `subject_hash` is intentionally omitted from the envelope: the preflight derives it before the real capture.
- `stop` — stop and surface `reason_code` without running a lifecycle operation. `ready_for_gates` means the finalized receipt exists: run the lifecycle gates (`biggz review gate <kind> <lineage>`) when the lifecycle demands, validating the same receipt.

**Consent.** When a start needs consent, `biggz review start --subject <file> --lineage <id> --contract biggz-ai.review-integration/v1` prints the typed `biggz-ai.review-consent/v1` envelope; every choice carries the exact follow-up invocation for that answer (`biggz review start --subject <file> --lineage <id> --consent granted|declined ...`, original flags echoed, the frozen candidate lineage pinned). Relay the envelope complete, get the human's answer, run EXACTLY the one named invocation, then query STATUS again. Never answer on behalf of the human; granted/declined is scoped to that one candidate; never hardcode or substitute START.

## State

The provider-owned lineage is the single source of truth. Query `biggz review status <lineage> --json`; it returns `lineage_id`, `head_hash`, `event_count`, `chain_valid`, `receipt`, `integrity_verdict`, and `budget_counters`. Route only from the returned state: an in-review lineage resumes with `biggz review resume <lineage>` (Blocked/NeedsChanges -> InReview); `biggz review validate <lineage>` verifies chain integrity; a `receipt` present means approved. Never substitute direct START, and the apply executor never launches review.

Reviewer findings enter the lineage through the native chain; the orchestrator never constructs canonical bytes or hashes. Only candidate-caused severe findings block: `pre-existing` and `base-only` become follow-ups, `unknown` escalates, WARNING/SUGGESTION remain info. Deterministic blockers need no refuter; inferential blockers share one read-only refuter batch. Judgment Day uses two independent judges.

## Transport

The reviewer runs as one foreground `task` call whose prompt is provider-owned and rewritten by the `review-result-artifacts` plugin — the caller-authored task body is discarded. The plugin is deployed by `biggz install` to the agent's plugin directory (`~/.config/opencode/plugins/` for OpenCode), which auto-loads it at startup; it is the transport between the orchestrator, the reviewer sub-agent, and the native capture CLI.

**Binding.** Begin the reviewer task prompt with the exact literal prefix `GENTLE_AI_REVIEW_BINDING ` — including the trailing space and never `=`; the same literal as gentle-ai, the de-facto standard across both projects — followed by one-line JSON assembled only from provider-returned state: `lineage`, `target` (subject commit), `lens`, `order`, `revision` from the lineage head (`expected-revision`), `repository_context` when provider-issued, and `subject_hash` when the artifact subject is already known; omit only provider-omitted fields. These are the prompt's first bytes. Never add `candidate_diff` or candidate bytes — the reviewer receives only the manifest reference.

**Preflight.** The plugin's `tool.execute.before` hook rejects background tasks and malformed bindings, then runs `biggz review capture-result --preflight` with the binding flags (no `--cwd` — biggz resolves the repository from the working directory; a provider-issued `repository_context` may pin it). The returned artifact subject (base/candidate trees + ordered changed-path manifest) is injected under `GENTLE_AI_REVIEW_CONTEXT ` followed by one-line JSON, and the binding is completed with the preflight `subject_hash`.

**Capture.** The reviewer echoes `subject_hash` and returns completed inspection over every manifest path, findings, and evidence. The plugin's `tool.execute.after` hook extracts strict JSON from the task result — rejecting empty and nested envelopes — runs `biggz review capture-result --input -` with the binding flags, and replaces the task output with the captured artifact.

**Quarantine.** biggz has no preserve CLI verb. On capture failure the raw payload is preserved without forwarding its contents: `.git/biggz/preserved-results/<lineage>-<lens>-<order>-<ts>.json` (exclusive write, never overwrite), at most 8 preserve attempts per session; only typed decisions (`reviewer artifact admission <decision>`, extraction classes) are forwarded. Re-capturing identical bytes can never satisfy admission; only a relaunched reviewer can produce a corrected result.

**Routing.** After each lens capture, query `biggz review status <lineage> --json` and proceed only from the returned state: when every selected lens slot is captured, `biggz review finalize <lineage>` materializes the terminal receipt, then `biggz review gate pre-pr|pre-push <lineage> --json` validates it. Never skip to a gate from transcript text alone.

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
