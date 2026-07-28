package release

import (
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
