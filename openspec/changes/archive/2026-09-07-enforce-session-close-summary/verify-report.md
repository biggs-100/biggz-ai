```yaml
schema: biggz-ai.verify-result/v1
evidence_revision: sha256:2e1c0ccffaa0c45ed68de476fb623ea7278a7f3e9fa5dc5b23c9fb5956650c41
verdict: pass_with_warnings
blockers: 0
critical_findings: 0
requirements: 8/8
scenarios: 21/21
test_command: go test ./cmd/biggz/ ./internal/sdd/ ./internal/extension/ -count=1 + node --test internal/assets/pi/*.test.mjs
test_exit_code: 0
test_output_hash: sha256:2e1c0ccffaa0c45ed68de476fb623ea7278a7f3e9fa5dc5b23c9fb5956650c41
build_command: go vet ./cmd/biggz/ ./internal/sdd/ + go build ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: enforce-session-close-summary
**Version**: N/A
**Mode**: Standard

### Completeness
| Metric | Value |
|--------|-------|
| Tasks total | 13 |
| Tasks complete | 13 |
| Tasks incomplete | 0 |

All checkboxes in `tasks.md` are `[x]` (Phase 1: 1.1-1.6, Phase 2: 2.1-2.5, Phase 3: 3.1-3.2).

### Build & Tests Execution
**Build**: ✅ Passed
```text
go vet ./cmd/biggz/ ./internal/sdd/
exit 0
empty output (hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855)

go build ./...
exit 0
empty output (hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855)

gofmt -l cmd/biggz/cli_session_close.go cmd/biggz/cli_session_close_test.go cmd/biggz/main.go cmd/biggz/cli_doctor_help.go
exit 0, no files listed (clean)

go build -o /tmp/biggz-verify.exe ./cmd/biggz
exit 0 (live binary for e2e checks below)
```

**Tests**: ✅ Passed (no failures)
```text
go test ./cmd/biggz/ -run TestSessionClose -count=1 -v
PASS 6/6: TestSessionCloseConflictingModeFlagsFail, TestSessionCloseForeignProjectAllows,
  TestSessionCloseJSONShape, TestSessionCloseFallbackNeverVerifies,
  TestSessionCloseTraversalChangeStaysAnchored, TestSessionCloseHelpDocumentsContract
ok github.com/biggs-100/biggz-ai/cmd/biggz 0.246s
exit 0

node --test internal/assets/pi/biggz-session-stop.test.mjs
PASS 10/10: Q2 timeout 1000ms, exit-0 allow, exit-1 block, stale degrade,
  pending-findings first, pending-lenses first, timeout degrade, ENOENT degrade,
  never-throws, both-file parity
exit 0

node --test internal/assets/pi/*.test.mjs
PASS 49/49 (6 suites, 0 fail) — includes session-stop 10/10 + footer/pills/synthesis-gate/anchors/provider
exit 0

go test ./cmd/biggz/ ./internal/sdd/ ./internal/extension/ -count=1
ok cmd/biggz 54.484s, ok internal/sdd 19.038s, ok internal/extension 0.132s
exit 0
```

**Live e2e (built binary /tmp/biggz-verify.exe)**:
```text
session-close --help → exit 0, lists --cwd/--json/--check-only/--save/--change + exits 0/1/2
session-close --check-only --save "x" → exit 2 (mutually exclusive, usage on stderr)
session-close --check-only --cwd /tmp/foreign-verify-test → exit 0
  "session-close: verified (project foreign-verify-test not gated, cwd ...)"
session-close --check-only --cwd /tmp/foreign-verify-test --json → exit 0
  {"verified":true,"reason":"","fallback":""}
/tmp/foreign-verify-test/openspec → absent (ls: cannot access; no fallback files written)
session-close --check-only --cwd C:/Users/USER/Desktop/biggz-ai --json → exit 0
  {"verified":true,"reason":"","fallback":""} (real BigMem summary verified)
unknownverb123 → exit 1 + Usage on stderr (router non-zero contract)
```

**Combined evidence**: `cat /tmp/v-go-focused.out /tmp/v-js-focused.out /tmp/v-js-full.out /tmp/v-vet.out /tmp/v-build.out /tmp/v-go-scoped.out /tmp/v-e2e-help.out /tmp/v-e2e-conflict.out /tmp/v-e2e-foreign.out /tmp/v-e2e-foreign-json.out /tmp/v-e2e-local.out /tmp/v-e2e-unknown.out > /tmp/verify-combined.out` → `sha256:2e1c0ccffaa0c45ed68de476fb623ea7278a7f3e9fa5dc5b23c9fb5956650c41` (109 lines). Ledger `sdd-attempt finish` settled COMPLETE with same `evidence_revision`; status `Complete: true`.

**Coverage**: ➖ Not measured (no coverage gate in tasks).

**Scope exclusion**: `internal/agents/pi/adapter.go` and `internal/assets/biggz/biggz-orchestrator-workflow.md` are modified in the worktree but belong to OTHER work — excluded from all evidence above and from the review subject. No commit was created.

### Spec Compliance Matrix
| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| CLI Verb Dispatch | Recognized verb | `cmd/biggz/main.go` `case "session-close"` + live `--help` exit 0 + live foreign `--check-only` exit 0 | ✅ COMPLIANT |
| CLI Verb Dispatch | Unknown first argument without --help | live `unknownverb123` exit 1 + Usage on stderr | ✅ COMPLIANT |
| CLI Session-Close Verb Contract | Verb routes to guard | live foreign allow + `TestSessionCloseForeignProjectAllows` | ✅ COMPLIANT |
| CLI Session-Close Verb Contract | Conflicting mode flags fail | `TestSessionCloseConflictingModeFlagsFail` + live `--check-only --save` exit 2 | ✅ COMPLIANT |
| CLI Session-Close Verb Contract | Help documents contract | `TestSessionCloseHelpDocumentsContract` + live `--help` lists flags + 0/1/2 | ✅ COMPLIANT |
| session-close Verify Summary Status | Summary present allows close | live biggz-ai `--json` exit 0 (real BigMem summary verified) | ✅ COMPLIANT |
| session-close Verify Summary Status | Summary absent blocks with reason | `TestSessionCloseJSONShape` exit 1 with reason+fallback keys | ✅ COMPLIANT |
| session-close Verify Summary Status | JSON output shape | `TestSessionCloseJSONShape` (verified/reason/fallback keys + bool) + live foreign `--json` | ✅ COMPLIANT |
| session-close Save Summary with Fallback | Save reaches BigMem | Guard delegation (`sessionCloseSave` → `SaveSessionSummaryWithFallbackForChange`); guard suite passes in scoped `internal/sdd` run | ✅ COMPLIANT |
| session-close Save Summary with Fallback | Persistent failure degrades without trapping | Guard degraded-file semantics via same delegation; CLI maps fallback path + `degraded` status | ✅ COMPLIANT |
| session-close Project Filter Scope | Foreign project always allows | `TestSessionCloseForeignProjectAllows` + live foreign no `openspec/` written | ✅ COMPLIANT |
| session-close Fallback File Never Satisfies Gate | Fallback alone still blocks | `TestSessionCloseFallbackNeverVerifies` (exit 1 despite file) | ✅ COMPLIANT |
| tool-interception Session-Stop Summary Verification | Verified summary allows stop | `biggz-session-stop.test.mjs > verified summary (CLI exit 0) allows close` | ✅ COMPLIANT |
| tool-interception Session-Stop Summary Verification | Missing summary blocks stop | `biggz-session-stop.test.mjs > missing summary (CLI exit 1) blocks with reason` | ✅ COMPLIANT |
| tool-interception Session-Stop Summary Verification | Stale binary never traps stop | `biggz-session-stop.test.mjs > stale binary degrades, never traps` | ✅ COMPLIANT |
| tool-interception Session-Stop Summary Verification | Pending work still blocks first | `biggz-session-stop.test.mjs > pending findings blocks first` + `pending lenses blocks first` (CLI not invoked) | ✅ COMPLIANT |
| tool-interception Session-Stop Summary Verification | CLI failure degrades without trapping | `biggz-session-stop.test.mjs > timeout degrades` + `crash (ENOENT) degrades` + `never throws` | ✅ COMPLIANT |
| extension-api Runner Wrapping | Runner delegates to PolicyInterceptor allow | Scoped `internal/extension` suite PASS (includes `TestRunner_BeforeAllow`, `TestRunner_ConsentAllow`) | ✅ COMPLIANT |
| extension-api Runner Wrapping | Runner blocks on consent deny | Scoped `internal/extension` suite PASS (includes `TestRunner_ConsentDeny`) | ✅ COMPLIANT |
| extension-api Runner Wrapping | Subagent child bypasses Runner | Scoped `internal/extension` suite PASS (includes `TestRunner_SubagentBypass`) | ✅ COMPLIANT |
| extension-api Runner Wrapping | Single session-stop guard | `biggz-session-stop.test.mjs > both files return identical verdicts` (extension-api delegates to `checkSessionStop`) | ✅ COMPLIANT |

**Compliance summary**: 21/21 scenarios compliant, 8/8 requirements compliant.

### Correctness (Static Evidence)
| Requirement | Status | Notes |
|------------|--------|-------|
| session-close thin wrapper (no duplicated BigMem logic) | ✅ Implemented | `cli_session_close.go` delegates via `sessionCloseVerify`/`sessionCloseSave` vars to `sdd.VerifySessionSummaryWithWorkspace` + `SaveSessionSummaryWithFallbackForChange`; `gofmt` clean, `go vet` clean |
| CLI exits 0/1/2 + `--change` default `session-close` | ✅ Implemented | `sessionCloseRun` manual flag parse; `--check-only==hasSave` → exit 2; foreign fast path via `project.DetectProjectFull`; `FallbackPath(change)` anchored (`../x` stays inside workspace per test) |
| JS `checkSessionStop` pending-first + argv array, no shell, 1000ms | ✅ Implemented | `biggz-tool-interception.js` exports `checkSessionStop` + `SESSION_STOP_TIMEOUT_MS=1000`; `execFileSync(bin, ["session-close","--check-only","--cwd",cwd], {timeout:1000, windowsHide:true})`; never throws (all failures → allow + `console.warn`) |
| Only gate token blocks; stale/timeout/crash degrade | ✅ Implemented | `catch` checks `err.status===1` + `/blocked\(session_summary_missing\)/` before returning `{block:true}`; else warn + allow |
| extension-api converges to single guard | ✅ Implemented | Static `import { checkSessionStop }` + `pi.on("session_stop", async () => checkSessionStop())`; inline duplicate deleted; one-way import, no cycle |
| Project filter biggz-ai-only; fallback never satisfies | ✅ Implemented | CLI foreign path returns verified without writes; check path uses `Verify` (not `IsBlocked`) so `session-fallback.md` alone still exits 1 |
| Guard untouched | ✅ Implemented | `git diff --name-only -- internal/sdd/session_guard.go` empty — reused as-is per design |

### Coherence (Design)
| Decision | Followed? | Notes |
|----------|-----------|-------|
| Use `Verify…` not `IsBlocked` (fallback must not satisfy) | ✅ Yes | `sessionCloseCheck` calls `sessionCloseVerify`; test `FallbackNeverVerifies` proves exit 1 despite file |
| Direct store + retry-once inside guard (no child re-exec) | ✅ Yes | `sessionCloseSaveRun` calls `sessionCloseSave` (guard handles retry + degraded file); CLI only maps `id`→fallback path |
| Exit 2 for usage vs 1 for blocked/degraded | ✅ Yes | Conflicting/missing flags → 2; blocked/degraded → 1; verified/saved → 0; `--help` → 0 |
| `--change` default `session-close` (no active-change derivation) | ✅ Yes | `sessionCloseDefaultChange` const; APPLY-DECIDE Q1 kept design default |
| `execFileSync` 1000ms argv array (no shell) | ✅ Yes | Measured 274–289ms cold-start win32; 250ms draft rejected (Q2 → 1000ms); test pins `SESSION_STOP_TIMEOUT_MS===1000` |
| Convergence via static ESM import (one-way, no cycle) | ✅ Yes | Precedent footer→extension-api; Q3 decided static import; parity test proves identical verdicts |

### Issues Found
**CRITICAL**: None

**WARNING**:
- Full `go test ./...` was NOT run; scoped equivalent (`./cmd/biggz/ ./internal/sdd/ ./internal/extension/` + full JS suite) was run and passes. Tasks 3.1 allows scoped equivalent. No regression signal; still recorded as WARNING per strict evidence rule.
- CLI `--save` success/degraded JSON mapping has no direct `cli_session_close_test.go` case; compliance rests on guard delegation (no logic to diverge) plus the scoped guard suite. Suggest adding `TestSessionCloseSave*` stub tests; not blocking (WARNING, not CRITICAL).
- Modern Go guidelines: `sh "skills/use-modern-go/scripts/run-tool.sh" list --file-path cmd/biggz/cli_session_close.go` was consulted (exit 0, full rule list reviewed). No `slices.*`/`maps.*`/`strings.*` modernization opportunity applies to the thin wrapper (manual flag loop is intentional, no manual search/sort loops). Recorded as consulted — no WARNING beyond this note.
- Ledger `sdd-attempt status` reports `Blocked reason: corrupt_authority / ledger is complete; reset required to continue` after `finish` (COMPLETE=true, Active attempt 0). Same provider quirk noted in prior `tool-interception` verify-report; does not affect test evidence. Tracked as WARNING, not CRITICAL.
- **Out-of-scope worktree changes excluded**: `internal/agents/pi/adapter.go`, `internal/assets/biggz/biggz-orchestrator-workflow.md` were NOT verified and MUST NOT enter the review subject.

**SUGGESTION**:
- Add CLI-level `--save` stub tests (verified + degraded JSON shapes) to close the indirect-coverage gap.
- Consider `strings.CutPrefix` for `--cwd=`/`--change=`/`--save=` parsing (currently `HasPrefix`+`TrimPrefix`; idiomatic but optional).

### Verdict
PASS WITH WARNINGS
All 13 tasks complete, 8 requirements and 21 scenarios compliant with passing runtime tests, design decisions followed, no critical findings. Warnings: scoped (not full) Go suite, indirect CLI save coverage, ledger-complete quirk. Out-of-scope files excluded; nothing committed.
