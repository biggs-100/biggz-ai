package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/sdd"
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

// TestSddGatekeeperCLI_StoreFromPreflight pins the CLI wiring: the active
// store comes from ResolvePreflightPrefs(cwd), so a "none" preflight makes
// the artifact check report a skip with a reason (never a silent pass) and
// routing_coherence fail closed naming the missing store.
func TestSddGatekeeperCLI_StoreFromPreflight(t *testing.T) {
	tmpDir := t.TempDir()
	changeDir := filepath.Join(tmpDir, "openspec", "changes", "test-change")
	if err := os.MkdirAll(changeDir, 0755); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	sdd.SetPreflightPrefs(cwd, sdd.PreflightPrefs{ArtifactStore: "none"})
	defer sdd.ClearPreflightPrefs(cwd)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	resultJSON := `{"status":"success","executive_summary":"Done","artifacts":[{"path":"sdd/test-change/proposal"}],"next_recommended":"spec"}`
	code := runSddGatekeeper([]string{"test-change", "explore", "--result", resultJSON}, stdout, stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1 with routing unresolved under store none, got %d: %s", code, stdout.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "routing_coherence") || !strings.Contains(out, "artifact store is none") {
		t.Errorf("expected routing_coherence to fail naming the missing store, got: %s", out)
	}
	if !strings.Contains(out, "artifact_existence") || !strings.Contains(out, `"skipped": true`) {
		t.Errorf("expected a skipped artifact check naming its reason, got: %s", out)
	}
}
