package doctor

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

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
	getwdFn   func() (string, error)
	statFn    func(string) (os.FileInfo, error)
	readDirFn func(string) ([]os.DirEntry, error)
	execFn    func(string, ...string) ([]byte, error)
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

// Remedy returns a repair action that removes stale lock files.
// It walks the git common dir and ~/.biggz looking for LOCK/.lock files
// and removes those older than 5 minutes or whose PID is dead. Safe and idempotent.
func (c *ReviewCheck) Remedy() *Remedy {
	return &Remedy{
		ID:          string(ReviewCheckID),
		Description: "Remove stale review locks",
		Action: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			var roots []string
			if gitDir, err := c.resolveGitDir(); err == nil {
				roots = append(roots, gitDir)
				// Also walk the parent of .git (repo root) for sdd-runtime in case common dir differs.
				if parent := filepath.Dir(gitDir); parent != gitDir && parent != "." {
					roots = append(roots, parent)
				}
			}
			if home, err := os.UserHomeDir(); err == nil && home != "" {
				roots = append(roots, filepath.Join(home, ".biggz"))
			}
			// Deduplicate roots.
			seen := make(map[string]bool)
			var uniq []string
			for _, r := range roots {
				r = filepath.Clean(r)
				if r == "" || seen[r] {
					continue
				}
				seen[r] = true
				uniq = append(uniq, r)
			}

			for _, root := range uniq {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
					if err != nil {
						return nil // skip unreadable
					}
					select {
					case <-ctx.Done():
						return fs.SkipAll
					default:
					}
					if d.IsDir() {
						return nil
					}
					base := filepath.Base(path)
					if base != "LOCK" && base != ".lock" && !strings.HasSuffix(base, ".lock") {
						return nil
					}
					stale, serr := isStaleLockFile(path)
					if serr != nil {
						return nil
					}
					if stale {
						_ = os.Remove(path)
					}
					return nil
				})
			}
			return nil
		},
	}
}

// staleLockAge is the maximum age after which a lock is considered stale.
const staleLockAge = 5 * time.Minute

// isStaleLockFile reports whether the lock file at path is stale.
// A lock is stale if its mtime exceeds staleLockAge or its PID is dead.
func isStaleLockFile(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if time.Since(info.ModTime()) > staleLockAge {
		return true, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		return false, nil
	}
	pidStr := strings.TrimSpace(lines[0])
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		return false, nil
	}
	if !isDoctorProcessAlive(pid) {
		return true, nil
	}
	return false, nil
}

// isDoctorProcessAlive reports whether a process with pid is alive.
func isDoctorProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	return true
}
