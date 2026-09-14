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
	sibling, err := filepath.EvalSymlinks(sibling)
	if err != nil {
		t.Fatalf("EvalSymlinks sibling: %v", err)
	}

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
	second, err = filepath.EvalSymlinks(second)
	if err != nil {
		t.Fatalf("EvalSymlinks second: %v", err)
	}
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

// TestSDDAttemptStatusGenerationAndResetPrint proves the two visible surface
// fixes of this change: `status` prints the live generation and the derived
// lifetime totals (a maintainer can see a successor generation and the
// change-wide accounting), and `reset` reports the attempts it PRESERVES —
// the chain survives the reset — instead of claiming they were cleared.
func TestSDDAttemptStatusGenerationAndResetPrint(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	chdir(t, t.TempDir())

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	change := "cli-generation"

	// Seed generation 1 through the package API (the CLI exposes no
	// --changed-lines flag), then advance to generation 2 with a distinct
	// work unit exactly as the runtime dogfood does.
	acq1, err := sddattempt.Acquire(sddattempt.AcquireParams{
		ChangeName: change, RepoRoot: cwd, RequestID: "gen-seed-1",
		WorkUnit: "apply", EvidenceGoal: "apply the slice",
		MaxAttempts: 3, MaxLines: 400, ChangedLines: 120,
	})
	if err != nil {
		t.Fatalf("Acquire generation 1: %v", err)
	}
	if _, err := sddattempt.Settle(sddattempt.SettleParams{
		ChangeName: change, RepoRoot: cwd, Token: acq1.Token, RequestID: "gen-seed-1-settle",
		Outcome: "passed", EvidenceRevision: "sha256:" + strings.Repeat("a", 64),
		Diagnosis: "passed", ChangedLines: 120,
	}); err != nil {
		t.Fatalf("Settle generation 1: %v", err)
	}
	acq2, err := sddattempt.Acquire(sddattempt.AcquireParams{
		ChangeName: change, RepoRoot: cwd, RequestID: "gen-seed-2",
		WorkUnit: "verify", EvidenceGoal: "verify the slice",
		MaxAttempts: 3, MaxLines: 400, ChangedLines: 30,
	})
	if err != nil {
		t.Fatalf("Acquire successor generation: %v", err)
	}
	if _, err := sddattempt.Settle(sddattempt.SettleParams{
		ChangeName: change, RepoRoot: cwd, Token: acq2.Token, RequestID: "gen-seed-2-settle",
		Outcome: "passed", EvidenceRevision: "sha256:" + strings.Repeat("b", 64),
		Diagnosis: "passed", ChangedLines: 30,
	}); err != nil {
		t.Fatalf("Settle successor generation: %v", err)
	}

	// Status shows the live generation and the change-wide totals.
	code, stdout, stderr := runSDDAttemptCLI(t, "status", change)
	if code != 0 {
		t.Fatalf("status exit code = %d (stderr: %q)", code, stderr)
	}
	for _, want := range []string{
		"Generation:        2",
		"Lifetime attempts: 2",
		"Lifetime lines:    150",
		"Complete:         true",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("status stdout missing %q:\n%s", want, stdout)
		}
	}
	// The legitimately complete ledger names its successor route instead of
	// claiming a corrupt authority: the status projection and the acquire
	// path tell the same story.
	for _, want := range []string{
		"Blocked reason:   work_unit_complete",
		`work unit "verify" is complete`,
		"with a different --work-unit",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("complete status stdout missing %q:\n%s", want, stdout)
		}
	}

	// Reset discards the live scope and PRESERVES the chain; the printed
	// count must match the chain that actually survives.
	code, stdout, stderr = runSDDAttemptCLI(t, "reset", change, "--reason", "dogfood discard", "--request-id", "gen-reset")
	if code != 0 {
		t.Fatalf("reset exit code = %d (stderr: %q)", code, stderr)
	}
	if !strings.Contains(stdout, "Previous attempts preserved: 2") {
		t.Fatalf("reset stdout missing the preserved count:\n%s", stdout)
	}
	if strings.Contains(stdout, "cleared") {
		t.Fatalf("reset stdout still claims attempts were cleared:\n%s", stdout)
	}
	store, err := sddattempt.LoadStore(change, cwd)
	if err != nil {
		t.Fatalf("LoadStore after reset: %v", err)
	}
	if len(store.Attempts) != 2 {
		t.Fatalf("attempts after reset = %d, want 2 (the chain survives, matching the printed count)", len(store.Attempts))
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
