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
// end-to-end fixture: sdd-status prints the blocked(edit_authority_missing)
// reason with the typed consent envelope's granted invocation, rerunning
// that exact invocation through `biggz sdd-attempt grant` exits 0 and
// persists the grant, and a second status projects the granted root and
// clears the block.
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

	code, stdout, stderr := runSDDStatusCLI(t)
	if code != 0 {
		t.Fatalf("blocked status exit code = %d (stderr: %q)", code, stderr)
	}
	if !strings.Contains(stdout, "blocked(edit_authority_missing)") {
		t.Fatalf("blocked status stdout lacks the reason code: %s", stdout)
	}
	if !strings.Contains(stdout, "consent grant: biggz sdd-attempt grant multi-repo-rollout ") ||
		!strings.Contains(stdout, "--change-instance ") {
		t.Fatalf("blocked status stdout lacks the runnable granted invocation: %s", stdout)
	}

	// Extract the granted invocation and rerun it through the real grant
	// verb: exit 0 and a committed revision.
	invocation := ""
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "consent grant: ") {
			invocation = strings.TrimPrefix(line, "consent grant: ")
			break
		}
	}
	if invocation == "" {
		t.Fatalf("no consent grant invocation in status stdout: %s", stdout)
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

	// Second status projects the granted root and clears the block.
	code, stdout, stderr = runSDDStatusCLI(t)
	if code != 0 {
		t.Fatalf("post-grant status exit code = %d (stderr: %q)", code, stderr)
	}
	if strings.Contains(stdout, "blocked(edit_authority_missing)") {
		t.Fatalf("post-grant status is still blocked: %s", stdout)
	}
	if strings.Contains(stdout, "consent grant:") {
		t.Fatalf("post-grant status still prints the consent envelope: %s", stdout)
	}
}
