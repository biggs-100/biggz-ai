package release

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionPattern(t *testing.T) {
	valid := []string{
		"v1.0.0",
		"v2.3.4",
		"v0.1.0-beta",
		"v10.20.30-rc.1",
	}
	invalid := []string{
		"1.0.0",
		"v1.0",
		"v1.0.0.0",
		"version-1",
		"",
		"v1.0.0_beta",
	}

	for _, v := range valid {
		if !VersionPattern.MatchString(v) {
			t.Errorf("expected %q to match version pattern", v)
		}
	}
	for _, v := range invalid {
		if VersionPattern.MatchString(v) {
			t.Errorf("expected %q to NOT match version pattern", v)
		}
	}
}

func TestCheckGitState(t *testing.T) {
	state, err := CheckGitState()
	if err != nil {
		t.Fatalf("CheckGitState() error: %v", err)
	}
	if state.Branch == "" {
		t.Error("expected non-empty branch name")
	}
	if state.Commit == "" {
		t.Error("expected non-empty commit SHA")
	}
}

func TestTag_InvalidVersion(t *testing.T) {
	_, err := Tag("not-a-version", false)
	if err == nil {
		t.Fatal("expected error for invalid version")
	}
}

// initTagRepo initializes a repository with one commit, so Tag's clean-tree
// precondition holds and tag creation can be exercised for real.
func initTagRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init")
}

// TestTagFailureOutputStaysRaw pins the failure contract of Tag across the
// single-owner migration: the error must carry git's own words about the
// failing tag command, so a swallowed or reformatted stderr would fail here.
func TestTagFailureOutputStaysRaw(t *testing.T) {
	repo := t.TempDir()
	initTagRepo(t, repo)
	t.Chdir(repo)

	if _, err := Tag("v9.9.9", false); err != nil {
		t.Fatalf("first Tag() error: %v", err)
	}
	_, err := Tag("v9.9.9", false)
	if err == nil {
		t.Fatal("duplicate tag must fail")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("git tag failure output must stay raw (want git's wording), got: %v", err)
	}
}

// TestCheckGitStateInTempRepo pins that the state probe answers from an
// explicit repository, not from the module checkout the test binary runs in.
func TestCheckGitStateInTempRepo(t *testing.T) {
	repo := t.TempDir()
	initTagRepo(t, repo)
	t.Chdir(repo)

	state, err := CheckGitState()
	if err != nil {
		t.Fatalf("CheckGitState() error: %v", err)
	}
	if !state.Clean {
		t.Error("fresh repository with one commit must report a clean tree")
	}
	if state.Branch == "" || state.Commit == "" {
		t.Errorf("branch and commit must be populated, got %q / %q", state.Branch, state.Commit)
	}
}
