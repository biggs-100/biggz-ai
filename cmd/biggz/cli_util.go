package main

import (
	"os/exec"
	"strings"

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

// detectGitDir returns the git common dir (e.g., .git) by running
// git rev-parse --git-common-dir. Returns "" if not in a git repo.
// detectGitDirs returns (commonDir, worktreeDir) for the current directory.
//   - commonDir:  `git rev-parse --git-common-dir` — shared by all worktrees
//   - worktreeDir: `git rev-parse --git-dir` — private to this worktree
//
// For the main worktree (non-linked), both return the same path.
// For a linked worktree, they differ: commonDir is the shared .git dir,
// worktreeDir is .git/worktrees/<name>.
func detectGitDirs() (commonDir, worktreeDir string) {
	// --git-common-dir (clone scope, shared)
	out, err := exec.Command("git", "rev-parse", "--git-common-dir").Output()
	if err == nil {
		commonDir = strings.TrimSpace(string(out))
	}
	// --git-dir (worktree scope, private)
	out, err = exec.Command("git", "rev-parse", "--git-dir").Output()
	if err == nil {
		worktreeDir = strings.TrimSpace(string(out))
	}
	return
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
