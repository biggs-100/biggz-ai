package sdd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateVerifyReport_Valid(t *testing.T) {
	report := "```yaml\nschema: gentle-ai.verify-result/v1\nverdict: pass\nblockers: 0\ncritical_findings: 0\nrequirements: 5/5\nscenarios: 10/10\ntest_exit_code: 0\nbuild_exit_code: 0\n```\n\n## Verification Report\n\n**CRITICAL**: None\n"

	path := filepath.Join(t.TempDir(), "verify-report.md")
	os.WriteFile(path, []byte(report), 0644)

	err := ValidateVerifyReport(path, 5, 10)
	if err != nil {
		t.Fatalf("ValidateVerifyReport() unexpected error: %v", err)
	}
}

func TestValidateVerifyReport_MissingYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "verify-report.md")
	os.WriteFile(path, []byte("# No YAML envelope"), 0644)

	err := ValidateVerifyReport(path, 0, 0)
	if err == nil {
		t.Fatal("expected error for missing YAML envelope")
	}
}

func TestValidateVerifyReport_RequirementsMismatch(t *testing.T) {
	report := "```yaml\nschema: gentle-ai.verify-result/v1\nverdict: pass\nrequirements: 3/5\nscenarios: 10/10\n```\n"
	path := filepath.Join(t.TempDir(), "verify-report.md")
	os.WriteFile(path, []byte(report), 0644)

	err := ValidateVerifyReport(path, 5, 10)
	if err == nil {
		t.Fatal("expected error for requirements mismatch")
	}
}

func TestValidateVerifyReport_ScenariosMismatch(t *testing.T) {
	report := "```yaml\nschema: gentle-ai.verify-result/v1\nverdict: pass\nrequirements: 5/5\nscenarios: 8/10\n```\n"
	path := filepath.Join(t.TempDir(), "verify-report.md")
	os.WriteFile(path, []byte(report), 0644)

	err := ValidateVerifyReport(path, 5, 10)
	if err == nil {
		t.Fatal("expected error for scenarios mismatch")
	}
}

func TestValidateVerifyReport_FailVerdict(t *testing.T) {
	report := "```yaml\nschema: gentle-ai.verify-result/v1\nverdict: fail\nblockers: 2\ncritical_findings: 1\nrequirements: 5/5\nscenarios: 10/10\n```\n"
	path := filepath.Join(t.TempDir(), "verify-report.md")
	os.WriteFile(path, []byte(report), 0644)

	err := ValidateVerifyReport(path, 5, 10)
	if err == nil {
		t.Fatal("expected error for fail verdict")
	}
}

func TestValidateVerifyReport_CriticalIssues(t *testing.T) {
	report := "```yaml\nschema: gentle-ai.verify-result/v1\nverdict: pass\nblockers: 0\ncritical_findings: 0\n```\n\n**CRITICAL**:\n- Something is broken\n"
	path := filepath.Join(t.TempDir(), "verify-report.md")
	os.WriteFile(path, []byte(report), 0644)

	err := ValidateVerifyReport(path, -1, -1)
	if err == nil {
		t.Fatal("expected error for critical issues")
	}
}
