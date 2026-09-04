# Exploration: organic-routing-parity

## Problem
biggz-ai forces SDD ceremony even for trivial work. gentle-ai routes by
context size and ambiguity, SDD is opt-in. Goal: port that routing without
losing biggz discipline (Recall, Preflight, gates).

## Sources
- gentle: `docs/trigger-rules.md`, `internal/components/agentguidance/routing.go::RenderRouting`
  - Direct inline: 1-3 files understanding, or 1 mechanical file, no research.
  - Delegated direct: 4+ files understanding, or 2+ non-trivial writes.
  - Optional SDD: only on explicit request or accepted proposal. Size/risk never selects SDD.
  - States: Working -> Checking -> Ready, or Needs your decision.
  - Per-action delegation (tests/builds) does not change route. Direct/delegated create zero SDD artifacts.
- biggz today: `internal/assets/biggz/biggz-orchestrator-delegation.md` (Work Routing Ladder),
  `biggz-orchestrator-workflow.md` (Recall/Preflight/Dispatcher). Ladder exists but SDD is default mental model.

## Approaches
1. **Prompt-only port** (recommended): add `trigger-rules` block to orchestrator guidance + delegation ladder thresholds (1-3 / 4+ / 2+), explicit `size never selects SDD`, public states. No Go changes. Low risk, reversible.
2. **Native gate**: enforce route in `sdd-status`/`sdd-continue` (new token `direct|delegated`). Stronger but adds FSM complexity and risks reintroducing the debt biggz removed.
3. **Do nothing**: keep SDD-default. Rejected: over-engineering trivial changes, against CONCEPTS>CODE.

## Recommendation
Go with 1. Scope: `biggz-orchestrator-delegation.md` + `biggz-orchestrator-workflow.md` + one spec `openspec/specs/orchestrator/spec.md` delta. No ledger, no review, no TUI changes in this slice.

## Open questions
- Should Preflight ask route explicitly, or infer and report?
- Do we need `Working/Checking/Ready` strings verbatim for parity, or keep biggz synthesis markers?
