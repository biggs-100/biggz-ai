# Tasks: branch-worktree-cleanup

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 180–260 |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | single PR |
| Delivery strategy | auto-chain |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Hygiene e2e: `cleanup.go`+`doctor`+`biggz cleanup`+Step3b | PR1 single | `go test ./internal/git -run TestIsCandidate` `go vet ./...` | `biggz cleanup --dry-run --cwd /tmp/repo`; `biggz doctor --json`; Step3b archive test | Revert 3 new files + Step3b docs; `archive.go` stays `os.Rename` |

## Phase 1: Foundation

- [x] 1.1 Create `internal/git/cleanup.go` types `GoneBranch/Worktree/Preview` + stubs `FetchPrune/ListGoneBranches/IsMergedTo/ListWorktrees/PruneBranches/IsCandidate`
- [x] 1.2 Parsers `parseBranchLine` (`branch -vv`→`[gone]`) + `parseWorktreePorcelain` golden fixtures
- [x] 1.3 `IsCandidate` predicate [Req Candidate: GIVEN `==change`/prefix `change-pr2`/merged WHEN predicate THEN candidate; GIVEN `my-change-fix`/protected `main/HEAD/current` WHEN predicate THEN not]

## Phase 2: Core

- [x] 2.1 `FetchPrune(ctx,cwd)` `git -C <cwd> fetch --prune` warn-only [Req Safety+Step3b: GIVEN fetch fails WHEN preview THEN warn+continue]
- [x] 2.2 `ListGoneBranches` (`branch -vv`) + `IsMergedTo` (`merge-base --is-ancestor`) + `ListWorktrees` (`worktree --porcelain`) via `git -C <cwd>` [Threat cwd RED: wrong cwd → wrong `[gone]`]
- [x] 2.3 `PruneBranches` preview `Table`; `dryRun` no exec; real `branch -d` only, dirty `status --porcelain!=""→skip`, no `-D` w/o 2nd confirm [Req Safety: merged→`-d` ok; unmerged→fail no `-D`; 2nd→`-D` | Req Worktree: prunable/clean→prune; dirty→`dirty - skipping`; unrelated→skip]

## Phase 3: Integration

- [x] 3.1 `internal/doctor/stale_branches.go` `StaleBranchesCheck` INFO `staleBranches:N` never fail `Remedy=nil` [Req INFO: GIVEN stale WHEN `biggz doctor` THEN INFO exit 0]
- [x] 3.2 `cmd/biggz/cli_cleanup.go` `cleanupRun --dry-run --prune-worktrees --cwd` shared predicates, dry-run no mutation, flag gates worktree, non-TTY→hint [Req Cleanup: GIVEN `--dry-run` WHEN run THEN table no delete; GIVEN non-TTY WHEN without dry-run THEN hint exit 0]
- [x] 3.3 `internal/assets/skills/sdd-archive/SKILL.md` Step3b after `ArchiveChange` pure `os.Rename`: `fetch→preview→Prune/Keep` consent→Prune does `branch -d`+`prune` clean only non-TTY skip; update `internal/assets/prompts/sdd/sdd-archive.md` [Req Step3b: GIVEN moved WHEN TTY THEN preview; GIVEN Keep/non-TTY WHEN done THEN no delete; GIVEN `.biggz-instance` WHEN `os.Rename` THEN kept]

## Phase 4: Testing & Verification

- [x] 4.1 Unit RED `IsCandidate` 5 cases + `parseBranchLine` golden + `IsMergedTo` mock + dry-run `Table` [Evidence: `go test ./internal/git -run TestIsCandidate|TestParse -count=1`]
- [x] 4.2 Threat RED unmerged no `-D` / dirty `pruned 0` / `isatty=false` no prompt / `my-change-fix` excluded / offline fetch ok [Evidence: `go test ./internal/git ./internal/doctor -run TestThreat -count=1`]
- [x] 4.3 Integration temp repo `[gone]` candidates vs substring/main; `biggz cleanup --dry-run` listing + `--prune-worktrees` gating; Step3b `.biggz-instance` kept [Evidence: `go test -tags=integration ./...` + `biggz cleanup --dry-run --cwd /tmp/biggz-test`]
- [x] 4.4 Guards `forbid-git` only `internal/git`, `gocyclo -over 15` empty, `go vet ./...`, `biggz doctor --json` INFO [Evidence: `grep -R exec.Command.*git --include=*.go` + `gocyclo -over 15 internal/git/cleanup.go`]

## Phase 5: Cleanup

- [x] 5.1 `go fmt ./...` cleanup `wc -w` ≤530 [Evidence: `wc -w openspec/changes/branch-worktree-cleanup/tasks.md`]
