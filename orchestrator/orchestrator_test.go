package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/biggz-ai/biggz/model"
	"github.com/biggz-ai/biggz/registry"
)

// errStageFailure is a sentinel error used by the failing stage test.
var errStageFailure = errors.New("stage failed intentionally")

// failingStage is a pipeline Stage that always fails on Execute.
type failingStage struct{}

func (s *failingStage) Name() string { return "failing-stage" }

func (s *failingStage) Execute(ctx context.Context, state *model.ReviewState) error {
	return errStageFailure
}

func (s *failingStage) Rollback(ctx context.Context, state *model.ReviewState) error {
	return nil
}

// TestOrchestrator_PipelineFailure verifies that when a stage fails during
// orchestration, the returned ReviewState has Status=Failed and the error
// is non-nil.
func TestOrchestrator_PipelineFailure(t *testing.T) {
	reg := registry.New()
	stage := &failingStage{}
	orch := New(reg, stage)

	subject := model.ReviewSubject{
		Repository: "test/repo",
		CommitSHA:  "abc123",
	}

	state, err := orch.Execute(context.Background(), subject)
	if err == nil {
		t.Fatal("expected non-nil error from orchestrator, got nil")
	}
	if state == nil {
		t.Fatal("expected non-nil ReviewState even on failure, got nil")
	}
	if state.Status != model.StatusFailed {
		t.Errorf("expected Status=%q, got %q", model.StatusFailed, state.Status)
	}
}
