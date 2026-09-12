package sdd

import (
	"strings"
	"testing"
)

// retitleMainSpec is a main spec whose headings exercise exact, REQ-id-prefix
// and normalized-name matching for the MODIFIED-not-found hint.
const retitleMainSpec = `# Payment Spec

## Purpose

Payment capabilities.

### Requirement: REQ-1 — Engram Import Dispatch (--from-engram)

The system SHALL import Engram chunks.

#### Scenario: Import runs

- **WHEN** an import starts
- **THEN** chunks land in the store

### Requirement: Payment Flow

The system SHALL process payments.

#### Scenario: Pay

- **WHEN** the user pays
- **THEN** the payment succeeds
`

func modifiedDeltas(name, body string) []RequirementDelta {
	return []RequirementDelta{{Kind: DeltaModified, Name: name, Body: body}}
}

func applyModified(t *testing.T, main, name, body string) (string, error) {
	t.Helper()
	return ApplyDeltas(main, modifiedDeltas(name, body))
}

func TestApplyModifiedDelta_UnknownNameCarriesRetitleHint(t *testing.T) {
	const name = "Totally Unrelated Capability"
	_, err := applyModified(t, retitleMainSpec, name, "### Requirement: "+name+"\n\nBody.")
	if err == nil {
		t.Fatal("ApplyDeltas succeeded for an unmatched MODIFIED name, want error")
	}
	msg := err.Error()
	for _, want := range []string{
		"not found in main spec",
		"match the main spec heading verbatim",
		"REMOVED + ADDED",
		"exact existing heading",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing hint %q", msg, want)
		}
	}
}

func TestApplyModifiedDelta_NoCandidateOmitsSuggestion(t *testing.T) {
	// Substring of an existing heading, but not equal by id prefix or
	// normalized name: no fuzzy matching, so no suggestion.
	const name = "Engram Import Dispatch"
	_, err := applyModified(t, retitleMainSpec, name, "### Requirement: "+name+"\n\nBody.")
	if err == nil {
		t.Fatal("ApplyDeltas succeeded for an unmatched MODIFIED name, want error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "match the main spec heading verbatim") {
		t.Errorf("error %q must still carry the hint", msg)
	}
	if strings.Contains(msg, "similar requirement heading") {
		t.Errorf("error %q must not suggest a candidate when none matches", msg)
	}
}

func TestApplyModifiedDelta_IDPrefixCandidateSuggested(t *testing.T) {
	const name = "REQ-1 — Import Dispatch (retitled)"
	_, err := applyModified(t, retitleMainSpec, name, "### Requirement: "+name+"\n\nBody.")
	if err == nil {
		t.Fatal("ApplyDeltas succeeded for a retitled heading, want error")
	}
	msg := err.Error()
	if !strings.Contains(msg, `"REQ-1 — Engram Import Dispatch (--from-engram)"`) {
		t.Errorf("error %q must name the exact existing heading", msg)
	}
	if !strings.Contains(msg, "similar requirement heading") {
		t.Errorf("error %q must frame the candidate as a suggestion", msg)
	}
	if !strings.Contains(msg, "REMOVED + ADDED") {
		t.Errorf("error %q must keep the retitle hint", msg)
	}
}

func TestApplyModifiedDelta_NormalizedNameCandidateSuggested(t *testing.T) {
	const name = "payment   FLOW"
	_, err := applyModified(t, retitleMainSpec, name, "### Requirement: payment FLOW\n\nBody.")
	if err == nil {
		t.Fatal("ApplyDeltas succeeded for a case/space variant heading, want error")
	}
	msg := err.Error()
	if !strings.Contains(msg, `"Payment Flow"`) {
		t.Errorf("error %q must suggest the differently-cased existing heading", msg)
	}
}

func TestApplyModifiedDelta_FirstCandidateInSpecOrder(t *testing.T) {
	const main = `# Spec

### Requirement: REQ-9 — First Candidate

One.

### Requirement: REQ-9 — Second Candidate

Two.
`
	_, err := applyModified(t, main, "REQ-9 — Renamed", "### Requirement: REQ-9 — Renamed\n\nNew.")
	if err == nil {
		t.Fatal("ApplyDeltas succeeded for a retitled heading, want error")
	}
	msg := err.Error()
	if !strings.Contains(msg, `"REQ-9 — First Candidate"`) {
		t.Errorf("error %q must name the first candidate in spec order", msg)
	}
	if strings.Contains(msg, `"REQ-9 — Second Candidate"`) {
		t.Errorf("error %q must name only the first candidate", msg)
	}
}

func TestApplyModifiedDelta_CandidateSuggestionIsBounded(t *testing.T) {
	longName := "REQ-7 — " + strings.Repeat("Very Long Heading ", 30)
	main := "# Spec\n\n### Requirement: " + longName + "\n\nBody.\n"
	_, err := applyModified(t, main, "REQ-7 — Retitled", "### Requirement: REQ-7 — Retitled\n\nNew.\n")
	if err == nil {
		t.Fatal("ApplyDeltas succeeded for a retitled heading, want error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "…") {
		t.Errorf("error should truncate a long candidate heading, got %q", msg)
	}
	if strings.Contains(msg, longName) {
		t.Errorf("error must not quote the full long heading, got %q", msg)
	}
}

func TestApplyModifiedDelta_ExactMatchingHeadingStillApplies(t *testing.T) {
	updated := "### Requirement: Payment Flow\n\nThe system SHALL process payments idempotently.\n\n#### Scenario: Pay\n\n- **WHEN** the user pays\n- **THEN** the payment is idempotent"
	out, err := applyModified(t, retitleMainSpec, "Payment Flow", updated)
	if err != nil {
		t.Fatalf("exact MODIFIED name must still apply: %v", err)
	}
	if !strings.Contains(out, "idempotently") {
		t.Errorf("updated body missing from result:\n%s", out)
	}
	if strings.Contains(out, "The system SHALL process payments.") {
		t.Errorf("old body still present in result:\n%s", out)
	}
	if !strings.Contains(out, "### Requirement: REQ-1 — Engram Import Dispatch (--from-engram)") {
		t.Errorf("untouched requirement dropped from result:\n%s", out)
	}
	if !strings.HasPrefix(out, "# Payment Spec") {
		t.Errorf("spec header lost:\n%s", out)
	}
	if first, pay := strings.Index(out, "REQ-1 — Engram"), strings.Index(out, "### Requirement: Payment Flow"); first < 0 || pay < 0 || first > pay {
		t.Errorf("requirement order changed (first=%d pay=%d):\n%s", first, pay, out)
	}
}

func TestApplyDeltas_ParsedRetitleDeltaSuggestsExistingHeading(t *testing.T) {
	deltaDoc := "## MODIFIED Requirements\n\n### Requirement: REQ-1 — Import Dispatch (retitled)\n\nPeek.\n\n#### Scenario: Retitled\n\n- **WHEN** x\n- **THEN** y\n"
	parsed, err := ParseDeltaSpec(deltaDoc)
	if err != nil {
		t.Fatalf("ParseDeltaSpec: %v", err)
	}
	if len(parsed.Deltas) != 1 || parsed.Deltas[0].Kind != DeltaModified {
		t.Fatalf("parsed deltas = %+v, want exactly one MODIFIED", parsed.Deltas)
	}
	_, err = ApplyDeltas(retitleMainSpec, parsed.Deltas)
	if err == nil {
		t.Fatal("ApplyDeltas succeeded for a retitled delta, want error")
	}
	if !strings.Contains(err.Error(), `"REQ-1 — Engram Import Dispatch (--from-engram)"`) {
		t.Errorf("error %q must suggest the exact existing heading", err)
	}
}

func TestRequirementIDPrefix(t *testing.T) {
	cases := []struct{ name, want string }{
		{"REQ-1 — Engram Import Dispatch (--from-engram)", "REQ-1"},
		{"REQ-PIPELINE-001 — StagePlan Prepare/Apply Contract", "REQ-PIPELINE-001"},
		{"req-b1 — lowercase id", "REQ-B1"},
		{"Payment Flow", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := requirementIDPrefix(tc.name); got != tc.want {
			t.Errorf("requirementIDPrefix(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}
