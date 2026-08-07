package review

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/model"
)

func subject() model.ReviewSubject {
	return model.ReviewSubject{Repository: "https://example.com/acme/repo", CommitSHA: "a1b2c3d4"}
}

// riskInputFor builds a classifier input with the given paths and a total
// line count. The diff summary is omitted, so the trivial-inert binary rule
// does not apply.
func riskInputFor(paths []string, lines int) RiskInput {
	return RiskInput{Paths: paths, ChangedLines: lines}
}

var (
	lowInput    = riskInputFor([]string{"README.md", "docs/guide.md"}, 12)
	mediumInput = riskInputFor([]string{"cmd/main.go"}, 80)
	highInput   = riskInputFor([]string{"internal/auth/token.go"}, 20)
)

func TestEvaluateStartConsent_LowRiskSilent(t *testing.T) {
	// Low tier: silent, no consent — even undeclared and non-interactive,
	// even with an explicit relay or granted declaration, and even with a
	// declared lens plan (declared lenses never escalate a low-tier tier).
	for _, lenses := range [][]string{nil, {"readability"}} {
		for _, declared := range []string{"", "relay", "granted"} {
			decision, err := EvaluateStartConsent(subject(), "lineage-1", lowInput, lenses, declared, false)
			if err != nil {
				t.Fatalf("EvaluateStartConsent(low, lenses=%v, %q) error: %v", lenses, declared, err)
			}
			if decision.Decision != "proceed" {
				t.Errorf("Decision = %q, want proceed for low risk with %v/%q", decision.Decision, lenses, declared)
			}
			if decision.Envelope != nil {
				t.Errorf("low risk with %v/%q must not carry an envelope", lenses, declared)
			}
		}
	}
}

func TestEvaluateStartConsent_DeclineLowRiskErrors(t *testing.T) {
	_, err := EvaluateStartConsent(subject(), "lineage-1", lowInput, nil, "declined", false)
	if err == nil {
		t.Fatal("expected error for declined on low-risk candidate")
	}
	if !strings.Contains(err.Error(), "nothing to decline") {
		t.Errorf("error should say there is nothing to decline, got: %v", err)
	}
}

func TestEvaluateStartConsent_SensitivePathAsksConsentWithoutDeclaredLenses(t *testing.T) {
	// The classifier tier — not the declared lens count — drives consent: a
	// sensitive path with no declared lenses is high and still asks.
	decision, err := EvaluateStartConsent(subject(), "lineage-1", highInput, nil, "relay", false)
	if err != nil {
		t.Fatalf("EvaluateStartConsent(relay) error: %v", err)
	}
	if decision.Decision != "relay" {
		t.Fatalf("Decision = %q, want %q", decision.Decision, "relay")
	}
	if decision.Envelope == nil {
		t.Fatal("Envelope is nil for relay decision")
	}
	env := decision.Envelope
	if env.Candidate.Risk != RiskHigh {
		t.Errorf("Candidate.Risk = %q, want %q", env.Candidate.Risk, RiskHigh)
	}
	if !strings.Contains(strings.Join(env.RiskEvidence, " "), "sensitive path") {
		t.Errorf("RiskEvidence should name the sensitive path: %v", env.RiskEvidence)
	}
}

func TestEvaluateStartConsent_RelayEnvelopeShape(t *testing.T) {
	lenses := []string{"risk", "readability", "reliability", "resilience"}
	decision, err := EvaluateStartConsent(subject(), "lineage-1", highInput, lenses, "relay", false)
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
	if !strings.Contains(strings.Join(env.RiskEvidence, " "), "risk") {
		t.Errorf("RiskEvidence does not name the lens plan: %v", env.RiskEvidence)
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
	if env.Candidate.Risk != RiskHigh {
		t.Errorf("Candidate.Risk = %q, want %q", env.Candidate.Risk, RiskHigh)
	}
	if len(env.Candidate.Lenses) != 4 {
		t.Errorf("Candidate.Lenses = %v, want 4 lenses", env.Candidate.Lenses)
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

func TestEvaluateStartConsent_MediumTierRelay(t *testing.T) {
	// A mixed candidate with no sensitive path and no declared lenses is
	// medium: one consolidated review, consent required.
	decision, err := EvaluateStartConsent(subject(), "lineage-1", mediumInput, nil, "relay", false)
	if err != nil {
		t.Fatalf("EvaluateStartConsent(medium, relay) error: %v", err)
	}
	if decision.Decision != "relay" || decision.Envelope == nil {
		t.Fatalf("Decision = %q envelope=%v, want relay with envelope", decision.Decision, decision.Envelope)
	}
	if decision.Envelope.Candidate.Risk != RiskMedium {
		t.Errorf("Candidate.Risk = %q, want %q", decision.Envelope.Candidate.Risk, RiskMedium)
	}
}

func TestEvaluateStartConsent_RelayNonInteractive(t *testing.T) {
	// Explicit relay must work headless: that is the point of the mode.
	decision, err := EvaluateStartConsent(subject(), "lineage-1", highInput, nil, "relay", false)
	if err != nil {
		t.Fatalf("EvaluateStartConsent(relay, non-interactive) error: %v", err)
	}
	if decision.Decision != "relay" || decision.Envelope == nil {
		t.Fatalf("Decision = %q envelope=%v, want relay with envelope", decision.Decision, decision.Envelope)
	}
}

func TestEvaluateStartConsent_GrantedProceeds(t *testing.T) {
	decision, err := EvaluateStartConsent(subject(), "lineage-1", highInput, nil, "granted", false)
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
	decision, err := EvaluateStartConsent(subject(), "lineage-1", highInput, nil, "declined", false)
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

func TestEvaluateStartConsent_NoConsentMediumHighNonInteractiveErrors(t *testing.T) {
	for _, input := range []RiskInput{mediumInput, highInput} {
		_, err := EvaluateStartConsent(subject(), "lineage-1", input, nil, "", false)
		if err == nil {
			t.Fatal("expected error for undeclared consent on a medium/high candidate in non-interactive mode")
		}
		msg := err.Error()
		if !strings.Contains(msg, "--consent relay") {
			t.Errorf("error should explain the relay flow, got: %s", msg)
		}
	}
}

func TestEvaluateStartConsent_NoConsentInteractiveDefaultsToRelay(t *testing.T) {
	decision, err := EvaluateStartConsent(subject(), "lineage-1", highInput, nil, "", true)
	if err != nil {
		t.Fatalf("EvaluateStartConsent(undeclared, interactive) error: %v", err)
	}
	if decision.Decision != "relay" || decision.Envelope == nil {
		t.Fatalf("Decision = %q envelope=%v, want relay with envelope", decision.Decision, decision.Envelope)
	}
}

func TestEvaluateStartConsent_InvalidMode(t *testing.T) {
	_, err := EvaluateStartConsent(subject(), "lineage-1", highInput, nil, "maybe", false)
	if err == nil {
		t.Fatal("expected error for invalid consent mode")
	}
}
