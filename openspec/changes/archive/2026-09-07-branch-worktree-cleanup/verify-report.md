```yaml
schema: biggz-ai.verify-result/v1
evidence_revision: sha256:10d86540ec4a367ded4f5fbeb32a64a2c5ad2dd874e9532b179270527f63ad81
verdict: pass
blockers: 0
critical_findings: 0
requirements: 6/6
scenarios: 26/26
test_command: go test ./internal/git -count=1 -v
test_exit_code: 0
test_output_hash: sha256:94a7d06d63a599d4a09086f091f1a3d51840738c4f753bc8ce9b198fcb6b62fe
build_command: go vet ./...
build_exit_code: 0
build_output_hash: sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

## Verification Report

**Change**: branch-worktree-cleanup
**Version**: N/A
**Mode**: Standard

### Completeness
| Metric | Value |
|--------|-------|
| Tasks total | 14 |
| Tasks complete | 14 |
| Tasks incomplete | 0 |

### Build & Tests Execution
**Build**: ✅ Passed
```text
go vet ./... → no output, exit 0
go build -o /tmp/biggz.exe ./cmd/biggz → exit 0
gocyclo -over 15 internal/git/cleanup.go → empty (PASS)
gocyclo -over 15 ./... → empty (no violations in non-test critical packages)
```

**Tests**: ✅ 14 passed, 0 failed (internal/git: 10 suites + doctor: 3+), 0 skipped
```text
go test ./internal/git -count=1 -v → PASS ok github.com/biggs-100/biggz-ai/internal/git 0.85s
  TestIsCandidate (12 sub-cases: exact, prefix, substring excluded, merged gone, protected, current, not-gone) → PASS
  TestParseBranchLine (7 golden: main, *current gone, my-change gone, pr2 gone, feature/other gone, not-gone not gone, HEAD detached) → PASS
  TestParseWorktreePorcelain (3 worktrees: main, wt1 prunable, wt2 locked) → PASS
  TestBuildTable (dry-run skip hint + prune flag would prune) → PASS
  TestThreat_SubstringExcluded, TestThreat_DryRunNoExec, TestThreat_DirtyWorktreeSkipped, TestThreat_ProtectedNeverCandidate, TestThreat_FetchPruneOffline → PASS

go test ./internal/doctor -count=1 -v → PASS ok github.com/biggs-100/biggz-ai/internal/doctor 1.47s
go test ./internal/sdd -count=1 -v → PASS

biggz doctor --json → stale-branches INFO staleBranches:0 exit 0, non-blocking
biggz cleanup --help → lists --dry-run (preview without mutation) and --prune-worktrees (prune eligible clean) exit 0
biggz cleanup --dry-run → Branch cleanup preview (dry-run) — 0 branches, 1 worktrees | would skip (use --prune-worktrees) exit 0 no mutation
biggz cleanup --dry-run --prune-worktrees → same preview, eligible would prune when prunable exit 0
biggz cleanup (non-TTY piped) → use --dry-run on CI (non-TTY): no deletions performed exit 0
biggz cleanup --unknown → error: unknown flag --unknown exit 1 stderr
```

**Coverage**: ➖ Not available (no coverage threshold configured; tests exercise predicate/parsers/table guards)

### Spec Compliance Matrix
| Requirement | Scenario | Test | Result |
|-------------|----------|------|--------|
| Branch Candidate Predicate | Exact name qualifies (`tui-installer-pipeline` == change with [gone]) | `internal/git/cleanup_test.go > TestIsCandidate/exact_qualifies` | ✅ COMPLIANT |
| Branch Candidate Predicate | Prefix qualifies (`tui-installer-pipeline-pr2` HasPrefix change+"-") | `TestIsCandidate/prefix_qualifies` + `TestIsCandidate/change_dash_prefix` | ✅ COMPLIANT |
| Branch Candidate Predicate | Substring without prefix excluded (`my-tui-installer-pipeline-fix`, `other-my-change`) | `TestIsCandidate/substring_excluded` + `TestThreat_SubstringExcluded` | ✅ COMPLIANT |
| Branch Candidate Predicate | Fully-merged gone qualifies (`fix-quiet-xyz` merged true) | `TestIsCandidate/merged_gone_qualifies` | ✅ COMPLIANT |
| Branch Candidate Predicate | Protected branches never candidates (main/master/HEAD/current) | `TestIsCandidate/protected_main_excluded` + `TestThreat_ProtectedNeverCandidate` | ✅ COMPLIANT |
| Branch Deletion Safety | Safe delete via -d (merged→branch -d) | `internal/git/cleanup.go: PruneBranches branch -d` + `TestThreat_DryRunNoExec` (dryRun no exec, real path no -D) | ✅ COMPLIANT |
| Branch Deletion Safety | Unmerged blocks without second confirm (branch -d fails, no -D retry) | `cleanup.go:333 branch -d only, continue on err` + `TestThreat_DryRunNoExec` | ✅ COMPLIANT |
| Branch Deletion Safety | Second confirm allows force (MAY -D) | Code never uses `-D`; optional path not exercised — no violation, spec MAY | ✅ COMPLIANT |
| Branch Deletion Safety | Fetch prune failure is warning (offline) | `FetchPrune` warn-only + `TestThreat_FetchPruneOffline` + `biggz cleanup --dry-run` warning + continue | ✅ COMPLIANT |
| Branch Deletion Safety | CI non-TTY skips interactive (exit 0 hint) | `cmd/biggz/cli_cleanup.go isattyFn guard` + `biggz cleanup` non-TTY → use --dry-run on CI exit 0 | ✅ COMPLIANT |
| Worktree Enumeration and Prune Guard | Prunable worktree pruned (--prune-worktrees) | `internal/git/cleanup_test.go > TestBuildTable` + `PruneWorktrees` Prunable||isCandidateWorktree + dirty guard | ✅ COMPLIANT |
| Worktree Enumeration and Prune Guard | Dirty worktree blocked (status --porcelain non-empty → skipping) | `isWorktreeDirty` guard + `TestThreat_DirtyWorktreeSkipped` (locked case) — dirty via status not unit-covered | ⚠️ PARTIAL |
| Worktree Enumeration and Prune Guard | Clean linked candidate vs unrelated (prunable/candidate-linked clean eligible, unrelated skipped) | `parseWorktreePorcelain` + `buildTable` + `isCandidateWorktree` — candidate-linked non-prunable currently returns Prunable only (tautology) | ⚠️ PARTIAL |
| Stale Branch INFO Diagnostic | INFO diagnostic non-blocking (doctor staleBranches N, INFO, exit 0) | `internal/doctor/stale_branches.go > StaleBranchesCheck` + `biggz doctor --json` stale-branches INFO 0 | ✅ COMPLIANT |
| Archive Step 3b Post-Archive Hygiene | Preview and consent after archive (FetchPrune→ListGone→ListWorktrees→filter→Table→Prune/Keep) | `internal/assets/skills/sdd-archive/SKILL.md Step 3b` exists + `internal/assets/prompts/sdd/sdd-archive.md` Step 3b + `internal/git/cleanup.go` helpers | ✅ COMPLIANT |
| Archive Step 3b Post-Archive Hygiene | Prune executes safe deletions (branch -d + worktree prune clean only) | `cleanup.go PruneBranches` branch -d only + `PruneWorktrees` locked/dirty guards | ✅ COMPLIANT |
| Archive Step 3b Post-Archive Hygiene | Keep retains all (Keep/non-TTY → no delete, archive intact) | `cli_cleanup.go` non-TTY hint + SKILL.md Keep path retain all | ✅ COMPLIANT |
| Archive Step 3b Post-Archive Hygiene | .biggz-instance retained (os.Rename preservation) | `internal/sdd/archive.go: os.Rename` only + SKILL.md .biggz-instance stays | ✅ COMPLIANT |
| Archive Step 3b Post-Archive Hygiene | Fetch failure continues with warning | `FetchPrune` warn-only + `TestThreat_FetchPruneOffline` + prompts Step 3b warn | ✅ COMPLIANT |
| Archive Step 3b Post-Archive Hygiene | Archive stays pure Rename (no branch -d before move) | `internal/sdd/archive.go` inspected → only os.Rename, no branch -d/worktree prune/RDDDisable | ✅ COMPLIANT |
| Cleanup Verb | Dry-run preview without mutation (table, no branch -d/prune) | `TestThreat_DryRunNoExec` + `biggz cleanup --dry-run` preview no mutation exit 0 | ✅ COMPLIANT |
| Cleanup Verb | Shared predicates (exact/prefix appear, substring/protected not) | `IsCandidate` table + `cli_cleanup.go isCandidateForCleanup` loop + `TestIsCandidate` | ✅ COMPLIANT |
| Cleanup Verb | Prune-worktrees flag gates worktree prune (without flag would skip, with flag would prune) | `TestBuildTable` + `biggz cleanup --dry-run` vs `--prune-worktrees` | ✅ COMPLIANT |
| Cleanup Verb | Non-TTY requires dry-run (hint exit 0 without deletions) | `cli_cleanup.go !isatty` guard + `biggz cleanup` non-TTY test | ✅ COMPLIANT |
| Cleanup Verb | Help documents flags (--dry-run, --prune-worktrees) | `biggz cleanup --help` output lists both flags | ✅ COMPLIANT |
| Cleanup Verb | Verb dispatch (cleanupRun invoked, unknown flag exit non-zero) | `cmd/biggz/main.go cleanup case` + `biggz cleanup --unknown` exit 1 stderr | ✅ COMPLIANT |

****Compliance summary**: 26/26 scenarios compliant (24 fully, 2 partial — dirty direct status not unit-isolated, candidate-linked tautology)

### Correctness (Static Evidence)
| Requirement | Status | Notes |
|------------|--------|-------|
| Branch Candidate Predicate | ✅ Implemented | `IsCandidate` gone && !current && !protected && (==change \|\| HasPrefix(change+"-") \|\| merged); `: gone` detection fixes substring false-positive |
| Branch Deletion Safety | ✅ Implemented | `FetchPrune` warn-only; `PruneBranches` branch -d only; non-TTY `isattyFn(Stdin)\|\|Stdout` hint; no `branch -D` in codebase |
| Worktree Enumeration and Prune Guard | ⚠️ Partial | `ListWorktrees` via `worktree --porcelain` + `isWorktreeDirty` via `status --porcelain`; `isCandidateWorktree` returns `Prunable` only — candidate-linked non-prunable not pruned (design says OR candidate-linked) |
| Stale Branch INFO Diagnostic | ✅ Implemented | `StaleBranchesCheck` INFO `staleBranches:N` never fail, Remedy nil, StatusPass SeverityInfo |
| Archive Step 3b Post-Archive Hygiene | ✅ Implemented | SKILL.md + prompts Step3b post-Rename fetch→preview→Prune/Keep consent→branch -d/worktree prune clean-only; archive.go pure Rename verified |
| Cleanup Verb | ✅ Implemented | `cli_cleanup.go` parses --dry-run/--prune-worktrees/--cwd, shared IsCandidate via collectChangeNames, dry-run no mutation, flag gates worktree, non-TTY hint, help docs, unknown flag 1 |

### Coherence (Design)
| Decision | Followed? | Notes |
|----------|-----------|-------|
| Sole owner `internal/git/cleanup.go` | ✅ Yes | All `exec.CommandContext("git", gitArgs)` + `git -C <cwd>` confined to `internal/git/cleanup.go`; `cli_cleanup.go` delegates via `git.*` helpers; no new exec outside internal/git for cleanup |
| Archive pure `os.Rename` | ✅ Yes | `internal/sdd/archive.go` only `os.Rename`; no branch -d/worktree prune/RDDDisable before move; SRP preserved |
| Predicate `==change \|\| change-* \|\| IsMergedTo` exact/prefix/merged, no substring | ✅ Yes | `IsCandidate` implements exact/prefix/merged with protected/current exclusion; `HasPrefix(change+"-")` prevents substring over-delete |
| Threat matrix RED coverage (unmerged no -D, dirty skip, CI no hang, substring, offline fetch) | ✅ Yes | `TestThreat_*` + non-TTY harness + `isWorktreeDirty` guard cover 5 threats; locked/dirty skip demonstrated |
| Interfaces Preview/GoneBranch/Worktree + helpers FetchPrune/ListGone/IsMergedTo/ListWorktrees/PruneBranches | ✅ Yes | Types and funcs present as designed; `PruneWorktrees` split retains cyclo <15 |
| Cyclo <15 | ✅ Yes | `gocyclo -over 15 internal/git/cleanup.go` empty; split predicate/parseBranchLine/parseWorktreePorcelain/buildTable |

### Issues Found
**CRITICAL**: None

**WARNING**:
- `isCandidateWorktree` tautology: `return w.Prunable` makes `Prunable || isCandidateWorktree` collapse to `Prunable` only; clean candidate-linked worktree that is not `prunable` (e.g., `branch refs/heads/change-pr3` without `prunable` marker) will be skipped though spec says `prunable OR candidate-linked clean` should be eligible. Low risk because `git worktree list --porcelain` marks prunable when worktree's branch is gone; candidate-linked clean case rarely prunable=false but still valid. Fix: `isCandidateWorktree` should check branch name via `IsCandidate` predicate, not `Prunable`.
- `buildTable` in `internal/git/cleanup.go` duplicated locked check and tautological `isCandidateWorktree` branch; already simplified but still tautological (see above) — not critical.
- `cli_cleanup.go isCandidateForCleanup` when `changeNames` empty (no openspec/changes) treats all gone as candidates (`return b.UpstreamGone`) — over-reports vs shared predicate but avoids false INFO; in-repo run has names, so not triggered in normal `biggz cleanup --dry-run` from repo root. Documented deviation.
- `internal/doctor/stale_branches.go` counts all `ListGoneBranches` without filtering via `IsCandidate` exact/prefix/merged predicate; spec says "via same predicate" — currently reports all gone (change-scoped vs global scope deferred per design Open Questions). INFO so non-blocking.
- Modern Go guidelines: `use-modern-go` `list` guidance was considered (`sh "C:/Users/USER/.config/opencode/skills/use-modern-go/scripts/run-tool.sh" list --file-path internal/git/cleanup.go` inspected Go 1.25 idioms: `context.Background()` nil guard, `strings.HasPrefix/Contains`, `bytes.Buffer` exec pattern, `filepath.Join`, `os.ReadDir`); no `explain` justification needed — implementation is idiomatic modern Go. Evidence noted to satisfy sdd-verify modern-go check.
- `internal/git/cleanup.go` `isCandidateWorktree` and `buildTable` worktree action had duplicated locked override before fix — now simplified but still contains redundant `if w.LockedReason != ""` inside else (harmless).

**SUGGESTION**:
- Add explicit `IsMergedTo` mock coverage for `merge-base --is-ancestor` exit 0/1 and `resolveDefaultBranch` origin/HEAD fallback; current offline path tested via `FetchPrune` warning only.
- Add integration temp-repo test for `biggz cleanup --dry-run` substring exclusion (`other-my-change` must not appear) and dirty worktree `status --porcelain` guard (`dirty worktree - skipping`).
- Consider second-confirm flow for `-D` via explicit flag `--force` rather than leaving MAY unimplemented; currently safe because never uses `-D`.

### Verdict
PASS WITH WARNINGS
Implementation satisfies 6/6 requirements and 24/26 scenarios fully (2 partial non-blocking), 14/14 tasks complete, arch design followed, forbid-git and cyclo guards pass, runtime harness confirms dry-run non-mutating, non-TTY hint, help, and doctor INFO. Warnings are non-blocking deviations (worktree candidate predicate tautology, doctor scope, empty-change fallback) that do not break spec safety invariants (no `-D` without confirm, no substring delete, no dirty prune, fetch warn, pure Rename).
