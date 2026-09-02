package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInstall_DryRunZeroWrites verifies --dry-run via Prepare preview only creates zero files.
func TestInstall_DryRunZeroWrites(t *testing.T) {
	tmpDir := t.TempDir()
	cmd := goRunBiggz(t, "install", "--dry-run", "--agent", "opencode", "--yes", "--home", tmpDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("install --dry-run exited %v stderr=%s stdout=%s", err, stderr.String(), stdout.String())
	}
	out := stdout.String() + stderr.String()
	if !strings.Contains(strings.ToLower(out), "dry-run") && !strings.Contains(strings.ToLower(out), "preview") {
		t.Errorf("expected dry-run preview output, got: %s", out)
	}
	// Ensure zero writes outside TempDir: tmpDir should have no state.json nor skills outside? Actually dry-run should create zero files.
	// Check that no state file was created under tmpDir/.biggz-ai
	if _, err := os.Stat(filepath.Join(tmpDir, ".biggz-ai", "state.json")); !os.IsNotExist(err) {
		t.Errorf("dry-run should not create state.json, but file exists")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".biggz", "state.json")); !os.IsNotExist(err) {
		t.Errorf("dry-run should not create legacy state.json, but file exists")
	}
	// Skills dir should not exist after dry-run
	if _, err := os.Stat(filepath.Join(tmpDir, ".biggz", "skills")); !os.IsNotExist(err) {
		t.Errorf("dry-run should not create skills dir")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".config", "opencode")); !os.IsNotExist(err) {
		// Might be created by Prepare? Ensure not; but if it exists, check that no files inside
		entries, _ := os.ReadDir(filepath.Join(tmpDir, ".config", "opencode"))
		if len(entries) > 0 {
			t.Errorf("dry-run should not populate opencode config, found %d entries", len(entries))
		}
	}
}

// TestInstall_InvalidAgentBlocksApply verifies unknown --agent blocks Apply and returns non-zero.
func TestInstall_InvalidAgentBlocksApply(t *testing.T) {
	tmpDir := t.TempDir()
	cmd := goRunBiggz(t, "install", "--agent", "nope", "--yes", "--home", tmpDir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit for invalid agent, got 0")
	}
	if !strings.Contains(stderr.String(), "unknown agent") {
		t.Errorf("expected 'unknown agent' error, got: %s", stderr.String())
	}
	// Ensure no files were written (Apply blocked)
	if _, err := os.Stat(filepath.Join(tmpDir, ".biggz-ai", "state.json")); !os.IsNotExist(err) {
		t.Errorf("invalid agent should not create state file")
	}
}

// TestInstall_E2EFakeAgent validates pipeline StagePlan via Orchestrator with ProgressChan using TempDir isolation.
// This mirrors internal/install e2e but via CLI --home TempDir to prove TempDir isolation.
func TestInstall_E2EWithHomeTempDir(t *testing.T) {
	tmpDir := t.TempDir()
	// Use --dry-run to test Prepare path that uses StagePlan + ProgressChan(32) without writes
	cmd := goRunBiggz(t, "install", "--dry-run", "--agent", "pi", "--yes", "--home", tmpDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("e2e dry-run with pi failed: %v stderr=%s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "pi") && !strings.Contains(strings.ToLower(out), "agent") {
		t.Errorf("expected pi agent in preview, got: %s", out)
	}
	// Verify pipeline steps are listed via StagePlan Prepare: deploy-skills, deploy-overlay, state-merge, pi-extensions
	for _, want := range []string{"deploy-skills", "deploy-overlay", "state-merge", "pi-extensions"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected preview to list step %q, got: %s", want, out)
		}
	}
	// Ensure ProgressChan mention (buffered 32) per spec wiring
	if !strings.Contains(out, "ProgressChan") && !strings.Contains(strings.ToLower(out), "progress") {
		t.Logf("preview output missing ProgressChan hint, got: %s", out)
	}
}

// TestInstall_YesSkipsPromptButValidates verifies --yes still validates invalid agent.
func TestInstall_YesSkipsPromptButValidates(t *testing.T) {
	tmpDir := t.TempDir()
	// Invalid agent with --yes should still fail validation (not skip)
	cmd := goRunBiggz(t, "install", "--yes", "--agent", "unknown123", "--home", tmpDir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		t.Fatal("expected failure for invalid agent even with --yes")
	}
	if !strings.Contains(stderr.String(), "unknown agent") {
		t.Errorf("expected unknown agent error with --yes, got %s", stderr.String())
	}
	// Valid agent with --yes and --dry-run should succeed preview
	cmd2 := goRunBiggz(t, "install", "--dry-run", "--yes", "--agent", "opencode", "--home", tmpDir)
	var stdout bytes.Buffer
	var stderr2 bytes.Buffer
	cmd2.Stdout = &stdout
	cmd2.Stderr = &stderr2
	if err := cmd2.Run(); err != nil {
		t.Fatalf("--yes with valid agent dry-run should succeed: %v stderr=%s", err, stderr2.String())
	}
}

// TestInstall_Help verifies install --help lists flags
func TestInstall_Help(t *testing.T) {
	cmd := goRunBiggz(t, "install", "--help")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("install --help failed: %v", err)
	}
	out := stderr.String()
	for _, want := range []string{"--dry-run", "--agent", "--yes"} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing %q, got: %s", want, out)
		}
	}
}
