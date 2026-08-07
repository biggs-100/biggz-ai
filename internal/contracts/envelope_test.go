package contracts

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/review"
	"github.com/biggs-100/biggz-ai/model"
)

// $ids of the contract schemas under test (declared in the schema files).
const (
	contractID             = "https://biggz-ai.dev/contracts/review-integration/v1/schemas/contract.schema.json"
	startID                = "https://biggz-ai.dev/contracts/review-integration/v1/schemas/start.schema.json"
	consentID              = "https://biggz-ai.dev/contracts/review-integration/v1/schemas/consent.schema.json"
	resultArtifactID       = "https://biggz-ai.dev/contracts/review-integration/v1/schemas/result-artifact.schema.json"
	receiptID              = "https://biggz-ai.dev/contracts/review-integration/v1/schemas/receipt.schema.json"
	recordID               = "https://biggz-ai.dev/contracts/review-integration/v1/schemas/record.schema.json"
	refutationID           = "https://biggz-ai.dev/contracts/review-integration/v1/schemas/refutation.schema.json"
	refutationEventID      = "https://biggz-ai.dev/contracts/review-integration/v1/schemas/refutation-event.schema.json"
	lensResultEventID      = "https://biggz-ai.dev/contracts/review-integration/v1/schemas/lens-result-event.schema.json"
	completeEventID        = "https://biggz-ai.dev/contracts/review-integration/v1/schemas/complete-event.schema.json"
	invalidateEventID      = "https://biggz-ai.dev/contracts/review-integration/v1/schemas/invalidate-event.schema.json"
	withdrawEventID        = "https://biggz-ai.dev/contracts/review-integration/v1/schemas/withdraw-event.schema.json"
	disposeEventID         = "https://biggz-ai.dev/contracts/review-integration/v1/schemas/dispose-event.schema.json"
	reopenEventID          = "https://biggz-ai.dev/contracts/review-integration/v1/schemas/reopen-event.schema.json"
	verificationRetryID    = "https://biggz-ai.dev/contracts/review-integration/v1/schemas/verification-retry.schema.json"
	reconcileID            = "https://biggz-ai.dev/contracts/review-integration/v1/schemas/reconcile.schema.json"
	inspectID              = "https://biggz-ai.dev/contracts/review-integration/v1/schemas/inspect.schema.json"
	rddStatusID            = "https://biggz-ai.dev/contracts/review-integration/v1/schemas/rdd-status.schema.json"
	rddConsentID           = "https://biggz-ai.dev/contracts/review-integration/v1/schemas/rdd-consent.schema.json"
	editAuthorityConsentID = "https://biggz-ai.dev/contracts/sdd-integration/v1/schemas/edit-authority-consent.schema.json"
	verifyAdmissionID      = "https://biggz-ai.dev/contracts/sdd-integration/v1/schemas/verify-admission.schema.json"
)

// ---------------------------------------------------------------------------
// Local git drivers (the review package's test helpers are package-private,
// so this package re-implements the minimal fixture repo).
// ---------------------------------------------------------------------------

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// fixtureRepo builds a git repo whose head commit adds a.txt and b.txt on
// top of a base commit (5 changed lines) and returns (repo, head).
func fixtureRepo(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	gitRun(t, repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0644); err != nil {
		t.Fatalf("write base.txt: %v", err)
	}
	gitRun(t, repo, "add", ".")
	gitRun(t, repo, "commit", "-q", "-m", "base")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("one\ntwo\nthree\n"), 0644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "b.txt"), []byte("x\ny\n"), 0644); err != nil {
		t.Fatalf("write b.txt: %v", err)
	}
	gitRun(t, repo, "add", ".")
	gitRun(t, repo, "commit", "-q", "-m", "candidate")
	return repo, gitRun(t, repo, "rev-parse", "HEAD")
}

// startLineage starts a lineage with a derived start plan, mirroring the
// review package's finalizeStart test driver.
func startLineage(t *testing.T, repo, commitSHA, lineageID string, lenses []string) *review.Store {
	t.Helper()
	store, err := review.Open(repo, lineageID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	base, lines, err := review.DeriveOriginalChangedLines(repo, commitSHA, "")
	if err != nil {
		t.Fatalf("DeriveOriginalChangedLines: %v", err)
	}
	budget, err := review.DeriveCorrectionBudget(lines)
	if err != nil {
		t.Fatalf("DeriveCorrectionBudget: %v", err)
	}
	plan := review.StartEventPayload{
		Schema: review.ReviewStartEventSchema, Repository: repo, CommitSHA: commitSHA,
		BaseRef: base, OriginalChangedLines: lines, CorrectionBudget: budget,
		MaxCorrectionAttempts: review.MaxCompactCorrectionAttempts, SelectedLenses: lenses,
	}
	r := review.New(model.ReviewSubject{Repository: repo, CommitSHA: commitSHA})
	r.State.Role = model.RoleReviewer
	r.WithStore(store).FreezeStartPlan(plan)
	if err := r.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return store
}

// captureLensClean captures a clean (no-findings) reviewer result for the
// given lens/order at the current chain head.
func captureLensClean(t *testing.T, repo, lineageID, commitSHA, lens string, order int) review.CapturedArtifact {
	t.Helper()
	store, err := review.Open(repo, lineageID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	binding := review.CaptureBinding{
		Repo: repo, LineageID: lineageID, TargetIdentity: commitSHA,
		Lens: lens, Order: order, ExpectedRevision: chain.HeadHash,
	}
	preflight, err := review.Preflight(binding)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	payload, err := json.Marshal(map[string]any{
		"subject_hash": preflight.Subject.SubjectHash,
		"inspection":   map[string]any{"status": "completed", "paths": review.ManifestPaths(preflight.ChangedPathManifest)},
		"lens":         lens,
		"findings":     []any{},
		"evidence":     []any{"clean sweep"},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	outcome, err := review.Capture(binding, payload)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if outcome.Artifact.AdmissionDecision != review.AdmissionCompleted {
		t.Fatalf("decision = %q, want completed", outcome.Artifact.AdmissionDecision)
	}
	return outcome.Artifact
}

// payloadSchemaID maps a chain event operation to the $id of the schema that
// formalizes its payload envelope. start_review genesis payloads are the one
// exception (ReviewSubject-shaped start payloads carry the start schema const).
func payloadSchemaID(operation string) string {
	switch operation {
	case "start_review":
		return startID
	case review.LensResultOperation:
		return lensResultEventID
	case review.CompleteReviewOperation:
		return completeEventID
	case review.RefutationOperation:
		return refutationEventID
	case review.InvalidateOperation:
		return invalidateEventID
	case review.WithdrawOperation:
		return withdrawEventID
	case review.DisposeOperation:
		return disposeEventID
	case review.ReopenOperation:
		return reopenEventID
	}
	return ""
}

// ---------------------------------------------------------------------------
// Emitted-payload conformance
// ---------------------------------------------------------------------------

// TestEnvelopeConformance_ContractEnvelope validates the real BuildNextTransition
// output: the negotiated routing envelope of a lineage mid-capture.
func TestEnvelopeConformance_ContractEnvelope(t *testing.T) {
	repo, head := fixtureRepo(t)
	startLineage(t, repo, head, "env-contract", []string{"risk", "readability"})
	captureLensClean(t, repo, "env-contract", head, "risk", 0)

	env, err := review.NewAuthority(repo).BuildNextTransition("env-contract")
	if err != nil {
		t.Fatalf("BuildNextTransition: %v", err)
	}
	payload, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := ValidateEnvelope(contractID, payload); err != nil {
		t.Fatalf("contract envelope rejected by contract.schema.json: %v", err)
	}
}

// TestEnvelopeConformance_ContractEnvelopeTerminal validates a stop envelope:
// a finalized lineage routes stop ready_for_gates.
func TestEnvelopeConformance_ContractEnvelopeTerminal(t *testing.T) {
	repo, head := fixtureRepo(t)
	startLineage(t, repo, head, "env-gate", []string{"risk"})
	captureLensClean(t, repo, "env-gate", head, "risk", 0)
	if _, err := review.Finalize(repo, "env-gate"); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	env, err := review.NewAuthority(repo).BuildNextTransition("env-gate")
	if err != nil {
		t.Fatalf("BuildNextTransition: %v", err)
	}
	if env.NextTransition.Type != "stop" {
		t.Fatalf("transition = %+v, want stop", env.NextTransition)
	}
	payload, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := ValidateEnvelope(contractID, payload); err != nil {
		t.Fatalf("stop envelope rejected by contract.schema.json: %v", err)
	}
}

// TestEnvelopeConformance_ConsentEnvelope validates the real
// buildConsentEnvelope output via the EvaluateStartConsent relay decision.
func TestEnvelopeConformance_ConsentEnvelope(t *testing.T) {
	decision, err := review.EvaluateStartConsent(
		model.ReviewSubject{Repository: "https://example.com/acme/repo", CommitSHA: "dba504c5ef0ecb92a006074854ac94a380fa26fe"},
		"env-consent",
		review.RiskInput{Paths: []string{"internal/auth/token.go"}, ChangedLines: 20},
		[]string{"risk", "readability", "reliability", "resilience"},
		"relay", false)
	if err != nil {
		t.Fatalf("EvaluateStartConsent: %v", err)
	}
	if decision.Envelope == nil {
		t.Fatal("relay decision carries no envelope")
	}
	payload, err := json.Marshal(decision.Envelope)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := ValidateEnvelope(consentID, payload); err != nil {
		t.Fatalf("consent envelope rejected by consent.schema.json: %v", err)
	}
}

// TestEnvelopeConformance_CapturedArtifact validates the real Capture output.
func TestEnvelopeConformance_CapturedArtifact(t *testing.T) {
	repo, head := fixtureRepo(t)
	startLineage(t, repo, head, "env-artifact", []string{"risk"})
	artifact := captureLensClean(t, repo, "env-artifact", head, "risk", 0)

	payload, err := json.Marshal(artifact)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := ValidateEnvelope(resultArtifactID, payload); err != nil {
		t.Fatalf("captured artifact rejected by result-artifact.schema.json: %v", err)
	}
}

// TestEnvelopeConformance_PersistedReceipt validates the receipt artifact
// Finalize materializes under receipts/<sha256>.json.
func TestEnvelopeConformance_PersistedReceipt(t *testing.T) {
	repo, head := fixtureRepo(t)
	store := startLineage(t, repo, head, "env-receipt", []string{"risk", "readability"})
	captureLensClean(t, repo, "env-receipt", head, "risk", 0)
	captureLensClean(t, repo, "env-receipt", head, "readability", 1)
	if _, err := review.Finalize(repo, "env-receipt"); err != nil {
		t.Fatalf("Finalize: %v", err)
	}

	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	var ref struct {
		ReceiptPath string `json:"receipt_path"`
		ReceiptHash string `json:"receipt_hash"`
	}
	found := false
	for index := len(chain.Records) - 1; index >= 0; index-- {
		if chain.Records[index].Operation != review.CompleteReviewOperation {
			continue
		}
		if err := json.Unmarshal(chain.Records[index].Payload, &ref); err != nil {
			t.Fatalf("unmarshal complete_review payload: %v", err)
		}
		found = true
		break
	}
	if !found || ref.ReceiptPath == "" {
		t.Fatal("lineage carries no complete_review receipt reference")
	}
	receiptBytes, err := os.ReadFile(filepath.Join(store.Dir, ref.ReceiptPath))
	if err != nil {
		t.Fatalf("read receipt artifact %s: %v", ref.ReceiptPath, err)
	}
	if err := ValidateEnvelope(receiptID, receiptBytes); err != nil {
		t.Fatalf("persisted receipt rejected by receipt.schema.json: %v", err)
	}
}

// TestEnvelopeConformance_ChainEventsAndRecords validates every real event
// payload of a full lifecycle against its event schema, and every record
// against record.schema.json — the schema that governs every content-
// addressed event file.
func TestEnvelopeConformance_ChainEventsAndRecords(t *testing.T) {
	repo, head := fixtureRepo(t)
	store := startLineage(t, repo, head, "env-chain", []string{"risk"})
	captureLensClean(t, repo, "env-chain", head, "risk", 0)
	if _, err := review.Finalize(repo, "env-chain"); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	if _, err := review.Invalidate(repo, "env-chain", "audit requirement"); err != nil {
		t.Fatalf("Invalidate: %v", err)
	}

	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	if chain.Count == 0 {
		t.Fatal("chain is empty")
	}
	for index := range chain.Records {
		rec := chain.Records[index]
		recordBytes, err := json.Marshal(rec)
		if err != nil {
			t.Fatalf("record %d marshal: %v", index, err)
		}
		if err := ValidateEnvelope(recordID, recordBytes); err != nil {
			t.Fatalf("record %d (%s) rejected by record.schema.json: %v", index, rec.Operation, err)
		}
		id := payloadSchemaID(rec.Operation)
		if id == "" {
			// Marker events (e.g. in_review) carry no payload envelope and
			// nothing to formalize beyond the record itself.
			if bytes.Equal(bytes.TrimSpace(rec.Payload), []byte("null")) || len(rec.Payload) == 0 {
				continue
			}
			t.Fatalf("record %d operation %q has no formalized payload schema", index, rec.Operation)
		}
		if err := ValidateEnvelope(id, rec.Payload); err != nil {
			t.Fatalf("record %d payload (%s) rejected by its event schema: %v", index, rec.Operation, err)
		}
	}
}

// TestEnvelopeConformance_RefutationRoundTrip validates the strict
// DecodeRefutationInput round trip: input bytes decode, re-marshal, and
// validate against refutation.schema.json.
func TestEnvelopeConformance_RefutationRoundTrip(t *testing.T) {
	input := review.RefutationInput{
		Schema:  review.RefutationSchema,
		Lineage: "env-refute",
		Verdicts: []review.RefutationVerdict{
			{FindingID: "R1-001", Verdict: "refuted", Evidence: "The cited function guards the path before any token is read."},
			{FindingID: "R1-002", Verdict: "stands", Evidence: "The reproducer still reaches the unguarded branch on empty input."},
		},
	}
	original, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	decoded, err := review.DecodeRefutationInput(original)
	if err != nil {
		t.Fatalf("DecodeRefutationInput: %v", err)
	}
	roundTripped, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if err := ValidateEnvelope(refutationID, original); err != nil {
		t.Fatalf("refutation input rejected by refutation.schema.json: %v", err)
	}
	if err := ValidateEnvelope(refutationID, roundTripped); err != nil {
		t.Fatalf("refutation round trip rejected by refutation.schema.json: %v", err)
	}
}

// TestEnvelopeConformance_VerificationRetry validates the real
// retry-final-verification report of a finalized lineage.
func TestEnvelopeConformance_VerificationRetry(t *testing.T) {
	repo, head := fixtureRepo(t)
	startLineage(t, repo, head, "env-retry", []string{"risk"})
	captureLensClean(t, repo, "env-retry", head, "risk", 0)
	if _, err := review.Finalize(repo, "env-retry"); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	report, err := review.RetryFinalVerification(repo, "env-retry")
	if err != nil {
		t.Fatalf("RetryFinalVerification: %v", err)
	}
	if !report.Passed {
		t.Fatalf("report = %+v, want passed", report)
	}
	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := ValidateEnvelope(verificationRetryID, payload); err != nil {
		t.Fatalf("verification report rejected by verification-retry.schema.json: %v", err)
	}
}

// TestEnvelopeConformance_Inspect validates the real inspect output of a
// lineage with a lens_result event.
func TestEnvelopeConformance_Inspect(t *testing.T) {
	repo, head := fixtureRepo(t)
	startLineage(t, repo, head, "env-inspect", []string{"risk"})
	captureLensClean(t, repo, "env-inspect", head, "risk", 0)
	result, err := review.Inspect(repo, "env-inspect")
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := ValidateEnvelope(inspectID, payload); err != nil {
		t.Fatalf("inspect report rejected by inspect.schema.json: %v", err)
	}
}
