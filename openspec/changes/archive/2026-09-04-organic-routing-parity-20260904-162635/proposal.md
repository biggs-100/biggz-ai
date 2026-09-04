# Proposal: organic-routing-parity

## Problem Statement

biggz-ai forces SDD ceremony for every change, even trivial 2-file fixes. This creates friction when the agent could resolve work inline or via delegation without SDD artifacts. gentle-ai routes by context size and ambiguity; biggz should too.

The pain is real: executing `explore→spec→tasks→apply→verify` for a fix that belongs in a single inline edit is over-engineering that erodes trust and wastes tokens.

## Target Users

Any operator of biggz-ai (harness user running Pi, Claude Code, OpenCode, etc.) who dispatches the `biggz-orchestrator` to execute work. The orchestrator is the sole routing decision-maker; downstream `sdd-*` agents receive the routed work but do not choose their own route.

## Business Rules

1. Size never selects SDD. Context file count determines route, not risk or perception.
2. SDD is selected only by explicit user request or accepted proposal. Never silently enrolled.
3. Per-action delegation (tests, builds, review) does not change the selected route.
4. Direct/delegated work creates zero SDD artifacts, phase attempts, or synthetic runs.
5. Public states are Working, Checking, Ready, or Needs your decision.

## Product Outcome

After the change, the orchestrator routes work through:

| Route | When | What happens |
|---|---|---|
| **Direct inline** | Understand/verify from 1–3 files, or one mechanical file already understood | Inline edit, no artifacts, no delegation |
| **Delegated direct** | Understand needs 4+ files, writer touches 2+ non-trivial files, or broad research is needed | One explorer or one writer, bounded, no SDD artifacts |
| **Optional SDD** | Substantial ambiguity where proposal/spec/design/tasks materially reduce uncertainty | Propose SDD only on explicit request or accepted proposal |

Public states (replacing current synthesis markers):

| State | Meaning |
|---|---|
| **Working** | Implementation can still change |
| **Checking** | Functional proof and bounded review in progress |
| **Ready** | Exact candidate has sufficient evidence for the delivery route |
| **Needs your decision** | Safe convergence impossible; orchestrator presents cause, impact, and choices |

## Current-State Gap

Today the `Work Routing Ladder` in `biggz-orchestrator-delegation.md` describes routing but SDD remains the default mental model. No enforcement exists: the orchestrator can and does route trivial work to SDD without being wrong by current rules.

## Implications and Impact

- **Orchestrator guidance files** (`biggz-orchestrator-delegation.md`, `biggz-orchestrator-workflow.md`) require prompt-level rewrite of the routing ladder.
- **Synthesis markers** (`◆ phase·status·next`) are replaced by public state strings. Any external tooling parsing current markers must adapt.
- **`sdd-status` output** gains a `route` field (`direct|delegated|sdd`) when work is active.
- **`sdd-continue`** returns route context (`direct-inline`, `delegated-direct`) alongside `nextRecommended`.
- **No FSM changes** in the first slice: routing is prompt-level with CLI surface changes only.

## Edge Cases

| Edge | Behavior |
|---|---|
| Work starts inline, reveals ambiguity | At next safe boundary, offer SDD. Never silently enroll. Decline leads to Needs your decision. |
| Delegated writer touches 3 non-trivial files | Route was correct (delegated). Writer scope is already bounded; no route change mid-flight. |
| User explicitly requests SDD for trivial fix | Honor it. SDD is opt-in, not gated by size. |
| `sdd-status` called on direct work | Returns `route: direct-inline` with no `nextRecommended`. States cycle Working→Checking→Ready. |
| Review mode enabled during direct work | Review is independent of route. Delivery follows repo policy after Ready. |

## Scope Boundaries

### In scope (this change)

- Rewrite `Work Routing Ladder` in `biggz-orchestrator-delegation.md` with gentle thresholds (1-3 / 4+ / 2+) and explicit `size never selects SDD`.
- Add public state definitions (Working/Checking/Ready/Needs your decision) to `biggz-orchestrator-workflow.md`.
- Add `route` field to `sdd-status` output (Go change in `internal/sdd/status.go`).
- Add route context to `sdd-continue` output (Go change in CLI).
- Spec delta: `openspec/specs/orchestrator/spec.md` with routing requirements and GIVEN/WHEN/THEN scenarios.

### Explicitly out of scope

- BigMem sync backends (separate change).
- Install/planner TUI parity with gentle (separate change).
- RDD integration with route (already independent).
- Multi-model orchestration profiles.
- Engram Cloud sync.

## Business Risk

If routing is too permissive, SDD becomes unused for work that genuinely benefits from planning. Mitigation: the orchestrator still proposes SDD when ambiguity is substantial; the user decides.

## Rollback Plan

Prompt-only routing is fully reversible: revert the orchestrator guidance files. The `route` field in `sdd-status` is additive and non-breaking; removing it is a single-field deletion.

## Non-Goals (deferred)

- Package-manager installers (apt/pacman/scoop) — separate ecosystem change.
- Engram Cloud sync — requires external service integration.
- GGA git-hooks — explicitly rejected in `docs/comparison-with-gentle.md`.
- Advisory review transport — absent by design.

## Proposal Question Round

Questions for orchestrator behavior refinement:

1. Should `sdd-status` report `route` when work is direct/delegated, or only when SDD is active?
2. Must `Needs your decision` block progress until answered, or can the orchestrator offer a default route with 10s auto-select?
3. When work transitions from direct to SDD, does the orchestrator generate a retroactive proposal, or start fresh from explore?

Assuming defaults: (1) always report route, (2) block until answered, (3) start fresh from explore.
