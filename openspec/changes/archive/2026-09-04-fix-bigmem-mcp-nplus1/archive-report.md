# Archive Report: fix-bigmem-mcp-nplus1

**Change**: fix-bigmem-mcp-nplus1
**Archived**: 2026-09-04
**Mode**: openspec
**Code commit**: e25b5ef4 (squash) — `feat(bigmem): batched relation hydration and paged export (SDD3)`
**Verify**: PASS WITH WARNINGS — 5/5 requirements, 11/11 scenarios
**Review lineage**: finalized with receipt (4 lenses, zero CRITICAL, no correction round)
**Tasks**: 14/14 complete, no stale checkboxes
**Do NOT commit**: archive file operations only, left uncommitted per instruction

## Skill resolution

- Read `internal/assets/biggz/biggz-orchestrator-workflow.md` before work: yes (SDD workflow, dependency graph, dispatcher, gates, ledger, archive handoff).
- Read `internal/assets/biggz/biggz-orchestrator-delegation.md` before work: yes (routing ladder, delegation rules, edit surfaces, lossless prompts, SDD agent authority).

## Task Completion Gate

`openspec/changes/fix-bigmem-mcp-nplus1/tasks.md` inspected before sync: all 14 tasks checked
(Phase 1: 1.1–1.4, Phase 2: 2.1–2.4, Phase 3: 3.1–3.4, Phase 4: 4.1–4.2). No `- [ ]` remains.
Gate PASSED — sync and move authorized. No stale-checkbox reconciliation needed.

## Native Review Receipt Gate (openspec adaptation)

Structured status reported `reviewGate.result: allow` lineage finalized with terminal receipt
(4 lenses, zero CRITICAL, no correction round) and `sdd-status: sync ready + archive ready`.
No `scope-changed`/`invalidated`/`escalated` state. `verify-report` shows zero CRITICAL findings.
Gate PASSED — archive authorized. (Exact transaction/ledger/receipt topics are Engram-side
review artifacts; in openspec mode the launch-prompt lineage + verify verdict + clean tree
constitute the governing evidence.)

## Specs synced

| Domain | Action | Details |
|--------|--------|---------|
| bigmem | Updated | 3 added, 0 modified, 0 removed — `openspec/specs/bigmem/spec.md` |
| cli | Updated | 2 added, 0 modified, 0 removed — `openspec/specs/cli/spec.md` |

Delta sources:
- `openspec/changes/fix-bigmem-mcp-nplus1/specs/bigmem/spec.md` — ADDED: Scoped relation lookup; mem_search annotation query bound; Explicit search-limit semantics.
- `openspec/changes/fix-bigmem-mcp-nplus1/specs/cli/spec.md` — ADDED: Paged export with explicit cap; Export shape and conflicts preservation.

Merge method: appended ADDED requirements to main spec Requirements sections; all pre-existing
requirements preserved verbatim; no MODIFIED/REMOVED/RENAMED entries so no replacement or
deletion was performed. Verified via `grep` — all 5 requirement headings present in main specs.

Source of truth now:
- `openspec/specs/bigmem/spec.md` — Scoped relation lookup, mem_search annotation query bound, Explicit search-limit semantics.
- `openspec/specs/cli/spec.md` — Paged export with explicit cap, Export shape and conflicts preservation.

## Archive move

```
openspec/changes/fix-bigmem-mcp-nplus1/
  → openspec/changes/archive/2026-09-04-fix-bigmem-mcp-nplus1/
```

Archive contents verified: proposal.md, specs/bigmem/spec.md, specs/cli/spec.md, design.md,
tasks.md (14/14 checked), apply-progress.md, verify-report.md, archive-report.md (this file).
Active `openspec/changes/fix-bigmem-mcp-nplus1/` no longer exists after move.

## What shipped (final state)

- Store: `SearchOptions.Offset` (OFFSET only when >0, legacy byte-identical) + `ListRelationsByIDs(ids)`
  (dedupe, empty→no query, 400-ID chunks, no LIMIT, ORDER BY created_at DESC, bound placeholders).
- `mem_search` (`cmd/biggz-mcp/main.go`): ID-union → single scoped lookup, in-memory title map with
  `"deleted"` fallback, zero hot-path `GetCtx`, unscoped `ListRelations("")` removed; limit validation
  (missing/non-numeric/≤0→20, >50→clamp 50 + stderr `limit clamped: requested=X effective=50`);
  errors never fail search.
- Export (`cmd/biggz/cli_bigmem.go`): 50-page Offset loop until short page or cap; `--limit N`
  (0/omitted=uncapped, negative=error exit 1) + `--project P`; JSON array shape unchanged (nil→`[]`).
- `conflicts list` untouched, byte-identical.
- Tests (per verify-report at verification time): focused suites 10/10 + 13/13 limit-parse + 5/5
  CLI export/conflicts PASS; full `internal/bigmem` ok; `go build ./...` + `go vet` exit 0.
  Known pre-existing failure `TestSDDStatusJSONEnvelopeDerivesStructuredFields` in full `cmd/biggz`
  suite documented as out-of-scope WARNING (also fails on clean master per apply stash evidence).
- Budget note: actual ≈830 lines (253 prod, 575 tests) vs 320–380 forecast; overrun entirely test
  code; single PR retained per auto-chain decision.

## WARNING follow-ups (review-recorded, NOT blockers)

Carried from review lineage (4 lenses, zero CRITICAL, no correction round) and verify WARNINGs.
None blocks archive; tracked here as the audit trail for future changes:

1. **Export `--limit`/`--project` help text** — verify the CLI help documents both flags with
   semantics (0/omitted=uncapped, negative=error; project filter forwarding). If help text is
   missing or vague, polish in a follow-up change.
2. **limit-0 semantics** — `mem_search` limit=0 currently defaults to 20 (invalid→default rule).
   Confirm this is the intended product semantic vs. treating 0 as uncapped/empty; document the
   decision in the next spec touch if ambiguous.
3. **Multi-relation overwrite parity with legacy** — annotation for observations with multiple
   relations keeps in-memory map resolution; confirm overwrite/merge order matches legacy
   per-rel Get behavior for identical output on multi-relation rows (golden-compare if needed).
4. **ORDER BY tiebreaker (Unit 1 follow-up)** — scoped query orders by `created_at DESC`; confirm
   tiebreaker behavior for equal timestamps matches `ListRelations` legacy ordering, or add an
   explicit secondary key in a follow-up if pagination determinism requires it.
5. **Verify WARNINGs (non-blocking)** — budget overrun accepted as test-heavy `size:exception`;
   pre-existing `TestSDDStatusJSONEnvelopeDerivesStructuredFields` failure stays out of scope;
   MCP live harness N/A (bound proven structurally + store-level).

## Verification checklist

- [x] Main specs updated correctly (5 ADDED requirements present, others preserved)
- [x] Change folder moved to `openspec/changes/archive/2026-09-04-fix-bigmem-mcp-nplus1/`
- [x] Archive contains all artifacts (proposal, specs, design, tasks, apply-progress, verify-report, archive-report)
- [x] Archived tasks.md has no unchecked implementation tasks (14/14)
- [x] Active changes directory no longer has this change
- [x] No CRITICAL issues in verify-report
- [x] No commit performed (working tree holds spec sync + archive move, unstaged)

## SDD Cycle Complete

The change has been fully planned, implemented, verified, and archived.
Ready for the next change.
