package plugintest

import (
	"context"
	"testing"

	"github.com/biggz-ai/biggz/model"
)

// TestDummyLens_Analyze_HappyPath verifies that a valid subject produces a
// LensResult with findings.
func TestDummyLens_Analyze_HappyPath(t *testing.T) {
	lens := &DummyLens{}
	subject := model.ReviewSubject{
		Repository: "test/repo",
		CommitSHA:  "abc123",
	}

	result, err := lens.Analyze(context.Background(), subject)
	if err != nil {
		t.Fatalf("Analyze returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Analyze returned nil result")
	}
	if result.LensID != "dummy-lens" {
		t.Errorf("LensID = %q, want %q", result.LensID, "dummy-lens")
	}
	if len(result.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result.Findings))
	}
	if result.Findings[0].Severity != "info" {
		t.Errorf("Severity = %q, want %q", result.Findings[0].Severity, "info")
	}
	if result.Findings[0].Message != "Dummy analysis complete" {
		t.Errorf("Message = %q, want %q", result.Findings[0].Message, "Dummy analysis complete")
	}
}

// TestDummyLens_Analyze_InvalidSubject verifies that an empty subject returns an error.
func TestDummyLens_Analyze_InvalidSubject(t *testing.T) {
	lens := &DummyLens{}
	subject := model.ReviewSubject{} // empty — Repository == ""

	_, err := lens.Analyze(context.Background(), subject)
	if err == nil {
		t.Fatal("expected error for empty subject, got nil")
	}
}

// TestDummyLens_Policies verifies that Policies returns at least one policy.
func TestDummyLens_Policies(t *testing.T) {
	lens := &DummyLens{}
	policies := lens.Policies()
	if len(policies) == 0 {
		t.Fatal("expected at least one policy, got none")
	}
	if policies[0].Name != "minimum-evidence" {
		t.Errorf("Policy Name = %q, want %q", policies[0].Name, "minimum-evidence")
	}
}
