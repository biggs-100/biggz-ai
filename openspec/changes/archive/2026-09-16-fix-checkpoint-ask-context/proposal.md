# Proposal: Enforce Checkpoint Ask Decision Context in the Live Runtime

## Superseded premise

The first version of this proposal (same file, 2026-09-16) assumed that adding a substance rule to `internal/sdd/question.go`'s `ValidateQuestionEnvelope` and mirroring it in `internal/assets/pi/biggz-synthesis-gate.js` would **enforce** the rule. That premise is FALSE and is superseded by this revision — the record is kept below, not silently replaced.

## Evidence (re-verified, 2026-09-16)

- `ValidateQuestionEnvelope` (`internal/sdd/question.go:68`) has **no production call site** — only `internal/sdd/question_test.go:11` and `:47` call it.
- `internal/assets/pi/biggz-synthesis-gate.js` is **not deployed**: `internal/install/steps/pi_extensions.go:64-65` — "biggz-synthesis-gate.js wrapper is excluded: native Go synthesis gate (internal/sdd/synthesis_gate.go) is the only enforcement path" — and the install self-heal removes stale copies (`:257` `for _, stale := range []string{"biggz-synthesis-gate.js"}`).
- That "native Go synthesis gate" is itself unwired: `ShouldBlock`, `CheckSynthesisPrecondition`, `BuildBlockedEnvelope`, `ShouldBlockApplyAdmission`, `HasSynthesis`, `IsCheckpointAsk`, `HasOptions` have call sites only in `_test.go` files; the sole non-test reference is a comment — `internal/sdd/synthesis.go:204` "gate b0d2fc1 (HasSynthesis)".
- `biggz sdd-gate` (`cmd/biggz/cli_sdd.go:1584`) is the RDD/review gate — a different surface.
- `docs/architecture.md:230` claims the Go gate "is the enforced gate" and `:238` describes blocking in "BOTH paths" — false in this build.

Kept from v1 (re-verified): `internal/sdd/question.go:131` `if strings.TrimSpace(o.Description) == ""` rejects emptiness only, so "yes, go ahead" passes; the JS mirror never reads `description`. The v1 line ref `:130` was off by one.

## Intent

Issue #14: a checkpoint ask offered 3 options at ~15 words each — no evidence, scope, cost, risk, deferral — and the human had to demand a report before deciding. A rule nobody invokes is not a rule, so this change defines the contract, makes it enforceable in the **live runtime**, and wires the deployed ask asset to refuse non-conforming asks.

## Contract to enforce

An option's description MUST carry decision context proportional to the decision: evidence found/verified; per-option problem, scope (files/areas), effort, risk, what it unlocks, deferral cost; a recommendation with its reason; no-go conditions (if context cannot be produced, research more instead of asking). Fail-closed at ask time: no context → question not presented.

## Scope

### In Scope
- Substance rule in the envelope validator + Go tests.
- A `biggz` CLI check: synthesis precondition + envelope validity (incl. substance) → non-zero exit + actionable message; bounded-timeout callable.
- Wiring: deployed `internal/assets/pi/ask-user-choice.ts` invokes the check and refuses to present on failure (session-guard pattern: argv array, no shell, bounded timeout, degrade-open, test seam).
- Tests both sides, incl. that the deployed asset **actually calls** the check.
- Docs: extend `## Visible Context Before Every Question (MANDATORY)` (`biggz-orchestrator-workflow.md:146-154`) with issue #14's four items; correct `docs/architecture.md:230/238`.

### Out of Scope
- Checkpoint token lists, header/label/option-count limits, sub-agent synthesis, pending-question format.
- Deleting non-deployed `biggz-synthesis-gate.js` (Phase-2 decision).
- RDD/review gates.

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `sdd` — "Synthesis Gate Markers and 120s Window": adds option-substance requirement + the CLI check surface that makes the gate reachable.
- `orchestrator` — "REQ-ORCH-001 — Blocking Synthesis Checkpoint (120s)": enforcement clause becomes real via the deployed path; docs clause gains the four-item checklist.
- `pi-integration` — "Question Envelope Validation": rejection moves to the deployed TS asset invoking the CLI check (not the undeployed JS mirror); scenario: thin option → not presented.

## Approach

1. **Substance rule** — requirement as above. **What counts as "substance" (minimum length, required cost/risk signal, explicit recommendation field, or a combination) is a DESIGN decision.**
2. **Live enforcement path** — a `biggz` CLI surface evaluating the synthesis precondition + envelope. **The exact command name/shape and whether it reuses `synthesis_gate.go`'s entry points (`CheckSynthesisPrecondition`, `BuildBlockedEnvelope`, `ShouldBlockApplyAdmission`) are DESIGN decisions.** Constraint: bounded timeout — session-guard precedent `SESSION_STOP_TIMEOUT_MS = 1000` (`biggz-session-guard.js:10`).
3. **Wiring** — `ask-user-choice.ts` checks before presenting; refuses on non-zero. Mirror `biggz-session-guard.js:33-35` `execFileSync(bin, ["session-close","--check-only","--cwd",cwd], { timeout })`, degrade-open + warn (`:52`), `_setSessionStopExecForTest`-style seam.
4. **Docs** — workflow checklist + architecture truth pass (make claims true or honest).
5. **Tests** — Go CLI + `question_test.go`; node test asserting the asset's invocation (argv + timeout, precedent `biggz-session-stop.test.mjs:81`) and the refuse path.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/sdd/question.go` (+ test) | Modified | substance rule + fixtures |
| `cmd/biggz/main.go`, `cmd/biggz/cli_sdd.go` (+ new test) | Modified/New | check surface + dispatch |
| `internal/sdd/synthesis_gate.go` | Modified (maybe) | reuse/wrap entry points |
| `internal/assets/pi/ask-user-choice.ts` (+ new test) | Modified/New | pre-present check + refusal |
| `internal/install/steps/pi_extensions.go` (+ factory test) | Modified (maybe) | deploy-list/factory sync |
| `internal/assets/biggz/biggz-orchestrator-workflow.md` | Modified | four-item checklist |
| `docs/architecture.md` | Modified | enforcement claims corrected |

## Slice Split & Magnitude (own estimate)

| Slice | Contents | Authored lines (est.) |
|-------|----------|----------------------|
| 1 | CLI check + substance rule + Go tests | 280–420 |
| 2 | Wiring + install sync + asset test + docs | 160–280 |
| **Total** | | **440–700** |

Arithmetic: 440–700 vs the 400-line review budget → over budget at the midpoint. **400-line budget forces a chain: Yes** — `auto-chain` / `stacked-to-main`, Feature Branch Chain (slice 2 retargets to slice 1). Slice 1 alone can approach 400; if rule+tests exceed ~350, split it (rule+tests / CLI) into its own PR.

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| False rejects on terse valid asks | Med | Design sets the bar; fixtures include minimal valid asks |
| New rule also ends up unwired | Low | Slice 2 test asserts invocation (argv + timeout) |
| Check adds latency/failure to asks | Med | 1000 ms budget; only option-bearing asks; timeout → allow + warn |
| TS→CLI escaping issues | Low | argv array, no shell; payload shape is a design decision |
| Drift returns in docs | Low | Claims tied to the shipped CLI path |

## Rollback Plan

Revert slice commits: `ask-user-choice.ts` presents directly again, CLI dispatch removed, rule reverted to emptiness-only, docs reverted. No schema/persistence touched. Field escape hatch without revert: disable the invocation (degrade-open seam) — the rule then reverts to advisory.

## Dependencies

- `sdd-design` resolves the substance mechanism + command shape before apply.
- Delivery strategy `auto-chain`; 400-line budget; `stacked-to-main` chain.
- Store `both`: this file + BigMem `sdd/fix-checkpoint-ask-context/proposal`.
- Issue `biggs-100/biggz-ai#14` acceptance checkboxes.

## Success Criteria

- [ ] Issue #14: orchestrator docs require evidence+cost+risk context on every checkpoint question.
- [ ] Issue #14: labels-only checkpoint options are a review-visible discipline failure.
- [ ] CLI check exits non-zero with an actionable message on missing synthesis or thin options; 0 when valid; honors the bounded timeout.
- [ ] Deployed `ask-user-choice.ts` invokes the check and refuses to present on failure (test asserts argv + timeout + refuse path).
- [ ] Go tests + node tests green; rich asks and non-checkpoint questions unaffected.
