# Proposal: fix-route-nextrecommended-clause

## Intent

`openspec/specs/orchestrator/spec.md:678` (REQ-OR-003, scenario 1) requires `nextRecommended` empty for organic work. Unsatisfiable: `resolveNextRecommended` (`internal/sdd/status.go:1215-1222`) falls back to `resolve-blockers`, never `""` for active changes; change-less organic work has no `active` entry to carry `route`/`subroute`. An unsatisfiable MUST fakes verification (`fix-false-green-guards` class); no sibling shares it (`rg 'MUST be empty' openspec/specs/`).

## Scope

### In Scope
- One `MODIFIED` delta for `orchestrator`: scenario 1 gets the invariant *a declared `subroute` MUST NOT influence routing — `nextRecommended` MUST equal the value derived without it*.
- Regression guard in `internal/sdd/subroute_test.go`: derive one change with and without `subroute: direct-inline`, assert equal `nextRecommended`, and assert the with-case surfaces the declaration (read, still inert).

### Out of Scope
- Production behavior: `deriveRoute`, `resolveNextRecommended`, `declaredOrganicSubroute` are correct as shipped; the defect is spec text.
- New subroute vocabulary, `route` changes, other REQ-OR-003 scenarios.
- Archived `add-odd-lane/**` (delivered evidence).
- Gates, workload, edit-authority, RDD surfaces.

## Capabilities

### New Capabilities
None

### Modified Capabilities
- `orchestrator`: REQ-OR-003 scenario 1 — unsatisfiable emptiness conjunct replaced by the subroute-is-routing-inert invariant.

## Approach

1. Delta carries full REQ-OR-003 (five scenarios preserved); the clause states its exact meaning: declaring a subroute cannot move a change toward SDD — not that nothing else changes routing.
2. Test: two-derivation equality in `subroute_test.go`.
3. Regression guard, NOT red-to-green: the invariant already holds; the test cannot fail before the spec edit. Name that in the comment.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `openspec/changes/fix-route-nextrecommended-clause/specs/orchestrator/spec.md` | New | MODIFIED delta for REQ-OR-003 |
| `internal/sdd/subroute_test.go` | Modified | Routing-inertness guard |
| `openspec/specs/orchestrator/spec.md` | Modified (sync) | Updated on sync |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Invariant misread as "nothing else changes routing" | Med | Clause scopes meaning to the declared subroute |
| Guard mistaken for proof the old wording was wrong | Med | Comment: guard locks behavior; the fallback proves the unsat |
| Sync blocks: full REQ-OR-003 (~34 lines) > threshold 20 (`internal/sdd/openspec-deltas.go:13`) | High | Expected; needs `allow-destructive` authorization |

## Rollback Plan

Revert the commit: delta removed, spec of record returns to current text (unsatisfiable clause back — documented status quo), test deleted. No runtime behavior involved.

## Dependencies

None.

## Workload Forecast

~250–350 authored lines (delta ~45, test ~50, record trail ~150–250): inside the 400-line budget with little headroom; risk is prose.

## Success Criteria

- [ ] Delta applies, clause gone, invariant present; other REQ-OR-003 scenarios intact.
- [ ] New test passes and would fail if `subroute` influenced `nextRecommended`.
- [ ] `go build ./...`, `go vet`, focused `internal/sdd` suite green.
- [ ] `openspec/specs/**` keeps no `nextRecommended MUST be empty` claim.
