package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runSDDStatusCLI invokes sddStatusRun in-process, capturing stdout and
// stderr through temp files.
func runSDDStatusCLI(t *testing.T) (code int, stdout, stderr string) {
	t.Helper()
	oldArgs := os.Args
	os.Args = []string{"biggz", "sdd-status"}
	defer func() { os.Args = oldArgs }()

	outFile, err := os.CreateTemp(t.TempDir(), "stdout-*")
	if err != nil {
		t.Fatalf("create stdout capture: %v", err)
	}
	errFile, err := os.CreateTemp(t.TempDir(), "stderr-*")
	if err != nil {
		t.Fatalf("create stderr capture: %v", err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outFile, errFile
	code = sddStatusRun()
	os.Stdout, os.Stderr = oldOut, oldErr
	outFile.Close()
	errFile.Close()
	outData, _ := os.ReadFile(outFile.Name())
	errData, _ := os.ReadFile(errFile.Name())
	return code, string(outData), string(errData)
}

// gitInit creates a git repository at dir, skipping the test when git is
// unavailable.
func gitInit(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not available: %v", err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v (%s)", dir, err, out)
	}
}

// TestSDDStatusBlockedPrintsEnvelopeAndGrantRerunClearsIt is the WU5
// end-to-end fixture for V2 (gentle v2.5.0-rc.1 I6): sdd-status is
// authority-free and never blocks on edit_authority_missing. The status
// projection filters blocked(edit_authority_missing) and the human view
// does not print the block. The native guard `biggz sdd-apply` still
// blocks and prints the typed consent envelope; rerunning that exact
// invocation through `biggz sdd-attempt grant` persists the grant and
// a second guard run passes. A second status remains unblocked (V2).
func TestSDDStatusBlockedPrintsEnvelopeAndGrantRerunClearsIt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())

	workspace := t.TempDir()
	planning := filepath.Join(workspace, "planning")
	serviceA := filepath.Join(workspace, "service-a")
	gitInit(t, planning)
	gitInit(t, serviceA)

	changeRoot := filepath.Join(planning, "openspec", "changes", "multi-repo-rollout")
	if err := os.MkdirAll(changeRoot, 0755); err != nil {
		t.Fatalf("mkdir change root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(changeRoot, "proposal.md"), []byte("# Proposal\n"), 0644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(changeRoot, "tasks.md"), []byte(
		"- [ ] 1.1 Update `../service-a/internal/api/handler.go` to accept the new header\n",
	), 0644); err != nil {
		t.Fatalf("write tasks: %v", err)
	}
	chdir(t, planning)

	// V2: sdd-status is authority-free — never prints blocked for edit_authority_missing.
	code, stdout, stderr := runSDDStatusCLI(t)
	if code != 0 {
		t.Fatalf("status exit code = %d (stderr: %q)", code, stderr)
	}
	if strings.Contains(stdout, "blocked(edit_authority_missing)") {
		t.Fatalf("V2 status must not print blocked(edit_authority_missing), got: %s", stdout)
	}
	if strings.Contains(stdout, "consent grant:") {
		t.Fatalf("V2 status must not print consent envelope, got: %s", stdout)
	}

	// The native apply-side guard still blocks and prints the envelope.
	gCode, gOut, gErr := runSDDApplyCLI(t, "multi-repo-rollout")
	if gCode != 1 {
		t.Fatalf("blocked guard exit code = %d, want 1 (stderr: %q, stdout: %q)", gCode, gErr, gOut)
	}
	if !strings.Contains(gOut, "blocked(edit_authority_missing)") {
		t.Fatalf("blocked guard stdout lacks the reason code: %s", gOut)
	}
	if !strings.Contains(gOut, "consent grant: biggz sdd-attempt grant multi-repo-rollout ") ||
		!strings.Contains(gOut, "--change-instance ") {
		t.Fatalf("blocked guard stdout lacks the runnable granted invocation: %s", gOut)
	}

	// Extract the granted invocation and rerun it through the real grant
	// verb: exit 0 and a committed revision.
	invocation := ""
	for _, line := range strings.Split(gOut, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "consent grant: ") {
			invocation = strings.TrimPrefix(line, "consent grant: ")
			break
		}
	}
	if invocation == "" {
		t.Fatalf("no consent grant invocation in guard stdout: %s", gOut)
	}
	fields := strings.Fields(invocation)
	if len(fields) < 3 || fields[0] != "biggz" || fields[1] != "sdd-attempt" || fields[2] != "grant" {
		t.Fatalf("granted invocation is not a runnable biggz sdd-attempt grant command: %q", invocation)
	}
	grantCode, grantOut, grantErr := runSDDAttemptCLI(t, fields[2:]...)
	if grantCode != 0 {
		t.Fatalf("granted invocation rerun exit code = %d, want 0 (stderr: %q, invocation %q)", grantCode, grantErr, invocation)
	}
	if !strings.Contains(grantOut, `"revision"`) {
		t.Fatalf("granted invocation rerun did not emit a GrantResult envelope: %s", grantOut)
	}

	// Second guard run projects the granted root and clears the block.
	gCode, gOut, gErr = runSDDApplyCLI(t, "multi-repo-rollout")
	if gCode != 0 {
		t.Fatalf("post-grant guard exit code = %d, want 0 (stderr: %q, stdout: %q)", gCode, gErr, gOut)
	}
	if !strings.Contains(gOut, "edit authority OK") {
		t.Fatalf("post-grant guard is not allowed: %s", gOut)
	}

	// Second status remains unblocked (V2 authority-free).
	code, stdout, stderr = runSDDStatusCLI(t)
	if code != 0 {
		t.Fatalf("post-grant status exit code = %d (stderr: %q)", code, stderr)
	}
	if strings.Contains(stdout, "blocked(edit_authority_missing)") {
		t.Fatalf("post-grant V2 status must not be blocked: %s", stdout)
	}
}
