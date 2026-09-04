# Delta Spec: Organic Routing Parity

> This is a DELTA spec. It adds to the existing orchestrator spec (`openspec/specs/orchestrator/spec.md`). Core orchestrator behavior is unchanged; this adds three routing routes, public states, and a route field in status/continue output.

## Requirement: REQ-OR-001 — Organic Implementation Routing

The orchestrator MUST route authorized work through exactly one of three implementation routes based on context file count and ambiguity. Size, risk, or perception alone NEVER selects SDD.

| Route | When | What happens |
|---|---|---|
| **Direct inline** | Understand/verify from 1–3 files; or one mechanical, already-understood file with no research or unresolved design decision | Inline edit. No artifacts. No delegation. No SDD. |
| **Delegated direct** | Understand needs 4+ files; reading prepares a write; broad research needed; or writer touches 2+ non-trivial files | One scout or one writer. Bounded. No SDD artifacts. |
| **Optional SDD** | Substantial ambiguity where durable proposal/spec/design/tasks materially reduce uncertainty | Propose SDD. Selected only by explicit request or accepted proposal. |

File counts describe context needed for the current action, not a risk score and not an SDD threshold.

### Scenario: Direct inline for 2-file fix

- GIVEN user says `fix the typo in README.md and update the test in parser_test.go`
- WHEN orchestrator evaluates route
- THEN it MUST route direct-inline: no SDD artifacts, no delegation, inline edit

### Scenario: Delegated direct for broad refactor

- GIVEN user says `add error handling to all 5 files in internal/sdd/`
- WHEN orchestrator evaluates route
- THEN it MUST delegate one writer (2+ non-trivial files) and produce no SDD artifacts

### Scenario: SDD only on explicit request

- GIVEN user says `implement full authentication middleware`
- WHEN orchestrator evaluates route
- THEN it MUST propose SDD (explicit request)
- AND it MUST NOT auto-enroll without acceptance

### Scenario: Size never selects SDD

- GIVEN user says `rename one variable across 12 files`
- WHEN orchestrator evaluates route
- THEN it MUST route delegated-direct (4+ context files) regardless of size
- AND it MUST NOT route to SDD because of file count

### Scenario: Ambiguity offers SDD at next boundary

- GIVEN work starts inline, reveals substantial ambiguity mid-flight
- WHEN orchestrator reaches next safe boundary
- THEN it MUST offer SDD via explicit proposal
- AND declining MUST lead to Needs your decision — never silent enrollment

### Scenario: Per-action delegation does not change route

- GIVEN work routed direct-inline, tests need fresh worker
- WHEN orchestrator dispatches test runner
- THEN it MAY use a fresh worker without changing route
- AND route MUST remain direct-inline

### Scenario: Direct/delegated creates zero SDD artifacts

- GIVEN work routed direct-inline or delegated-direct
- WHEN implementation completes
- THEN it MUST NOT create proposal.md, spec.md, tasks.md, or phase attempts
- AND it MUST NOT create synthetic SDD runs

## Requirement: REQ-OR-002 — Public Implementation States

The orchestrator MUST expose exactly four public states. These replace current synthesis markers (◆ phase·status·next).

| State | Meaning |
|---|---|
| **Working** | Implementation can still change |
| **Checking** | Functional proof and bounded review in progress |
| **Ready** | Exact candidate has sufficient evidence for the delivery route |
| **Needs your decision** | Safe convergence impossible; orchestrator presents cause, impact, and choices |

### Scenario: Working state on active edit

- GIVEN orchestrator is editing inline
- WHEN state is evaluated
- THEN it MUST report `Working`

### Scenario: Checking state on test run

- GIVEN orchestrator launched `go test` after direct-inline edit
- WHEN tests are running
- THEN it MUST report `Checking`

### Scenario: Ready state on passing tests

- GIVEN tests pass, no pending changes
- WHEN orchestrator evaluates state
- THEN it MUST report `Ready`

### Scenario: Needs your decision on ambiguity

- GIVEN work revealed ambiguity, SDD offered, user declined
- WHEN orchestrator cannot safely converge
- THEN it MUST report `Needs your decision` with cause, impact, and choices

### Scenario: State replaces synthesis markers

- GIVEN old marker format `◆ spec · success · design`
- WHEN new state system active
- THEN output MUST use `Ready` or `Needs your decision` — NOT old ◆ markers

## Requirement: REQ-OR-003 — Route Field in Status and Continue

`sdd-status --json` MUST include a `route` field on active changes. `sdd-continue` MUST include route context in output.

### Scenario: Status reports route for direct work

- GIVEN work routed direct-inline, no SDD active
- WHEN `biggz sdd-status --json` called
- THEN `route` MUST be `direct-inline`
- AND `nextRecommended` MUST be empty (no SDD next)

### Scenario: Status reports route for delegated work

- GIVEN work routed delegated-direct
- WHEN `biggz sdd-status --json` called
- THEN `route` MUST be `delegated-direct`

### Scenario: Status reports route for SDD work

- GIVEN work routed SDD, spec phase active
- WHEN `biggz sdd-status --json` called
- THEN `route` MUST be `sdd`
- AND `nextRecommended` MUST contain the SDD next phase

### Scenario: Continue includes route context

- GIVEN work routed direct-inline
- WHEN `biggz sdd-continue <change>` called
- THEN output MUST include `route: direct-inline`

## Requirement: REQ-OR-004 — SDD Opt-In Gate

The orchestrator MUST NOT auto-enroll work into SDD. SDD is selected only by: (1) explicit user request (`use SDD`, `propose a spec`, or equivalent), or (2) accepted proposal after the orchestrator offered it.

### Scenario: Explicit request triggers SDD

- GIVEN user says `use SDD for this`
- WHEN orchestrator evaluates
- THEN it MUST route to SDD and begin with `sdd-new` or `sdd-propose`

### Scenario: Proposal accepted triggers SDD

- GIVEN orchestrator offered SDD proposal, user accepted
- WHEN orchestrator evaluates
- THEN it MUST route to SDD and begin planning phases

### Scenario: Auto-enrollment blocked

- GIVEN orchestrator working inline, detects ambiguity
- WHEN it considers SDD enrollment
- THEN it MUST offer SDD at next safe boundary
- AND it MUST NOT auto-enroll without explicit acceptance

### Scenario: SDD declined stays on current route

- GIVEN orchestrator offered SDD, user declined
- WHEN work continues
- THEN route MUST remain direct-inline or delegated-direct
- AND state MAY become Needs your decision if ambiguity persists

## Requirement: REQ-OR-005 — Routing Ladder Thresholds

The orchestrator MUST use these exact thresholds for route selection:

| Context files | Route |
|---|---|
| 1–3 | Direct inline |
| 4+ | Delegated direct |
| Ambiguous + user request | Optional SDD |

Writer dispatch: 2+ non-trivial files → delegated direct. Per-action delegation (tests/builds) does not change route.

### Scenario: Threshold 3 stays inline

- GIVEN 3 files to understand
- WHEN route evaluated
- THEN it MUST be direct-inline

### Scenario: Threshold 4 delegates

- GIVEN 4 files to understand
- WHEN route evaluated
- THEN it MUST be delegated-direct

### Scenario: Writer 2+ non-trivial delegates

- GIVEN writer needs to change 2 non-trivial files
- WHEN route evaluated
- THEN it MUST be delegated-direct

### Scenario: Writer 1 non-trivial stays inline

- GIVEN writer needs to change 1 non-trivial file
- WHEN route evaluated
- THEN it MAY stay direct-inline

## Requirement: REQ-OR-006 — Direct/Delegated Create Zero SDD Artifacts

Direct-inline and delegated-direct work MUST NOT create SDD artifacts, phase attempts, synthetic SDD runs, or change `openspec/changes/` state. Only SDD-routed work touches SDD artifact store.

### Scenario: Direct work leaves openspec clean

- GIVEN direct-inline fix applied
- WHEN `openspec/changes/` inspected
- THEN no new directories or files MUST exist

### Scenario: Delegated work leaves openspec clean

- GIVEN delegated-direct refactor completed
- WHEN `openspec/changes/` inspected
- THEN no new directories or files MUST exist

### Scenario: SDD work creates artifacts

- GIVEN SDD route accepted, spec phase launched
- WHEN phase completes
- THEN `openspec/changes/<change>/specs/` MUST exist with spec.md

## Non-Goals (this change)

- BigMem sync backends.
- Install/planner TUI parity.
- RDD integration with route (already independent).
- Multi-model orchestration profiles.
- Package-manager installers.
- Engram Cloud sync.
- GGA git-hooks (rejected per `docs/comparison-with-gentle.md`).
