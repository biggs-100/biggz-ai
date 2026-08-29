package sdd

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/fzipp/gocyclo"
	"github.com/uudashr/gocognit"
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
	EvidenceRevision string
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
		case "evidence_revision":
			r.EvidenceRevision = val
		}
	}
	return r, nil
}

// ─── Derivation evaluation (status.go consumes this) ─────────────────────────

// SpecCounts holds the authoritative requirement and scenario counts derived
// from the change's specs/**/spec.md files.
type SpecCounts struct {
	Requirements int
	Scenarios    int
}

type verifyCompletion struct {
	Completed int
	Total     int
}

// verifyResultEvaluation is the status-layer verdict on a verify report:
// Passing is true only when every archive-readiness condition holds, and
// Reason names the first failing condition.
type verifyResultEvaluation struct {
	Passing          bool
	Reason           string
	EvidenceRevision string
}

// sha256IdentityPattern matches a canonical sha256: identity, the only
// accepted form for evidence revisions.
var sha256IdentityPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// requirementHeadingPattern and scenarioHeadingPattern count spec content.
// Requirements are `### Requirement: ...` or historical numeric
// `### REQ-<n>: ...` headings; scenarios are `#### Scenario: ...` headings.
// Malformed or arbitrary headings are excluded.
var requirementHeadingPattern = regexp.MustCompile(`(?m)^### (?:Requirement|REQ-[0-9]+):\s+\S`)
var scenarioHeadingPattern = regexp.MustCompile(`(?m)^#### Scenario:\s+\S`)

// countSpecRequirementsAndScenarios sums requirement and scenario headings
// over the given spec contents.
func countSpecRequirementsAndScenarios(specs []string) SpecCounts {
	var counts SpecCounts
	for _, spec := range specs {
		counts.Requirements += len(requirementHeadingPattern.FindAllStringIndex(spec, -1))
		counts.Scenarios += len(scenarioHeadingPattern.FindAllStringIndex(spec, -1))
	}
	return counts
}

// readSpecCounts loads and counts every spec path.
func readSpecCounts(paths []string) (SpecCounts, error) {
	contents := make([]string, 0, len(paths))
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return SpecCounts{}, err
		}
		contents = append(contents, string(content))
	}
	return countSpecRequirementsAndScenarios(contents), nil
}

// readVerifyResult evaluates the verify report at path, or reports
// "verify result is missing" when no report exists.
func readVerifyResult(path string, counts SpecCounts) verifyResultEvaluation {
	if path == "" {
		return verifyResultEvaluation{Reason: "verify result is missing"}
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return verifyResultEvaluation{Reason: "verify result is missing"}
	}
	return parseVerifyResult(string(content), counts)
}

// parseVerifyResult evaluates a verify report envelope against the
// authoritative spec counts, accepting both the legacy gentle-ai schema and
// the biggz-native schema. Failing conditions are checked IN ORDER:
// envelope/schema/field errors, test_exit_code, build_exit_code, requirement
// and scenario totals, blockers, critical findings, requirement and scenario
// completion, and finally the verdict. The evidence_revision is extracted
// when it is a canonical sha256: identity.
func extractVerifyReport(text string) (*VerifyReport, verifyResultEvaluation, bool) {
	yamlRe := regexp.MustCompile("(?s)```yaml\\s+(.*?)```")
	matches := yamlRe.FindStringSubmatch(text)
	if len(matches) < 2 {
		return nil, verifyResultEvaluation{Reason: "verify result: missing YAML envelope (```yaml ... ```)"}, false
	}
	report, err := parseYAMLEnvelope(matches[1])
	if err != nil {
		return nil, verifyResultEvaluation{Reason: "verify result: parse envelope: " + err.Error()}, false
	}
	if report.Schema != VerifyResultSchema && report.Schema != BiggzVerifyResultSchema {
		return nil, verifyResultEvaluation{Reason: fmt.Sprintf("verify result: unknown schema %q", report.Schema)}, false
	}
	return report, verifyResultEvaluation{}, true
}

func validateVerifyHeader(report *VerifyReport) (verifyResultEvaluation, verifyCompletion, verifyCompletion, bool) {
	evaluation := verifyResultEvaluation{}
	if report.EvidenceRevision != "" {
		if !sha256IdentityPattern.MatchString(report.EvidenceRevision) {
			return verifyResultEvaluation{Reason: "invalid evidence_revision in verify result envelope"}, verifyCompletion{}, verifyCompletion{}, false
		}
		evaluation.EvidenceRevision = report.EvidenceRevision
	}
	requirements, ok := parseVerifyCompletion(report.Requirements)
	if !ok {
		return verifyResultEvaluation{Reason: "invalid requirements in verify result envelope"}, verifyCompletion{}, verifyCompletion{}, false
	}
	scenarios, ok := parseVerifyCompletion(report.Scenarios)
	if !ok {
		return verifyResultEvaluation{Reason: "invalid scenarios in verify result envelope"}, verifyCompletion{}, verifyCompletion{}, false
	}
	return evaluation, requirements, scenarios, true
}

func checkVerifyExitAndTotals(report *VerifyReport, evaluation verifyResultEvaluation, requirements, scenarios verifyCompletion, expected SpecCounts) (verifyResultEvaluation, bool) {
	if report.TestExitCode != 0 {
		evaluation.Reason = "test_exit_code must be zero for archive readiness"
		return evaluation, false
	}
	if report.BuildExitCode != 0 {
		evaluation.Reason = "build_exit_code must be zero for archive readiness"
		return evaluation, false
	}
	if requirements.Total != expected.Requirements {
		evaluation.Reason = fmt.Sprintf("verify result total %d does not match actual requirement count %d", requirements.Total, expected.Requirements)
		return evaluation, false
	}
	if scenarios.Total != expected.Scenarios {
		evaluation.Reason = fmt.Sprintf("verify result total %d does not match actual scenario count %d", scenarios.Total, expected.Scenarios)
		return evaluation, false
	}
	return evaluation, true
}

func checkVerifyBlockersAndCompletion(report *VerifyReport, evaluation verifyResultEvaluation, requirements, scenarios verifyCompletion) (verifyResultEvaluation, bool) {
	if report.Blockers != 0 {
		evaluation.Reason = "blockers must be zero for archive readiness"
		return evaluation, false
	}
	if report.CriticalFindings != 0 {
		evaluation.Reason = "critical_findings must be zero for archive readiness"
		return evaluation, false
	}
	if requirements.Completed != requirements.Total {
		evaluation.Reason = "requirements are incomplete"
		return evaluation, false
	}
	if scenarios.Completed != scenarios.Total {
		evaluation.Reason = "scenarios are incomplete"
		return evaluation, false
	}
	return evaluation, true
}

func checkVerifyVerdict(report *VerifyReport, evaluation verifyResultEvaluation) (verifyResultEvaluation, bool) {
	switch report.Verdict {
	case "pass", "pass_with_warnings":
		evaluation.Passing = true
		return evaluation, true
	case "fail":
		evaluation.Reason = "verdict requires remediation"
		return evaluation, false
	default:
		return verifyResultEvaluation{Reason: fmt.Sprintf("verify result: unknown verdict %q", report.Verdict)}, false
	}
}

func parseVerifyResult(text string, expected SpecCounts) verifyResultEvaluation {
	report, eval, ok := extractVerifyReport(text)
	if !ok {
		return eval
	}
	evaluation, requirements, scenarios, ok := validateVerifyHeader(report)
	if !ok {
		return evaluation
	}
	if evaluation, ok = checkVerifyExitAndTotals(report, evaluation, requirements, scenarios, expected); !ok {
		return evaluation
	}
	if evaluation, ok = checkVerifyBlockersAndCompletion(report, evaluation, requirements, scenarios); !ok {
		return evaluation
	}
	evaluation, ok = checkVerifyVerdict(report, evaluation)
	if !ok {
		return evaluation
	}
	return evaluation
}

// parseVerifyCompletion parses a "N/M" completion field into its completed
// and total counts, rejecting malformed values and completed > total.
func parseVerifyCompletion(value string) (verifyCompletion, bool) {
	completedRaw, totalRaw, ok := strings.Cut(value, "/")
	if !ok || strings.Contains(totalRaw, "/") {
		return verifyCompletion{}, false
	}
	completed, completedOK := parseNonnegativeInt(completedRaw)
	total, totalOK := parseNonnegativeInt(totalRaw)
	if !completedOK || !totalOK || completed > total {
		return verifyCompletion{}, false
	}
	return verifyCompletion{Completed: completed, Total: total}, true
}

func parseNonnegativeInt(value string) (int, bool) {
	if value == "" {
		return 0, false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return 0, false
		}
	}
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil
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
		Schema:   report.Schema,
		Verdict:  report.Verdict,
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

func validateVerifyReportEnvelope(content string) (*VerifyReport, error) {
	yamlRe := regexp.MustCompile("(?s)```yaml\\s+(.*?)```")
	matches := yamlRe.FindStringSubmatch(content)
	if len(matches) < 2 {
		return nil, fmt.Errorf("verify report: missing YAML envelope (```yaml ... ```)")
	}
	report, err := parseYAMLEnvelope(matches[1])
	if err != nil {
		return nil, fmt.Errorf("verify report: parse envelope: %w", err)
	}
	if report.Schema != VerifyResultSchema && report.Schema != BiggzVerifyResultSchema {
		return nil, fmt.Errorf("verify report: unknown schema %q", report.Schema)
	}
	return report, nil
}

func validateVerifyReportCounts(report *VerifyReport, reqRequirements, reqScenarios int) error {
	if report.Requirements != "" {
		var n int
		if _, err := fmt.Sscanf(report.Requirements, "%d/", &n); err == nil && reqRequirements >= 0 && n != reqRequirements {
			return fmt.Errorf("verify report: requirements count %d does not match authoritative %d", n, reqRequirements)
		}
	}
	if report.Scenarios != "" {
		var n int
		if _, err := fmt.Sscanf(report.Scenarios, "%d/", &n); err == nil && reqScenarios >= 0 && n != reqScenarios {
			return fmt.Errorf("verify report: scenarios count %d does not match authoritative %d", n, reqScenarios)
		}
	}
	return nil
}

func validateVerifyReportVerdict(report *VerifyReport, content string) error {
	switch report.Verdict {
	case "pass", "pass_with_warnings":
	case "fail":
		return fmt.Errorf("verify report: verdict is FAIL (%d blockers, %d critical)", report.Blockers, report.CriticalFindings)
	default:
		return fmt.Errorf("verify report: unknown verdict %q", report.Verdict)
	}
	if strings.Contains(content, "**CRITICAL**:") && !strings.Contains(content, "**CRITICAL**: None") {
		return fmt.Errorf("verify report: contains unaddressed CRITICAL issues")
	}
	return nil
}

// ValidateVerifyReport reads a verify report, checks its format,
// and validates requirement/scenario counts against authoritative values.
func ValidateVerifyReport(path string, reqRequirements, reqScenarios int) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read verify report: %w", err)
	}
	content := string(data)
	report, err := validateVerifyReportEnvelope(content)
	if err != nil {
		return err
	}
	if err := validateVerifyReportCounts(report, reqRequirements, reqScenarios); err != nil {
		return err
	}
	return validateVerifyReportVerdict(report, content)

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
func initVerifyAdmission(declaredRequirements, declaredScenarios int) (VerifyAdmission, bool) {
	admission := VerifyAdmission{Schema: VerifyAdmissionSchema, Decision: "denied"}
	if declaredRequirements < 0 && declaredScenarios < 0 {
		return admission, true
	}
	if declaredRequirements >= 0 && declaredScenarios >= 0 {
		admission.Requirements.Declared = &declaredRequirements
		admission.Scenarios.Declared = &declaredScenarios
		return admission, true
	}
	admission.Reason = "declared requirement and scenario counts must be provided together"
	return admission, false
}

func admissionExtractReport(text string) (*VerifyReport, string, bool) {
	yamlRe := regexp.MustCompile("(?s)```yaml\\s+(.*?)```")
	matches := yamlRe.FindStringSubmatch(text)
	if len(matches) < 2 {
		return nil, "verify report: missing YAML envelope (```yaml ... ```)", false
	}
	report, err := parseYAMLEnvelope(matches[1])
	if err != nil {
		return nil, "verify report: parse envelope: " + err.Error(), false
	}
	if report.Schema != VerifyResultSchema && report.Schema != BiggzVerifyResultSchema {
		return nil, fmt.Sprintf("verify report: unknown schema %q", report.Schema), false
	}
	return report, "", true
}

func admissionCheckCounts(report *VerifyReport, admission *VerifyAdmission) bool {
	counted, ok := parseCount(report.Requirements)
	if !ok {
		admission.Reason = fmt.Sprintf("verify report: invalid requirements count %q", report.Requirements)
		return false
	}
	admission.Requirements.Counted = counted
	if admission.Requirements.Declared != nil && counted != *admission.Requirements.Declared {
		admission.Reason = fmt.Sprintf("verify report: requirements count %d does not match authoritative %d", counted, *admission.Requirements.Declared)
		return false
	}
	counted, ok = parseCount(report.Scenarios)
	if !ok {
		admission.Reason = fmt.Sprintf("verify report: invalid scenarios count %q", report.Scenarios)
		return false
	}
	admission.Scenarios.Counted = counted
	if admission.Scenarios.Declared != nil && counted != *admission.Scenarios.Declared {
		admission.Reason = fmt.Sprintf("verify report: scenarios count %d does not match authoritative %d", counted, *admission.Scenarios.Declared)
		return false
	}
	return true
}

func admissionCheckVerdict(report *VerifyReport, text string, admission *VerifyAdmission) bool {
	switch report.Verdict {
	case "pass", "pass_with_warnings":
	case "fail":
		admission.Reason = fmt.Sprintf("verify report: verdict is FAIL (%d blockers, %d critical)", report.Blockers, report.CriticalFindings)
		return false
	default:
		admission.Reason = fmt.Sprintf("verify report: unknown verdict %q", report.Verdict)
		return false
	}
	if strings.Contains(text, "**CRITICAL**:") && !strings.Contains(text, "**CRITICAL**: None") {
		admission.Reason = "verify report: contains unaddressed CRITICAL issues"
		return false
	}
	return true
}

func ValidateVerifyReportAdmission(content []byte, declaredRequirements, declaredScenarios int) VerifyAdmission {
	admission, ok := initVerifyAdmission(declaredRequirements, declaredScenarios)
	if !ok {
		return admission
	}
	text := string(content)
	report, reason, ok := admissionExtractReport(text)
	if !ok {
		admission.Reason = reason
		return admission
	}
	if !admissionCheckCounts(report, &admission) {
		return admission
	}
	if !admissionCheckVerdict(report, text, &admission) {
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

// anchoredVerifyReportPath returns the repository-relative verify-report path
// when the report is canonically anchored under the workspace and repo.
//
// This is the biggz port of gentle's canonicalVerifyReportPaths anchoring
// (91919996+765e46c1): every anchor is canonicalized through the same
// filepath.Abs+EvalSymlinks+Clean the change root was derived from, and the
// comparison itself is pure slash-form with platform-aware case folding via
// filepath.Rel("a","A") probe. Callers that have repo/workspace/changeRoot
// context should prefer this over a raw filepath.Join.
func anchoredVerifyReportPath(repo, workspace, changeRoot, change string) (string, error) {
	return canonicalVerifyReportPaths(repo, workspace, changeRoot, change)
}

// ─── Complexity Debt Report ────────────────────────────────────────────────

const (
	debtCyclomaticThreshold = 15
	debtCognitiveThreshold  = 20
)

var debtCriticalRoots = []string{
	"internal/review",
	"internal/sdd",
	"internal/verification",
}

// DebtOffender is one function exceeding debt thresholds.
type DebtOffender struct {
	Package    string `json:"package"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Function   string `json:"function"`
	Cyclomatic int    `json:"cyclomatic"`
	Cognitive  int    `json:"cognitive"`
}

// DebtPackageReport holds per-package debt totals and top offenders.
type DebtPackageReport struct {
	Package              string
	TotalFuncs           int
	CyclomaticViolations int
	CognitiveViolations  int
	TopOffenders         []DebtOffender
	TestOffenders        []DebtOffender
}

// CollectComplexityDebt scans critical packages and returns per-package reports.
// It is CostQuick/ReadOnly and excludes *_test.go from blocking counts (informational only).
func CollectComplexityDebt() (map[string]*DebtPackageReport, error) {
	return CollectComplexityDebtForRoots(debtCriticalRoots)
}

// CollectComplexityDebtForRoots scans the given roots and returns per-package reports. Exported for testing.
func CollectComplexityDebtForRoots(roots []string) (map[string]*DebtPackageReport, error) {
	reports := make(map[string]*DebtPackageReport)
	for _, root := range roots {
		reports[root] = &DebtPackageReport{Package: root}
	}
	for _, root := range roots {
		if err := collectForRoot(root, reports[root]); err != nil {
			return nil, err
		}
	}
	return reports, nil
}

// collectForRoot scans a single root directory and populates the report.
// It delegates per-file analysis to DebtForFile and sorts the results.
func collectForRoot(root string, report *DebtPackageReport) error {
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if isDebtSkippedDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isGoFile(path) {
			return nil
		}
		return DebtForFile(path, report)
	})
	if err != nil {
		return err
	}
	sortDebtOffenders(report)
	return nil
}

// isDebtSkippedDir reports whether the directory should be skipped during debt scanning.
func isDebtSkippedDir(base string) bool {
	return base == "vendor" || base == "testdata" || strings.HasPrefix(base, ".") || strings.HasPrefix(base, "_")
}

// isGoFile reports whether path is a Go source file.
func isGoFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".go")
}

// DebtForFile analyzes a single Go file for complexity violations and updates the report.
// It is exported for testing and for the required DebtForFile helper.
func DebtForFile(path string, report *DebtPackageReport) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil
	}
	isTest := strings.HasSuffix(path, "_test.go")
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		debtAnalyzeFunc(fn, fset, path, report, isTest)
	}
	return nil
}

// debtForFile is an internal alias for DebtForFile to satisfy lower-case naming.
func debtForFile(path string, report *DebtPackageReport) error {
	return DebtForFile(path, report)
}

// debtAnalyzeFunc evaluates a single function for debt and records offenders.
func debtAnalyzeFunc(fn *ast.FuncDecl, fset *token.FileSet, path string, report *DebtPackageReport, isTest bool) {
	report.TotalFuncs++
	cyclo := gocyclo.Complexity(fn)
	cog := gocognit.Complexity(fn)
	isCycloViol := cyclo > debtCyclomaticThreshold
	isCogViol := cog > debtCognitiveThreshold
	if isCycloViol {
		report.CyclomaticViolations++
	}
	if isCogViol {
		report.CognitiveViolations++
	}
	if !isCycloViol && !isCogViol {
		return
	}
	off := DebtOffender{
		Package:    filepath.ToSlash(filepath.Dir(path)),
		File:       filepath.ToSlash(path),
		Line:       fset.Position(fn.Pos()).Line,
		Function:   debtFuncName(fn),
		Cyclomatic: cyclo,
		Cognitive:  cog,
	}
	if isTest {
		report.TestOffenders = append(report.TestOffenders, off)
	} else {
		report.TopOffenders = append(report.TopOffenders, off)
	}
}

// sortDebtOffenders sorts top offenders by max complexity descending and caps to 10.
func sortDebtOffenders(report *DebtPackageReport) {
	slices.SortFunc(report.TopOffenders, func(a, b DebtOffender) int {
		ma := max(a.Cyclomatic, a.Cognitive)
		mb := max(b.Cyclomatic, b.Cognitive)
		if ma != mb {
			return cmp.Compare(mb, ma)
		}
		if a.File != b.File {
			return cmp.Compare(a.File, b.File)
		}
		return cmp.Compare(a.Line, b.Line)
	})
	if len(report.TopOffenders) > 10 {
		report.TopOffenders = report.TopOffenders[:10]
	}
}

func debtFuncName(fn *ast.FuncDecl) string {
	if fn.Recv != nil && fn.Recv.NumFields() > 0 {
		typ := fn.Recv.List[0].Type
		switch t := typ.(type) {
		case *ast.StarExpr:
			if ident, ok := t.X.(*ast.Ident); ok {
				return fmt.Sprintf("(%s).%s", "*"+ident.Name, fn.Name.Name)
			}
		case *ast.Ident:
			return fmt.Sprintf("(%s).%s", t.Name, fn.Name.Name)
		}
		return fmt.Sprintf("(%T).%s", typ, fn.Name.Name)
	}
	return fn.Name.Name
}

func debtHasViolations(reports map[string]*DebtPackageReport, roots []string) bool {
	for _, root := range roots {
		r := reports[root]
		if r.CyclomaticViolations > 0 || r.CognitiveViolations > 0 {
			return true
		}
	}
	return false
}

func writeDebtZeroSummary(sb *strings.Builder, reports map[string]*DebtPackageReport, roots []string) {
	totalFuncs, totalCyclo, totalCog := 0, 0, 0
	for _, root := range roots {
		r := reports[root]
		totalFuncs += r.TotalFuncs
		totalCyclo += r.CyclomaticViolations
		totalCog += r.CognitiveViolations
	}
	sb.WriteString(fmt.Sprintf("0 violations across %d functions scanned (cyclomatic >%d: %d, cognitive >%d: %d)\n\n", totalFuncs, debtCyclomaticThreshold, totalCyclo, debtCognitiveThreshold, totalCog))
	for _, root := range roots {
		r := reports[root]
		sb.WriteString(fmt.Sprintf("- %s: %d functions scanned, %d cyclomatic violations, %d cognitive violations, 0 top offenders\n", r.Package, r.TotalFuncs, r.CyclomaticViolations, r.CognitiveViolations))
	}
}

func writeDebtOffenders(sb *strings.Builder, offenders []DebtOffender) {
	if len(offenders) == 0 {
		sb.WriteString("- Top offenders: none\n")
		return
	}
	sb.WriteString("- Top 10 offenders (sorted by max complexity descending):\n")
	for _, o := range offenders {
		sb.WriteString(fmt.Sprintf("  - %s:%d %s cyclomatic=%d cognitive=%d\n", o.File, o.Line, o.Function, o.Cyclomatic, o.Cognitive))
	}
}

func writeDebtTestOffenders(sb *strings.Builder, offenders []DebtOffender) {
	if len(offenders) == 0 {
		return
	}
	sb.WriteString(fmt.Sprintf("- Informational test file violations: %d (never block)\n", len(offenders)))
	for _, o := range offenders {
		sb.WriteString(fmt.Sprintf("  - %s:%d %s cyclomatic=%d cognitive=%d (test)\n", o.File, o.Line, o.Function, o.Cyclomatic, o.Cognitive))
	}
}

func writeDebtPackageSection(sb *strings.Builder, r *DebtPackageReport) {
	sb.WriteString(fmt.Sprintf("### %s\n", r.Package))
	sb.WriteString(fmt.Sprintf("- Total functions scanned: %d\n", r.TotalFuncs))
	sb.WriteString(fmt.Sprintf("- Cyclomatic violations (>%d): %d\n", debtCyclomaticThreshold, r.CyclomaticViolations))
	sb.WriteString(fmt.Sprintf("- Cognitive violations (>%d): %d\n", debtCognitiveThreshold, r.CognitiveViolations))
	writeDebtOffenders(sb, r.TopOffenders)
	writeDebtTestOffenders(sb, r.TestOffenders)
	sb.WriteString("\n")
}

// ComplexityDebtMarkdownForRoots renders debt markdown for arbitrary roots (testing).
func ComplexityDebtMarkdownForRoots(roots []string) (string, error) {
	reports, err := CollectComplexityDebtForRoots(roots)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	sb.WriteString("## Complexity Debt\n\n")
	if !debtHasViolations(reports, roots) {
		writeDebtZeroSummary(&sb, reports, roots)
		return sb.String(), nil
	}
	for _, root := range roots {
		writeDebtPackageSection(&sb, reports[root])
	}
	return sb.String(), nil
}

// ComplexityDebtMarkdown returns a markdown section for verify-report.md.
// It lists per-package totals (scanned, violations by threshold) and top 10 offenders per package
// sorted by max complexity descending. *_test.go findings are informational only.
func ComplexityDebtMarkdown() (string, error) {
	reports, err := CollectComplexityDebt()
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	sb.WriteString("## Complexity Debt\n\n")
	if !debtHasViolations(reports, debtCriticalRoots) {
		writeDebtZeroSummary(&sb, reports, debtCriticalRoots)
		return sb.String(), nil
	}
	for _, root := range debtCriticalRoots {
		writeDebtPackageSection(&sb, reports[root])
	}
	return sb.String(), nil
}
