# Design: Enforce Checkpoint Ask Decision Context in the Live Runtime

## Technical Approach

The premise that a rule "nobody invokes" enforces anything is superseded: this change creates exactly ONE live invocation path. A new CLI verb `biggz sdd-ask-check` owns all rule logic in Go (reusing `synthesis_gate.go` entry points plus a new substance rule in `question.go`), and the deployed `ask-user-choice.ts` — through a small shared JS guard module following the `biggz-session-guard.js` precedent — invokes it via argv array + stdin under a 1000 ms bound, refusing to present on a decided block and degrading-open with a visible notice when the check cannot decide. Every design choice below traces to a delta-spec scenario; enforcement claims in `docs/architecture.md` are rewritten to name this path and stop asserting enforcement by unwired components.

## Architecture Decisions

| # | Decision | Options (tradeoff) | Choice + why |
|---|----------|--------------------|--------------|
| D1 | Substance mechanism | (a) min-length only — gameable with long filler, false-accepts "we discussed this at length"; (b) required cost/risk keyword only — one buzzword passes, false-rejects scope-only valid asks; (c) new `recommendation` field — breaks `additionalProperties:false` ask schema and `biggz-ai.pending-question/v1`; (d) all five context classes required — false-rejects terse-but-complete asks, violates the "context-bearing passes" scenario | **(a)∧(b) conjunction, bilingual**: `Substantive(description) := runeLen(TrimSpace(d)) ≥ 24 AND ≥ 2 distinct classes matched` over `{scope, effort, risk, unlock, deferral}` (case-insensitive word-boundary tokens, Spanish+English, e.g. scope `file(s)/path/commit/archivo/ruta`, effort `min/hour/test(s)/minutos`, risk `risk/revert/rollback/riesgo`, unlock `unlock/ship/desbloquea`, deferral `defer/later/blocked by/aplazar`). Each conjunct kills one attack (filler length; single buzzword) while the 24-rune floor admits complete terse asks. Deterministic, model-free, implemented ONCE in Go, testable without a human |
| D2 | Live check surface | `--question/--markdown` flags (win32 quoting/length), file path (no live writer of current-turn markdown exists), `sdd-gate ask` subverb (collides with the RDD review gate `cmd/biggz/cli_sdd.go:1584`) | **`biggz sdd-ask-check`** (house `sdd-<thing>` convention), reads one JSON payload from **stdin**: `{"question": "<raw ask params/envelope JSON>", "markdown": "<current-turn markdown>"}`. Stdin avoids shell/quoting entirely; the caller (TS asset) is the only live holder of current-turn markdown. The pure rule pipeline is a new thin entry `sdd.CheckCheckpointAsk` that reuses `CheckSynthesisPrecondition`/`ShouldBlock`/`ValidateQuestionEnvelope` — no parallel logic |
| D3 | Current-turn window in a stateless process | Pass `elapsedMs` (extra contract surface), enforce real 120 s from a persisted timestamp (no store exists) | The CLI seeds `SetCurrentTurnMarkdown(markdown)` once (one-shot process) so `ShouldBlock`'s window evaluates 0 s elapsed — the CALLER attests "this is the current turn"; the live turn-reset semantics live in the TS buffer (reset on `turn_start`/`agent_start`, D4). In-process 120 s behavior is untouched |
| D4 | Block vs degrade | Any non-zero blocks (a stale binary without the verb exits 1 with help text → traps every ask); only `0|1` coded (can't distinguish classes); throw on block (pi renders it as a tool crash) | Exit-code + token taxonomy (below). The guard blocks ONLY when output contains `blocked(synthesis_required|envelope_invalid|checkpoint_option_thin)` (session-close precedent: only the gate token blocks); any other failure (spawn ENOENT, timeout, unparsable/foreign non-zero, `BIGGZ_ASK_CHECK=0`) → `{degraded, notice}` → `ctx.ui.notify(notice, "warning")` + present. Refusal = `{content:[{type:"text",text:reason}], isError:true}`, question NOT presented, message visible to the agent (old gate-wrapper precedent, not a thrown error) |
| D5 | Rule integration point | CLI-only function (misses value-only checkpoint tokens), duplicated inside `ValidateQuestionEnvelope` only (misses `value`/`id`/`name`/`title` signals that real `ask_user_choice` envelopes use) | `ValidateCheckpointSubstance(env) error` is called from `ValidateQuestionEnvelope` (label-signaled checkpoints, spec-literal "envelope validation MUST additionally reject") AND from `CheckCheckpointAsk` when `IsCheckpointAsk(raw)` signals via `value`/`id`/`name`/`title`. One function, two call sites, no duplicated rule; substance errors wrap sentinel `ErrThinCheckpointOption` so the CLI maps `errors.Is` → exit 3 vs 2 |
| D6 | TS wiring shape | Inline TS logic (dual implementation = the original drift bug); buffer in Go (no live caller of `SetCurrentTurnMarkdown` exists) | New deployed plain-JS module `biggz-ask-guard.js` owns the turn buffer (`message_end`/`message_update` record; `turn_start`/`agent_start` reset — old-gate pattern) + `checkCheckpointAsk(params)`; `ask-user-choice.ts` imports it statically (extension-api ↔ session-guard precedent, shared module state) and calls it in `execute()` BEFORE `ctx.ui.custom`. Test seam `_setAskCheckExecForTest` mirrors `_setSessionStopExecForTest` |
| D7 | Slice split | One PR (440–700 authored lines > 400 budget), split at rule/CLI seam (CLI without rule is untestable end-to-end) | Two stacked slices (below); slice 2 retargets to slice 1's branch (Feature Branch Chain) |

### Substance predicate (verbatim, Go)

```go
// Substantive: floor AND at least two distinct context classes.
func CheckpointOptionSubstance(description string) (bool, string)
// runeLen(collapse(TrimSpace(d))) >= 24 && distinctClassesMatch(d) >= 2
// classes: scope | effort | risk | unlock | deferral (bilingual token tables, word-boundary regex, case-insensitive)
```

Boundary fixtures: REJECT `"yes, go ahead"` (13 runes, 0 classes) · REJECT `"Sí, adelante"` (0 classes) · REJECT `"Adopt B — clearly better; we discussed it at length earlier"` (long, 0 classes) · REJECT `"Run the tests and keep going as we agreed because it matters"` (1 class) · PASS `"Edit 2 files; low risk!"` (exactly 24 runes, scope+risk) · REJECT `"Edit 2 files; low risk."` (23 runes) · PASS `"Revert one commit; unblocks slice 2."` · PASS `"Revirte un commit; riesgo bajo; desbloquea slice 2."` · non-checkpoint envelope + thin option → pass (rule scoped to checkpoint asks only).

## Data Flow

```
pi turn: message_end/message_update ─▶ biggz-ask-guard.js buffer ─▶ reset on turn_start/agent_start
agent calls ask_user_choice(params)
  execute() ─▶ checkCheckpointAsk(params)
     ├─ BIGGZ_ASK_CHECK=0 ┊ spawn ENOENT ┊ timeout(1000ms) ┊ non-token exit
     │     └─▶ {degraded, notice} ─▶ ctx.ui.notify(warning) ─▶ present (existing rendering)
     └─ execFileSync("biggz[.exe]", ["sdd-ask-check"], {input: JSON{question, markdown}, timeout})
            Go: stdin ─▶ SetCurrentTurnMarkdown(md) ─▶ CheckSynthesisPrecondition
                 ─▶ ValidateQuestionEnvelope (+substance) ─▶ exit 0|1|2|3|4 + message
     ├─ exit 0 ─▶ present
     └─ blocked(*) token ─▶ {block, reason} ─▶ isError:true result, NOT presented
```

## Interfaces / Contracts

**Go** (`internal/sdd/ask_check.go` new; signatures — implementation pinned by tests):
```go
type AskCheckRequest struct { Question string `json:"question"`; Markdown string `json:"markdown"` }
type AskCheckResult struct { Code int; Message string } // Code maps 1:1 to process exit
func CheckCheckpointAsk(req AskCheckRequest) AskCheckResult
// internal/sdd/question.go:
var ErrThinCheckpointOption = errors.New("checkpoint_option_thin")
func CheckpointOptionSubstance(description string) (bool, string)
func ValidateCheckpointSubstance(q QuestionEnvelope) error // wraps ErrThinCheckpointOption
```
CLI core: `func runSddAskCheck(args []string, stdin io.Reader, stdout, stderr io.Writer) int` (house shape, `cli_sdd.go`).

**Exit codes** (stdout message; stderr only for usage):

| Code | Meaning | Class | Message shape (actionable) |
|------|---------|-------|---------------------------|
| 0 | both preconditions pass | allow | silent (empty stdout) |
| 1 | synthesis precondition fails | BLOCK | `blocked(synthesis_required): synthesis required: missing ## Sub-agent Result with 4 markers in current turn (120s window); emit the block first, adjacent, same turn, then retry` |
| 2 | envelope invalid (structure/limits/ownership) | BLOCK | `blocked(envelope_invalid): header exceeds limit 16: got 17 for question 0` |
| 3 | checkpoint option description thin | BLOCK | `blocked(checkpoint_option_thin): option "Yes, go ahead" (question 1) carries no decision context (needs ≥24 chars and ≥2 of scope/effort/risk/unlock/deferral signals); add scope, effort, risk, unlocks, deferral cost — or do more research before asking` |
| 4 | usage/input error (unparsable stdin, unknown flag) | INDETERMINATE | `error: cannot parse stdin payload: ...` |

Ordering: synthesis (1) → envelope incl. substance (2|3) → pass (0). A decided block must name the offending option/question; indeterminate exits never print a `blocked(` token.

**TS seam** (`internal/assets/pi/biggz-ask-guard.js`, deployed):
```js
export const ASK_CHECK_TIMEOUT_MS = 1000;
export function _setAskCheckExecForTest(fn);            // seam, session-guard parity
export function recordAskTurnText(t, {reset});          // buffer in / reset out
export function getAskTurnMarkdown();
export async function checkCheckpointAsk(params, opts = {});
// → undefined (pass) | {block: true, reason} | {degraded: true, notice}
```
`ask-user-choice.ts` `execute()`: `const verdict = await checkCheckpointAsk(params)`; `verdict?.block` → `return {content:[{type:"text",text:verdict.reason}], isError:true}` before any UI; `verdict?.degraded` → `ctx.ui.notify(verdict.notice, "warning")` (try/catch; fallback `console.warn`) then present.

## File Changes

| File | Action | Magnitude |
|------|--------|-----------|
| `internal/sdd/question.go` | Modify | ~70–90: sentinel, token tables, predicate, `ValidateCheckpointSubstance` + call in `ValidateQuestionEnvelope` |
| `internal/sdd/question_test.go` | Modify | ~120–160: boundary table (D1 fixtures), non-checkpoint unaffected |
| `internal/sdd/ask_check.go` | Create | ~80–100: request/result, ordered pipeline, message rendering |
| `internal/sdd/ask_check_test.go` | Create | ~80–110: taxonomy unit table |
| `cmd/biggz/cli_sdd.go` | Modify | ~60–80: `runSddAskCheck` (stdin read, exit mapping) |
| `cmd/biggz/main.go` | Modify | ~4: `case "sdd-ask-check"` dispatch |
| `cmd/biggz/cli_sdd_ask_check_test.go` | Create | ~140–180: exit table, both-preconditions, usage, message naming |
| `internal/assets/pi/ask-user-choice.ts` | Modify | ~50–70: import, execute-path check, refusal, degraded notice |
| `internal/assets/pi/biggz-ask-guard.js` | Create | ~100–130: buffer, exec invocation, token mapping, seam, no-op factory |
| `internal/assets/pi/biggz-ask-guard.test.mjs` | Create | ~130–170: argv/timeout/seam, block, degrade, buffer reset, TS source-scan |
| `internal/install/steps/pi_extensions.go` | Modify | ~2: deploy entry `pi/biggz-ask-guard.js` |
| `internal/install/steps/pi_extensions_guard_test.go` | Modify | ~2: JS count 12→13 |
| `internal/install/steps/pi_extensions_drop_test.go` | Modify | ~2: deploy-list expectation |
| `internal/assets/pi/biggz-pi-extensions-factory.test.mjs` | Modify | ~2: `DEPLOY_LIST` entry |
| `internal/assets/biggz/biggz-orchestrator-workflow.md` | Modify | ~10: four checklist items in `## Visible Context Before Every Question (MANDATORY)` + live-path citation |
| `docs/architecture.md` | Modify | ~15–25: enforcement claims name `ask-user-choice.ts → biggz sdd-ask-check`; no unwired-component claims |
| `internal/assets/biggz/orchestrator_test.go` | Modify | ~60–80: four-items assertion + `docs/architecture.md` claims test |

## Slice Split

| Slice | Files | Authored lines | PR boundary | Base |
|-------|-------|----------------|-------------|------|
| 1 — rule + CLI + Go tests | `question.go/_test.go`, `ask_check.go/_test.go`, `cli_sdd.go`, `main.go`, `cli_sdd_ask_check_test.go` | est. **380–470** | Rule + `biggz sdd-ask-check` fully testable end-to-end (`go test ./internal/sdd ./cmd/biggz`) | targets `main` |
| 2 — wiring + deploy + docs | `ask-user-choice.ts`, `biggz-ask-guard.js`, `biggz-ask-guard.test.mjs`, install trio, workflow md, `architecture.md`, `orchestrator_test.go` | est. **250–340** | Deployed asset refuses; deploy list synced; docs truthful | retargets slice 1 branch |

400-line risk: slice 1 **Medium-High** (upper bound ~470; if the tasks forecast exceeds ~400, split at rule/CLI seam: 1a `question.go`+tests / 1b `ask_check.go`+`cmd/`). Slice 2 **Low-Medium**. Total 630–810 → `stacked-to-main` chain confirmed, matches proposal. Decision needed before apply: No (chain already approved).

## Testing Strategy

| Layer | What | Where / how |
|-------|------|-------------|
| Unit (rule) | Predicate boundaries: all D1 fixtures, 24-rune floor edges, long-zero-classes, one-class, Spanish tokens, non-checkpoint unaffected | `internal/sdd/question_test.go` table test |
| Unit (pipeline) | Exit taxonomy, message naming option/question, ordering synthesis→envelope | `internal/sdd/ask_check_test.go` |
| CLI integration | stdin payload → exit codes; missing synthesis + valid envelope → 1; valid synthesis + thin option → 3; both valid → 0; malformed stdin → 4; message content | `cmd/biggz/cli_sdd_ask_check_test.go` (house `runX(args, stdin, stdout, stderr)` seam) |
| Asset (invocation proof) | `_setAskCheckExecForTest` mock: argv `["sdd-ask-check"]`, no `shell`, `timeout === 1000`, stdin JSON payload; exit 0 → undefined; exit 3 token → block + reason; ENOENT/timeout/help-text exit → degraded + warn; buffer records/resets; **source-scan**: `ask-user-choice.ts` imports `checkCheckpointAsk` and calls it before `ctx.ui.custom` | `internal/assets/pi/biggz-ask-guard.test.mjs` (`node --test`) |
| Docs/claims | Four checklist items in workflow md; `docs/architecture.md` contains `ask-user-choice.ts` + `sdd-ask-check` and no enforcement claim by unwired components; deploy list parity | `internal/assets/biggz/orchestrator_test.go`, install tests |

RED-first: substance table (fail case `"yes, go ahead"`), CLI exit table, and asset degrade tests are written before implementation (threat rows below map 1:1).

## Threat Matrix

| Boundary | Applicability | Design response | RED test |
|----------|---------------|-----------------|----------|
| Documentation-like paths | N/A — no path classification/execution of files | — | — |
| Git repository selection | N/A — no git invocation | — | — |
| Commit state | N/A — no index/worktree operations | — | — |
| Push state | N/A — no push automation | — | — |
| PR commands | N/A — no composed PR commands | — | — |
| Subprocess invocation (TS asset → CLI, the only applicable boundary) | **Applicable** | argv array, no shell; payload via stdin (no argument injection/quoting); 1000 ms timeout; missing/stale binary → degrade-open + visible notice, never block; only `blocked(` tokens block | `biggz-ask-guard.test.mjs`: injection-shaped payload reaches child verbatim on stdin · timeout → degraded · stale binary exit 1 + help text → degraded, never trap · (Go side) malformed stdin → exit 4, no `blocked(` token |

## Migration / Rollout

No schema or persistence change. Rollout = re-run `biggz install`: deploys `biggz-ask-guard.js` + updated `ask-user-choice.ts` (existing deploy mechanism + self-heal, no new step). Stale installed assets (old asset without the check) simply present asks as before until reinstall — enforcement is not retroactive in that window. No flag gate; `BIGGZ_ASK_CHECK=0` is the visible, warning-emitting escape hatch.

## Rollback

Revert slice commits independently: reverting slice 2 restores direct presentation (rule reverts to advisory); reverting slice 1 while slice 2 stays deployed makes `sdd-ask-check` exit 1 with help text — no `blocked(` token → guard degrades visibly and asks still present. No data migration to undo.

## Open Questions

- [ ] Should the four checklist items in `biggz-orchestrator-workflow.md` be asserted by exact-substring tests (brittle to rewording) or by four marker phrases chosen in tasks? Design assumes **four pinned marker phrases**; tasks must pin them.
- [ ] `BIGGZ_ASK_CHECK=0` is inferred from the proposal's "degrade-open seam" escape hatch; if unwanted, tasks drop it (1 line + 1 test).

## Budget Note

Measured 1,779 words excluding fenced code blocks (2,191 raw `wc -w`) vs the 800-word design budget (`budget_deviations` precedent): the mandated coverage (7 decisions, verbatim predicate + fixtures, exit taxonomy, TS seam, 17-row file table, slice table, 6-row threat matrix, rollback) cannot compress under 800 without deleting required sections. Coverage kept; prose compressed.
