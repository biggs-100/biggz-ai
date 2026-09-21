# Design: fix-route-nextrecommended-clause

## Technical Approach

Spec-truthfulness fix, zero production deltas. The change record replaces an unsatisfiable clause in REQ-OR-003 scenario 1 with a falsifiable invariant — *a declared `subroute` MUST NOT influence routing; `nextRecommended` MUST equal the value derived without it* — and adds one regression guard in `internal/sdd/subroute_test.go` pinning the shipped behavior. `deriveRoute`, `resolveNextRecommended`, and `declaredOrganicSubroute` stay unmodified; the invariant already holds.

## Context and Constraints

- Defect is spec text: `openspec/specs/orchestrator/spec.md:678` demands `nextRecommended` be empty for organic direct work; `resolveNextRecommended` (`internal/sdd/status.go:1215-1222`) always returns `resolve-blockers` for an active change; change-less organic work has no `active` entry for `route`/`subroute`.
- Guard test only; no production fix exists to apply.
- `strict_tdd: false` (Standard mode): behavior-locking guard, not red-to-green.
- Review budget 400 lines; delivery `auto-chain` cached; forecast under budget.

## Architecture Decisions

| # | Option | Tradeoff | Decision |
|---|--------|----------|----------|
| D1 | New test in `subroute_test.go` vs. new `subroute_inert_test.go` | Separates concerns, duplicates fixtures | **Extend `internal/sdd/subroute_test.go` with one test** — reuses `isolatedSubrouteHome`, `seedSubrouteChange`, `deriveSubrouteChange`, and keeps the subroute surface's tests together |
| D2 | Equality-only assertion vs. equality + declaration-visibility | Equality alone could pass vacuously if the declaration never surfaced | **Both**: equality (a) plus `Subroute == "direct-inline"` on the declared derivation (b) plus `Route == "organic"` on both (c) |
| D3 | Two seeded derivations in one test vs. one derivation with in-place rewrite | Rewrite needs re-derivation anyway and hides the precondition | **Two seeds, one test**: derive twice from two `state.yaml` variants — `subroute: direct-inline` and no key |

### D2 — assertion set (minimal, non-vacuous)

Derive the same proposal-only organic change twice — declared variant (`subroute: direct-inline`) and no-key variant. Assert:

1. `withDecl.NextRecommended == withoutDecl.NextRecommended` — the declaration is routing-inert.
2. `withDecl.Subroute == "direct-inline"` — the declaration actually surfaced, so (1) is not vacuous.
3. `withDecl.Route == "organic"` and `withoutDecl.Route == "organic"` — precondition making the comparison meaningful.

Explicitly NOT asserted: `BlockedReasons`, `Active`, `ApplyState`, artifact/task widening, JSON wire shape (`TestSubrouteOrganicDeclared` already covers the wire key via `assertSubrouteWire`), and the `resolve-blockers` token value itself. Those belong to other requirements; pinning them would widen the guard beyond REQ-OR-003 scenario 1.

### D3 — falsifiability

The guard fails if anyone couples the declared subroute into routing — e.g., `resolveNextRecommended` branches on `cs.Subroute`, or `deriveRoute` downgrades a declared organic change. The two derivations would then diverge and assertion (1) breaks; a route-level coupling breaks (3). The guard cannot fail before this change — there is no code fix — so it locks behavior, it does not prove the old wording wrong. The non-empty fallback plus the change-less no-`active`-entry case prove the unsatisfiability; the test comment must say so.

## Data Flow

```text
state.yaml (A: subroute: direct-inline | B: no key)
   └─> deriveSubrouteChange ──> ChangeStatus{Route, Subroute, NextRecommended}
       assert: NextRecommended(A) == NextRecommended(B)
               Subroute(A) == "direct-inline"; Route(A) == Route(B) == "organic"
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `openspec/changes/fix-route-nextrecommended-clause/specs/orchestrator/spec.md` | Already written | MODIFIED REQ-OR-003 delta; five scenarios preserved; clause replaced with the routing-inertness invariant |
| `internal/sdd/subroute_test.go` | Modify (apply) | One new test reusing existing fixtures; comment names the guard's non-red nature |
| `openspec/specs/orchestrator/spec.md` | Modify (sync) | Merge the delta into the spec of record |
| `openspec/changes/fix-route-nextrecommended-clause/{tasks.md,verify-report.md,state.yaml}` | Change record | Orchestrator-owned per OpenSpec convention |

## Interfaces / Contracts

None. No CLI flag, no JSON key, no `state.yaml` schema change, no `--json` envelope change. The delta adds no vocabulary (`direct-inline` exists); the guard reads existing `ChangeStatus` fields from `StatusWithOptions`.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Declared subroute is routing-inert | New test in `internal/sdd/subroute_test.go` (Standard mode, no RED ceremony) |
| Integration | Status derivation over a temp workspace | Existing subroute-test boundary; no new harness |
| E2E | — | None; no CLI or process surface changes |

## Threat Matrix

`N/A — no routing/shell/subprocess/VCS/executable/process boundary modified.` The change adds a read-only test over existing in-process functions and edits spec text; no production routing path, shell, subprocess, repository automation, or executable classification changes.

## Migration / Rollout

No migration. **Sync is a large-MODIFIED destructive candidate**: the REQ-OR-003 block is ~39 lines, above `largeMutationThreshold = 20` (`internal/sdd/openspec-deltas.go:13`), so sync/archive MUST obtain explicit `allow-destructive` authorization before running. Delivery is one PR (`auto-chain` cached). Archived `add-odd-lane/**` is delivered evidence — out of scope, never edited.

## Rollback Boundary

Revert the delivery commit: the guard test disappears, the delta disappears with the change folder, and `openspec/specs/orchestrator/spec.md` returns to the unsatisfiable clause (documented status quo). No runtime behavior is involved.

## Open Questions

None — scope is fixed by the approved proposal. Non-technical gate: destructive-sync authorization (see Migration / Rollout).
