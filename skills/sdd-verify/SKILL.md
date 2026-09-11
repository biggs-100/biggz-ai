---
name: sdd-verify
description: "Verify SDD implementation against specs, design, and tasks. Run tests, validate requirements, check design coherence, and produce verify report. Trigger: orchestrator launches verification after apply."
disable-model-invocation: true
user-invocable: false
license: MIT
metadata:
  author: gentleman-programming
  version: "3.0"
  delegate_only: true
---
<!-- section:model-capable -->
## Language

Artifacts default to English (neutral Spanish only if explicitly requested for that artifact). Replies match the user's language; comments follow the target context language.

## Activation Contract

Quality gate: prove completion with source inspection plus real execution evidence.

Use structured status from `_shared/sdd-status-contract.md` (`schemaName`, `planningHome`, `changeRoot`, `artifactPaths`, `contextFiles`, progress, dependencies, `actionContext`) before judging.

## Hard Rules

- Read all status `contextFiles` first (proposal/specs/design/tasks; partial sets degrade per Graceful Handling).
- Full verification only when all tasks complete; else `blocked` without the suite.
- Run tests — static analysis is never verification; a scenario is compliant only with a passing covering test.
- Compare specs → design → task completion. Report issues; never fix them.
- Build the report as exact candidate bytes, then `biggz sdd-verify-validate` before any write. On validator denial/unavailability: zero writes, prior report untouched.
- Persist `verify-report` per mode (Engram / openspec file / hybrid both / inline-only `none`); return the Section D envelope.
- Strict TDD active → load `strict-tdd-verify.md`; else never load it.
- Count real requirements/scenarios (never invent totals); record commands, exit codes, output hashes.
- Model/provider/profile/effort selection is user-owned; verification never changes it.
- Contradictions/failing checks return FAIL/escalation — never another review loop. Native final verification consumes only the preterminal transaction + policy + ledger preimages (never terminal-only artifacts).
- Return and preserve exact canonical verification-evidence bytes (hashes can't reconstruct content).
- Ledger evidence (mandatory): `sdd-attempt acquire` before tests → `sha256sum` output → `sdd-attempt settle --evidence-revision sha256:<hash>` before the report. Persisted `evidence_revision` MUST equal the settled hash — never hand-edit. EXCEPTION (orchestrated runs): if the delegation prompt provides a ledger `token`, bind evidence to it and DO NOT acquire/settle — the orchestrator settles after validating the report. Self-managing the ledger alongside an orchestrator-held token causes invalid_continuation collisions.
- Before the RDD gate runs, write `<changeRoot>/review-subject.json` containing `{"repository":"<workspace root>","commit_sha":"HEAD"}` so the offered `biggz review start --subject '<changeRoot>/review-subject.json'` surfaced by `sdd-status` is runnable. `sdd-status` stays read-only; the writer lives in verify, never in status.
- Preflight-only denial (missing review authority) → failed strict envelope with `authority_only_failure/missing_review_authority: true`, `test/build_exit_code: 125`, observed authority revision. Never for substantive failures.
- Modern Go check: `*.go` touched → report MUST note `use-modern-go` `list` consulted; else WARNING (CRITICAL if an obvious fix was missed without `explain`).

## Decision Gates

| Condition | Action |
|---|---|
| Orchestrator says `STRICT TDD MODE IS ACTIVE` | Treat as authoritative. |
| Cached/config `strict_tdd: true` and runner exists | Strict TDD verify; load module. |
| Strict TDD false or no runner | Standard verify; skip TDD checks. |
| `actionContext.mode: workspace-planning` | STOP — unsupported in this slice. |
| Only tasks exist | Task completion only; record skipped dimensions. |
| Tasks + specs exist | Completeness + correctness; skip design coherence. |
| Full artifacts exist | Verify all dimensions. |
| Task incomplete | CRITICAL for core task, WARNING for cleanup task. |
| Test command exits non-zero | CRITICAL. |
| Spec scenario has no passing covering test | CRITICAL `UNTESTED` or `FAILING`. |
| Design deviation exists | WARNING unless it breaks a spec. |
| Go changes lack modern-guidelines evidence | WARNING (CRITICAL if an obvious modernization was missed). |

## Execution Steps

1. Load skills via Section A; retrieve artifacts via Section B (or `contextFiles` from status).
2. Resolve TDD mode from capabilities/config/project files.
3. Count tasks — any unchecked blocks full verification.
4. Map each spec requirement/scenario to implementation + covering test; check design vs code (skip + record if design missing).
5. Run test/build/coverage commands — inspection alone never proves compliance.
7a. Ledger gate (mandatory): `acquire` before tests, `sha256sum` the output, `settle` with that hash — report `evidence_revision` MUST match; never hand-edit.
7b. Modern Go check: confirm `use-modern-go` list was consulted for `*.go` changes or record WARNING.
8. Build the compliance matrix from actual test results; persist + return the report with skipped dimensions.

## Output Contract

Return `## Verification Report`: change, mode, completeness, build/test/coverage evidence, spec + correctness + coherence tables, CRITICAL/WARNING/SUGGESTION issues, verdict `PASS` / `PASS WITH WARNINGS` / `FAIL`.

## Graceful Artifact Handling

- **Tasks only**: completion only (`PASS WITH WARNINGS` max without runtime evidence); never claim spec/design correctness.
- **Tasks + specs**: completeness + correctness; missing covering tests are CRITICAL unless config allows manual verification.
- **Full artifacts**: completeness, correctness, coherence.
- **Unchecked tasks**: always CRITICAL.

## References

- [references/report-format.md](references/report-format.md) — report template + evidence fields.
- [strict-tdd-verify.md](strict-tdd-verify.md) — only when Strict TDD is active.
- `_shared/sdd-phase-common.md` — loading, retrieval, persistence, envelope.
<!-- /section:model-capable -->

<!-- section:model-small -->
---
name: sdd-verify
description: "Verify SDD implementation against specs, design, and tasks. Run tests, validate requirements, check design coherence, and produce verify report. Trigger: orchestrator launches verification after apply."
disable-model-invocation: true
user-invocable: false
license: MIT
metadata:
  author: gentleman-programming
  version: "3.0"
  delegate_only: true
---

> **ORCHESTRATOR GATE**: If you loaded this skill via the `skill()` tool, you are the ORCHESTRATOR — STOP. Do NOT execute these instructions inline. Do NOT delegate, do NOT call task/delegate, and do NOT launch sub-agents. Read this SKILL.md and follow it exactly.

## Language

Artifacts default to English (neutral Spanish only if explicitly requested for that artifact). Replies match the user's language.

## Purpose

You are a VERIFY sub-agent. You check implementation matches spec acceptance criteria with real test evidence. Do NOT delegate and do NOT fix issues.

## Hard Rules

- Spec acceptance criteria only; count real requirements/scenarios; inspect only changed files (max 3 at a time)
- Structured status; stop on workspace-planning. Run test + build commands — never inspection-only
- Before the RDD gate runs, write `<changeRoot>/review-subject.json` containing `{"repository":"<workspace root>","commit_sha":"HEAD"}` so the offered `biggz review start --subject '<changeRoot>/review-subject.json'` surfaced by `sdd-status` is runnable. `sdd-status` stays read-only; the writer lives in verify, never in status
- Strict envelope records command, exit code, `test_output_hash`, `build_output_hash`
- Report issues; never fix or re-loop. Go changes need `use-modern-go` evidence or WARNING

## Steps

1. Load ≤2 orchestrator-passed SKILL.md paths (only these); retrieve artifacts via Section B (or `contextFiles`)
2. Resolve TDD mode; load `strict-tdd-verify.md` only if active
3. Count tasks — any unchecked returns `blocked`
4. Map each requirement/scenario to implementation + covering test; check design coherence (skip + record if absent)
5. Run test/build/coverage; confirm `use-modern-go` evidence for Go changes (else WARNING)
6. Persist `verify-report` per mode after `biggz sdd-verify-validate`; return minimal JSON + Section D envelope.

## References

- `skills/_shared/sdd-phase-common.md` — shared loading, retrieval, persistence, and envelope
- `skills/sdd-verify/references/report-format.md` — report template

## Return Minimal Report

```json
{
  "status": "pass|fail",
  "checks": [{"criterion": "text", "result": "pass|fail", "evidence": "one-line"}],
  "next": "ready-for-archive|fixes-required"
}
```
<!-- /section:model-small -->


