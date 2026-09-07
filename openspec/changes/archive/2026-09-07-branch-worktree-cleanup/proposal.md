# Proposal: branch-worktree-cleanup

## Intent
`sdd-archive` does `os.Rename` only — no git cleanup. Stacked PRs left local `[gone]` branches (tui-installer-pipeline pr1..pr5). Add consent-gated prune coupled to archive + standalone `biggz cleanup --dry-run`.

## Scope

### In Scope
- Post-archive: `fetch --prune` preview, delete `[gone]` where `==change || change-* || merged-gone` via `branch -d`; `worktree prune`.
- `internal/git/cleanup.go`: `FetchPrune`, `ListGoneBranches`, `IsMergedTo`, `ListWorktrees`.
- `sdd-archive` Step 3b: dry-run table + `Prune/Keep` consent; skip if non-TTY.
- `biggz cleanup [--dry-run] [--prune-worktrees]` shared predicates.
- Keep `.biggz-instance` inside archive.

### Out of Scope
- Remote/tag/stash delete, `-D` without second confirm, sync auto-delete, retroactive rewrite.

## Capabilities

### New Capabilities
- `branch-worktree-cleanup`: local branch/worktree hygiene.

### Modified Capabilities
- `sdd`: archive Step 3b wiring.
- `cli`: `cleanup` verb.

## Approach
1. `internal/git/cleanup.go` sole git owner; parse `branch -vv`, `worktree list --porcelain`, `merge-base --is-ancestor`.
2. `archive.go` stays pure Rename; skill calls cleanup post-move.
3. Step 3b: preview → consent → `branch -d` + `worktree prune`.
4. `biggz cleanup` reuses helpers; archive preview is `--dry-run`.

## Constraints
- forbid-git: only `internal/git`; cyclo <15.
- Predicate exact/prefix, no substring; never `-D` without 2nd confirm.
- Dirty worktree aborts; CI non-TTY never blocks; fetch fail = warning.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/git/cleanup.go` | New | git helpers |
| `internal/assets/skills/sdd-archive/SKILL.md` | Modified | Step 3b |
| `cmd/biggz/cli_cleanup.go` + `main.go` | New/Modified | cleanup verb |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Unmerged delete | Low | `branch -d` + IsMergedTo; `-D` needs 2nd confirm |
| Dirty worktree | Low | `status --porcelain` guard |
| CI hang | Low | skip if non-TTY |
| Name collision | Low | exact/prefix only |
| Fetch fail | Med | warning, continue |

## Rollback Plan
Archive: `os.Rename` inverse from `archive/YYYY-MM-DD-{change}/`. Branches: restore via logged SHA/reflog. Remove `cleanup.go`/`cli_cleanup.go` + revert Step 3b.

## Dependencies
- `internal/git` ownership; `ArchiveChange` authority.

## Success Criteria
- [ ] Archive previews then deletes only matching `[gone]` via `-d`; `.biggz-instance` stays.
- [ ] `biggz cleanup --dry-run` lists without mutation; needs consent otherwise.
- [ ] No `-D` without 2nd confirm; headless CI no block.
- [ ] forbid-git + cyclo <15 pass.

## Proposal question round
Blocked from live ask — 4 questions, assumptions need review:
1. Opt-in vs opt-out? Assumed Prune/Keep prompt.
2. `change-*` vs `change-pr*`? Assumed `change-*`.
3. Offline fetch fail blocks archive? Assumed warning only.
4. `cleanup` prunes worktrees by default? Assumed flag only.
Reply skip/correct or request second round.

## Open Questions
- Doctor INFO scope: all `[gone]` or change-scoped? Defer to spec.
