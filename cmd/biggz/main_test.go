package main

import (
	"bytes"
	"os/exec"
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
// with code 0.
func TestMain_ValidJSONInput(t *testing.T) {
	cmd := exec.Command("go", "run", "C:\\Users\\USER\\Desktop\\biggz-ai\\cmd\\biggz")
	cmd.Stdin = strings.NewReader(`{"repository":"test/repo","commit_sha":"abc123"}`)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("expected exit 0 for valid JSON, got error: %v (stderr: %s)", err, stderr.String())
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
