# Delta for tui-sanitize

## MODIFIED Requirements

### Requirement: Git Wrapper — Single Owner

The system MUST provide `internal/git` as the sole owner of git process spawns in Go code, exposing `GitStatus`/`GitDiff` (migrating `status.go:detectGitDirs`) plus the surface other packages need so that none of them spawns git directly: a generic runner `Run(ctx, dir, args…)` returning byte-identical stdout and git's unmodified stderr, canonical absolute resolvers (`TopLevel`, `ResolveGitDirs`), and an env/limits-aware constructor for the isolated review runner (stripped `GIT_*`, `LANG=C`, byte caps, `--no-pager`). MUST preserve `os.IsNotExist` handling (not panic). All other Go code MUST NOT call `exec.Command`/`exec.CommandContext` with git and MUST NOT pass a literal `"git"` in command position to any helper. Non-Go host spawns are outside this invariant and are declared in the guard baseline.
(Previously: the wrapper exposed only `GitStatus`/`GitDiff` behind an unexported runner, so 66 spawn sites re-implemented the same primitives outside it.)

#### Scenario: Git missing handled

- GIVEN `git` not on PATH (`IsNotExist`)
- WHEN `GitStatus` called
- THEN MUST return handled error, not panic

#### Scenario: Preserves detectGitDirs semantics

- GIVEN worktree where `git rev-parse --git-common-dir` → `/repo/.git`
- WHEN `DetectGitDirs()` called
- THEN MUST return trimmed `commonDir`/`gitDir` identical to prior inline logic

#### Scenario: Generic runner preserves raw output and error

- GIVEN a caller that classifies "not a git repository" by git's stderr wording
- WHEN it runs the call through `git.Run(ctx, dir, …)`
- THEN stdout MUST be byte-identical and the error MUST carry git's unmodified stderr text

#### Scenario: Env-controlled spawn stays in the wrapper

- GIVEN the review frozen-inspector needs `GIT_*` stripped, `LANG=C` and byte caps
- WHEN it spawns git
- THEN the command MUST come from `internal/git`'s env/limits-aware constructor and a golden test MUST assert its resulting environment and args

### Requirement: CI Forbid Git Exec Outside Wrapper

CI (`.github/workflows/ci.yml`) MUST hard-fail through the compiled checker defined by `ci-guard-integrity` when a git spawn site exists outside `internal/git` and is not in the ratchet baseline. The step MUST observe the checker's own exit status — no pipeline whose status belongs to a later command, no `2>/dev/null` hiding a failing checker, no warn-only path. A checker that is absent, unbuildable, or exits non-zero for any reason MUST fail the step.
(Previously: the step was `rg … 2>/dev/null | grep -q .`; on a runner without `rg` the shell's 127 was swallowed, `grep -q .` saw empty stdin, and the step reported success.)

#### Scenario: Violation fails CI

- GIVEN `internal/tui/screens/foo.go` contains `exec.Command("git","status")`
- WHEN CI forbid step runs
- THEN MUST exit non-zero reporting file

#### Scenario: Allowlisted passes

- GIVEN only `internal/git/git.go` contains `exec.Command("git",`
- WHEN CI step runs
- THEN MUST exit zero

#### Scenario: Indirected write op fails CI

- GIVEN `cmd/biggz/pr.go` passes `"git"` to `runCmd` for `add`/`commit`/`push`
- WHEN CI forbid step runs
- THEN MUST exit non-zero naming that file:line

#### Scenario: Absent checker cannot report success

- GIVEN the checker binary is absent from the runner or fails to build
- WHEN the forbid step runs
- THEN the step MUST exit non-zero and MUST NOT print "No forbidden git exec found"
