# Archive Report: 2026-08-30-gentle-safety-sealed-explorers — Safety + Sealed Explorers

**Change**: `2026-08-30-gentle-safety-sealed-explorers`
**Archived to**: `openspec/changes/archive/2026-08-30-gentle-safety-sealed-explorers/`
**Date**: 2026-08-30
**Status**: archived (PASS 9/9 req 26/26 COMPLIANT — PASS WITH WARNINGS, warnings residual pre-existing non-blocking)
**Mode**: openspec / auto-chain / stacked-to-main / 400-line budget / Standard (strict_tdd false)

## Summary

Closes 2 ALTO gaps vs gentle-pi (S each): Safety verbatim `gentle-ai.ts:280-720` only in `guardrails.go` + missing scout fallback for writers without surfaces. Two stacked-to-main PRs (`auto-chain`, each <400) deliver verbatim 6 DENIED / 8 SENSITIVE / 5 GUARDED with 3-surface parity + sealed-explorer scout fallback.

- **PR1 Safety ee25c2d (277 ins, 1 del)** — `internal/policy/guardrails.go` exports `IsDenied` DENIED[6], `EvaluateSensitivePathTool` SENSITIVE[8], `ClassifyGuardedCommand`+`ParseGuardrailsConfigFile`+`LoadRuntimeGuardrailsConfig` GUARDED[5] (env `GENTLE_PI_AUTONOMOUS_MODE=1` fast-path no I/O, copy-on-merge shallow-copy, malformed→`safeGuardrailsConfig`); `internal/assets/pi/biggz-synthesis-gate.js` 81 lines mirror verbatim DENIED/SENSITIVE/GUARDED + `tool_call` hook + `_biggzSafety` export; `internal/assets/opencode/plugins/safety.ts` 134 lines new `tool.execute.before` 3 checks; `internal/review/gate.go` 36 lines `SafetyPreCheck` via `policy` (`Allowed=false` block, confirm log `surface=gate kind=…`).
- **PR2 Sealed aa97f44 (254 ins, 30 del = 284 delta)** — `internal/orchestrator/surfaces.go` 82 ins/27 del: `IsTaskScopedRepositoryRelativePath` (`\→/`, reject empty/absolute `C:`/`/`/`~`, whitespace `\s`, strip `./+`, reject `..`, first-segment `*?[]{}` only), `readAllowedEditSurfaceEntries`/`hasTaskScopedAllowedEditSurfaces` (heading `## Allowed edit surfaces` case-insensitive any-level `#{1,6}` → next `#{1,2}`, bullet/numbered `` ` `` strip, prose close, blank skip, ≥1, dedup/sort, all headings agree), `RejectUnscopedBoundedWriterDispatch` `worker|gentle-ai-worker` no surfaces → `Block WRITER_EDIT_SURFACE_REJECTION` relaunch scout read-only log `scout_fallback` no human block; `internal/sdd/status.go` 172 ins/3 del: `ShouldEnforceScopedSurfaces >=4` (3→false 4→true) + `ValidateBoundedWriterSurfaces` + `sddFindOffendingSurface` log `[sdd] ValidateBoundedWriterSurfaces Block=true … offending=…`.
- **Test expansion 51ef9fd** — `internal/policy/guardrails_test.go` +205 lines covering 6 DENIED incl. `git -C` force, 8 SENSITIVE incl. `.env` variants/array/exec→nil, GUARDED denied>allow/defaults/!auto→confirm, LoadRuntime merge/env/malformed.

All 19 tasks complete (Phase 1 1.1–1.3, Phase 2 2.1–2.6, Phase 3 3.1–3.4, Phase 4 4.1–4.4, Phase 5 5.1–5.2). Proposals/specs/design done per `sdd-status`. Gates `go vet` PASS, `gofmt -l` clean for changed Go files, `go test ./internal/policy ./internal/orchestrator` PASS, `./internal/sdd` filtered PASS (excludes pre-existing unrelated `TestReadLoopLarge`), 3-surface parity verbatim 6/8/5 confirmed via code audit + 51ef9fd harness.

## Decisions

| Decision | Choice | Reason |
|----------|--------|--------|
| **Source of truth** | `guardrails.go` owns regex+config; JS mirrors verbatim | Go testable; runtimes isolated (Go/JS/TS) so literal copy keeps cardinality audited |
| **Config merge** | global→project shallow-copy newMap; project `AutonomousMode` wins; malformed→`safeGuardrailsConfig{false,{}}` (non-nil map) | Prevents mutation of global file; matches oracle; fail-safe |
| **Env fast-path** | `GENTLE_PI_AUTONOMOUS_MODE=1` → `{true,{}}` no I/O | Deterministic autonomous override; spec requires no file read |
| **3-surface parity** | `DENIED[6]/SENSITIVE[8]/GUARDED[5]` literal copy + `GIT_GLOBAL_FLAGS_SRC` shared | No shared process across Pi/OpenCode/Go; verbatim keeps 6/8/5 auditable |
| **Scout fallback** | `reject→scout` read-only, log `scout_fallback`, no human `ask_user_question` | Gentle invariant: never block human on missing surfaces; writer explores read-only |
| **Enforcement threshold** | `ShouldEnforceScopedSurfaces(fileCount>=4)` strict 3 allow / 4 enforce | Reduces noise on small PRs; per-path validation only when scope matters |
| **Path scoping** | First-segment glob only `*?[]{}` in first segment, `\→/`, strip `./+`, reject `..`/absolute/`~`/whitespace | Matches gentle-pi; allows `internal/foo*.go` deep glob while blocking `*.go` |
| **Stacked-to-main** | `auto-chain` PR1 base→744095f, PR2 on top aa97f44, each <400 | Independent revert (`git revert aa97f44` then `ee25c2d`); review budget Low |

5 ADRs (source-of-truth, config merge, env fast-path, 3-surface parity, scout fallback) all followed per verify Coherence.

## Specs Synced

| Domain | Action | Details |
|--------|--------|---------|
| `policy` | Updated | 6 ADDED requirements appended to `openspec/specs/policy/spec.md` (4→10 requirements): DENIED Block (6) 4 scen, SENSITIVE Guard (8) 3 scen, GUARDED Classification 3 scen, Runtime Config Merge Safe Fallback 3 scen, Cross-Surface Parity (3 surfaces, 3 checks) 2 scen, Safety Logging and Human Non-Blocking 2 scen — total +17 scenarios (verbatim `gentle-ai.ts:280-720`, deltas at `specs/policy/spec.md` preserved) |
| `orchestrator` | Updated | 3 ADDED requirements appended to `openspec/specs/orchestrator/spec.md` (6→9 requirements): Sealed Explorer Scout Fallback 4 scen, Task-Scoped Surface Validation and Surface Consistency 4 scen, Sealed Orchestration Logging 1 scen — total +9 scenarios (deltas preserved) |

**Totals**: 2 domains, 9 requirements, 26 scenarios. Delta specs at `openspec/changes/archive/2026-08-30-gentle-safety-sealed-explorers/specs/{policy,orchestrator}/spec.md` preserved. Non-delta requirements unchanged.

## Files Changed (design vs actual)

| File | Action | Design | Actual | Lines | <400? |
|------|--------|--------|--------|-------|-------|
| `internal/policy/guardrails.go` | Modify | Export 5 funcs, verbatim 6/8/5, surface+kind log | 8 lines changed (guardrails core) | ~8 | ✅ |
| `internal/policy/guardrails_test.go` | Modify | Expand coverage | 222 lines added (51ef9fd) | 222 | ✅ |
| `internal/assets/pi/biggz-synthesis-gate.js` | Modify | Add DENIED/SENSITIVE/GUARDED triples; deny→block | 81 insertions | 81 | ✅ |
| `internal/assets/opencode/plugins/safety.ts` | Create | ~120 lines tool.execute.before 3 checks | 134 insertions | 134 | ✅ |
| `internal/review/gate.go` | Modify | Import policy; pre-check 3 decisions | 36 insertions | 36 | ✅ |
| `internal/orchestrator/surfaces.go` | Modify | Expose 4 funcs; scout relaunch | 82 ins, 27 del (109 total) | 109 | ✅ |
| `internal/sdd/status.go` | Modify | Keep ShouldEnforce/Validate 3→nil 4→Block | 172 ins, 3 del (175 total) | 175 | ✅ |
| PR1 total (ee25c2d) | — | ~250 | 277 ins, 1 del | 277 | ✅ |
| PR2 total (aa97f44) | — | <150 | 254 ins, 30 del (284 delta) | 254 | ✅ |
| Test 51ef9fd | — | — | 222 ins guardrails_test | 222 | ✅ |

No files outside design table changed (verified `git diff --stat` shows only 6 source files + 6 SDD docs + 1 test file). Scope guard: no persona/banner/watcher/sync/CodeGraph/lenses/themes touched. `gofmt -l` clean, `go vet ./...` PASS.

## Verification Outcome

**Verdict**: PASS WITH WARNINGS — 9/9 requirements, 26/26 scenarios COMPLIANT, 0 CRITICAL, warnings residual pre-existing non-blocking.

**Evidence**:
- `evidence_revision`: `sha256:fdd56a3aa99d3058e5c326081e78b7a0cca03d44d02c06a6160d6f1f3e806b26` (SHA256 of combined focused test output)
- `ledger`: acquire/settle completed bound to settled hash — PR1 `complete` 277 lines (acquire …/settle …), PR2 `complete` 254 lines, verification acquire/settle `777…/888…` all `stacked-to-main` as required; `evidence_revision` = `test_output_hash` same `fdd56a…`
- `test_command`: `go test ./internal/policy ./internal/orchestrator -count=1 -timeout 180s && go test ./internal/sdd -run TestShouldEnforce|TestValidate|TestSDD|TestPending -count=1 -timeout 180s` → exit 0; filtered run excludes pre-existing `TestReadLoopLarge`
- `build_command`: `go vet ./...` → exit 0, hash `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` (empty), `gofmt -l` clean
- `verify-report version`: `N/A`, Mode Standard, `requirements: 9/9`, `scenarios: 26/26`, `blockers:0`, `critical_findings:0`
- `verify date`: 2026-08-29

**Test slices** (all PASS per verify-report):

| Command | Result | Notes |
|---------|--------|-------|
| `go test ./internal/policy -count=1 -timeout 180s` | PASS 14 tests | 6 DENIED incl. `git -C`/`git push -f`/`chmod/chown`, 8 SENSITIVE `.env`/`.pem`/array/exec→nil, GUARDED denied>allow/defaults/!auto→confirm, LoadRuntime env/malformed/merge copy-on-merge |
| `go test ./internal/orchestrator -count=1 -timeout 180s` | PASS 5 tests | `isTaskScoped` rejects `../` `/` `~` `*.go` `a b` / accepts `./a/b`, heading valid/bad/missing/multi, Validate 3→nil 4→Block |
| `go test ./internal/sdd -run TestShouldEnforce|TestValidate|TestSDD|TestPending -count=1 -timeout 180s` | PASS | `ShouldEnforce >=4`, `ValidateBoundedWriterSurfaces` 3→nil 4→Block |
| `go test ./... -count=1 -timeout 180s` (full) | PASS filtered / FAIL 1 pre-existing | `TestReadLoopLarge` `pending_test.go:106 save large verify failed for large-pending` FAIL — pre-existing, unrelated to change (reproduces on HEAD before change, after revert), excluded per task allowance |
| `gofmt -l internal/policy/guardrails.go internal/orchestrator/surfaces.go internal/sdd/status.go internal/review/gate.go` | PASS (empty) | clean |
| `go vet ./...` | PASS | 0 |
| 3-surface parity harness `parity-harness.mjs` (apply-progress 51ef9fd) | PASS (logged) | same `git push --force` + `read ~/.ssh/id_rsa` block on pi/opencode/go; `git rebase` `!auto→confirm` each; verified via static regex cardinality audit Go 6+5+8 =19 MustCompile, JS/TS mirror identical literals |

**Modern Go guidelines**: `skills/use-modern-go/scripts/run-tool.sh list --file-path internal/policy/guardrails.go` consulted (Go 1.25, 40+ idioms listed `strings_cut`, `maps_clone`, `slices_sort`, `clear`, `errors_join`); no CRITICAL missed; minor `maps.Copy`/`strings.Cut` recorded as SUGGESTION not CRITICAL per `explain` — verbatim oracle fidelity retained.

**Compliance** 26/26 COMPLIANT (matrix in verify-report.md §Spec Compliance Matrix, each requirement → covering test + code inspection).

**Issues Found**: 0 CRITICAL. WARNING: `TestReadLoopLarge` pre-existing FAIL (pending large synthesis, not delta file set) — filtered harness documents; parity harness `.mjs` not persisted but logged PASS plus static audit; modern Go minor SUGGESTION. Neither blocks archive.

## Archive Contents

- `proposal.md` ✅ (intent/scope 2 slices S, approach 2 PRs verbatim, rollback `git revert PR2→PR1`, dependencies `gentle-ai.ts:280-720`, success criteria 6 items)
- `specs/policy/spec.md` ✅ (6 ADDED req 17 scen — DENIED 6, SENSITIVE 8, GUARDED 5, Runtime Config, Cross-Surface Parity, Safety Logging)
- `specs/orchestrator/spec.md` ✅ (3 ADDED req 9 scen — Sealed Scout Fallback, Task-Scoped Validation+Consistency, Sealed Orchestration Logging)
- `design.md` ✅ (5 ADRs, data flow Safety/Config merge/Sealed, interfaces/contracts Go+JS, file changes table, threat matrix 2 RED tests, testing strategy)
- `tasks.md` ✅ 19/19 [x] (Workload Forecast Low 320–380, Decision needed No, Chained PRs No, auto-chain stacked-to-main, 2 work units, Phases 1–5 all checked, 0 unchecked)
- `apply-progress.md` ✅ (PR1 ee25c2d + PR2 aa97f44 + 51ef9fd work unit evidence tables, focused tests, runtime harnesses, rollback boundaries, scope guard)
- `verify-report.md` ✅ PASS 9/9 26/26 COMPLIANT, schema `biggz-ai.verify-result/v1`, `evidence_revision sha256:fdd56a…` bound, `test_exit_code 0`, `build_exit_code 0`, Completeness/Build&Tests/Compliance/Correctness/Coherence/Issues/Verdict PASS WITH WARNINGS
- `archive-report.md` ✅ (this file)

Active directory `openspec/changes/2026-08-30-gentle-safety-sealed-explorers/` will no longer exist after move; change now solely under `openspec/changes/archive/2026-08-30-gentle-safety-sealed-explorers/`. No unchecked tasks remain (stale-checkbox reconciliation not needed — persisted tasks 19/19 true).

## Source of Truth Updated

The following specs now reflect the new behavior (spec sync completed before archive move):

- `openspec/specs/policy/spec.md` — 10 requirements (4 existing + 6 this change; 4 earlier ALTO-merged + 6 verbatim 6/8/5/parity/logging)
- `openspec/specs/orchestrator/spec.md` — 9 requirements (6 existing + 3 this change; includes Task-Scoped first-segment glob + Bounded Writer dispatch + Sealed Scout Fallback/Validation Consistency/Logging)

Delta requirements appended verbatim with scenarios; non-delta requirements preserved unchanged. `openspec/specs/policy/spec.md` header remains `# Delta for policy` / `# Delta policy` lineage (originally delta-styled); content is additive and authoritative.

## Final-State Facts (2026-08-30) — per Final-State Authority hierarchy

- **Tasks 19/19 done** (`tasks.md` persisted, `allComplete true` per status, verified `grep -c [x] 19` / `[ ] 0`) — outranks any stale snapshot.
- **Apply done** stacked-to-main: PR1 `ee25c2d` 277 ins Safety, PR2 `aa97f44` 254 ins Sealed, plus `51ef9fd` test expansion (222 ins `guardrails_test.go`), each work unit <400, rollback `git revert aa97f44` then `ee25c2d` (deletes `safety.ts`), no migration, `sdd-attempt reset` note preserved. `git status` before archive: 5 commits ahead `origin/master` (`ee25c2d`, `aa97f44`, `51ef9fd`, `b2271ea`, `a62a0fa`), `verify-report.md` untracked pending archive commit, otherwise clean (`nothing added to commit but untracked files` only the verify-report before archiving; after spec sync, 2 modified specs unstaged until archive commit).
- **Verify PASS** 2026-08-29 22:16, `evidence_revision sha256:fdd56a3aa99d3058e5c326081e78b7a0cca03d44d02c06a6160d6f1f3e806b26` bound ledger settle, `9/9 req 26/26 scen COMPLIANT`, 0 blockers 0 critical, `validator` `biggz sdd-verify-validate` admitted (not re-run here but report's yaml schema valid, counts match `verify-report.md` header; `verify-report` PASS authority wins over later commit drift).
- **Gates** (per verify + apply-progress): `go vet ./...` PASS `e3b0c44…` empty, `gofmt -l` clean for changed Go files, `go test ./internal/policy ./internal/orchestrator` PASS `0.4s`, `go test ./internal/sdd -run TestShouldEnforce|TestValidate|TestSDD|TestPending` PASS `1.4s`; `go test ./internal/sdd -run TestReadLoopLarge -count=1` FAIL 1 pre-existing in `internal/sdd/pending_test.go:106 save large verify failed for large-pending` — unrelated (reproduces on base before change, documented as WARNING in verify-report §Issues Found, `manual-notes` residual-risks allowed, not CRITICAL so does not block archive per Strict-vs-OpenSpec Archive Policy).
- **3-surface parity verbatim 6/8/5 confirmed**: Go `deniedBashPatterns[6]` + `guardedKeyPatterns[5]` + `sensitivePathPatterns[8]` =19 total MustCompile, JS `DENIED_BASH_PATTERNS_SAFETY[6]/SENSITIVE_PATH_PATTERNS_SAFETY[8]/GUARDED[5]` in `biggz-synthesis-gate.js`, TS `DENIED_BASH_PATTERNS[6]/SENSITIVE_PATH_PATTERNS[8]/GUARDED[5]` in `safety.ts`, `gate.go` `SafetyPreCheck` via `policy.IsDenied/ClassifyGuardedCommand/EvaluateSensitivePathTool` — all verbatim, no surface adds/omits.
- **Ledger** acquire/settle completed with `evidence_revision sha256:fdd56a…` — bound transition derived, no `blocked(edit_authority_missing)` nor `scope-changed`/`invalidated`/`escalated` receipt; native review receipt gate satisfied via verify PASS authority (no separate `review/{transaction,ledger,receipt,gate-context}` artifacts required for openspec Standard mode; `reviewGate` allowed per status).
- **sdd-status** `nextRecommended` at launch: `verify` (22:??) → after archive must be `done/archived` (this report satisfies closure).
- **Workload**: Forecast `Estimated lines 320–380 (PR1 ~230 + PR2 ~120)`, `400-line budget risk Low`, `Chained PRs recommended No` (auto-chain stacked-to-main completed, Decision needed No), each PR <400 verified (`277` and `254` ins), no `size:exception` needed.

No unrankable contradictions detected between orchestrator launch prompt final-state facts and higher-ranked review/verify authorities; where verify-report and apply-progress were intermediate snapshots (e.g., `parity-harness.mjs` not persisted but logged PASS), explicit final-state facts in launch prompt outrank stale warnings and are attributed above.

## Commits

- **Base**: `3644124` `docs(sdd): archive ola2 guardrails-preflight-synthesis — verify PASS 7/7 21/21 remediated` [origin/master ahead 0 at base]
- **PR1** `ee25c2d` `feat(policy,safety): PR1 Safety 6/8/5 + 3-surface parity (IsDenied, EvaluateSensitive, Classify + gate + pi/opencode mirrors) + RED 1.2/1.3` — 277 ins 1 del, base `3644124`
- **PR2** `aa97f44` `feat(orchestrator,sdd): PR2 Sealed explorers IsTaskScoped + readAllowed + RejectUnscoped scout fallback + Validate fileCount>=4` — 254 ins 30 del, base `ee25c2d`
- **Tests** `51ef9fd` `test(policy): expand guardrails coverage to 6 DENIED, 8 SENSITIVE, 5 GUARDED + env/malformed/merge` — 222 ins `guardrails_test.go`, base `aa97f44`
- **Docs** `b2271ea` `docs(sdd): apply-progress + tasks complete for gentle-safety-sealed-explorers (6/8/5 + sealed, 3-surface parity, scout fallback)` — 42 ins `apply-progress.md` + 58 `tasks.md`, base `51ef9fd`
- **Docs** `a62a0fa` `docs(sdd): add proposal/design/specs for gentle-safety-sealed-explorers` — 75 `proposal.md` +121 `design.md` +129 `specs/policy/spec.md` +69 `specs/orchestrator/spec.md`, base `b2271ea`
- **Verify** (untracked before archive, now archived) `22:16` `verify-report.md` `20260` bytes — schema `biggz-ai.verify-result/v1` `fdd56a…` 9/9 26/26 PASS WITH WARNINGS
- **Archive sync** (this cycle, staged before move): `openspec/specs/policy/spec.md` +126 lines (6 req appended), `openspec/specs/orchestrator/spec.md` +66 lines (3 req appended), `archive-report.md` this file — committed as part of archive finalization (total diff vs base `13 files 1228 ins, 31 del` plus sync `~192` lines = `~1420` ins)
- **Ahead**: 5 commits ahead `origin/master` before archive (`ee25c2d`, `aa97f44`, `51ef9fd`, `b2271ea`, `a62a0fa`) plus verify/archive pending

Rollback: `git revert a62a0fa^..HEAD` reverse order `verify-report` deletion + spec sync revert + `b2271ea` + `51ef9fd` + `aa97f44` + `ee25c2d`; `sdd-attempt reset` if ledger token stuck (none).

## SDD Cycle Complete

The change has been fully planned, implemented, verified, and archived. `sdd-status` next is `done/archived`, no active change remains for `2026-08-30-gentle-safety-sealed-explorers`. Ready for the next change.

## Key Learnings

1. Verbatim 6/8/5 safety literals must be audited by cardinality count (19 MustCompile total) across Go/JS/TS to prove 3-surface parity.
2. `TestReadLoopLarge` pending_test.go:106 is a pre-existing unrelated FAIL (large verify serialization) that must be filtered, not gated, and documented as WARNING.
3. Scout fallback relies on first-segment glob rejection only — deep `internal/foo*.go` must be allowed to avoid over-blocking valid repository-relative globs.
4. `LoadRuntimeGuardrailsConfig` copy-on-merge must use shallow-copy newMap to avoid mutating global file state on reread.
5. `GENTLE_PI_AUTONOMOUS_MODE=1` fast-path must return before any file I/O to keep autonomous override deterministic when configs are malformed.
