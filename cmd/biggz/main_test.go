package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain_InvalidJSONInput verifies that malformed JSON input causes the CLI
// to exit with code 1 and print an error message to stderr.
func TestMain_InvalidJSONInput(t *testing.T) {
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = t.TempDir() // Use a temp dir to avoid side effects; the binary is run from the package dir

	// Actually, go run needs to be invoked from the package directory.
	// Let's use the proper approach: build and run a test binary.
	// Recreate: we must call from cmd/biggz directory
	cmd = exec.Command("go", "run", "C:\\Users\\USER\\Desktop\\biggz-ai\\cmd\\biggz")
	cmd.Stdin = strings.NewReader("this is not json")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Expect a non-zero exit code
	if err == nil {
		t.Fatal("expected non-zero exit code for invalid JSON, got exit 0")
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 1 {
			t.Errorf("expected exit code 1, got %d", exitErr.ExitCode())
		}
	} else {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}

	// stderr should contain an error message
	if stderr.Len() == 0 {
		t.Fatal("expected error message on stderr, got empty")
	}
	if !strings.Contains(stderr.String(), "error") {
		t.Errorf("expected stderr to contain 'error', got: %s", stderr.String())
	}
}

// TestMain_ValidJSONInput verifies that valid JSON input causes the CLI to exit
// with code 0. It creates a temporary git repo so the RiskLens can run its
// git commands successfully.
func TestMain_ValidJSONInput(t *testing.T) {
	repoDir := t.TempDir()

	// Initialize a git repo
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Test")
	runGit(t, repoDir, "config", "user.email", "test@test.com")

	// Create an initial commit so HEAD~1 exists
	readmePath := filepath.Join(repoDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# test"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "initial")

	// Make a change and commit it so git diff HEAD~1..HEAD has output
	if err := os.WriteFile(readmePath, []byte("# test\n\nchanged content"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "second commit")

	jsonSubject := fmt.Sprintf(`{"repository":"%s","commit_sha":"HEAD"}`, filepath.ToSlash(repoDir))
	cmd := exec.Command("go", "run", "C:\\Users\\USER\\Desktop\\biggz-ai\\cmd\\biggz")
	cmd.Stdin = strings.NewReader(jsonSubject)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("expected exit 0 for valid JSON, got error: %v (stderr: %s)", err, stderr.String())
	}
}

// TestSync_DryRun verifies that "biggz sync --dry-run" reports expected component
// names without writing any files and exits with code 0.
func TestSync_DryRun(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := exec.Command("go", "run", "C:\\Users\\USER\\Desktop\\biggz-ai\\cmd\\biggz", "sync", "--dry-run", "--home", tmpDir)
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

	cmd := exec.Command("go", "run", "C:\\Users\\USER\\Desktop\\biggz-ai\\cmd\\biggz", "sync", "--dry-run", "--skills", "--config", "--home", tmpDir)
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
	cmd := exec.Command("go", "run", "C:\\Users\\USER\\Desktop\\biggz-ai\\cmd\\biggz", "sync", "--help")
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
	cmd := exec.Command("go", "run", "C:\\Users\\USER\\Desktop\\biggz-ai\\cmd\\biggz", "sync", "--unknown")
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

// runGit is a helper that runs a git command in the given directory.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}

// TestMain_InvalidJSONInput_ErrorMessage verifies that the stderr message is
// descriptive enough to identify the problem.
func TestMain_InvalidJSONInput_ErrorMessage(t *testing.T) {
	cmd := exec.Command("go", "run", "C:\\Users\\USER\\Desktop\\biggz-ai\\cmd\\biggz")
	cmd.Stdin = strings.NewReader("{broken}")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit code, got exit 0")
	}

	errMsg := stderr.String()
	if !strings.Contains(errMsg, "parsing") && !strings.Contains(errMsg, "invalid") && !strings.Contains(errMsg, "JSON") {
		t.Errorf("expected descriptive error message containing 'parsing', 'invalid', or 'JSON', got: %s", errMsg)
	}
}
