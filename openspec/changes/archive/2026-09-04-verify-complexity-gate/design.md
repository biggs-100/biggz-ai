# Design: 2026-09-04-verify-complexity-gate

## Decisions

1. **Reuse, don't duplicate.** The hunk engine lives in `internal/review/lens/readability` (`offendersInHunk`, `parseHunkHeaders`, thresholds, critical-package list). The gate calls it via a new exported `OffendersForFileDiffs(repoRoot, hunks map[string][]byte)`. No new AST code in `sdd`.
2. **Grandfathering is structural.** Only functions overlapping diff hunks are measured, so pre-existing violations can never fail the gate. No baseline file, no drift.
3. **Worktree scan, not history.** `GateWorkingTreeComplexity` = `git diff HEAD` (staged+unstaged) + untracked `.go` files as full-file hunks. Committed work units mid-change are out of scope (R2/CI cover them).
4. **Skip, don't fail, without git.** Exotic non-git workspaces get `Skipped` with reason; failing closed there would break valid setups.
5. **No VerifyReport schema change.** Evidence travels in the `GatekeeperCheck` (`complexity_gate`, reason lists offenders); the existing `sdd-gatekeeper` CLI surfaces it unchanged.

## Data flow

```
verify phase
  → Gatekeeper(..., "verify", result)
    → checkComplexityGate: repoRoot = dir(openspecRoot)
      → git available? no → Skipped("not a git repo")
      → GateWorkingTreeComplexity(repoRoot)
        → git diff HEAD → SplitFileDiffs → + untracked .go (full-file)
        → readability.OffendersForFileDiffs(repoRoot, hunks)
        → critical offenders → block w/ file:function cyclo/cog
        → test offenders → warnings; non-critical passes silent
```

## File changes

- `internal/review/lens/readability/complexity.go` (+~25L): `OffendersForFileDiffs` + `SplitFileDiffs`.
- `internal/sdd/complexity_gate.go` (new, ~120L): gate verdict type, git scan, untracked handling.
- `internal/sdd/gatekeeper.go` (+~40L): `checkComplexityGate` wired for `verify` only.
- `internal/assets/prompts/sdd/sdd-apply.md` (+1 line): 15/20 budget contract.
- Tests: lens export w/ fixture diff, gate w/ temp git repo + fixture diffs, gatekeeper verify pass/block/skip.
