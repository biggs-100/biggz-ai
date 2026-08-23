package review

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// severeFinding builds one reviewer finding payload for a capture.
func severeFinding(id, lens, evidenceClass, disposition, severity string) map[string]any {
	return map[string]any{
		"id": id, "lens": lens, "severity": severity, "claim": "candidate-causal defect",
		"evidence_class": evidenceClass, "causal_disposition": disposition,
	}
}

// refuteVerdict builds one refutation input verdict payload.
func refuteVerdict(id, verdict, evidence string) map[string]any {
	return map[string]any{"finding_id": id, "verdict": verdict, "evidence": evidence}
}

// captureLensFindings captures a lens result carrying the given findings.
func captureLensFindings(t *testing.T, repo, lineageID, commitSHA, lens string, order int, findings []map[string]any) {
	t.Helper()
	store, err := Open(repo, lineageID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	binding := CaptureBinding{
		Repo: repo, LineageID: lineageID, TargetIdentity: commitSHA,
		Lens: lens, Order: order, ExpectedRevision: chain.HeadHash,
	}
	preflight, err := Preflight(binding)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	paths := ManifestPaths(preflight.ChangedPathManifest)
	envelope := map[string]any{
		"subject_hash": preflight.Subject.SubjectHash,
		"inspection":   map[string]any{"status": "completed", "paths": paths},
		"lens":         lens,
		"findings":     findings,
		"evidence":     []any{"candidate inspection completed"},
	}
	for index := range findings {
		if findings[index]["location"] == nil {
			findings[index]["location"] = paths[0] + ":2"
		}
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	outcome, err := Capture(binding, payload)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if outcome.Artifact.AdmissionDecision != AdmissionCompleted {
		t.Fatalf("decision = %q, want completed", outcome.Artifact.AdmissionDecision)
	}
}

// refutePayload renders the strict refutation input JSON envelope.
func refutePayload(t *testing.T, lineageID string, verdicts []map[string]any) []byte {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"schema": RefutationSchema, "lineage": lineageID, "verdicts": verdicts,
	})
	if err != nil {
		t.Fatalf("marshal refutation payload: %v", err)
	}
	return payload
}

// refuteVerdicts registers a refutation batch for the given verdicts.
func refuteVerdicts(t *testing.T, repo, lineageID string, verdicts ...map[string]any) RefuteOutcome {
	t.Helper()
	outcome, err := Refute(repo, lineageID, refutePayload(t, lineageID, verdicts))
	if err != nil {
		t.Fatalf("Refute: %v", err)
	}
	return outcome
}

// refuteFixture starts a one-lens lineage and captures a risk lens with the
// given findings. It returns the repo, the reviewed head, and the lineage id.
func refuteFixture(t *testing.T, findings []map[string]any) (string, string, string) {
	t.Helper()
	repo, _, head := finalizeFixtureRepo(t)
	lineageID := "refute-lineage"
	finalizeStart(t, repo, head, lineageID, []string{"risk"}, "")
	captureLensFindings(t, repo, lineageID, head, "risk", 0, findings)
	return repo, head, lineageID
}

// persistedReceipt loads and validates the receipt referenced by the lineage's
// complete_review event.
func persistedReceipt(t *testing.T, repo, lineageID string) PersistedReceipt {
	t.Helper()
	store, err := Open(repo, lineageID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	ref := receiptArtifactOf(chain)
	if ref == nil {
		t.Fatal("no receipt artifact in chain")
	}
	payload, err := os.ReadFile(filepath.Join(store.Dir, ref.Path))
	if err != nil {
		t.Fatalf("read receipt: %v", err)
	}
	var receipt PersistedReceipt
	if err := json.Unmarshal(payload, &receipt); err != nil {
		t.Fatalf("parse receipt: %v", err)
	}
	if err := receipt.Validate(); err != nil {
		t.Fatalf("validate receipt: %v", err)
	}
	return receipt
}

// twoInferentialFindings is the canonical fixture: two CRITICAL inferential
// candidate-causal findings in one risk capture.
func twoInferentialFindings() []map[string]any {
	return []map[string]any{
		severeFinding("R1-001", "risk", "inferential", "introduced", "CRITICAL"),
		severeFinding("R1-002", "risk", "inferential", "introduced", "CRITICAL"),
	}
}

// ---------------------------------------------------------------------------
// Refute happy path: one refuted + one stands
// ---------------------------------------------------------------------------

func TestRefute_HappyPathRefutedAndStands(t *testing.T) {
	origBurn := BurnEnabled
	BurnEnabled = false
	defer func() { BurnEnabled = origBurn }()
	repo, head, lineageID := refuteFixture(t, twoInferentialFindings())

	// Finalize must refuse until the refuter batch covers every inferential
	// candidate-causal finding, naming the pending ids and the command.
	_, err := Finalize(repo, lineageID)
	if err == nil {
		t.Fatal("expected finalize rejection with pending refutations")
	}
	for _, want := range []string{"R1-001", "R1-002", "biggz review refute " + lineageID + " --input -"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("finalize error must name %q, got: %v", want, err)
		}
	}
	store, err := Open(repo, lineageID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if chain, _ := store.LoadChain(); chain.Count != 3 {
		t.Errorf("event count = %d, want 3 (rejection must not append)", chain.Count)
	}
	if _, err := os.Stat(filepath.Join(store.Dir, ReceiptsDirName)); !os.IsNotExist(err) {
		t.Error("finalize rejection must not create the receipts directory")
	}

	// One batch with both verdicts: R1-001 refuted, R1-002 stands.
	outcome := refuteVerdicts(t, repo, lineageID,
		refuteVerdict("R1-001", "refuted", "locked counterexample at a.txt:2"),
		refuteVerdict("R1-002", "stands", "sink is reachable from the diff"))
	if outcome.Revision == "" {
		t.Fatal("refute outcome missing revision")
	}
	if !reflect.DeepEqual(outcome.Refuted, []string{"R1-001"}) || !reflect.DeepEqual(outcome.Stands, []string{"R1-002"}) {
		t.Errorf("outcome = %+v, want refuted [R1-001] stands [R1-002]", outcome)
	}

	// Finalize now succeeds and the receipt routes the verdicts.
	if _, err := Finalize(repo, lineageID); err != nil {
		t.Fatalf("Finalize after refutation: %v", err)
	}
	receipt := persistedReceipt(t, repo, lineageID)
	if !reflect.DeepEqual(receipt.ResolvedFindingIDs, []string{"R1-001"}) {
		t.Errorf("resolved finding IDs = %v, want [R1-001]", receipt.ResolvedFindingIDs)
	}
	if !reflect.DeepEqual(receipt.StandingFindingIDs, []string{"R1-002"}) {
		t.Errorf("standing finding IDs = %v, want [R1-002]", receipt.StandingFindingIDs)
	}

	// Gate: the refuted finding must not block; the standing one must.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	result, err := EvaluateGate(GatePostApply, repo, lineageID, GateOptions{})
	if err != nil {
		t.Fatalf("EvaluateGate: %v", err)
	}
	if result.Allowed || result.Passed {
		t.Fatal("expected denial with a standing finding")
	}
	if result.Findings == nil || result.Findings.Blocking != 1 || result.Findings.Resolved != 1 || result.Findings.FollowUp != 0 {
		t.Errorf("findings = %+v, want blocking=1 resolved=1 follow_up=0", result.Findings)
	}
	if !strings.Contains(result.Reason, "R1-002") {
		t.Errorf("reason must name the standing finding, got: %v", result.Reason)
	}
	if strings.Contains(result.Reason, "R1-001") {
		t.Errorf("reason must not block on the refuted finding, got: %v", result.Reason)
	}
	if !strings.Contains(result.Reason, "stands") {
		t.Errorf("reason should explain the stands verdict, got: %v", result.Reason)
	}
	_ = head
}

func TestRefute_AllRefutedGatePasses(t *testing.T) {
	origBurn := BurnEnabled
	BurnEnabled = false
	defer func() { BurnEnabled = origBurn }()
	repo, _, lineageID := refuteFixture(t, twoInferentialFindings())
	refuteVerdicts(t, repo, lineageID,
		refuteVerdict("R1-001", "refuted", "counterexample reproduced at a.txt:2"),
		refuteVerdict("R1-002", "refuted", "race is unreachable: mutex held across the call"))
	if _, err := Finalize(repo, lineageID); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	receipt := persistedReceipt(t, repo, lineageID)
	if !reflect.DeepEqual(receipt.ResolvedFindingIDs, []string{"R1-001", "R1-002"}) {
		t.Errorf("resolved finding IDs = %v, want both refuted", receipt.ResolvedFindingIDs)
	}
	if len(receipt.StandingFindingIDs) != 0 {
		t.Errorf("standing finding IDs = %v, want none", receipt.StandingFindingIDs)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	result, err := EvaluateGate(GatePostApply, repo, lineageID, GateOptions{})
	if err != nil {
		t.Fatalf("EvaluateGate: %v", err)
	}
	if !result.Passed || !result.Allowed {
		t.Fatalf("expected pass with all findings refuted, got reasons=%v", result.Reasons)
	}
	if result.Findings == nil || result.Findings.Blocking != 0 || result.Findings.Resolved != 2 {
		t.Errorf("findings = %+v, want blocking=0 resolved=2", result.Findings)
	}
}

// ---------------------------------------------------------------------------
// Refute validation
// ---------------------------------------------------------------------------

func TestRefute_MissingVerdictRejectsNamingIds(t *testing.T) {
	repo, _, lineageID := refuteFixture(t, twoInferentialFindings())
	_, err := Refute(repo, lineageID, refutePayload(t, lineageID, []map[string]any{
		refuteVerdict("R1-001", "refuted", "counterexample at a.txt:2"),
	}))
	if err == nil {
		t.Fatal("expected rejection for an incomplete batch")
	}
	if !strings.Contains(err.Error(), "R1-002") {
		t.Errorf("error must name the missing finding id, got: %v", err)
	}
}

func TestRefute_UnknownFindingIDRejects(t *testing.T) {
	repo, _, lineageID := refuteFixture(t, twoInferentialFindings())
	_, err := Refute(repo, lineageID, refutePayload(t, lineageID, []map[string]any{
		refuteVerdict("R1-001", "refuted", "counterexample at a.txt:2"),
		refuteVerdict("R9-999", "refuted", "counterexample elsewhere"),
	}))
	if err == nil {
		t.Fatal("expected rejection for an unknown finding id")
	}
	if !strings.Contains(err.Error(), "R9-999") {
		t.Errorf("error must name the offending id, got: %v", err)
	}
}

func TestRefute_DeterministicFindingNotRefutable(t *testing.T) {
	repo, _, lineageID := refuteFixture(t, []map[string]any{
		severeFinding("R1-001", "risk", "deterministic", "introduced", "CRITICAL"),
	})
	_, err := Refute(repo, lineageID, refutePayload(t, lineageID, []map[string]any{
		refuteVerdict("R1-001", "refuted", "counterexample at a.txt:2"),
	}))
	if err == nil {
		t.Fatal("expected rejection for refuting a deterministic finding")
	}
	if !strings.Contains(err.Error(), "deterministic") || !strings.Contains(err.Error(), "R1-001") {
		t.Errorf("error must explain the deterministic auto-block, got: %v", err)
	}
	// The lineage must not have gained a refutation event.
	chain, err := Open(repo, lineageID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if loaded, _ := chain.LoadChain(); loaded.Count != 3 {
		t.Errorf("event count = %d, want 3 (start + capture only; rejection must not append)", loaded.Count)
	}
}

func TestRefute_NonCandidateCausalRejects(t *testing.T) {
	repo, _, lineageID := refuteFixture(t, []map[string]any{
		severeFinding("R1-001", "risk", "inferential", "pre-existing", "CRITICAL"),
	})
	_, err := Refute(repo, lineageID, refutePayload(t, lineageID, []map[string]any{
		refuteVerdict("R1-001", "refuted", "counterexample at a.txt:2"),
	}))
	if err == nil {
		t.Fatal("expected rejection for a non-candidate-causal finding")
	}
	if !strings.Contains(err.Error(), "R1-001") || !strings.Contains(err.Error(), "not candidate-causal") {
		t.Errorf("error must reject the non-candidate-causal id, got: %v", err)
	}
}

func TestRefute_NonSevereRejects(t *testing.T) {
	repo, _, lineageID := refuteFixture(t, []map[string]any{
		severeFinding("R1-001", "risk", "inferential", "introduced", "WARNING"),
	})
	_, err := Refute(repo, lineageID, refutePayload(t, lineageID, []map[string]any{
		refuteVerdict("R1-001", "refuted", "counterexample at a.txt:2"),
	}))
	if err == nil {
		t.Fatal("expected rejection for a non-severe finding")
	}
	if !strings.Contains(err.Error(), "not a severe finding") {
		t.Errorf("error must reject the non-severe id, got: %v", err)
	}
}

func TestRefute_DuplicateIDsReject(t *testing.T) {
	repo, _, lineageID := refuteFixture(t, twoInferentialFindings())
	_, err := Refute(repo, lineageID, refutePayload(t, lineageID, []map[string]any{
		refuteVerdict("R1-001", "refuted", "counterexample at a.txt:2"),
		refuteVerdict("R1-001", "stands", "duplicate entry"),
		refuteVerdict("R1-002", "refuted", "race is unreachable"),
	}))
	if err == nil {
		t.Fatal("expected rejection for duplicate verdict ids")
	}
	if !strings.Contains(err.Error(), "duplicate") || !strings.Contains(err.Error(), "R1-001") {
		t.Errorf("error must name the duplicate id, got: %v", err)
	}
}

func TestRefute_StrictInputValidation(t *testing.T) {
	repo, _, lineageID := refuteFixture(t, twoInferentialFindings())
	base := refuteVerdict("R1-001", "refuted", "counterexample at a.txt:2")
	full := []map[string]any{base, refuteVerdict("R1-002", "stands", "sink reachable")}

	cases := []struct {
		name    string
		payload map[string]any
		want    string
	}{
		{"unknown field", map[string]any{"schema": RefutationSchema, "lineage": lineageID, "verdicts": full, "extra": 1}, "unknown field"},
		{"wrong schema", map[string]any{"schema": "biggz-ai.other/v1", "lineage": lineageID, "verdicts": full}, "schema"},
		{"lineage mismatch", map[string]any{"schema": RefutationSchema, "lineage": "other-lineage", "verdicts": full}, "lineage"},
		{"empty verdicts", map[string]any{"schema": RefutationSchema, "lineage": lineageID, "verdicts": []any{}}, "missing verdicts"},
		{"invalid verdict value", map[string]any{"schema": RefutationSchema, "lineage": lineageID,
			"verdicts": []any{map[string]any{"finding_id": "R1-001", "verdict": "maybe", "evidence": "proof"}, refuteVerdict("R1-002", "refuted", "x")}}, "unsupported verdict"},
		{"empty evidence", map[string]any{"schema": RefutationSchema, "lineage": lineageID,
			"verdicts": []any{map[string]any{"finding_id": "R1-001", "verdict": "refuted", "evidence": ""}, refuteVerdict("R1-002", "refuted", "x")}}, "evidence"},
		{"placeholder evidence", map[string]any{"schema": RefutationSchema, "lineage": lineageID,
			"verdicts": []any{map[string]any{"finding_id": "R1-001", "verdict": "refuted", "evidence": "n/a"}, refuteVerdict("R1-002", "refuted", "x")}}, "evidence"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			_, err = Refute(repo, lineageID, payload)
			if err == nil {
				t.Fatal("expected rejection")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want mention of %q", err, tc.want)
			}
		})
	}
}

func TestRefute_NothingToRefute(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	lineageID := "refute-nothing"
	finalizeStart(t, repo, head, lineageID, []string{"risk"}, "")
	captureLensClean(t, repo, lineageID, head, "risk", 0)
	_, err := Refute(repo, lineageID, refutePayload(t, lineageID, nil))
	if err == nil {
		t.Fatal("expected rejection when nothing requires refutation")
	}
	if !strings.Contains(err.Error(), "no inferential candidate-causal findings") {
		t.Errorf("error = %v, want nothing-to-refute message", err)
	}
}

// ---------------------------------------------------------------------------
// Exactly one batch
// ---------------------------------------------------------------------------

func TestRefute_SecondBatchRejectedIdempotentRerunAccepted(t *testing.T) {
	repo, _, lineageID := refuteFixture(t, twoInferentialFindings())
	batch := []map[string]any{
		refuteVerdict("R1-001", "refuted", "counterexample at a.txt:2"),
		refuteVerdict("R1-002", "stands", "sink reachable from the diff"),
	}
	payload := refutePayload(t, lineageID, batch)
	first, err := Refute(repo, lineageID, payload)
	if err != nil {
		t.Fatalf("first Refute: %v", err)
	}

	// Identical re-run: idempotent, nothing appended.
	again, err := Refute(repo, lineageID, payload)
	if err != nil {
		t.Fatalf("idempotent Refute: %v", err)
	}
	if !again.Idempotent || again.Revision != first.Revision {
		t.Errorf("idempotent outcome = %+v, want revision %s", again, first.Revision)
	}
	store, err := Open(repo, lineageID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if chain, _ := store.LoadChain(); chain.Count != 4 {
		t.Errorf("event count = %d, want 4 (start + capture + batch; idempotent rerun must not append)", chain.Count)
	}

	// A different batch is rejected: exactly one refuter batch per review.
	flipped := refutePayload(t, lineageID, []map[string]any{
		refuteVerdict("R1-001", "stands", "actually it holds"),
		refuteVerdict("R1-002", "refuted", "actually refuted"),
	})
	if _, err := Refute(repo, lineageID, flipped); err == nil {
		t.Fatal("expected rejection for a second, different batch")
	} else if !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("error must explain the one-batch rule, got: %v", err)
	}
}

func TestRefute_AfterFinalizeRejected(t *testing.T) {
	repo, _, lineageID := refuteFixture(t, twoInferentialFindings())
	refuteVerdicts(t, repo, lineageID,
		refuteVerdict("R1-001", "refuted", "counterexample at a.txt:2"),
		refuteVerdict("R1-002", "refuted", "race is unreachable"))
	if _, err := Finalize(repo, lineageID); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	_, err := Refute(repo, lineageID, refutePayload(t, lineageID, []map[string]any{
		refuteVerdict("R1-001", "stands", "changed my mind"),
	}))
	if err == nil {
		t.Fatal("expected rejection for refutation after finalize")
	}
	if !strings.Contains(err.Error(), "already finalized") {
		t.Errorf("error must explain the ordering, got: %v", err)
	}
}

func TestFinalize_RejectsMultipleRefutationBatches(t *testing.T) {
	repo, _, lineageID := refuteFixture(t, twoInferentialFindings())
	refuteVerdicts(t, repo, lineageID,
		refuteVerdict("R1-001", "refuted", "counterexample at a.txt:2"),
		refuteVerdict("R1-002", "stands", "sink reachable from the diff"))
	// Forge a second batch directly (Refute itself refuses): finalize must
	// treat the chain as violating the one-batch rule.
	store, err := Open(repo, lineageID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	forged, err := marshalRefutationEvent(lineageID, []RefutationVerdict{
		{FindingID: "R1-002", Verdict: RefutationVerdictRefuted, Evidence: "forged re-verdict"},
	})
	if err != nil {
		t.Fatalf("marshal forged batch: %v", err)
	}
	if _, err := store.Append(chain.HeadHash, Record{
		Operation: RefutationOperation, Role: refuterRole, Actor: refuterRole,
		Timestamp: "2026-01-01T00:00:00Z", Payload: forged,
	}); err != nil {
		t.Fatalf("append forged batch: %v", err)
	}
	_, err = Finalize(repo, lineageID)
	if err == nil {
		t.Fatal("expected finalize rejection for multiple refutation batches")
	}
	if !strings.Contains(err.Error(), "2 refutation batches") || !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("error must name the batch count and rule, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Escalation: unknown/insufficient evidence can never be refuted away
// ---------------------------------------------------------------------------

func TestFinalize_EscalatesInsufficientEvidence(t *testing.T) {
	repo, _, lineageID := refuteFixture(t, []map[string]any{
		severeFinding("R1-001", "risk", "insufficient", "introduced", "CRITICAL"),
	})
	_, err := Refute(repo, lineageID, refutePayload(t, lineageID, []map[string]any{
		refuteVerdict("R1-001", "refuted", "counterexample at a.txt:2"),
	}))
	if err == nil {
		t.Fatal("expected refute rejection for an insufficient-evidence finding")
	}
	if !strings.Contains(err.Error(), "insufficient") || !strings.Contains(err.Error(), "escalate") {
		t.Errorf("error must escalate, got: %v", err)
	}
	_, err = Finalize(repo, lineageID)
	if err == nil {
		t.Fatal("expected finalize refusal for insufficient evidence")
	}
	if !strings.Contains(err.Error(), "insufficient") || !strings.Contains(err.Error(), "escalate") {
		t.Errorf("finalize error must escalate, got: %v", err)
	}
}

func TestFinalize_EscalatesUnknownCausalDisposition(t *testing.T) {
	repo, _, lineageID := refuteFixture(t, []map[string]any{
		severeFinding("R1-001", "risk", "inferential", "unknown", "CRITICAL"),
	})
	_, err := Refute(repo, lineageID, refutePayload(t, lineageID, []map[string]any{
		refuteVerdict("R1-001", "refuted", "counterexample at a.txt:2"),
	}))
	if err == nil {
		t.Fatal("expected refute rejection for an unknown-disposition finding")
	}
	if !strings.Contains(err.Error(), "unknown causal disposition") || !strings.Contains(err.Error(), "escalate") {
		t.Errorf("error must escalate, got: %v", err)
	}
	_, err = Finalize(repo, lineageID)
	if err == nil {
		t.Fatal("expected finalize refusal for unknown disposition")
	}
	if !strings.Contains(err.Error(), "unknown causal disposition") || !strings.Contains(err.Error(), "escalate") {
		t.Errorf("finalize error must escalate, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Status surface
// ---------------------------------------------------------------------------

func TestStatus_RefutationsSurface(t *testing.T) {
	repo, _, lineageID := refuteFixture(t, twoInferentialFindings())
	auth := NewAuthority(repo)

	// Captured, no batch yet: both required, both pending.
	st, err := auth.Status(lineageID)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Refutations == nil {
		t.Fatal("status must surface refutations")
	}
	if *st.Refutations != (RefutationSummary{Total: 2, Refuted: 0, Stands: 0, Pending: 2}) {
		t.Errorf("refutations = %+v, want total 2 pending 2", st.Refutations)
	}

	// After the batch: verdicts split.
	refuteVerdicts(t, repo, lineageID,
		refuteVerdict("R1-001", "refuted", "counterexample at a.txt:2"),
		refuteVerdict("R1-002", "stands", "sink reachable from the diff"))
	st, err = auth.Status(lineageID)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if *st.Refutations != (RefutationSummary{Total: 2, Refuted: 1, Stands: 1, Pending: 0}) {
		t.Errorf("refutations = %+v, want refuted 1 stands 1 pending 0", st.Refutations)
	}
}

// ---------------------------------------------------------------------------
// Receipt binding: resolved/standing must be canonical and disjoint
// ---------------------------------------------------------------------------

func TestReceipt_ResolvedAndStandingMustBeDisjoint(t *testing.T) {
	origBurn := BurnEnabled
	BurnEnabled = false
	defer func() { BurnEnabled = origBurn }()
	repo, _, head := finalizeFixtureRepo(t)
	lineageID := "refute-receipt"
	store, _ := finalizeStart(t, repo, head, lineageID, []string{"risk"}, "")
	captureLensFindings(t, repo, lineageID, head, "risk", 0, twoInferentialFindings())
	refuteVerdicts(t, repo, lineageID,
		refuteVerdict("R1-001", "refuted", "counterexample at a.txt:2"),
		refuteVerdict("R1-002", "stands", "sink reachable from the diff"))
	if _, err := Finalize(repo, lineageID); err != nil {
		t.Fatalf("Finalize: %v", err)
	}

	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	ref := receiptArtifactOf(chain)
	receipt := persistedReceipt(t, repo, lineageID)

	// A forged overlap must fail validation.
	overlap := receipt
	overlap.ResolvedFindingIDs = []string{"R1-001", "R1-002"}
	if err := overlap.Validate(); err == nil || !strings.Contains(err.Error(), "both resolved and standing") {
		t.Errorf("overlap must fail validation with the disjointness rule, got: %v", err)
	}
	// A non-canonical list must fail too.
	nonCanonical := receipt
	nonCanonical.StandingFindingIDs = []string{"R1-002", "R1-001"}
	if err := nonCanonical.Validate(); err == nil || !strings.Contains(err.Error(), "canonical") {
		t.Errorf("non-canonical standing ids must fail validation, got: %v", err)
	}
	// The persisted receipt's hash binding stays intact (it covers the new field).
	if receipt.ReceiptHash != receipt.computeHash() {
		t.Error("receipt hash must bind the standing finding ids")
	}
	_ = ref
}
