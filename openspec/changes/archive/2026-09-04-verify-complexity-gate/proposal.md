# Proposal: 2026-09-04-verify-complexity-gate — Diff-Aware Complexity Gate at Verify

## Intent

Agents learn about complexity violations late (review lens R2, CI gate, or manual `doctor`) — never while the SDD loop can still act. Move the control left into the `verify` phase: block verify on NEW 15/20 violations in code the change touched, grandfather everything else, no git hooks (project decision: gates live in CLI verbs).

## Scope

### In Scope
- **Lens export:** `OffendersForFileDiffs(repoRoot, hunks)` in `internal/review/lens/readability` reusing `offendersFromHunks` (hunk-bounded, critical-package filtered).
- **SDD gate:** `internal/sdd/complexity_gate.go` — `GateDiffComplexity(repoRoot, diff)` (pure-ish, testable) + `GateWorkingTreeComplexity(repoRoot)` (`git diff HEAD` + untracked `.go` as full-file).
- **Gatekeeper wiring:** new `complexity_gate` check for `completedPhase == "verify"` (SKIP with reason when not a git repo); surfaces via existing `sdd-gatekeeper` CLI.
- **Prevention:** 15/20 budget line in `internal/assets/prompts/sdd/sdd-apply.md`.
- **Tests/docs:** unit (lens export, gate w/ fixture diffs, gatekeeper verify pass/block/skip), CHANGELOG.

### Out of Scope
- Git hooks (rejected by architecture decision), live watcher/LSP, blocking on `_test.go`, VerifyReport schema change (evidence travels in the gatekeeper check), baseline files (grandfathering is structural: only diff hunks are measured).

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `sdd-status`/`verify` flow: verify gains a blocking complexity dimension for new critical offenders.
- `readability` lens: exposes its hunk engine for reuse outside review.

## Approach

Single work unit (<400 lines): lens export (~25L) + gate (~120L) + gatekeeper check (~40L) + prompt line + tests (~150L) + SDD artifacts + CHANGELOG.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/review/lens/readability/complexity.go` | Modified | Export `OffendersForFileDiffs` wrapper |
| `internal/sdd/complexity_gate.go` | New | Diff splitting, gate verdict, git worktree scan |
| `internal/sdd/gatekeeper.go` | Modified | `complexity_gate` check on verify phase |
| `internal/assets/prompts/sdd/sdd-apply.md` | Modified | 15/20 budget contract line |
| `openspec/changes/2026-09-04-verify-complexity-gate/` | New | This SDD change (proposal/spec/design/tasks) |

## Risks
- `git` absent in exotic envs → gate SKIPs (documented), R2/CI remain backstops.
- Committed-but-unpushed work units at verify: out of scope (worktree scan); R2/CI catch those.

## Rollback
`git revert` the single commit; gatekeeper returns to 5 checks, prompt line removed.
