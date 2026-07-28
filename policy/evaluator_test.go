package policy

import (
	"context"
	"testing"

	"github.com/biggz-ai/biggz/model"
)

// minimumEvidenceEvaluator checks that at least one evidence entry exists.
// This is a copy of the inline evaluator from cmd/biggz/main.go, reproduced
// here so the policy package tests can exercise a concrete Evaluator.
type minimumEvidenceEvaluator struct{}

func (e *minimumEvidenceEvaluator) Name() string { return "minimum-evidence" }

func (e *minimumEvidenceEvaluator) Evaluate(ctx context.Context, state *model.ReviewState) (*model.PolicyVerdict, error) {
	passed := len(state.Evidence) > 0
	reason := "At least one evidence entry exists"
	severity := "info"
	if !passed {
		reason = "No evidence entries found"
		severity = "error"
	}
	return &model.PolicyVerdict{
		Policy:   e.Name(),
		Passed:   passed,
		Reason:   reason,
		Severity: severity,
	}, nil
}

// TestMinimumEvidenceEvaluator_Passing verifies that a ReviewState with
// evidence entries produces a passing verdict.
func TestMinimumEvidenceEvaluator_Passing(t *testing.T) {
	eval := &minimumEvidenceEvaluator{}
	state := model.NewReviewState(model.ReviewSubject{
		Repository: "test/repo",
		CommitSHA:  "abc123",
	})
	// Append evidence so the policy passes.
	state.Evidence = model.AppendEvidence(state.Evidence, "test", `{"msg":"hello"}`)

	verdict, err := eval.Evaluate(context.Background(), state)
	if err != nil {
		t.Fatalf("Evaluate returned unexpected error: %v", err)
	}
	if verdict == nil {
		t.Fatal("Evaluate returned nil verdict")
	}
	if !verdict.Passed {
		t.Errorf("expected Passed=true, got Passed=%v", verdict.Passed)
	}
	if verdict.Policy != "minimum-evidence" {
		t.Errorf("Policy = %q, want %q", verdict.Policy, "minimum-evidence")
	}
}

// TestMinimumEvidenceEvaluator_Failing verifies that a ReviewState with empty
// evidence produces a failing verdict with severity populated.
func TestMinimumEvidenceEvaluator_Failing(t *testing.T) {
	eval := &minimumEvidenceEvaluator{}
	state := model.NewReviewState(model.ReviewSubject{
		Repository: "test/repo",
		CommitSHA:  "abc123",
	})
	// No evidence appended — policy should fail.

	verdict, err := eval.Evaluate(context.Background(), state)
	if err != nil {
		t.Fatalf("Evaluate returned unexpected error: %v", err)
	}
	if verdict == nil {
		t.Fatal("Evaluate returned nil verdict")
	}
	if verdict.Passed {
		t.Errorf("expected Passed=false, got Passed=%v", verdict.Passed)
	}
	if verdict.Severity == "" {
		t.Errorf("expected Severity to be populated, got empty string")
	}
	if verdict.Severity != "error" {
		t.Errorf("Severity = %q, want %q", verdict.Severity, "error")
	}
	if verdict.Reason == "" {
		t.Errorf("expected Reason to be populated, got empty string")
	}
}
