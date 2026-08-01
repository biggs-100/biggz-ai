package sdd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateVerifyReport_Valid(t *testing.T) {
	report := "```yaml\nschema: biggz-ai.verify-result/v1\nverdict: pass\nblockers: 0\ncritical_findings: 0\nrequirements: 5/5\nscenarios: 10/10\ntest_exit_code: 0\nbuild_exit_code: 0\n```\n\n## Verification Report\n\n**CRITICAL**: None\n"

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
	report := "```yaml\nschema: biggz-ai.verify-result/v1\nverdict: pass\nrequirements: 3/5\nscenarios: 10/10\n```\n"
	path := filepath.Join(t.TempDir(), "verify-report.md")
	os.WriteFile(path, []byte(report), 0644)

	err := ValidateVerifyReport(path, 5, 10)
	if err == nil {
		t.Fatal("expected error for requirements mismatch")
	}
}

func TestValidateVerifyReport_ScenariosMismatch(t *testing.T) {
	report := "```yaml\nschema: biggz-ai.verify-result/v1\nverdict: pass\nrequirements: 5/5\nscenarios: 8/10\n```\n"
	path := filepath.Join(t.TempDir(), "verify-report.md")
	os.WriteFile(path, []byte(report), 0644)

	err := ValidateVerifyReport(path, 5, 10)
	if err == nil {
		t.Fatal("expected error for scenarios mismatch")
	}
}

func TestValidateVerifyReport_FailVerdict(t *testing.T) {
	report := "```yaml\nschema: biggz-ai.verify-result/v1\nverdict: fail\nblockers: 2\ncritical_findings: 1\nrequirements: 5/5\nscenarios: 10/10\n```\n"
	path := filepath.Join(t.TempDir(), "verify-report.md")
	os.WriteFile(path, []byte(report), 0644)

	err := ValidateVerifyReport(path, 5, 10)
	if err == nil {
		t.Fatal("expected error for fail verdict")
	}
}

func TestValidateVerifyReport_CriticalIssues(t *testing.T) {
	report := "```yaml\nschema: biggz-ai.verify-result/v1\nverdict: pass\nblockers: 0\ncritical_findings: 0\n```\n\n**CRITICAL**:\n- Something is broken\n"
	path := filepath.Join(t.TempDir(), "verify-report.md")
	os.WriteFile(path, []byte(report), 0644)

	err := ValidateVerifyReport(path, -1, -1)
	if err == nil {
		t.Fatal("expected error for critical issues")
	}
}

func TestValidateVerifyReport_LegacySchemaStillValidates(t *testing.T) {
	report := "```yaml\nschema: gentle-ai.verify-result/v1\nverdict: pass\nblockers: 0\ncritical_findings: 0\nrequirements: 5/5\nscenarios: 10/10\ntest_exit_code: 0\nbuild_exit_code: 0\n```\n\n## Verification Report\n\n**CRITICAL**: None\n"
	path := filepath.Join(t.TempDir(), "verify-report.md")
	os.WriteFile(path, []byte(report), 0644)

	if err := ValidateVerifyReport(path, 5, 10); err != nil {
		t.Fatalf("ValidateVerifyReport() rejected legacy gentle-ai schema: %v", err)
	}
}

func TestValidateRemediationResult_SchemaCompat(t *testing.T) {
	writeReport := func(schema string) string {
		path := filepath.Join(t.TempDir(), "remediation-result.md")
		content := fmt.Sprintf("```yaml\nschema: %s\nverdict: resolved\nblockers: 0\n```\n", schema)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write remediation result: %v", err)
		}
		return path
	}

	for _, tc := range []struct {
		name   string
		schema string
		wantOK bool
	}{
		{"native schema admitted", BiggzRemediationResultSchema, true},
		{"legacy schema admitted", RemediationResultSchema, true},
		{"unknown schema rejected", "other.schema/v1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ValidateRemediationResult(writeReport(tc.schema))
			if tc.wantOK && err != nil {
				t.Fatalf("ValidateRemediationResult() unexpected error: %v", err)
			}
			if !tc.wantOK {
				if err == nil {
					t.Fatal("expected error for unknown schema")
				}
				return
			}
			if result.Schema != tc.schema {
				t.Errorf("result.Schema = %q, want %q", result.Schema, tc.schema)
			}
		})
	}
}
