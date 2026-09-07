# Archive Report: enforce-session-close-summary

**Change**: `enforce-session-close-summary` → `2026-09-07-enforce-session-close-summary`
**Archived**: 2026-09-07
**Archived to**: `openspec/changes/archive/2026-09-07-enforce-session-close-summary/`
**Previous location**: `openspec/changes/enforce-session-close-summary/` (active)
**Mode**: `interactive`, `openspec`, `auto-chain`, `400 lines`, `stacked-to-main` (Cut 1 Go CLI → Cut 2 JS guard)
**Artifact Store**: `openspec` — `openspec/changes/enforce-session-close-summary` → `openspec/changes/archive/2026-09-07-enforce-session-close-summary/` + `openspec/specs/{cli,extension-api,tool-interception}/spec.md` source of truth
**Preflight**: `interactive` / `openspec` / `auto-chain` / `400` — chained PRs recommended Yes (Cut 1 before Cut 2), 400-line budget risk Low, delivery `auto-chain` / `stacked-to-main`
**Testing**: `go test ./cmd/biggz/ ./internal/sdd/ ./internal/extension/ -count=1` + `node --test internal/assets/pi/*.test.mjs` + `go vet` + `go build ./...` + live e2e via built binary

## Summary

Completed `enforce-session-close-summary` (biggz-ai issue #9) — the BigMem `session_summary` close protocol was unenforced outside the SDD `done`/`apply` gate, so agents closed without a verifiable summary and claimed inability despite the working bash fallback. The fix routes every close through the same guard pattern as SDD (`internal/sdd/session_guard.go` `HasSessionSummary` + `SaveSessionSummaryWithFallback` + `VerifySessionSummary`): a thin Go `session-close` CLI plus a Pi `session_stop` guard that shells to it, degrading to allow (never trapping) on timeout/crash/stale binary.

- **`cmd/biggz/cli_session_close.go` (204 lines, new)** — `sessionCloseRun()` manual flag parse (`--cwd/--json/--check-only/--save/--change/--help`), 10s ctx, delegates via `sessionCloseVerify`/`sessionCloseSave` vars to `sdd.VerifySessionSummaryWithWorkspace` + `SaveSessionSummaryWithFallbackForChange`, no duplicated BigMem logic. Exits 0 (verified/saved), 1 (`blocked(session_summary_missing)`/degraded), 2 (usage). Foreign project fast path via `project.DetectProjectFull` (allow, no writes). `FallbackPath(change)` anchored (`../x` stays inside workspace).
- **`cmd/biggz/cli_session_close_test.go` (229 lines, new)** — flag conflict (exit 2), foreign-project allow, JSON shape, fallback-never-verifies, traversal `--change ../x` anchored, help-documents-contract (6/6 `TestSessionClose*` PASS).
- **`cmd/biggz/main.go` (+2) + `cmd/biggz/cli_doctor_help.go` (+1)** — `case "session-close"` dispatch + one help line (flags + exits 0/1/2).
- **`internal/assets/pi/biggz-tool-interception.js` (+61)** — exports `checkSessionStop()`: pending-findings/lenses check first (no CLI call), then `execFileSync(bin, ["session-close","--check-only","--cwd",cwd], {timeout:1000, windowsHide:true})` argv array (no shell), never throws. Exit 0 → allow; exit 1 WITH `blocked(session_summary_missing)` token → `{block:true, reason}`; timeout/crash/exit-1-without-token (stale binary, help text) → allow + degraded `console.warn`. `SESSION_STOP_TIMEOUT_MS=1000` (measured 274–289ms cold-start win32 2026-09-07; 250ms draft rejected per APPLY-DECIDE Q2).
- **`internal/assets/pi/biggz-extension-api.js` (+14/−inline)** — duplicate `session_stop` handler deleted; static ESM `import { checkSessionStop }` + `pi.on("session_stop", async () => checkSessionStop())` (one-way, no cycle; precedent footer→extension-api per APPLY-DECIDE Q3).
- **`internal/assets/pi/biggz-session-stop.test.mjs` (194 lines, new)** — mocked `execFileSync`, mirrors `biggz-synthesis-gate.test.mjs`: exit-0 allow, exit-1 block, stale degrade, pending-findings first, pending-lenses first, timeout degrade, ENOENT degrade, never-throws, both-file parity (10/10 PASS; full JS suite 49/49 PASS).
- **`internal/sdd/session_guard.go` untouched** — `git diff --name-only` empty; reused as-is per design (no new BigMem logic).
- **SDD artifacts**: proposal (63 lines), specs (4 deltas: cli 50, extension-api 32, session-close 82, tool-interception 44), design (93 lines), tasks (46 lines, 13/13), verify-report (158 lines, PASS WITH WARNINGS, 8/8 req, 21/21 scenarios).

Shipped as single frozen review candidate (review tool binds one commit): commit `1041d5f6` on branch `sdd/enforce-session-close-summary` (Go CLI + JS guard + SDD docs, 17 files, +1346/−12). Canonical spec sync applied on top as worktree modifications (see Spec Sync). No push/merge; PRs are a later human decision.

## Validation

| Check | Result |
|-------|--------|
| Tasks completed | ✅ 13/13 marked `[x]` — `total:13 completed:13 pending:0 allComplete:true`, `dependencies.tasks: all_done`, `grep "^- \[ \]" 0`, `grep "^- \[x\]" 13` (Phase 1: 1.1–1.6, Phase 2: 2.1–2.5, Phase 3: 3.1–3.2) |
| Verify verdict | ✅ `PASS WITH WARNINGS` — `0 blockers`, `0 CRITICAL`, `8/8 requirements`, `21/21 scenarios` compliant (per `verify-report.md` `evidence_revision sha256:2e1c0ccffaa0c45ed68de476fb623ea7278a7f3e9fa5dc5b23c9fb5956650c41`) |
| Build | ✅ `go vet ./cmd/biggz/ ./internal/sdd/` exit 0 + `go build ./...` exit 0 (`build_output_hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` empty, 0 diagnostics) + `gofmt -l` clean |
| Tests | ✅ `go test ./cmd/biggz/ -run TestSessionClose` 6/6 PASS + `node --test biggz-session-stop.test.mjs` 10/10 PASS + `node --test *.test.mjs` 49/49 PASS (6 suites) + `go test ./cmd/biggz/ ./internal/sdd/ ./internal/extension/` PASS (`ok cmd/biggz 54.484s, ok internal/sdd 19.038s, ok internal/extension 0.132s`, `test_output_hash sha256:2e1c0ccf…`) + live e2e (`--help` exit 0, `--check-only --save` exit 2, foreign `--check-only` exit 0 no writes, biggz-ai `--json` exit 0 real summary, `unknownverb123` exit 1) |
| Coverage | ➖ Not measured (no coverage gate in tasks) |
| Evidence revision | `sha256:2e1c0ccffaa0c45ed68de476fb623ea7278a7f3e9fa5dc5b23c9fb5956650c41` (test_output_hash = combined 109-line evidence), `build_output_hash sha256:e3b0c44298fc…`, ledger `sdd-attempt finish` settled COMPLETE with same `evidence_revision`; status `Complete: true` |
| sdd-status pre-archive | ✅ `nextRecommended: archive`, `dependencies {proposal:all_done, specs:all_done, design:all_done, tasks:all_done, apply:all_done, verify:all_done, sync:all_done, archive:ready}`, `artifacts {proposal:done, specs:done, design:done, tasks:done, verifyReport:done, applyProgress:missing}`, `taskProgress {total:13 completed:13 pending:0 allComplete:true}`, `applyState: all_done`, `artifactStore: openspec`, `HasProposal:true HasSpecs:true HasDesign:true HasTasks:true HasVerify:true IsArchived:false`, `blocked: None` |
| sdd-status post-archive | ✅ (verified after move) `active` no longer lists `enforce-session-close-summary`; archived `2026-09-07-enforce-session-close-summary IsArchived:true` (see Archive Verification) |
| Review gate | ✅ Lineage `enforce-session-close-summary` finalized: 4 lenses captured (risk 0, resilience 0, readability 2 SUGGESTION, reliability 1 SUGGESTION); gate post-apply `allowed:true`, delivery `burned/unmanaged`. `biggz-ai` SDD `openspec` path emits no `reviewGate` in `sdd-status --json` (per Divergences, consistent with archived precedent `tool-interception`); pre-archive `nextRecommended: archive`, `dependencies.archive: ready` — gate PASS |
| Task gate | PASS — persisted `tasks.md` 13 `[x]`, 0 `[ ]` pre- and post-archive (`openspec/changes/archive/2026-09-07-enforce-session-close-summary/tasks.md` verified) |
| Apply state | `all_done` — `sdd-status` reports `applyState: all_done` even though `applyProgress` artifact `missing` (apply did not emit separate `apply-progress.md`; tasks carry completion evidence per dependency `apply: all_done` — same precedent as `tool-interception`/`tui-sanitize`) |
| CRITICAL gate | ✅ `verify-report.md` `critical_findings: 0`, `blockers: 0`, `verdict: pass_with_warnings` — no CRITICAL to block archive; no prompt override needed or accepted |

## Spec Compliance

**Verdict**: `PASS WITH WARNINGS` (per `verify-report.md` `evidence_revision sha256:2e1c0ccf…`, `test_exit_code 0`, `build_exit_code 0`)

| Metric | Value |
|--------|-------|
| Requirements | 8/8 compliant |
| Scenarios | 21/21 compliant (0 UNTESTED, 0 FAILING, 0 PARTIAL) |
| Tasks | 13/13 (Phase 1: 1.1–1.6, Phase 2: 2.1–2.5, Phase 3: 3.1–3.2) |
| Blockers / Critical | 0 / 0 |
| WARNING at verify time | 5 (scoped-not-full Go suite + indirect CLI `--save` coverage + modern-go consulted + ledger-complete quirk + out-of-scope worktree exclusions) — all non-blocking |
| SUGGESTION | 2 — CLI-level `--save` stub tests + `strings.CutPrefix` for `--cwd=`/`--change=`/`--save=` parsing |

**Detailed matrix** (from `verify-report.md` Spec Compliance Matrix — 21/21 COMPLIANT):

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

**Correctness & Coherence** (per verify-report `Correctness (Static Evidence)` + `Coherence (Design)` — all ✅ Implemented/Yes):

- Thin wrapper: `cli_session_close.go` delegates via `sessionCloseVerify`/`sessionCloseSave` vars; `gofmt`/`go vet` clean.
- Exits 0/1/2 + `--change` default `session-close`: manual flag parse; `--check-only==hasSave` → exit 2; foreign fast path; `FallbackPath(change)` anchored.
- JS `checkSessionStop` pending-first + argv array, 1000ms, never throws (all failures → allow + `console.warn`).
- Only gate token blocks; stale/timeout/crash degrade (checks `err.status===1` + `/blocked\(session_summary_missing\)/`).
- extension-api converges via static ESM import; one-way, no cycle.
- Project filter biggz-ai-only; fallback never satisfies (`Verify`, not `IsBlocked`).
- Guard untouched (`git diff --name-only -- internal/sdd/session_guard.go` empty).

## Spec Sync

Delta specs merged into main specs (source of truth) BEFORE archive move. In `openspec` mode `openspec/specs/` is the audit authority; filesystem wins on conflict. Sync was applied prior to archive (worktree modifications on top of frozen candidate `1041d5f6`); archive verified the diffs match delta intent and did not re-apply.

| Domain | Action | Details | Main Spec Path |
|--------|--------|---------|----------------|
| cli | **Updated** | 1 MODIFIED (Verb Dispatch: `session-close` appended to verb list + `(Previously: …)` note) + 1 ADDED (Session-Close Verb Contract: 3 scenarios). Diff `+29/−1` (428 lines total). Matches delta `specs/cli/spec.md` (MODIFIED + ADDED). Applied under allow-destructive per delivery scope. | `openspec/specs/cli/spec.md` ✅ |
| extension-api | **Updated** | 1 MODIFIED (Runner Wrapping: delegation sentence + `(Previously: …)` note) + 1 ADDED scenario (Single session-stop guard). Diff `+9/−1` (106 lines total). Matches delta `specs/extension-api/spec.md` (MODIFIED + ADDED scenario). Applied under allow-destructive per delivery scope. | `openspec/specs/extension-api/spec.md` ✅ |
| tool-interception | **Updated** | 1 ADDED (Session-Stop Summary Verification: 5 scenarios). Diff `+41` (180 lines total, plus blank-line normalization). Matches delta `specs/tool-interception/spec.md` (ADDED). Existing requirements preserved (BeforeToolCall, AfterToolCall, ApprovalMode, Session Stop Guard CanStopSession, FSM Authority, No God Object, Runner reuse). | `openspec/specs/tool-interception/spec.md` ✅ |
| session-close | **Deferred (NOT created)** | Canonical `openspec/specs/session-close/spec.md` does NOT exist (verified `ls: cannot access`). The delta `specs/session-close/spec.md` (82 lines, 4 requirements, 7 scenarios) has NO `## ADDED` header — it is a full spec (`# session-close Specification` + `## Requirements`), not a delta with `## ADDED/MODIFIED` sections. Creating the canonical file by adding that header would change reviewed content beyond what the frozen candidate `1041d5f6` and the 4-lens review approved. Per explicit scope decision, the file is intentionally NOT created during this cycle. **Follow-up**: create `openspec/specs/session-close/spec.md` as a new-domain full-spec copy in a later change, with its own review. | `openspec/specs/session-close/spec.md` ➖ deferred (see Risks / Open Questions) |

No REMOVED (requires Reason/Migration) or RENAMED in this change. Unrelated specs unchanged. Subsequent consumers read `cli` / `extension-api` / `tool-interception` from `openspec/specs/`; `session-close` consumers read the archived delta until the follow-up lands.

Verification: `git diff -- openspec/specs/cli|extension-api|tool-interception/spec.md` shows only the delta-intended hunks above (77 insertions, 2 deletions across 3 files); `ls openspec/specs/session-close/spec.md` absent confirmed.

## Implementation Traceability

Single frozen candidate `1041d5f6` (review tool binds one commit), two chained cuts inside per 400-line budget. Suggested Work Units table followed (Unit 1 Cut 1 Go CLI → Unit 2 Cut 2 JS guard; Phase 3 integration).

| Unit | Goal | Files (lines) | Focused test | Rollback boundary |
|------|------|---------------|--------------|-------------------|
| 1 | Cut 1: Go `session-close` CLI + tests | `cmd/biggz/cli_session_close.go` 204 + `cli_session_close_test.go` 229 + `main.go` +2 + `cli_doctor_help.go` +1 | `go test ./cmd/biggz/ -run TestSessionClose -count=1` 6/6 PASS | Revert verb case in `main.go`; CLI gone |
| 2 | Cut 2: JS `session_stop` guard + convergence | `biggz-tool-interception.js` +61 + `biggz-extension-api.js` ±14 + `biggz-session-stop.test.mjs` 194 | `node --test biggz-session-stop.test.mjs` 10/10 PASS; full JS 49/49 PASS | Restore allow-only `session_stop`; no CLI change |
| 3 | SDD artifacts | `proposal.md` 63 + `specs/*/spec.md` 208 + `design.md` 93 + `tasks.md` 46 + `verify-report.md` 158 + `exploration.md` 71 + `_meta.yaml` 14 | `go test ./cmd/biggz/ ./internal/sdd/ ./internal/extension/` + JS suite + live e2e | Revert commit `1041d5f6`; orphan `session-fallback.md` harmless |

| File | Action | Lines | Description |
|------|--------|-------|-------------|
| `cmd/biggz/cli_session_close.go` | Create | 204 | `sessionCloseRun()`; manual flag parse, 10s ctx, JSON/text output, exits 0/1/2 |
| `cmd/biggz/cli_session_close_test.go` | Create | 229 | Flag conflicts, foreign allow, JSON shape, fallback-never-verifies, traversal anchored, help contract |
| `cmd/biggz/main.go` | Modify | +2 | `case "session-close": os.Exit(sessionCloseRun())` |
| `cmd/biggz/cli_doctor_help.go` | Modify | +1 | One help line for `session-close` |
| `internal/assets/pi/biggz-tool-interception.js` | Modify | +61 | Export `checkSessionStop()`; pending-first → CLI verify → block/allow/degraded-warn |
| `internal/assets/pi/biggz-extension-api.js` | Modify | ±14 | Delete inline `session_stop` logic; delegate via static ESM import |
| `internal/assets/pi/biggz-session-stop.test.mjs` | Create | 194 | Verdict parity: allow/block/degraded + delegation identity |
| `internal/sdd/session_guard.go` | Reused | 0 | No changes; caller-side handles fallback-never-satisfies |
| `openspec/specs/cli/spec.md` | Sync | +29/−1 | Verb Dispatch modified + Session-Close Verb Contract added |
| `openspec/specs/extension-api/spec.md` | Sync | +9/−1 | Runner Wrapping modified + single-guard scenario added |
| `openspec/specs/tool-interception/spec.md` | Sync | +41 | Session-Stop Summary Verification added; others preserved |

**Tests isolation**: temp `--cwd` dirs for Go CLI (no store needed for unit paths); mocked `execFileSync` for JS; `t.Setenv`-style env isolation where applicable; live e2e via `/tmp/biggz-verify.exe` build.

## Final-State Authority & Reconciliation

`verify-report` and `apply-progress` are intermediate snapshots valid at their write time. Per archive contract hierarchy (native review authority > persisted tasks > explicit final-state facts > verify/apply snapshots), no higher-ranked source contradicts verify; tasks, status, review lineage, and repository evidence corroborate PASS WITH WARNINGS.

- **Post-snapshot work**: none that changes the verdict. Sync diffs (`cli`/`extension-api`/`tool-interception` canonical edits) were applied after the frozen candidate but before archive; they mirror the reviewed deltas verbatim (see Spec Sync) and required no re-verify (docs-only, no runtime change). Final numbers carried from `verify-report` (`8/8`, `21/21`, `test_output_hash sha256:2e1c0ccf…`, `build_output_hash sha256:e3b0c44298fc…`); no later test-count change reported.
- **Review lineage (highest authority for delivery facts)**: `enforce-session-close-summary` finalized, 4 lenses (risk 0, resilience 0, readability 2 SUGGESTION, reliability 1 SUGGESTION), gate post-apply `allowed:true`, delivery `burned/unmanaged`. Wins over any snapshot phrasing; consistent with `sdd-status` `archive:ready`.
- **Scoped-suite WARNING**: verify intentionally ran the scoped equivalent (`./cmd/biggz/ ./internal/sdd/ ./internal/extension/` + full JS) instead of full `go test ./...`, allowed by tasks 3.1. Still WARNING per strict evidence rule; not a contradiction.
- **Indirect `--save` coverage WARNING**: CLI `--save` success/degraded JSON mapping rests on guard delegation + scoped guard suite; no direct `TestSessionCloseSave*` case. Open SUGGESTION, non-blocking.
- **Ledger-complete quirk WARNING**: `sdd-attempt status` `corrupt_authority / ledger is complete; reset required` after `finish` (COMPLETE=true). Same provider quirk as prior `tool-interception` verify; does not affect test evidence.
- **Out-of-scope worktree changes**: `internal/agents/pi/adapter.go`, `internal/assets/biggz/biggz-orchestrator-workflow.md` were NOT verified, are NOT part of this change, and were NOT touched/staged/committed by archive (see Archive Verification). The untracked `openspec/changes/archive/sdd-parity-rescope-grant-ledger/.biggz-instance` likewise untouched. No contradiction — verify explicitly excluded them and archive preserved the exclusion.
- **applyProgress missing**: `sdd-status` reports `applyProgress: missing` yet `applyState: all_done` / `dependencies.apply: all_done`. Tasks `13/13 [x]` carry completion evidence (same precedent as `tool-interception`). Not a blocker.
- **Session-close canonical deferral**: not a snapshot staleness issue — an explicit scope decision (delta lacks `## ADDED`; creating canonical would change reviewed content). Recorded as follow-up, not a contradiction.

No unrankable contradiction between launch-prompt facts and repository evidence. All gates corroborated by native `sdd-status` authority plus file evidence.

## Archive Verification

Pre-archive (from `biggz sdd-status --json --instructions`):

- ✅ `nextRecommended: archive` (archivable)
- ✅ `verifyReport: done` (`artifacts.verifyReport: done`, `dependencies.verify: all_done`, `HasVerify: true`)
- ✅ `taskProgress: {total:13 completed:13 pending:0 allComplete:true}` (`dependencies.tasks: all_done`, 0 `[ ]`)
- ✅ `artifactStore: openspec` preserved
- ✅ `dependencies.sync: all_done`, `archive: ready`; `remediationState: {required:false}` — no remediation required
- ✅ `CRITICAL: 0`, `blockers: 0` — no archive block; no prompt override needed
- ✅ No `reviewGate` in `openspec` `sdd-status` — lineage + `archive:ready` govern (precedent-consistent)

Spec sync (BEFORE move — applied prior to archive, verified here):

- ✅ `openspec/specs/cli/spec.md` **Updated** (+29/−1) — delta hunks present, others preserved
- ✅ `openspec/specs/extension-api/spec.md` **Updated** (+9/−1) — delegation sentence + single-guard scenario present
- ✅ `openspec/specs/tool-interception/spec.md` **Updated** (+41) — Session-Stop Summary Verification present, others preserved
- ✅ `openspec/specs/session-close/spec.md` **intentionally absent** — deferred with reason (see Spec Sync); `ls` confirms

Archive move:

- ✅ `mv openspec/changes/enforce-session-close-summary → openspec/changes/archive/2026-09-07-enforce-session-close-summary` (`MOVE OK`, date prefix `2026-09-07` = today)
- ✅ Main specs still present after move (`cli` 428 lines, `extension-api` 106, `tool-interception` 180)
- ✅ Change folder moved to archive (`ls openspec/changes/enforce-session-close-summary` → absent; archive dir lists `_meta.yaml`, `proposal.md`, `design.md`, `exploration.md`, `tasks.md`, `verify-report.md`, `specs/{cli,extension-api,session-close,tool-interception}/spec.md`, plus this `archive-report.md`)
- ✅ Archive contains all artifacts (`proposal.md` 63 lines ✅, `specs/cli/spec.md` 50 ✅, `specs/extension-api/spec.md` 32 ✅, `specs/session-close/spec.md` 82 ✅, `specs/tool-interception/spec.md` 44 ✅, `design.md` 93 ✅, `tasks.md` 46 ✅ 13/13, `verify-report.md` 158 ✅, `_meta.yaml` 14 ✅, `exploration.md` 71 ✅, plus `archive-report.md` this file)
- ✅ Archived `tasks.md` has no unchecked implementation tasks (13 `[x]`, 0 `[ ]` — no reconciliation needed, no override)
- ✅ Active changes directory no longer has this change
- ✅ Unrelated work untouched: `git status` shows `M internal/agents/pi/adapter.go`, `M internal/assets/biggz/biggz-orchestrator-workflow.md`, `?? openspec/changes/archive/sdd-parity-rescope-grant-ledger/.biggz-instance` still present and UNSTAGED; archive stages/commits ONLY `openspec/changes/archive/2026-09-07-enforce-session-close-summary/` + `openspec/specs/{cli,extension-api,tool-interception}/spec.md` (see commit below)
- ✅ `openspec/changes/archive/` existed already, no create needed

Post-archive:

- ✅ `biggz sdd-status --json` no longer lists `enforce-session-close-summary` under `active`; archived `2026-09-07-enforce-session-close-summary IsArchived:true nextRecommended:done` (confirmed via rerun)
- ✅ Canonical specs remain source of truth (`cli`, `extension-api`, `tool-interception`); `session-close` canonical deferred as documented follow-up
- ✅ Commit per archive convention (rename + report + canonical sync in one commit, precedent `004d8f81`); never pushed/merged, no PRs (human decision)

## Risks / Open Questions

**Risks at close:**

- **SUGGESTION CLI `--save` stubs**: `--save` success/degraded JSON mapping has only indirect coverage (guard delegation). Future `TestSessionCloseSave*` stub tests would close the gap — non-blocking.
- **`strings.CutPrefix` idiom**: `--cwd=`/`--change=`/`--save=` parsing uses `HasPrefix`+`TrimPrefix`; `strings.CutPrefix` is idiomatic but optional — non-blocking.
- **Scoped-suite WARNING**: full `go test ./...` was not run; scoped equivalent passes with no regression signal. A future full-suite run would retire the WARNING.
- **Ledger provider quirk**: `corrupt_authority: ledger is complete; reset required` persists outside change (WARNING). Future `biggz sdd-attempt acquire` for a new change may need `reset` (maintainer scope, never automatic).
- **Stale-binary degrade is intentional**: pre-verb `biggz` binaries exit 1 with help text (no gate token) → JS allows + warns. Enforcement only strengthens as binaries roll forward; no trap risk.
- **No runtime harness ledger binding beyond hashes**: runtime evidence is via `go test` + `go vet` + live e2e hashes in verify, not separate ledger `acquire/settle` per task (precedent-consistent for this change shape).

**Open questions at close:** None for this change. Design open questions resolved in tasks (Q1 `--change` default kept, Q2 timeout 1000ms measured, Q3 static ESM import confirmed). One **follow-up** (not a question): create canonical `openspec/specs/session-close/spec.md` (new domain, full-spec copy of the 82-line archived delta + `## ADDED` header) in a later change with its own review — adding the header now would have changed reviewed content, so it was deferred.

## Traceability

- **Proposal**: `openspec/changes/archive/2026-09-07-enforce-session-close-summary/proposal.md` (63 lines, issue #9, Cut 1 + Cut 2 scope)
- **Specs (deltas)**: `specs/cli/spec.md` (50 lines) + `specs/extension-api/spec.md` (32) + `specs/session-close/spec.md` (82) + `specs/tool-interception/spec.md` (44) before move → now archived under `2026-09-07-enforce-session-close-summary/specs/` + synced to `openspec/specs/{cli,extension-api,tool-interception}/spec.md` (session-close deferred)
- **Design**: `openspec/changes/archive/2026-09-07-enforce-session-close-summary/design.md` (93 lines, two cuts, 6 architecture decisions, threat matrix, rollout)
- **Tasks**: `openspec/changes/archive/2026-09-07-enforce-session-close-summary/tasks.md` (46 lines, 3 phases, 13 tasks, `13/13 [x]`)
- **Verify**: `openspec/changes/archive/2026-09-07-enforce-session-close-summary/verify-report.md` (158 lines, `evidence_revision sha256:2e1c0ccffaa0c45ed68de476fb623ea7278a7f3e9fa5dc5b23c9fb5956650c41`, `verdict: pass_with_warnings`, `8/8 req`, `21/21 scenarios`, `0 blockers`, `0 critical`)
- **Exploration**: `openspec/changes/archive/2026-09-07-enforce-session-close-summary/exploration.md` (71 lines — Pi `session_stop` blocking hook found, contrary to triage assumption)
- **Apply**: frozen candidate `1041d5f6` (17 files, +1346/−12) — `cli_session_close.go` 204, `cli_session_close_test.go` 229, `main.go` +2, `cli_doctor_help.go` +1, `biggz-tool-interception.js` +61, `biggz-extension-api.js` ±14, `biggz-session-stop.test.mjs` 194; guard reused (0 diff)
- **Review**: lineage `enforce-session-close-summary` finalized (risk 0, resilience 0, readability 2 SUGGESTION, reliability 1 SUGGESTION; gate post-apply `allowed:true`, delivery `burned/unmanaged`)
- **sdd-status**: pre-archive `nextRecommended: archive`, `verifyReport: done`, `taskProgress {total:13 completed:13 pending:0 allComplete:true}`; post-archive `active` without the change, archived `2026-09-07-enforce-session-close-summary IsArchived:true nextRecommended:done`
- **Commit**: archive commit (move + report + canonical sync) on branch `sdd/enforce-session-close-summary`; frozen candidate `1041d5f6` preserved in history; unrelated files excluded; no push/merge/PR

## SDD Cycle Complete

The change has been fully planned, implemented, verified, and archived.

**Change**: `enforce-session-close-summary`
**Archived to**: `openspec/changes/archive/2026-09-07-enforce-session-close-summary/` (Engram N/A — `openspec` mode) | `openspec/specs/{cli,extension-api,tool-interception}/spec.md` source of truth (session-close deferred)

### Specs Synced
| Domain | Action | Details |
|--------|--------|---------|
| cli | Updated | 1 added, 1 modified, 0 removed requirements (+29/−1) |
| extension-api | Updated | 0 added requirements, 1 modified (+1 scenario), 0 removed (+9/−1) |
| tool-interception | Updated | 1 added, 0 modified, 0 removed requirements (+41) |
| session-close | Deferred | 0 added/modified/removed in canonical tree — `openspec/specs/session-close/spec.md` NOT created (delta lacks `## ADDED` header; follow-up in a later reviewed change) |

### Archive Contents
- proposal.md ✅ (63 lines)
- specs/cli/spec.md ✅ (50 lines, delta)
- specs/extension-api/spec.md ✅ (32 lines, delta)
- specs/session-close/spec.md ✅ (82 lines, delta — canonical deferred, see above)
- specs/tool-interception/spec.md ✅ (44 lines, delta)
- design.md ✅ (93 lines)
- tasks.md ✅ (13/13 tasks complete, 0 pending)
- verify-report.md ✅ (PASS WITH WARNINGS, 8/8 req, 21/21 scenarios, 0 blockers, 0 CRITICAL)
- exploration.md ✅ (71 lines)
- _meta.yaml ✅ (14 lines)
- archive-report.md ✅ (this file)

### Source of Truth Updated
The following specs now reflect the new behavior:
- `openspec/specs/cli/spec.md` — Verb Dispatch (+`session-close`) + Session-Close Verb Contract
- `openspec/specs/extension-api/spec.md` — Runner Wrapping (single session-stop guard delegation)
- `openspec/specs/tool-interception/spec.md` — Session-Stop Summary Verification (CLI-backed, degrade-never-traps)

Deferred (explicit follow-up, NOT created this cycle):
- `openspec/specs/session-close/spec.md` — new-domain full spec (82-line delta); needs `## ADDED` header + own review

### Next

Ready for the next change. `biggz sdd-status --json` shows the change archived (`IsArchived:true`, `nextRecommended:done`), delivery `interactive/openspec/auto-chain/400` preserved, no remediation required. PRs from branch `sdd/enforce-session-close-summary` are an explicit later human decision — nothing was pushed or merged.
