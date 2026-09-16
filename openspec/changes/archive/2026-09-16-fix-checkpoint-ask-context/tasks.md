# Tasks: fix-checkpoint-ask-context

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 630–810 (design headline); 927–1,217 recomputed from the File Changes table |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1a → PR 1b → PR 2 (stacked-to-main) |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

Arithmetic: slice 1 (380–470, design) recomputed per file — 1a = `question.go` 70–90 + `question_test.go` 120–160 = **190–250**; 1b = `ask_check.go` 80–100 + `ask_check_test.go` 80–110 + `cli_sdd.go` 60–80 + `main.go` 4 + `cli_sdd_ask_check_test.go` 140–180 = **364–474**. Slice 1 midpoint 425 > 400 either way → **slice 1 MUST split into 1a + 1b** (design D7 authorizes the rule/CLI seam). 1b upper bound 474 > 400 → swing factor is the CLI exit-table test; if measured >400 at apply, carve the integration table into 1c. Slice 2 = 250–340 (design) / 373–493 (recomputed) → one PR; if measured >400 split wiring (JS+tests) from docs. `auto-chain` → no ask; proceed with 1a.

### Suggested Work Units

| # | Goal (start → finish) | PR | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1a | clean tree → predicate + sentinel + `ValidateCheckpointSubstance` wired into `ValidateQuestionEnvelope`; rule exists, no live caller yet | PR 1a | `go test ./internal/sdd -run 'CheckpointOptionSubstance\|ValidateCheckpointSubstance\|TestValidate' -count=1` | N/A — pure deterministic predicate, no process/IO surface; D1 fixtures run in-process | Revert `question.go` + `question_test.go`; no other reader of the predicate |
| 1b | [1a] → `biggz sdd-ask-check` end-to-end on stdin: exit 0/1/2/3/4, actionable messages | PR 1b | `go test ./internal/sdd -run TestCheckCheckpointAsk -count=1 && go test ./cmd/biggz -run SddAskCheck -count=1` | `printf '{"question":"…","markdown":"…"}' \| go run ./cmd/biggz sdd-ask-check; echo $?` → 0/1/3 matrix | Revert `ask_check.go`, `cli_sdd.go` hunk, `main.go` case; deploy untouched |
| 2 | [1b] → deployed asset refuses thin asks, degrades visibly, install ships the guard, docs truthful | PR 2 | `node --test internal/assets/pi/biggz-ask-guard.test.mjs internal/assets/pi/biggz-pi-extensions-factory.test.mjs && go test ./internal/install/... ./internal/assets/biggz/... -count=1` | `_setAskCheckExecForTest` mock (argv/stdin/timeout 1000); stale-binary path: `go run` with verb removed → exit 1 + help → degraded, present + warn | Revert TS/JS + install trio + docs; asks present directly again (rule advisory) |

## Phase 1: PR 1a — Substance predicate (RED first)

- [x] 1.1 RED `internal/sdd/question_test.go`: table `TestCheckpointOptionSubstance` with the D1 fixtures — REJECT `"yes, go ahead"` (0 classes); REJECT `"Sí, adelante"`; REJECT long-0-classes `"Adopt B — clearly better; we discussed it at length earlier"`; REJECT 1-class `"Run the tests and keep going as we agreed because it matters"`; PASS exactly-24-runes/2-classes (design literal `"Edit 2 files; low risk!"` — pin `runeLen==24`, adjust filler if the literal measures otherwise: the property is the contract, not the characters); REJECT 23-rune sibling `"Edit 2 files; low risk."`; PASS `"Revert one commit; unblocks slice 2."`; PASS Spanish `"Revirte un commit; riesgo bajo; desbloquea slice 2."`. Evidence: pre-implementation run fails.
  - Evidence RED-1 (tests first): `internal\sdd\question_test.go:99:20: undefined: CheckpointOptionSubstance` / `:123:22: undefined: ErrThinCheckpointOption` / `FAIL github.com/biggs-100/biggz-ai/internal/sdd [build failed]`.
  - Filler adjusted: design literal "Edit 2 files; low risk!" measures 23 runes; fixture pinned as "Edit 12 files; low risk!" = exactly 24 runes (test asserts `runeLen==24`). Sibling stays "Edit 2 files; low risk." (23).
- [x] 1.2 RED `TestValidateCheckpointSubstance`: checkpoint envelope + thin option → error wrapping sentinel, naming option + question; context-bearing envelope → nil; non-checkpoint envelope + thin option → nil. Evidence: fails until 1.3.
  - Evidence RED-2 (predicate present, not wired): `--- FAIL: TestValidateCheckpointSubstance/thin_option_rejected_wrapping_sentinel` / `question_test.go:121: expected thin checkpoint option to be rejected`.
- [x] 1.3 GREEN `internal/sdd/question.go`: `ErrThinCheckpointOption`; bilingual word-boundary class tables {scope, effort, risk, unlock, deferral}; `CheckpointOptionSubstance` = `runeLen(TrimSpace) ≥ 24 ∧ ≥2 classes`; `ValidateCheckpointSubstance(env)`; called from `ValidateQuestionEnvelope`.
  - Evidence GREEN: `go test ./internal/sdd -run 'TestCheckpointOptionSubstance|TestValidateCheckpointSubstance' -count=1` -> ok 0.124s.
- [x] 1.4 Regression: `go test ./internal/sdd -run 'TestValidate|Checkpoint' -count=1` → ok; ownership rule (`question.go:70` sub-agent + checkpoint) and limits untouched.
  - Evidence: pre-existing cases PASS (`TestValidate/header_17`, `label_61`, `5_questions`, `1_option`, `missing_description`) + new cases PASS; ownership line byte-identical (no pre-existing test pins it; full package green). Full package `go test ./internal/sdd -count=1` -> ok 26.066s.

## Phase 2: PR 1b — `biggz sdd-ask-check` CLI (RED first)

- [x] 2.1 RED `internal/sdd/ask_check_test.go`: unit table over `CheckCheckpointAsk` — ordering synthesis → envelope; codes 0/1/2/3/4; exit 1 message pinned `synthesis required: missing ## Sub-agent Result with 4 markers in current turn (120s window)`; exit 3 names option + question; exit 4 never prints a `blocked(` token.
  - Evidence RED: `internal\sdd\ask_check_test.go:29:12: undefined: AskCheckRequest` / `:38:14: undefined: AskCheckExitSynthesisRequired` / `:51:14: undefined: AskCheckExitThinOption` / `FAIL github.com/biggs-100/biggz-ai/internal/sdd [build failed]`. GREEN: `go test ./internal/sdd -run TestCheckCheckpointAsk -count=1 -v` -> PASS 8/8 subtests. Codes 0/1/2/3 unit-level; exit 4 is CLI-level (2.2).
- [x] 2.2 RED `cmd/biggz/cli_sdd_ask_check_test.go` on `runSddAskCheck(args, stdin, stdout, stderr)`: missing synthesis + valid envelope → 1; valid synthesis + thin option `"yes, go ahead"` → 3 naming it; both valid → 0 silent stdout; header 17 → 2 naming limit 16; `PI_SUBAGENT_CHILD=1` + checkpoint envelope → 2 naming ownership; malformed stdin → 4 (usage on stderr); non-checkpoint + terse option → 0.
  - Evidence RED: `cmd\biggz\cli_sdd_ask_check_test.go:34:10: undefined: runSddAskCheck` / `FAIL github.com/biggs-100/biggz-ai/cmd/biggz [build failed]`. GREEN: `go test ./cmd/biggz -run SddAskCheck -count=1 -v` -> PASS 7/7 subtests. First GREEN run caught a real hazard: the usage text listed `1 — blocked(synthesis_required)` etc., tripping the "exit 4 never prints a blocked( token" assertion; usage rewritten token-free.
- [x] 2.3 GREEN `internal/sdd/ask_check.go`: `AskCheckRequest{Question,Markdown}`, `AskCheckResult{Code,Message}`, `CheckCheckpointAsk` = `SetCurrentTurnMarkdown(md)` seed (D3) → `CheckSynthesisPrecondition` → `IsCheckpointAsk` (`value`/`id`/`name`/`title`) → `ValidateQuestionEnvelope` + substance; `errors.Is(ErrThinCheckpointOption)` → 3, else 2.
  - Evidence: 75 authored lines; exits `AskCheckExitAllow|SynthesisRequired|EnvelopeInvalid|ThinOption`; `askEnvelopeFailure` strips `isError:true` + `checkpoint_option_thin: ` prefixes; D5 second call site runs `ValidateCheckpointSubstance` when `IsCheckpointAsk(raw) && !IsCheckpointEnvelope(env)` (pinned by `value-signaled checkpoint gets substance check`).
- [x] 2.4 GREEN `cmd/biggz/cli_sdd.go` `runSddAskCheck` (stdin JSON, exit mapping, message rendering) + `cmd/biggz/main.go` `case "sdd-ask-check"`.
  - Evidence: 51 lines in `cli_sdd.go` + 2 in `main.go`; usage on stderr contains no `blocked(` token; malformed stdin → 4 with `error: cannot parse stdin payload` + usage.
- [x] 2.5 GREEN + manual matrix: focused tests ok; `printf '{"question":"…","markdown":"…"}' \| go run ./cmd/biggz sdd-ask-check; echo $?` → 0/1/3 observed.
  - Evidence: `go test ./internal/sdd -count=1` -> ok 29.036s; `go test ./cmd/biggz -count=1` -> ok 83.280s. Matrix on `go build -o /tmp/askcheck/biggz.exe ./cmd/biggz` (stdin payload files): ok.json -> exit 0 (silent); missing.json -> exit 1 `blocked(synthesis_required): synthesis required: missing ## Sub-agent Result with 4 markers in current turn (120s window); emit the block first, adjacent, same turn, then retry`; thin.json -> exit 3 `blocked(checkpoint_option_thin): option "Yes, go ahead" (question 1) carries no decision context ...`. No 1c split needed: 357 changed lines ≤ 400.

## Phase 3: PR 2 — Deployed asset invokes the check (RED first)

- [x] 3.1 RED `internal/assets/pi/biggz-ask-guard.test.mjs`: `_setAskCheckExecForTest` mock captures argv `["sdd-ask-check"]`, no shell, `timeout === 1000` (`ASK_CHECK_TIMEOUT_MS`), stdin JSON `{"question","markdown"}`; injection-shaped payload reaches the child verbatim (threat row); exit 0 → `undefined`; exit 3 `blocked(checkpoint_option_thin)` → `{block:true, reason}`.
  - Evidence RED: `node --test internal/assets/pi/biggz-ask-guard.test.mjs` -> `Error [ERR_MODULE_NOT_FOUND]: Cannot find module ...biggz-ask-guard.js imported from ...biggz-ask-guard.test.mjs` / 1 test, 1 fail. GREEN: 18 pass, 0 fail.
- [x] 3.2 RED degrade table: spawn ENOENT → degraded + notice; timeout → degraded (not a pass, not a hang); stale binary exit 1 + help text (no `blocked(` token) → degraded, never block; foreign non-zero → degraded; `BIGGZ_ASK_CHECK=0` → skipped + warning notice; buffer records `message_end`/`message_update`, resets `turn_start`/`agent_start`.
  - Evidence: 4-row degrade table + escape hatch + buffer via factory handlers all PASS in the same 18/18 run; ENOENT/timeout/stale/foreign never return `block`; escape hatch asserts `exec.calls.length === 0`.
- [x] 3.3 RED source-scan + asset behavior: `ask-user-choice.ts` imports `checkCheckpointAsk` and calls it before `ctx.ui.custom`; block → question NOT presented, `isError:true` reason surfaced; degraded → `ctx.ui.notify(notice,"warning")` then present; check exits 0 but header 17 → still rejected naming 16; 1 option → rejected; valid 3-question envelope → allowed.
  - Evidence: source-scan (import regex, call index < `ctx.ui.custom` index, `isError:true`, notify+warning) PASS; taxonomy flow-through tests for exit 2 naming `limit 16` and `2-4`, exit 0 allow, all PASS.
- [x] 3.4 GREEN `internal/assets/pi/biggz-ask-guard.js`: timeout const, `_setAskCheckExecForTest`, `recordAskTurnText`, `getAskTurnMarkdown`, `checkCheckpointAsk` (argv execFile, stdin, token-gated block), no-op default export factory (A4).
  - Evidence: 124 lines; token gate regex `blocked\((synthesis_required|envelope_invalid|checkpoint_option_thin)\)`; default factory `export default function biggzAskGuard(pi)` feeds the buffer and survives absent/hostile pi.
- [x] 3.5 GREEN `internal/assets/pi/ask-user-choice.ts`: static import; execute-path check before any UI; refusal `{content:[{type:"text",text:reason}], isError:true}`.
  - Evidence: +19 lines; check runs after TUI-mode guard and before `ctx.ui.custom`; degraded path notifies then presents.
- [x] 3.6 Deploy sync: `internal/install/steps/pi_extensions.go` entry `pi/biggz-ask-guard.js`; `pi_extensions_guard_test.go` (12→13 JS), `pi_extensions_drop_test.go`, factory `DEPLOY_LIST` (13→14). Evidence: `node --test internal/assets/pi/biggz-pi-extensions-factory.test.mjs` → ok; `go test ./internal/install/... -count=1` → ok.
  - Evidence: full `node --test internal/assets/pi/*.test.mjs` -> 133 pass, 0 fail; `go test ./internal/install/... -count=1` -> ok internal/install 9.854s + ok steps 4.933s. Drop test required no change (asserts names, not counts).
- [x] 3.7 GREEN: `node --test internal/assets/pi/biggz-ask-guard.test.mjs` → ok, 0 fail.
  - Evidence: pass 18, fail 0 (single suite; source-scan merged 2 tests into 1 to hold budget), duration_ms 109.

## Phase 4: PR 2 — Docs truth pass (test first) — LANDED IN SLICE 2b

The part-A diff to the branch point had reached exactly 400 changed lines (366 new JS + test + 34 tracked), the work-unit budget cap; per the budget rule the docs pass moved to slice 2b (`orchestrator_test.go`, the workflow doc, `docs/architecture.md`).

- [x] 4.1 RED `internal/assets/biggz/orchestrator_test.go`: assert the four pinned phrases — `evidence found and how it was verified` · `problem, scope, effort, risk, what it unlocks, and deferral cost` · `recommendation with its reason` · `no-go condition — research more instead of asking early` — and that `docs/architecture.md` contains `ask-user-choice.ts` + `sdd-ask-check` with no enforcement claim by unwired components or `biggz-synthesis-gate.js`. Pre-edit: fails.
  - Evidence RED: `go test ./internal/assets/biggz/... -count=1 -run 'TestOrchestratorAskContextItemsInvariant|TestArchitectureEnforcementClaimsMatchLivePath'` → FAIL, 8 errors: `orchestrator_test.go:430: ... missing pinned phrase "evidence found and how it was verified"` (×4 phrases), `:438` missing live-path citation ×3, `:454` docs missing deployed-path component ×3, `:476: biggz-synthesis-gate.js claim lacks deployment-status marker ... "`internal/assets/pi/biggz-synthesis-gate.js` mirrors Go strictly: ..."`. GREEN after 4.2+4.3: `go test ./internal/assets/biggz/... -count=1` → ok 0.631s.
- [x] 4.2 `internal/assets/biggz/biggz-orchestrator-workflow.md` `## Visible Context Before Every Question (MANDATORY)`: add issue #14's four items in the pinned phrasing + live-path citation (`ask-user-choice.ts → biggz sdd-ask-check`).
  - Evidence: +2 bullets inside the section: the four items verbatim (evidence/verification; per-option problem+scope+effort+risk+unlock+deferral; recommendation+reason; no-go condition) and the deployed path `ask-user-choice.ts` → `biggz-ask-guard.js` → `biggz sdd-ask-check` (argv array, no shell, 1000 ms bound, stdin JSON; decided block → not presented; indeterminate → present + visible warning; `biggz-synthesis-gate.js` NOT deployed, enforces nothing).
- [x] 4.3 `docs/architecture.md`: rewrite the enforcement claims (`:230`, `:238`) to name the deployed path — true or explicitly honest.
  - Evidence: `:230` rule core now "reached at ask time by the deployed path `ask-user-choice.ts` → `biggz-ask-guard.js` → `biggz sdd-ask-check` (argv array, no shell, 1000 ms bound, stdin JSON): a decided block refuses to present ... an indeterminate check ... degrades visibly with a warning"; `:238` Layer 2 heading retitled "Go canonical + deployed ask check (blocking) + retired Pi gate source (advisory only)"; the `biggz-synthesis-gate.js` "mirrors Go strictly" sentence now opens "(source-only, NOT deployed to `~/.pi/agent/extensions/` — see `pi-wrapper-removal` ...)"; Blocking bullet names the live path. Test scan: every sentence containing `biggz-synthesis-gate.js` carries a deployment-status marker.
- [x] 4.4 GREEN: `go test ./internal/assets/biggz -count=1` → ok (also re-runs preserved assertions: markers + `INVALID and will be blocked`, REMINDER ≥12, label re-read self-check).
  - Evidence: `ok github.com/biggs-100/biggz-ai/internal/assets/biggz 0.631s` — all pre-existing subtests (4 markers, 6 omit-empty sections, INVALID rule, REMINDER convergence, synthesis-separation, thin-orchestrator ≤120 lines, lazy routing, session recall, alias, recall discipline) PASS unchanged.

## Phase 5: Final gates

- [x] 5.1 `go test ./internal/sdd ./cmd/biggz ./internal/install/... ./internal/assets/biggz/... -count=1` → ok, 0 FAIL (re-pins existing gate tests: `TestHasSynthesis`, `TestIsCheckpointAskEnvelopeLabelsOnly`, `TestShouldBlock*`, `TestCheckSynthesisPrecondition_Message`, `TestShouldBlockApplyAdmission_NoBypasses`).
  - Slice 1a scope only: `go test ./internal/sdd -count=1` -> ok 26.066s (all `TestHasSynthesis`/`TestShouldBlock*`/`TestCheckSynthesisPrecondition_Message`/`TestShouldBlockApplyAdmission_NoBypasses` green). Remaining packages are slice 1b/2 surface — full command deferred to their PRs.
  - Slice 1b scope: `go test ./internal/sdd ./cmd/biggz -count=1` -> ok 29.036s / 83.280s, 0 FAIL. Remaining packages (`./internal/install/...`, `./internal/assets/biggz/...`) are slice 2 surface.
  - Slice 2 part A scope: same command -> ok sdd 35.394s, cmd/biggz 86.490s, install 13.376s, steps 6.227s, assets/biggz 1.274s, 0 FAIL. The docs-pass tests (4.1-4.4, slice 2b) are not yet in the tree; the full-change re-run belongs to the 2b PR.
- [x] 5.2 `go vet` on touched packages → clean; `gofmt -l` on touched Go files → no output.
  - Evidence: `go vet ./internal/sdd` clean (no output); `gofmt -l internal/sdd/question.go internal/sdd/question_test.go` -> no output.
  - Slice 1b: `go vet ./internal/sdd ./cmd/biggz` clean; `gofmt -l` on the 5 touched files -> no output.
  - Slice 2: `go vet ./internal/install/...` clean; `gofmt -l internal/install/steps/pi_extensions.go internal/install/steps/pi_extensions_guard_test.go` -> no output.
- [x] 5.3 `go build -o /tmp/gitexec.exe ./tools/gitexec/cmd/gitexec && /tmp/gitexec.exe -root .` → exit 0, `0 debt, 7 boundary`.
  - Evidence: `gitexec: OK — self-check 9 sites, 351 .go + 17 host targets, 0 debt, 7 boundary, baseline matched`, exit 0.
  - Slice 1b: same command -> exit 0, `gitexec: OK — self-check 9 sites, 352 .go + 17 host targets, 0 debt, 7 boundary, baseline matched`.
  - Slice 2: same command -> exit 0, `gitexec: OK — self-check 9 sites, 352 .go + 18 host targets, 0 debt, 7 boundary, baseline matched` (census +1 target from the new shipped JS asset).
- [x] 5.4 `node --test internal/assets/pi/*.test.mjs` → ok (guard, factory, session-stop unchanged).
  - Slice 2 scope: full pi suite -> 133 pass, 0 fail; `biggz-ask-guard.test.mjs` -> 18 pass, 0 fail; factory suite green with DEPLOY_LIST 12→13 JS (15→16 total).
  - Final (slice 2b, whole-change candidate): full pi suite -> tests 132, suites 16, pass 132, fail 0, cancelled 0, skipped 0, todo 0 (duration_ms 1877.643). Count delta vs the part-A run is reporter-side only; zero failures/skips both times, no pi file touched by 2b.
  - Closing 2b evidence (whole change): 5.1 `go test ./internal/sdd ./cmd/biggz ./internal/install/... ./internal/assets/biggz/... -count=1` -> ok sdd 38.375s, cmd/biggz 87.266s, install 14.007s, steps 6.168s, assets/biggz 0.772s, 0 FAIL. 5.2 `go vet ./internal/assets/biggz/...` clean; `gofmt -l internal/assets/biggz/orchestrator_test.go` -> no output. 5.3 -> exit 0, `gitexec: OK — self-check 9 sites, 352 .go + 18 host targets, 0 debt, 7 boundary, baseline matched`.

## Threat Matrix (verbatim from design)

| Boundary | Applicability | Design response | RED test |
|---|---|---|---|
| Documentation-like paths | N/A — no path classification/execution of files | — | — |
| Git repository selection | N/A — no git invocation | — | — |
| Commit state | N/A — no index/worktree operations | — | — |
| Push state | N/A — no push automation | — | — |
| PR commands | N/A — no composed PR commands | — | — |
| Subprocess invocation (TS asset → CLI, the only applicable boundary) | **Applicable** | argv array, no shell; payload via stdin (no argument injection/quoting); 1000 ms timeout; missing/stale binary → degrade-open + visible notice, never block; only `blocked(` tokens block | `biggz-ask-guard.test.mjs` (3.1/3.2): injection-shaped payload reaches child verbatim on stdin · timeout → degraded · stale binary exit 1 + help text → degraded, never trap · (Go side, 2.2) malformed stdin → exit 4, no `blocked(` token |

## Coverage (29 scenarios → proving tasks)

- sdd 1→5.1 (`TestHasSynthesis`); 2→5.1; 3→5.1; 4→5.1 (`TestShouldBlock`); 5→5.1 + 3.2 (buffer reset); 6→5.1; 7→5.1; 8→5.1 + 2.1/2.2 (exit 1 message); 9→5.1.
- sdd 10→1.1/1.2/1.3 + 2.2 (exit 3); 11→1.1/1.4; 12→1.2 + 2.2 (exit 0); 13→2.1/2.2/2.5; 14→3.1/3.2; 15→3.2/3.3.
- orchestrator 1→5.1; 2→5.1; 3→5.1; 4→4.4; 5→4.4; 6→4.1/4.2; 7→4.1/4.3.
- pi-integration 1→3.3; 2→3.3; 3→3.3; 4→3.1; 5→3.1/3.3; 6→3.3; 7→3.2/3.3.

Budget note (`budget_deviations` precedent, both prior changes): measured **1,565 words** (raw `wc -w`) vs the sdd-tasks 530-word MUST budget — in the precedents' own range (1,540 / 1,408). Arithmetic of mandated components: forecast table + 4 guard lines ≈ 150; 3-unit table with harness/rollback ≈ 200; 24 tasks × 1–2 evidence-bearing lines ≈ 600+; 6-row threat matrix ≈ 110; 29-scenario coverage map ≈ 120. Sum ≈ 1,180 minimum, >530 — coverage cannot shrink without deleting required components. Coverage kept; prose compressed.
