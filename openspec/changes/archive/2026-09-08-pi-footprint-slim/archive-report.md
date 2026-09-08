# Archive Report: pi-footprint-slim

**Change**: pi-footprint-slim
**Archived**: 2026-09-08 → `openspec/changes/archive/2026-09-08-pi-footprint-slim/`
**Verdict at close**: PASS WITH WARNINGS — 8/8 tasks complete, 3/3 requirements, 9/9 scenarios, 0 blockers, 0 CRITICAL

## Final State (terminal record — outranks intermediate snapshots)

The proven deployed 429 relief is ported into sources so reinstall converges every install
(single PR ~200 lines, stacked-to-main, within 400-line budget — all uncommitted, pending user review):
`piDirectTools` in `internal/agents/pi/adapter.go` trimmed 20 → 10-tool allowlist
(`save`, `search`, `get_observation`, `context`, `session_summary`, `save_prompt`, `update`,
`timeline`, `review`, `judge`); `mergePiDirectTools` switched to allowlist-prune
(`removedPiDirectTools` drops exactly the 10 removed names, preserves foreign entries, atomic write);
3 orchestrator assets trimmed (REMINDER dupes → x1, markers/template/tokens verbatim);
mirror tests updated (fresh == 10, reinstall prunes stale 10, foreign preserved, idempotent).

Final evidence (orchestrator final-state facts, outranking `apply-progress`/`verify-report` snapshots):
8/8 tasks; live reinstall converged (`directTools` == 10 allowlist, APPEND_SYSTEM REMINDER == 1,
markers intact, server profile 20, `doctor pi-mcp-adapter` PASS); single PR ~200 lines stacked-to-main.

## Gates

- **Task Completion Gate**: PASS — `tasks.md` shows 8/8 checked (1.1–1.2, 2.1–2.3, 3.1–3.3).
  No stale-checkbox reconciliation was needed.
- **CRITICAL gate**: PASS — `verify-report` records 0 CRITICAL findings, verdict `pass_with_warnings`.
  Warnings are documentation-level only (token-location wording: `{{BIGGZ_BACKGROUND_POLICY}}`
  lives asset-level in delegation doc, not literally inlined in APPEND_SYSTEM by design).
- **Native Review Receipt Gate**: closed under orchestrator-granted relaxation — orchestrator stated
  "Archive needs NO ledger attempt. Full verify PASS WITH WARNINGS admitted 3/3 + 9/9;
  evidence_revision sha256:5c795312… recorded in verify-report (verify agent self-settled;
  ledger rev 73d99e80 complete — KNOWN anomaly, second occurrence, record it, do not attempt settle)".
  No new ledger transaction was opened by archive; no code was changed by archive.
- **Known anomaly (recorded, not resolved silently)**: verify-full ledger shows `complete:true`
  without an explicit settle (second occurrence of the anomaly); `evidence_revision
  sha256:5c795312ce19f4ea239517ca663ce7c6acff43e5ad3386e0912bde934ab01551` is recorded in
  `verify-report`. Future reader: re-check ledger state before assuming settle receipts exist.

## Specs Synced (Step 2, before move)

| Domain | Action | Details |
|--------|--------|---------|
| pi-integration | Already in tree — no edit by archive | 1 modified (Pi BigMem MCP Provisioning via Adapter → 10-tool allowlist + allowlist-prune + server-stays-20, 5 scenarios), 2 added (Slim APPEND_SYSTEM Generation, Reinstall Convergence and Rollback, 4 scenarios). Verified verbatim in `openspec/specs/pi-integration/spec.md`; untouched requirements preserved. |

Non-destructive merge: no existing requirement was deleted. Archive made zero edits to
`openspec/specs/` because the delta was already synced by apply-2.3; the file also carries the
coexisting already-archived `pi-wrapper-removal` diff (separate PR scope) which archive did NOT touch
per delegation caution.

## Archive Contents

- proposal.md ✅
- design.md ✅
- specs/pi-integration/spec.md (delta) ✅
- tasks.md ✅ (8/8 complete)
- apply-progress.md ✅
- verify-report.md ✅
- exploration.md ✅
- archive-report.md ✅ (this file)

## Verification (Step 4)

- [x] Main spec already held the delta verbatim — no archive edit, no wrapper-removal file touched
- [x] Change folder moved to `openspec/changes/archive/2026-09-08-pi-footprint-slim/`
- [x] Archive contains all artifacts listed above
- [x] Archived `tasks.md` has no unchecked implementation tasks
- [x] Active `openspec/changes/` no longer holds `pi-footprint-slim`
- [x] No `.biggz-instance` present — nothing to preserve
- [x] Hygiene Step 3b correctly skipped (non-TTY automation — delete nothing, no prompt)

## SDD Cycle Complete

Planned, implemented, verified, and archived. No commits were made; all changes (footprint-slim +
coexisting wrapper-removal diff) remain uncommitted for user review as a stacked-to-main PR.
