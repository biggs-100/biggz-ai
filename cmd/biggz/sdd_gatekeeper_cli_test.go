package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSddGatekeeperCLI_Help(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := runSddGatekeeper([]string{"--help"}, stdout, stderr)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Usage: biggz sdd-gatekeeper") {
		t.Errorf("expected help text in stderr, got: %s", stderr.String())
	}
}

func TestSddGatekeeperCLI_Pass(t *testing.T) {
	// Setup: create a change with proposal.md
	tmpDir := t.TempDir()
	changeDir := filepath.Join(tmpDir, "openspec", "changes", "test-change")
	os.MkdirAll(changeDir, 0755)
	os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Proposal\n\n## Intent\n\nTest\n"), 0644)

	// Change to temp dir
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	resultJSON := `{"status":"success","executive_summary":"Done","artifacts":[{"path":"proposal.md"}],"next_recommended":"spec"}`
	code := runSddGatekeeper([]string{"test-change", "explore", "--result", resultJSON}, stdout, stderr)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "gatekeeper PASS") {
		t.Errorf("expected PASS in output, got: %s", stdout.String())
	}
}

func TestSddGatekeeperCLI_Fail(t *testing.T) {
	// Setup: create a change WITHOUT proposal.md
	tmpDir := t.TempDir()
	changeDir := filepath.Join(tmpDir, "openspec", "changes", "test-change")
	os.MkdirAll(changeDir, 0755)

	// Change to temp dir
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	resultJSON := `{"status":"success","executive_summary":"Done","artifacts":[{"path":"proposal.md"}],"next_recommended":"spec"}`
	code := runSddGatekeeper([]string{"test-change", "explore", "--result", resultJSON}, stdout, stderr)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "gatekeeper FAIL") {
		t.Errorf("expected FAIL in output, got: %s", stdout.String())
	}
}

func TestSddGatekeeperCLI_MissingArgs(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := runSddGatekeeper([]string{}, stdout, stderr)
	// With no args, shows help and returns 0
	if code != 0 {
		t.Errorf("expected exit code 0 (help), got %d", code)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("expected help text, got: %s", stderr.String())
	}
}

func TestSddGatekeeperCLI_MissingResult(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	code := runSddGatekeeper([]string{"test-change", "explore"}, stdout, stderr)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "--result is required") {
		t.Errorf("expected --result error, got: %s", stderr.String())
	}
}
