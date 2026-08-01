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

// RemediationResultSchema is the legacy gentle-ai schema identifier for
// remediation results. It remains accepted for read-compatibility with
// historical remediation reports; new reports are emitted with the
// biggz-native schema.
const RemediationResultSchema = "gentle-ai.remediation-result/v1"

// BiggzRemediationResultSchema is the biggz-native schema identifier for
// remediation results. The validator accepts both this and the legacy
// gentle-ai schema.
const BiggzRemediationResultSchema = "biggz-ai.remediation-result/v1"

// BiggzRemediationEvidenceSchema is the biggz-native schema identifier for
// the focused-remediation evidence JSON emitted immediately after the
// remediation result envelope. It is produced by emitters; no Go-side
// validator exists for the evidence JSON today.
const BiggzRemediationEvidenceSchema = "biggz-ai.remediation-evidence/v1"

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

	// Validate schema: accept the native biggz schema and the legacy
	// gentle-ai schema (read-compatibility with historical reports).
	if result.Schema != RemediationResultSchema && result.Schema != BiggzRemediationResultSchema {
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

	// Validate schema: accept the native biggz schema and the legacy
	// gentle-ai schema (read-compatibility with historical reports).
	if report.Schema != VerifyResultSchema && report.Schema != BiggzVerifyResultSchema {
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

// ─── Verify admission (Phase C1 rigor) ───────────────────────────────────────

const (
	// VerifyResultSchema is the legacy gentle-ai verify report envelope
	// schema, accepted for read-compatibility with historical reports.
	VerifyResultSchema = "gentle-ai.verify-result/v1"
	// BiggzVerifyResultSchema is the biggz-native alias of the verify report
	// envelope. Admission accepts both.
	BiggzVerifyResultSchema = "biggz-ai.verify-result/v1"
	// VerifyAdmissionSchema identifies the structured admission decision.
	VerifyAdmissionSchema = "biggz-ai.verify-admission/v1"
	// MaxVerifyReportBytes caps the input report size at 1 MiB.
	MaxVerifyReportBytes = 1 << 20
)

// VerifyCountPair reports the authoritative declared count and the count the
// validator counted from the report. Declared is nil when no authoritative
// count was given (lenient mode).
type VerifyCountPair struct {
	Declared *int `json:"declared,omitempty"`
	Counted  int  `json:"counted"`
}

// VerifyAdmission is the structured admission decision for a verify report.
type VerifyAdmission struct {
	Schema       string          `json:"schema"`
	Decision     string          `json:"decision"` // "admitted" | "denied"
	Reason       string          `json:"reason,omitempty"`
	Requirements VerifyCountPair `json:"requirements"`
	Scenarios    VerifyCountPair `json:"scenarios"`
}

// ValidateVerifyReportAdmission validates exact report bytes against
// authoritative spec counts and returns the structured admission decision.
//
// Pass -1 for both declared counts to keep lenient mode: the report is
// checked for intrinsic validity (envelope, schema, verdict, critical
// issues) but counts are not compared. When either declared count is
// nonnegative both must be, and they are authoritative: a report whose
// counts differ is denied with a named reason.
func ValidateVerifyReportAdmission(content []byte, declaredRequirements, declaredScenarios int) VerifyAdmission {
	admission := VerifyAdmission{Schema: VerifyAdmissionSchema, Decision: "denied"}
	if declaredRequirements >= 0 || declaredScenarios >= 0 {
		if declaredRequirements < 0 || declaredScenarios < 0 {
			admission.Reason = "declared requirement and scenario counts must be provided together"
			return admission
		}
		admission.Requirements.Declared = &declaredRequirements
		admission.Scenarios.Declared = &declaredScenarios
	}

	text := string(content)
	yamlRe := regexp.MustCompile("(?s)```yaml\\s+(.*?)```")
	matches := yamlRe.FindStringSubmatch(text)
	if len(matches) < 2 {
		admission.Reason = "verify report: missing YAML envelope (```yaml ... ```)"
		return admission
	}

	report, err := parseYAMLEnvelope(matches[1])
	if err != nil {
		admission.Reason = "verify report: parse envelope: " + err.Error()
		return admission
	}

	if report.Schema != VerifyResultSchema && report.Schema != BiggzVerifyResultSchema {
		admission.Reason = fmt.Sprintf("verify report: unknown schema %q", report.Schema)
		return admission
	}

	counted, ok := parseCount(report.Requirements)
	if !ok {
		admission.Reason = fmt.Sprintf("verify report: invalid requirements count %q", report.Requirements)
		return admission
	}
	admission.Requirements.Counted = counted
	if admission.Requirements.Declared != nil && counted != *admission.Requirements.Declared {
		admission.Reason = fmt.Sprintf("verify report: requirements count %d does not match authoritative %d", counted, *admission.Requirements.Declared)
		return admission
	}

	counted, ok = parseCount(report.Scenarios)
	if !ok {
		admission.Reason = fmt.Sprintf("verify report: invalid scenarios count %q", report.Scenarios)
		return admission
	}
	admission.Scenarios.Counted = counted
	if admission.Scenarios.Declared != nil && counted != *admission.Scenarios.Declared {
		admission.Reason = fmt.Sprintf("verify report: scenarios count %d does not match authoritative %d", counted, *admission.Scenarios.Declared)
		return admission
	}

	switch report.Verdict {
	case "pass", "pass_with_warnings":
	case "fail":
		admission.Reason = fmt.Sprintf("verify report: verdict is FAIL (%d blockers, %d critical)", report.Blockers, report.CriticalFindings)
		return admission
	default:
		admission.Reason = fmt.Sprintf("verify report: unknown verdict %q", report.Verdict)
		return admission
	}

	if strings.Contains(text, "**CRITICAL**:") && !strings.Contains(text, "**CRITICAL**: None") {
		admission.Reason = "verify report: contains unaddressed CRITICAL issues"
		return admission
	}

	admission.Decision = "admitted"
	return admission
}

// parseCount extracts the count from a "N/M" completion field. It mirrors the
// legacy validator's comparison semantics: the counted value is the completed
// count (the number before the slash).
func parseCount(field string) (int, bool) {
	if field == "" {
		return 0, false
	}
	var n int
	if _, err := fmt.Sscanf(field, "%d/", &n); err != nil {
		return 0, false
	}
	return n, true
}
