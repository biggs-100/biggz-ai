// Package release provides version tagging and release verification for biggz-ai.
//
// It validates git state, creates version tags, and verifies that a release
// tag matches the current repository state.
package release

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// GitState describes the current git repository state.
type GitState struct {
	Clean           bool   // true if working tree is clean
	Branch          string // current branch name
	Commit          string // current commit SHA (short)
	LastTag         string // most recent tag reachable from HEAD
	CommitsSinceTag int    // commits since last tag
}

// VersionPattern is the regex for valid semantic version tags.
var VersionPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+(-[\w.]+)?$`)

// CheckGitState inspects the current git repository state.
func CheckGitState() (*GitState, error) {
	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("git not found: %w", err)
	}

	state := &GitState{}

	// Check clean working tree
	output, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}
	state.Clean = len(strings.TrimSpace(string(output))) == 0

	// Get branch
	branch, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err == nil {
		state.Branch = strings.TrimSpace(string(branch))
	}

	// Get short commit
	commit, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err == nil {
		state.Commit = strings.TrimSpace(string(commit))
	}

	// Get last tag
	tag, err := exec.Command("git", "describe", "--tags", "--abbrev=0", "--always").Output()
	if err == nil {
		state.LastTag = strings.TrimSpace(string(tag))
	}

	return state, nil
}

// Tag creates a signed version tag and returns the tag name.
func Tag(version string, sign bool) (string, error) {
	if !VersionPattern.MatchString(version) {
		return "", fmt.Errorf("invalid version %q: must match vMAJOR.MINOR.PATCH", version)
	}

	state, err := CheckGitState()
	if err != nil {
		return "", fmt.Errorf("check git state: %w", err)
	}
	if !state.Clean {
		return "", fmt.Errorf("working tree is not clean (branch: %s, commit: %s)", state.Branch, state.Commit)
	}

	args := []string{"tag"}
	if sign {
		args = append(args, "-s")
	}
	args = append(args, "-m", fmt.Sprintf("Release %s", version), version)

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git tag: %s: %w", strings.TrimSpace(string(output)), err)
	}

	return version, nil
}

// VerifyTag checks that the given version tag exists and matches the expected
// format. Returns the tag commit and any error.
func VerifyTag(version string) (string, error) {
	if !VersionPattern.MatchString(version) {
		return "", fmt.Errorf("invalid version %q", version)
	}

	output, err := exec.Command("git", "rev-parse", "--verify", version).Output()
	if err != nil {
		return "", fmt.Errorf("tag %q not found: %w", version, err)
	}

	return strings.TrimSpace(string(output)), nil
}
