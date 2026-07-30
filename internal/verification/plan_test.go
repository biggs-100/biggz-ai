package verification

import (
	"testing"
)

func TestNewPlan(t *testing.T) {
	subject := NewVerificationSubjectFromSnapshot("current-changes", "workspace", "abc123", "def456", []string{"main.go", "test.go"})

	obligations := []VerificationObligation{
		{ID: "lens/risk", Contract: "biggz.functional-proof/v1", Cost: CostQuick, ReadOnly: true, Mandatory: true},
		{ID: "lens/reliability", Contract: "biggz.functional-proof/v1", Cost: CostLong, ReadOnly: true, Mandatory: true},
	}

	plan := NewPlan(subject, obligations)

	if plan.Schema != PlanSchema {
		t.Errorf("expected PlanSchema, got %s", plan.Schema)
	}
	if plan.AuthorityRef == "" {
		t.Error("expected non-empty AuthorityRef")
	}
	if plan.Effects == nil {
		t.Fatal("expected Effects")
	}
	if !plan.Effects.Applicable {
		t.Error("expected applicable")
	}
	if plan.Effects.AggregateCost != CostLong {
		t.Errorf("expected CostLong, got %s", plan.Effects.AggregateCost)
	}

	// Validate
	if err := plan.Validate(); err != nil {
		t.Errorf("Validate() error: %v", err)
	}
}

func TestPlan_ContentAddressed(t *testing.T) {
	s1 := NewVerificationSubjectFromSnapshot("current-changes", "workspace", "base1", "cand1", []string{"a.go"})
	s2 := NewVerificationSubjectFromSnapshot("current-changes", "workspace", "base1", "cand1", []string{"a.go"})

	ops := []VerificationObligation{
		{ID: "lens/risk", Cost: CostQuick, ReadOnly: true},
	}

	p1 := NewPlan(s1, ops)
	p2 := NewPlan(s2, ops)

	if p1.AuthorityRef != p2.AuthorityRef {
		t.Error("same inputs should produce same AuthorityRef")
	}
}

func TestPlan_Gate_AutoRun(t *testing.T) {
	subject := NewVerificationSubjectFromSnapshot("current-changes", "workspace", "a", "b", nil)
	obligations := []VerificationObligation{
		{ID: "lens/risk", Cost: CostQuick, ReadOnly: true},
	}
	plan := NewPlan(subject, obligations)
	gate := plan.ResolveGate()

	if gate.Decision != DecisionAutoRun {
		t.Errorf("expected AutoRun, got %s", gate.Decision)
	}
}

func TestPlan_Gate_NeedsConsent(t *testing.T) {
	subject := NewVerificationSubjectFromSnapshot("current-changes", "workspace", "a", "b", nil)
	obligations := []VerificationObligation{
		{ID: "lens/risk", Cost: CostVeryLong, ReadOnly: true},
	}
	plan := NewPlan(subject, obligations)
	gate := plan.ResolveGate()

	if gate.Decision != DecisionNeedsConsent {
		t.Errorf("expected NeedsConsent, got %s", gate.Decision)
	}
	if !gate.RequiresConsent {
		t.Error("expected RequiresConsent")
	}
}

func TestPlan_Gate_NeedsAuth(t *testing.T) {
	subject := NewVerificationSubjectFromSnapshot("current-changes", "workspace", "a", "b", nil)
	obligations := []VerificationObligation{
		{ID: "auto-formatter", Cost: CostQuick, ReadOnly: false}, // destructive
	}
	plan := NewPlan(subject, obligations)
	gate := plan.ResolveGate()

	if gate.Decision != DecisionNeedsAuth {
		t.Errorf("expected NeedsAuth, got %s", gate.Decision)
	}
	if !gate.RequiresAuth {
		t.Error("expected RequiresAuth")
	}
}

func TestPlan_Gate_NotApplicable(t *testing.T) {
	subject := NewVerificationSubjectFromSnapshot("current-changes", "workspace", "a", "b", nil)
	plan := NewPlan(subject, nil) // no obligations
	gate := plan.ResolveGate()

	if gate.Decision != DecisionNotApplicable {
		t.Errorf("expected NotApplicable, got %s", gate.Decision)
	}
}

func TestConsent(t *testing.T) {
	subject := NewVerificationSubjectFromSnapshot("current-changes", "workspace", "a", "b", nil)
	plan := NewPlan(subject, []VerificationObligation{
		{ID: "lens/risk", Cost: CostQuick, ReadOnly: true},
	})

	consent := NewConsent(plan.AuthorityRef, plan.Effects.Digest, "test-user")
	if err := consent.Validate(); err != nil {
		t.Errorf("consent Validate() error: %v", err)
	}
}

func TestConvergence(t *testing.T) {
	s1 := NewVerificationSubjectFromSnapshot("current-changes", "workspace", "a", "b", []string{"main.go"})
	s2 := NewVerificationSubjectFromSnapshot("current-changes", "workspace", "a", "b", []string{"main.go"})

	result := CheckConvergence(s1, s2)
	if !result.Passed {
		t.Error("expected convergence to pass for identical subjects")
	}

	s3 := NewVerificationSubjectFromSnapshot("current-changes", "workspace", "a", "c", []string{"main.go"})
	result = CheckConvergence(s1, s3)
	if result.Passed {
		t.Error("expected convergence to fail for different subjects")
	}
}

func TestPlanStore(t *testing.T) {
	store := NewPlanStore()
	subject := NewVerificationSubjectFromSnapshot("current-changes", "workspace", "a", "b", nil)
	plan := NewPlan(subject, []VerificationObligation{
		{ID: "lens/risk", Cost: CostQuick, ReadOnly: true},
	})

	if err := store.Publish(plan); err != nil {
		t.Errorf("Publish() error: %v", err)
	}

	got, ok := store.Get(plan.AuthorityRef)
	if !ok {
		t.Fatal("expected to find plan")
	}
	if got.AuthorityRef != plan.AuthorityRef {
		t.Error("AuthorityRef mismatch")
	}

	// Idempotent
	if err := store.Publish(plan); err != nil {
		t.Errorf("second Publish() error: %v", err)
	}
}
