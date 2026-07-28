package review

import (
	"context"
	"testing"

	"github.com/biggz-ai/biggz/model"
)

func makeCompletedReview(t *testing.T) *model.ReviewState {
	t.Helper()
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	r.Start(context.Background())
	r.Complete(context.Background())
	return r.State
}

func TestValidateCheck_PreCommit_Passes(t *testing.T) {
	state := makeCompletedReview(t)
	cfg := DefaultGateConfigs()[GatePreCommit]
	receipt := GenerateReceipt(state)

	result := ValidateCheck(GatePreCommit, state, cfg, receipt)
	if !result.Allowed {
		t.Errorf("pre-commit should pass: %s", result.Reason)
	}
}

func TestValidateCheck_PrePush_Passes(t *testing.T) {
	state := makeCompletedReview(t)
	cfg := DefaultGateConfigs()[GatePrePush]
	receipt := GenerateReceipt(state)

	result := ValidateCheck(GatePrePush, state, cfg, receipt)
	if !result.Allowed {
		t.Errorf("pre-push should pass: %s", result.Reason)
	}
}

func TestValidateCheck_NoReceipt(t *testing.T) {
	state := makeCompletedReview(t)
	cfg := DefaultGateConfigs()[GatePreCommit]

	result := ValidateCheck(GatePreCommit, state, cfg, nil)
	if result.Allowed {
		t.Fatal("expected failure without receipt")
	}
}

func TestValidateCheck_WrongStatus(t *testing.T) {
	state := model.NewReviewState(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	// State is Pending, not Completed
	cfg := DefaultGateConfigs()[GatePreCommit]
	receipt := GenerateReceipt(state)

	result := ValidateCheck(GatePreCommit, state, cfg, receipt)
	if result.Allowed {
		t.Fatal("expected failure for non-completed review")
	}
}

func TestValidateCheck_TamperedReceipt(t *testing.T) {
	state := makeCompletedReview(t)
	cfg := DefaultGateConfigs()[GatePreCommit]

	receipt := GenerateReceipt(state)
	receipt.MerkleRoot = "tampered"

	result := ValidateCheck(GatePreCommit, state, cfg, receipt)
	if result.Allowed {
		t.Fatal("expected failure for tampered receipt")
	}
}
