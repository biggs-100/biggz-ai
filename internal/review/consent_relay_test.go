package review

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/biggz-ai/biggz/model"
)

func subject() model.ReviewSubject {
	return model.ReviewSubject{Repository: "https://example.com/acme/repo", CommitSHA: "a1b2c3d4"}
}

func TestResolveReviewRisk(t *testing.T) {
	if got := ResolveReviewRisk(nil); got != RiskLow {
		t.Fatalf("ResolveReviewRisk(nil) = %q, want %q", got, RiskLow)
	}
	if got := ResolveReviewRisk([]string{}); got != RiskLow {
		t.Fatalf("ResolveReviewRisk(empty) = %q, want %q", got, RiskLow)
	}
	if got := ResolveReviewRisk([]string{"code_review"}); got != RiskMedium {
		t.Fatalf("ResolveReviewRisk(1 lens) = %q, want %q", got, RiskMedium)
	}
	if got := ResolveReviewRisk([]string{"code_review", "security"}); got != RiskMedium {
		t.Fatalf("ResolveReviewRisk(2 lenses) = %q, want %q", got, RiskMedium)
	}
}

func TestEvaluateStartConsent_RelayEnvelopeShape(t *testing.T) {
	decision, err := EvaluateStartConsent(subject(), "lineage-1", []string{"code_review", "security"}, "relay", false)
	if err != nil {
		t.Fatalf("EvaluateStartConsent(relay) error: %v", err)
	}
	if decision.Decision != "relay" {
		t.Fatalf("Decision = %q, want %q", decision.Decision, "relay")
	}
	if decision.Envelope == nil {
		t.Fatal("Envelope is nil for relay decision")
	}

	payload, err := json.Marshal(decision.Envelope)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	for _, key := range []string{"schema", "headline", "reason", "risk_evidence", "candidate", "choices", "off_path_note"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("envelope missing key %q", key)
		}
	}

	env := decision.Envelope
	if env.Schema != ConsentModeSchema {
		t.Errorf("Schema = %q, want %q", env.Schema, ConsentModeSchema)
	}
	if env.Headline == "" {
		t.Error("Headline is empty")
	}
	if env.Reason == "" {
		t.Error("Reason is empty")
	}
	if len(env.RiskEvidence) == 0 {
		t.Error("RiskEvidence is empty")
	}
	if !strings.Contains(strings.Join(env.RiskEvidence, " "), "code_review") {
		t.Errorf("RiskEvidence does not name declared lenses: %v", env.RiskEvidence)
	}
	if env.Candidate.Repository != subject().Repository {
		t.Errorf("Candidate.Repository = %q", env.Candidate.Repository)
	}
	if env.Candidate.CommitSHA != subject().CommitSHA {
		t.Errorf("Candidate.CommitSHA = %q", env.Candidate.CommitSHA)
	}
	if env.Candidate.Lineage != "lineage-1" {
		t.Errorf("Candidate.Lineage = %q, want lineage-1", env.Candidate.Lineage)
	}
	if env.Candidate.Risk != RiskMedium {
		t.Errorf("Candidate.Risk = %q, want %q", env.Candidate.Risk, RiskMedium)
	}
	if len(env.Candidate.Lenses) != 2 {
		t.Errorf("Candidate.Lenses = %v, want 2 lenses", env.Candidate.Lenses)
	}
	if env.OffPathNote == "" {
		t.Error("OffPathNote is empty")
	}

	if len(env.Choices) != 2 {
		t.Fatalf("Choices = %d entries, want 2", len(env.Choices))
	}
	ids := map[string]ConsentChoice{}
	for _, choice := range env.Choices {
		ids[choice.ID] = choice
	}
	for _, id := range []string{"granted", "declined"} {
		choice, ok := ids[id]
		if !ok {
			t.Errorf("missing choice %q", id)
			continue
		}
		if choice.Label == "" {
			t.Errorf("choice %q has empty label", id)
		}
		if choice.Effect == "" {
			t.Errorf("choice %q has empty effect", id)
		}
	}
}

func TestEvaluateStartConsent_RelayNonInteractive(t *testing.T) {
	// Explicit relay must work headless: that is the point of the mode.
	decision, err := EvaluateStartConsent(subject(), "lineage-1", []string{"code_review"}, "relay", false)
	if err != nil {
		t.Fatalf("EvaluateStartConsent(relay, non-interactive) error: %v", err)
	}
	if decision.Decision != "relay" || decision.Envelope == nil {
		t.Fatalf("Decision = %q envelope=%v, want relay with envelope", decision.Decision, decision.Envelope)
	}
}

func TestEvaluateStartConsent_GrantedProceeds(t *testing.T) {
	decision, err := EvaluateStartConsent(subject(), "lineage-1", []string{"code_review"}, "granted", false)
	if err != nil {
		t.Fatalf("EvaluateStartConsent(granted) error: %v", err)
	}
	if decision.Decision != "proceed" {
		t.Fatalf("Decision = %q, want %q", decision.Decision, "proceed")
	}
	if decision.Envelope != nil {
		t.Error("granted must not carry an envelope")
	}
}

func TestEvaluateStartConsent_DeclinedPersistsNothing(t *testing.T) {
	decision, err := EvaluateStartConsent(subject(), "lineage-1", []string{"code_review"}, "declined", false)
	if err != nil {
		t.Fatalf("EvaluateStartConsent(declined) error: %v", err)
	}
	if decision.Decision != "declined" {
		t.Fatalf("Decision = %q, want %q", decision.Decision, "declined")
	}
	if decision.Envelope != nil {
		t.Error("declined must not carry an envelope")
	}
	if decision.Message == "" {
		t.Error("declined must carry a scoped-decline message")
	}
}

func TestEvaluateStartConsent_NoConsentWithLensesNonInteractiveErrors(t *testing.T) {
	_, err := EvaluateStartConsent(subject(), "lineage-1", []string{"code_review"}, "", false)
	if err == nil {
		t.Fatal("expected error for undeclared consent with lenses in non-interactive mode")
	}
	msg := err.Error()
	if !strings.Contains(msg, "--consent relay") {
		t.Errorf("error should explain the relay flow, got: %s", msg)
	}
}

func TestEvaluateStartConsent_NoConsentInteractiveDefaultsToRelay(t *testing.T) {
	decision, err := EvaluateStartConsent(subject(), "lineage-1", []string{"code_review"}, "", true)
	if err != nil {
		t.Fatalf("EvaluateStartConsent(undeclared, interactive) error: %v", err)
	}
	if decision.Decision != "relay" || decision.Envelope == nil {
		t.Fatalf("Decision = %q envelope=%v, want relay with envelope", decision.Decision, decision.Envelope)
	}
}

func TestEvaluateStartConsent_LowRiskSilent(t *testing.T) {
	// Zero declared lenses: silent, no consent — even undeclared and
	// non-interactive, and even with an explicit relay declaration.
	for _, declared := range []string{"", "relay", "granted"} {
		decision, err := EvaluateStartConsent(subject(), "lineage-1", nil, declared, false)
		if err != nil {
			t.Fatalf("EvaluateStartConsent(low risk, %q) error: %v", declared, err)
		}
		if decision.Decision != "proceed" {
			t.Errorf("Decision = %q, want proceed for low risk with %q", decision.Decision, declared)
		}
		if decision.Envelope != nil {
			t.Errorf("low risk with %q must not carry an envelope", declared)
		}
	}
}

func TestEvaluateStartConsent_DeclineLowRiskErrors(t *testing.T) {
	_, err := EvaluateStartConsent(subject(), "lineage-1", nil, "declined", false)
	if err == nil {
		t.Fatal("expected error for declined on low-risk candidate")
	}
	if !strings.Contains(err.Error(), "nothing to decline") {
		t.Errorf("error should say there is nothing to decline, got: %v", err)
	}
}

func TestEvaluateStartConsent_InvalidMode(t *testing.T) {
	_, err := EvaluateStartConsent(subject(), "lineage-1", []string{"code_review"}, "maybe", false)
	if err == nil {
		t.Fatal("expected error for invalid consent mode")
	}
}
