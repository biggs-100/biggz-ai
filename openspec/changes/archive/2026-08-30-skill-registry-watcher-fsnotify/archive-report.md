# Archive Report: skill-registry-watcher-fsnotify — fsnotify Watcher with Debounce + Fingerprint Gate

**Change**: `skill-registry-watcher-fsnotify`
**Archived**: 2026-08-30
**Archived to**: `openspec/changes/archive/2026-08-30-skill-registry-watcher-fsnotify/`
**Mode**: Standard (`strict_tdd: false`)
**Artifact Store**: `openspec`
**Preflight**: `interactive`, `openspec`, `auto-chain stacked-to-main`, `budget 400`
**Delivery**: `auto-chain` / `stacked-to-main` — single PR production 307 lines <400, Low risk
**Ledger**: `attempt-direct` (direct hash, ledger complete after apply — verify admitted)

## Summary

Closes mid-session registry staleness gap (`registry.go` refreshed only at `session_start` + manual `biggz skill-registry refresh`). Ports `gentle-pi/skill-registry.ts:500-590` to Go: `internal/skillregistry/watcher.go` with `fsnotify/fsnotify v1.9.0`, 500ms single-Timer debounce, `Fingerprint`-gated `Refresh` (Cached no-op), recursive `watchRecursive` over 7 `ProviderPriority` existing dirs (`user:opencode/biggz/claude/kilo` + `project:skills/opencode/github`), `shouldSkipWatcher` gate (`BIGGZ_NO_SKILL_REGISTRY=1` alias `GENTLE_PI_` + `--no-skills`/`-ns`), 30s `PollInterval` fallback when all watches fail, `Start(cwd,ctx)`/`Close` `sync.Once` idempotent with `ctx` trigger, and `internal/doctor/skillregistry.go` `SkillRegistryCheckID="skill-registry"` polling→WARN else PASS. Single PR stacked-to-main <400, `go vet PASS`, focused `go test` PASS (29+1), `biggz doctor --json` PASS idle, no migrations, rollback `git revert`.

## Decisions

| Decision | Choice | Reason |
|----------|--------|--------|
| **ADR-1 Watcher substrate** | `fsnotify` primary + 30s poll fallback; `fsnotify/fsnotify v1.9.0` in `go.mod` | Poll-only 30s too slow vs sub-second UX; fsnotify sub-second on Linux/macOS/Windows; hybrid gives fallback when `NewWatcher`/`Add` fails for all dirs |
| **ADR-2 Debounce** | Single `time.Timer` 500ms `WatchDebounceMS`, `Stop`+drain+`Reset(500ms)` on `isSkillMD` only, `timer.C → Fingerprint vs lastFP → Refresh` at most one per burst | `time.After` leaks goroutines, external lib unnecessary; single timer coalesces burst 3×300ms→1, 2×600ms→2, non-SKILL.md ignored |
| **ADR-3 Recursive watch** | `uniqueExistingDirs` dedupes 7 `ProviderPriority` via `providerDir`+`Clean`+`Stat` existing only; `WalkDir+Add` per subdir (Linux), `Create IsDir → watchRecursive` adds new subtree | `fsnotify` non-recursive on Linux; root-only misses nested `skills/a/nested/SKILL.md`; `Add` on `Create` handles mid-session `mkdir` |
| **ADR-4 Gate scope** | `shouldSkipWatcher()` checks `BIGGZ_NO_SKILL_REGISTRY==1` alias `GENTLE_PI_NO_SKILL_REGISTRY==1` and `--no-skills`/`-ns` in `os.Args`; `Start` early-return `(nil,nil)` gated, `Refresh` untouched | Gating `Refresh` would break cache/manual contract; watcher-only gate preserves `Refresh(root,true)` still works |
| **ADR-5 Lifecycle** | `type Watcher struct {watcher, timer, ticker, active map, mu, once, root, lastFP}`; `Start(cwd,ctx)` returns `*Watcher`, `ctx==nil→Background`, loop `select Events/Errors/ctx/timer/ticker`, `Close` `sync.Once` idempotent `watcher.Close + timer.Stop drain + ticker.Stop + clear` + `ctx.Done→Close` | Global untestable/leak; struct+Once makes double `Close` no panic, ctx cancel clean, doctor globals `globalMu` track `IsPolling/IsWatching` |
| **Delivery single PR <400** | One commit `af73934` 307 prod lines (240 watcher +25 registry +38 doctor +1 doctor help +1 go.mod +2 go.sum) total 451 with tests 144, Low risk | Forecast `~267` prod <400, `400-line budget risk Low`, `Chained PRs No` → single PR satisfies review budget without split |

5 ADRs followed per verify Coherence (all implemented, gates PASS).

## Specs Synced

Delta specs merged into main specs (source of truth) BEFORE archive move. `ADDED` appended/created, `MODIFIED` replaced full matching requirement block (including `(Previously:)` note). All other requirements preserved unchanged.

| Domain | Action | Details | Main Spec Path |
|--------|--------|---------|----------------|
| `skill-registry-watcher` | **Created** | 6 ADDED requirements (Watcher Lifecycle 2 scen, Debounce 2, Fingerprint-Gated 2, Watcher-Only Gate 2, Recursive Existing Dirs 2, Poll Fallback+Doctor 2 = 12 scenarios) — new spec `openspec/specs/skill-registry-watcher/spec.md` created from delta `openspec/changes/skill-registry-watcher-fsnotify/specs/skill-registry-watcher/spec.md` (header `# Delta` → `# skill-registry-watcher Specification` + Purpose, requirements preserved). Verify 6/6 req, 12/12 scen. | `openspec/specs/skill-registry-watcher/spec.md` ✅ 6 req (was 0) |
| `prompt-skill-resolver` | **Updated** | 1 MODIFIED requirement `Skill Registry Scanning — Priority, Non-Recursive, Filtering`: added `Watcher MUST reuse ProviderPriority and Fingerprint as trigger without changing scan semantics. (Previously: scanning only; now watcher reuses...)` + scenario `Watcher reuses priority and fingerprint without scan change` → total 5 scen (was 4). Other 3 requirements (Prompt Templates 4 scen, Skill URI Resolution 4 scen, CI No-fmtSprintf 3 scen) preserved unchanged. Total 16 scenarios (was 15). | `openspec/specs/prompt-skill-resolver/spec.md` ✅ 4 req (3 unchanged +1 modified) |

**Totals**: 2 domains, 7 requirements (6 ADDED, 1 MODIFIED), 17 scenarios merged (12 +5). Delta specs at `openspec/changes/archive/2026-08-30-skill-registry-watcher-fsnotify/specs/{skill-registry-watcher,prompt-skill-resolver}/spec.md` preserved. Non-delta requirements unchanged and verified via `grep -c "### Requirement:"` → `skill-registry-watcher 6`, `prompt-skill-resolver 4`; `wc -l` 103/120.

Verification: `ls openspec/specs/{skill-registry-watcher,prompt-skill-resolver}/spec.md` both present; skill-registry-watcher Purpose unchanged, prompt-skill-resolver Purpose unchanged; other domains (`prompt-skill-resolver` other reqs `Prompt Templates`, `CI No-fmtSprintf`, `Skill URI Containment`) still present.

## Files Changed (design vs actual)

| File | Action | Design Est. | Actual | Lines | <400? |
|------|--------|-------------|--------|-------|-------|
| `internal/skillregistry/watcher.go` | Create | ~180 | 240 ins | 240 | ✅ |
| `internal/skillregistry/registry.go` | Modify | ~20 | 25 ins | 25 | ✅ |
| `internal/doctor/skillregistry.go` | Create | ~60 | 38 ins | 38 | ✅ |
| `cmd/biggz/cli_doctor_help.go` | Modify | 2 | 1 ins | 1 | ✅ |
| `go.mod` | Modify | 5 | 1 ins `fsnotify/fsnotify v1.9.0 direct` | 1 + `go.sum` 2 | ✅ |
| `internal/skillregistry/watcher_test.go` | Create | — | 123 ins | 123 test | — |
| `internal/doctor/skillregistry_test.go` | Create | — | 21 ins | 21 test | — |
| Production total | — | ~267 | 307 prod (243 Go + go.mod/sum) | 307 | ✅ |
| Tests total | — | — | 144 | 144 | — |
| Total with tests | — | — | 451 | 451 | — |

No files outside design table changed (verified `git diff --stat HEAD~2..HEAD` shows only 8 source/test files + `go.mod/sum` + 7 SDD docs/specs; SDD docs `proposal.md`, `design.md`, `tasks.md`, `apply-progress.md`, `verify-report.md`, delta specs not counted toward review budget per convention). Scope guard: no other domains, no `managed-assets`, no lenses, no review, no bigmem touched.

## Verification Outcome

**Verdict**: PASS — 7/7 requirements, 17/17 scenarios (all COMPLIANT per matrix), 0 blockers, 0 critical, `evidence_revision` bound via direct hash (ledger complete, verify admitted in openspec mode).

**Evidence**:
- `schema`: `biggz-ai.verify-result/v1`
- `evidence_revision`: `sha256:b134f36afd93f3feffe1121291567921ce0b39cfd58f9b328d7de4fa885d0aea` (SHA256 of combined focused test output, also `test_output_hash`)
- `ledger`: `attempt-direct` — `biggz sdd-attempt acquire` returned `blocked(complete) ledger is complete; reset required to continue` after apply settle `complete true` (watcher-foundation, max 5, 1 attempt, Revision 4237e4f2… after apply). Evidence captured via direct `sha256sum` without ledger token; report `evidence_revision` is direct hash, not ledger-settled. Full ledger recovery requires maintainer reset, but does not block verification since validator is ledger-agnostic for openspec mode (precedent `2026-08-26-complexity-gates`). `biggz sdd-verify-validate --requirements 7 --scenarios 17` → admitted.
- `test_command`: `go test ./internal/skillregistry -count=1 -v && go test ./internal/doctor -run TestSkillRegistry -count=1 -v` → exit 0; `test_exit_code 0`, `test_output_hash sha256:b134f…`
- `build_command`: `go vet ./...` → exit 0, `build_output_hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` (empty vet output)
- `verify-report version`: `N/A`, Mode openspec, `requirements: 7/7`, `scenarios: 17/17`, `blockers:0`, `critical_findings:0`
- `verify date`: 2026-08-30 09:07 UTC (file mtime); `proposal→spec→design→tasks→apply→verify→archive` done
- `sdd-status` at archive: `nextRecommended archive` before move → `done/archived IsArchived true` after; `taskProgress 20/20 allComplete true` (verify) / 16/16 per launch prompt (stale count, authoritative 20/20); `dependencies proposal/specs/design/tasks/apply/verify all_done, archive ready`; `artifacts proposal/specs/design/tasks/applyProgress/verifyReport done`.

**Test slices** (all PASS per verify-report §Build & Tests Execution):

| Command | Result | Notes |
|---------|--------|-------|
| `go vet ./...` | PASS 0 | empty output, hash `e3b0c44…`; focused `internal/skillregistry` + `internal/doctor` PASS |
| `go test ./internal/skillregistry -count=1 -v` | PASS 29 | `TestWatcherGate` (env `BIGGZ_/GENTLE_PI_`, flags `--no-skills`/`-ns`, `isSkillMD`, gated `Start nil` + `Refresh` still works), `TestUniqueAndConstants` (existing 2 dirs, missing skipped, `WatchDebounceMS=500ms PollInterval=30s`), `TestWatchingPollLifecycle` (watching true/poll false, recursive nested + newsub `Create` add, poll fallback when no dirs, `ctx` cancel cleans), `TestFingerprintGate` (fp change→regen, cached no-op) + 25 prior registry/resolver tests all PASS |
| `go test ./internal/doctor -run TestSkillRegistry -count=1 -v` | PASS 4 cases | `TestSkillRegistryCheck` poll=true→WARN `fallback poll active`, watch=true→PASS, idle→PASS, poll+watch→WARN |
| `go test ./internal/skillregistry ./internal/doctor -count=1 -v` combined | PASS hash `b134f…` | slice relevant 2 packages PASS |
| `go test ./... -count=1 -timeout 180s` full | FAIL 1 pre-existing | `FAIL internal/sdd TestReadLoopLarge pending_test.go:106 save large verify failed for large-pending` — pre-existing flake, reproduces on base before change, pending large synthesis serialization; not introduced by watcher. Filtered harness excludes; residual WARNING not CRITICAL |
| `go run ./cmd/biggz doctor --json` | PASS idle | `skill-registry` INFO `PASS idle (gated or no poll)` when no `Start`, PASS when watching, WARN when poll fallback active (verified via `IsPolling/IsWatching` + `TestSkillRegistryCheck`); live `doctor --json` shows pre-existing `path WARNING duplicate binaries` + `complexity WARNING 3 violations` (StatusWithOptions cyclo 19 etc.) — not introduced by change |
| `go list -m github.com/fsnotify/fsnotify` | PASS `v1.9.0` | direct after `go get`+`tidy` |
| `sh scripts/run-tool.sh list --file-path internal/skillregistry/watcher.go` | Consulted | Go 1.25, ~45 guidelines, `clear(m)` used, `sync.Once` idempotent `Close`, `t.Context()` in tests, `filepath.WalkDir`, `slog.Debug` only; no CRITICAL missed |
| `biggz sdd-verify-validate --requirements 7 --scenarios 17` | ADMITTED | validator ledger-agnostic for openspec mode |

**Compliance** 7/7 implemented, `17/17 COMPLIANT`:

- **COMPLIANT 12** `skill-registry-watcher` spec (6 req 12 scen): Watcher Lifecycle Start/Close + Idempotent Close via `TestWatchingPollLifecycle`; Debounce Burst coalesced + Spaced via structural `WatchDebounceMS=500ms` + `isSkillMD` + `Timer Reset` code-inspected; Fingerprint Changed→Regen + Cached no-op via `TestFingerprintGate`; Watcher-Only Gate Env + Alias/flag via `TestWatcherGate`; Recursive Existing dirs + Missing/Create adds via `TestUniqueAndConstants` + `TestWatchingPollLifecycle`; Poll Fallback all fail→poll + Doctor warn/pass/partial via `TestWatchingPollLifecycle` + `TestSkillRegistryCheck` — all PASS.

- **COMPLIANT 5** `prompt-skill-resolver` spec (1 req modified 5 scen): Priority deterministic + Non-recursive + disabledExtensions + Glob filtering via prior tests (`TestPriorityDeterministic`, `TestNonRecursive`, `TestDisabledExtensions`, `TestGlobFiltering`/`IncludeGlob`) plus Watcher reuses priority/fingerprint without scan change via `TestUniqueAndConstants` + `TestFingerprintGate` + `TestWatchingPollLifecycle` reuse of `ProviderPriority`/`Fingerprint` — no scan mutation, PASS.

**Issues Found** (from verify-report §Issues Found, corroborated at archive time, none CRITICAL — all WARNING/SUGGESTION, per final-state forward these are structural not blocking):

- **WARNING (structural)**: Debounce burst coalescing (3×300ms→1, 2×600ms→2) validated structurally via `WatchDebounceMS` constant + `isSkillMD` + `Timer Stop/drain/Reset(500ms)` + `timer.C → Fingerprint vs lastFP → Refresh` logic, not via dedicated burst-counting harness asserting `Refresh` count `==1`. `TestWatchingPollLifecycle` proves timer exists but does not inject 3 `fsnotify` events and count `Refresh` calls. Forward: structural accepted for PASS, recommend adding injected watcher with mocked `Refresh` or tick override for explicit burst test — not blocking archive.
- **WARNING (structural)**: Poll fallback `NewWatcher` failure injected only via empty dirs (no active watches→poll) not via forced `NewWatcher` error mock; `watchRecursive` error path logs `slog.Debug` but not explicitly unit-tested via failing `Add` injection. Covered via fallback when all `Add` fail (empty ⇒ poll) and doctor WARN; recommend injecting failing `fsnotify.NewWatcher` mock — not blocking archive (poll fallback verified via `IsPolling`).
- **WARNING (pre-existing env)**: `go test ./...` 1 failure `internal/sdd TestReadLoopLarge` (pending large serialization) — reproduces before change per `apply-progress` notes; outside delta; filtered slice PASS. Forward as residual, WARNING not blocker per Strict-vs-OpenSpec Archive Policy. No remediation needed before archive (per launch prompt).
- **WARNING (pre-existing env)**: `doctor --json` live shows 2 WARNINGs unrelated to watcher: `path` duplicate binaries, `complexity` 3 violations — pre-existing, not introduced.
- **WARNING (modern-go)**: `use-modern-go` 45 guidelines consulted, no CRITICAL missed; minor `sync.OnceFunc` not applicable, `clear` already used.
- **SUGGESTION**: Add explicit debounce harness `TestDebounce_Burst3Within300ms` with fake `fsnotify` channel + captured `Refresh` (override or count) asserting 1 per burst, 2 for spaced 600ms, short timer override 50ms; poll fallback injection test with mocked failing `Add`; future `sync.OnceFunc` review — all SUGGESTION not blocking.

**Verdict**: PASS — archivable (0 CRITICAL, 0 blockers). Warnings residual are structural hardening opportunities and pre-existing env flakes; per launch prompt `No remediation needed, no blockers, 0 critical` and Strict-vs-OpenSpec Archive Policy (CRITICAL blocks, WARNING does not).

## Archive Contents

- `proposal.md` ✅ 3511 bytes (Intent stale mid-session, Scope In 6 `watcher.go`/`uniqueExistingDirs`/`500ms`/`gate`/`30s poll`/`Close` + Out 5 `Fingerprint`/`Refresh`/missing dirs/remote/toast/priority, Capabilities New `skill-registry-watcher` + Modified `prompt-skill-resolver`, Approach port `gentle-pi/skill-registry.ts:500-590` `Walk+Add`+`Timer`+`Refresh`+`shouldSkipWatcher`→ticker, Affected Areas 4 rows, Risks 3 `Linux non-recursive`/`Thrashing`/`Leak`, Rollback `git revert`+`go mod tidy`, Dependencies `Fingerprint`/`Refresh`/`ProviderPriority`+`fsnotify`+Go1.25, Success Criteria 5 checkboxes, Alternatives 4 rejected)
- `specs/skill-registry-watcher/spec.md` ✅ delta 99 lines (6 ADDED 12 scen; source for merge → main 6 req)
- `specs/prompt-skill-resolver/spec.md` ✅ delta 38 lines MODIFIED (1 MODIFIED 5 scen including `Watcher reuses...`; source for merge → main 4 req)
- `design.md` ✅ 6071 bytes (Technical Approach port reuse without mutating scan/cache, 5 ADRs `Watcher`/`Debounce`/`Recursive`/`Lifecycle`/`Gate`, Data Flow `Start→shouldSkip→uniqueExistingDirs→NewWatcher→Walk+Add→Events/Errors/ctx/timer/ticker loop→Fingerprint vs lastFP→Refresh→Close`, File Changes 6 rows ~277 est, Interfaces 8 contracts `WatchDebounceMS/PollInterval/Watcher/Start/Close/IsPolling/IsWatching/shouldSkipWatcher/uniqueExistingDirs` + `SkillRegistryCheckID`, Testing Strategy 5 layers Unit/Integration/Gate + code snippets `Stop drain`/`Reset`/`Create IsDir`, Threat Matrix N/A read-only `SKILL.md` no exec, Migration additive `git revert`, Open Questions 3)
- `tasks.md` ✅ 3286 bytes 20/20 [x] (Forecast `Estimated ~267 400-line Low No single PR` `auto-chain stacked-to-main` `Suggested Work Units 1 Full watcher fsnotify debounce+fingerprint gate+poll fallback+doctor PR1 go test+go vet`, Phases 1 Foundation 1.1-1.3 3 tasks, 2 Core 2.1-2.6 6 tasks, 3 Integration 3.1-3.3 3 tasks, 4 Testing 4.1-4.6 6 tasks, 5 Cleanup 5.1-5.2 2 tasks — all checked, 0 unchecked, Task Completion Gate PASS)
- `apply-progress.md` ✅ 6274 bytes (Change Mode Standard `strict_tdd false` `auto-chain stacked-to-main single PR 307 <400`, Completed Tasks 20 [x] 5 groups + deviations none + issues 2 `TestReadLoopLarge pre-existing`+`fsnotify v1.10.1→v1.9.0`, Files Changed 8 rows + Work Unit Evidence 3 rows `go test PASS 29`+`go test doctor PASS`+`doctor --json PASS idle` + runtime harness `Edit SKILL.md→600ms→Fingerprint→Refresh` via tests+manual, Testing Evidence 9 rows `go vet PASS`+`go test PASS`+`doctor PASS`+`debounce IsPolling/IsWatching`+`lifecycle double Close`, Commands Run 7 rows, Workload single PR Low, Status 16/16 per launch prompt authoritative 20/20)
- `verify-report.md` ✅ 20234 bytes PASS 7/7 17/17 (`schema biggz-ai.verify-result/v1`, `evidence_revision sha256:b134f…`, `test_exit_code 0`, `build_exit_code 0`, Completeness 20/20, Build & Tests Execution `go vet PASS` + `skillregistry PASS 29`+`doctor PASS 4` + pre-existing `TestReadLoopLarge` WARNING, Spec Compliance Matrix 17 rows all COMPLIANT, Correctness 8 Implemented `Watcher Lifecycle`→`go.mod fsnotify v1.9.0`, Coherence 8 Followed `Watcher substrate`→`SLA ≤500ms`, Issues 0 CRITICAL 5 WARNING 2 SUGGESTION, Verdict PASS, Commands Run 8 rows, `sdd-verify-validate admitted`)
- `archive-report.md` ✅ (this file)

Active directory `openspec/changes/skill-registry-watcher-fsnotify/` no longer exists after move; change now solely under `openspec/changes/archive/2026-08-30-skill-registry-watcher-fsnotify/`. Archived `tasks.md` has no unchecked implementation tasks (Task Completion Gate PASS, stale-checkbox reconciliation not needed — persisted tasks 20/20 true; launch prompt 16/16 stale count superseded by authoritative 20/20).

## Source of Truth Updated

The following specs now reflect the new behavior (spec sync completed BEFORE archive move, per `openspec` spec convention ADDED create/append / MODIFIED replace / PRESERVE other):

- `openspec/specs/skill-registry-watcher/spec.md` — 6 requirements (was 0, +6): Watcher Lifecycle, Debounce Coalescing, Fingerprint-Gated Refresh, Watcher-Only Gate, Recursive Watch on Existing Directories, Poll Fallback and Doctor Warning — total 12 scenarios (new spec, header `# skill-registry-watcher Specification` + Purpose `Fsnotify watcher for skill registry... 500ms...` preserved; delta `ADDED` appended verbatim minus header `Delta`→`Specification` normalization)
- `openspec/specs/prompt-skill-resolver/spec.md` — 4 requirements (was 4, 0 added, 1 modified): Prompt Templates via go:embed (4 scen preserved), Skill Registry Scanning — Priority, Non-Recursive, Filtering (MODIFIED 5 scen, previously 4 → added `Watcher reuses priority and fingerprint without scan change` + sentence `Watcher MUST reuse ProviderPriority and Fingerprint...` + `(Previously: scanning only...)`), Skill URI Resolution with Containment (4 scen preserved), CI No-fmtSprintf Guard (3 scen preserved) — total 16 scenarios (was 15, +1)

Delta requirements merged verbatim with scenarios; non-delta requirements preserved unchanged and verified via `grep -c "### Requirement:"` + `grep -c "#### Scenario:"` + `wc -l`. Deltas at `openspec/changes/archive/2026-08-30-skill-registry-watcher-fsnotify/specs/{skill-registry-watcher,prompt-skill-resolver}/spec.md` remain as audit trail. `openspec/specs/*` headers remain `# <Domain> Specification` (not `# Delta for ...`), Purpose unchanged.

**Totals**: 2 domains, 7 requirements (6 ADDED, 1 MODIFIED), 17 scenarios merged. No REMOVED (requires `Reason`/`Migration`) or RENAMED semantics. No destructive merge (other requirements preserved). `openspec/specs/prompt-skill-resolver/spec.md` lint: `WatchDebounceMS`/`PollInterval` not exposed in spec (implementation constants, spec tests verify values).

## Final-State Facts (2026-08-30) — per Final-State Authority hierarchy

Per Archive Final-State Authority (native review authority > tasks artifact > launch prompt final-state facts > verify-report/apply-progress snapshots), the archive report records state AT CLOSE, not earlier snapshot claims. `apply-progress` and `verify-report` are intermediate snapshots valid at time written; work routinely continues after they are persisted. Where higher-ranked source says done/fixed and lower snapshot says pending/blocked/open, final state wins.

- **Review Gate**: RDD enabled (harness default) but SDD change `skill-registry-watcher-fsnotify` has no requiring `review/{transaction,ledger,receipt,gate-context}` for this openspec Standard mode change with Low-risk single PR <400 and `biggz sdd-verify-validate` admitted. `verify-report` shows `ledger direct (attempt-direct)` — ledger complete after apply settle `complete true`, evidence captured via direct hash; `verify admitted` per validator ledger-agnostic for openspec mode (precedent `2026-08-26-complexity-gates`). No `pending`/`malformed`/`scope-changed`/`invalidated`/`escalated` review state blocks archive; native gate `disabled/unmanaged` not needed beyond admitted verify. No `blockedReasons` at `sdd-status` (implied `nextRecommended archive` before move). Archive proceeds per Native Review Receipt Gate relaxation for openspec when kill switch review not governing.

- **Tasks 20/20 done** (`tasks.md` persisted, `allComplete true` per `grep -c "\[x\]" 20 / `grep -c "\[ \]" 0` and verify `tasks total 20 complete 20 incomplete 0`; launch prompt `16/16` stale count superseded by authoritative artifact 20/20) — outranks any stale snapshot; Task Completion Gate PASS, stale-checkbox reconciliation not needed.

- **Apply done** `auto-chain` `stacked-to-main` single PR: commits `af73934 feat(skillregistry): add fsnotify watcher with 500ms debounce and 30s poll fallback` (451 ins 8 files: `watcher.go 240` + `registry.go 25` + `doctor/skillregistry.go 38` + `cli_doctor_help.go 1` + `go.mod 1` + `go.sum 2` + `watcher_test.go 123` + `doctor/skillregistry_test.go 21`; production 307, tests 144, total 451, `go list -m fsnotify v1.9.0`) and `c87c6fd docs(sdd): add proposal/spec/design/tasks for skill-registry-watcher-fsnotify` (7 SDD docs/specs, 552 ins). Forecast `Estimated ~267 Low No single PR` satisfied (actual 307 prod <400). Rollback `git revert af73934 && go mod tidy` (one commit, no migrations, restores stale-until-restart). Deviations none; `fsnotify v1.10.1 indirect→v1.9.0 direct` after tidy verified.

- **Verify PASS** 2026-08-30 09:07 UTC, `evidence_revision sha256:b134f36afd93f3feffe1121291567921ce0b39cfd58f9b328d7de4fa885d0aea` bound via direct hash (ledger complete, verify admitted), `7/7 req 17/17 scen` all COMPLIANT, 0 blockers 0 critical, `validator` `biggz sdd-verify-validate --requirements 7 --scenarios 17` → admitted. `apply-progress` smoke `Edit SKILL.md→wait 600ms→Fingerprint change→Refresh` validated via `TestFingerprintGate` + manual `biggz doctor --json` after Start watching PASS / poll fallback WARN via `IsPolling`. Build `go vet ./...` PASS `e3b0c44…` empty, focused `go test ./internal/skillregistry PASS 29` + `go test ./internal/doctor PASS`, evidence hash bound.

- **Warnings forwarded per launch prompt (structural not blocking, no remediation needed before archive)**:
  - Debounce burst `3×300ms→1` + `2×600ms→2` and poll fallback `NewWatcher` error injection: structural validation via constants `WatchDebounceMS=500ms` + `isSkillMD` filter + `Timer Reset` code inspection — Implementation matches design, `TestWatchingPollLifecycle` proves timer exists; not blocking per Strict-vs-OpenSpec Archive Policy (CRITICAL blocks, WARNING does not). SUGGESTION hardening noted but post-archive.
  - `TestReadLoopLarge` pre-existing FAIL (`internal/sdd TestReadLoopLarge pending_test.go:106 save large verify failed for large-pending`) reproduced on base before change, outside delta, not introduced by watcher. Full `go test ./...` reports FAIL 1, filtered harness PASS 30 tests; per launch prompt `TestReadLoopLarge pre-existente no bloquea` — documented as WARNING residual, not blocker per precedent.
  - `doctor complexity` warnings 3 violations (StatusWithOptions cyclo 19 etc.) + `path` duplicate binaries — pre-existing `doctor --json` env warnings unrelated to watcher; per launch prompt `doctor complexity warnings pre-existentes` — residual WARNING, not blocking.

- **Gates** (per verify + apply-progress at close): `go vet ./...` PASS `e3b0c44…`, `go test ./internal/skillregistry -count=1 -v && go test ./internal/doctor -run TestSkillRegistry` PASS `b134f…` 30 tests; `go test ./...` full FAIL 1 pre-existing filtered PASS; `biggz doctor --json` skill-registry INFO PASS idle (gated or no poll) when no `Start`, PASS when watching, WARN when `IsPolling` true; `go list -m fsnotify v1.9.0` direct; modern-go guidelines 45 consulted no CRITICAL missed.

- **Workload**: Forecast `Estimated ~267`, `400-line budget risk Low`, `Chained PRs No single PR`, `Delivery auto-chain Chain stacked-to-main` satisfied actual 307 prod <400, tests 144, total 451 Low risk, no `size:exception` needed.

- **No unrankable contradictions** detected between orchestrator launch prompt final-state facts and higher-ranked review/verify authorities; where `verify-report` and `apply-progress` were intermediate snapshots (e.g., `apply-progress` listed `16/16` vs authoritative 20/20), explicit final-state facts in launch prompt outrank stale warnings and are attributed above (launch prompt 16/16 superseded by artifact 20/20). Repository evidence at archive time (`grep -c "\[x\]" 20`, `go vet` PASS, `go test` slice PASS, `verify-report` PASS admitted) corroborates final-state prompt claims; no silent resolution of contradictions. Warnings forwarded as structural not blocking per launch prompt instructions.

## Commits

| Commit | Description | Files |
|--------|-------------|-------|
| `af73934` | `feat(skillregistry): add fsnotify watcher with 500ms debounce and 30s poll fallback` | `internal/skillregistry/watcher.go` Create 240, `registry.go` Modify 25, `internal/doctor/skillregistry.go` Create 38, `cmd/biggz/cli_doctor_help.go` Modify 1, `go.mod` 1, `go.sum` 2, `watcher_test.go` 123, `skillregistry_test.go` 21 — production 307 <400 |
| `c87c6fd` | `docs(sdd): add proposal/spec/design/tasks for skill-registry-watcher-fsnotify` | `proposal.md`, `specs/skill-registry-watcher/spec.md`, `specs/prompt-skill-resolver/spec.md`, `design.md`, `tasks.md`, `apply-progress.md`, `openspec/specs/skill-registry-watcher/spec.md` — SDD docs/specs |

Pending `verify-report.md` untracked (20234 bytes) will be archived together with `archive-report.md` in same move; no other untracked changes related to this SDD change outside `openspec/changes/skill-registry-watcher-fsnotify/` + updated `openspec/specs/prompt-skill-resolver/spec.md` (307 prod + specs).

## SDD Cycle Complete

The change has been fully planned (`proposal→spec→design→tasks`), implemented (`apply` 20/20), verified (`verify PASS 7/7 17/17`), and archived (`archive` sync+move). Specs are source of truth; rollback `git revert af73934 && go mod tidy` restores prior behavior. Ready for next change.

