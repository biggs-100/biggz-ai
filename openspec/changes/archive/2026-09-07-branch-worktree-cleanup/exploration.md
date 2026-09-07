# Exploration: branch-worktree-cleanup

## Current State
biggz-orchestrator-workflow.md defines terminal SDD sequence proposal -> spec -> design -> tasks -> apply -> verify -> archive. sdd-archive (internal/assets/skills/sdd-archive/SKILL.md + internal/sdd/archive.go:ArchiveChange) only syncs delta specs openspec/changes/{change}/specs/ -> openspec/specs/ and moves openspec/changes/{change} -> openspec/changes/archive/YYYY-MM-DD-{change}/ via os.Rename. No git operation is performed. Verified via grep worktree|branch.*delete|prune across internal/, skills/, cmd/biggz/ — zero SDD-owned cleanup. internal/git/git.go only exposes DetectGitDirs, GitStatus, GitDiff; no branch/worktree wrappers. .github/workflows/ci.yml has no fetch-prune.

Observed symptom is expected: tui-installer-pipeline was delivered via 5 stacked PRs pr1..pr5, remote branches auto-deleted but local refs remained as [gone] until manual git fetch --prune && git branch -d. Current repo is clean after manual prune. Historical stray .biggz-instance in archive confirms ArchiveChange moves .biggz-instance with directory (per internal/sdd/edit_authority_consent.go:27). No code deletes it. internal/sdd/status.go never touches branches. Upstream gentle-ai same gap.

## Affected Areas
- internal/sdd/archive.go — ArchiveChange move logic; insertion point for post-archive prune
- internal/assets/skills/sdd-archive/SKILL.md — Step 3 Move to Archive + Step 4 Verify
- internal/git/git.go — sole exec.Command("git") owner per forbid-git CI; any new git cleanup MUST live here
- cmd/biggz/cli_sdd.go — no cleanup verb; needs flag or biggz cleanup
- internal/assets/biggz/biggz-orchestrator-workflow.md + delegation.md — orchestrator consent envelope
- internal/doctor/* — candidate for INFO warning on stale branches/worktrees
- openspec/config.yaml rules.archive — extension point
- openspec/changes/archive/ — .biggz-instance stays inside archived folder

## Approaches

### 1. Extend sdd-archive with optional post-archive prune (recommended)
Brief: After successful ArchiveChange, run git fetch --prune, list branches where upstream [gone] AND (branch == change OR branch HasPrefix(change+"-") OR fully merged to origin/master), list worktrees via git worktree list --porcelain, preview --dry-run, explicit consent, then git branch -d and git worktree prune.
Pros: Coupled to lifecycle, minimal friction, reuses archive consent, safe preview, respects internal/git ownership.
Cons: Adds interactive I/O to archive, needs filtering.
Effort: Medium

### 2. Standalone biggz cleanup CLI (manual)
Brief: Add biggz cleanup [--dry-run] [--prune-worktrees] [--delete-gone-branches].
Pros: Usable outside SDD, decoupled risk.
Cons: Doesnot auto-fix, low discoverability.
Effort: Low-Medium

### 3. Status quo: GitHub auto-delete + manual git fetch --prune
Pros: Zero code.
Cons: Local [gone] branches remain — bug persists.
Effort: Low

## Recommendation
GO — Approach 1 scoped + Approach 2 as dry-run preview helper. Add internal/git/cleanup.go helpers (FetchPrune, ListGoneBranches, IsMergedTo, ListWorktrees), extend sdd-archive Step 3b with dry-run preview + lossless blocking consent (Prune vs Keep), expose biggz cleanup --dry-run, keep .biggz-instance inside archive.

## Risks
- Data loss on unmerged branches: mitigate branch -d only + IsMergedTo check; -D requires second confirmation.
- Dirty worktree: worktree remove refuses when dirty; guard status --porcelain.
- CI hang: preview skip via --dry-run non-interactive.
- Branch name collision: exact change or change-* prefix only.
- Remote not fetched: fetch --prune failure is warning not error.
- Complexity budget: stay under 15 cyclo in internal/git.

## Ready for Proposal
Yes — go. Scope is small additive change: one new internal/git/cleanup.go + ~20-line extension to sdd-archive Step 3b + orchestrator prompt + biggz cleanup --dry-run.
