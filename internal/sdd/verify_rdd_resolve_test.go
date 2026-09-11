package sdd

// RDD verify gate candidate-lineage resolution tests (task 4.3, design D1):
// the verify preflight resolves the lineage that governs `HEAD^{commit}`
// before evaluating the gate — derived-first, then the read-only legacy scan
// — and fails closed naming the runnable producer invocation instead of the
// bare change name (bug #60).

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/review"
	"github.com/biggs-100/biggz-ai/model"
)

// rddResolveIsolateHome pins HOME so RDD resolution never reads the real
// developer machine state.
func rddResolveIsolateHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

// rddResolveRepo creates a git repository with a base and a candidate commit
// and returns the repository root plus the candidate (HEAD) full SHA.
func rddResolveRepo(t *testing.T) (repo, head string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not available: %v", err)
	}
	repo = t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=rdd", "GIT_AUTHOR_EMAIL=rdd@test",
			"GIT_COMMITTER_NAME=rdd", "GIT_COMMITTER_EMAIL=rdd@test")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	run("init", "-q")
	write("base.txt", "base\n")
	run("add", "-A")
	run("commit", "-q", "-m", "base", "--no-gpg-sign")
	write("candidate.txt", "candidate\n")
	run("add", "-A")
	run("commit", "-q", "-m", "candidate", "--no-gpg-sign")
	return repo, run("rev-parse", "HEAD")
}

// rddResolveStart begins a lineage whose frozen lens plan selects `risk`.
func rddResolveStart(t *testing.T, repo, head, lineageID string) *review.Store {
	t.Helper()
	store, err := review.Open(repo, lineageID)
	if err != nil {
		t.Fatalf("review.Open(%s): %v", lineageID, err)
	}
	plan := review.StartEventPayload{
		Schema:         review.ReviewStartEventSchema,
		Repository:     repo,
		CommitSHA:      head,
		SelectedLenses: []string{"risk"},
	}
	started := review.New(model.ReviewSubject{Repository: repo, CommitSHA: head})
	started.State.Role = model.RoleReviewer
	started.WithStore(store).FreezeStartPlan(plan)
	if err := started.Start(t.Context()); err != nil {
		t.Fatalf("review start %s: %v", lineageID, err)
	}
	return store
}

// rddResolveCapture captures one admitted zero-finding `risk` lens result at
// the lineage head, so the lineage can be finalized into a captured receipt.
func rddResolveCapture(t *testing.T, store *review.Store, repo, head, lineageID string) {
	t.Helper()
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	binding := review.CaptureBinding{
		Repo: repo, LineageID: lineageID, TargetIdentity: head,
		Lens: "risk", Order: 0, ExpectedRevision: chain.HeadHash,
	}
	preflight, err := review.Preflight(binding)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	payload, err := json.Marshal(map[string]any{
		"subject_hash": preflight.Subject.SubjectHash,
		"inspection": map[string]any{
			"status": "completed",
			"paths":  review.ManifestPaths(preflight.ChangedPathManifest),
		},
		"lens":     "risk",
		"findings": []any{},
		"evidence": []any{"inspected the complete frozen candidate manifest"},
	})
	if err != nil {
		t.Fatalf("marshal reviewer payload: %v", err)
	}
	outcome, err := review.Capture(binding, payload)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if outcome.Artifact.AdmissionDecision != review.AdmissionCompleted {
		t.Fatalf("admission = %q, want completed", outcome.Artifact.AdmissionDecision)
	}
}

// rddResolveFinalize finalizes a lineage into its terminal captured receipt.
func rddResolveFinalize(t *testing.T, repo, lineageID string) review.FinalizeOutcome {
	t.Helper()
	outcome, err := review.Finalize(repo, lineageID)
	if err != nil {
		t.Fatalf("Finalize(%s): %v", lineageID, err)
	}
	return outcome
}

func TestVerifyRDDResolveCandidateLineagePassesWithCapturedReceipt(t *testing.T) {
	rddResolveIsolateHome(t)
	repo, head := rddResolveRepo(t)
	change := "fix-rdd-receipt-collection"

	lineageID, err := review.DeriveLineageID(repo, head)
	if err != nil {
		t.Fatalf("DeriveLineageID: %v", err)
	}
	store := rddResolveStart(t, repo, head, lineageID)
	rddResolveCapture(t, store, repo, head, lineageID)
	receipt := rddResolveFinalize(t, repo, lineageID)

	if err := VerifyPreflightAt(repo, change); err != nil {
		t.Fatalf("verify preflight must pass with a captured receipt for HEAD (lineage %s, receipt %s), got: %v",
			lineageID, receipt.ReceiptHash, err)
	}
}

func TestVerifyRDDResolveLegacyUUIDLineage(t *testing.T) {
	rddResolveIsolateHome(t)
	repo, head := rddResolveRepo(t)
	change := "legacy-uuid-change"

	// A pre-derivation lineage: UUIDv7-style id, full subject SHA persisted.
	legacyID := "01932d0a-7f4e-7c31-9a6b-2f6f6b6c0007"
	store := rddResolveStart(t, repo, head, legacyID)
	rddResolveCapture(t, store, repo, head, legacyID)
	receipt := rddResolveFinalize(t, repo, legacyID)

	if err := VerifyPreflightAt(repo, change); err != nil {
		t.Fatalf("verify preflight must resolve the legacy lineage %s by its genesis subject (receipt %s), got: %v",
			legacyID, receipt.ReceiptHash, err)
	}
}

func TestVerifyRDDResolveMissingNamesProducerInvocation(t *testing.T) {
	rddResolveIsolateHome(t)
	repo, _ := rddResolveRepo(t)
	change := "fix-rdd-receipt-collection"

	err := VerifyPreflightAt(repo, change)
	if err == nil {
		t.Fatal("expected the missing receipt to block verify")
	}
	message := err.Error()
	if !strings.Contains(message, "rdd_receipt_missing") {
		t.Fatalf("missing receipt must stay rdd_receipt_missing, got: %s", message)
	}
	if !strings.Contains(message, "biggz review start --subject") {
		t.Fatalf("refusal must name the runnable producer invocation, got: %s", message)
	}
	subjectPath := filepath.Join(repo, "openspec", "changes", change, "review-subject.json")
	if !strings.Contains(message, subjectPath) {
		t.Fatalf("refusal must name the subject file %s, got: %s", subjectPath, message)
	}
	if strings.Contains(message, "--lineage") {
		t.Fatalf("refusal must not fall back to a bare change lineage name, got: %s", message)
	}
}

func TestVerifyRDDResolveRuntimeHarness(t *testing.T) {
	rddResolveIsolateHome(t)
	change := "fix-rdd-receipt-collection"

	// (d) a real change with a captured receipt no longer reports
	// rdd_receipt_missing: the gate resolves the derived lineage of HEAD.
	repo, head := rddResolveRepo(t)
	lineageID, err := review.DeriveLineageID(repo, head)
	if err != nil {
		t.Fatalf("DeriveLineageID: %v", err)
	}
	store := rddResolveStart(t, repo, head, lineageID)
	rddResolveCapture(t, store, repo, head, lineageID)
	receipt := rddResolveFinalize(t, repo, lineageID)
	gateErr := VerifyPreflightAt(repo, change)
	fmt.Printf("harness: repo=%s head=%s derived=%s receipt=%s preflight_err=%v\n",
		repo, head, lineageID, receipt.ReceiptHash, gateErr)
	if gateErr != nil {
		t.Fatalf("captured receipt must satisfy the gate, got: %v", gateErr)
	}

	// Missing receipt: fail closed naming the producer invocation.
	emptyRepo, emptyHead := rddResolveRepo(t)
	missingErr := VerifyPreflightAt(emptyRepo, change)
	fmt.Printf("harness: repo=%s head=%s missing_preflight_err=%v\n", emptyRepo, emptyHead, missingErr)
	if missingErr == nil || !strings.Contains(missingErr.Error(), "rdd_receipt_missing") {
		t.Fatalf("missing receipt must fail closed with rdd_receipt_missing, got: %v", missingErr)
	}
}
