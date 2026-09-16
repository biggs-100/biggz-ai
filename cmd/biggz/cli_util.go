package main

import (
	"github.com/biggs-100/biggz-ai/internal/git"
	"github.com/biggs-100/biggz-ai/internal/pathquote"
)

// quotePathCLI wraps a filesystem path in double quotes for copy-paste into
// a shell. It delegates to pathquote.Quote.
func quotePathCLI(path string) string {
	return pathquote.Quote(path)
}

// lastOperationStatus maps a chain's last event operation to a display status.
func lastOperationStatus(lastOp string) string {
	switch lastOp {
	case "start_review", "in_review", "resume":
		return "in_review"
	case "complete_review":
		return "completed"
	case "block":
		return "blocked"
	case "invalidate":
		return "invalidated"
	case "withdraw":
		return "withdrawn"
	}
	return lastOp
}

// shortHash abbreviates a revision for table output.
func shortHash(hash string) string {
	if len(hash) > 12 {
		return hash[:12]
	}
	return hash
}

// detectGitDirs returns (commonDir, worktreeDir) for the current directory,
// delegating to the single owner (internal/git). The pair's order and the
// empty-on-error behavior stay byte-compatible for its callers.
//   - commonDir:  `git rev-parse --git-common-dir` — shared by all worktrees
//   - worktreeDir: `git rev-parse --git-dir` — private to this worktree
//
// For the main worktree (non-linked), both return the same path.
// For a linked worktree, they differ: commonDir is the shared .git dir,
// worktreeDir is .git/worktrees/<name>.
func detectGitDirs() (commonDir, worktreeDir string) {
	return git.DetectGitDirs()
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
