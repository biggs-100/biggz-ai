package sdd

import (
	"strings"
	"testing"
)

// verifyEnvelope builds a biggz-native verify report envelope with the
// given fields, mirroring gentle-ai's testVerifyEnvelope helper.
func verifyEnvelope(verdict string, blockers, critical int, requirements, scenarios string, testExit, buildExit int) string {
	return strings.Join([]string{
		"```yaml",
		"schema: biggz-ai.verify-result/v1",
		"evidence_revision: sha256:" + strings.Repeat("a", 64),
		"verdict: " + verdict,
		"blockers: " + itoa(blockers),
		"critical_findings: " + itoa(critical),
		"requirements: " + requirements,
		"scenarios: " + scenarios,
		"test_command: go test ./internal/example",
		"test_exit_code: " + itoa(testExit),
		"test_output_hash: sha256:" + strings.Repeat("b", 64),
		"build_command: go test ./cmd/biggz",
		"build_exit_code: " + itoa(buildExit),
		"build_output_hash: sha256:" + strings.Repeat("c", 64),
		"```",
	}, "\n")
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	return "1"
}

// TestParseVerifyResultFailsClosedAndNamesFirstCondition is T2: the
// derivation evaluation fails closed with the first failing condition in
// order, accepts both schemas, and extracts the evidence revision.
func TestParseVerifyResultFailsClosedAndNamesFirstCondition(t *testing.T) {
	valid := verifyEnvelope("pass", 0, 0, "2/2", "3/3", 0, 0)
	tests := []struct {
		name         string
		report       string
		expected     SpecCounts
		wantPass     bool
		wantReason   string
		wantEvidence string
	}{
		{name: "valid measured result", report: valid, expected: SpecCounts{Requirements: 2, Scenarios: 3}, wantPass: true, wantEvidence: "sha256:" + strings.Repeat("a", 64)},
		{name: "legacy gentle schema accepted", report: strings.Replace(valid, "schema: biggz-ai.verify-result/v1", "schema: gentle-ai.verify-result/v1", 1), expected: SpecCounts{Requirements: 2, Scenarios: 3}, wantPass: true},
		{name: "prose cannot pass", report: "Verdict: PASS\nAll checks passed.", expected: SpecCounts{Requirements: 2, Scenarios: 3}, wantReason: "missing YAML envelope"},
		{name: "unknown schema fails closed", report: strings.Replace(valid, "schema: biggz-ai.verify-result/v1", "schema: other.schema/v1", 1), expected: SpecCounts{Requirements: 2, Scenarios: 3}, wantReason: "unknown schema"},
		{name: "unknown verdict fails closed", report: strings.Replace(valid, "verdict: pass", "verdict: maybe", 1), expected: SpecCounts{Requirements: 2, Scenarios: 3}, wantReason: "unknown verdict"},
		{name: "invalid evidence revision fails closed", report: strings.Replace(valid, "sha256:"+strings.Repeat("a", 64), "sha256:nope", 1), expected: SpecCounts{Requirements: 2, Scenarios: 3}, wantReason: "invalid evidence_revision"},
		{name: "failed tests cannot pass", report: verifyEnvelope("pass", 0, 0, "2/2", "3/3", 1, 0), expected: SpecCounts{Requirements: 2, Scenarios: 3}, wantReason: "test_exit_code must be zero"},
		{name: "failed build cannot pass", report: verifyEnvelope("pass", 0, 0, "2/2", "3/3", 0, 1), expected: SpecCounts{Requirements: 2, Scenarios: 3}, wantReason: "build_exit_code must be zero"},
		{name: "requirement total mismatch", report: valid, expected: SpecCounts{Requirements: 3, Scenarios: 3}, wantReason: "does not match actual requirement count"},
		{name: "scenario total mismatch", report: valid, expected: SpecCounts{Requirements: 2, Scenarios: 4}, wantReason: "does not match actual scenario count"},
		{name: "blockers must be zero", report: strings.Replace(valid, "blockers: 0", "blockers: 1", 1), expected: SpecCounts{Requirements: 2, Scenarios: 3}, wantReason: "blockers must be zero"},
		{name: "critical findings must be zero", report: strings.Replace(valid, "critical_findings: 0", "critical_findings: 1", 1), expected: SpecCounts{Requirements: 2, Scenarios: 3}, wantReason: "critical_findings must be zero"},
		{name: "incomplete requirements", report: verifyEnvelope("pass", 0, 0, "1/2", "3/3", 0, 0), expected: SpecCounts{Requirements: 2, Scenarios: 3}, wantReason: "requirements are incomplete"},
		{name: "incomplete scenarios", report: verifyEnvelope("pass", 0, 0, "2/2", "2/3", 0, 0), expected: SpecCounts{Requirements: 2, Scenarios: 3}, wantReason: "scenarios are incomplete"},
		{name: "fail verdict requires remediation", report: strings.Replace(valid, "verdict: pass", "verdict: fail", 1), expected: SpecCounts{Requirements: 2, Scenarios: 3}, wantReason: "verdict requires remediation"},
		{name: "malformed requirements field", report: strings.Replace(valid, "requirements: 2/2", "requirements: broken", 1), expected: SpecCounts{Requirements: 2, Scenarios: 3}, wantReason: "invalid requirements"},
		{name: "missing requirements field", report: strings.Replace(valid, "requirements: 2/2\n", "", 1), expected: SpecCounts{Requirements: 2, Scenarios: 3}, wantReason: "invalid requirements"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseVerifyResult(tt.report, tt.expected)
			if got.Passing != tt.wantPass {
				t.Fatalf("Passing = %v, want %v (reason %q)", got.Passing, tt.wantPass, got.Reason)
			}
			if tt.wantReason != "" && !strings.Contains(got.Reason, tt.wantReason) {
				t.Fatalf("Reason = %q, want containing %q", got.Reason, tt.wantReason)
			}
			if tt.wantEvidence != "" && got.EvidenceRevision != tt.wantEvidence {
				t.Fatalf("EvidenceRevision = %q, want %q", got.EvidenceRevision, tt.wantEvidence)
			}
			if got.Passing && got.EvidenceRevision == "" {
				t.Fatal("passing evaluation lost the evidence revision")
			}
		})
	}
}

// TestReadVerifyResultMissing asserts the missing-file reason.
func TestReadVerifyResultMissing(t *testing.T) {
	got := readVerifyResult("", SpecCounts{Requirements: 1, Scenarios: 1})
	if got.Passing {
		t.Fatal("missing verify report passed")
	}
	if !strings.Contains(got.Reason, "verify result is missing") {
		t.Fatalf("Reason = %q, want %q", got.Reason, "verify result is missing")
	}
}

// TestCountSpecRequirementsAndScenariosUsesActualArtifacts ports gentle-ai's
// spec counting semantics: canonical and historical heading forms count,
// malformed and arbitrary headings are excluded.
func TestCountSpecRequirementsAndScenariosUsesActualArtifacts(t *testing.T) {
	tests := []struct {
		name  string
		specs []string
		want  SpecCounts
	}{
		{
			name: "canonical Requirement headings",
			specs: []string{
				"### Requirement: First\n#### Scenario: A\n#### Scenario: B\n",
				"### Requirement: Second\n#### Scenario: C\n",
			},
			want: SpecCounts{Requirements: 2, Scenarios: 3},
		},
		{
			name: "historical numeric REQ headings",
			specs: []string{
				"### REQ-1: First\n#### Scenario: A\n",
				"### REQ-12: Second\n#### Scenario: B\n",
			},
			want: SpecCounts{Requirements: 2, Scenarios: 2},
		},
		{
			name: "malformed and arbitrary headings are excluded",
			specs: []string{
				"### REQ-: Missing number\n### REQ-ABC: Not historical\n### Requirements: Plural\n### Overview: Arbitrary\n#### Scenario: Covered\n",
			},
			want: SpecCounts{Requirements: 0, Scenarios: 1},
		},
		{
			name: "scenario without requirement body counts alone",
			specs: []string{
				"#### Scenario: Solo\n",
			},
			want: SpecCounts{Requirements: 0, Scenarios: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := countSpecRequirementsAndScenarios(tt.specs); got != tt.want {
				t.Fatalf("counts = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// TestDeriveSpecCountsFeedVerifyEvaluation wires spec counts into the
// derivation end to end: a verify report whose totals match the actual specs
// passes; one whose totals drift fails with the named counts.
func TestDeriveSpecCountsFeedVerifyEvaluation(t *testing.T) {
	workspace := t.TempDir()
	specs := "### Requirement: A\n#### Scenario: A1\n#### Scenario: A2\n### Requirement: B\n#### Scenario: B1\n"
	changeRoot := seedDeriveChange(t, workspace, "counts-change", map[string]string{
		"proposal.md":        "# Proposal\n",
		"specs/core/spec.md": specs,
		"design.md":          "# Design\n",
		"tasks.md":           "- [x] T1\n",
		"verify-report.md":   passingReport("2/2", "3/3"),
	})
	cs, err := readChange(changeRoot, "counts-change", false, workspace, false)
	if err != nil {
		t.Fatalf("readChange: %v", err)
	}
	if cs.NextRecommended != "archive" {
		t.Errorf("NextRecommended = %q, want archive", cs.NextRecommended)
	}

	changeRootDrift := seedDeriveChange(t, workspace, "counts-drift", map[string]string{
		"proposal.md":        "# Proposal\n",
		"specs/core/spec.md": specs,
		"design.md":          "# Design\n",
		"tasks.md":           "- [x] T1\n",
		"verify-report.md":   passingReport("2/2", "2/2"),
	})
	csDrift, err := readChange(changeRootDrift, "counts-drift", false, workspace, false)
	if err != nil {
		t.Fatalf("readChange: %v", err)
	}
	if csDrift.NextRecommended != "remediate" {
		t.Errorf("drift NextRecommended = %q, want remediate", csDrift.NextRecommended)
	}
	found := false
	for _, reason := range csDrift.BlockedReasons {
		if strings.Contains(reason, "does not match actual scenario count") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("drift BlockedReasons = %#v, want a scenario count mismatch reason", csDrift.BlockedReasons)
	}
}
