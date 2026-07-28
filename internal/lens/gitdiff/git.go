package gitdiff

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/biggz-ai/biggz/model"
)

// GetDiffStat runs "git diff --stat HEAD~1..HEAD" in the subject's repository
// and returns the parsed list of changed files.
func GetDiffStat(ctx context.Context, subject model.ReviewSubject) ([]DiffFile, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--stat", "HEAD~1..HEAD")
	cmd.Dir = subject.Repository
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --stat: %w", err)
	}
	return ParseDiffStat(string(output)), nil
}

// HasModeChanges runs "git diff --raw HEAD~1..HEAD" to detect executable
// mode changes (100644 → 100755) in the diff.
func HasModeChanges(ctx context.Context, subject model.ReviewSubject) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--raw", "HEAD~1..HEAD")
	cmd.Dir = subject.Repository
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git diff --raw: %w", err)
	}
	return DetectModeChanges(string(output)), nil
}
