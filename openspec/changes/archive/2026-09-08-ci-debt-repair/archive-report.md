# Archive Report: ci-debt-repair

**Change**: ci-debt-repair
**Archived**: 2026-09-08 → `openspec/changes/archive/2026-09-08-ci-debt-repair/`
**Verdict at close**: PASS WITH WARNINGS — 15/15 tasks complete, 8/8 requirements, 16/16 scenarios, 0 CRITICAL

## Final State (terminal record — outranks intermediate snapshots)

CI debt triaged and repaired in the working tree (uncommitted, pending user review + PR decision;
delivery of the stacked chain needs issue #24 approval first, currently `status:approved` —
recorded as a pre-delivery need, not a code blocker):

- **Slice A (skill lint, spec-change)**: `HARD_MAX=3200` in `scripts/check-skill-lint.mjs` and
  `internal/skills/lint.go`; WARN-only exits 0, FAIL exits 1 (no `exit 2`); mirror-drift FAIL added.
  7 oversized skills trimmed to ≤1000 tokens in both mirrors (branch-pr 912, sdd-apply 994,
  sdd-verify 994, sdd-archive 998, sdd-design 972, sdd-explore 988, sdd-onboard 994).
- **Slice D (release smoke)**: `goreleaser-action` pinned to `v6.4.0` (+SHA
  `e435ccd777264be153ace6237001ef4d979d3a7a`) with explicit `version: v2.18.1` in
  `.github/workflows/ci.yml` (task text said `v6.3.1` — tag does not exist upstream, 404;
  deviated to newest v6 per design D3). No `.goreleaser.yaml` fix needed (repro clean).
- **Slice B (test timeouts)**: 30s `WithTimeout` in `updateRun`/`upgradeRun`; existing
  `httptest` fake wired into the two live tests — zero skips added (localhost-hermetic).
- **Slice C (e2e quarantine)**: 2 ticketed `#24` skips — unconditional in `TestOrganicDoctor`,
  windows-scoped in `TestDockerE2E` (ubuntu leg stays blocking per spec anti-over-broad rule).
  Evidence showed 2 distinct failing tests, not 3 as the task text assumed.

Final evidence (orchestrator final-state facts, outranking `apply-progress`/`verify-report`
snapshots): 15/15 tasks; full verify PASS WITH WARNINGS 8/8 + 16/16 admitted; slices A/D/B/C
each verified. Per `verify-report` at verification time, the WARNING below was open; no later
commit changed it, so it carries forward as the final state.

## Gates

- **Task Completion Gate**: PASS — `tasks.md` shows 0 unchecked (`grep -c "^- \[ \]"` → 0),
  15/15 checked across Phases 1–4. No stale-checkbox reconciliation was needed.
- **CRITICAL gate**: PASS — full `verify-report` records 0 CRITICAL findings, verdict
  `PASS WITH WARNINGS`; all four slice reports record `verdict: pass`, `critical_findings: 0`.
- **Native Review Receipt Gate**: closed under orchestrator-granted relaxation — orchestrator stated
  "NO ledger attempt needed (full verify settled passed, rev b773e273)". No new ledger transaction
  was opened by archive; no code was changed by archive.
- **Action Context Guard**: mode `openspec`, interactive, stacked-to-main auto-chain noted; no
  workspace-planning mode. Spec sync touched `openspec/specs/` per the sdd-archive contract
  (outside the delegation's listed edit surfaces — contract-mandated, recorded here).

## Specs Synced (Step 2, before move)

| Domain | Action | Details |
|--------|--------|---------|
| ci-e2e | Created | 1 added (Exact-Scope E2E Quarantine), 3 scenarios |
| ci-release | Created | 1 added (Pinned Reproducible Release Smoke), 3 scenarios |
| ci-test | Created | 2 added (Hermetic Bounded Update Tests, Ticketed Quarantines Only), 4 scenarios |
| skills | Already synced by apply | Delta MODIFIED + ADDED content verified present in working-tree `openspec/specs/skills/spec.md` (HARD_MAX 3200 buckets, wrapper exit 0/1, mirror sync, 7-skill trim plan); archive made no further edit |

New-domain deltas were copied verbatim (`cp`); no existing main spec existed to preserve.
Non-destructive merge: no existing requirement was deleted or renamed anywhere.

## Archive Contents

- `_meta.yaml` ✅
- `exploration.md` ✅
- `proposal.md` ✅
- `specs/ci-e2e/spec.md` (delta) ✅
- `specs/ci-release/spec.md` (delta) ✅
- `specs/ci-test/spec.md` (delta) ✅
- `specs/skills/spec.md` (delta) ✅
- `design.md` ✅
- `tasks.md` ✅ (15/15 complete)
- `apply-progress.md` ✅ (Slice A record)
- `verify-report.md` ✅ (PASS WITH WARNINGS, 0 CRITICAL)
- `verify-slice-a/b/c/d.md` ✅ (all pass, 0 CRITICAL)
- `archive-report.md` ✅ (this file)

No `.biggz-instance` was present in the change folder, so none was carried over.
Post-archive hygiene (Step 3b): skipped — non-TTY execution, deleted nothing
(`use --dry-run on CI` equivalent). No commits made; tree left uncommitted for user review.

## Follow-ups for Future Work (not blockers)

- **WARNING (carried forward)**: `TestDockerE2E` windows-scoped quarantine line verified
  statically only (local skip via daemon env guard); CI windows leg is the runtime backstop.
  Full 10min e2e and goreleaser snapshot NOT re-run per task (focused evidence + prior repro).
- **Pre-delivery need**: stacked PR chain (PR-A lint → PR-D release → PR-B timeouts → PR-C e2e)
  needs issue #24 approval before merge; tree is uncommitted pending user review + PR decision.
