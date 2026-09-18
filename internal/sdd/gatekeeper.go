// Package sdd — Auto Gatekeeper validates completed SDD phases before launching the next.
//
// The gatekeeper runs AFTER every phase completes and BEFORE launching the next sub-agent.
// It is autonomous validation — it does NOT ask the user; it only surfaces to the user
// when it catches a problem.
//
// Checks performed:
//   - Contract conformance: phase returned required fields (status, executive_summary, artifacts, etc.)
//   - Artifact existence: the completed phase's canonical artifact exists under the active store
//   - No hallucination: declared paths resolve (repo-relative or change-relative)
//   - No drift from inputs: output is consistent with phase's required inputs
//   - Routing coherence: next_recommended matches the successor the dependency
//     authority resolves for the change
package sdd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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
	Status           string        `json:"status"`
	ExecutiveSummary string        `json:"executive_summary"`
	Artifacts        []ArtifactRef `json:"artifacts"`
	NextRecommended  string        `json:"next_recommended"`
	Risks            []RiskRef     `json:"risks,omitempty"`
	SkillResolution  string        `json:"skill_resolution,omitempty"`
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

// Gatekeeper validates a completed phase's result before launching the next phase.
// It reads the actual artifacts from disk and validates against the phase contract.
//
// Parameters:
//   - openspecRoot: the openspec/ directory root (e.g., "/path/to/project/openspec")
//   - changeName: the SDD change name (e.g., "add-dark-mode")
//   - completedPhase: the phase that just completed (e.g., "spec")
//   - store: the active artifact store (openspec, hybrid, or "" for none, as
//     produced by ResolvePreflightPrefs); never inferred from the result
//   - result: the phase's declared result (parsed from sub-agent output)
//
// Returns a GatekeeperResult with passed=true if all checks pass, or passed=false
// with specific reasons for the orchestrator to act on.
func Gatekeeper(openspecRoot, changeName, completedPhase string, store ArtifactStore, result *PhaseResult) *GatekeeperResult {
	gr := &GatekeeperResult{
		Passed: true,
		Phase:  completedPhase,
	}
	store = normalizeGatekeeperStore(store)

	// 1. Contract conformance
	gr.checkContract(result)

	// 2. Artifact existence (canonical artifact per store)
	gr.checkArtifacts(openspecRoot, changeName, completedPhase, store, result)

	// 3. No hallucination (declared paths must resolve)
	gr.checkNoHallucination(openspecRoot, changeName, result)

	// 4. No drift from inputs
	gr.checkNoDrift(openspecRoot, changeName, completedPhase, result)

	// 5. Routing coherence (against the dependency authority, not a static table)
	gr.checkRouting(openspecRoot, changeName, completedPhase, store, result)

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

// normalizeGatekeeperStore maps the raw store input onto the normalized
// preflight values (openspec, hybrid, or "" for none), so the gatekeeper
// accepts exactly what ResolvePreflightPrefs produces.
func normalizeGatekeeperStore(store ArtifactStore) ArtifactStore {
	return ArtifactStore(NormalizePreflightArtifactStore(string(store)))
}

// checkArtifacts validates that the completed phase's canonical artifact
// exists under the active store (design D4). The canonical artifact decides
// pass/fail; declared paths are never accepted as proof of existence. Store
// "" expects no filesystem artifact and skips with an explicit reason.
func (gr *GatekeeperResult) checkArtifacts(openspecRoot, changeName, completedPhase string, store ArtifactStore, result *PhaseResult) {
	check := GatekeeperCheck{Name: "artifact_existence", Passed: true}

	if result == nil || len(result.Artifacts) == 0 {
		check.Skipped = true
		gr.Details = append(gr.Details, check)
		return
	}

	if store == "" {
		check.Passed = false
		check.Skipped = true
		check.Reason = "artifact store is none: no filesystem artifact is expected, so the canonical artifact cannot be reported as passed"
		gr.Details = append(gr.Details, check)
		return
	}
	if store != ArtifactStoreOpenSpec && store != ArtifactStoreHybrid {
		check.Passed = false
		check.Reason = fmt.Sprintf("unknown artifact store %q: the canonical artifact for phase %q cannot be resolved", string(store), completedPhase)
		gr.Details = append(gr.Details, check)
		return
	}

	changeDir := filepath.Join(openspecRoot, "changes", changeName)
	existing, expected, defined := canonicalArtifacts(changeDir, store, completedPhase)
	if !defined {
		check.Passed = false
		check.Skipped = true
		check.Reason = fmt.Sprintf("no canonical artifact location is defined for phase %q; declared paths are not accepted as proof", completedPhase)
		gr.Details = append(gr.Details, check)
		return
	}
	if len(existing) == 0 {
		check.Passed = false
		check.Reason = fmt.Sprintf("missing canonical artifact for phase %q: expected %s", completedPhase, strings.Join(expected, " or "))
		gr.Details = append(gr.Details, check)
		return
	}
	check.Reason = "canonical artifact resolved: " + strings.Join(existing, ", ")
	gr.Details = append(gr.Details, check)
}

// canonicalArtifacts resolves the completed phase's canonical artifact under
// store: existing holds the paths that exist (resolveArtifactPaths is the
// single canonical resolver for openspec/hybrid) and expected names the
// absolute locations to report when nothing exists yet. defined is false for
// phases without a canonical artifact slot (explore, archive).
func canonicalArtifacts(changeDir string, store ArtifactStore, phase string) (existing, expected []string, defined bool) {
	resolved := resolveArtifactPaths(changeDir, store)
	plan := filepath.Join(changeDir, "plan.md")
	switch phase {
	case "propose":
		return resolved.Proposal, []string{filepath.Join(changeDir, "proposal.md"), plan}, true
	case "spec":
		return resolved.Specs, []string{filepath.Join(changeDir, "specs", "*", "spec.md"), plan}, true
	case "design":
		return resolved.Design, []string{filepath.Join(changeDir, "design.md"), plan}, true
	case "tasks":
		return resolved.Tasks, []string{filepath.Join(changeDir, "tasks.md"), plan}, true
	case "apply":
		return resolved.ApplyProgress, []string{filepath.Join(changeDir, "apply-progress.md")}, true
	case "verify":
		return resolved.VerifyReport, []string{filepath.Join(changeDir, "verify-report.md")}, true
	default:
		return nil, nil, false
	}
}

// checkNoHallucination validates that declared artifact paths resolve against
// the workspace root (repo-relative) or the change dir (change-relative).
// BigMem topic keys, PR references and URLs are declarations, not filesystem
// paths, and are never stat'ed as files.
func (gr *GatekeeperResult) checkNoHallucination(openspecRoot, changeName string, result *PhaseResult) {
	check := GatekeeperCheck{Name: "no_hallucination", Passed: true}

	if result == nil {
		check.Skipped = true
		gr.Details = append(gr.Details, check)
		return
	}

	workspaceRoot := filepath.Dir(openspecRoot)
	changeDir := filepath.Join(openspecRoot, "changes", changeName)
	var hallucinated []string

	for _, art := range result.Artifacts {
		if !isFilesystemArtifactDeclaration(changeName, art.Path) {
			continue
		}
		if _, ok := resolveDeclaredArtifactPath(workspaceRoot, changeDir, art.Path); !ok {
			hallucinated = append(hallucinated, art.Path)
		}
	}

	if len(hallucinated) > 0 {
		check.Passed = false
		check.Reason = fmt.Sprintf("hallucinated paths (referenced but do not exist): %s", strings.Join(hallucinated, ", "))
	}

	gr.Details = append(gr.Details, check)
}

// isFilesystemArtifactDeclaration reports whether a declared artifact names a
// filesystem location. Empty paths, BigMem topic keys of this change, PR
// references and URLs are declarations but not paths.
func isFilesystemArtifactDeclaration(changeName, decl string) bool {
	if decl == "" || strings.HasPrefix(decl, "#") || strings.Contains(decl, "://") {
		return false
	}
	return !isBigMemTopicRef(changeName, decl)
}

// isBigMemTopicRef reports whether a declaration names a BigMem topic of this
// change (sdd/{change}/…, optionally prefixed with bigmem:), which the engram
// and hybrid stores use instead of filesystem paths.
func isBigMemTopicRef(changeName, decl string) bool {
	trimmed := strings.TrimPrefix(decl, "bigmem:")
	return strings.HasPrefix(trimmed, "sdd/"+changeName+"/")
}

// resolveDeclaredArtifactPath resolves one declared artifact path against the
// workspace root (repo-relative) and then the change directory
// (change-relative); absolute declarations resolve to themselves. The second
// result reports whether the declaration matched an existing path.
func resolveDeclaredArtifactPath(workspaceRoot, changeDir, decl string) (string, bool) {
	if decl == "" {
		return "", false
	}
	if filepath.IsAbs(decl) {
		if _, err := os.Stat(decl); err == nil {
			return decl, true
		}
		return "", false
	}
	for _, base := range []string{workspaceRoot, changeDir} {
		candidate := filepath.Join(base, filepath.FromSlash(decl))
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
	}
	return "", false
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
		if !dirHasArtifactPattern(changeDir, patterns) {
			missing = append(missing, prereq)
		}
	}

	if len(missing) > 0 {
		check.Passed = false
		check.Reason = fmt.Sprintf("prerequisite artifacts missing or empty: %s", strings.Join(missing, ", "))
	}

	gr.Details = append(gr.Details, check)
}

// dirHasArtifactPattern reports whether changeDir contains a non-trivial file
// matching any of the patterns. Extracted from checkNoDrift to keep it
// under the cognitive complexity budget.
func dirHasArtifactPattern(changeDir string, patterns []string) bool {
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		found := false
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
			return true
		}
	}
	return false
}

// checkRouting validates that next_recommended matches the successor the
// dependency authority resolves for the change (design D2, D5): the declared
// label is normalized, terminal done/empty passes before resolution, and any
// other label must equal the resolved successor or the check fails naming the
// expected phase. Phases outside the routing contract skip with an explicit
// reason (D3); underivable dependency state fails closed naming the cause
// (D4) — it is never reported as skipped.
func (gr *GatekeeperResult) checkRouting(openspecRoot, changeName, completedPhase string, store ArtifactStore, result *PhaseResult) {
	check := GatekeeperCheck{Name: "routing_coherence", Passed: true}

	if result == nil {
		check.Skipped = true
		gr.Details = append(gr.Details, check)
		return
	}

	if !isRoutingContractPhase(completedPhase) {
		check.Skipped = true
		check.Reason = fmt.Sprintf("phase %q is outside the routing contract; no dependency successor is defined", completedPhase)
		gr.Details = append(gr.Details, check)
		return
	}

	declared := normalizeRoutingLabel(result.NextRecommended)
	if declared == "" || declared == "done" {
		gr.Details = append(gr.Details, check)
		return
	}

	successor, err := resolveRoutingSuccessor(openspecRoot, changeName, store)
	if err != nil {
		check.Passed = false
		check.Reason = fmt.Sprintf("cannot resolve the dependency successor for change %q: %v", changeName, err)
		gr.Details = append(gr.Details, check)
		return
	}
	if declared != successor {
		check.Passed = false
		check.Reason = fmt.Sprintf("next_recommended %q is not the dependency successor of %q: expected %q",
			result.NextRecommended, completedPhase, successor)
	}

	gr.Details = append(gr.Details, check)
}

// resolveRoutingSuccessor returns the successor the dependency authority
// resolves for the change under the active store (design D2). OpenSpec and
// hybrid changes with a filesystem record derive through
// readChangeWithForcedStore + the forced-store derivation (the same authority
// sdd-status uses); a hybrid change without a filesystem record derives from
// its BigMem topics. An absent store and an unknown store fail closed, and a
// missing record names its absolute change directory (design D4).
func resolveRoutingSuccessor(openspecRoot, changeName string, store ArtifactStore) (string, error) {
	changeDir := filepath.Join(openspecRoot, "changes", changeName)
	workspaceRoot := filepath.Dir(openspecRoot)

	switch store {
	case "":
		return "", errors.New("artifact store is none: no artifact store is active for this run")
	case ArtifactStoreOpenSpec, ArtifactStoreHybrid:
		if _, err := os.Stat(changeDir); err == nil {
			cs, err := readChangeWithForcedStore(changeDir, changeName, false, workspaceRoot, false, store)
			if err != nil {
				return "", err
			}
			return cs.NextRecommended, nil
		}
		if store == ArtifactStoreOpenSpec {
			return "", fmt.Errorf("no record under store %q: %s", string(store), changeDir)
		}
		successor, found, err := bigMemRoutingSuccessor(workspaceRoot, changeName)
		if err != nil {
			return "", err
		}
		if !found {
			return "", fmt.Errorf("no record under store %q: %s; no BigMem topics under sdd/%s/", string(store), changeDir, changeName)
		}
		return successor, nil
	default:
		return "", fmt.Errorf("unknown artifact store %q", string(store))
	}
}

// bigMemRoutingSuccessor resolves the dependency successor for a hybrid
// change without a filesystem record (design D2). Changes derive through
// deriveBigMemChangeStatus, the same authority the engram/hybrid status path
// uses. found reports whether any BigMem topic exists for the change.
func bigMemRoutingSuccessor(workspaceRoot, changeName string) (successor string, found bool, err error) {
	active, archived, err := collectBigMemChangesWithArchiveCtx(context.Background(), workspaceRoot, false)
	if err != nil {
		return "", false, err
	}
	for _, cs := range slices.Concat(active, archived) {
		if cs.Name == changeName {
			return cs.NextRecommended, true, nil
		}
	}
	return "", false, nil
}

// normalizeRoutingLabel trims a declared successor and strips its progress
// suffix, so "apply (3/5 tasks)" normalizes to "apply" (design D5).
func normalizeRoutingLabel(declared string) string {
	label := strings.TrimSpace(declared)
	if before, _, found := strings.Cut(label, " ("); found {
		label = before
	}
	return label
}

// isRoutingContractPhase reports whether a completed phase has a dependency
// successor defined by the routing contract. validPhases enumerates exactly
// those SDD phases; research, sync and any other phase outside it have no
// successor to resolve and skip with an explicit reason (design D3).
func isRoutingContractPhase(phase string) bool {
	return slices.Contains(validPhases, phase)
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
func GatekeeperFromJSON(openspecRoot, changeName, completedPhase string, store ArtifactStore, resultJSON string) (*GatekeeperResult, error) {
	result, err := ParsePhaseResult(resultJSON)
	if err != nil {
		return &GatekeeperResult{
			Passed:  false,
			Phase:   completedPhase,
			Reasons: []string{fmt.Sprintf("failed to parse phase result: %v", err)},
		}, nil
	}
	return Gatekeeper(openspecRoot, changeName, completedPhase, store, result), nil
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
