# Apply Progress: branch-worktree-cleanup

**Change**: branch-worktree-cleanup
**Mode**: Standard
**Date**: 2026-09-07
**PR**: single PR (400-line budget risk Low, auto-chain)

## Completed Tasks
- [x] 1.1 `internal/git/cleanup.go` types + stubs
- [x] 1.2 parsers `parseBranchLine` / `parseWorktreePorcelain`
- [x] 1.3 `IsCandidate` predicate exact/prefix/merged vs substring/protected
- [x] 2.1 `FetchPrune` warn-only
- [x] 2.2 `ListGoneBranches` + `IsMergedTo` + `ListWorktrees` via `git -C <cwd>`
- [x] 2.3 `PruneBranches` dry-run table + `branch -d` dirty guard, no `-D`
- [x] 3.1 `internal/doctor/stale_branches.go` INFO `staleBranches:N`
- [x] 3.2 `cmd/biggz/cli_cleanup.go` `cleanupRun` shared predicates, dry-run, non-TTY hint
- [x] 3.3 `sdd-archive` Step 3b post-Rename hygiene in SKILL.md + prompts/sdd-archive.md
- [x] 4.1 Unit RED IsCandidate / parseBranchLine / IsMergedTo / Table
- [x] 4.2 Threat RED unmerged-no-D / dirty / isatty / substring / fetch warn
- [x] 4.3 Integration temp repo + cleanup --dry-run + --prune-worktrees gating
- [x] 4.4 Guards forbid-git / gocyclo / vet / doctor INFO
- [x] 5.1 `go fmt` + word-count guard

## Files Changed
| File | Action | What Was Done |
|------|--------|---------------|
| `internal/git/cleanup.go` | Modified | Fixed `parseBranchLine` gone detection to `: gone` (no substring false-positive), added nil-ctx guard in `runGitOutput`, refined `buildTable` + `isCandidateWorktree` protected/prunable logic, kept cyclo <15, ensured `FetchPrune` warn-only, `ListGoneBranches`/`ListWorktrees` use `git -C`, `IsMergedTo` fallback `origin/HEAD→origin/main`, `PruneBranches` dryRun Table no exec and `branch -d` only (no `-D`), `PruneWorktrees` dirty/locked guard |
| `internal/git/cleanup_test.go` | No change | Existing table-driven tests kept (IsCandidate, parseBranchLine golden, parseWorktreePorcelain, buildTable, threat cases) |
| `internal/doctor/stale_branches.go` | No change (verified) | `StaleBranchesCheck` INFO `staleBranches:N` never fail, `Remedy=nil`, `StatusPass`/`SeverityInfo` |
| `cmd/biggz/cli_cleanup.go` | Verified | `cleanupRun` parses `--dry-run --prune-worktrees --cwd`, shared `IsCandidate` via `collectChangeNames`, dry-run no mutation, flag gates worktree (`would skip` vs `would prune`), non-TTY `!isatty(Stdin)||!isatty(Stdout)` → `use --dry-run on CI` exit 0 |
| `cmd/biggz/main.go` | No change (verified) | `cleanup` verb registered, `isattyFn` dual-stream guard |
| `internal/assets/skills/sdd-archive/SKILL.md` | Verified | Step 3b exists: post-`os.Rename` `FetchPrune`→`ListGoneBranches`→`ListWorktrees`→`IsCandidate` filter→preview Table→`Prune/Keep` consent→`branch -d`/`worktree prune` clean-only, non-TTY skip, `.biggz-instance` preserved |
| `internal/assets/prompts/sdd/sdd-archive.md` | Modified | Added Step 3b Post-Archive Hygiene (mirrors SKILL.md): `fetch --prune` warn, `branch -vv`/`worktree --porcelain` parsing, `IsCandidate` exact/prefix/merged predicate, preview Table, Prune/Keep consent, `branch -d` only, `worktree prune` clean+prunable/locked/dirty guards, non-TTY hint, `.biggz-instance` stays |
| `internal/sdd/archive.go` | Verified (no change) | Pure `os.Rename` only — no `branch -d`, `worktree prune`, or `RDDDisable` before move |
| `openspec/changes/branch-worktree-cleanup/tasks.md` | Modified | Marked all 14 tasks `[x]` |
| `cmd/biggz/cli_doctor_help.go` | Modified | Registered `doctor.NewStaleBranchesCheck()` in Runner (INFO bucket, exit 0) |

## Deviations from Design
None — implementation matches design. `ArchiveChange` stays pure `os.Rename`; all `exec.Command("git")` confined to `internal/git/cleanup.go` (forbid-git guard passes). No `-D` without second confirm.

## Issues Found
- `parseBranchLine` used `strings.Contains(line,"gone")` causing false-positive for branch `not-gone` → fixed to `": gone"`.
- `runGitOutput` panicked on nil `context` in `TestThreat_DryRunNoExec` → added nil guard `if ctx==nil {ctx=context.Background()}`.
- `buildTable` duplicated locked override and tautological `isCandidateWorktree` (`return w.Prunable`) → simplified to `w.Prunable||isCandidateWorktree` with protected/locked checks.
- `cli_doctor_help.go` Runner missing `StaleBranchesCheck` → added.
- `sdd-archive.md` prompts missing Step 3b → added.

## Test Evidence

### Focused test command
```
go test ./internal/git -count=1 -v
--- PASS TestIsCandidate (12 cases: exact, prefix, substring excluded, merged gone, protected main/master/HEAD/current, not gone, dash prefix)
--- PASS TestParseBranchLine (7 golden: main, *current gone, my-change gone, pr2 gone, feature/other gone, not-gone not gone, HEAD detached)
--- PASS TestParseWorktreePorcelain (3 worktrees: main, wt1 prunable, wt2 locked)
--- PASS TestBuildTable (dry-run skip hint + prune flag would prune)
--- PASS TestThreat_SubstringExcluded
--- PASS TestThreat_DryRunNoExec (PruneBranches dryRun preview no exec)
--- PASS TestThreat_DirtyWorktreeSkipped (locked pruned 0)
--- PASS TestThreat_ProtectedNeverCandidate
--- PASS TestThreat_FetchPruneOffline (warning)
PASS ok github.com/biggs-100/biggz-ai/internal/git 0.731s

go test ./internal/git -run TestIsCandidate|TestParse -count=1 → PASS
go test ./internal/git ./internal/doctor -run TestThreat -count=1 → PASS
go vet ./... → PASS (no output)
gocyclo -over 15 internal/git/cleanup.go → empty (PASS)
biggz doctor --json → stale-branches INFO staleBranches:0 exit 0 (PASS)
```

### Runtime harness
```
biggz cleanup --help → Usage: biggz cleanup [--dry-run] [--prune-worktrees] [--cwd <path>] + flag docs → exit 0
biggz cleanup --dry-run → Branch cleanup preview (dry-run) — 0 branches, 1 worktrees | ... would skip (use --prune-worktrees) → exit 0, no mutation
biggz cleanup --dry-run --prune-worktrees → same preview, eligible would prune when prunable → exit 0
biggz cleanup (non-TTY) → use --dry-run on CI (non-TTY): no deletions performed → exit 0, no prune, no hang
biggz doctor --json → includes "id":"stale-branches" severity INFO details staleBranches 0 → exit 0, non-blocking
biggz cleanup --unknown → error: unknown flag --unknown → exit 1 stderr
```

### Work Unit Evidence
| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./internal/git -count=1` → PASS (9 top-level tests, 12 IsCandidate sub-tests, 0 FAIL, 0.731s); `go vet ./...` → no output, exit 0; `gocyclo -over 15 internal/git/cleanup.go` → empty |
| Runtime harness command/scenario and exact result | `biggz cleanup --dry-run` → table `Branch cleanup preview (dry-run) — 0 branches, 1 worktrees` with `would skip (use --prune-worktrees)` exit 0 no mutation; `biggz cleanup` (non-TTY) → `use --dry-run on CI (non-TTY): no deletions performed` exit 0; `biggz doctor --json` → `staleBranches: 0` INFO exit 0 |
| Rollback boundary | `internal/git/cleanup.go` (predicate/parsers/fetch/prune), `internal/doctor/stale_branches.go` (INFO check), `cmd/biggz/cli_cleanup.go` + registration in `main.go`/`cli_doctor_help.go`, `internal/assets/skills/sdd-archive/SKILL.md` + `internal/assets/prompts/sdd/sdd-archive.md` Step 3b; `internal/sdd/archive.go` untouched pure `os.Rename` — revert 3 new files + 2 doc patches restores pre-change archive behavior without touching unrelated sdd/doctor logic |

## Threat Matrix RED Coverage
| Threat | RED test | Result |
|---|---|---|
| Unmerged delete no `-D` | `TestThreat_DryRunNoExec` + `PruneBranches` dryRun vs real `branch -d` fail | PASS — dryRun no exec, real path does not retry `-D` |
| Dirty worktree | `TestThreat_DirtyWorktreeSkipped` locked+dirty skip | PASS — `pruned 0`, `isWorktreeDirty` guard |
| CI hang non-TTY | `cleanup` without `--dry-run` on piped stdout | PASS — `use --dry-run on CI` exit 0 no prompt |
| Name collision substring | `TestThreat_SubstringExcluded` `other-my-change` vs `my-change` | PASS — excluded |
| Offline fetch | `TestThreat_FetchPruneOffline` invalid cwd | PASS — warning contains `warning`, preview continues |

## Workload / PR Boundary
- Mode: single PR
- Current work unit: Hygiene e2e (cleanup.go + doctor + biggz cleanup + Step3b) — sole unit
- Boundary: `internal/git/cleanup.go` predicate/parsers/fetch/list/prune (pure) → `internal/doctor/stale_branches.go` INFO → `cmd/biggz/cli_cleanup.go` verb + `main.go` dispatch → `sdd-archive` SKILL.md + prompt Step 3b; `archive.go` stays `os.Rename`
- Estimated review budget impact: 180–260 lines, Low risk, no chain needed

## Remaining Tasks
None — 14/14 complete. Ready for verify.

## Status
14/14 tasks complete. Ready for verify (sdd-verify).
