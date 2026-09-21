# Delta for orchestrator

## MODIFIED Requirements

### Requirement: REQ-OR-003 — Route Field in Status and Continue

`sdd-status --json` MUST include a `route` field on active changes, with value `organic` or `sdd` (`deriveRoute`, `internal/sdd/status.go:1225`). An archived-only status is the sole case where `route` is empty. Organic work MAY declare an OPTIONAL orchestrator-supplied `subroute` in `{direct-inline, delegated-direct}`; the CLI MUST NOT infer, default, or synthesize `subroute`, because both sub-routes produce zero SDD artifacts (`internal/sdd/status.go:1224`). `sdd-continue` MUST include route context, plus the declared `subroute` when present. `route` MUST NOT be `direct-inline` or `delegated-direct`.
(Previously: `route` was required to be `direct-inline` or `delegated-direct`, values the CLI never emits)

#### Scenario: Status reports organic route for direct work

- GIVEN work routed direct-inline, no SDD active
- WHEN `biggz sdd-status --json` called
- THEN `route` MUST be `organic` and `subroute` MUST be `direct-inline` when the orchestrator declared it
- AND `nextRecommended` MUST equal the value reported without the declared subroute — declaring a subroute MUST NOT influence routing (no SDD next derived from it)

#### Scenario: Status reports organic route for delegated work

- GIVEN work routed delegated-direct
- WHEN `biggz sdd-status --json` called
- THEN `route` MUST be `organic`
- AND the declared `subroute` MUST be `delegated-direct`

#### Scenario: Subroute omitted when undeclared

- GIVEN organic work whose orchestrator declared no subroute
- WHEN `biggz sdd-status --json` called
- THEN `subroute` MUST be absent or empty
- AND it MUST NOT be guessed from artifact presence

#### Scenario: Status reports route for SDD work

- GIVEN work routed SDD, spec phase active
- WHEN `biggz sdd-status --json` called
- THEN `route` MUST be `sdd`
- AND `nextRecommended` MUST contain the SDD next phase

#### Scenario: Continue includes route context

- GIVEN work routed direct-inline with declared subroute
- WHEN `biggz sdd-continue <change>` called
- THEN output MUST include `route: organic` and `subroute: direct-inline`

