package review

import (
	"context"
	"fmt"
	"testing"

	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/pipeline"
)

func TestNew(t *testing.T) {
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	if r.State == nil {
		t.Fatal("New() returned nil state")
	}
	if r.State.Status != model.StatusUnreviewed {
		t.Errorf("expected Unreviewed, got %s", r.State.Status)
	}
	if r.State.Subject.Repository != "test/repo" {
		t.Errorf("expected repo test/repo, got %s", r.State.Subject.Repository)
	}
}

func TestStartAndComplete(t *testing.T) {
	ctx := context.Background()
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	// Lead can perform all transitions including Complete.
	r.State.Role = model.RoleLead

	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start() unexpected error: %v", err)
	}
	if r.State.Status != model.StatusInReview {
		t.Errorf("expected InReview, got %s", r.State.Status)
	}

	if err := r.Complete(ctx); err != nil {
		t.Fatalf("Complete() unexpected error: %v", err)
	}
	if r.State.Status != model.StatusCompleted {
		t.Errorf("expected Completed, got %s", r.State.Status)
	}
}

func TestFail(t *testing.T) {
	ctx := context.Background()
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	r.State.Role = model.RoleReviewer

	r.Start(ctx)
	r.Fail(ctx, nil)

	if r.State.Status != model.StatusFailed {
		t.Errorf("expected Failed, got %s", r.State.Status)
	}
}

func TestRunPipeline_Success(t *testing.T) {
	ctx := context.Background()
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	r.State.Role = model.RoleLead

	p := pipeline.New(&noopStage{})
	if err := r.RunPipeline(ctx, p); err != nil {
		t.Fatalf("RunPipeline() unexpected error: %v", err)
	}
	if r.State.Status != model.StatusCompleted {
		t.Errorf("expected Completed, got %s", r.State.Status)
	}
}

func TestRunPipeline_Fail(t *testing.T) {
	ctx := context.Background()
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	r.State.Role = model.RoleReviewer

	p := pipeline.New(&failStage{})
	err := r.RunPipeline(ctx, p)
	if err == nil {
		t.Fatal("expected error from RunPipeline(), got nil")
	}
	if r.State.Status != model.StatusFailed {
		t.Errorf("expected Failed, got %s", r.State.Status)
	}
}

func TestAddCorrection(t *testing.T) {
	ctx := context.Background()
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	r.State.Role = model.RoleReviewer

	// Start the review (InReview)
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Now add a correction (InReview → NeedsChanges)
	corr := Correction{
		ID:           "corr-1",
		Files:        []string{"main.go"},
		LinesChanged: 5,
		Reason:       "fixed vulnerability",
		BeforeHash:   "abc",
		AfterHash:    "def",
	}
	if err := r.AddCorrection(corr); err != nil {
		t.Fatalf("AddCorrection() unexpected error: %v", err)
	}

	// Should be NeedsChanges (new FSM path)
	if r.State.Status != model.StatusNeedsChanges {
		t.Errorf("expected NeedsChanges after correction, got %s", r.State.Status)
	}

	// Evidence should have the correction entry
	if len(r.State.Evidence) < 1 {
		t.Fatal("expected at least 1 evidence entry after correction")
	}
	last := r.State.Evidence[len(r.State.Evidence)-1]
	if last.Kind != "correction" {
		t.Errorf("expected correction evidence kind, got %s", last.Kind)
	}

	// Budget should be incremented
	if r.State.BudgetCounters.FixRounds != 1 {
		t.Errorf("expected FixRounds=1, got %d", r.State.BudgetCounters.FixRounds)
	}
}

func TestAddCorrection_NotInReview(t *testing.T) {
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	// State is Unreviewed, not InReview
	err := r.AddCorrection(Correction{ID: "corr-1", Reason: "test"})
	if err == nil {
		t.Fatal("expected error adding correction to non-in-review review")
	}
}

func TestAddCorrection_BudgetExhausted(t *testing.T) {
	ctx := context.Background()
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	r.State.Role = model.RoleReviewer
	r.Start(ctx)

	// Exhaust the fix rounds budget.
	r.State.BudgetCounters = model.BudgetCounters{FixRounds: model.MaxFixRounds}

	err := r.AddCorrection(Correction{ID: "corr-1", Reason: "over budget"})
	if err == nil {
		t.Fatal("expected error when correction budget exhausted")
	}
}

func TestReceipt(t *testing.T) {
	ctx := context.Background()
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	r.State.Role = model.RoleLead
	r.Start(ctx)
	r.Complete(ctx)

	receipt := r.Receipt()
	if receipt.LineageID != r.State.LineageID {
		t.Errorf("receipt LineageID mismatch: %s vs %s", receipt.LineageID, r.State.LineageID)
	}
	if receipt.HeadRevision != r.State.MerkleRoot {
		t.Errorf("receipt HeadRevision mismatch: %s vs %s", receipt.HeadRevision, r.State.MerkleRoot)
	}
}

func TestStartWithStore(t *testing.T) {
	dir := t.TempDir()
	s := OpenWithDir(dir, "review-store")

	ctx := context.Background()
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	r.State.Role = model.RoleLead
	r.WithStore(s)

	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start() with store: %v", err)
	}

	// Store should now have events.
	chain, err := s.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	if chain.Count < 1 {
		t.Error("expected at least 1 event after Start with store")
	}

	// Complete and verify chain persists.
	if err := r.Complete(ctx); err != nil {
		t.Fatalf("Complete() with store: %v", err)
	}

	chain, err = s.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain after Complete: %v", err)
	}
	if chain.Count < 2 {
		t.Errorf("expected at least 2 events, got %d", chain.Count)
	}

	// Verify chain integrity.
	verdict := s.Validate()
	if !verdict.Valid {
		t.Errorf("chain integrity: %s", verdict.Reason)
	}
}

// --- test helpers ---

type noopStage struct{}

func (s *noopStage) Name() string { return "noop" }
func (s *noopStage) Execute(ctx context.Context, state *model.ReviewState) error { return nil }
func (s *noopStage) Rollback(ctx context.Context, state *model.ReviewState) error { return nil }

type failStage struct{}

func (s *failStage) Name() string { return "fail" }
func (s *failStage) Execute(ctx context.Context, state *model.ReviewState) error {
	return fmt.Errorf("stage failed intentionally")
}
func (s *failStage) Rollback(ctx context.Context, state *model.ReviewState) error { return nil }
