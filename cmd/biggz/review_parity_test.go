package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── review start consent gate (Phase C1) ────────────────────────────────────

// chdir changes the working directory for the test and restores it on exit.
func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}

// runReviewStart invokes reviewStartRun in-process with the given args,
// capturing stdout and stderr through temp files.
func runReviewStart(t *testing.T, args []string) (code int, stdout, stderr string) {
	t.Helper()
	oldArgs := os.Args
	os.Args = append([]string{"biggz", "review", "start"}, args...)
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
	code = reviewStartRun()
	os.Stdout, os.Stderr = oldOut, oldErr
	outFile.Close()
	errFile.Close()
	outData, _ := os.ReadFile(outFile.Name())
	errData, _ := os.ReadFile(errFile.Name())
	return code, string(outData), string(errData)
}

// reviewSubjectFile writes a ReviewSubject JSON fixture for a repo dir.
func reviewSubjectFile(t *testing.T, repoDir string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "subject.json")
	subject := fmt.Sprintf(`{"repository":%q,"commit_sha":"HEAD"}`, filepath.ToSlash(repoDir))
	if err := os.WriteFile(path, []byte(subject), 0644); err != nil {
		t.Fatalf("write subject: %v", err)
	}
	return path
}

// lineageStoreDir is where review lineages live for a repo.
func lineageStoreDir(repoDir string) string {
	return filepath.Join(repoDir, ".git", "biggz", "review-transactions")
}

// gitRepoWithCommit creates a git repo with an initial commit.
func gitRepoWithCommit(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Test")
	runGit(t, repoDir, "config", "user.email", "test@test.com")
	readme := filepath.Join(repoDir, "README.md")
	if err := os.WriteFile(readme, []byte("# test\n\nchanged content"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "initial")
	return repoDir
}

func TestReviewStart_NoConsentWithLensesErrors(t *testing.T) {
	repoDir := gitRepoWithCommit(t)
	chdir(t, repoDir)
	subject := reviewSubjectFile(t, repoDir)

	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject, "--lenses", "risk"})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (stdout: %q)", code, stdout)
	}
	if !strings.Contains(stderr, "--consent relay") {
		t.Errorf("stderr should explain the relay flow, got: %s", stderr)
	}
	if _, err := os.Stat(lineageStoreDir(repoDir)); !os.IsNotExist(err) {
		t.Error("no lineage may be created when consent is missing")
	}
}

func TestReviewStart_RelayPrintsEnvelopeAndCreatesNoLineage(t *testing.T) {
	repoDir := gitRepoWithCommit(t)
	chdir(t, repoDir)
	subject := reviewSubjectFile(t, repoDir)

	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject, "--lenses", "risk", "--consent", "relay"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if stderr != "" {
		t.Errorf("relay should print the envelope on stdout only, got stderr: %s", stderr)
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("relay stdout is not JSON: %v\n%s", err, stdout)
	}
	if string(envelope["schema"]) != `"biggz-ai.review-consent/v1"` {
		t.Errorf("envelope schema = %s", envelope["schema"])
	}
	for _, key := range []string{"headline", "reason", "risk_evidence", "candidate", "choices", "off_path_note"} {
		if _, ok := envelope[key]; !ok {
			t.Errorf("envelope missing key %q", key)
		}
	}

	if _, err := os.Stat(lineageStoreDir(repoDir)); !os.IsNotExist(err) {
		t.Error("relay must not create the review store")
	}
}

func TestReviewStart_DeclinedPersistsNothing(t *testing.T) {
	repoDir := gitRepoWithCommit(t)
	chdir(t, repoDir)
	subject := reviewSubjectFile(t, repoDir)

	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject, "--lenses", "risk", "--consent", "declined"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "Review skipped for this candidate") {
		t.Errorf("stdout should carry the scoped-decline message, got: %q", stdout)
	}
	if _, err := os.Stat(lineageStoreDir(repoDir)); !os.IsNotExist(err) {
		t.Error("declined must not create the review store")
	}
}

func TestReviewStart_GrantedCreatesLineage(t *testing.T) {
	repoDir := gitRepoWithCommit(t)
	chdir(t, repoDir)
	subject := reviewSubjectFile(t, repoDir)

	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject, "--lenses", "risk", "--consent", "granted"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "Review started:") {
		t.Errorf("stdout should report the started review, got: %q", stdout)
	}

	entries, err := os.ReadDir(lineageStoreDir(repoDir))
	if err != nil {
		t.Fatalf("lineage store missing after granted start: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("lineages = %d, want 1", len(entries))
	}
}

func TestReviewStart_LowRiskSilent(t *testing.T) {
	repoDir := gitRepoWithCommit(t)
	chdir(t, repoDir)
	subject := reviewSubjectFile(t, repoDir)

	// No lenses and no consent: silent structural readback — the review
	// starts without any consent ceremony.
	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "Review started:") {
		t.Errorf("stdout should report the started review, got: %q", stdout)
	}
}

// ─── sdd-verify-validate (Phase C1) ──────────────────────────────────────────

const verifyReportFixture = "```yaml\nschema: biggz-ai.verify-result/v1\nverdict: pass\nblockers: 0\ncritical_findings: 0\nrequirements: 5/5\nscenarios: 10/10\ntest_exit_code: 0\nbuild_exit_code: 0\n```\n\n## Verification Report\n\n**CRITICAL**: None\n"

func runVerifyValidate(args []string, stdin string) (code int, stdout, stderr string) {
	var out, errBuf bytes.Buffer
	code = runSDDVerifyValidate(args, strings.NewReader(stdin), &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func TestSDDVerifyValidate_StdinAdmissionEnvelope(t *testing.T) {
	code, stdout, stderr := runVerifyValidate([]string{"--input", "-", "--requirements", "5", "--scenarios", "10", "--json"}, verifyReportFixture)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}

	var admission map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &admission); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if string(admission["schema"]) != `"biggz-ai.verify-admission/v1"` {
		t.Errorf("schema = %s", admission["schema"])
	}
	if string(admission["decision"]) != `"admitted"` {
		t.Errorf("decision = %s", admission["decision"])
	}
	var requirements struct {
		Declared int `json:"declared"`
		Counted  int `json:"counted"`
	}
	if err := json.Unmarshal(admission["requirements"], &requirements); err != nil {
		t.Fatalf("requirements pair: %v", err)
	}
	if requirements.Declared != 5 || requirements.Counted != 5 {
		t.Errorf("requirements = %+v, want declared 5 counted 5", requirements)
	}
}

func TestSDDVerifyValidate_LegacySchemaStillAdmitted(t *testing.T) {
	legacy := strings.Replace(verifyReportFixture, "biggz-ai.verify-result/v1", "gentle-ai.verify-result/v1", 1)
	code, stdout, stderr := runVerifyValidate([]string{"--input", "-", "--requirements", "5", "--scenarios", "10", "--json"}, legacy)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 for legacy schema (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, `"decision": "admitted"`) {
		t.Errorf("legacy report should be admitted, got: %s", stdout)
	}
}

func TestSDDVerifyValidate_CountsMismatchDenied(t *testing.T) {
	code, stdout, _ := runVerifyValidate([]string{"--input", "-", "--requirements", "7", "--scenarios", "10", "--json"}, verifyReportFixture)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	var admission struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(stdout), &admission); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if admission.Decision != "denied" {
		t.Errorf("decision = %q, want denied", admission.Decision)
	}
	if !strings.Contains(admission.Reason, "requirements") {
		t.Errorf("reason should name the requirements mismatch, got: %q", admission.Reason)
	}
}

func TestSDDVerifyValidate_CountsMismatchDeniedHumanMode(t *testing.T) {
	code, stdout, stderr := runVerifyValidate([]string{"--input", "-", "--requirements", "7", "--scenarios", "10"}, verifyReportFixture)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "requirements count 5") {
		t.Errorf("stderr should name the mismatch, got: %s", stderr)
	}
	if stdout != "" {
		t.Errorf("human-mode denial should not write stdout, got: %q", stdout)
	}
}

func TestSDDVerifyValidate_HumanOutputWhenNoJSON(t *testing.T) {
	code, stdout, stderr := runVerifyValidate([]string{"--input", "-", "--requirements", "5", "--scenarios", "10"}, verifyReportFixture)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if strings.TrimSpace(stdout) != "Verify report is valid." {
		t.Errorf("stdout = %q, want human-readable validity line", stdout)
	}
}

func TestSDDVerifyValidate_CountsMustBeProvidedTogether(t *testing.T) {
	code, _, stderr := runVerifyValidate([]string{"--input", "-", "--requirements", "5"}, verifyReportFixture)
	if code != 1 || !strings.Contains(stderr, "together") {
		t.Fatalf("code = %d stderr = %q, want 1 with together error", code, stderr)
	}
}

func TestSDDVerifyValidate_CountsMayBothBeOmittedLenient(t *testing.T) {
	code, _, stderr := runVerifyValidate([]string{"--input", "-"}, verifyReportFixture)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 in lenient mode (stderr: %s)", code, stderr)
	}
}

func TestSDDVerifyValidate_InputTooLarge(t *testing.T) {
	payload := strings.Repeat("x", 1<<20+1)
	code, _, stderr := runVerifyValidate([]string{"--input", "-", "--requirements", "1", "--scenarios", "1"}, payload)
	if code != 1 || !strings.Contains(stderr, "exceeds") {
		t.Fatalf("code = %d stderr = %q, want 1 with size error", code, stderr)
	}
}

func TestSDDVerifyValidate_InvalidCountValue(t *testing.T) {
	code, _, stderr := runVerifyValidate([]string{"--input", "-", "--requirements", "abc", "--scenarios", "1"}, verifyReportFixture)
	if code != 1 || !strings.Contains(stderr, "invalid --requirements") {
		t.Fatalf("code = %d stderr = %q, want 1 with invalid count error", code, stderr)
	}
}

func TestSDDVerifyValidate_FileInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "verify-report.md")
	if err := os.WriteFile(path, []byte(verifyReportFixture), 0644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	code, stdout, stderr := runVerifyValidate([]string{"--input", path, "--requirements", "5", "--scenarios", "10", "--json"}, "unused")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, `"decision": "admitted"`) {
		t.Errorf("stdout should admit, got: %s", stdout)
	}
}
