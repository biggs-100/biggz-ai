package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runSDDApplyCLI invokes sddApplyRun in-process with the given args
// (excluding the "biggz sdd-apply" prefix), capturing stdout and stderr
// through temp files.
func runSDDApplyCLI(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	oldArgs := os.Args
	os.Args = append([]string{"biggz", "sdd-apply"}, args...)
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
	code = sddApplyRun()
	os.Stdout, os.Stderr = oldOut, oldErr
	outFile.Close()
	errFile.Close()
	outData, _ := os.ReadFile(outFile.Name())
	errData, _ := os.ReadFile(errFile.Name())
	return code, string(outData), string(errData)
}

// seedApplyChange writes a minimal change (proposal + tasks) into a planning
// workspace and returns the change's own directory.
func seedApplyChange(t *testing.T, planning, name, tasks string) string {
	t.Helper()
	changeRoot := filepath.Join(planning, "openspec", "changes", name)
	if err := os.MkdirAll(changeRoot, 0755); err != nil {
		t.Fatalf("mkdir change root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(changeRoot, "proposal.md"), []byte("# Proposal\n"), 0644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(changeRoot, "tasks.md"), []byte(tasks), 0644); err != nil {
		t.Fatalf("write tasks: %v", err)
	}
	return changeRoot
}

// TestSDDApplyGuardBlockedPrintsConsentAndGrantClearsIt is the Work Unit A
// end-to-end fixture for the native apply-side guard: a change whose tasks
// target an outside-root repository exits non-zero with the
// blocked(edit_authority_missing) reason and the typed consent envelope's
// granted invocation, rerunning that exact invocation through
// `biggz sdd-attempt grant` exits 0 and persists the grant, and a second
// guard run passes with the granted root projected into the allowed roots.
func TestSDDApplyGuardBlockedPrintsConsentAndGrantClearsIt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())

	workspace := t.TempDir()
	planning := filepath.Join(workspace, "planning")
	serviceA := filepath.Join(workspace, "service-a")
	gitInit(t, planning)
	gitInit(t, serviceA)

	change := "multi-repo-rollout"
	seedApplyChange(t, planning, change,
		"- [ ] 1.1 Update `../service-a/internal/api/handler.go` to accept the new header\n")
	chdir(t, planning)

	code, stdout, stderr := runSDDApplyCLI(t, change)
	if code != 1 {
		t.Fatalf("blocked guard exit code = %d, want 1 (stderr: %q)", code, stderr)
	}
	if !strings.Contains(stdout, "blocked(edit_authority_missing)") {
		t.Fatalf("blocked guard stdout lacks the reason code: %s", stdout)
	}
	if !strings.Contains(stdout, "consent grant: biggz sdd-attempt grant "+change+" ") ||
		!strings.Contains(stdout, "--change-instance ") {
		t.Fatalf("blocked guard stdout lacks the runnable granted invocation: %s", stdout)
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
		t.Fatalf("no consent grant invocation in guard stdout: %s", stdout)
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
	code, stdout, stderr = runSDDApplyCLI(t, change)
	if code != 0 {
		t.Fatalf("post-grant guard exit code = %d, want 0 (stderr: %q)", code, stderr)
	}
	if !strings.Contains(stdout, "edit authority OK") {
		t.Fatalf("post-grant guard is not allowed: %s", stdout)
	}
	if !strings.Contains(stdout, filepath.Base(serviceA)) {
		t.Fatalf("post-grant guard allowed roots lack the granted repository %q: %s", filepath.Base(serviceA), stdout)
	}
	if strings.Contains(stdout, "blocked(edit_authority_missing)") {
		t.Fatalf("post-grant guard still prints the block: %s", stdout)
	}
}

// TestSDDApplyGuardAllowsSingleRepoChange proves the no-false-positive
// contract at the CLI: a change whose tasks stay inside the planning
// repository passes the guard with exit 0 and no block.
func TestSDDApplyGuardAllowsSingleRepoChange(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())

	planning := t.TempDir()
	gitInit(t, planning)
	change := "single-repo-change"
	seedApplyChange(t, planning, change,
		"- [ ] 1.1 Update `internal/auth/login.go` with the new claim check\n")
	chdir(t, planning)

	code, stdout, stderr := runSDDApplyCLI(t, change)
	if code != 0 {
		t.Fatalf("single-repo guard exit code = %d, want 0 (stderr: %q)", code, stderr)
	}
	if !strings.Contains(stdout, "edit authority OK") {
		t.Fatalf("single-repo guard stdout lacks the allow line: %s", stdout)
	}
	if strings.Contains(stdout, "blocked(edit_authority_missing)") {
		t.Fatalf("single-repo guard stdout is blocked: %s", stdout)
	}
}

// TestSDDApplyGuardUnknownChangeFails proves the guard refuses a change
// that does not exist: non-zero exit and a named error on stderr.
func TestSDDApplyGuardUnknownChangeFails(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())

	planning := t.TempDir()
	gitInit(t, planning)
	if err := os.MkdirAll(filepath.Join(planning, "openspec", "changes"), 0755); err != nil {
		t.Fatalf("mkdir openspec/changes: %v", err)
	}
	chdir(t, planning)

	code, _, stderr := runSDDApplyCLI(t, "ghost-change")
	if code != 1 {
		t.Fatalf("unknown-change guard exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "error") || !strings.Contains(stderr, "ghost-change") {
		t.Fatalf("unknown-change stderr = %q, want a named error", stderr)
	}
}
