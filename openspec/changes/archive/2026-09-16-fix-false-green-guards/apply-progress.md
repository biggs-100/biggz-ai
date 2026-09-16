## SDD fix-false-green-guards — apply-progress (Phase 7 / PR6 — final wave, debt zero, 2026-09-16)

Status: Phase 7 COMPLETE (7.1, 7.2, 7.3). PR #99 OPEN against master (branch fix/final-wave-debt-zero, 6 commits, base 2b2daba4). tasks.md is the authoritative ledger and marks Phase 7 done.
This save MERGES with the prior batches: 5a (PR #97 merged) and 5b (PR #98) are kept condensed below; nothing overwritten.

### Batch 7 (PR #99) — the last 13 go-git-spawn sites -> internal/git; debt 0
Completed: 7.1, 7.2, 7.3 (full). Commits (each deletes its baseline entries in the same commit):
- db1bd0ff refactor(release): release.go (6 entries) -> git.Run/git.RevParse; Tag failure keeps raw stderr via exitErr.Stderr fallback (TestTagFailureOutputStaysRaw, TestCheckGitStateInTempRepo).
- 16209436 refactor(install): install.go (2 entries) -> git.ResolveGitDirs dedupes the rev-parse pair; tolerance preserved (failure -> both dirs empty, install continues to global scope; TestEnsureRDDEnabledToleratesMissingRepository).
- 094f442a refactor(sddattempt): cas_store.go (1 entry) -> git.Run(context.Background(), repoRoot, "rev-parse", "--git-common-dir"); cmd.Dir replaces -C; raw stderr classification intact (TestResolveCloneStoreNotGitRepoStaysClassified).
- fa254aa3 refactor(project): detect.go (2 entries) -> git.TopLevel(ctx, dir) + git.Run(ctx, dir, "remote", "get-url", "origin"), explicit repo=dir never caller cwd (TM-2); dead newProjectCommandContext removed; TestDetectGitHelpers_EmptyOnNonRepo + TestDetectGitHelpers_ExplicitDirBeatsCallerCwd.
- 0717d7f1 refactor(sdd): session_guard.go (1 entry) -> GitLogFallback uses git.Run(ctx, workspaceRoot, "log", "--oneline", "-15"); TestGitLogFallbackAnchorsToWorkspaceRoot; TestSessionGuard_EmptyFallbackGitLog now observes only the sdd-status probe via execCommand (git half no longer mockable, pinned against a real repository).
- 7196c364 refactor(doctor): git.go dead probeCmd/EnsureCommandDir pair removed (1 entry); declared-boundary line refreshed 76->69; the 5 doctor boundary entries stay live.
Baseline end state: exactly 7 declared-boundary lines (biggz-footer.js 422, codegraph-tools.ts 80, doctor/git.go 69, doctor/review.go 82/93/251, doctor/version.go 86).
- Evidence: focused `go test ./internal/release/... ./internal/install/... ./internal/sddattempt/... ./internal/project/... ./internal/sdd/... ./internal/doctor/... -count=1` -> all ok; harness `go build -o /tmp/gitexec.exe ./tools/gitexec/cmd/gitexec && /tmp/gitexec.exe -root .` -> exit 0, "self-check 9 sites, 351 .go + 17 host targets, 0 debt, 7 boundary, baseline matched" (twice); `-update` used exactly once after all sites migrated, file verified = 7 lines; gofmt clean; go vet OK on touched packages.
- Budget: 331 changed lines (256 add + 75 del, 12 files) < 400. No internal/git surface added (Run/RevParse/TopLevel/ResolveGitDirs only).
- Rollback boundary: revert PR #99 as a unit — sites + their baseline entries revert together; doctor execFn boundary seams untouched.
- Scope note: the three sites not named in Phase 7 (detect.go 357/416, session_guard.go 353) were assigned to this unit by the maintainer so 7.3 (debt=0) is reachable; recorded in the unit's launch context.

### Prior batches (recap, unchanged)
- Batch 5a (PR #97, MERGED): 6.1-6.3 + pr.go/cli_util half of 6.4; TM-3/4/5 goldens; 18 debt, 7 boundary; budget 361 lines.
- Batch 5b (PR #98): five sites (cli_bigmem x2, cli_codegraph, export, sdd_new) -> git.TopLevel/git.Run; 18->13 debt; budget 142 lines; commit 55ea1cc9.

### Next Recommended
Phase 8 (verification): tasks.md 8.1-8.2 — full `go test ./... -count=1 -timeout 180s`, `go vet`, `gofmt -l`; CI once (guards observe own status, zero rg, no `|| true`); violation fixture fails CI, allowlisted tree passes. Then sdd-verify, then PR6 merge (human decision).
