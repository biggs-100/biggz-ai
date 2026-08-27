package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/review"
)

// ─── negotiated review-integration contract (Phase D2) ───────────────────────

// runReviewStatus invokes reviewStatusRun in-process with the given args.
func runReviewStatus(t *testing.T, args []string) (code int, stdout, stderr string) {
	t.Helper()
	return runReviewCommand(t, reviewStatusRun, args, "")
}

// contractStatus runs the negotiated status query and decodes the envelope.
func contractStatus(t *testing.T, lineageID string) map[string]json.RawMessage {
	t.Helper()
	code, stdout, stderr := runReviewStatus(t, []string{"biggz", "review", "status", lineageID,
		"--contract", review.ContractSchema, "--next-transition"})
	if code != 0 {
		t.Fatalf("contract status exit code = %d (stderr: %s)", code, stderr)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("contract status stdout is not JSON: %v\n%s", err, stdout)
	}
	return envelope
}

// contractTransition decodes the next_transition of a contract envelope.
func contractTransition(t *testing.T, envelope map[string]json.RawMessage) (nt struct {
	Type       string                     `json:"type"`
	Operation  string                     `json:"operation"`
	Arguments  []string                   `json:"arguments"`
	ReasonCode string                     `json:"reason_code"`
	Inputs     map[string]json.RawMessage `json:"inputs"`
}) {
	t.Helper()
	if err := json.Unmarshal(envelope["next_transition"], &nt); err != nil {
		t.Fatalf("next_transition: %v", err)
	}
	return nt
}

func TestReviewStatus_ContractEnvelopeOnly(t *testing.T) {
	repoDir := gitRepoWithAuthCommit(t)
	chdir(t, repoDir)
	subject := reviewSubjectFile(t, repoDir)

	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject, "--lenses", "risk", "--consent", "granted"})
	if code != 0 {
		t.Fatalf("start: exit code = %d (stderr: %s)", code, stderr)
	}
	lineageID := startedLineageID(t, stdout)

	envelope := contractStatus(t, lineageID)
	// The contract mode prints ONLY the envelope: no raw status fields.
	for _, key := range []string{"schema", "lineage", "next_transition"} {
		if _, ok := envelope[key]; !ok {
			t.Errorf("envelope missing key %q", key)
		}
	}
	for _, key := range []string{"lineage_id", "head_hash", "event_count", "chain_valid", "receipt", "integrity_verdict", "budget_counters"} {
		if _, ok := envelope[key]; ok {
			t.Errorf("envelope must not carry raw status field %q", key)
		}
	}
	if string(envelope["schema"]) != `"biggz-ai.review-integration/v1"` {
		t.Errorf("schema = %s", envelope["schema"])
	}
	if string(envelope["lineage"]) != `"`+lineageID+`"` {
		t.Errorf("lineage = %s", envelope["lineage"])
	}
	nt := contractTransition(t, envelope)
	if nt.Type != "collect" {
		t.Fatalf("type = %q, want collect", nt.Type)
	}
}

func TestReviewStatus_UnknownContractErrors(t *testing.T) {
	repoDir := gitRepoWithCommit(t)
	chdir(t, repoDir)

	code, _, stderr := runReviewStatus(t, []string{"biggz", "review", "status", "some-lineage",
		"--contract", "bogus-contract/v1", "--next-transition"})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "unknown review integration contract") || !strings.Contains(stderr, review.ContractSchema) {
		t.Errorf("stderr should name the unknown contract and the supported one, got: %s", stderr)
	}
}

func TestReviewStatus_ContractWithoutNextTransitionErrors(t *testing.T) {
	repoDir := gitRepoWithCommit(t)
	chdir(t, repoDir)

	code, _, stderr := runReviewStatus(t, []string{"biggz", "review", "status", "some-lineage",
		"--contract", review.ContractSchema})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "--next-transition") {
		t.Errorf("stderr should demand --next-transition, got: %s", stderr)
	}
}

func TestReviewStatus_NextTransitionWithoutContractErrors(t *testing.T) {
	repoDir := gitRepoWithCommit(t)
	chdir(t, repoDir)

	code, _, stderr := runReviewStatus(t, []string{"biggz", "review", "status", "some-lineage", "--next-transition"})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "requires --contract") || !strings.Contains(stderr, review.ContractSchema) {
		t.Errorf("stderr should name the required contract, got: %s", stderr)
	}
}

func TestReviewStart_ContractRelayEnvelopeWithInvocations(t *testing.T) {
	repoDir := gitRepoWithAuthCommit(t)
	chdir(t, repoDir)
	subject := reviewSubjectFile(t, repoDir)

	// Contract mode on a high-risk candidate: relay envelope with follow-up
	// invocations, exit 0, no lineage created — never the headless hard error.
	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject, "--lenses", "risk",
		"--contract", review.ContractSchema})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if stderr != "" {
		t.Errorf("relay should print the envelope on stdout only, got stderr: %s", stderr)
	}

	var envelope struct {
		Schema    string `json:"schema"`
		Candidate struct {
			Lineage string   `json:"lineage"`
			Risk    string   `json:"risk"`
			Lenses  []string `json:"lenses"`
		} `json:"candidate"`
		Choices []struct {
			ID         string `json:"id"`
			Invocation string `json:"invocation"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("relay stdout is not JSON: %v\n%s", err, stdout)
	}
	if envelope.Schema != "biggz-ai.review-consent/v1" {
		t.Errorf("schema = %q, want the existing consent schema", envelope.Schema)
	}
	if envelope.Candidate.Risk != "high" {
		t.Errorf("candidate risk = %q, want high", envelope.Candidate.Risk)
	}
	if len(envelope.Choices) != 2 {
		t.Fatalf("choices = %d, want 2", len(envelope.Choices))
	}
	invocations := map[string]string{}
	for _, choice := range envelope.Choices {
		invocations[choice.ID] = choice.Invocation
	}
	for _, answer := range []string{"granted", "declined"} {
		invocation := invocations[answer]
		if !strings.HasPrefix(invocation, "biggz review start ") {
			t.Errorf("%s invocation = %q, want a biggz review start invocation", answer, invocation)
		}
		// The original flags are echoed and the frozen candidate lineage pinned.
		if !strings.Contains(invocation, "--subject "+followUpShellWord(subject)) {
			t.Errorf("%s invocation should echo --subject, got: %q", answer, invocation)
		}
		if !strings.Contains(invocation, "--lineage "+envelope.Candidate.Lineage) {
			t.Errorf("%s invocation should pin the frozen candidate lineage %q, got: %q",
				answer, envelope.Candidate.Lineage, invocation)
		}
		if !strings.Contains(invocation, "--lenses risk") {
			t.Errorf("%s invocation should echo --lenses risk, got: %q", answer, invocation)
		}
		if !strings.HasSuffix(invocation, "--consent "+answer) {
			t.Errorf("%s invocation should end with --consent %s, got: %q", answer, answer, invocation)
		}
	}

	if _, err := os.Stat(lineageStoreDir(repoDir)); !os.IsNotExist(err) {
		t.Error("contract relay must not create the review store")
	}
}

func TestReviewStart_ContractUndeclaredRelaysInsteadOfError(t *testing.T) {
	repoDir := gitRepoWithAuthCommit(t)
	chdir(t, repoDir)
	subject := reviewSubjectFile(t, repoDir)

	// Headless + undeclared + contract: the relay envelope replaces the hard
	// error of the non-contract headless path.
	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject, "--contract", review.ContractSchema})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("relay stdout is not JSON: %v\n%s", err, stdout)
	}
	if string(envelope["schema"]) != `"biggz-ai.review-consent/v1"` {
		t.Errorf("schema = %s", envelope["schema"])
	}
	if _, err := os.Stat(lineageStoreDir(repoDir)); !os.IsNotExist(err) {
		t.Error("contract relay must not create the review store")
	}
}

func TestReviewStart_ContractGrantedBehaviorUnchanged(t *testing.T) {
	repoDir := gitRepoWithAuthCommit(t)
	chdir(t, repoDir)
	subject := reviewSubjectFile(t, repoDir)

	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject, "--consent", "granted",
		"--contract", review.ContractSchema})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "Review started:") {
		t.Errorf("granted must keep its existing behavior, got: %q", stdout)
	}
}

func TestReviewStart_UnknownContractErrors(t *testing.T) {
	repoDir := gitRepoWithAuthCommit(t)
	chdir(t, repoDir)
	subject := reviewSubjectFile(t, repoDir)

	code, _, stderr := runReviewStart(t, []string{"--subject", subject, "--contract", "bogus/v1"})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "unknown review integration contract") || !strings.Contains(stderr, review.ContractSchema) {
		t.Errorf("stderr should name the supported contract, got: %s", stderr)
	}
}

// splitInvocation tokenizes a follow-up invocation string into argv, honoring
// the double-quoted values followUpShellWord emits.
func splitInvocation(t *testing.T, invocation string) []string {
	t.Helper()
	var fields []string
	var current strings.Builder
	inQuote := false
	for _, char := range invocation {
		switch {
		case char == '"':
			inQuote = !inQuote
		case (char == ' ' || char == '\t') && !inQuote:
			if current.Len() > 0 {
				fields = append(fields, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(char)
		}
	}
	if current.Len() > 0 {
		fields = append(fields, current.String())
	}
	return fields
}

// captureCollectInput runs `capture-result --preflight` and the real capture
// for one collect input with a clean reviewer payload, through the CLI.
func captureCollectInput(t *testing.T, repoDir, headSHA string, input struct {
	Lineage           string                     `json:"lineage"`
	Target            string                     `json:"target"`
	Lens              string                     `json:"lens"`
	Order             int                        `json:"order"`
	ExpectedRevision  string                     `json:"expected_revision"`
	RepositoryContext map[string]json.RawMessage `json:"repository_context"`
}) {
	t.Helper()
	code, stdout, stderr := runReviewCapture(t, []string{"biggz", "review", "capture-result",
		"--lineage", input.Lineage, "--target", input.Target, "--lens", input.Lens,
		"--order", strconv.Itoa(input.Order), "--expected-revision", input.ExpectedRevision, "--preflight"}, "")
	if code != 0 {
		t.Fatalf("preflight: exit code = %d (stderr: %s)", code, stderr)
	}
	var preflight review.PreflightResult
	if err := json.Unmarshal([]byte(stdout), &preflight); err != nil {
		t.Fatalf("preflight JSON: %v\n%s", err, stdout)
	}
	paths := review.ManifestPaths(preflight.ChangedPathManifest)
	resultPayload, err := json.Marshal(map[string]any{
		"subject_hash": preflight.Subject.SubjectHash,
		"inspection":   map[string]any{"status": "completed", "paths": paths},
		"lens":         input.Lens,
		"findings":     []any{},
		"evidence":     []any{"clean sweep"},
	})
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	code, stdout, stderr = runReviewCapture(t, []string{"biggz", "review", "capture-result",
		"--lineage", input.Lineage, "--target", input.Target, "--lens", input.Lens,
		"--order", strconv.Itoa(input.Order), "--expected-revision", input.ExpectedRevision, "--input", "-"},
		string(resultPayload))
	if code != 0 {
		t.Fatalf("capture: exit code = %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, `"admission_decision": "completed"`) {
		t.Errorf("capture should be admitted, got: %s", stdout)
	}
}

// TestReviewContractLoopSmoke drives the full negotiated loop end to end:
// start (consent relay -> granted) → status collect → capture → status
// finalize execute → finalize → status stop ready_for_gates.
func TestReviewContractLoopSmoke(t *testing.T) {
	repoDir := gitRepoWithAuthCommit(t)
	chdir(t, repoDir)
	headSHA := runGitOutput(t, repoDir, "rev-parse", "HEAD")
	subject := filepath.Join(t.TempDir(), "subject.json")
	if err := os.WriteFile(subject, []byte(`{"repository":`+strconv.Quote(filepath.ToSlash(repoDir))+`,"commit_sha":`+strconv.Quote(headSHA)+`}`), 0644); err != nil {
		t.Fatalf("write subject: %v", err)
	}

	// 1. START with the contract: the consent envelope relays, exit 0, no
	//    lineage yet.
	code, stdout, stderr := runReviewStart(t, []string{"--subject", subject, "--lenses", "risk",
		"--contract", review.ContractSchema})
	if code != 0 {
		t.Fatalf("start: exit code = %d (stderr: %s)", code, stderr)
	}
	var consent struct {
		Candidate struct {
			Lineage string `json:"lineage"`
		} `json:"candidate"`
		Choices []struct {
			ID         string `json:"id"`
			Invocation string `json:"invocation"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(stdout), &consent); err != nil {
		t.Fatalf("consent envelope: %v\n%s", err, stdout)
	}
	if _, err := os.Stat(lineageStoreDir(repoDir)); !os.IsNotExist(err) {
		t.Fatal("consent relay must not create the lineage")
	}
	lineageID := consent.Candidate.Lineage
	var grantedInvocation string
	for _, choice := range consent.Choices {
		if choice.ID == "granted" {
			grantedInvocation = choice.Invocation
		}
	}
	if grantedInvocation == "" {
		t.Fatal("consent envelope lacks the granted invocation")
	}

	// 2. Run EXACTLY the named follow-up invocation for the human's answer.
	argv := splitInvocation(t, grantedInvocation)
	if argv[0] != "biggz" || argv[1] != "review" || argv[2] != "start" {
		t.Fatalf("invocation = %v, want a biggz review start command", argv)
	}
	code, stdout, stderr = runReviewStart(t, argv[3:])
	if code != 0 {
		t.Fatalf("granted follow-up: exit code = %d (stderr: %s)", code, stderr)
	}
	if got := startedLineageID(t, stdout); got != lineageID {
		t.Fatalf("granted rerun lineage = %q, want the frozen candidate %q", got, lineageID)
	}

	// 3. STATUS: the envelope routes to collect with the exact capture input.
	envelope := contractStatus(t, lineageID)
	nt := contractTransition(t, envelope)
	if nt.Type != "collect" {
		t.Fatalf("transition = %+v, want collect", nt)
	}
	var captureInput struct {
		Lineage          string `json:"lineage"`
		Target           string `json:"target"`
		Lens             string `json:"lens"`
		Order            int    `json:"order"`
		ExpectedRevision string `json:"expected_revision"`
	}
	if err := json.Unmarshal(nt.Inputs["capture"], &captureInput); err != nil {
		t.Fatalf("capture input: %v", err)
	}
	if captureInput.Lineage != lineageID || captureInput.Target != headSHA || captureInput.Lens != "risk" || captureInput.Order != 0 {
		t.Errorf("capture input = %+v, want the exact bound slot (target %s, lens risk, order 0)", captureInput, headSHA)
	}

	// 4. Satisfy the collect input: preflight derives subject_hash, capture
	//    persists, STATUS re-queried.
	captureCollectInput(t, repoDir, headSHA, struct {
		Lineage           string                     `json:"lineage"`
		Target            string                     `json:"target"`
		Lens              string                     `json:"lens"`
		Order             int                        `json:"order"`
		ExpectedRevision  string                     `json:"expected_revision"`
		RepositoryContext map[string]json.RawMessage `json:"repository_context"`
	}{Lineage: captureInput.Lineage, Target: captureInput.Target, Lens: captureInput.Lens,
		Order: captureInput.Order, ExpectedRevision: captureInput.ExpectedRevision})

	envelope = contractStatus(t, lineageID)
	nt = contractTransition(t, envelope)
	if nt.Type != "execute" || nt.Operation != "finalize" {
		t.Fatalf("transition = %+v, want execute finalize", nt)
	}
	if len(nt.Arguments) != 1 || nt.Arguments[0] != lineageID {
		t.Errorf("arguments = %v, want [%s]", nt.Arguments, lineageID)
	}

	// 5. Execute the named operation.
	code, _, stderr = runReviewFinalize(t, []string{"biggz", "review", "finalize", lineageID})
	if code != 0 {
		t.Fatalf("finalize: exit code = %d (stderr: %s)", code, stderr)
	}

	// 6. STATUS: stop ready_for_gates — the receipt exists, gates run when the
	//    lifecycle demands.
	envelope = contractStatus(t, lineageID)
	nt = contractTransition(t, envelope)
	if nt.Type != "stop" || nt.ReasonCode != "ready_for_gates" {
		t.Fatalf("transition = %+v, want stop ready_for_gates", nt)
	}
}
