package gitdiff

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/biggz-ai/biggz/model"
)

// defaultGitTimeout is the maximum time for a git operation.
// Prevents hung processes from blocking reviews.
const defaultGitTimeout = 30 * time.Second

// GetDiffStat runs git diff --stat with a timeout.
// Ported from gentle-ai's git process control with simplified timeout handling.
func GetDiffStat(ctx context.Context, subject model.ReviewSubject) ([]DiffFile, error) {
	if subject.Repository == "" {
		// Use current directory
		return getDiffStatInDir(ctx, ".", "HEAD~1..HEAD")
	}
	return getDiffStatInDir(ctx, subject.Repository, "HEAD~1..HEAD")
}

// getDiffStatInDir runs git diff in the specified directory.
func getDiffStatInDir(ctx context.Context, dir, rev string) ([]DiffFile, error) {
	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, defaultGitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "diff", "--stat", rev)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("git diff timed out after %s", defaultGitTimeout)
		}
		return nil, fmt.Errorf("git diff: %w", err)
	}

	return ParseDiffStat(string(output)), nil
}

// DetectGitModeChanges checks for executable mode changes in the diff.
func DetectGitModeChanges(ctx context.Context, subject model.ReviewSubject) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultGitTimeout)
	defer cancel()

	dir := "."
	if subject.Repository != "" {
		dir = subject.Repository
	}

	cmd := exec.CommandContext(ctx, "git", "diff", "-p", "--diff-filter=M", "HEAD~1..HEAD")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git diff mode: %w", err)
	}
	return hasModeChange(string(output)), nil
}

func hasModeChange(diff string) bool {
	// Look for "old mode" / "new mode" lines
	for i := 0; i < len(diff)-8; i++ {
		if diff[i:i+8] == "old mode" || diff[i:i+9] == "new mode" {
			return true
		}
	}
	return false
}
