# Design: branch-worktree-cleanup

## Technical Approach

Post-`os.Rename` hygiene + `biggz cleanup`. Owner `internal/git/cleanup.go` parses `branch -vv`/`worktree --porcelain`/`merge-base`; exact/prefix/merged predicate. `fetch --prune` (warn) → `ListGoneBranches` → filter → preview → consent → `branch -d`/`prune`. `ArchiveChange` pure `os.Rename`; non-TTY skip.

## Architecture Decisions

| Decision | Options | Tradeoff | Choice |
|----------|---------|----------|--------|
| **Sole owner `internal/git/cleanup.go`** | A) `internal/git` B) `internal/sdd`/`cmd/biggz` C) new `internal/cleanup` | A enforces `forbid-git` (only `internal/git` execs git), cyclo<15; B violates ownership; C fragments git | **A** — all `exec.Command("git")` + parsing in `cleanup.go` |
| **Archive pure `os.Rename`** | A) extend `ArchiveChange` B) skill post-move | A couples VCS to archival, breaks `Archive Never Auto-Disable`; B keeps SRP + testability | **B** — `archive.go` unchanged; SKILL.md Step 3b invokes helpers after move |
| **Predicate `==change\|\|change-*\|\|IsMergedTo`** | A) substring/regex B) exact+prefix+merged | A over-deletes `my-change-fix`; B covers stacked `change-pr1..5` + merged-gone, no collision | **B** — `name==change \|\| HasPrefix(change+"-") \|\| merged`; `main/master/HEAD/current` excluded |

## Data Flow

```
ArchiveChange --os.Rename--> FetchPrune() --warn--> ListGoneBranches() --branch -vv--> predicate filter
                                                            │
                              ListWorktrees() --porcelain--> PruneBranches(dryRun) --> preview table --> consent?
                                                            │                              │
                                                     Prune? --> branch -d / worktree prune --> doctor INFO
                                                            └-- .biggz-instance stays in archive/YYYY-MM-DD-{change}/
```
Predicate: `gone && !protected && !current && (name==change || HasPrefix(change+"-") || IsMergedTo)`.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/git/cleanup.go` | Create | `FetchPrune`, `ListGoneBranches`, `IsMergedTo`, `ListWorktrees`, `PruneBranches`; parsers; cyclo<15 |
| `internal/sdd/archive.go` | No change | Pure `os.Rename` |
| `internal/assets/skills/sdd-archive/SKILL.md` | Modify | Step 3b: preview→`Prune/Keep`→`branch -d`/`prune`; non-TTY skip |
| `internal/assets/prompts/sdd/sdd-archive.md` | Modify | Consent envelope for Step 3b |
| `cmd/biggz/cli_cleanup.go` | Create | `cleanupRun` `--dry-run --prune-worktrees --cwd`; shared predicates |
| `cmd/biggz/main.go` | Modify | Register `cleanup` |
| `internal/doctor/stale_branches.go` | Create | `StaleBranchesCheck` INFO `staleBranches:N`; same predicate |

## Interfaces / Contracts

```go
// internal/git/cleanup.go
type GoneBranch struct { Branch string; UpstreamGone bool; Merged bool }
type Worktree struct { Path, Branch, LockedReason string; Prunable bool }
type Preview struct { Branches []GoneBranch; Worktrees []Worktree; Table string }

func FetchPrune(ctx context.Context, cwd string) error // fetch --prune; warn, never fatal
func ListGoneBranches(ctx context.Context, cwd string) ([]GoneBranch, error) // branch -vv
func IsMergedTo(ctx context.Context, cwd, base, branch string) bool // merge-base --is-ancestor
func ListWorktrees(ctx context.Context, cwd string) ([]Worktree, error) // worktree --porcelain + status
func PruneBranches(ctx context.Context, cwd string, branches []GoneBranch, dryRun bool) (Preview, error)
func IsCandidate(name, change string, gone, merged, isCurrent, protected bool) bool // pure predicate
```
CLI dry-run no mutation; non-TTY exits 0 hint.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|--------------|----------|
| Unit | `IsCandidate` exact/prefix/merged | Table-driven, no git |
| Unit | `ListGoneBranches` | Golden `branch -vv` |
| Unit | `IsMergedTo` | Mock `merge-base` 0/1 |
| Unit | Dry-run preview | Assert `Table`, no exec |
| Integration | `cleanup --dry-run` | Temp repo `[gone]`; no mutation |
| Integration | Archive Step 3b | Archive→preview; `.biggz-instance` ok |
| E2E | Non-TTY/fetch-fail | `isattyFn=false`→warn |

Cyclo<15; split `predicate`/`parseBranchLine`/`parseWorktreePorcelain`.

## Threat Matrix

`references/threat-matrix.md`:

| Boundary | Cases | Applicability | Response | RED test |
|----------|-------|---------------|----------|----------|
| Doc-like paths | `requirements.txt`, MDX | N/A — no file classification | — | — |
| Git repo selection | `git -C`, rel/abs | **Applicable** — `cwd` vs root | `cwd` param, `EnsureCommandDir` | RED: wrong cwd → correct `[gone]` |
| Commit state | staged, `commit -a` | N/A — no commit | — | — |
| Push state | tracking, refspec | N/A — no push | — | — |
| PR commands | `--head`, env | N/A — no PR | — | — |

Domain safety (propagate to tasks):

| Threat | Safe / Failure | RED test |
|--------|----------------|----------|
| Unmerged delete | `branch -d` fails; no `-D` w/o 2nd confirm | Unmerged fixture → no `-D` exec |
| Dirty worktree | `porcelain != ""` → `dirty - skipping` | Dirty wt → pruned 0 |
| CI hang | non-TTY skip, exit 0 + hint | `isatty=false` → no prompt |
| Name collision | `my-change-fix` excluded | Substring not candidate |
| Offline fetch | warn, continue preview | Stub fetch err → preview ok |

## Migration / Rollout

No migration. Additive; rollback via reflog/SHA logged in `Preview`; delete `cleanup.go`/`cli_cleanup.go`/`stale_branches.go`, revert Step 3b.

## Open Questions

- [ ] Doctor INFO: all `[gone]` vs change-scoped? Default change-scoped if inferable else all.
- [ ] Default branch for `IsMergedTo`: `origin/HEAD`→`origin/main` fallback?
