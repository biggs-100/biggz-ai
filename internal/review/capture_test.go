package review

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggz-ai/biggz/model"
)

// ---------------------------------------------------------------------------
// Fixture: git repo + started review lineage
// ---------------------------------------------------------------------------

// captureFixtureRepo builds a temp git repo with two commits touching
// a.txt and b.txt and returns the head commit SHA.
func captureFixtureRepo(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	gitInit(t, repo)
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0644); err != nil {
		t.Fatalf("write base: %v", err)
	}
	runGitInDir(t, repo, "add", ".")
	runGitInDir(t, repo, "commit", "-m", "base")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("one\ntwo\nthree\n"), 0644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "b.txt"), []byte("x\ny\n"), 0644); err != nil {
		t.Fatalf("write b.txt: %v", err)
	}
	runGitInDir(t, repo, "add", ".")
	runGitInDir(t, repo, "commit", "-m", "candidate")
	return repo, runGitInDir(t, repo, "rev-parse", "HEAD")
}

// captureFixture starts a review lineage on the fixture repo and returns the
// store, expected revision (chain head), and binding.
func captureFixture(t *testing.T) (*Store, CaptureBinding, []string) {
	t.Helper()
	repo, commitSHA := captureFixtureRepo(t)
	store, err := Open(repo, "capture-lineage")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	review := New(model.ReviewSubject{Repository: repo, CommitSHA: commitSHA})
	review.State.Role = model.RoleReviewer
	review.WithStore(store)
	if err := review.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	binding := CaptureBinding{
		Repo: repo, LineageID: "capture-lineage", TargetIdentity: commitSHA,
		Lens: "risk", Order: 0, ExpectedRevision: chain.HeadHash,
	}
	return store, binding, nil
}

// captureResultJSON renders a valid reviewer payload for the given binding.
func captureResultJSON(t *testing.T, binding CaptureBinding, paths []string, subjectHash string) []byte {
	t.Helper()
	prefix := map[string]string{
		"risk": "R1", "readability": "R2", "reliability": "R3", "resilience": "R4",
		"performance": "R5", "dependencies": "R6",
	}[binding.Lens]
	envelope := map[string]any{
		"subject_hash": subjectHash,
		"inspection":   map[string]any{"status": "completed", "paths": paths},
		"lens":         binding.Lens,
		"findings": []any{
			map[string]any{
				"id": prefix + "-001", "lens": binding.Lens, "location": paths[0] + ":2",
				"severity": "CRITICAL", "claim": "unbounded loop",
				"evidence_class": "deterministic", "causal_disposition": "introduced",
			},
		},
		"evidence": []any{"go test -race reproduced the hang"},
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return payload
}

// ---------------------------------------------------------------------------
// Capture happy path + slot immutability
// ---------------------------------------------------------------------------

func TestCapture_HappyPathPersistsSlotAndManifest(t *testing.T) {
	store, binding, _ := captureFixture(t)

	preflight, err := Preflight(binding)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if preflight.Subject.SubjectHash == "" || preflight.BaseTree == "" || preflight.CandidateTree == "" {
		t.Fatalf("preflight subject is incomplete: %+v", preflight.Subject)
	}
	if len(preflight.ChangedPathManifest) != 2 {
		t.Fatalf("manifest entries = %d, want 2 (a.txt, b.txt)", len(preflight.ChangedPathManifest))
	}
	paths := ManifestPaths(preflight.ChangedPathManifest)

	payload := captureResultJSON(t, binding, paths, preflight.Subject.SubjectHash)
	outcome, err := Capture(binding, payload)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	artifact := outcome.Artifact
	if artifact.AdmissionDecision != AdmissionCompleted {
		t.Fatalf("decision = %q, want completed", artifact.AdmissionDecision)
	}
	if artifact.Revision == "" || artifact.ManifestPath == "" {
		t.Fatalf("artifact is incomplete: %+v", artifact)
	}
	if outcome.Idempotent {
		t.Error("first capture must not be idempotent")
	}
	// The event file must exist and be content-addressed.
	if _, err := os.Stat(artifact.Path); err != nil {
		t.Fatalf("event file missing: %v", err)
	}
	if filepath.Base(artifact.Path) != artifact.Revision {
		t.Errorf("event file name %s != revision %s", filepath.Base(artifact.Path), artifact.Revision)
	}
	// The manifest file must exist under the lineage manifests dir.
	manifestAbs := filepath.Join(store.Dir, artifact.ManifestPath)
	if _, err := os.Stat(manifestAbs); err != nil {
		t.Fatalf("manifest file missing: %v", err)
	}
	// Chain integrity must hold after capture.
	verdict := store.Validate()
	if !verdict.Valid {
		t.Fatalf("chain integrity after capture: %s", verdict.Reason)
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	if chain.Count != 3 {
		t.Errorf("event count = %d, want 3 (start_review, in_review, lens_result)", chain.Count)
	}
	if chain.HeadHash != artifact.Revision {
		t.Errorf("head %s != artifact revision %s", chain.HeadHash, artifact.Revision)
	}
	// The event must carry the canonical payload + manifest reference.
	var payloadRecord lensResultEventPayload
	if err := json.Unmarshal(chain.Records[2].Payload, &payloadRecord); err != nil {
		t.Fatalf("parse lens_result payload: %v", err)
	}
	if payloadRecord.Schema != LensResultEventSchema {
		t.Errorf("event schema = %q", payloadRecord.Schema)
	}
	if payloadRecord.ManifestSHA256 != preflight.Subject.ChangedPathManifestSHA256 {
		t.Error("event manifest reference does not match the subject manifest digest")
	}
	if len(payloadRecord.CandidateCausalFindingIDs) != 1 || payloadRecord.CandidateCausalFindingIDs[0] != "R1-001" {
		t.Errorf("candidate causal ids = %v, want [R1-001]", payloadRecord.CandidateCausalFindingIDs)
	}
}

func TestCapture_IdempotentRecaptureReturnsExistingSlot(t *testing.T) {
	store, binding, _ := captureFixture(t)
	preflight, err := Preflight(binding)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	paths := ManifestPaths(preflight.ChangedPathManifest)
	payload := captureResultJSON(t, binding, paths, preflight.Subject.SubjectHash)

	first, err := Capture(binding, payload)
	if err != nil {
		t.Fatalf("first Capture: %v", err)
	}
	second, err := Capture(binding, payload)
	if err != nil {
		t.Fatalf("second Capture: %v", err)
	}
	if !second.Idempotent {
		t.Error("re-capture of identical payload must be idempotent")
	}
	if second.Artifact.Revision != first.Artifact.Revision {
		t.Errorf("idempotent re-capture returned revision %s, want existing %s", second.Artifact.Revision, first.Artifact.Revision)
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	if chain.Count != 3 {
		t.Errorf("event count = %d after idempotent re-capture, want 3", chain.Count)
	}
}

func TestCapture_RejectsDifferentCanonicalBytes(t *testing.T) {
	store, binding, _ := captureFixture(t)
	preflight, err := Preflight(binding)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	paths := ManifestPaths(preflight.ChangedPathManifest)

	first, err := Capture(binding, captureResultJSON(t, binding, paths, preflight.Subject.SubjectHash))
	if err != nil {
		t.Fatalf("first Capture: %v", err)
	}
	// Same envelope but a different claim: different canonical bytes.
	different := captureResultJSON(t, binding, paths, preflight.Subject.SubjectHash)
	different = bytes.Replace(different, []byte("unbounded loop"), []byte("off-by-one"), 1)
	if _, err := Capture(binding, different); err == nil {
		t.Fatal("expected rejection for different canonical bytes")
	} else if !strings.Contains(err.Error(), "immutable") {
		t.Errorf("want immutable-slot rejection, got: %v", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	if chain.Count != 3 {
		t.Errorf("event count = %d after conflict, want 3 (rejection must not append)", chain.Count)
	}
	if chain.HeadHash != first.Artifact.Revision {
		t.Error("head must remain on the first capture")
	}
}

func TestCapture_RejectsStaleExpectedRevision(t *testing.T) {
	_, binding, _ := captureFixture(t)
	preflight, err := Preflight(binding)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	paths := ManifestPaths(preflight.ChangedPathManifest)
	if _, err := Capture(binding, captureResultJSON(t, binding, paths, preflight.Subject.SubjectHash)); err != nil {
		t.Fatalf("first Capture: %v", err)
	}
	// The head moved; a different lens against the stale revision must be
	// rejected because the slot is occupied by the risk capture.
	stale := binding
	stale.Lens = "readability"
	stale.Order = 1
	staleSubject, err := NewArtifactSubject(stale.LineageID, stale.ExpectedRevision, stale.TargetIdentity,
		preflight.BaseTree, preflight.CandidateTree, preflight.Subject.ChangedPathManifestSHA256, stale.Lens, stale.Order)
	if err != nil {
		t.Fatalf("NewArtifactSubject: %v", err)
	}
	if _, err := Capture(stale, captureResultJSON(t, stale, paths, staleSubject.SubjectHash)); err == nil {
		t.Fatal("expected stale-revision rejection")
	} else if !strings.Contains(err.Error(), "occupied") {
		t.Errorf("want slot-occupied rejection, got: %v", err)
	}
	// Re-capturing the original lens with the stale revision must be rejected
	// when the canonical bytes differ.
	different := captureResultJSON(t, binding, paths, preflight.Subject.SubjectHash)
	different = bytes.Replace(different, []byte("unbounded loop"), []byte("off-by-one"), 1)
	if _, err := Capture(binding, different); err == nil {
		t.Fatal("expected stale-revision rejection")
	} else if !strings.Contains(err.Error(), "immutable") {
		t.Errorf("want immutable-slot rejection, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Preflight
// ---------------------------------------------------------------------------

func TestPreflight_PersistsNothing(t *testing.T) {
	store, binding, _ := captureFixture(t)
	preflight, err := Preflight(binding)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	// The preflight must verify the subject hash binding when provided.
	binding.SubjectHash = preflight.Subject.SubjectHash
	if _, err := Preflight(binding); err != nil {
		t.Fatalf("Preflight with matching subject hash: %v", err)
	}
	binding.SubjectHash = strings.Repeat("e", 64)
	if _, err := Preflight(binding); err == nil {
		t.Fatal("expected rejection for mismatched subject hash")
	}
	if _, err := os.Stat(filepath.Join(store.Dir, ManifestsDirName)); !os.IsNotExist(err) {
		t.Error("preflight must not create the manifests directory")
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	if chain.Count != 2 {
		t.Errorf("event count = %d after preflight, want 2", chain.Count)
	}
}

func TestPreflight_RejectsStaleExpectedRevision(t *testing.T) {
	_, binding, _ := captureFixture(t)
	binding.ExpectedRevision = strings.Repeat("d", 64)
	if _, err := Preflight(binding); err == nil {
		t.Fatal("expected rejection for stale expected revision")
	}
}

// ---------------------------------------------------------------------------
// Admission rejections at the capture boundary
// ---------------------------------------------------------------------------

func TestCapture_RejectsWrongSubjectHash(t *testing.T) {
	_, binding, _ := captureFixture(t)
	preflight, err := Preflight(binding)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	paths := ManifestPaths(preflight.ChangedPathManifest)
	payload := captureResultJSON(t, binding, paths, strings.Repeat("f", 64))
	_, err = Capture(binding, payload)
	if err == nil {
		t.Fatal("expected rejection for wrong subject hash")
	}
	var admissionErr *ArtifactAdmissionError
	if !errors.As(err, &admissionErr) || admissionErr.Admission.Decision != AdmissionBindingMismatch {
		t.Fatalf("want binding_mismatch admission error, got %T: %v", err, err)
	}
}

func TestCapture_RejectsIncompleteInspection(t *testing.T) {
	_, binding, _ := captureFixture(t)
	preflight, err := Preflight(binding)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	paths := ManifestPaths(preflight.ChangedPathManifest)
	envelope := map[string]any{
		"subject_hash": preflight.Subject.SubjectHash,
		"inspection":   map[string]any{"status": "failed", "paths": paths},
		"lens":         binding.Lens,
		"findings":     []any{},
		"evidence":     []any{"go test -race reproduced the hang"},
	}
	payload, _ := json.Marshal(envelope)
	_, err = Capture(binding, payload)
	if err == nil || !strings.Contains(err.Error(), "completed") {
		t.Fatalf("want incomplete-inspection rejection, got %v", err)
	}
}

func TestCapture_RejectsUnknownField(t *testing.T) {
	_, binding, _ := captureFixture(t)
	preflight, err := Preflight(binding)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	paths := ManifestPaths(preflight.ChangedPathManifest)
	envelope := map[string]any{
		"subject_hash": preflight.Subject.SubjectHash,
		"inspection":   map[string]any{"status": "completed", "paths": paths},
		"lens":         binding.Lens,
		"findings":     []any{},
		"evidence":     []any{"go test -race reproduced the hang"},
		"summary":      "extra field",
	}
	payload, _ := json.Marshal(envelope)
	_, err = Capture(binding, payload)
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("want unknown-field rejection, got %v", err)
	}
}

func TestCapture_RejectsMissingManifestPath(t *testing.T) {
	_, binding, _ := captureFixture(t)
	preflight, err := Preflight(binding)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	paths := ManifestPaths(preflight.ChangedPathManifest)
	payload := captureResultJSON(t, binding, paths[:1], preflight.Subject.SubjectHash)
	if _, err := Capture(binding, payload); err == nil {
		t.Fatal("expected rejection for incomplete inspection paths")
	}
}

func TestCapture_RejectsSevereFindingMissingDisposition(t *testing.T) {
	_, binding, _ := captureFixture(t)
	preflight, err := Preflight(binding)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	paths := ManifestPaths(preflight.ChangedPathManifest)
	envelope := map[string]any{
		"subject_hash": preflight.Subject.SubjectHash,
		"inspection":   map[string]any{"status": "completed", "paths": paths},
		"lens":         binding.Lens,
		"findings": []any{
			map[string]any{
				"id": "R1-001", "lens": binding.Lens, "location": paths[0] + ":2",
				"severity": "BLOCKER", "claim": "unbounded loop",
			},
		},
		"evidence": []any{"go test -race reproduced the hang"},
	}
	payload, _ := json.Marshal(envelope)
	if _, err := Capture(binding, payload); err == nil {
		t.Fatal("expected rejection for severe finding without evidence_class/causal_disposition")
	}
}

func TestCapture_RejectsEmptyPayload(t *testing.T) {
	_, binding, _ := captureFixture(t)
	if _, err := Capture(binding, []byte("   ")); err == nil {
		t.Fatal("expected rejection for empty payload")
	}
}

// ---------------------------------------------------------------------------
// Binding validation
// ---------------------------------------------------------------------------

func TestCapture_RejectsTargetMismatch(t *testing.T) {
	_, binding, _ := captureFixture(t)
	binding.TargetIdentity = strings.Repeat("9", 40)
	if _, err := Preflight(binding); err == nil {
		t.Fatal("expected rejection for target mismatch")
	}
}

func TestRepositoryContext_EchoValidation(t *testing.T) {
	contextJSON := fmt.Sprintf(`{"repo":"/tmp/repo","lineage_id":"capture-lineage","lens":"risk","order":0,"subject_hash":"%s"}`,
		strings.Repeat("a", 64))
	context, err := DecodeRepositoryContext([]byte(contextJSON))
	if err != nil {
		t.Fatalf("DecodeRepositoryContext: %v", err)
	}
	if context.Repo != "/tmp/repo" {
		t.Errorf("repo = %q", context.Repo)
	}
	if err := context.Validate(CaptureBinding{
		LineageID: "capture-lineage", Lens: "risk", Order: 0, SubjectHash: strings.Repeat("a", 64),
	}); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if err := context.Validate(CaptureBinding{LineageID: "other"}); err == nil {
		t.Fatal("expected echo mismatch rejection")
	}
	if _, err := DecodeRepositoryContext([]byte(`{"repo":"/tmp/repo","unknown":true}`)); err == nil {
		t.Fatal("expected rejection of unknown context key")
	}
}

// ---------------------------------------------------------------------------
// Status lenses
// ---------------------------------------------------------------------------

func TestStatus_ShowsCapturedLenses(t *testing.T) {
	store, binding, _ := captureFixture(t)
	preflight, err := Preflight(binding)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	paths := ManifestPaths(preflight.ChangedPathManifest)
	if _, err := Capture(binding, captureResultJSON(t, binding, paths, preflight.Subject.SubjectHash)); err != nil {
		t.Fatalf("Capture: %v", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	lenses := CapturedLenses(chain)
	if len(lenses) != 1 {
		t.Fatalf("captured lenses = %d, want 1", len(lenses))
	}
	lens := lenses[0]
	if lens.Lens != "risk" || lens.SelectedOrder != 0 {
		t.Errorf("lens = %q order %d", lens.Lens, lens.SelectedOrder)
	}
	if lens.SubjectHash != preflight.Subject.SubjectHash {
		t.Error("lens subject hash mismatch")
	}
	if lens.Status != CapturedLensStatus {
		t.Errorf("status = %q, want captured", lens.Status)
	}
	if !strings.HasPrefix(filepath.ToSlash(lens.ManifestPath), ManifestsDirName+"/") {
		t.Errorf("manifest path = %q, want under manifests/", lens.ManifestPath)
	}
}
