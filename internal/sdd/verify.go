package sdd

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// VerifyReport represents the parsed YAML envelope of a verify report.
type VerifyReport struct {
	Schema           string
	Verdict          string
	Blockers         int
	CriticalFindings int
	Requirements     string
	Scenarios        string
	TestExitCode     int
	BuildExitCode    int
}

// parseYAMLEnvelope extracts key:value pairs from the YAML block.
func parseYAMLEnvelope(yamlContent string) (*VerifyReport, error) {
	r := &VerifyReport{}
	lines := strings.Split(yamlContent, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "schema":
			r.Schema = val
		case "verdict":
			r.Verdict = val
		case "blockers":
			r.Blockers, _ = strconv.Atoi(val)
		case "critical_findings":
			r.CriticalFindings, _ = strconv.Atoi(val)
		case "requirements":
			r.Requirements = val
		case "scenarios":
			r.Scenarios = val
		case "test_exit_code":
			r.TestExitCode, _ = strconv.Atoi(val)
		case "build_exit_code":
			r.BuildExitCode, _ = strconv.Atoi(val)
		}
	}
	return r, nil
}

// ─── Remediation Result ──────────────────────────────────────────────────────

// RemediationResultSchema is the schema identifier for remediation results.
const RemediationResultSchema = "gentle-ai.remediation-result/v1"

// RemediationResult represents a parsed remediation result.
type RemediationResult struct {
	Schema    string `json:"schema"`
	Verdict   string `json:"verdict"` // "resolved", "partial", "unresolved"
	Evidence  string `json:"evidence,omitempty"`
	Blockers  int    `json:"blockers,omitempty"`
	Diagnosis string `json:"diagnosis,omitempty"`
}

// ValidateRemediationResult validates a remediation result file.
func ValidateRemediationResult(path string) (*RemediationResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read remediation result: %w", err)
	}

	content := string(data)

	// Extract YAML envelope
	yamlRe := regexp.MustCompile("(?s)```yaml\\s+(.*?)```")
	matches := yamlRe.FindStringSubmatch(content)
	if len(matches) < 2 {
		return nil, fmt.Errorf("remediation result: missing YAML envelope")
	}

	report, err := parseYAMLEnvelope(matches[1])
	if err != nil {
		return nil, fmt.Errorf("parse remediation: %w", err)
	}

	// Map fields to RemediationResult
	result := &RemediationResult{
		Schema:  report.Schema,
		Verdict: report.Verdict,
		Blockers: report.Blockers,
	}

	// Validate schema
	if result.Schema != RemediationResultSchema {
		return nil, fmt.Errorf("remediation result: unknown schema %q", result.Schema)
	}

	// Validate verdict
	switch result.Verdict {
	case "resolved":
		// OK
	case "partial":
		return nil, fmt.Errorf("remediation result: partial — remaining blockers: %d", result.Blockers)
	case "unresolved":
		return nil, fmt.Errorf("remediation result: unresolved — remediation failed, diagnosis required")
	default:
		return nil, fmt.Errorf("remediation result: unknown verdict %q", result.Verdict)
	}

	return result, nil
}

// ValidateVerifyReport reads a verify report, checks its format,
// and validates requirement/scenario counts against authoritative values.
func ValidateVerifyReport(path string, reqRequirements, reqScenarios int) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read verify report: %w", err)
	}

	content := string(data)

	// Extract YAML envelope (between ```yaml and ```)
	yamlRe := regexp.MustCompile("(?s)```yaml\\s+(.*?)```")
	matches := yamlRe.FindStringSubmatch(content)
	if len(matches) < 2 {
		return fmt.Errorf("verify report: missing YAML envelope (```yaml ... ```)")
	}

	report, err := parseYAMLEnvelope(matches[1])
	if err != nil {
		return fmt.Errorf("verify report: parse envelope: %w", err)
	}

	// Validate schema
	if report.Schema != "gentle-ai.verify-result/v1" {
		return fmt.Errorf("verify report: unknown schema %q", report.Schema)
	}

	// Validate requirements count
	if report.Requirements != "" {
		var n int
		if _, err := fmt.Sscanf(report.Requirements, "%d/", &n); err == nil {
			if reqRequirements >= 0 && n != reqRequirements {
				return fmt.Errorf("verify report: requirements count %d does not match authoritative %d", n, reqRequirements)
			}
		}
	}

	// Validate scenarios count
	if report.Scenarios != "" {
		var n int
		if _, err := fmt.Sscanf(report.Scenarios, "%d/", &n); err == nil {
			if reqScenarios >= 0 && n != reqScenarios {
				return fmt.Errorf("verify report: scenarios count %d does not match authoritative %d", n, reqScenarios)
			}
		}
	}

	// Check verdict
	switch report.Verdict {
	case "pass", "pass_with_warnings":
		// OK
	case "fail":
		return fmt.Errorf("verify report: verdict is FAIL (%d blockers, %d critical)", report.Blockers, report.CriticalFindings)
	default:
		return fmt.Errorf("verify report: unknown verdict %q", report.Verdict)
	}

	// Check for unaddressed CRITICAL issues
	if strings.Contains(content, "**CRITICAL**:") && !strings.Contains(content, "**CRITICAL**: None") {
		return fmt.Errorf("verify report: contains unaddressed CRITICAL issues")
	}

	return nil
}
