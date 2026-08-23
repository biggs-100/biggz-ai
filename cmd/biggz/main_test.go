package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// moduleRoot returns the repository root containing this package, located
// via runtime.Caller instead of a hardcoded absolute path so the tests run
// on every OS and checkout location.
func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed to locate main_test.go")
	}
	// main_test.go lives in cmd/biggz; the module root is three parents up.
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

// goRunBiggz builds the `go run ./cmd/biggz` invocation rooted at the module
// root, so the binary resolves regardless of the test's working directory.
func goRunBiggz(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("go", append([]string{"run", "./cmd/biggz"}, args...)...)
	cmd.Dir = moduleRoot(t)
	return cmd
}

// runGit is a shared helper that runs a git command in the given directory.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}

// TestSync_DryRun verifies that "biggz sync --dry-run" reports expected component
// names without writing any files and exits with code 0.
func TestSync_DryRun(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := goRunBiggz(t, "sync", "--dry-run", "--home", tmpDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("sync --dry-run exited with error: %v (stderr: %s)", err, stderr.String())
	}

	output := stdout.String()
	// Should mention expected component names
	if !strings.Contains(output, "skills") {
		t.Error("expected output to mention 'skills'")
	}
	if !strings.Contains(output, "config") {
		t.Error("expected output to mention 'config'")
	}
	if !strings.Contains(output, "prompts") {
		t.Error("expected output to mention 'prompts'")
	}
	if !strings.Contains(output, "commands") {
		t.Error("expected output to mention 'commands'")
	}
	if !strings.Contains(output, "dry-run") && !strings.Contains(output, "Dry-run") {
		t.Error("expected output to indicate dry-run mode")
	}

	// Verify no files were actually written to the temp dir
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) > 0 {
		t.Errorf("expected no files written in dry-run mode, but found %d entries", len(entries))
	}
}

// TestSync_SelectiveFlags verifies that "biggz sync --dry-run --skills --config"
// reports only the selected categories.
func TestSync_SelectiveFlags(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := goRunBiggz(t, "sync", "--dry-run", "--skills", "--config", "--home", tmpDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("sync --dry-run --skills --config exited with error: %v (stderr: %s)", err, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "skills") {
		t.Error("expected output to mention 'skills'")
	}
	if !strings.Contains(output, "config") {
		t.Error("expected output to mention 'config'")
	}

	// Should NOT mention prompts or commands
	if strings.Contains(output, "prompts") && strings.Contains(output, "would") {
		t.Error("output should not mention prompts (not selected)")
	}
	if strings.Contains(output, "commands") && strings.Contains(output, "would") {
		t.Error("output should not mention commands (not selected)")
	}
}

// TestSync_Help verifies that "biggz sync --help" prints usage and exits with 0.
func TestSync_Help(t *testing.T) {
	cmd := goRunBiggz(t, "sync", "--help")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("sync --help exited with error: %v", err)
	}

	if !strings.Contains(stderr.String(), "Usage: biggz sync") {
		t.Errorf("expected help output to contain 'Usage: biggz sync', got: %s", stderr.String())
	}
}

// TestSync_UnknownFlag verifies that "biggz sync --unknown" exits with non-zero
// and prints an error to stderr.
func TestSync_UnknownFlag(t *testing.T) {
	cmd := goRunBiggz(t, "sync", "--unknown")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit code for unknown flag, got exit 0")
	}

	if !strings.Contains(stderr.String(), "unknown flag") {
		t.Errorf("expected error message containing 'unknown flag', got: %s", stderr.String())
	}
}
