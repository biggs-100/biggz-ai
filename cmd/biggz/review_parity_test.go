package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/biggz-ai/biggz/internal/review"
)

// ─── review start consent gate (Phase D1) ────────────────────────────────────

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

// runGitOutput runs a git command in the given directory and returns trimmed
// stdout.
func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
	return strings.TrimSpace(string(out))
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

// gitRepoWithAuthCommit creates a git repo whose head commit touches a
// sensitive domain (internal/auth/), classifying the head as high risk.
func gitRepoWithAuthCommit(t *testing.T) string {
	t.Helper()
	repoDir := gitRepoWithCommit(t)
	authDir := filepath.Join(repoDir, "internal", "auth")
	if err := os.MkdirAll(authDir, 0755); err != nil {
		t.Fatalf("mkdir auth: %v", err)
	}
	if err := os.WriteFile(filepath.Join(authDir, "token.go"), []byte("package auth\n\nfunc Issue() string { return \"\" }\n"), 0644); err != nil {
		t.Fatalf("write token.go: %v", err)
	}
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "auth change")
	return repoDir
}

// startedLineageID extracts the lineage ID from a successful start's stdout.
func startedLineageID(t *testing.T, stdout string) string {
	t.Helper()
	fields := strings.Fields(stdout)
	if len(fields) < 3 || fields[0] != "Review" || !strings.HasPrefix(fields[1], "started:") {
		t.Fatalf("cannot parse lineage from start output: %q", stdout)
	}
	return fields[2]
}

// frozenStartPlan reads the start_review genesis payload of a lineage.
func frozenStartPlan(t *testing.T, repoDir, lineageID string) review.StartEventPayload {
	t.Helper()
	auth := review.NewAuthority(repoDir)
	chain, err := auth.LoadChain(lineageID)
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	if chain.Count == 0 {
		t.Fatal("lineage has no events")
	}
	var plan review.StartEventPayload
	if err := json.Unmarshal(chain.Records[0].Payload, &plan); err != nil {
		t.Fatalf("unmarshal genesis: %v", err)
	}
	return plan
}

func TestReviewStart_NoConsentHighRiskErrors(t *testing.T) {
	repoDir := gitRepoWithAuthCommit(t)
	chdir(t, repoDir)
	subject := reviewSubjectFile(t, repoDir)

	// A sensitive-domain change is high risk with no declared lenses: consent
	// is still required, and headless it errors.
	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject})
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
	repoDir := gitRepoWithAuthCommit(t)
	chdir(t, repoDir)
	subject := reviewSubjectFile(t, repoDir)

	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject, "--consent", "relay"})
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
	var candidate struct {
		Risk   string   `json:"risk"`
		Lenses []string `json:"lenses"`
	}
	if err := json.Unmarshal(envelope["candidate"], &candidate); err != nil {
		t.Fatalf("candidate: %v", err)
	}
	if candidate.Risk != "high" {
		t.Errorf("candidate risk = %q, want high", candidate.Risk)
	}
	if want := []string{"risk", "readability", "reliability", "resilience"}; !reflect.DeepEqual(candidate.Lenses, want) {
		t.Errorf("candidate lenses = %v, want the planned 4R %v", candidate.Lenses, want)
	}

	if _, err := os.Stat(lineageStoreDir(repoDir)); !os.IsNotExist(err) {
		t.Error("relay must not create the review store")
	}
}

func TestReviewStart_DeclinedPersistsNothing(t *testing.T) {
	repoDir := gitRepoWithAuthCommit(t)
	chdir(t, repoDir)
	subject := reviewSubjectFile(t, repoDir)

	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject, "--consent", "declined"})
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
	repoDir := gitRepoWithAuthCommit(t)
	chdir(t, repoDir)
	subject := reviewSubjectFile(t, repoDir)

	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject, "--consent", "granted"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "Review started:") {
		t.Errorf("stdout should report the started review, got: %q", stdout)
	}
	if !strings.Contains(stdout, "risk tier: high") {
		t.Errorf("stdout should report the classifier tier, got: %q", stdout)
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
	if !strings.Contains(stdout, "risk tier: low") {
		t.Errorf("stdout should report the low tier, got: %q", stdout)
	}
}

func TestReviewStart_WithoutLensesFreezesPlannedSelection(t *testing.T) {
	repoDir := gitRepoWithAuthCommit(t)
	chdir(t, repoDir)
	subject := reviewSubjectFile(t, repoDir)

	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject, "--consent", "granted"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	lineageID := startedLineageID(t, stdout)
	plan := frozenStartPlan(t, repoDir, lineageID)

	if plan.RiskTier != "high" {
		t.Errorf("frozen risk_tier = %q, want high", plan.RiskTier)
	}
	want4R := []string{"risk", "readability", "reliability", "resilience"}
	if !reflect.DeepEqual(plan.SelectedLenses, want4R) {
		t.Errorf("frozen lenses = %v, want the planned 4R %v", plan.SelectedLenses, want4R)
	}
	if !reflect.DeepEqual(plan.LensPlan, want4R) {
		t.Errorf("frozen lens_plan = %v, want %v", plan.LensPlan, want4R)
	}
}

func TestReviewStart_DeclaredLensesWinButTierStaysLow(t *testing.T) {
	repoDir := gitRepoWithCommit(t)
	chdir(t, repoDir)
	subject := reviewSubjectFile(t, repoDir)

	// Declared lenses win the selection on a docs change, but the tier stays
	// low: the start is silent and needs no consent.
	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject, "--lenses", "readability"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	lineageID := startedLineageID(t, stdout)
	plan := frozenStartPlan(t, repoDir, lineageID)

	if plan.RiskTier != "low" {
		t.Errorf("frozen risk_tier = %q, want low (docs change)", plan.RiskTier)
	}
	if !reflect.DeepEqual(plan.SelectedLenses, []string{"readability"}) {
		t.Errorf("frozen lenses = %v, want declared [readability]", plan.SelectedLenses)
	}
}

func TestReviewStart_StatusSurfacesTierAndPlan(t *testing.T) {
	repoDir := gitRepoWithAuthCommit(t)
	chdir(t, repoDir)
	subject := reviewSubjectFile(t, repoDir)

	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject, "--consent", "granted"})
	if code != 0 {
		t.Fatalf("start: exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	lineageID := startedLineageID(t, stdout)

	oldArgs := os.Args
	os.Args = []string{"biggz", "review", "status", lineageID, "--json"}
	defer func() { os.Args = oldArgs }()
	outFile, err := os.CreateTemp(t.TempDir(), "status-*")
	if err != nil {
		t.Fatalf("create status capture: %v", err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outFile, outFile
	statusCode := reviewStatusRun()
	os.Stdout, os.Stderr = oldOut, oldErr
	outFile.Close()
	if statusCode != 0 {
		t.Fatalf("status exit code = %d, want 0", statusCode)
	}
	outData, _ := os.ReadFile(outFile.Name())

	var status map[string]json.RawMessage
	if err := json.Unmarshal(outData, &status); err != nil {
		t.Fatalf("status --json is not JSON: %v\n%s", err, outData)
	}
	if string(status["risk_tier"]) != `"high"` {
		t.Errorf("status risk_tier = %s, want high", status["risk_tier"])
	}
	var lensPlan []string
	if err := json.Unmarshal(status["lens_plan"], &lensPlan); err != nil {
		t.Fatalf("status lens_plan: %v", err)
	}
	if want := []string{"risk", "readability", "reliability", "resilience"}; !reflect.DeepEqual(lensPlan, want) {
		t.Errorf("status lens_plan = %v, want %v", lensPlan, want)
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

// ─── review refute (Debt D2) ─────────────────────────────────────────────────

// runReviewCommand invokes one review CLI runner in-process with the given
// args and optional stdin, capturing stdout and stderr through temp files.
func runReviewCommand(t *testing.T, fn func() int, args []string, stdin string) (code int, stdout, stderr string) {
	t.Helper()
	oldArgs, oldIn, oldOut, oldErr := os.Args, os.Stdin, os.Stdout, os.Stderr
	os.Args = args
	defer func() {
		os.Args, os.Stdin, os.Stdout, os.Stderr = oldArgs, oldIn, oldOut, oldErr
	}()
	if stdin != "" {
		inFile, err := os.CreateTemp(t.TempDir(), "stdin-*")
		if err != nil {
			t.Fatalf("create stdin capture: %v", err)
		}
		if _, err := inFile.WriteString(stdin); err != nil {
			t.Fatalf("write stdin: %v", err)
		}
		if _, err := inFile.Seek(0, 0); err != nil {
			t.Fatalf("rewind stdin: %v", err)
		}
		os.Stdin = inFile
		defer inFile.Close()
	}
	outFile, err := os.CreateTemp(t.TempDir(), "stdout-*")
	if err != nil {
		t.Fatalf("create stdout capture: %v", err)
	}
	errFile, err := os.CreateTemp(t.TempDir(), "stderr-*")
	if err != nil {
		t.Fatalf("create stderr capture: %v", err)
	}
	os.Stdout, os.Stderr = outFile, errFile
	code = fn()
	os.Stdout, os.Stderr = oldOut, oldErr
	outFile.Close()
	errFile.Close()
	outData, _ := os.ReadFile(outFile.Name())
	errData, _ := os.ReadFile(errFile.Name())
	return code, string(outData), string(errData)
}

func runReviewCapture(t *testing.T, args []string, stdin string) (code int, stdout, stderr string) {
	t.Helper()
	return runReviewCommand(t, reviewCaptureResultRun, args, stdin)
}

func runReviewRefute(t *testing.T, args []string, stdin string) (code int, stdout, stderr string) {
	t.Helper()
	return runReviewCommand(t, reviewRefuteRun, args, stdin)
}

func runReviewFinalize(t *testing.T, args []string) (code int, stdout, stderr string) {
	t.Helper()
	return runReviewCommand(t, reviewFinalizeRun, args, "")
}

func runReviewGate(t *testing.T, args []string) (code int, stdout, stderr string) {
	t.Helper()
	return runReviewCommand(t, reviewGateRun, args, "")
}

// statusJSON runs `biggz review status <lineage> --json` and returns the
// decoded JSON document.
func statusJSON(t *testing.T, lineageID string) map[string]json.RawMessage {
	t.Helper()
	code, stdout, stderr := runReviewCommand(t, reviewStatusRun,
		[]string{"biggz", "review", "status", lineageID, "--json"}, "")
	if code != 0 {
		t.Fatalf("status exit code = %d (stderr: %s)", code, stderr)
	}
	var status map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &status); err != nil {
		t.Fatalf("status --json is not JSON: %v\n%s", err, stdout)
	}
	return status
}

// refutationSmoke drives the CLI end to end for the given verdicts and returns
// the final gate exit code and stderr: start (high-risk auth change, granted),
// preflight + capture one risk lens with two inferential candidate-causal
// findings, finalize (must fail naming the pending ids), register the refuter
// batch via stdin, finalize again, and run the post-apply gate.
func refutationSmoke(t *testing.T, verdicts any) (gateCode int, gateReason string, gateFindings *struct {
	Blocking int `json:"blocking"`
	Resolved int `json:"resolved"`
	FollowUp int `json:"follow_up"`
}, lineageID string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	repoDir := gitRepoWithAuthCommit(t)
	chdir(t, repoDir)
	headSHA := runGitOutput(t, repoDir, "rev-parse", "HEAD")
	subject := filepath.Join(t.TempDir(), "subject.json")
	if err := os.WriteFile(subject, []byte(fmt.Sprintf(`{"repository":%q,"commit_sha":%q}`, filepath.ToSlash(repoDir), headSHA)), 0644); err != nil {
		t.Fatalf("write subject: %v", err)
	}

	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject, "--lenses", "risk", "--consent", "granted"})
	if code != 0 {
		t.Fatalf("start: exit code = %d (stderr: %s)", code, stderr)
	}
	lineageID = startedLineageID(t, stdout)
	status := statusJSON(t, lineageID)
	var headHash string
	if err := json.Unmarshal(status["head_hash"], &headHash); err != nil {
		t.Fatalf("status head_hash: %v", err)
	}

	// Preflight through the CLI, then capture with two inferential findings.
	code, stdout, stderr = runReviewCapture(t, []string{"biggz", "review", "capture-result",
		"--lineage", lineageID, "--target", headSHA, "--lens", "risk", "--order", "0",
		"--expected-revision", headHash, "--preflight"}, "")
	if code != 0 {
		t.Fatalf("preflight: exit code = %d (stderr: %s)", code, stderr)
	}
	var preflight review.PreflightResult
	if err := json.Unmarshal([]byte(stdout), &preflight); err != nil {
		t.Fatalf("preflight JSON: %v\n%s", err, stdout)
	}
	paths := review.ManifestPaths(preflight.ChangedPathManifest)
	findings := []any{
		map[string]any{"id": "R1-001", "lens": "risk", "location": paths[0] + ":2",
			"severity": "CRITICAL", "claim": "unbounded retry loop",
			"evidence_class": "inferential", "causal_disposition": "introduced"},
		map[string]any{"id": "R1-002", "lens": "risk", "location": paths[0] + ":3",
			"severity": "CRITICAL", "claim": "token reuse across sessions",
			"evidence_class": "inferential", "causal_disposition": "introduced"},
	}
	resultPayload, err := json.Marshal(map[string]any{
		"subject_hash": preflight.Subject.SubjectHash,
		"inspection":   map[string]any{"status": "completed", "paths": paths},
		"lens":         "risk", "findings": findings,
		"evidence": []any{"candidate inspection completed"},
	})
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	code, stdout, stderr = runReviewCapture(t, []string{"biggz", "review", "capture-result",
		"--lineage", lineageID, "--target", headSHA, "--lens", "risk", "--order", "0",
		"--expected-revision", headHash, "--input", "-"}, string(resultPayload))
	if code != 0 {
		t.Fatalf("capture: exit code = %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, `"admission_decision": "completed"`) {
		t.Errorf("capture artifact should be admitted, got: %s", stdout)
	}

	// Finalize must fail: the refuter batch is pending, naming ids + command.
	code, _, stderr = runReviewFinalize(t, []string{"biggz", "review", "finalize", lineageID})
	if code != 1 {
		t.Fatalf("finalize with pending refutations: exit code = %d, want 1 (stderr: %s)", code, stderr)
	}
	for _, want := range []string{"R1-001", "R1-002", "biggz review refute " + lineageID + " --input -"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("finalize stderr must name %q, got: %s", want, stderr)
		}
	}

	// Status surfaces the pending refutation set.
	status = statusJSON(t, lineageID)
	var refutations struct {
		Total   int `json:"total"`
		Refuted int `json:"refuted"`
		Stands  int `json:"stands"`
		Pending int `json:"pending"`
	}
	if err := json.Unmarshal(status["refutations"], &refutations); err != nil {
		t.Fatalf("status refutations: %v", err)
	}
	if refutations != (struct {
		Total   int `json:"total"`
		Refuted int `json:"refuted"`
		Stands  int `json:"stands"`
		Pending int `json:"pending"`
	}{Total: 2, Refuted: 0, Stands: 0, Pending: 2}) {
		t.Errorf("status refutations = %+v, want total 2 pending 2", refutations)
	}

	// Register the one refuter batch through the CLI.
	refuteInput, err := json.Marshal(map[string]any{
		"schema": "biggz-ai.review-refutation/v1", "lineage": lineageID,
		"verdicts": verdicts,
	})
	if err != nil {
		t.Fatalf("marshal refute input: %v", err)
	}
	code, stdout, stderr = runReviewRefute(t, []string{"biggz", "review", "refute", lineageID, "--input", "-"}, string(refuteInput))
	if code != 0 {
		t.Fatalf("refute: exit code = %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, `"idempotent": false`) {
		t.Errorf("refute stdout should report a fresh batch, got: %s", stdout)
	}

	// Status now shows the verdict split.
	status = statusJSON(t, lineageID)
	if err := json.Unmarshal(status["refutations"], &refutations); err != nil {
		t.Fatalf("status refutations: %v", err)
	}
	if refutations.Refuted+refutations.Stands != 2 || refutations.Pending != 0 {
		t.Errorf("status refutations = %+v, want refuted+stands=2 pending=0", refutations)
	}

	// Finalize succeeds with the batch registered.
	code, stdout, stderr = runReviewFinalize(t, []string{"biggz", "review", "finalize", lineageID})
	if code != 0 {
		t.Fatalf("finalize after refutation: exit code = %d (stderr: %s)", code, stderr)
	}

	// Gate: the verdicts decide the outcome.
	gateCode, gateStdout, _ := runReviewGate(t, []string{"biggz", "review", "gate", "post-apply", lineageID, "--json"})
	var gateResult struct {
		Passed   bool   `json:"passed"`
		Allowed  bool   `json:"allowed"`
		Reason   string `json:"reason"`
		Findings *struct {
			Blocking int `json:"blocking"`
			Resolved int `json:"resolved"`
			FollowUp int `json:"follow_up"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(gateStdout), &gateResult); err != nil {
		t.Fatalf("gate --json is not JSON: %v\n%s", err, gateStdout)
	}
	return gateCode, gateResult.Reason, gateResult.Findings, lineageID
}
func TestReviewRefute_StandsFindingBlocksGate(t *testing.T) {
	gateCode, gateReason, gateFindings, _ := refutationSmoke(t, []any{
		map[string]any{"finding_id": "R1-001", "verdict": "refuted", "evidence": "counterexample at auth/token.go:2"},
		map[string]any{"finding_id": "R1-002", "verdict": "stands", "evidence": "token reuse reachable from the diff"},
	})
	if gateCode != 1 {
		t.Fatalf("gate exit code = %d, want 1 with a standing finding (reason: %s)", gateCode, gateReason)
	}
	if !strings.Contains(gateReason, "R1-002") || strings.Contains(gateReason, "R1-001") {
		t.Errorf("gate must block on the standing finding only, got: %s", gateReason)
	}
	if gateFindings == nil || gateFindings.Blocking != 1 || gateFindings.Resolved != 1 || gateFindings.FollowUp != 0 {
		t.Errorf("gate findings = %+v, want blocking=1 resolved=1 follow_up=0", gateFindings)
	}
}

func TestReviewRefute_AllRefutedGatePasses(t *testing.T) {
	gateCode, gateReason, gateFindings, _ := refutationSmoke(t, []any{
		map[string]any{"finding_id": "R1-001", "verdict": "refuted", "evidence": "counterexample at auth/token.go:2"},
		map[string]any{"finding_id": "R1-002", "verdict": "refuted", "evidence": "session tokens are scoped per request"},
	})
	if gateCode != 0 {
		t.Fatalf("gate exit code = %d, want 0 with every finding refuted (reason: %s)", gateCode, gateReason)
	}
	if !strings.Contains(gateReason, "gate passed") {
		t.Errorf("gate reason should report the pass, got: %s", gateReason)
	}
	if gateFindings == nil || gateFindings.Blocking != 0 || gateFindings.Resolved != 2 || gateFindings.FollowUp != 0 {
		t.Errorf("gate findings = %+v, want blocking=0 resolved=2 follow_up=0", gateFindings)
	}
}
