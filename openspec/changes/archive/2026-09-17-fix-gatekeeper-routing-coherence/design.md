# Design: fix-gatekeeper-routing-coherence

## Intent / Context

`routing_coherence` (`internal/sdd/gatekeeper.go:404`) must decide against the successor the dependency authority resolves for the change, not the retired table `nextPhaseValid` (`gatekeeper.go:92`). Contract: authority-decided successor; `done`/empty terminal; underivable fails closed naming the cause; sibling checks untouched. Spec: `specs/sdd/spec.md`.

## Architecture Decisions

### D1 — Plumbing: sibling-shaped parameters

| Option | Tradeoff | Decision |
|---|---|---|
| `checkRouting(openspecRoot, changeName, completedPhase string, store ArtifactStore, result *PhaseResult)` | Mirrors `checkArtifacts` | **Chosen** |
| Infer from the result | Impossible; contract forbids | Rejected |
| Context struct | New type for 4 values | Rejected |

Call site `:136`: `gr.checkRouting(openspecRoot, changeName, completedPhase, store, result)`.

### D2 — Resolution per store: one authority, forced store

| Store | Entry point | Compared |
|---|---|---|
| `openspec` | `readChangeWithForcedStore` → `deriveChangeStatusWithForcedStore` (`status.go:465`) → `resolveNextRecommended` (`status.go:1211`) | `NextRecommended` |
| `hybrid` | Same, `ArtifactStoreHybrid`, when the record is on disk; else BigMem: `collectBigMemChangesWithArchiveCtx` → `deriveBigMemChangeStatus` (`engram_status.go:345`) | `NextRecommended` |
| `""` / unknown | No derivation — fail closed (D4) | — |

`normalizeGatekeeperStore` maps `engram`/`bigmem`/`both` to `hybrid`, so BigMem resolves through the hybrid branch. Filesystem-before-BigMem mirrors `mergeFilesystemAndBigMem`. Rejected: `deriveChangeStatusCtx` (`status.go:703`) — re-reads the config store, can diverge from the run's store; `StatusWithOptions` — heavyweight, fallback-only.

### D3 — Unknown phase: skip with reason

Other phases (`research`, `sync`) get `Skipped=true` plus `phase %q is outside the routing contract; no dependency successor is defined`. Verdict unchanged; only the reason text changes (was empty). Not fail-closed: no successor to resolve.

### D4 — Underivable taxonomy (fail closed, not skipped)

Set `Passed=false, Skipped=false`, prefixed `cannot resolve the dependency successor for change %q: `:

| Cause | Reason names |
|---|---|
| Store none | `artifact store is none: no artifact store is active for this run` |
| Unknown store | `unknown artifact store %q` |
| Record absent | `no record under store %q: <abs change dir>` (+ `; no BigMem topics under sdd/<name>/` for hybrid) |
| Derivation error | Wrapped error (`read change-instance marker …`, `bigmem sdd-status …`) |

Boundary: zero-artifact records (`state.yaml` only) stay DERIVABLE (fallback resolves `propose`); fail-close = record absent. Post-archive non-terminal declarations are underivable.

### D5 — Normalize, then terminal, then resolve

`normalizeRoutingLabel`: `TrimSpace`, strip `" (…)"` (`apply (3/5 tasks)` → `apply`). Terminal `done`/empty is checked BEFORE resolution.

## Interfaces / Contracts

```go
func (gr *GatekeeperResult) checkRouting(openspecRoot, changeName, completedPhase string, store ArtifactStore, result *PhaseResult)
func resolveRoutingSuccessor(openspecRoot, changeName string, store ArtifactStore) (string, error)
func bigMemRoutingSuccessor(workspaceRoot, changeName string) (string, bool, error)
func normalizeRoutingLabel(declared string) string
func isRoutingContractPhase(phase string) bool
```

Check name stays `routing_coherence`.

## Data Flow

```
sdd-gatekeeper (cli_sdd.go:1341) · store ← ResolvePreflightPrefs(cwd)
  checkRouting(openspecRoot, change, phase, store, result)   ← new plumbing (:136)
    ├─ phase ∉ validPhases    → skip + reason (D3)
    ├─ declared done/empty    → PASS (D5)
    └─ resolveRoutingSuccessor
         ├─ none/unknown      → error → FAIL cause (D4)
         ├─ openspec          → readChangeWithForcedStore → resolveNextRecommended
         └─ hybrid            → on disk? openspec path · else BigMem collect
    equal → PASS · mismatch → FAIL(expected) · error → FAIL(cause)
```

## File Changes

| File:line | Action | Change |
|---|---|---|
| `internal/sdd/gatekeeper.go:92-102, 136` | Modify | Delete `nextPhaseValid`; call site passes `openspecRoot, changeName, store` |
| `internal/sdd/gatekeeper.go:403-447+` | Modify | Rewrite `checkRouting` (guard, terminal, resolve, compare, fail-closed); add resolver, BigMem lookup, normalizer, contract-phase helper (~120 lines) |
| `internal/sdd/gatekeeper_test.go:177-207, 209-267, ~363` | Modify | Canonical `specs/sdd/spec.md` fixtures; `VerifyCanRemediate` declares `remediate`; none-case `wantPass` true→false |
| `cmd/biggz/sdd_gatekeeper_cli_test.go:110-130` | Modify | Preflight-none: exit 1 + `artifact store is none` |

## Test Matrix

Table-driven `TestGatekeeper_RoutingCoherence` (store-aware pattern + `findCheck`):

- **dependency-correct accepted** — spec; proposal+spec+design; `tasks` → PASS (S1)
- **retired-table rejected** — same; `design` → names `tasks` (S2)
- **incoherent rejected** — propose; `archive` → names `spec` (S3; `TestGatekeeper_InvalidRouting` pins the verdict)
- **done / empty terminal** — complete; `done`, then `""` (S4)
- **underivable** — no record / store `""` / `.biggz-instance` dir → FAIL, not skipped, names cause (S5)
- **normalized label** — apply; `apply (3/5 tasks)` (S6)
- **empty record derives propose** — explore; no artifacts (D4)
- **unknown phase skip** — `sync` → skipped + reason (D3)
- **hybrid BigMem-only** — seeded topics, no dir (D2)

Suite green in the same commit; routing fixtures stay below all-done (auto-record inherited).

## Rollback

No migration, flags, or persisted state. `git revert` the commit; table and comparison restore verbatim.

## Threat Matrix

N/A — no shell/subprocess/VCS/PR/executable-classification/process boundary; in-process comparison.

## Workload Forecast

Production ≈ +120/−30; tests ≈ +175/−10, CLI +6/−3 → **≈ 300–380 changed lines, single PR holds**.

```
Decision needed before apply: No
Chained PRs recommended: No
400-line budget risk: Medium
```

Contingency if >400: slice 1 = production + repaired tests + core cases; slice 2 = extended matrix + CLI test.

## Open Questions

None blocking. Verify-time: "no artifacts" read as record-absent; zero-artifact records stay derivable.
