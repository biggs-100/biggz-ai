# Proposal: fix-gatekeeper-routing-coherence

## Intent

`checkRouting` (`internal/sdd/gatekeeper.go:404`) validates `next_recommended` against the hardcoded table `nextPhaseValid` (`internal/sdd/gatekeeper.go:92`) instead of the change's real dependency state. A result that correctly declares the successor the dependency authority resolves FAILS `routing_coherence`; the verdict depends on phrasing, not state. Defect record: `openspec/changes/archive/2026-09-16-fix-false-green-guards/state.yaml:32` (`gatekeeper-routing-static-table`, medium).

## Evidence (OBSERVED)

- Measured symptom: a `spec` result declaring `next_recommended: tasks` (dependency-correct once design is complete) FAILS; only the stale `design` passes (`gatekeeper.go:92`, `"spec": {"design"}`).
- `checkRouting` receives neither `openspecRoot` nor `changeName` (`gatekeeper.go:404`), unlike siblings `checkArtifacts`/`checkNoDrift`/`checkNoHallucination`; call site `gatekeeper.go:136`.
- Real authority: `resolveNextRecommended(...)` (`status.go:1211`), fed by `resolveDependencies`; called from `status.go:495`, `status.go:736`, `engram_status.go:378`.
- `TestGatekeeper_InvalidRouting` (`gatekeeper_test.go:106`) drives propose→archive — invalid under ANY derivation, stays RED-never-green. No test pins the dependency-correct successor as accepted.

## Scope

### In Scope
- `routing_coherence` accepts the successor the dependency authority resolves for the change; `done`/empty stay terminal; anything else is rejected naming the expected phase. The static table is no longer an authority.
- Underivable state FAILS naming the cause (no artifacts, unreadable workspace, store unavailable) — never approves because it could not look.
- Interface cost: `openspecRoot`, `changeName`, store plumbed into `checkRouting` (`gatekeeper.go:136`).
- Tests: dependency-correct successor accepted; genuinely invalid still rejected; underivable state fails naming cause.

### Non-goals
- Unifying `no_drift`/`artifact_existence`: both already store-aware and green (`TestGatekeeper_StoreAwareArtifactResolution`, PR #95).
- No behavior change outside `routing_coherence`.

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `sdd` — new requirement: `routing_coherence` MUST decide against the dependency-resolved successor; underivable state MUST fail closed naming the cause (`openspec/specs/sdd/spec.md:573`).

## Approach

Reuse the existing derivation entry points (`deriveChangeStatusWithForcedStore` `status.go:465` / `deriveChangeStatusCtx` `status.go:703`; BigMem path `engram_status.go:378`) in `checkRouting`; compare normalized `NextRecommended` to the resolved value. Derivation error → fail naming the cause. Both sides live in `internal/sdd`; no new persisted state.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/sdd/gatekeeper.go` | Modified | `checkRouting`; table retired |
| `internal/sdd/gatekeeper_test.go` | New | Dependency-correct + fail-closed tests |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Store derivation divergence | Med | Per-store tests (filesystem + BigMem paths) |
| Spurious fail-closed on sparse changes | Med | `done`/empty terminal; reason names the cause |
| Scope creep to sibling checks | Low | Explicit non-goal above |

## Rollback

`git revert` the single commit. `checkRouting` and the table are self-contained: no persisted state, no migration; previous comparison restores verbatim.

## Success Criteria

- [ ] `spec` result declaring dependency-correct `tasks` passes `routing_coherence`.
- [ ] Underivable state fails `routing_coherence` naming the cause; never passes.
- [ ] `TestGatekeeper_InvalidRouting` stays RED-never-green; `go test ./internal/sdd` clean.

## Dependencies

None external. Source: `fix-false-green-guards` archive. Delivery: `auto-chain` · 400-line · `stacked-to-main` · `both`.
