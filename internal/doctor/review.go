package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/platform"
	"github.com/biggs-100/biggz-ai/internal/review"
)

const (
	// ReviewCheckID is the check identifier for review store chain integrity.
	ReviewCheckID CheckID = "review"
)

// ReviewCheck verifies the integrity of all review lineages in the
// review event store. It enumerates lineages under
// .git/biggz/review-transactions/ and calls store.Validate() on each.
type ReviewCheck struct {
	getwdFn  func() (string, error)
	statFn   func(string) (os.FileInfo, error)
	readDirFn func(string) ([]os.DirEntry, error)
	execFn   func(string, ...string) ([]byte, error)
}

// NewReviewCheck creates a ReviewCheck using the default environment.
func NewReviewCheck() *ReviewCheck {
	return &ReviewCheck{
		getwdFn:   os.Getwd,
		statFn:    os.Stat,
		readDirFn: os.ReadDir,
		execFn:    execCommand,
	}
}

// NewReviewCheckWithCustom creates a ReviewCheck with injected functions for testing.
func NewReviewCheckWithCustom(
	getwdFn func() (string, error),
	statFn func(string) (os.FileInfo, error),
	readDirFn func(string) ([]os.DirEntry, error),
	execFn func(string, ...string) ([]byte, error),
) *ReviewCheck {
	return &ReviewCheck{
		getwdFn:   getwdFn,
		statFn:    statFn,
		readDirFn: readDirFn,
		execFn:    execFn,
	}
}

// execCommand wraps os/exec for testability and ensures the child command
// inherits a valid working directory via platform.EnsureCommandDir.
var execCommand = func(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	platform.EnsureCommandDir(cmd)
	return cmd.Output()
}

// ID returns the check identifier.
func (c *ReviewCheck) ID() CheckID { return ReviewCheckID }

// Run enumerates all review lineages and validates each one.
func (c *ReviewCheck) Run(ctx context.Context) *Result {
	gitDir, err := c.resolveGitDir()
	if err != nil {
		// If we can't find a git directory, the review store doesn't exist — WARNING.
		return &Result{
			ID:       ReviewCheckID,
			Status:   StatusWarn,
			Message:  "Not a git repository — review store not available",
			Severity: SeverityWarning,
			Error:    err.Error(),
		}
	}

	storeRoot := filepath.Join(gitDir, "biggz", "review-transactions")

	// Check if the review-transactions directory exists.
	_, err = c.statFn(storeRoot)
	if err != nil {
		if os.IsNotExist(err) {
			// No review store yet — not necessarily a problem, but worth noting.
			return &Result{
				ID:       ReviewCheckID,
				Status:   StatusPass,
				Message:  "No review lineages found — store is empty",
				Severity: SeverityInfo,
			}
		}
		return &Result{
			ID:       ReviewCheckID,
			Status:   StatusFail,
			Message:  fmt.Sprintf("Cannot access review store at %s", storeRoot),
			Severity: SeverityCritical,
			Error:    err.Error(),
		}
	}

	// Enumerate lineage directories.
	entries, err := c.readDirFn(storeRoot)
	if err != nil {
		return &Result{
			ID:       ReviewCheckID,
			Status:   StatusFail,
			Message:  fmt.Sprintf("Cannot read review store at %s", storeRoot),
			Severity: SeverityCritical,
			Error:    err.Error(),
		}
	}

	var lineageIDs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		lineageIDs = append(lineageIDs, e.Name())
	}

	if len(lineageIDs) == 0 {
		return &Result{
			ID:       ReviewCheckID,
			Status:   StatusPass,
			Message:  "No review lineages found",
			Severity: SeverityInfo,
		}
	}

	// Validate each lineage.
	var failures []string
	for _, lineageID := range lineageIDs {
		store := review.OpenWithDir(filepath.Join(storeRoot, lineageID), lineageID)
		verdict := store.Validate()
		if !verdict.Valid {
			failures = append(failures, fmt.Sprintf("%s: %s", lineageID, verdict.Reason))
		}
	}

	if len(failures) > 0 {
		return &Result{
			ID:       ReviewCheckID,
			Status:   StatusFail,
			Message:  fmt.Sprintf("Review chain integrity violations: %s", strings.Join(failures, "; ")),
			Severity: SeverityCritical,
			Error:    fmt.Sprintf("lineages with violations: %v", failures),
		}
	}

	return &Result{
		ID:       ReviewCheckID,
		Status:   StatusPass,
		Message:  fmt.Sprintf("All %d review lineages pass integrity check", len(lineageIDs)),
		Severity: SeverityInfo,
	}
}

// resolveGitDir finds the .git directory using git rev-parse.
func (c *ReviewCheck) resolveGitDir() (string, error) {
	// First try: check for a direct .git path in the current directory.
	cwd, err := c.getwdFn()
	if err != nil {
		return "", err
	}

	gitPath := filepath.Join(cwd, ".git")
	info, err := c.statFn(gitPath)
	if err == nil && info.IsDir() {
		return gitPath, nil
	}

	// Second try: use git command as fallback.
	out, err := c.execFn("git", "rev-parse", "--git-dir")
	if err != nil {
		return "", fmt.Errorf("no git directory: %w", err)
	}

	gitDir := strings.TrimSpace(string(out))
	if gitDir == "" {
		return "", fmt.Errorf("empty git dir from rev-parse")
	}

	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(cwd, gitDir)
	}

	return filepath.Clean(gitDir), nil
}

// Remedy returns nil — chain repair requires manual intervention.
func (c *ReviewCheck) Remedy() *Remedy { return nil }
