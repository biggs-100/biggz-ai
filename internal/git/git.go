package git

import (
	"errors"
	"os"
	"os/exec"
	"strings"
)

// DetectGitDirs returns the git common directory and git directory for the current worktree.
// It mirrors the prior inline logic in screens/status.go:detectGitDirs and preserves
// os.IsNotExist handling so a missing git binary does not panic.
func DetectGitDirs() (commonDir, gitDir string) {
	out, err := exec.Command("git", "rev-parse", "--git-common-dir").Output()
	if err != nil {
		if isNotExist(err) {
			return "", ""
		}
		// Non-IsNotExist errors (not a git repo, etc.) return empty as before.
	} else if len(out) > 0 {
		commonDir = strings.TrimSpace(string(out))
	}

	out, err = exec.Command("git", "rev-parse", "--git-dir").Output()
	if err != nil {
		if isNotExist(err) {
			return commonDir, ""
		}
	} else if len(out) > 0 {
		gitDir = strings.TrimSpace(string(out))
	}
	return commonDir, gitDir
}

// GitStatus runs `git status` with the given arguments in dir and returns output.
// It preserves os.IsNotExist handling so callers can detect a missing git binary without panic.
func GitStatus(dir string) ([]byte, error) {
	args := []string{"status", "--porcelain"}
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		if isNotExist(err) {
			return nil, err
		}
		// Return git's error output for debugging while preserving wrapped error.
		return out, err
	}
	return out, nil
}

// GitDiff runs `git diff` with the provided args in dir and returns output.
// It preserves os.IsNotExist handling.
func GitDiff(dir string, args ...string) ([]byte, error) {
	cmdArgs := append([]string{"diff"}, args...)
	cmd := exec.Command("git", cmdArgs...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		if isNotExist(err) {
			return nil, err
		}
		return out, err
	}
	return out, nil
}

func isNotExist(err error) bool {
	if err == nil {
		return false
	}
	if os.IsNotExist(err) {
		return true
	}
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	// exec.Error may wrap ErrNotExist via PathError
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return os.IsNotExist(pathErr.Err) || errors.Is(pathErr.Err, os.ErrNotExist)
	}
	// Some Go versions wrap exec.ErrNotFound
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return errors.Is(execErr.Err, os.ErrNotExist) || os.IsNotExist(execErr.Err)
	}
	return false
}
