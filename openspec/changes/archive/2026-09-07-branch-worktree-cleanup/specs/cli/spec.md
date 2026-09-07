# Delta for cli

## ADDED Requirements

### Requirement: Cleanup Verb

The system MUST dispatch `biggz cleanup [--dry-run] [--prune-worktrees] [--cwd <path>]` via the switch router, reuse `internal/git` candidate predicates, render `--dry-run` as preview table without mutation, and require consent or `--dry-run` on non-TTY.

#### Scenario: Dry-run preview without mutation

- GIVEN `biggz cleanup --dry-run` with candidates present
- WHEN executed
- THEN table of branches/worktrees matching exact/prefix/merged predicate MUST be printed and no `branch -d` or `worktree prune` MUST execute

#### Scenario: Shared predicates

- GIVEN branches `my-change` (exact), `my-change-pr1` (prefix), `other-my-change` (substring), `main` (protected)
- WHEN `biggz cleanup` evaluates (dry-run or real)
- THEN first two MUST appear, last two MUST NOT appear

#### Scenario: Prune-worktrees flag gates worktree prune

- GIVEN `biggz cleanup --dry-run` without `--prune-worktrees` vs with it
- WHEN preview rendered
- THEN without flag worktrees MUST show `would skip (use --prune-worktrees)`; with flag eligible clean worktrees MUST show `would prune`

#### Scenario: Non-TTY requires dry-run

- GIVEN non-TTY and `biggz cleanup` without `--dry-run`
- WHEN invoked
- THEN system MUST print hint `use --dry-run on CI` and exit 0 without deletions

#### Scenario: Help documents flags

- GIVEN `biggz cleanup --help`
- WHEN help renders
- THEN output MUST list `--dry-run` (preview without mutation) and `--prune-worktrees` (prune eligible clean worktrees)

#### Scenario: Verb dispatch

- GIVEN CLI invoked as `biggz cleanup`
- WHEN router dispatches
- THEN `cleanupRun` MUST be invoked and unknown flag MUST exit non-zero with error to stderr
