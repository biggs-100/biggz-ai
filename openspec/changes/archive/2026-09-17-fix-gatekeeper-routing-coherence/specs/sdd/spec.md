# Delta for sdd

## ADDED Requirements

### Requirement: Gatekeeper Routing Coherence from Dependency Authority

`routing_coherence` MUST be decided against the successor the dependency authority resolves for the change — `resolveNextRecommended` (`internal/sdd/status.go:1211`) as fed by the derivation entry points `deriveChangeStatusWithForcedStore` (`internal/sdd/status.go:465`), `deriveChangeStatusCtx` (`internal/sdd/status.go:703`) and the BigMem path (`internal/sdd/engram_status.go:378`) — and MUST NOT be decided against the static table `nextPhaseValid` (`internal/sdd/gatekeeper.go:92`). To do so, `checkRouting` (`internal/sdd/gatekeeper.go:404`) MUST receive the workspace root, the change name and the active store from its call site (`internal/sdd/gatekeeper.go:136`), and MUST NOT infer them from the phase result it validates. A declared `next_recommended` of `done` or empty stays terminal and MUST pass. Any other declared successor MUST be rejected unless it equals the dependency-resolved successor, and the rejection reason MUST name the expected phase. When the dependency state cannot be derived — change with no artifacts, unreadable workspace, artifact store unavailable — the check MUST fail closed naming the cause; it MUST NOT pass and MUST NOT be reported as skipped. Behavior of the sibling checks `artifact_existence` and `no_drift` MUST NOT change.

#### Scenario: Dependency-correct successor accepted

- GIVEN a `spec`-phase result for a change whose proposal, spec and design artifacts are complete, so the dependency authority resolves `tasks`
- WHEN `routing_coherence` validates the declared `next_recommended: tasks`
- THEN the check MUST pass, even though the retired table (`gatekeeper.go:92`) lists `design` as the only successor of `spec`

#### Scenario: Static table is no longer an authority

- GIVEN a `spec`-phase result declaring `next_recommended: design` while the dependency authority resolves `tasks`
- WHEN `routing_coherence` validates the declaration
- THEN it MUST fail with a reason naming `tasks` as the expected successor

#### Scenario: Genuinely incoherent successor still rejected

- GIVEN a `propose`-phase result declaring `next_recommended: archive` (the `TestGatekeeper_InvalidRouting` case, `internal/sdd/gatekeeper_test.go:106`) and a dependency authority resolving `spec`
- WHEN `routing_coherence` validates the declaration
- THEN it MUST fail with a reason naming `spec` as the expected phase AND the gatekeeper verdict MUST be failed

#### Scenario: Done and empty remain terminal

- GIVEN a phase result declaring `next_recommended: done` or `next_recommended: ""` for any derivable change state
- WHEN `routing_coherence` validates the declaration
- THEN the check MUST pass

#### Scenario: Underivable state fails closed naming the cause

- GIVEN a change with no artifacts, or an unreadable workspace, or an unavailable artifact store
- WHEN `routing_coherence` attempts to resolve the dependency successor
- THEN the check MUST fail with a reason naming the cause and MUST NOT be reported as passed or as skipped

#### Scenario: Normalized successor label

- GIVEN a declared `next_recommended: apply (3/5 tasks)` whose dependency-resolved successor is `apply`
- WHEN `routing_coherence` validates the declaration
- THEN the phase label MUST be normalized to `apply` and the check MUST pass
