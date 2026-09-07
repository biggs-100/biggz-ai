package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/platform"
)

// GoneBranch represents a local branch whose upstream is gone.
type GoneBranch struct {
	Branch       string
	UpstreamGone bool
	Merged       bool
}

// Worktree represents a git worktree entry from porcelain output.
type Worktree struct {
	Path         string
	Branch       string
	LockedReason string
	Prunable     bool
}

// Preview holds the result of a prune preview or execution.
type Preview struct {
	Branches  []GoneBranch
	Worktrees []Worktree
	Table     string
}

// isProtectedBranch reports whether branch is protected and must never be deleted.
func isProtectedBranch(name string) bool {
	switch name {
	case "main", "master", "HEAD":
		return true
	default:
		return false
	}
}

// IsCandidate implements the predicate:
// gone && !protected && !current && (name==change || HasPrefix(change+"-") || merged)
// Never matches substring without prefix and never matches protected/current.
func IsCandidate(name, change string, gone, merged, isCurrent, protected bool) bool {
	if !gone {
		return false
	}
	if isCurrent {
		return false
	}
	if protected || isProtectedBranch(name) {
		return false
	}
	if name == change && change != "" {
		return true
	}
	if change != "" && strings.HasPrefix(name, change+"-") {
		return true
	}
	if merged {
		return true
	}
	return false
}

// parseBranchLine parses a single line of `git branch -vv`.
// It detects the branch name, whether upstream is gone, and whether it is the current branch.
func parseBranchLine(line string) (branch string, gone bool, isCurrent bool, ok bool) {
	if strings.TrimSpace(line) == "" {
		return "", false, false, false
	}
	trimmed := strings.TrimSpace(line)
	isCurrent = strings.HasPrefix(trimmed, "*")
	stripped := trimmed
	if isCurrent {
		stripped = strings.TrimSpace(strings.TrimPrefix(trimmed, "*"))
	}
	fields := strings.Fields(stripped)
	if len(fields) == 0 {
		return "", false, false, false
	}
	branch = fields[0]
	// Branch names in detached HEAD state look like "(HEAD".
	if branch == "(HEAD" {
		return branch, strings.Contains(line, ": gone"), isCurrent, false
	}
	gone = strings.Contains(line, ": gone")
	return branch, gone, isCurrent, true
}

// parseWorktreePorcelain parses `git worktree list --porcelain` output.
func parseWorktreePorcelain(out string) []Worktree {
	var result []Worktree
	var cur Worktree
	flush := func() {
		if cur.Path != "" {
			result = append(result, cur)
			cur = Worktree{}
		}
	}
	lines := strings.Split(out, "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			flush()
			cur.Path = strings.TrimPrefix(line, "worktree ")
			continue
		}
		if strings.HasPrefix(line, "branch ") {
			ref := strings.TrimPrefix(line, "branch ")
			if strings.HasPrefix(ref, "refs/heads/") {
				cur.Branch = strings.TrimPrefix(ref, "refs/heads/")
			} else {
				cur.Branch = ref
			}
			continue
		}
		if strings.HasPrefix(line, "locked") {
			reason := strings.TrimSpace(strings.TrimPrefix(line, "locked"))
			if reason == "" {
				reason = "locked"
			}
			cur.LockedReason = reason
			continue
		}
		if strings.HasPrefix(line, "prunable") {
			cur.Prunable = true
			continue
		}
	}
	flush()
	return result
}

// runGitOutput executes `git -C <cwd> <args>` and returns combined stdout.
func runGitOutput(ctx context.Context, cwd string, args ...string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if cwd == "" {
		cwd = "."
	}
	gitArgs := append([]string{"-C", cwd}, args...)
	cmd := exec.CommandContext(ctx, "git", gitArgs...)
	platform.EnsureCommandDir(cmd)
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	err := cmd.Run()
	combined := out.String()
	if errBuf.Len() > 0 && combined == "" {
		combined = errBuf.String()
	} else if errBuf.Len() > 0 {
		combined += errBuf.String()
	}
	if err != nil {
		return strings.TrimSpace(combined), err
	}
	return strings.TrimSpace(combined), nil
}

// resolveDefaultBranch tries origin/HEAD and falls back to origin/main.
func resolveDefaultBranch(ctx context.Context, cwd string) string {
	out, err := runGitOutput(ctx, cwd, "rev-parse", "--abbrev-ref", "origin/HEAD")
	if err == nil && out != "" && out != "origin/HEAD" {
		return strings.TrimSpace(out)
	}
	return "origin/main"
}

// FetchPrune runs `git -C <cwd> fetch --prune` warn-only; never fatal.
func FetchPrune(ctx context.Context, cwd string) error {
	if cwd == "" {
		cwd = "."
	}
	_, err := runGitOutput(ctx, cwd, "fetch", "--prune")
	if err != nil {
		return fmt.Errorf("fetch --prune warning: %w", err)
	}
	return nil
}

// ListGoneBranches runs `git -C <cwd> branch -vv` and returns gone branches filtered for current/protected.
func ListGoneBranches(ctx context.Context, cwd string) ([]GoneBranch, error) {
	out, err := runGitOutput(ctx, cwd, "branch", "-vv")
	if err != nil {
		return nil, fmt.Errorf("branch -vv: %w", err)
	}
	var branches []GoneBranch
	for _, line := range strings.Split(out, "\n") {
		branch, gone, isCurrent, ok := parseBranchLine(line)
		if !ok || branch == "" {
			continue
		}
		if isCurrent {
			continue
		}
		if isProtectedBranch(branch) {
			continue
		}
		if !gone {
			continue
		}
		merged := IsMergedTo(ctx, cwd, "", branch)
		branches = append(branches, GoneBranch{
			Branch:       branch,
			UpstreamGone: gone,
			Merged:       merged,
		})
	}
	return branches, nil
}

// IsMergedTo reports whether branch is ancestor of base via merge-base --is-ancestor.
func IsMergedTo(ctx context.Context, cwd, base, branch string) bool {
	if branch == "" {
		return false
	}
	if base == "" {
		base = resolveDefaultBranch(ctx, cwd)
	}
	_, err := runGitOutput(ctx, cwd, "merge-base", "--is-ancestor", branch, base)
	return err == nil
}

// ListWorktrees runs `git -C <cwd> worktree list --porcelain` and parses entries.
func ListWorktrees(ctx context.Context, cwd string) ([]Worktree, error) {
	out, err := runGitOutput(ctx, cwd, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("worktree list: %w", err)
	}
	return parseWorktreePorcelain(out), nil
}

// isWorktreeDirty checks whether the worktree at path has uncommitted changes.
func isWorktreeDirty(ctx context.Context, wtPath string) bool {
	if ctx == nil {
		ctx = context.Background()
	}
	out, err := runGitOutput(ctx, wtPath, "status", "--porcelain")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) != ""
}

// buildTable builds a markdown preview table for branches and worktrees.
func buildTable(branches []GoneBranch, worktrees []Worktree, dryRun bool, pruneWorktrees bool) string {
	var sb strings.Builder
	mode := "dry-run"
	if !dryRun {
		mode = "execute"
	}
	sb.WriteString(fmt.Sprintf("Branch cleanup preview (%s) — %d branches, %d worktrees\n", mode, len(branches), len(worktrees)))
	sb.WriteString("| Branch/Worktree | Type | Merged/Prunable | Action |\n")
	sb.WriteString("|---|---|---|---|\n")
	for _, b := range branches {
		action := "would delete (branch -d)"
		if !dryRun {
			action = "delete (branch -d)"
		}
		sb.WriteString(fmt.Sprintf("| %s | branch | %v | %s |\n", b.Branch, b.Merged, action))
	}
	for _, w := range worktrees {
		action := "would skip (use --prune-worktrees)"
		if pruneWorktrees {
			if w.LockedReason != "" {
				action = "skip (locked)"
			} else if w.Prunable || isCandidateWorktree(w) {
				if dryRun {
					action = "would prune"
				} else {
					action = "prune"
				}
			} else {
				action = "would skip (use --prune-worktrees)"
				if w.LockedReason != "" {
					action = "skip (locked)"
				}
			}
		}
		label := w.Path
		if w.Branch != "" {
			label = fmt.Sprintf("%s (%s)", w.Path, w.Branch)
		}
		sb.WriteString(fmt.Sprintf("| %s | worktree | prunable=%v | %s |\n", label, w.Prunable, action))
	}
	if len(branches) == 0 && len(worktrees) == 0 {
		sb.WriteString("| — | — | — | no candidates |\n")
	}
	return sb.String()
}

// isCandidateWorktree reports whether a worktree's branch looks candidate-linked.
// Used for preview gating when worktree is not explicitly prunable but linked to candidate branch.
func isCandidateWorktree(w Worktree) bool {
	if w.Branch == "" {
		return false
	}
	if isProtectedBranch(w.Branch) {
		return false
	}
	if w.LockedReason != "" {
		return false
	}
	return w.Prunable
}

// PruneBranches builds a preview table; when dryRun is false it executes `git branch -d` for each branch.
// It never uses `-D` without second confirmation; unmerged branches will fail and remain.
// Dirty worktree guard is handled at worktree level via isWorktreeDirty.
func PruneBranches(ctx context.Context, cwd string, branches []GoneBranch, dryRun bool) (Preview, error) {
	if cwd == "" {
		cwd = "."
	}
	// List worktrees for preview completeness (warn-only).
	worktrees, _ := ListWorktrees(ctx, cwd)
	table := buildTable(branches, worktrees, dryRun, false)
	preview := Preview{Branches: branches, Worktrees: worktrees, Table: table}
	if dryRun {
		return preview, nil
	}
	for _, b := range branches {
		_, err := runGitOutput(ctx, cwd, "branch", "-d", b.Branch)
		if err != nil {
			// branch -d fails for unmerged; do not retry with -D.
			continue
		}
	}
	// Worktree prune is separate via PruneWorktrees; PruneBranches does not auto-prune worktrees.
	return preview, nil
}

// PruneWorktrees prunes eligible clean worktrees; used when --prune-worktrees is set.
// It respects dirty guard and locked guard.
func PruneWorktrees(ctx context.Context, cwd string, worktrees []Worktree, dryRun bool) (int, error) {
	if cwd == "" {
		cwd = "."
	}
	pruned := 0
	for _, w := range worktrees {
		if w.LockedReason != "" {
			continue
		}
		if !w.Prunable && !isCandidateWorktree(w) {
			continue
		}
		if w.Path != "" && isWorktreeDirty(ctx, w.Path) {
			continue
		}
		if dryRun {
			pruned++
			continue
		}
		_, err := runGitOutput(ctx, cwd, "worktree", "prune")
		if err == nil {
			pruned++
			break // git worktree prune is global, one invocation suffices
		}
	}
	return pruned, nil
}
