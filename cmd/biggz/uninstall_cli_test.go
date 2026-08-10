package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// TestUninstall_Help verifies "biggz uninstall --help" prints usage and
// exits 0.
func TestUninstall_Help(t *testing.T) {
	cmd := goRunBiggz(t, "uninstall", "--help")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("uninstall --help exited with error: %v", err)
	}
	if !strings.Contains(stderr.String(), "Usage: biggz uninstall") {
		t.Errorf("expected help to contain 'Usage: biggz uninstall', got: %s", stderr.String())
	}
}

// TestUninstall_UnknownFlag verifies "biggz uninstall --unknown" exits 1.
func TestUninstall_UnknownFlag(t *testing.T) {
	cmd := goRunBiggz(t, "uninstall", "--unknown")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit for unknown flag")
	}
	if !strings.Contains(stderr.String(), "unknown flag") {
		t.Errorf("expected 'unknown flag' error, got: %s", stderr.String())
	}
}

// TestUninstall_UnknownAgent verifies an unknown --agent exits 1 with a
// clear error.
func TestUninstall_UnknownAgent(t *testing.T) {
	cmd := goRunBiggz(t, "uninstall", "--yes", "--agent", "nope")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit for unknown agent")
	}
	if !strings.Contains(stderr.String(), "unknown agent") {
		t.Errorf("expected 'unknown agent' error, got: %s", stderr.String())
	}
}

// TestUninstall_NonInteractiveRequiresYes verifies that without --yes the
// child process (stdin is /dev/null, not a TTY) fails with a clear message.
func TestUninstall_NonInteractiveRequiresYes(t *testing.T) {
	cmd := goRunBiggz(t, "uninstall", "--home", t.TempDir())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit without --yes in non-interactive mode")
	}
	if !strings.Contains(stderr.String(), "--yes") {
		t.Errorf("expected error mentioning --yes, got: %s", stderr.String())
	}
}

// TestUninstall_EmptyHome verifies a clean run on an empty home exits 0,
// reports zero agents and leaves no leftovers.
func TestUninstall_EmptyHome(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := goRunBiggz(t, "uninstall", "--yes", "--home", tmpDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("uninstall --yes --home exited with error: %v (stderr: %s)", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "0 agents uninstalled, 0 failed") {
		t.Errorf("expected summary line, got: %s", stdout.String())
	}
	if len(stderr.String()) > 0 {
		t.Errorf("expected no stderr output, got: %s", stderr.String())
	}
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) > 0 {
		t.Errorf("expected no leftovers in home, found %d entries", len(entries))
	}
}

// TestUninstall_DryRunOnEmptyHome verifies --dry-run works without --yes
// (it changes nothing) and exits 0.
func TestUninstall_DryRunOnEmptyHome(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := goRunBiggz(t, "uninstall", "--dry-run", "--home", tmpDir)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("uninstall --dry-run exited with error: %v", err)
	}
	if !strings.Contains(stdout.String(), "Dry-run") {
		t.Errorf("expected dry-run summary, got: %s", stdout.String())
	}
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) > 0 {
		t.Errorf("expected no files written in dry-run, found %d entries", len(entries))
	}
}

// TestUninstall_NoLeftoversAfterPurgeOnEmptyHome verifies --purge on an
// empty home leaves no directories behind.
func TestUninstall_NoLeftoversAfterPurgeOnEmptyHome(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := goRunBiggz(t, "uninstall", "--yes", "--purge", "--home", tmpDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("uninstall --purge exited with error: %v", err)
	}
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) > 0 {
		t.Errorf("expected no leftovers after purge, found %d entries", len(entries))
	}
}

// TestUninstall_HelpInMainHelp verifies the verb is listed in the main help.
func TestUninstall_HelpInMainHelp(t *testing.T) {
	cmd := goRunBiggz(t, "--help")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("--help exited with error: %v", err)
	}
	if !strings.Contains(stderr.String(), "uninstall") {
		t.Errorf("expected main help to list uninstall, got: %s", stderr.String())
	}
}
