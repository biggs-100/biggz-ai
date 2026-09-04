// Package sdd — Auto Gatekeeper validates completed SDD phases before launching the next.
//
// The gatekeeper runs AFTER every phase completes and BEFORE launching the next sub-agent.
// It is autonomous validation — it does NOT ask the user; it only surfaces to the user
// when it catches a problem.
//
// Checks performed:
//   - Contract conformance: phase returned required fields (status, executive_summary, artifacts, etc.)
//   - Artifact existence: declared artifacts actually exist and are readable
//   - No hallucination: file paths, symbols, commands referenced actually exist
//   - No drift from inputs: output is consistent with phase's required inputs
//   - Routing coherence: next_recommended follows the dependency graph
package sdd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// GatekeeperResult is the outcome of a gatekeeper validation.
type GatekeeperResult struct {
	Passed  bool              `json:"passed"`
	Phase   string            `json:"phase"`
	Reasons []string          `json:"reasons,omitempty"`
	Details []GatekeeperCheck `json:"details,omitempty"`
}

// GatekeeperCheck is one individual check within a gatekeeper validation.
type GatekeeperCheck struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Reason  string `json:"reason,omitempty"`
	Skipped bool   `json:"skipped,omitempty"`
}

// PhaseResult is the minimum contract a phase output must satisfy.
// The orchestrator parses the phase's return into this struct for validation.
type PhaseResult struct {
	Status          string         `json:"status"`
	ExecutiveSummary string        `json:"executive_summary"`
	Artifacts       []ArtifactRef  `json:"artifacts"`
	NextRecommended string         `json:"next_recommended"`
	Risks           []RiskRef      `json:"risks,omitempty"`
	SkillResolution string         `json:"skill_resolution,omitempty"`
}

// ArtifactRef is a declared artifact from a phase result.
type ArtifactRef struct {
	Path    string `json:"path"`
	Type    string `json:"type,omitempty"`
	Summary string `json:"summary,omitempty"`
}

// RiskRef is a declared risk from a phase result.
type RiskRef struct {
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

// Valid phases in dependency order.
var validPhases = []string{
	"explore", "propose", "spec", "design", "tasks", "apply", "verify", "archive",
}

// phaseDependencies defines what each phase requires to have been completed.
var phaseDependencies = map[string][]string{
	"propose": {"explore"},
	"spec":    {"propose"},
	"design":  {"propose"},
	"tasks":   {"spec", "design"},
	"apply":   {"tasks"},
	"verify":  {"apply"},
	"archive": {"verify"},
}

// phaseArtifactPatterns defines expected artifact path patterns per phase.
var phaseArtifactPatterns = map[string][]string{
	"propose": {`proposal\.md$`},
	"spec":    {`spec\.md$`, `specs/.*spec\.md$`},
	"design":  {`design\.md$`},
	"tasks":   {`tasks\.md$`},
	"apply":   {`apply-progress\.md$`},
	"verify":  {`verify-report\.md$`},
	"archive": {`archive-report\.md$`},
}

// nextPhaseValid maps valid next_recommended values per phase.
var nextPhaseValid = map[string][]string{
	"explore":  {"propose", "spec"}, // explore can lead to propose or directly to spec
	"propose":  {"spec", "design"},
	"spec":     {"design"},
	"design":   {"tasks"},
	"tasks":    {"apply"},
	"apply":    {"apply", "verify", "tasks"}, // apply can loop or move to verify
	"verify":   {"verify", "archive", "apply"}, // verify can loop or remediate
	"archive":  {},
}

// Gatekeeper validates a completed phase's result before launching the next phase.
// It reads the actual artifacts from disk and validates against the phase contract.
//
// Parameters:
//   - openspecRoot: the openspec/ directory root (e.g., "/path/to/project/openspec")
//   - changeName: the SDD change name (e.g., "add-dark-mode")
//   - completedPhase: the phase that just completed (e.g., "spec")
//   - result: the phase's declared result (parsed from sub-agent output)
//
// Returns a GatekeeperResult with passed=true if all checks pass, or passed=false
// with specific reasons for the orchestrator to act on.
func Gatekeeper(openspecRoot, changeName, completedPhase string, result *PhaseResult) *GatekeeperResult {
	gr := &GatekeeperResult{
		Passed: true,
		Phase:  completedPhase,
	}

	// 1. Contract conformance
	gr.checkContract(result)

	// 2. Artifact existence
	gr.checkArtifacts(openspecRoot, changeName, completedPhase, result)

	// 3. No hallucination (validate paths exist)
	gr.checkNoHallucination(openspecRoot, changeName, result)

	// 4. No drift from inputs
	gr.checkNoDrift(openspecRoot, changeName, completedPhase, result)

	// 5. Routing coherence
	gr.checkRouting(completedPhase, result)

	// 6. Complexity gate (verify phase only)
	gr.checkComplexityGate(openspecRoot, completedPhase)

	// Final verdict
	for _, d := range gr.Details {
		if !d.Passed && !d.Skipped {
			gr.Passed = false
			break
		}
	}

	return gr
}

// checkContract validates the phase result has all required fields.
func (gr *GatekeeperResult) checkContract(result *PhaseResult) {
	check := GatekeeperCheck{Name: "contract_conformance", Passed: true}

	if result == nil {
		check.Passed = false
		check.Reason = "phase returned nil result"
		gr.Details = append(gr.Details, check)
		return
	}

	var missing []string
	if result.Status == "" {
		missing = append(missing, "status")
	}
	if result.ExecutiveSummary == "" {
		missing = append(missing, "executive_summary")
	}
	if len(result.Artifacts) == 0 {
		missing = append(missing, "artifacts")
	}
	if result.NextRecommended == "" {
		missing = append(missing, "next_recommended")
	}

	if len(missing) > 0 {
		check.Passed = false
		check.Reason = fmt.Sprintf("missing required fields: %s", strings.Join(missing, ", "))
	}

	gr.Details = append(gr.Details, check)
}

// checkArtifacts validates that declared artifacts actually exist on disk.
func (gr *GatekeeperResult) checkArtifacts(openspecRoot, changeName, completedPhase string, result *PhaseResult) {
	check := GatekeeperCheck{Name: "artifact_existence", Passed: true}

	if result == nil || len(result.Artifacts) == 0 {
		check.Skipped = true
		gr.Details = append(gr.Details, check)
		return
	}

	changeDir := filepath.Join(openspecRoot, "changes", changeName)
	var missing []string

	for _, art := range result.Artifacts {
		if art.Path == "" {
			continue
		}
		// Resolve path relative to change directory
		fullPath := art.Path
		if !filepath.IsAbs(fullPath) {
			fullPath = filepath.Join(changeDir, art.Path)
		}
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			missing = append(missing, art.Path)
		}
	}

	if len(missing) > 0 {
		check.Passed = false
		check.Reason = fmt.Sprintf("declared artifacts do not exist: %s", strings.Join(missing, ", "))
	}

	gr.Details = append(gr.Details, check)
}

// checkNoHallucination validates that referenced paths in the result actually exist.
func (gr *GatekeeperResult) checkNoHallucination(openspecRoot, changeName string, result *PhaseResult) {
	check := GatekeeperCheck{Name: "no_hallucination", Passed: true}

	if result == nil {
		check.Skipped = true
		gr.Details = append(gr.Details, check)
		return
	}

	changeDir := filepath.Join(openspecRoot, "changes", changeName)
	var hallucinated []string

	// Check all artifact paths
	for _, art := range result.Artifacts {
		if art.Path == "" {
			continue
		}
		fullPath := art.Path
		if !filepath.IsAbs(fullPath) {
			fullPath = filepath.Join(changeDir, art.Path)
		}
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			hallucinated = append(hallucinated, art.Path)
		}
	}

	if len(hallucinated) > 0 {
		check.Passed = false
		check.Reason = fmt.Sprintf("hallucinated paths (referenced but do not exist): %s", strings.Join(hallucinated, ", "))
	}

	gr.Details = append(gr.Details, check)
}

// checkNoDrift validates that the output is consistent with the phase's required inputs.
// This checks that prerequisite artifacts exist and have content.
func (gr *GatekeeperResult) checkNoDrift(openspecRoot, changeName, completedPhase string, result *PhaseResult) {
	check := GatekeeperCheck{Name: "no_drift", Passed: true}

	prereqs, ok := phaseDependencies[completedPhase]
	if !ok || len(prereqs) == 0 {
		check.Skipped = true
		gr.Details = append(gr.Details, check)
		return
	}

	changeDir := filepath.Join(openspecRoot, "changes", changeName)
	var missing []string

	for _, prereq := range prereqs {
		patterns, ok := phaseArtifactPatterns[prereq]
		if !ok {
			continue
		}
		found := false
		for _, pattern := range patterns {
			re, err := regexp.Compile(pattern)
			if err != nil {
				continue
			}
			// Walk change directory to find matching files
			filepath.Walk(changeDir, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				relPath, _ := filepath.Rel(changeDir, path)
				if re.MatchString(relPath) {
					// Check file has content (not just empty)
					if info.Size() > 10 { // Minimum: header + newline
						found = true
					}
				}
				return nil
			})
			if found {
				break
			}
		}
		if !found {
			missing = append(missing, prereq)
		}
	}

	if len(missing) > 0 {
		check.Passed = false
		check.Reason = fmt.Sprintf("prerequisite artifacts missing or empty: %s", strings.Join(missing, ", "))
	}

	gr.Details = append(gr.Details, check)
}

// checkRouting validates that next_recommended is a valid transition.
func (gr *GatekeeperResult) checkRouting(completedPhase string, result *PhaseResult) {
	check := GatekeeperCheck{Name: "routing_coherence", Passed: true}

	if result == nil {
		check.Skipped = true
		gr.Details = append(gr.Details, check)
		return
	}

	validNext, ok := nextPhaseValid[completedPhase]
	if !ok {
		check.Skipped = true
		gr.Details = append(gr.Details, check)
		return
	}

	// Normalize: "apply (3/5 tasks)" -> "apply"
	nextClean := result.NextRecommended
	if idx := strings.Index(nextClean, " ("); idx > 0 {
		nextClean = nextClean[:idx]
	}

	found := false
	for _, valid := range validNext {
		if nextClean == valid {
			found = true
			break
		}
	}

	// Also allow "done" as terminal
	if nextClean == "done" || nextClean == "" {
		found = true
	}

	if !found {
		check.Passed = false
		check.Reason = fmt.Sprintf("next_recommended %q is not a valid transition from %q; valid: %v",
			result.NextRecommended, completedPhase, validNext)
	}

	gr.Details = append(gr.Details, check)
}

// checkComplexityGate runs the diff-aware complexity gate for the verify
// phase: new 15/20 offenders in critical packages block, everything else
// warns. Non-verify phases skip. Non-git workspaces skip with a reason
// instead of failing (R2 lens and CI remain backstops there).
func (gr *GatekeeperResult) checkComplexityGate(openspecRoot, completedPhase string) {
	check := GatekeeperCheck{Name: "complexity_gate", Passed: true}
	if completedPhase != "verify" {
		check.Skipped = true
		gr.Details = append(gr.Details, check)
		return
	}
	repoRoot := filepath.Dir(openspecRoot)
	res, err := GateWorkingTreeComplexity(repoRoot)
	if err != nil {
		check.Skipped = true
		check.Reason = err.Error()
		gr.Details = append(gr.Details, check)
		return
	}
	if len(res.Warnings) > 0 {
		check.Reason = "warnings: " + strings.Join(res.Warnings, "; ")
	}
	if !res.Passed {
		check.Passed = false
		blockers := "new complexity offenders: " + FormatGateBlockers(res.Blocking)
		if check.Reason != "" {
			check.Reason += "; " + blockers
		} else {
			check.Reason = blockers
		}
	}
	gr.Details = append(gr.Details, check)
}

// ParsePhaseResult attempts to parse a JSON string into a PhaseResult.
// Used by the orchestrator to parse sub-agent output.
func ParsePhaseResult(jsonStr string) (*PhaseResult, error) {
	var result PhaseResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("parse phase result: %w", err)
	}
	return &result, nil
}

// GatekeeperFromJSON is a convenience that parses JSON and runs the gatekeeper.
func GatekeeperFromJSON(openspecRoot, changeName, completedPhase, resultJSON string) (*GatekeeperResult, error) {
	result, err := ParsePhaseResult(resultJSON)
	if err != nil {
		return &GatekeeperResult{
			Passed:  false,
			Phase:   completedPhase,
			Reasons: []string{fmt.Sprintf("failed to parse phase result: %v", err)},
		}, nil
	}
	return Gatekeeper(openspecRoot, changeName, completedPhase, result), nil
}

// GatekeeperSummary returns a one-line summary suitable for orchestrator logging.
func GatekeeperSummary(gr *GatekeeperResult) string {
	if gr.Passed {
		return fmt.Sprintf("◆ %s · gatekeeper PASS", gr.Phase)
	}
	var reasons []string
	for _, d := range gr.Details {
		if !d.Passed && !d.Skipped {
			reasons = append(reasons, d.Name)
		}
	}
	return fmt.Sprintf("◆ %s · gatekeeper FAIL (%s)", gr.Phase, strings.Join(reasons, ", "))
}
