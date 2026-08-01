package review

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// gateFixture builds a finalized lineage on the fixture repo (base commit +
// candidate commit touching a.txt and b.txt), capturing one risk lens. HOME is
// isolated so the RDD kill switch is deterministic.
func gateFixture(t *testing.T) (repo, head string, outcome FinalizeOutcome) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	repo, _, head = finalizeFixtureRepo(t)
	finalizeStart(t, repo, head, "gate-fixture", []string{"risk"}, "")
	captureLens(t, repo, "gate-fixture", head, "risk", 0)
	var err error
	outcome, err = Finalize(repo, "gate-fixture")
	if err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	return repo, head, outcome
}

// gateFixtureWithExtraCommit is gateFixture plus one unreviewed commit (c.txt)
// on top of the reviewed candidate, with HEAD checked back out at the
// candidate. Returns the repo, the reviewed head, and the extra commit SHA.
func gateFixtureWithExtraCommit(t *testing.T) (repo, reviewedHead, extraCommit string) {
	t.Helper()
	repo, reviewedHead, _ = gateFixture(t)
	if err := os.WriteFile(filepath.Join(repo, "c.txt"), []byte("new\n"), 0644); err != nil {
		t.Fatalf("write c.txt: %v", err)
	}
	runGitInDir(t, repo, "add", ".")
	runGitInDir(t, repo, "commit", "-m", "unreviewed extra commit")
	extraCommit = runGitInDir(t, repo, "rev-parse", "HEAD")
	runGitInDir(t, repo, "checkout", "-q", reviewedHead)
	return repo, reviewedHead, extraCommit
}

// ---------------------------------------------------------------------------
// Happy path: every gate kind with a finalized lineage
// ---------------------------------------------------------------------------

func TestEvaluateGate_PostApplyHappyPath(t *testing.T) {
	repo, _, outcome := gateFixture(t)

	result, err := EvaluateGate(GatePostApply, repo, "gate-fixture", GateOptions{})
	if err != nil {
		t.Fatalf("EvaluateGate: %v", err)
	}
	if !result.Passed || !result.Allowed {
		t.Fatalf("expected pass, got passed=%t allowed=%t reasons=%v", result.Passed, result.Allowed, result.Reasons)
	}
	if result.Delivery != DeliveryReceiptGoverned {
		t.Errorf("delivery = %q, want %q", result.Delivery, DeliveryReceiptGoverned)
	}
	if result.ReceiptHash != outcome.ReceiptHash {
		t.Errorf("receipt_hash = %s, want %s", result.ReceiptHash, outcome.ReceiptHash)
	}
	if result.Findings == nil || result.Findings.Blocking != 0 || result.Findings.Resolved != 1 || result.Findings.FollowUp != 0 {
		t.Errorf("findings = %+v, want blocking=0 resolved=1 follow_up=0", result.Findings)
	}
}

func TestEvaluateGate_PreCommitHappyPath(t *testing.T) {
	repo, _, _ := gateFixture(t)

	result, err := EvaluateGate(GatePreCommit, repo, "gate-fixture", GateOptions{})
	if err != nil {
		t.Fatalf("EvaluateGate: %v", err)
	}
	if !result.Passed || !result.Allowed {
		t.Fatalf("expected pass (clean index reproduces the reviewed candidate), got reasons=%v", result.Reasons)
	}
}

func TestEvaluateGate_PrePushHappyPath(t *testing.T) {
	repo, _, _ := gateFixture(t)

	result, err := EvaluateGate(GatePrePush, repo, "gate-fixture", GateOptions{})
	if err != nil {
		t.Fatalf("EvaluateGate: %v", err)
	}
	if !result.Passed || !result.Allowed {
		t.Fatalf("expected pass (reviewed commit at HEAD, no unreviewed commits), got reasons=%v", result.Reasons)
	}
}

func TestEvaluateGate_PrePRHappyPath(t *testing.T) {
	repo, _, _ := gateFixture(t)

	result, err := EvaluateGate(GatePrePR, repo, "gate-fixture", GateOptions{})
	if err != nil {
		t.Fatalf("EvaluateGate: %v", err)
	}
	if !result.Passed || !result.Allowed {
		t.Fatalf("expected pass (base boundary diff inside the reviewed manifest), got reasons=%v", result.Reasons)
	}
}

func TestEvaluateGate_ReleaseHappyPath(t *testing.T) {
	repo, _, _ := gateFixture(t)

	result, err := EvaluateGate(GateRelease, repo, "gate-fixture", GateOptions{})
	if err != nil {
		t.Fatalf("EvaluateGate: %v", err)
	}
	if !result.Passed || !result.Allowed {
		t.Fatalf("expected pass (reviewed commit at HEAD), got reasons=%v", result.Reasons)
	}
}

func TestEvaluateGate_RejectsUnknownKind(t *testing.T) {
	_, err := EvaluateGate("pre-merge", "", "x", GateOptions{})
	if err == nil || !strings.Contains(err.Error(), "unsupported review gate") {
		t.Fatalf("expected unsupported-kind error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Receipt requirements
// ---------------------------------------------------------------------------

func TestEvaluateGate_MissingReceiptNamesFinalize(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	repo, _, head := finalizeFixtureRepo(t)
	finalizeStart(t, repo, head, "gate-noreceipt", []string{"risk"}, "")
	captureLens(t, repo, "gate-noreceipt", head, "risk", 0)

	result, err := EvaluateGate(GatePostApply, repo, "gate-noreceipt", GateOptions{})
	if err != nil {
		t.Fatalf("EvaluateGate: %v", err)
	}
	if result.Allowed || result.Passed {
		t.Fatal("expected denial without a persisted receipt")
	}
	if !strings.Contains(result.Reason, "finalize") {
		t.Errorf("reason must name finalize as the required step, got: %v", result.Reason)
	}
	if result.Delivery == DeliveryDisabledUnmanaged {
		t.Error("managed evaluation must not report disabled/unmanaged")
	}
}

func TestEvaluateGate_RejectsTamperedReceipt(t *testing.T) {
	repo, _, outcome := gateFixture(t)
	store, err := Open(repo, "gate-fixture")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	abs := filepath.Join(store.Dir, outcome.ReceiptPath)
	original, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read receipt: %v", err)
	}
	if err := os.WriteFile(abs, append(original, []byte("tamper")...), 0644); err != nil {
		t.Fatalf("tamper receipt: %v", err)
	}

	result, err := EvaluateGate(GatePostApply, repo, "gate-fixture", GateOptions{})
	if err != nil {
		t.Fatalf("EvaluateGate: %v", err)
	}
	if result.Allowed {
		t.Fatal("expected denial for a tampered receipt artifact")
	}
	if !strings.Contains(result.Reason, "receipt") {
		t.Errorf("reason must name the receipt, got: %v", result.Reason)
	}
}

func TestEvaluateGate_RejectsForeignReceipt(t *testing.T) {
	repo, _, _ := gateFixture(t)
	store, err := Open(repo, "gate-fixture")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	binding, err := deriveGateBinding(repo, chain)
	if err != nil {
		t.Fatalf("deriveGateBinding: %v", err)
	}

	// Forge a self-consistent but foreign receipt (valid content hash, wrong
	// lineage and revisions) and append a complete_review event referencing it.
	forged := PersistedReceipt{
		Schema: ReviewReceiptSchema, LineageID: "foreign-lineage", Generation: 1,
		GenesisRevision: strings.Repeat("a", 64), HeadRevision: strings.Repeat("b", 64),
		BaseTree: binding.baseTree, InitialReviewTree: binding.candidateTree, FinalCandidateTree: binding.candidateTree,
		PathsDigest: binding.manifestSHA256, FixDeltaHash: EmptyFixDeltaHash, EvidenceHash: EmptyFixDeltaHash,
		RiskTier: "medium", SelectedLenses: []string{"risk"},
		LensSubjects: []ReceiptLensSubject{{
			Lens: "risk", SelectedOrder: 0,
			SubjectHash: "sha256:" + strings.Repeat("c", 64), ResultHash: "sha256:" + strings.Repeat("d", 64),
		}},
		TerminalState: ReviewReceiptTerminalState,
	}
	forged.ReceiptHash = forged.computeHash()
	if err := forged.Validate(); err != nil {
		t.Fatalf("forged receipt must be self-consistent: %v", err)
	}
	path, err := writeReceiptLocked(store, forged)
	if err != nil {
		t.Fatalf("writeReceiptLocked: %v", err)
	}
	evtPayload, err := json.Marshal(completeEventPayload{
		Schema: FinalizeEventSchema, ReceiptPath: path, ReceiptHash: forged.ReceiptHash,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if _, err := store.Append(chain.HeadHash, Record{
		Operation: CompleteReviewOperation,
		Role:      "Lead",
		Actor:     "Lead",
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Payload:   evtPayload,
	}); err != nil {
		t.Fatalf("append: %v", err)
	}

	result, err := EvaluateGate(GatePostApply, repo, "gate-fixture", GateOptions{})
	if err != nil {
		t.Fatalf("EvaluateGate: %v", err)
	}
	if result.Allowed {
		t.Fatal("expected denial for a foreign receipt")
	}
	if !strings.Contains(result.Reason, "receipt binding") {
		t.Errorf("reason must name the receipt binding, got: %v", result.Reason)
	}
}

// ---------------------------------------------------------------------------
// Blocking findings
// ---------------------------------------------------------------------------

func TestEvaluateGate_BlocksUnresolvedFindingAfterResume(t *testing.T) {
	repo, head, _ := gateFixture(t)
	store, err := Open(repo, "gate-fixture")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	// Advance the chain after finalize and capture a second lens whose
	// candidate-causal finding is NOT covered by the persisted receipt.
	if _, err := store.Append(chain.HeadHash, Record{
		Operation: "resume",
		Role:      "Admin",
		Actor:     "Admin",
		Timestamp: time.Now().Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatalf("append resume: %v", err)
	}
	fresh, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	binding := CaptureBinding{
		Repo: repo, LineageID: "gate-fixture", TargetIdentity: head,
		Lens: "readability", Order: 1, ExpectedRevision: fresh.HeadHash,
	}
	preflight, err := Preflight(binding)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	payload := captureResultJSON(t, binding, ManifestPaths(preflight.ChangedPathManifest), preflight.Subject.SubjectHash)
	if _, err := Capture(binding, payload); err != nil {
		t.Fatalf("Capture: %v", err)
	}

	result, err := EvaluateGate(GatePostApply, repo, "gate-fixture", GateOptions{})
	if err != nil {
		t.Fatalf("EvaluateGate: %v", err)
	}
	if result.Allowed {
		t.Fatal("expected denial with an unresolved candidate-causal finding")
	}
	if result.Findings == nil || result.Findings.Blocking != 1 || result.Findings.Resolved != 1 {
		t.Errorf("findings = %+v, want blocking=1 resolved=1", result.Findings)
	}
	if !strings.Contains(result.Reason, "R2-001") {
		t.Errorf("reason must name the blocking finding, got: %v", result.Reason)
	}
}

// ---------------------------------------------------------------------------
// Per-kind checks
// ---------------------------------------------------------------------------

func TestEvaluateGate_PreCommitStagedMismatch(t *testing.T) {
	repo, _, _ := gateFixture(t)

	// Modify a reviewed file and stage it: the staged tree no longer
	// reproduces the reviewed candidate tree.
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("changed\n"), 0644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	runGitInDir(t, repo, "add", "a.txt")

	result, err := EvaluateGate(GatePreCommit, repo, "gate-fixture", GateOptions{})
	if err != nil {
		t.Fatalf("EvaluateGate: %v", err)
	}
	if result.Allowed {
		t.Fatal("expected denial for a staged tree that does not reproduce the candidate")
	}
	if !strings.Contains(result.Reason, "staged tree") {
		t.Errorf("reason must name the staged tree mismatch, got: %v", result.Reason)
	}
}

func TestEvaluateGate_PreCommitStagedPathOutsideManifest(t *testing.T) {
	repo, _, _ := gateFixture(t)

	// Stage a file that was never part of the reviewed candidate.
	if err := os.WriteFile(filepath.Join(repo, "c.txt"), []byte("new\n"), 0644); err != nil {
		t.Fatalf("write c.txt: %v", err)
	}
	runGitInDir(t, repo, "add", "c.txt")

	result, err := EvaluateGate(GatePreCommit, repo, "gate-fixture", GateOptions{})
	if err != nil {
		t.Fatalf("EvaluateGate: %v", err)
	}
	if result.Allowed {
		t.Fatal("expected denial for a staged path outside the reviewed candidate")
	}
	if !strings.Contains(result.Reason, "outside the reviewed candidate") {
		t.Errorf("reason must name the out-of-scope staged path, got: %v", result.Reason)
	}
}

func TestEvaluateGate_PrePushUnreviewedCommits(t *testing.T) {
	repo, head, _ := gateFixture(t)

	// New commit on top of the reviewed candidate, not covered by any review.
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\nchanged\n"), 0644); err != nil {
		t.Fatalf("write base.txt: %v", err)
	}
	runGitInDir(t, repo, "add", ".")
	runGitInDir(t, repo, "commit", "-m", "unreviewed commit")

	result, err := EvaluateGate(GatePrePush, repo, "gate-fixture", GateOptions{})
	if err != nil {
		t.Fatalf("EvaluateGate: %v", err)
	}
	if result.Allowed {
		t.Fatal("expected denial for unreviewed commits in the publication range")
	}
	if !strings.Contains(result.Reason, "unreviewed commit") || !strings.Contains(result.Reason, head) {
		t.Errorf("reason must name the unreviewed commits and the reviewed head, got: %v", result.Reason)
	}
}

func TestEvaluateGate_PrePushReviewedCommitNotOnHeadLineage(t *testing.T) {
	repo, _, _ := gateFixture(t)

	// Rewrite history: the reviewed candidate disappears from HEAD's ancestry.
	runGitInDir(t, repo, "reset", "--hard", "HEAD~1")

	result, err := EvaluateGate(GatePrePush, repo, "gate-fixture", GateOptions{})
	if err != nil {
		t.Fatalf("EvaluateGate: %v", err)
	}
	if result.Allowed {
		t.Fatal("expected denial when the reviewed commit is not on the HEAD lineage")
	}
	if !strings.Contains(result.Reason, "not on the current HEAD lineage") {
		t.Errorf("reason must name the missing reviewed commit, got: %v", result.Reason)
	}
}

func TestEvaluateGate_PrePRBaseBoundaryOutsideManifest(t *testing.T) {
	repo, _, extraCommit := gateFixtureWithExtraCommit(t)

	// Use the extra commit as the PR base boundary: its tree contains c.txt,
	// so the boundary diff against the reviewed candidate touches a path
	// outside the reviewed manifest.
	result, err := EvaluateGate(GatePrePR, repo, "gate-fixture", GateOptions{
		BaseRef: extraCommit, // the unreviewed extra commit
	})
	if err != nil {
		t.Fatalf("EvaluateGate: %v", err)
	}
	if result.Allowed {
		t.Fatal("expected denial for a base boundary whose diff leaves the reviewed scope")
	}
	if !strings.Contains(result.Reason, "outside the reviewed candidate scope") {
		t.Errorf("reason must name the out-of-scope diff paths, got: %v", result.Reason)
	}
}

func TestEvaluateGate_PrePRCIAttestation(t *testing.T) {
	repo, _, _ := gateFixture(t)

	// Accepted best-effort: presence + parse of a signed JSON file.
	attest := filepath.Join(repo, "attestation.json")
	if err := os.WriteFile(attest, []byte(`{"signature":"MEUCIQD...","payload":"..."}`), 0644); err != nil {
		t.Fatalf("write attestation: %v", err)
	}
	result, err := EvaluateGate(GatePrePR, repo, "gate-fixture", GateOptions{PrePRCIAttestation: attest})
	if err != nil {
		t.Fatalf("EvaluateGate: %v", err)
	}
	if !result.Passed || !result.Allowed {
		t.Fatalf("expected pass with a parseable CI attestation, got reasons=%v", result.Reasons)
	}

	// Missing file fails.
	result, err = EvaluateGate(GatePrePR, repo, "gate-fixture", GateOptions{PrePRCIAttestation: filepath.Join(repo, "nope.json")})
	if err != nil {
		t.Fatalf("EvaluateGate: %v", err)
	}
	if result.Allowed {
		t.Fatal("expected denial for an unreadable CI attestation")
	}
	if !strings.Contains(result.Reason, "cannot be read") {
		t.Errorf("reason must name the unreadable attestation, got: %v", result.Reason)
	}

	// Malformed JSON fails.
	if err := os.WriteFile(attest, []byte("{not json"), 0644); err != nil {
		t.Fatalf("write attestation: %v", err)
	}
	result, err = EvaluateGate(GatePrePR, repo, "gate-fixture", GateOptions{PrePRCIAttestation: attest})
	if err != nil {
		t.Fatalf("EvaluateGate: %v", err)
	}
	if result.Allowed {
		t.Fatal("expected denial for a malformed CI attestation")
	}
	if !strings.Contains(result.Reason, "not valid signed JSON") {
		t.Errorf("reason must name the malformed attestation, got: %v", result.Reason)
	}
}

func TestEvaluateGate_ReleaseHeadDrifted(t *testing.T) {
	repo, _, _ := gateFixture(t)

	// Drift HEAD away from the reviewed candidate: release freshness fails.
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\nchanged\n"), 0644); err != nil {
		t.Fatalf("write base.txt: %v", err)
	}
	runGitInDir(t, repo, "add", ".")
	runGitInDir(t, repo, "commit", "-m", "post-review drift")

	result, err := EvaluateGate(GateRelease, repo, "gate-fixture", GateOptions{})
	if err != nil {
		t.Fatalf("EvaluateGate: %v", err)
	}
	if result.Allowed {
		t.Fatal("expected denial when HEAD no longer matches the reviewed candidate")
	}
	if !strings.Contains(result.Reason, "HEAD") {
		t.Errorf("reason must name the HEAD freshness mismatch, got: %v", result.Reason)
	}
}

// ---------------------------------------------------------------------------
// Disabled mode + dry run
// ---------------------------------------------------------------------------

func TestEvaluateGate_DisabledModeAllKinds(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	RDDDisable("", "", "global")

	for _, kind := range []GateKind{GatePostApply, GatePreCommit, GatePrePush, GatePrePR, GateRelease} {
		result, err := EvaluateGate(kind, "", "any-lineage", GateOptions{})
		if err != nil {
			t.Fatalf("EvaluateGate(%s): %v", kind, err)
		}
		if result.Passed || result.Allowed {
			t.Errorf("gate %s: disabled mode must not pass (passed=%t allowed=%t)", kind, result.Passed, result.Allowed)
		}
		if result.Delivery != DeliveryDisabledUnmanaged {
			t.Errorf("gate %s: delivery = %q, want %q", kind, result.Delivery, DeliveryDisabledUnmanaged)
		}
		if result.Gate != kind {
			t.Errorf("gate %s: result carries gate %s", kind, result.Gate)
		}
	}
}

func TestEvaluateGate_DryRunReportsButDoesNotFail(t *testing.T) {
	repo, _, _ := gateFixture(t)

	// Create a blocking condition (unreviewed commit) and dry-run.
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\nchanged\n"), 0644); err != nil {
		t.Fatalf("write base.txt: %v", err)
	}
	runGitInDir(t, repo, "add", ".")
	runGitInDir(t, repo, "commit", "-m", "unreviewed")

	result, err := EvaluateGate(GatePrePush, repo, "gate-fixture", GateOptions{DryRun: true})
	if err != nil {
		t.Fatalf("EvaluateGate: %v", err)
	}
	if !result.Passed {
		t.Error("dry-run must report pass (exit zero) even under denial")
	}
	if result.Allowed {
		t.Error("dry-run must not fabricate an authorization")
	}
	if len(result.Reasons) == 0 {
		t.Error("dry-run must report the blocking reasons")
	}
	if result.Delivery != "" {
		t.Errorf("denied managed evaluation must not name a delivery, got %q", result.Delivery)
	}
}
