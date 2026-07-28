package review

import (
	"context"
	"fmt"
	"testing"

	"github.com/biggz-ai/biggz/model"
	"github.com/biggz-ai/biggz/pipeline"
)

func TestNew(t *testing.T) {
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	if r.State == nil {
		t.Fatal("New() returned nil state")
	}
	if r.State.Status != model.StatusPending {
		t.Errorf("expected Pending, got %s", r.State.Status)
	}
	if r.State.Subject.Repository != "test/repo" {
		t.Errorf("expected repo test/repo, got %s", r.State.Subject.Repository)
	}
}

func TestStartAndComplete(t *testing.T) {
	ctx := context.Background()
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})

	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start() unexpected error: %v", err)
	}
	if r.State.Status != model.StatusInProgress {
		t.Errorf("expected InProgress, got %s", r.State.Status)
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

	r.Start(ctx)
	r.Fail(ctx, nil)

	if r.State.Status != model.StatusFailed {
		t.Errorf("expected Failed, got %s", r.State.Status)
	}
}

func TestRunPipeline_Success(t *testing.T) {
	ctx := context.Background()
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})

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

	// Complete the review first
	r.Start(ctx)
	r.Complete(ctx)

	// Now add a correction
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

	// Should be back InProgress
	if r.State.Status != model.StatusInProgress {
		t.Errorf("expected InProgress after correction, got %s", r.State.Status)
	}

	// Evidence should have the correction entry
	if len(r.State.Evidence) < 1 {
		t.Fatal("expected at least 1 evidence entry after correction")
	}
	last := r.State.Evidence[len(r.State.Evidence)-1]
	if last.Kind != "correction" {
		t.Errorf("expected correction evidence kind, got %s", last.Kind)
	}
}

func TestAddCorrection_NotCompleted(t *testing.T) {
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	// Don't complete — should fail
	err := r.AddCorrection(Correction{ID: "corr-1", Reason: "test"})
	if err == nil {
		t.Fatal("expected error adding correction to non-completed review")
	}
}

func TestReceipt(t *testing.T) {
	ctx := context.Background()
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	r.Start(ctx)
	r.Complete(ctx)

	receipt := r.Receipt()
	if receipt.ReviewID != r.State.ID {
		t.Errorf("receipt ReviewID mismatch: %s vs %s", receipt.ReviewID, r.State.ID)
	}
	if receipt.MerkleRoot != r.State.MerkleRoot {
		t.Errorf("receipt MerkleRoot mismatch")
	}
}

// --- test helpers ---

type noopStage struct{}

func (s *noopStage) Name() string                          { return "noop" }
func (s *noopStage) Execute(ctx context.Context, state *model.ReviewState) error { return nil }
func (s *noopStage) Rollback(ctx context.Context, state *model.ReviewState) error { return nil }

type failStage struct{}

func (s *failStage) Name() string { return "fail" }
func (s *failStage) Execute(ctx context.Context, state *model.ReviewState) error {
	return fmt.Errorf("stage failed intentionally")
}
func (s *failStage) Rollback(ctx context.Context, state *model.ReviewState) error { return nil }
