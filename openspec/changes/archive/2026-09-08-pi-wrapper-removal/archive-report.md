# Archive Report: pi-wrapper-removal

**Change**: pi-wrapper-removal
**Archived**: 2026-09-08 → `openspec/changes/archive/2026-09-08-pi-wrapper-removal/`
**Verdict at close**: PASS WITH WARNINGS — 11/11 tasks complete, 6/6 requirements, 15/15 scenarios, 0 blockers, 0 CRITICAL

## Final State (terminal record — outranks intermediate snapshots)

Phase 1 of the Pi wrapper removal shipped in the working tree (uncommitted, pending user review):
`biggz-memory-chrome.js` (14KB) and `biggz-synthesis-gate.js` (53KB) are excluded from
`piExtensionsDeployList()` (now 10 JS + 3 TS); `Apply()` self-heals stale deployed copies via
`os.Remove` (non-DryRun, silent no-op when absent). Factory and guard mirrors fixed (`12` → `10`).
The 1231-line `biggz-synthesis-gate.test.mjs` is deleted; CI/test clauses point at the Go canonical
gate (`internal/sdd/synthesis_gate_test.go`, `transcript_lint_test.go`,
`internal/assets/biggz/orchestrator_test.go`) plus `biggz-pi-extensions-factory.test.mjs`.
Docs (`docs/architecture.md`, `docs/validation-guide.md`) and `CHANGELOG.md` carry the Phase 1 entry.
Wrapper JS sources stay on disk pending the Phase-2 stability gate + one-release soak.

Final evidence (orchestrator final-state facts, outranking `apply-progress`/`verify-report` snapshots):
8 files, 32 insertions / 1270 deletions (test deletion dominated); `go vet` clean; `go test`
(`./internal/install/...`, `./internal/sdd/`) pass; node factory suite 11/11;
`biggz doctor` 19 ok, 0 CRITICAL (0 WARNING at apply-Unit-2 time).
Per `verify-report` at verification time, W1/W2 below were open follow-ups; no later commit
changed them, so they carry forward as the final state.

## Gates

- **Task Completion Gate**: PASS — `tasks.md` shows 11/11 checked (1.1–1.3, 2.1–2.3, 3.1–3.3, 4.1–4.2).
  No stale-checkbox reconciliation was needed.
- **CRITICAL gate**: PASS — `verify-report` records 0 CRITICAL findings, verdict `pass_with_warnings`.
- **Native Review Receipt Gate**: closed under orchestrator-granted relaxation — orchestrator stated
  "Archive phase needs NO ledger attempt (apply U2 settled passed, verify full PASS WITH WARNINGS
  admitted 6/6 15/15)". No new ledger transaction was opened by archive; no code was changed by archive.
- **Known anomaly (recorded, not resolved silently)**: the verify-full ledger shows `complete:true`
  without an explicit settle (rate-limit incident mid-phase); `evidence_revision
  sha256:10340a79…` is recorded in `verify-report`. Unit-1 settle rev `ac77f97`, U2 settle rev
  `578f4207` per orchestrator. Future reader: re-check ledger state before assuming settle receipts exist.

## Specs Synced (Step 2, before move)

| Domain | Action | Details |
|--------|--------|---------|
| pi-deploy-list | Updated | 1 modified (Guard Registered: exclusion of both wrappers), 2 added (Stale Wrapper Self-Heal, Factory Test Mirror) |
| pi-integration | Updated | 1 removed already in tree by apply-2.3 (Adapter-Aware Wrapper Fallback, with Reason + Migration), 1 modified already in tree (Synthesis Gate Verification and CI), 3 added by archive (Native-Only Pi Memory Path, Phase-2 Stability Gate, Single-Commit Rollback) |

Untouched requirements in both main specs were preserved verbatim. The REMOVED requirement carried
`(Reason: …)` and `(Migration: …)` in the delta, satisfying the removal rule. Non-destructive merge:
no existing requirement was deleted except the delta-directed removal.
Note: archive edited `openspec/specs/` per the sdd-archive contract (`sdd-archive Updates
openspec/specs/{domain}/spec.md`); all changes remain uncommitted for user review per delegation.

## Archive Contents

- `_meta.yaml` ✅
- `exploration.md` ✅
- `proposal.md` ✅
- `specs/pi-deploy-list/spec.md` (delta) ✅
- `specs/pi-integration/spec.md` (delta) ✅
- `design.md` ✅
- `tasks.md` ✅ (11/11 complete)
- `apply-progress.md` ✅ (Unit 1 + Unit 2 merged)
- `verify-report.md` ✅ (PASS WITH WARNINGS, 0 CRITICAL)
- `archive-report.md` ✅ (this file)

No `.biggz-instance` was present in the change folder, so none was carried over.
Post-archive hygiene (Step 3b): skipped — non-TTY execution, deleted nothing (`use --dry-run on CI` equivalent).

## Follow-ups for Future Work (not blockers)

- **W1**: wrapper self-heal proven by Unit-1 manual tmp-home harness only; committed
  `pi_extensions_drop_test.go` seeds 4 TS stalers, not the 2 wrappers — recommend extending the test.
- **W2**: legacy `internal/install/install.go` `DeployPiSynthesisGate`/`DeployPiMemoryChrome`
  remain live exported API with zero production callers (only test caller is
  `pi_memory_chrome_test.go`) — deprecate in Phase 2. (Also flagged in `apply-progress` Issues.)
- **S1**: Phase-2 source deletion needs one-release soak with zero fallback-firing reports,
  gated by the 5-criteria Phase-2 Stability Gate now recorded in `openspec/specs/pi-integration/spec.md`.
- Delivery: chained PRs recommended (400-line budget risk High; net new code ~80 lines but
  1231-line deletion dominates the diff). Suggested split per `tasks.md`: PR 1 code+mirrors,
  PR 2 delete+docs+evidence. User commits only on explicit ok — working tree left uncommitted.

## Source of Truth Updated

- `openspec/specs/pi-deploy-list/spec.md`
- `openspec/specs/pi-integration/spec.md`
