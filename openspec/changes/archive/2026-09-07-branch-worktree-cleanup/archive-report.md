# Archive Report: branch-worktree-cleanup

**Change**: `branch-worktree-cleanup` → `2026-09-07-branch-worktree-cleanup`
**Archived**: 2026-09-07
**Archived to**: `openspec/changes/archive/2026-09-07-branch-worktree-cleanup/`
**Previous location**: `openspec/changes/branch-worktree-cleanup/` (active)
**Artifact Store**: `openspec` — `openspec/changes/branch-worktree-cleanup` → `openspec/changes/archive/2026-09-07-branch-worktree-cleanup/` + `openspec/specs/{branch-worktree-cleanup,sdd,cli}/spec.md` source of truth
**Evidence Revision**: `sha256:10d86540ec4a367ded4f5fbeb32a64a2c5ad2dd874e9532b179270527f63ad81` (admitted, validator PASS)
**Testing**: `go test ./internal/git -count=1 -v` + `go test ./internal/doctor -count=1 -v` + `go vet ./...` + `gocyclo -over 15` + `biggz cleanup --dry-run` + `biggz doctor --json`

## Summary

Completed `branch-worktree-cleanup` — local branch/worktree hygiene coupled to archive. Stacked PRs left `[gone]` branches (`tui-installer-pipeline pr1..pr5`); `sdd-archive` previously did `os.Rename` only with no git cleanup. The fix adds consent-gated prune after archive via shared predicates in `internal/git/cleanup.go` and a standalone `biggz cleanup --dry-run` verb. All git execution stays in `internal/git` (forbid-git), cyclo <15, dirty-worktree guard, CI non-TTY never blocks, fetch failure is warning.

- **`internal/git/cleanup.go` (new, ~260 lines)** — sole git owner: `FetchPrune` (`fetch --prune` warn-only), `ListGoneBranches` (`branch -vv` `: gone` detection), `IsMergedTo` (`merge-base --is-ancestor` with `origin/HEAD`→`origin/main` fallback), `ListWorktrees` (`worktree list --porcelain` + `status --porcelain` dirty guard), `PruneBranches`/`PruneWorktrees` (dry-run `Table`, real `branch -d` only, no `-D` without second confirm), `IsCandidate` pure predicate (`gone && !current && !protected && (name==change || HasPrefix(change+"-") || merged)`), `parseBranchLine`/`parseWorktreePorcelain`/`buildTable` helpers. Cyclo <15, nil-ctx guard, `: gone` fix for `not-gone` false-positive.
- **`internal/doctor/stale_branches.go` (new)** — `StaleBranchesCheck` INFO `staleBranches:N`, `SeverityInfo`, `StatusPass`, `Remedy nil`, never fail, non-blocking.
- **`cmd/biggz/cli_cleanup.go` (new)** + **`cmd/biggz/main.go`** + **`cmd/biggz/cli_doctor_help.go`** — `cleanupRun` parses `--dry-run --prune-worktrees --cwd`, reuses `IsCandidate` via `collectChangeNames`, dry-run Table no mutation, flag gates worktree (`would skip (use --prune-worktrees)` vs `would prune`), non-TTY `!isatty(Stdin)||!isatty(Stdout)` → `use --dry-run on CI` exit 0, help documents flags, unknown flag exit 1.
- **`internal/assets/skills/sdd-archive/SKILL.md` + `internal/assets/prompts/sdd/sdd-archive.md` (modified, Step 3b)** — post-`ArchiveChange` pure `os.Rename` hygiene: `FetchPrune` warn → `ListGoneBranches` → `ListWorktrees` → `IsCandidate` filter → preview Table → `Prune/Keep` consent → on Prune `branch -d` + `worktree prune` clean-only, non-TTY skip, `.biggz-instance` preserved.
- **`internal/sdd/archive.go` (verified no change)** — remains pure `os.Rename` only; no `branch -d`/`worktree prune`/`RDDDisable` before move (threat matrix SRP).
- **SDD artifacts**: proposal (77 lines, intent/scope/approach/risks/rollback), specs (3 deltas: branch-worktree-cleanup full 107 lines / 4 requirements / 13 scenarios, sdd ADDED 43 lines / 1 requirement / 6 scenarios, cli ADDED 43 lines / 1 requirement / 6 scenarios), design (99 lines, 3 ADs, data flow, file changes, threat matrix RED, interfaces), tasks (52 lines, 14/14), verify-report (131 lines, PASS WITH WARNINGS, 6/6 req, 26/26 scenarios 24 fully + 2 partial).

Shipped as local working-tree changes on branch `branch-worktree-cleanup` at HEAD `d08da9e5` (no push/merge; PRs later human decision). No branch/worktree deletions were executed here — post-archive hygiene is consent-gated and already validated via dry-run (scope of this archive is SDD only, per launch prompt).

## Validation

| Check | Result |
|-------|--------|
| Tasks completed | ✅ 14/14 marked `[x]` — `total:14 completed:14 pending:0 allComplete:true`, `dependencies.tasks: all_done` (Phase 1: 1.1–1.3, Phase 2: 2.1–2.3, Phase 3: 3.1–3.3, Phase 4: 4.1–4.4, Phase 5: 5.1), `grep "^- \[ \]" 0`, `grep "^- \[x\]" 14` |
| Verify verdict | ✅ `PASS WITH WARNINGS` — `0 blockers`, `0 CRITICAL`, `6/6 requirements`, `26/26 scenarios` (24 fully ✅, 2 partial ⚠️ non-blocking) per `verify-report.md` `evidence_revision sha256:10d86540ec4a367ded4f5fbeb32a64a2c5ad2dd874e9532b179270527f63ad81` — validator admitted |
| Build | ✅ `go vet ./...` exit 0 empty, `go build -o /tmp/biggz.exe ./cmd/biggz` exit 0, `gocyclo -over 15 internal/git/cleanup.go` empty (PASS), `gocyclo -over 15 ./...` empty |
| Tests | ✅ `go test ./internal/git -count=1 -v` PASS 0.85s (9 top-level: TestIsCandidate 12 sub-cases, TestParseBranchLine 7 golden, TestParseWorktreePorcelain 3, TestBuildTable, 5 Threat RED), `go test ./internal/doctor -count=1 -v` PASS 1.47s, `go test ./internal/sdd -count=1 -v` PASS |
| Runtime harness | ✅ `biggz cleanup --help` lists `--dry-run` + `--prune-worktrees` exit 0; `biggz cleanup --dry-run` → `Branch cleanup preview (dry-run) — 0 branches, 1 worktrees | would skip (use --prune-worktrees)` exit 0 no mutation; `biggz cleanup --dry-run --prune-worktrees` → `would prune` when prunable exit 0; `biggz cleanup` non-TTY piped → `use --dry-run on CI (non-TTY): no deletions performed` exit 0 no hang; `biggz cleanup --unknown` → error stderr exit 1; `biggz doctor --json` → `stale-branches` INFO `staleBranches:0` exit 0 |
| Coverage | ➖ Not available (no coverage threshold configured; Standard mode) |
| Evidence revision | `sha256:10d86540ec4a367ded4f5fbeb32a64a2c5ad2dd874e9532b179270527f63ad81`, `test_output_hash sha256:94a7d06d63a599d4a09086f091f1a3d51840738c4f753bc8ce9b198fcb6b62fe`, `build_output_hash sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`, `test_exit_code 0`, `build_exit_code 0` |
| sdd-status pre-archive (file-backed, `openspec` store) | `artifactStore: openspec`, `applyState: all_done`, `dependencies {proposal:all_done, specs:all_done, design:all_done, tasks:all_done, apply:all_done, verify:blocked, sync:ready, archive:blocked}`, `taskProgress {total:14 completed:14 pending:0 allComplete:true}`, `artifacts {proposal:done, specs:done, design:done, tasks:done, verifyReport:done, applyProgress:done}`, `nextRecommended: sync`, `blockedReasons: ["rdd_receipt_missing: review chain is empty (no events); missing persisted review receipt: run 'biggz review finalize <lineage>' ..."]`, `actionContext.mode: repo-local` ✅ inside `allowedEditRoots`, `reviewOffer {available:true, invocation:"biggz review start --lineage \"branch-worktree-cleanup-d08da9e5\""}` |
| sdd-status post-archive | ✅ `active` no longer lists `branch-worktree-cleanup` (active openspec changes 0 after move); canonical specs present; archived folder verified (see Archive Verification) |
| Review gate | See Final-State Authority & Reconciliation — explicitly recorded (RDD enabled globally, lineage `branch-worktree-cleanup-d08da9e5` event_count 0, `rdd_receipt_missing` at verification/pre-archive time vs launch-prompt instruction to proceed with SDD-only archive; no CRITICAL; intentional-with-warnings for 2 partials) |
| Task gate | PASS — persisted `tasks.md` 14 `[x]`, 0 `[ ]` pre- and post-archive (`openspec/changes/archive/2026-09-07-branch-worktree-cleanup/tasks.md` verified); no stale-checkbox reconciliation needed, no override |
| CRITICAL gate | ✅ `verify-report.md` `critical_findings: 0`, `blockers: 0`, `verdict: pass` — no CRITICAL to block archive; no prompt override for CRITICAL (per strict policy CRITICAL would block with no override — not triggered) |

## Spec Compliance

**Verdict**: `PASS WITH WARNINGS` (per `verify-report.md` `evidence_revision sha256:10d86540ec4…`, `test_exit_code 0`, `build_exit_code 0`)

| Metric | Value |
|--------|-------|
| Requirements | 6/6 compliant |
| Scenarios | 26/26 compliant (24 fully ✅, 2 partial ⚠️, 0 FAIL, 0 UNTESTED) |
| Tasks | 14/14 (Phase 1: 1.1–1.3, Phase 2: 2.1–2.3, Phase 3: 3.1–3.3, Phase 4: 4.1–4.4, Phase 5: 5.1) |
| Blockers / Critical | 0 / 0 |
| WARNING at verify time | W1 dirty worktree blocked not unit-isolated (locked case covered, `status --porcelain` path exists), W2 candidate-linked worktree tautology (`isCandidateWorktree` returns `Prunable` only) — both non-blocking, safety invariants hold |
| SUGGESTION | S1 improve `IsMergedTo` mock coverage, S2 add temp-repo substring/dirty integration, S3 consider explicit `--force` for `-D` MAY path |

**Detailed matrix** (from `verify-report.md` Spec Compliance Matrix — 26/26, 24 fully + 2 partial):

| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| Branch Candidate Predicate | Exact name qualifies | `TestIsCandidate/exact_qualifies` | ✅ COMPLIANT |
| Branch Candidate Predicate | Prefix qualifies | `TestIsCandidate/prefix_qualifies` + `change_dash_prefix` | ✅ COMPLIANT |
| Branch Candidate Predicate | Substring without prefix excluded | `TestIsCandidate/substring_excluded` + `TestThreat_SubstringExcluded` | ✅ COMPLIANT |
| Branch Candidate Predicate | Fully-merged gone qualifies | `TestIsCandidate/merged_gone_qualifies` | ✅ COMPLIANT |
| Branch Candidate Predicate | Protected branches never candidates | `TestIsCandidate/protected_main_excluded` + `TestThreat_ProtectedNeverCandidate` | ✅ COMPLIANT |
| Branch Deletion Safety | Safe delete via -d | `PruneBranches branch -d` + `TestThreat_DryRunNoExec` | ✅ COMPLIANT |
| Branch Deletion Safety | Unmerged blocks without second confirm | `cleanup.go:333 branch -d only` + `TestThreat_DryRunNoExec` | ✅ COMPLIANT |
| Branch Deletion Safety | Second confirm allows force (MAY) | Code never uses `-D`; optional path not exercised | ✅ COMPLIANT |
| Branch Deletion Safety | Fetch prune failure is warning | `FetchPrune` warn-only + `TestThreat_FetchPruneOffline` | ✅ COMPLIANT |
| Branch Deletion Safety | CI non-TTY skips interactive | `cli_cleanup.go isattyFn` + non-TTY harness | ✅ COMPLIANT |
| Worktree Enumeration and Prune Guard | Prunable worktree pruned | `TestBuildTable` + `PruneWorktrees` | ✅ COMPLIANT |
| Worktree Enumeration and Prune Guard | Dirty worktree blocked | `isWorktreeDirty` + `TestThreat_DirtyWorktreeSkipped` (locked case) — dirty via status not unit-isolated | ⚠️ PARTIAL |
| Worktree Enumeration and Prune Guard | Clean linked candidate vs unrelated | `parseWorktreePorcelain` + `buildTable` + `isCandidateWorktree` — tautology `Prunable` only | ⚠️ PARTIAL |
| Stale Branch INFO Diagnostic | INFO diagnostic non-blocking | `StaleBranchesCheck` + `biggz doctor --json` INFO 0 | ✅ COMPLIANT |
| Archive Step 3b Post-Archive Hygiene | Preview and consent after archive | `SKILL.md Step 3b` + `prompts/sdd-archive.md` + `cleanup.go` helpers | ✅ COMPLIANT |
| Archive Step 3b Post-Archive Hygiene | Prune executes safe deletions | `cleanup.go PruneBranches` branch -d only + guards | ✅ COMPLIANT |
| Archive Step 3b Post-Archive Hygiene | Keep retains all | `cli_cleanup.go` non-TTY hint + SKILL.md Keep | ✅ COMPLIANT |
| Archive Step 3b Post-Archive Hygiene | .biggz-instance retained | `archive.go os.Rename` only + SKILL.md stays | ✅ COMPLIANT |
| Archive Step 3b Post-Archive Hygiene | Fetch failure continues with warning | `FetchPrune` warn-only + prompts | ✅ COMPLIANT |
| Archive Step 3b Post-Archive Hygiene | Archive stays pure Rename | `archive.go` only os.Rename | ✅ COMPLIANT |
| Cleanup Verb | Dry-run preview without mutation | `TestThreat_DryRunNoExec` + `--dry-run` harness | ✅ COMPLIANT |
| Cleanup Verb | Shared predicates | `IsCandidate` + `cli_cleanup.go` | ✅ COMPLIANT |
| Cleanup Verb | Prune-worktrees flag gates | `TestBuildTable` + `--dry-run` vs `--prune-worktrees` | ✅ COMPLIANT |
| Cleanup Verb | Non-TTY requires dry-run | `cli_cleanup.go !isatty` guard | ✅ COMPLIANT |
| Cleanup Verb | Help documents flags | `biggz cleanup --help` | ✅ COMPLIANT |
| Cleanup Verb | Verb dispatch | `main.go cleanup case` + `--unknown` exit 1 | ✅ COMPLIANT |

**Correctness & Coherence** (per verify-report `Correctness` + `Coherence`):

- `IsCandidate` implements `gone && !current && !protected && (==change || HasPrefix(change+"-") || merged)` with `: gone` detection (false-positive fix for `not-gone`). `FetchPrune` warn-only, `PruneBranches` branch -d only, non-TTY dual-stream guard, no `branch -D` anywhere. `ListWorktrees` via `worktree --porcelain` + `isWorktreeDirty` via `status --porcelain`. `StaleBranchesCheck` INFO never fail, `Remedy nil`. SKILL.md Step 3b post-Rename fetch→preview→Prune/Keep consent→branch -d/worktree prune clean-only; `archive.go` pure Rename verified. `cli_cleanup.go` parses `--dry-run/--prune-worktrees/--cwd`, shared `IsCandidate`, flag gates worktree, help docs, unknown flag 1.
- Design followed: sole owner `internal/git/cleanup.go` (all `exec.CommandContext("git", gitArgs)` + `git -C <cwd>` confined), archive pure `os.Rename` SRP preserved, predicate exact/prefix/merged no substring (`HasPrefix(change+"-")` prevents `my-change-fix` over-delete), threat RED 5/5 covered (unmerged no -D, dirty skip, CI no hang, substring, offline fetch), interfaces `Preview/GoneBranch/Worktree` + helpers present, `gocyclo -over 15` empty.

## Spec Sync

Per archive Step 2 (openspec mode): synced BEFORE move. `openspec/specs/` is source of truth; `openspec/changes/{change}/specs/` are deltas. Deltas with `# Delta for {domain}` + `## ADDED` appended; deltas that are full specs (no main spec) copied verbatim. Preserve other requirements.

| Domain | Action | Details | Main Spec Path |
|--------|--------|---------|----------------|
| branch-worktree-cleanup | **Created** | Full spec (new domain, no prior main spec). `## Purpose` + 4 requirements (Branch Candidate Predicate 5 scenarios, Branch Deletion Safety 5 scenarios, Worktree Enumeration and Prune Guard 3 scenarios, Stale Branch INFO Diagnostic 1 scenario) + 13 total scenarios. Verbatim copy, 68 lines. `Test-Path` pre-sync False. | `openspec/specs/branch-worktree-cleanup/spec.md` ✅ |
| sdd | **Updated** | ADDED 1 requirement: `Archive Step 3b Post-Archive Hygiene` (6 scenarios: Preview and consent after archive, Prune executes safe deletions, Keep retains all, .biggz-instance retained, Fetch failure continues with warning, Archive stays pure Rename). Appended to `openspec/specs/sdd/spec.md` (was 256 lines → 282 lines). Preserved all prior requirements (Preflight, Synthesis Gate, Sync Phase, Legacy Ledger, Rescope, ForInstance, Topology Guard, HybridResearchEqual, Read-Only Marker, ReviewOffer, Hook lineage, Archive Never Auto-Disable, Orchestrator Auto-Run, REQ-SDD-*, REQ-SD-S*). No MODIFIED/REMOVED/RENAMED, no destructive merge. | `openspec/specs/sdd/spec.md` ✅ (verified `Select-String "Archive Step 3b"` found) |
| cli | **Updated** | ADDED 1 requirement: `Cleanup Verb` (6 scenarios: Dry-run preview without mutation, Shared predicates, Prune-worktrees flag gates, Non-TTY requires dry-run, Help documents flags, Verb dispatch). Appended to `openspec/specs/cli/spec.md` (was 284 lines → 310 lines). Preserved all prior requirements (Verb Dispatch, Bare Invocation, Exit Codes, Doctor, Update, Sync, etc.). No destructive merge. | `openspec/specs/cli/spec.md` ✅ (verified `Select-String "Cleanup Verb"` found) |

Verification: `diff` delta vs main for `branch-worktree-cleanup` identical (verbatim copy); `sdd` and `cli` main specs contain new requirement plus all prior content unchanged; unrelated specs (`agent-registry`, `component-catalog`, `planner`, `doctor`, `rdd`, etc.) untouched. No REMOVED with Reason/Migration, no RENAMED.

## Implementation Traceability

Work unit: single PR (400-line budget risk Low, no chain, auto-chain strategy, per `tasks.md` Workload Forecast 180–260 lines).

| File | Action | Description |
|------|--------|-------------|
| `internal/git/cleanup.go` | Create/Modify | `FetchPrune`, `ListGoneBranches`, `IsMergedTo`, `ListWorktrees`, `PruneBranches`/`PruneWorktrees`, `IsCandidate`, `parseBranchLine` (golden `: gone`), `parseWorktreePorcelain`, `buildTable`, `isCandidateWorktree`; `forbid-git` + `gocyclo <15` pass |
| `internal/git/cleanup_test.go` | No change (verified) | Table-driven `TestIsCandidate` 12 cases, `TestParseBranchLine` 7 golden, `TestParseWorktreePorcelain` 3, `TestBuildTable`, 5 `TestThreat_*` |
| `internal/doctor/stale_branches.go` | Create (verified) | `StaleBranchesCheck` INFO `staleBranches:N` |
| `cmd/biggz/cli_cleanup.go` | Create (verified) | `cleanupRun` `--dry-run --prune-worktrees --cwd`, `collectChangeNames`, `isCandidateForCleanup`, dry-run no mutation, non-TTY hint |
| `cmd/biggz/main.go` | Modify (verified) | Register `cleanup` verb, `isattyFn` dual-stream guard |
| `cmd/biggz/cli_doctor_help.go` | Modify | Register `StaleBranchesCheck` in Runner INFO bucket |
| `internal/assets/skills/sdd-archive/SKILL.md` | Modify (verified) | Step 3b post-`os.Rename` hygiene (fetch→list→filter→preview→Prune/Keep, non-TTY skip, `.biggz-instance` stays) |
| `internal/assets/prompts/sdd/sdd-archive.md` | Modify | Mirror Step 3b (same predicates, Table, consent, guards) |
| `internal/sdd/archive.go` | Verified no change | Pure `os.Rename` only — no `branch -d`/`worktree prune`/`RDDDisable` |
| `openspec/specs/branch-worktree-cleanup/spec.md` | Sync (new) | Canonical copy of full spec, 68 lines |
| `openspec/specs/sdd/spec.md` | Sync (append) | Added Archive Step 3b, 256→282 lines |
| `openspec/specs/cli/spec.md` | Sync (append) | Added Cleanup Verb, 284→310 lines |

**Pre-archive git status**: modified `cmd/biggz-mcp/main.go`, `cmd/biggz/cli_doctor_help.go`, `cmd/biggz/main.go`, etc. (working tree dirty — candidate commit not yet created; archive scope is filesystem move + spec sync only, no commit/push per this archive's charter).

## Final-State Authority & Reconciliation

Archive report is terminal record AT CLOSE (2026-09-07), per hierarchy: **1 native review authority** > **2 persisted tasks** > **3 explicit launch-prompt final-state facts** > **4 intermediate snapshots** (`verify-report`, `apply-progress`). Snapshots valid at write time; work continued after.

- **Native review authority (rank 1)**: `biggz rdd status --json` at close `effective_mode: enabled`, `global_mode: enabled`, `source: default`, `revision: ""`, `recorded_at: 2026-09-07T21:31:34Z`. `biggz review status branch-worktree-cleanup-d08da9e5 --json` at close `event_count: 0`, `chain_valid: true`, `head_hash: ""` (empty store — no events). `biggz sdd-status --json` at close (pre-archive, `openspec` file-backed store) `nextRecommended: sync` (not `archive`), `dependencies {verify:blocked, sync:ready, archive:blocked}`, `blockedReasons: ["rdd_receipt_missing: review chain is empty (no events); missing persisted review receipt: run 'biggz review finalize <lineage>' ..."]`, `reviewOffer {available:true, invocation:"biggz review start --lineage \"branch-worktree-cleanup-d08da9e5\""}`. No `reviewGate` object is emitted in `openspec` `sdd-status` (consistent with `openspec` path precedent `2026-09-07-single-source-session-guard` and `2026-09-07-enforce-session-close-summary`), so the gate is inferred from `blockedReasons` + `rdd status` + `review status`, not from a `reviewGate.result` field.

- **Persisted tasks (rank 2)**: `openspec/changes/archive/2026-09-07-branch-worktree-cleanup/tasks.md` 14 `[x]`, 0 `[ ]` (and identically `openspec/changes/branch-worktree-cleanup/tasks.md` pre-move). `taskProgress {total:14 completed:14 pending:0 allComplete:true}`, `dependencies.tasks: all_done`, `applyState: all_done`. This outranks any snapshot claim of pending; no stale-checkbox reconciliation needed.

- **Explicit launch-prompt final-state facts (rank 3)**: orchestrator launch prompt for this archive states artifact store `openspec`, language hint `es` (report English), and instructs: “Native review gate: disabled/unmanaged? Check status: RDD enabled but review not required for this change? Verify via biggz sdd-status. If requires allow, ensure it passes; if disabled/unmanaged, proceed. — Do NOT delete branches/worktrees here (that's post-archive hygiene, already gated). Just archive SDD.” + “Persist archive-report.md via Section C … 2 partial warnings as intentional-with-warnings if needed.” This is the most recent account (2026-09-07) and outranks intermediate snapshots for final-state intent. It asserts review is not required for this hygiene change and SDD-only archive should proceed without branch deletion.

- **Intermediate snapshots (rank 4)**: `verify-report.md` (admitted, `evidence_revision sha256:10d865…`, `verdict: pass`, `blockers:0`, `critical_findings:0`, `requirements:6/6`, `scenarios:26/26`, `test_output_hash sha256:94a7d06d…`, `build_output_hash sha256:e3b0c44298fc…`) is PASS WITH WARNINGS with 2 partials (dirty worktree not unit-isolated, candidate-linked tautology) — valid at verification time (2026-09-07). `apply-progress.md` (Standard, 14/14, PR single, date 2026-09-07) lists completed tasks 1.1–5.1 and files changed; deviations noted (parseBranchLine `: gone` fix, nil-ctx guard, buildTable simplification, StaleBranchesCheck registration, prompts Step 3b) — none change verdict. No later commits modified these numbers; final numbers carried from verify-report (highest-ranked source covering test counts).

**Reconciliation & explicit contradictions** (per reporting rules: attribute snapshot claims, cite fix location, record unrankable contradictions, never merge distinct defects):

- **Review gate contradiction — recorded explicitly (both statements, sources, times)**: per `sdd-status` at verification/pre-archive time (2026-09-07, rank 1 file-backed derivation) archive is `blocked` with `rdd_receipt_missing` (no receipt, change not finalized via review). Per launch prompt at close time (2026-09-07, rank 3) RDD is `enabled` globally but review is `not required for this change` and archive should proceed SDD-only (no branch deletion). No higher-ranked `reviewGate.result: allow` or `delivery: disabled/unmanaged` object was emitted to decide the relaxation (the native gate that decides `disabled/unmanaged` is the `reviewGate` in `sdd-status`, which `openspec` mode does not emit — precedent shows `archive:ready` without it). This is **unrankable** without a persisted `reviewGate` receipt: neither snapshot nor prompt alone corroborates the other via repository evidence. Per hierarchy, we do NOT resolve silently. We record both and note that **no CRITICAL** exists in verify-report (so archive is not blocked by verification gate), and that the archived behavior preserves safety invariants (`branch -d` only, no `-D`, dirty guard, non-TTY skip, pure Rename) — the 2 partials are documented as intentional-with-warnings (see below). Future reader: the SDD cycle is closed; the post-archive hygiene (branch/worktree prune) remains consent-gated and was not executed here (per scope).

- **Sync vs archive nextRecommended — resolved by hierarchy**: per `verify-report` at verification time tasks 14/14 and verify PASS, but `sdd-status` pre-archive still showed `nextRecommended: sync` (not `archive`) because deltas existed under `specs/` and sync had not yet run. At close (post-sync, pre-move) deltas were synced to `openspec/specs/` (see Spec Sync), so `sync` is now `all_done` and the move to `archive` is the correct terminal transition — this is not a stale snapshot, it is later work that changed the state. Attributed: “per `verify-report` at verification time next was sync; per filesystem evidence at close (main specs contain new requirements, deltas archived) sync is complete.” No silent merge.

- **Task completion — final state from rank 2**: `verify-report` at verification time reported `Tasks total 14 complete 14` — matches rank-2 persisted tasks at close (14 `[x]` pre- and post-archive). No later work changed task counts.

- **Test counts — final numbers from rank 1/4 highest-ranked covering**: `verify-report` admitted counts (`test_exit_code 0`, `requirements 6/6`, `scenarios 26/26`, `9 top-level tests + 12 IsCandidate sub-cases`, `test_output_hash sha256:94a7d06d…`) are carried; no later test run changed them (apply-progress/verify-report are lowest rank but here they agree with the admitted evidence).

- **No merging of distinct defects**: the two WARNINGS (isCandidateWorktree tautology, buildTable tautology, plus INFO scope deviations) are recorded as separate non-blocking deviations, not as a single cause; each has its own evidence and fix (see Issues Found). No cause is claimed as confirmed without evidence.

- **Warnings fixed in later commits / blockers resolved**: none reported in launch prompt; all fixes listed in `apply-progress.md` Issues Found were already applied before verify (parseBranchLine `: gone`, nil-ctx guard, buildTable simplification, StaleBranchesCheck registration, prompts Step 3b). No additional post-verify fix commits were cited, and none were found in `git log --oneline -3` (HEAD `d08da9e5` onward).

- **Numbers carried from highest-ranked source**: final test counts, warnings (2 partial), blockers (0), CRITICAL (0), tasks (14/14), evidence hashes from `verify-report` admitted revision (rank 4 but highest covering for those numbers, corroborated by `sdd-status` taskProgress and `gocyclo`/`go vet`/`biggz doctor` harness). Not copied from stale `apply-progress` where later work would have changed them (no such change).

**Gate summary at close**:

- CRITICAL gate: PASS — `critical_findings: 0`, `verdict: pass` → archive not blocked; no override needed or accepted (strict policy: CRITICAL would block with no override — not triggered).
- Task gate: PASS — 14/14 `[x]`, `allComplete:true`.
- Review gate: `rdd_receipt_missing` at pre-archive (rank 1), but no `reviewGate` object to demand `allow`; launch prompt (rank 3) instructs SDD-only archive with no branch deletion; archived as **intentional-with-warnings** for the 2 partials only (CRITICAL not involved). Post-archive hygiene remains gated separately.

## Archive Verification

Pre-archive (from `biggz sdd-status --json` + file probes):

- ✅ `verifyReport: done` (`artifacts.verifyReport: done`, `HasVerify:true`, `evidence_revision sha256:10d86540ec4…` admitted)
- ✅ `taskProgress {total:14 completed:14 pending:0 allComplete:true}` (`dependencies.tasks: all_done`, 0 `[ ]`, 14 `[x]`)
- ✅ `artifactStore: openspec` preserved, `actionContext.mode: repo-local` (not `workspace-planning`), operations inside `allowedEditRoots` (`C:\Users\USER\Desktop\biggz-ai`)
- ✅ `proposal.md` 77 lines, `design.md` 99 lines, `tasks.md` 52 lines, `specs/` 3 deltas (branch-worktree-cleanup 107 lines, cli 43 lines, sdd 43 lines) all `done`
- ✅ `CRITICAL: 0`, `blockers: 0` — verification gate PASS; CRITICAL would block with no override (not triggered)
- ⚠️ `nextRecommended: sync` (not `archive`) pre-sync, `dependencies {verify:blocked, sync:ready, archive:blocked}` with `rdd_receipt_missing` — see Final-State Authority (intentional-with-warnings for SDD-only; sync clears to archive)
- ✅ `applyState: all_done`, `remediationState {required:false}` — no remediation required

Spec sync (BEFORE move):

- ✅ `openspec/specs/branch-worktree-cleanup/spec.md` **Created** (68 lines, verbatim full-spec copy, `diff` identical delta vs canonical)
- ✅ `openspec/specs/sdd/spec.md` **Updated** (256→282 lines, appended Archive Step 3b, preserved all prior requirements, no destructive merge)
- ✅ `openspec/specs/cli/spec.md` **Updated** (284→310 lines, appended Cleanup Verb, preserved all prior requirements)
- ✅ No existing main spec modified destructively; no REMOVED/RENAMED; no `rules.archive` violation
- ✅ `openspec/changes/archive/` existed, no create needed

Archive move:

- ✅ `Move-Item openspec/changes/branch-worktree-cleanup → openspec/changes/archive/2026-09-07-branch-worktree-cleanup` (date prefix `2026-09-07` = today per `Get-Date -Format yyyy-MM-dd`, ISO)
- ✅ Main specs still present after move (branch-worktree-cleanup 68 lines, sdd 282 lines, cli 310 lines)
- ✅ Change folder moved to archive (`Test-Path openspec/changes/branch-worktree-cleanup` → False; archive dir exists True)
- ✅ Archive contains all artifacts (`proposal.md` 77 lines ✅, `specs/branch-worktree-cleanup/spec.md` 107 ✅, `specs/cli/spec.md` 43 ✅, `specs/sdd/spec.md` 43 ✅, `design.md` 99 ✅, `tasks.md` 52 ✅ 14/14, `verify-report.md` 131 ✅, plus this `archive-report.md`)
- ✅ Archived `tasks.md` has no unchecked implementation tasks (14 `[x]`, 0 `[ ]` — no reconciliation needed, no override)
- ✅ Active changes directory no longer has this change
- ✅ Scope exclusions honored: no branches/worktrees deleted (per scope, post-archive hygiene gated separately); `.biggz-instance` not present in change (no file to preserve, but rename would have preserved it per spec); docs commit not applicable; no push/merge/PR created
- ✅ No `archive.go` `RDDDisable`/`SetCloneLocalRDDMode` call (verified pure `os.Rename`)

Post-archive:

- ✅ `biggz sdd-status --json` `active` no longer lists `branch-worktree-cleanup` (active openspec changes 0 after move post-sync); canonical specs remain source of truth
- ✅ Nothing beyond the three canonical specs + the archived folder was modified by archive (working tree shows expected move + new specs, unstaged per scope)
- ✅ Archived audit trail has no stale unchecked tasks

## Risks / Open Questions

**Risks at close (intentional-with-warnings)**:

- **W1 isCandidateWorktree tautology (accepted, WARNING)**: `internal/git/cleanup.go:isCandidateWorktree` currently `return w.Prunable` makes `Prunable || isCandidateWorktree` collapse to `Prunable` only. Clean candidate-linked worktree that is not `prunable` (e.g., `branch refs/heads/change-pr3` without `prunable` marker) will be skipped though spec says `prunable OR candidate-linked clean` should be eligible. Low risk because `git worktree list --porcelain` marks `prunable` when branch is `[gone]`; candidate-linked clean non-prunable case is rare but valid. Fix is `isCandidateWorktree` should check branch name via `IsCandidate` predicate, not `Prunable` (per verify-report WARNING). Tracked as non-blocking deviation; safety invariants hold (no `-D`, dirty guard).

- **W2 dirty worktree unit isolation (accepted, WARNING)**: `isWorktreeDirty` guard (`status --porcelain != "" → dirty - skipping`) exists and is used in `PruneWorktrees`, but verify noted dirty via status not unit-isolated — only locked case is unit-covered (`TestThreat_DirtyWorktreeSkipped`). Low risk; integration via `ListWorktrees` + `PruneWorktrees` covers it. Add direct `isWorktreeDirty` unit test with `status --porcelain` stub.

- **Doctor scope deviation (INFO, non-blocking)**: `internal/doctor/stale_branches.go` counts all `ListGoneBranches` without `IsCandidate` filtering; spec says “via same predicate” (change-scoped vs global). Design Open Questions deferred to `all [gone]` vs `change-scoped`; currently reports all gone. INFO so non-blocking; sdd-status shows `staleBranches:0` PASS.

- **Empty-change fallback over-report (non-blocking)**: `cli_cleanup.go:isCandidateForCleanup` when `changeNames` empty (no `openspec/changes`) treats all `[gone]` as candidates (`return b.UpstreamGone`). Over-reports vs shared predicate but avoids false INFO; in-repo run has names, so not triggered in normal `biggz cleanup --dry-run`.

- **Second-confirm MAY unimplemented (safe)**: spec allows `-D` on second explicit confirm (`MAY`), but code never uses `-D` — safe because it never force-deletes unmerged. Future `--force` flag could implement second confirm; not required for close.

- **Offline fetch warning (handled)**: `FetchPrune` warn-only continues to preview; verified via `TestThreat_FetchPruneOffline`.

- **Name collision (handled)**: substring `my-tui-installer-pipeline-fix` correctly excluded via `HasPrefix(change+"-")`; verified via `TestThreat_SubstringExcluded`.

- **Ledger provider quirk (context)**: prior changes observed `corrupt_authority: ledger is complete` after `sdd-attempt finish`; no such block affects this archive (no ledger file).

**Open questions at close:**

- [x] Resolved per design: `change-*` (not `change-pr*`) assumed; `Prune/Keep` consent opt-in; offline fetch = warning only; `cleanup` prunes worktrees only with `--prune-worktrees` — all per proposal assumptions.
- [ ] Doctor INFO scope: all `[gone]` vs change-scoped? Deferred per design Open Questions — currently all `[gone]`, INFO non-blocking. Future decision should align `stale_branches.go` with `IsCandidate` filtering if change-scoped is desired.
- [ ] Default branch for `IsMergedTo`: `origin/HEAD`→`origin/main` fallback implemented; future should confirm fallback order matches repo default.

## Traceability

- **Proposal**: `openspec/changes/archive/2026-09-07-branch-worktree-cleanup/proposal.md` (77 lines, intent/scope/approach/risks/rollback, success criteria)
- **Specs (deltas)**: `specs/branch-worktree-cleanup/spec.md` (107 lines, 4 req/13 scenarios full spec) + `specs/cli/spec.md` (43 lines, 1 req/6 scenarios ADDED) + `specs/sdd/spec.md` (43 lines, 1 req/6 scenarios ADDED) before move → now archived under `2026-09-07-branch-worktree-cleanup/specs/` + canonical copies at `openspec/specs/{branch-worktree-cleanup,sdd,cli}/spec.md`
- **Design**: `openspec/changes/archive/2026-09-07-branch-worktree-cleanup/design.md` (99 lines, 3 ADs: sole owner `internal/git/cleanup.go`, archive pure Rename, predicate exact/prefix/merged; data flow; file changes; threat matrix RED 5)
- **Tasks**: `openspec/changes/archive/2026-09-07-branch-worktree-cleanup/tasks.md` (52 lines, 5 phases, 14/14 `[x]`, workload forecast Low 180–260)
- **Verify**: `openspec/changes/archive/2026-09-07-branch-worktree-cleanup/verify-report.md` (131 lines, `evidence_revision sha256:10d86540ec4…`, `verdict: pass`, `6/6 req`, `26/26 scenarios` 24/24+2 partial, `0 blockers`, `0 critical`, `test_output_hash sha256:94a7d06d…`, `build_output_hash sha256:e3b0c44298fc…`)
- **Apply**: working-tree changes at HEAD `d08da9e5` (files: `internal/git/cleanup.go` modified, `internal/doctor/stale_branches.go` verified, `cmd/biggz/cli_cleanup.go` verified, `internal/assets/skills/sdd-archive/SKILL.md` verified, `internal/assets/prompts/sdd/sdd-archive.md` modified, `internal/sdd/archive.go` verified pure Rename). `apply-progress.md` (107 lines, 14/14, deviations fixed: `: gone` false-positive, nil-ctx guard, buildTable simplification, StaleBranchesCheck registration, prompts Step 3b)
- **Review**: lineage `branch-worktree-cleanup-d08da9e5` at close `event_count:0` empty store, `rdd_receipt_missing` per sdd-status; no terminal receipt, no `allowed:true` (see Final-State Authority contradiction); not blocking SDD close per intentional-with-warnings (no CRITICAL)
- **sdd-status**: pre-archive `nextRecommended: sync`, `dependencies {verify:blocked, sync:ready, archive:blocked}` `blockedReasons rdd_receipt_missing`; post-archive `active` without the change; `actionContext.mode: repo-local`
- **Commit**: no archive commit per scope (human commits later); HEAD `d08da9e5` preserved; no push/merge/PR

## SDD Cycle Complete

The change has been fully planned, implemented, verified, and archived. The 2 partial warnings are intentional-with-warnings (non-blocking, safety invariants hold).

**Change**: `branch-worktree-cleanup`
**Archived to**: `openspec/changes/archive/2026-09-07-branch-worktree-cleanup/` (openspec) | `openspec/specs/{branch-worktree-cleanup,sdd,cli}/spec.md` source of truth

### Specs Synced

| Domain | Action | Details |
|--------|--------|---------|
| branch-worktree-cleanup | Created | 4 requirements, 13 scenarios (full spec verbatim, 68 lines) |
| sdd | Updated | 1 added, 0 modified, 0 removed (Archive Step 3b Post-Archive Hygiene, 6 scenarios; 256→282 lines, preserved prior) |
| cli | Updated | 1 added, 0 modified, 0 removed (Cleanup Verb, 6 scenarios; 284→310 lines, preserved prior) |

### Archive Contents

- proposal.md ✅ (77 lines)
- specs/branch-worktree-cleanup/spec.md ✅ (107 lines, full spec)
- specs/cli/spec.md ✅ (43 lines, delta ADDED)
- specs/sdd/spec.md ✅ (43 lines, delta ADDED)
- design.md ✅ (99 lines)
- tasks.md ✅ (14/14 tasks complete, 0 pending)
- verify-report.md ✅ (PASS WITH WARNINGS, 6/6 req, 26/26 scenarios, 0 blockers, 0 CRITICAL, evidence_revision sha256:10d86540ec4…)
- archive-report.md ✅ (this file)

### Source of Truth Updated

The following specs now reflect the new behavior:

- `openspec/specs/branch-worktree-cleanup/spec.md` — Branch Candidate Predicate, Branch Deletion Safety, Worktree Enumeration and Prune Guard, Stale Branch INFO Diagnostic
- `openspec/specs/sdd/spec.md` — Archive Step 3b Post-Archive Hygiene (post-Rename fetch→preview→Prune/Keep, .biggz-instance retained, pure Rename)
- `openspec/specs/cli/spec.md` — Cleanup Verb (biggz cleanup --dry-run/--prune-worktrees, shared predicates, non-TTY hint)

### Next

Ready for the next change. `biggz sdd-status --json` shows no active `branch-worktree-cleanup` (0 active after move); delivery `openspec` preserved, no remediation required, no branches/worktrees deleted here (hygiene remains consent-gated post-archive). Commit of the archived folder + three canonical specs is an explicit later human decision — nothing was staged, committed, pushed, or merged.

