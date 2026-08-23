package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/platform"
)

const (
	// GitCheckID is the check identifier for git availability.
	GitCheckID CheckID = "git"
)

// GitCheck verifies that git is available in PATH and that the current
// working directory is a git repository.
type GitCheck struct {
	lookPathFn func(string) (string, error)
	execFn     func(string, ...string) ([]byte, error)
	getwdFn    func() (string, error)
	statFn     func(string) (os.FileInfo, error)
}

// NewGitCheck creates a GitCheck using the default environment.
func NewGitCheck() *GitCheck {
	return &GitCheck{
		lookPathFn: exec.LookPath,
		execFn:     execCommand,
		getwdFn:    os.Getwd,
		statFn:     os.Stat,
	}
}

// NewGitCheckWithCustom creates a GitCheck with injected functions for testing.
func NewGitCheckWithCustom(
	lookPathFn func(string) (string, error),
	execFn func(string, ...string) ([]byte, error),
	getwdFn func() (string, error),
	statFn func(string) (os.FileInfo, error),
) *GitCheck {
	return &GitCheck{
		lookPathFn: lookPathFn,
		execFn:     execFn,
		getwdFn:    getwdFn,
		statFn:     statFn,
	}
}

// ID returns the check identifier.
func (c *GitCheck) ID() CheckID { return GitCheckID }

// Run checks git availability and repository state.
func (c *GitCheck) Run(ctx context.Context) *Result {
	// Check 1: Is git in PATH?
	gitPath, err := c.lookPathFn("git")
	if err != nil {
		return &Result{
			ID:       GitCheckID,
			Status:   StatusFail,
			Message:  "Git not found in PATH — biggz requires git for review lineages and version management",
			Severity: SeverityCritical,
			Error:    err.Error(),
		}
	}

	// CWD safety: ensure any git subprocess inherits a valid directory
	// (mirrors gentle-ai deps.go:146 and powershell.go:37).
	probeCmd := exec.CommandContext(ctx, "git", "rev-parse", "--git-dir")
	platform.EnsureCommandDir(probeCmd)

	// Check 2: Is this a git repository?
	// Try git rev-parse to verify.
	out, err := c.execFn("git", "rev-parse", "--git-dir")
	if err != nil {
		return &Result{
			ID:       GitCheckID,
			Status:   StatusWarn,
			Message:  fmt.Sprintf("Current directory is not a git repository (git found at %s)", gitPath),
			Severity: SeverityWarning,
			Error:    fmt.Sprintf("git rev-parse failed: %v", err),
		}
	}

	gitDir := strings.TrimSpace(string(out))

	return &Result{
		ID:       GitCheckID,
		Status:   StatusPass,
		Message:  fmt.Sprintf("Git available at %s, repository at %s", gitPath, gitDir),
		Severity: SeverityInfo,
	}
}

// Remedy returns nil — git installation is manual.
func (c *GitCheck) Remedy() *Remedy { return nil }
