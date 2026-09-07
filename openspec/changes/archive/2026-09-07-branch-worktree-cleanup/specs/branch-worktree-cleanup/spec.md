# Branch Worktree Cleanup Specification

## Purpose

Local hygiene for branches/worktrees after archive. Predicates, guards, and prune rules shared by archive Step 3b and `biggz cleanup` via `internal/git`.

## Requirements

### Requirement: Branch Candidate Predicate

The system MUST candidate a `[gone]` branch iff name is `==change` OR `change-*` OR `IsMergedTo(default)`; MUST exclude `master`/`main`/`HEAD`/current and MUST NOT match substring.

#### Scenario: Exact name qualifies

- GIVEN branch `tui-installer-pipeline` with `[gone]` and change `tui-installer-pipeline`
- WHEN predicate evaluated
- THEN it MUST be a candidate

#### Scenario: Prefix qualifies

- GIVEN branch `tui-installer-pipeline-pr2` with `[gone]` and change `tui-installer-pipeline`
- WHEN predicate evaluated
- THEN it MUST be a candidate

#### Scenario: Substring without prefix excluded

- GIVEN branch `my-tui-installer-pipeline-fix` with `[gone]` and change `tui-installer-pipeline`
- WHEN predicate evaluated
- THEN it MUST NOT be a candidate

#### Scenario: Fully-merged gone qualifies

- GIVEN branch `fix-quiet-xyz` with `[gone]` and `merge-base --is-ancestor` true to `origin/main`
- WHEN predicate evaluated
- THEN it MUST be a candidate even without name match

#### Scenario: Protected branches never candidates

- GIVEN branch `main`, `master`, `HEAD`, or currently checked-out with `[gone]`
- WHEN predicate evaluated
- THEN it MUST NOT be a candidate

### Requirement: Branch Deletion Safety

The system MUST delete via `branch -d` only, MUST NOT use `-D` without second explicit confirm, MUST warn on `fetch --prune` failure, and MUST skip prompts on non-TTY.

#### Scenario: Safe delete via -d

- GIVEN candidate is fully merged
- WHEN deletion runs
- THEN `git branch -d <name>` MUST be used and succeed

#### Scenario: Unmerged blocks without second confirm

- GIVEN candidate is not merged
- WHEN deletion attempted without second confirm
- THEN `branch -d` MUST fail and system MUST NOT retry with `-D`

#### Scenario: Second confirm allows force

- GIVEN candidate not merged and user gave second explicit confirm
- WHEN forced deletion runs
- THEN `git branch -D <name>` MAY be used

#### Scenario: Fetch prune failure is warning

- GIVEN `git fetch --prune` fails (offline)
- WHEN preview starts
- THEN system MUST log warning and continue to preview

#### Scenario: CI non-TTY skips interactive

- GIVEN stdin/stdout is non-TTY (CI)
- WHEN archive Step 3b or `biggz cleanup` without `--dry-run` would prompt
- THEN system MUST skip prompt, delete nothing, exit 0 with hint

### Requirement: Worktree Enumeration and Prune Guard

The system MUST list via `worktree list --porcelain`, MUST prune only `prunable` or candidate-linked clean (`status --porcelain` empty), and MUST block dirty worktrees.

#### Scenario: Prunable worktree pruned

- GIVEN worktree `prunable` at `/tmp/wt` linked to candidate branch
- WHEN prune runs with `--prune-worktrees` or Step 3b consent
- THEN `git worktree prune` MUST remove it

#### Scenario: Dirty worktree blocked

- GIVEN worktree path has `git status --porcelain` non-empty
- WHEN prune evaluated
- THEN system MUST block prune and report `dirty worktree - skipping`

#### Scenario: Clean linked candidate vs unrelated

- GIVEN worktree linked to `change-pr3` (candidate, clean) vs `feature/other` (not candidate)
- WHEN evaluated
- THEN first MUST be eligible, second MUST be skipped

### Requirement: Stale Branch INFO Diagnostic

The system MAY expose `doctor` INFO for stale `[gone]` branches via same predicate; MUST be `INFO`, MUST NOT be `fail` nor block phases.

#### Scenario: INFO diagnostic non-blocking

- GIVEN stale `[gone]` branches exist
- WHEN `biggz doctor` runs
- THEN diagnostic MUST report `staleBranches: N` with `INFO` and exit 0
