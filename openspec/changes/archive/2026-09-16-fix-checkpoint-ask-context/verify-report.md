```yaml
schema: biggz-ai.verify-result/v1
evidence_revision: sha256:a63a797521d353a0eeffce8f97aef5d00e1b8462aa6d1cd457eff595eee1505a
verdict: pass
blockers: 0
critical_findings: 0
requirements: 3/3
scenarios: 29/29
test_command: go test ./internal/sdd ./cmd/biggz ./internal/install/... ./internal/assets/biggz/... -count=1
test_exit_code: 0
test_output_hash: sha256:a63a797521d353a0eeffce8f97aef5d00e1b8462aa6d1cd457eff595eee1505a
build_command: go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: fix-checkpoint-ask-context (combined candidate = PR #111 head `a007191a`; slices 1a/1b/2a/2b in #108/#109/#110/#111)
**Version**: delta specs — 3 MODIFIED requirements, 29 scenarios (sdd 15, orchestrator 7, pi-integration 7)
**Mode**: Standard (Strict TDD not active — no `STRICT TDD MODE` signal in the launch prompt; `strict-tdd-verify.md` not loaded)

### Completeness
| Metric | Value |
|--------|-------|
| Tasks total | 24 |
| Tasks complete | 24 |
| Tasks incomplete | 0 |
| Gates (phase 5) | 4/4 executed with recorded output (5.1 focused suites, 5.2 vet/fmt, 5.3 gitexec, 5.4 node pi suite) |

All 24 tasks carry per-task RED/GREEN evidence, including the closing whole-change re-run in Phase 5. No pending tasks.

### Build & Tests Execution
**Build**: ✅ Passed (`go build ./...`, exit 0, empty output — hash `sha256:e3b0c4…b785`)

**Tests (focused set, per tasks 5.1)**: ✅ exit 0

```text
go test ./internal/sdd ./cmd/biggz ./internal/install/... ./internal/assets/biggz/... -count=1
ok  internal/sdd          35.506s
ok  cmd/biggz             84.290s
ok  internal/install      13.778s
ok  internal/install/steps 6.932s
ok  internal/assets/biggz  1.403s
```

Output hash: `sha256:a63a797521d353a0eeffce8f97aef5d00e1b8462aa6d1cd457eff595eee1505a` (captured at `/tmp/verify-gotest.txt`).

**Tests (node pi suite)**: ✅ 132 passed / 0 failed / 0 skipped (16 suites, duration ≈1.92s): `node --test internal/assets/pi/*.test.mjs`, exit 0 — hash `sha256:fb41178363917c0b192fa2838c787ce00463d32be1e6f856ef11afb526e4e98c`.

**Targeted verbose re-runs**: ✅ `TestCheckpointOptionSubstance`, `TestValidateCheckpointSubstance`, `TestCheckCheckpointAsk`, `TestShouldBlock`, `TestShouldBlock_OptionBearingAndBypass`, `TestShouldBlock_SessionRecallAndChild`, `TestShouldBlock_TurnResetAndBodyThin`, `TestShouldBlockApplyAdmission_NoBypasses` — all PASS; `TestSddAskCheckExitTable` 7/7 subtests PASS.

**Full `go test ./...`**: NOT run — out of scope by delegation (CI is the full-suite evidence; see External State).

### Premise Check — the deployed asset really invokes the check
The change's premise ("a rule nobody invokes is not a rule") holds as implemented:

1. **Static import**: `ask-user-choice.ts:5` — `import { checkCheckpointAsk } from "./biggz-ask-guard.js";` (pinned by regex in `biggz-ask-guard.test.mjs` "statically imports checkCheckpointAsk from the shared guard module").
2. **Call before UI**: `ask-user-choice.ts:79` — `const verdict = await checkCheckpointAsk(params);` runs after the TUI-mode guard and before `ctx.ui.custom` (line ~102). Pinned by the source-scan `indexOf('checkCheckpointAsk(') < indexOf('ctx.ui.custom')` (test "calls the check BEFORE rendering any UI"). **Weakness noted**: this index assertion is textual and could in principle be satisfied by a comment mention of `checkCheckpointAsk(`; here it is corroborated by (a) the import regex, (b) my source inspection confirming the real call site at :79 precedes `ctx.ui.custom`, and (c) the behavioural suite. Residual brittleness recorded as SUGGESTION.
3. **Behavioural proof (not just existence)**: `biggz-ask-guard.test.mjs` mocks the exec seam and asserts the guard invokes `biggz[.exe] sdd-ask-check` via argv array, **no shell**, `timeout === ASK_CHECK_TIMEOUT_MS === 1000`, stdin payload `{"question","markdown"}` verbatim (including an injection-shaped payload); exit 3 `blocked(checkpoint_option_thin)` → `{block:true, reason}`; exit 1/2 tokens flow through with the check's message.
4. **Refuse vs degrade**: `verdict?.block` → `{content:[{type:"text",text:verdict.reason}], isError:true}` with no UI call (source-scan + guard tests); `verdict?.degraded` → `ctx.ui.notify(verdict.notice, "warning")` then present (source-scan "refuses to present on a decided block" + degrade table: ENOENT / ETIMEDOUT / stale-binary help-text exit 1 / foreign exit 42 / `BIGGZ_ASK_CHECK=0` → degraded + notice, never block).

### Spec Compliance Matrix
| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| sdd — Synthesis Gate Markers and 120s Window | HasSynthesis requires 4 markers plus table | `internal/sdd/synthesis_gate_test.go:31 TestHasSynthesis` | ✅ COMPLIANT |
| sdd | IsCheckpointAsk label-only not body | `synthesis_gate_test.go:77 TestIsCheckpointAskEnvelopeLabelsOnly` | ✅ COMPLIANT |
| sdd | HasOptions alone never blocks | `synthesis_gate_test.go:104 TestHasOptionsAdviseOnly` + `:152 TestShouldBlock_OptionBearingAndBypass` | ✅ COMPLIANT |
| sdd | Checkpoint detection and 120s window expiry | `synthesis_gate_test.go:119 TestShouldBlock` | ✅ COMPLIANT |
| sdd | Strict currentTurnMarkdown and turn_start reset | `synthesis_gate_test.go:206 TestShouldBlock_TurnResetAndBodyThin` + `biggz-ask-guard.test.mjs` "current-turn buffer" (reset on turn_start/agent_start) | ✅ COMPLIANT |
| sdd | Child, recall, and preflight bypass | `synthesis_gate_test.go:188 TestShouldBlock_SessionRecallAndChild` | ✅ COMPLIANT |
| sdd | Thin synthesis warn-only with advise | `synthesis_gate_test.go:104 TestHasOptionsAdviseOnly` | ✅ COMPLIANT |
| sdd | CheckSynthesisPrecondition message | `synthesis_gate_test.go:312 TestCheckSynthesisPrecondition_Message` + `cli_sdd_ask_check_test.go:38` subtest "missing synthesis blocks with pinned message" (exit 1, token `blocked(synthesis_required)`) | ✅ COMPLIANT |
| sdd | ApplyAdmission ignores bypass | `synthesis_gate_test.go:248 TestShouldBlockApplyAdmission_NoBypasses` | ✅ COMPLIANT |
| sdd | Thin option description rejected | `internal/sdd/question_test.go:110 TestValidateCheckpointSubstance` + `TestSddAskCheckExitTable` "thin option blocks naming option and question" (exit 3, names `"Yes, go ahead"` + question 1) | ✅ COMPLIANT |
| sdd | Context-bearing option description passes | `question_test.go:76 TestCheckpointOptionSubstance` (accept 24-rune/2-class, EN + ES) + `TestValidateCheckpointSubstance` context-bearing case | ✅ COMPLIANT |
| sdd | Non-checkpoint ask unaffected | `question_test.go:110` non-checkpoint case + `TestSddAskCheckExitTable` "non-checkpoint terse option passes" (exit 0) | ✅ COMPLIANT |
| sdd | Check balances both preconditions | `internal/sdd/ask_check_test.go:23 TestCheckCheckpointAsk` (8 subtests) + `TestSddAskCheckExitTable` (1/3/0 matrix) | ✅ COMPLIANT |
| sdd | Check invocation is bounded | `biggz-ask-guard.test.mjs` "invokes … bounded by ASK_CHECK_TIMEOUT_MS" (`timeout===1000`, no shell) + "timeout → degraded + notice, never block" | ✅ COMPLIANT |
| sdd | Indeterminate outcome never a silent pass | Degrade table (ENOENT, timeout, stale binary, foreign exit 42, `BIGGZ_ASK_CHECK=0`) all → degraded + visible notice; `cli_sdd_ask_check_test.go:113` exit-4 output must not contain a `blocked(` token (usage verified token-free) | ✅ COMPLIANT |
| orchestrator — REQ-ORCH-001 Blocking Synthesis Checkpoint (120s) | Synthesis within window allows | `synthesis_gate_test.go:119 TestShouldBlock` | ✅ COMPLIANT |
| orchestrator | Missing or expired blocks with fallback | `TestShouldBlock` + `:206 TestShouldBlock_TurnResetAndBodyThin`; deployed path refuses via guard block path (`biggz-ask-guard.test.mjs`) | ✅ COMPLIANT |
| orchestrator | Non-checkpoint never blocks | `synthesis_gate_test.go:152 TestShouldBlock_OptionBearingAndBypass` | ✅ COMPLIANT |
| orchestrator | Template markers and INVALID present in docs | `internal/assets/biggz/orchestrator_test.go:39 TestOrchestratorSynthesisTemplateInvariant` | ✅ COMPLIANT |
| orchestrator | REMINDER convergence and self-check | `orchestrator_test.go:186 TestOrchestratorSynthesisTemplateGuardsDrift` | ✅ COMPLIANT |
| orchestrator | Docs carry the four context items | `orchestrator_test.go:419 TestOrchestratorAskContextItemsInvariant` (4 pinned phrases verbatim inside `## Visible Context Before Every Question (MANDATORY)`; workflow md:155) | ✅ COMPLIANT |
| orchestrator | Enforcement claims match the live path | `orchestrator_test.go:448 TestArchitectureEnforcementClaimsMatchLivePath` (docs/architecture.md cites `ask-user-choice.ts`/`biggz-ask-guard.js`/`biggz sdd-ask-check`; every `biggz-synthesis-gate.js` sentence carries a deployment-status marker) | ✅ COMPLIANT |
| pi-integration — Question Envelope Validation | Header too long | `TestSddAskCheckExitTable` "header over limit names the 16 limit" + `question_test.go:9 TestValidate/header_17` + mjs exit-2 flow (limit 16) | ✅ COMPLIANT |
| pi-integration | Options range | `question_test.go:9 TestValidate/1_option` + mjs "pass does not skip envelope limits … 2-4" | ✅ COMPLIANT |
| pi-integration | Valid passes | `TestSddAskCheckExitTable` "both preconditions pass with silent stdout" + mjs "valid 3-question envelope allows" | ✅ COMPLIANT |
| pi-integration | Deployed asset invokes the check | mjs invocation test (argv array, no shell, 1000 ms, stdin JSON) + source-scan import/call-before-UI | ✅ COMPLIANT |
| pi-integration | Decided block refuses presentation | Source-scan "refuses to present on a decided block (`verdict?.block`, `isError:true`, reason)" + mjs exit-3 + `ask-user-choice.ts:79-85` inspection | ✅ COMPLIANT |
| pi-integration | Pass does not skip envelope limits | mjs "thin 1-option envelope is still rejected by the check" + Go exit table header/1-option blocks | ✅ COMPLIANT |
| pi-integration | Indeterminate check degrades visibly | Degrade table + source-scan `ctx.ui.notify(verdict.notice, "warning")` | ✅ COMPLIANT |

**Compliance summary**: 29/29 scenarios compliant (3/3 requirements).

### Correctness (Static Evidence)
| Requirement | Status | Notes |
|------------|--------|-------|
| sdd — substance rule | ✅ Implemented | `question.go`: `ErrThinCheckpointOption`, bilingual word-boundary class tables (scope/effort/risk/unlock/deferral), `CheckpointOptionSubstance` = runeLen ≥ 24 ∧ ≥2 classes, `ValidateCheckpointSubstance` wired into `ValidateQuestionEnvelope` scoped by `IsCheckpointEnvelope`. Boundary fixtures verified in the table incl. the 24-rune property assertion (`wantRunes`) and 23-rune sibling. Spanish fixture `"Revirte un commit; riesgo bajo; desbloquea slice 2."` accepted. |
| sdd — CLI check surface | ✅ Implemented | `ask_check.go`: ordered pipeline synthesis(1) → envelope(2|3) → allow(0); `cli_sdd.go:1692 runSddAskCheck` (stdin JSON, exit mapping, token-free usage on stderr); `main.go:87 case "sdd-ask-check"`. Exit 4 = indeterminate; usage text contains no `blocked(` token (asserted). |
| orchestrator — deployed path | ✅ Implemented | `ask-user-choice.ts` static import + execute-path check before `ctx.ui.custom`; block → `isError:true` without presenting; degraded → `ctx.ui.notify(..., "warning")` then present. `biggz-ask-guard.js`: argv execFile, no shell, `ASK_CHECK_TIMEOUT_MS=1000`, token-gated block regex, test seam, turn buffer with reset on `turn_start`/`agent_start`, no-op factory. |
| pi-integration — envelope limits | ✅ Intact | Header 16 / label 60 / questions 4 / options 2-4 unchanged (limits block in `question.go` untouched); ownership rule (`IsSubAgent` + checkpoint) unchanged; check pass does not bypass limits (check owns them; exit 2 naming limit 16). |
| Pre-existing gate contract | ✅ Intact | All existing synthesis-gate tests (`TestHasSynthesis`, `TestIsCheckpointAsk*`, `TestShouldBlock*`, `TestCheckSynthesisPrecondition_Message`, `TestShouldBlockApplyAdmission_NoBypasses`, `FormatFallback`, ownership) pass in the focused run. |
| Docs truth | ✅ Implemented | `biggz-orchestrator-workflow.md:155` carries the four pinned items + live-path citation; `docs/architecture.md:230/238` name `ask-user-choice.ts → biggz-ask-guard.js → biggz sdd-ask-check` and mark `biggz-synthesis-gate.js` as source-only/not deployed. |
| Deploy reality | ✅ Implemented | `internal/install/steps/pi_extensions.go:78` deploy entry `pi/biggz-ask-guard.js`; guard test 13 JS + 3 TS; factory `DEPLOY_LIST` 13→14 incl. new asset; `validatePiExtensionsFactory` passes (install + node factory suites green). |

**Modern Go check**: `use-modern-go` `list` consulted (via the skill's `scripts/run-tool.sh list --file-path internal/sdd/ask_check.go`). Surfaced rules (sync.WaitGroup→wg.Go, t.Context, omitzero, range-over-int, slices.Contains/SortFunc, cmp.Or, etc.) reviewed against the diff: none applies — no goroutines, no benchmark loops, no map iteration, no manual search loops, no manual min/max in the touched Go. No WARNING.

### Coherence (Design)
| Decision | Followed? | Notes |
|----------|-----------|-------|
| D1 substance = (a)∧(b) conjunction, bilingual | ✅ Yes | Exact threshold 24 / 2 classes; token tables match the design's examples; each conjunct kills one attack (filler; single buzzword). |
| D2 `biggz sdd-ask-check`, stdin JSON | ✅ Yes | `{"question","markdown"}` one-shot; house `runSddAskCheck(args, stdin, stdout, stderr)` shape. |
| D3 one-shot window seeding | ✅ Yes | `CheckCheckpointAsk` calls `SetCurrentTurnMarkdown(req.Markdown)`; live turn-reset semantics live in the TS buffer. |
| D4 token-gated block, else degrade | ✅ Yes | Guard blocks only on `blocked(synthesis_required|envelope_invalid|checkpoint_option_thin)`; ENOENT/timeout/stale/foreign/escape-hatch degrade with notice. Exit-4 message token-free. |
| D5 two substance call sites | ✅ Yes | `ValidateQuestionEnvelope` (label-signaled) + `CheckCheckpointAsk` when `IsCheckpointAsk(raw) && !IsCheckpointEnvelope(env)` (value-signaled); pinned by subtest "value-signaled checkpoint gets substance check". |
| D6 shared JS guard module + seam | ✅ Yes | `biggz-ask-guard.js` owns buffer + check; static import; `_setAskCheckExecForTest` mirrors session-guard precedent; no dependence on `biggz-synthesis-gate.js`. |
| D7 slice split rule/CLI/wiring/docs | ✅ Yes | #108 (155 lines), #109 (357), #110 (394), #111 (~96+2+6) — each ≤400 budget; 1c split not needed (357 ≤ 400). |
| Design Open Question 1 (pinned phrases) | ✅ Resolved | Four marker phrases pinned by `orchestrator_test.go:419`; live-path citation asserted separately. |
| Design Open Question 2 (`BIGGZ_ASK_CHECK=0`) | ✅ Resolved | Escape hatch kept, warns visibly, does not exec (guard tests). |

**Assumptions A1–A6**: no labeled A1–A6 list exists in design.md/proposal.md/tasks.md — the only reference is tasks 3.4's "(A4)" (no-op default export factory), which is implemented and tested ("factory survives hostile/absent pi without throwing"). Recorded as a SUGGESTION traceability nit, not a deviation.

**Declared deviation (declared, accepted)**: the design's literal fixture `"Edit 2 files; low risk!"` measures 23 runes, not 24; the fixture was adjusted to `"Edit 12 files; low risk!"` and the test pins the rune-length property (`wantRunes: 24`) rather than the characters — exactly the property the design's contract fixes. The 23-rune sibling remains `"Edit 2 files; low risk."` (`wantRunes: 23`). This meets the spec scenario "context-bearing description passes" (≥24 floor) and keeps the negative boundary at 23.

### External State (re-read at verify time)
| Candidate | Head | Checks (fresh `gh pr checks <n> --repo biggs-100/biggz-ai`) |
|-----------|------|------------------------------------------------------------|
| #108 | `d26d0d97` | 18/18 pass |
| #109 | `dc086fe5` | 18/18 pass |
| #110 | `7c9c3010` | 18/18 pass (previously 17 pass + 1 pending → now green) |
| #111 | `a007191a` | 18/18 pass — the `Test (windows-latest)` leg was still pending at report time after a bounded wait (treated as unverified, never green), and completed green in a final re-read at verify time; the other legs on the same head were green throughout (`Test (ubuntu-latest)` 2m8s, `Test (macos-latest)` 3m36s). |

Working tree: HEAD `a007191a` on `fix/ask-context-docs`; clean except untracked `openspec/changes/fix-checkpoint-ask-context/` (the SDD artifacts, expected).

### Ledger Binding
Orchestrated run: the delegation prompt supplied ledger token `tok-0c0de245c1f66b110bda7623`; the verifier did NOT acquire/settle (orchestrator settles after validating this report). Evidence digest bound to that token: `sha256:a63a797521d353a0eeffce8f97aef5d00e1b8462aa6d1cd457eff595eee1505a` (the focused-suite output captured at `/tmp/verify-gotest.txt`; equal to `test_output_hash` and `evidence_revision`).

### Issues Found
**CRITICAL**: None
**WARNING**: None — PR #111's last leg (`Test (windows-latest)`) was pending at report time after a bounded wait (recorded as unverified, never green) and completed green in the final re-read; all four PRs are now 18/18 green on their heads.
**SUGGESTION**: 2 — (1) the source-scan call-order assertion (`indexOf('checkCheckpointAsk(') < indexOf('ctx.ui.custom')`) is textual and could in principle be satisfied by a comment mention; it is corroborated by the import regex, the behavioural guard suite, and source inspection (call at `ask-user-choice.ts:79`, UI at :102), but a parse-based assertion would harden it. (2) The design references "A4" in tasks with no labeled A1–A6 assumption list in the artifacts; adding the list (or dropping the label) would restore traceability.

### Verdict
**PASS** — 24/24 tasks and 4/4 gates complete; all 29/29 scenarios covered by passing tests (focused Go suites exit 0, node pi suite 132/132, build exit 0); the premise is proven (deployed asset invokes `biggz sdd-ask-check` via argv + bounded timeout before any UI, blocks on decided violations, degrades visibly otherwise); CI is fully green on all four heads (18/18 each, including #111's windows leg on the final re-read); the two recorded SUGGESTIONs are hardening nits, not compliance gaps.
