# Sync Report — fix-attempt-ledger-scope

```yaml
schema: biggz-ai.sync-report/v1
change: fix-attempt-ledger-scope
date: 2026-09-14T04:24:15Z
result: applied
store: openspec (file-backed; hybrid semantics — filesystem wins)
invoked_via: temporary Go runner calling internal/sdd/sync.go:Sync (no CLI entry point exists); runner deleted after execution
prompt_marker: allow-destructive (maintainer-approved large MODIFIED on domain runtime)
domains: [runtime]
```

## Result

`SyncResult: applied` — message: `sync applied for change fix-attempt-ledger-scope`.

The applier (`internal/sdd/openspec-deltas.go` via `internal/sdd/sync.go`) synchronized the delta at `openspec/changes/fix-attempt-ledger-scope/specs/runtime/spec.md` into the canonical spec `openspec/specs/runtime/spec.md`. Nothing was committed and the change was not archived.

## Counts

| Metric | Before | Delta contribution | After |
|--------|--------|--------------------|-------|
| Requirements | 11 | +10 ADDED, 1 MODIFIED (in place) | 21 |
| Scenarios | 34 | +22 ADDED, replaced requirement 3 → 4 | 57 |

- **Replaced requirement**: `### Requirement: Interrupted Refund Capped at 2×` — full replacement (old 22-line block / 3 scenarios → new 29-line block / 4 scenarios). The cap semantics change from a change-wide `2×MaxAttempts` total to the live objective generation; the `(Previously: …)` annotation is carried from the delta. This is the mutation approved by the `allow-destructive` marker (`largeMutationThreshold = 20`, `internal/sdd/openspec-deltas.go:13`).
- **ADDED requirements** (appended): Successor Advance, Repeated Work Unit Refusal, Scope-Change Guard, Reset Preserves the Audit, Reset Opens a Fresh Budget Epoch, Generation Attribution, Lifetime Accounting, Pre-Change Ledger Compatibility, Remediation Admission, No New CLI Surface.
- **Untouched requirements**: all 10 remaining pre-existing requirements survive byte-verbatim (verified by block-level comparison against `git show HEAD:openspec/specs/runtime/spec.md`; the trailing requirement differs only by the standard blank separator line before the append). Spec header/purpose unchanged.
- **Delta fidelity**: every ADDED block and the MODIFIED block in the canonical spec are byte-identical to their delta counterparts (verified for all 11 blocks). Canonical file is valid UTF-8 without BOM.
- **Diff footprint**: `git diff --stat openspec/specs/runtime/spec.md` → 1 file changed, 187 insertions(+), 8 deletions(-). Deletions are exactly the old `Interrupted Refund Capped at 2×` lines.

## Guardrails consumed

| Guard | Outcome |
|-------|---------|
| Store gate (`openspec`/file-backed) | PASS — store is `openspec` |
| Verify PASS gate | PASS — `verify-report.md` verdict `pass_with_warnings` (`internal/sdd/verify.go:230` → `Passing=true`); `sdd-status` routed `verify: all_done`, `sync: ready` |
| RENAMED | Not present in delta |
| Legacy flat main spec | Not applicable — canonical spec uses `### Requirement:` headings |
| Destructive | Cleared by `allow-destructive` prompt marker (approved; see replaced requirement above) |
| Collision | None — `openspec/changes/` contains only this active change (`fix-attempt-ledger-scope`) plus `archive/` |
| Allowed edit roots | `openspec/specs/runtime/spec.md` within `actionContext.allowedEditRoots` (`C:\Users\USER\Desktop\biggz-ai`) |

## Invariants

- [x] Change directory still exists: `openspec/changes/fix-attempt-ledger-scope/` (proposal, specs, design, tasks, apply-progress, verify-report present) — the change is still ACTIVE, not archived (`openspec/changes/archive/` does not contain it).
- [x] No commit made — `git log --oneline -1` remains `f6ee068b Merge pull request #79 from biggs-100/feat/wire-pi-review-relay-sdd` (unchanged before/after sync).
- [x] No leftover runner — temporary runner at `tools/tmpsyncrun/` deleted after execution; `tools/` contains only the pre-existing `nosourcegrep`.
- [x] Native status re-read: `dependencies.sync: all_done`, `nextRecommended: archive`.
- [x] No BigMem spec artifacts touched (hybrid mode: filesystem wins); this report is mirrored to BigMem as its only observation.

## Next phase

`sdd-archive` — verify gate consumed, sync applied, change remains active awaiting archive with the usual post-merge flow.
