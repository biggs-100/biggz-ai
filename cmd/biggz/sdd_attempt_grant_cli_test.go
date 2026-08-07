package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/sddattempt"
)

// runSDDAttemptCLI invokes sddAttemptRun in-process with the given args
// (excluding the "biggz sdd-attempt" prefix), capturing stdout and stderr
// through temp files.
func runSDDAttemptCLI(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	oldArgs := os.Args
	os.Args = append([]string{"biggz", "sdd-attempt"}, args...)
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
	code = sddAttemptRun()
	os.Stdout, os.Stderr = oldOut, oldErr
	outFile.Close()
	errFile.Close()
	outData, _ := os.ReadFile(outFile.Name())
	errData, _ := os.ReadFile(errFile.Name())
	return code, string(outData), string(errData)
}

// TestSDDAttemptGrantPersistsAndReplaysThroughCLI is the CLI dispatch proof
// for the grant verb: a first grant on a fresh ledger needs no
// --expected-revision but always needs --change-instance, the persisted
// roots round-trip through `sdd-attempt status --change-instance`, an exact
// duplicate --request-id replays the committed revision idempotently, and a
// widening grant reusing the SAME instance token chains --expected-revision
// on the first and accumulates roots in grant order, deduplicating
// already-granted ones. A different token (a recreated change) projects
// nothing.
func TestSDDAttemptGrantPersistsAndReplaysThroughCLI(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	chdir(t, t.TempDir())

	change := "cli-grant"
	instance := "cli-grant-instance-token"
	sibling := filepath.Clean(t.TempDir())

	grantArgs := []string{
		"grant", change, "--root", sibling, "--change-instance", instance,
		"--actor", "maintainer", "--reason", "sequential multi-repository rollout", "--request-id", "cli-grant-1",
	}
	code, stdout, stderr := runSDDAttemptCLI(t, grantArgs...)
	if code != 0 {
		t.Fatalf("grant exit code = %d, want 0 (stderr: %q)", code, stderr)
	}
	var granted sddattempt.GrantResult
	if err := json.Unmarshal([]byte(stdout), &granted); err != nil {
		t.Fatalf("grant stdout is not a GrantResult JSON envelope: %v\n%s", err, stdout)
	}
	if len(granted.GrantedRoots) != 1 || granted.GrantedRoots[0] != sibling || granted.Revision == "" {
		t.Fatalf("grant CLI result = %#v, want granted root %q with a committed revision", granted, sibling)
	}

	// Status with the same instance token replays the persisted chain: the
	// grant survives the process boundary between the mutating call and a
	// later read that declares the instance it serves.
	code, stdout, stderr = runSDDAttemptCLI(t, "status", change, "--change-instance", instance)
	if code != 0 || !strings.Contains(stdout, "Granted roots:") || !strings.Contains(stdout, sibling) {
		t.Fatalf("post-grant status code=%d stdout=%q stderr=%q, want the granted roots line naming %q", code, stdout, stderr, sibling)
	}

	// Status WITHOUT an instance declaration projects no granted roots: the
	// conservative containment for undeclared readers.
	code, stdout, stderr = runSDDAttemptCLI(t, "status", change)
	if code != 0 || strings.Contains(stdout, "Granted roots:") {
		t.Fatalf("undeclared-instance status code=%d stdout=%q stderr=%q, want no granted roots projection", code, stdout, stderr)
	}

	// Status under a DIFFERENT instance token projects nothing either: a
	// recreated change reusing this archived name inherits no authority.
	code, stdout, stderr = runSDDAttemptCLI(t, "status", change, "--change-instance", "recreated-instance-token")
	if code != 0 || strings.Contains(stdout, "Granted roots:") {
		t.Fatalf("recreated-instance status code=%d stdout=%q stderr=%q, want no granted roots projection", code, stdout, stderr)
	}

	// An exact duplicate request-id is idempotent through the CLI: same
	// committed revision, no second record.
	code, stdout, stderr = runSDDAttemptCLI(t, grantArgs...)
	if code != 0 {
		t.Fatalf("grant CLI replay exit code = %d (stderr: %q)", code, stderr)
	}
	var replayed sddattempt.GrantResult
	if err := json.Unmarshal([]byte(stdout), &replayed); err != nil {
		t.Fatalf("replay stdout is not a GrantResult JSON envelope: %v", err)
	}
	if replayed.Revision != granted.Revision || !reflect.DeepEqual(replayed.GrantedRoots, []string{sibling}) {
		t.Fatalf("grant CLI replay = %#v, want committed revision %s", replayed, granted.Revision)
	}

	// A widening grant reusing the SAME instance token chains on the first
	// revision, accumulates the new root after the already-granted one, and
	// deduplicates the repeat. The root arrives shell-quoted to prove the
	// CLI tolerates quoted values from the consent envelope invocation.
	second := filepath.Clean(t.TempDir())
	code, stdout, stderr = runSDDAttemptCLI(t,
		"grant", change,
		"--expected-revision", granted.Revision,
		"--root", `"`+second+`"`,
		"--root", sibling,
		"--change-instance", instance,
		"--actor", "maintainer", "--reason", "maintainer widened the change", "--request-id", "cli-grant-2",
	)
	if code != 0 {
		t.Fatalf("widening grant exit code = %d (stderr: %q)", code, stderr)
	}
	var widened sddattempt.GrantResult
	if err := json.Unmarshal([]byte(stdout), &widened); err != nil {
		t.Fatalf("widening grant stdout is not a GrantResult JSON envelope: %v", err)
	}
	if !reflect.DeepEqual(widened.GrantedRoots, []string{sibling, second}) {
		t.Fatalf("widened granted roots = %#v, want [%q %q]", widened.GrantedRoots, sibling, second)
	}
}

// TestSDDAttemptGrantMissingFlags pins the missing-flag refusal: it
// enumerates every missing flag and names the rerunnable continuation.
func TestSDDAttemptGrantMissingFlags(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	chdir(t, t.TempDir())

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "no flags",
			args: []string{"grant", "thin"},
			want: "sdd-attempt grant requires --root, --change-instance, --request-id, --actor, --reason; rerun `biggz sdd-attempt grant` with those missing flags",
		},
		{
			name: "only root",
			args: []string{"grant", "thin", "--root", t.TempDir()},
			want: "sdd-attempt grant requires --change-instance, --request-id, --actor, --reason; rerun `biggz sdd-attempt grant` with those missing flags",
		},
		{
			name: "missing audit fields",
			args: []string{"grant", "thin", "--root", t.TempDir(), "--change-instance", "token"},
			want: "sdd-attempt grant requires --request-id, --actor, --reason; rerun `biggz sdd-attempt grant` with those missing flags",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runSDDAttemptCLI(t, tt.args...)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if stdout != "" {
				t.Fatalf("refusal wrote to stdout: %q", stdout)
			}
			if !strings.Contains(stderr, tt.want) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr, tt.want)
			}
		})
	}
}
