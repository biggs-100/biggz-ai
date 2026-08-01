package sdd

import (
	"strings"
	"testing"
)

const validReport = "```yaml\nschema: biggz-ai.verify-result/v1\nverdict: pass\nblockers: 0\ncritical_findings: 0\nrequirements: 5/5\nscenarios: 10/10\ntest_exit_code: 0\nbuild_exit_code: 0\n```\n\n## Verification Report\n\n**CRITICAL**: None\n"

func TestValidateVerifyReportAdmission_Admitted(t *testing.T) {
	admission := ValidateVerifyReportAdmission([]byte(validReport), 5, 10)
	if admission.Decision != "admitted" {
		t.Fatalf("Decision = %q, want admitted (reason: %q)", admission.Decision, admission.Reason)
	}
	if admission.Schema != VerifyAdmissionSchema {
		t.Errorf("Schema = %q, want %q", admission.Schema, VerifyAdmissionSchema)
	}
	if admission.Requirements.Declared == nil || *admission.Requirements.Declared != 5 {
		t.Errorf("Requirements.Declared = %v, want 5", admission.Requirements.Declared)
	}
	if admission.Requirements.Counted != 5 {
		t.Errorf("Requirements.Counted = %d, want 5", admission.Requirements.Counted)
	}
	if admission.Scenarios.Declared == nil || *admission.Scenarios.Declared != 10 {
		t.Errorf("Scenarios.Declared = %v, want 10", admission.Scenarios.Declared)
	}
	if admission.Scenarios.Counted != 10 {
		t.Errorf("Scenarios.Counted = %d, want 10", admission.Scenarios.Counted)
	}
	if admission.Reason != "" {
		t.Errorf("Reason = %q, want empty for admission", admission.Reason)
	}
}

func TestValidateVerifyReportAdmission_LegacySchemaAccepted(t *testing.T) {
	report := strings.Replace(validReport, "biggz-ai.verify-result/v1", "gentle-ai.verify-result/v1", 1)
	admission := ValidateVerifyReportAdmission([]byte(report), 5, 10)
	if admission.Decision != "admitted" {
		t.Fatalf("Decision = %q, want admitted for legacy schema (reason: %q)", admission.Decision, admission.Reason)
	}
}

func TestValidateVerifyReportAdmission_RequirementsMismatchDenied(t *testing.T) {
	admission := ValidateVerifyReportAdmission([]byte(validReport), 7, 10)
	if admission.Decision != "denied" {
		t.Fatalf("Decision = %q, want denied", admission.Decision)
	}
	if !strings.Contains(admission.Reason, "requirements count 5") {
		t.Errorf("Reason should name the requirements mismatch, got: %q", admission.Reason)
	}
	if *admission.Requirements.Declared != 7 || admission.Requirements.Counted != 5 {
		t.Errorf("Requirements pair = declared %d counted %d, want 7/5", *admission.Requirements.Declared, admission.Requirements.Counted)
	}
}

func TestValidateVerifyReportAdmission_ScenariosMismatchDenied(t *testing.T) {
	admission := ValidateVerifyReportAdmission([]byte(validReport), 5, 9)
	if admission.Decision != "denied" {
		t.Fatalf("Decision = %q, want denied", admission.Decision)
	}
	if !strings.Contains(admission.Reason, "scenarios count 10") {
		t.Errorf("Reason should name the scenarios mismatch, got: %q", admission.Reason)
	}
}

func TestValidateVerifyReportAdmission_OnlyOneDeclaredCountDenied(t *testing.T) {
	admission := ValidateVerifyReportAdmission([]byte(validReport), 5, -1)
	if admission.Decision != "denied" {
		t.Fatalf("Decision = %q, want denied", admission.Decision)
	}
	if !strings.Contains(admission.Reason, "together") {
		t.Errorf("Reason should require both counts, got: %q", admission.Reason)
	}
}

func TestValidateVerifyReportAdmission_LenientMode(t *testing.T) {
	admission := ValidateVerifyReportAdmission([]byte(validReport), -1, -1)
	if admission.Decision != "admitted" {
		t.Fatalf("Decision = %q, want admitted in lenient mode (reason: %q)", admission.Decision, admission.Reason)
	}
	if admission.Requirements.Declared != nil || admission.Scenarios.Declared != nil {
		t.Error("lenient mode must not carry declared counts")
	}
	if admission.Requirements.Counted != 5 || admission.Scenarios.Counted != 10 {
		t.Errorf("counted = req %d scen %d, want 5/10", admission.Requirements.Counted, admission.Scenarios.Counted)
	}
}

func TestValidateVerifyReportAdmission_MissingEnvelopeDenied(t *testing.T) {
	admission := ValidateVerifyReportAdmission([]byte("# no envelope"), 5, 10)
	if admission.Decision != "denied" || !strings.Contains(admission.Reason, "missing YAML envelope") {
		t.Fatalf("Decision = %q reason %q, want denied with envelope reason", admission.Decision, admission.Reason)
	}
}

func TestValidateVerifyReportAdmission_UnknownSchemaDenied(t *testing.T) {
	report := strings.Replace(validReport, "biggz-ai.verify-result/v1", "other.schema/v1", 1)
	admission := ValidateVerifyReportAdmission([]byte(report), 5, 10)
	if admission.Decision != "denied" || !strings.Contains(admission.Reason, "unknown schema") {
		t.Fatalf("Decision = %q reason %q, want denied with schema reason", admission.Decision, admission.Reason)
	}
}

func TestValidateVerifyReportAdmission_FailVerdictDenied(t *testing.T) {
	report := "```yaml\nschema: biggz-ai.verify-result/v1\nverdict: fail\nblockers: 2\ncritical_findings: 1\nrequirements: 5/5\nscenarios: 10/10\n```\n"
	admission := ValidateVerifyReportAdmission([]byte(report), 5, 10)
	if admission.Decision != "denied" || !strings.Contains(admission.Reason, "verdict is FAIL") {
		t.Fatalf("Decision = %q reason %q, want denied for fail verdict", admission.Decision, admission.Reason)
	}
}

func TestValidateVerifyReportAdmission_CriticalIssuesDenied(t *testing.T) {
	report := "```yaml\nschema: biggz-ai.verify-result/v1\nverdict: pass\nblockers: 0\ncritical_findings: 0\nrequirements: 5/5\nscenarios: 10/10\n```\n\n**CRITICAL**:\n- broken\n"
	admission := ValidateVerifyReportAdmission([]byte(report), 5, 10)
	if admission.Decision != "denied" || !strings.Contains(admission.Reason, "CRITICAL") {
		t.Fatalf("Decision = %q reason %q, want denied for critical issues", admission.Decision, admission.Reason)
	}
}

func TestValidateVerifyReportAdmission_InvalidCountFieldDenied(t *testing.T) {
	report := "```yaml\nschema: biggz-ai.verify-result/v1\nverdict: pass\nrequirements: x/5\nscenarios: 10/10\n```\n"
	admission := ValidateVerifyReportAdmission([]byte(report), 5, 10)
	if admission.Decision != "denied" || !strings.Contains(admission.Reason, "invalid requirements count") {
		t.Fatalf("Decision = %q reason %q, want denied for invalid count field", admission.Decision, admission.Reason)
	}
}
