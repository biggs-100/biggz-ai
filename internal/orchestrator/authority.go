package orchestrator

import (
	"fmt"
	"strings"
)

// SD Agent Authority guard — SDD phases MUST use sdd-* agents.
// Maps SDD phases to their canonical sdd-* agent; general/explore for SDD MUST be rejected fail-closed.
var sddPhaseToAgent = map[string]string{
	"propose":  "sdd-propose",
	"spec":     "sdd-spec",
	"design":   "sdd-design",
	"tasks":    "sdd-tasks",
	"apply":    "sdd-apply",
	"verify":   "sdd-verify",
	"archive":  "sdd-archive",
	"explore":  "sdd-explore",
	"research": "sdd-research",
}

// IsSDDPhase reports whether phase is an SDD phase that requires sdd-* agent.
func IsSDDPhase(phase string) bool {
	_, ok := sddPhaseToAgent[strings.ToLower(strings.TrimSpace(phase))]
	return ok
}

// ExpectedAgentForPhase returns the canonical sdd-* agent for an SDD phase, or empty if not SDD.
func ExpectedAgentForPhase(phase string) string {
	return sddPhaseToAgent[strings.ToLower(strings.TrimSpace(phase))]
}

// GuardSDAgentAuthority enforces SD Agent Authority: SDD phases must use sdd-* agents only.
// Returns error with "SD Agent Authority" when a forbidden agent (general/explore or non-sdd-*) is used for SDD.
// Non-SDD phases allow any agent (including general) — caller determines SDD context.
func GuardSDAgentAuthority(phase, agent string) error {
	phaseNorm := strings.ToLower(strings.TrimSpace(phase))
	agentNorm := strings.ToLower(strings.TrimSpace(agent))
	expected, isSDD := sddPhaseToAgent[phaseNorm]
	if !isSDD {
		// Non-SDD phase: general/worker allowed.
		return nil
	}
	// SDD phase must use sdd-* prefix.
	if strings.HasPrefix(agentNorm, "sdd-") {
		// Allow any sdd-* for this SDD change (strict mapping is ideal but prefix covers all phases).
		// Optionally enforce exact match: if caller expects strict, they can compare with ExpectedAgentForPhase.
		// Require that agent is at least sdd-<phase> or any sdd-*; keep fail-closed for general/explore only is min.
		// For stricter enforcement, reject mismatched sdd-* mapping if agent != expected and agent is known SDD agent.
		// Keep permissive: any sdd-* passes, but we still surface canonical expectation in error for non-sdd-.
		return nil
	}
	// Forbidden: general/explore or any non-sdd-* for SDD.
	_ = expected // captured for error message
	return fmt.Errorf("SD Agent Authority: SDD phase %q must use sdd-* agent (%q), got %q — general/explore forbidden for SDD", phase, expected, agent)
}

// ShouldSelectSDD implements Work Routing Ladder fail-closed rule (REQ-SDD-002 / REQ-ORCH-004).
// SDD is selected ONLY on explicit request (biggz sdd-new / sdd-continue or direct ask) or accepted proposal.
// Size, file count, or risk alone NEVER selects SDD. The 12-file / 800-line example must stay Simple Delegation.
// Returns true only when explicitRequest is true.
func ShouldSelectSDD(explicitRequest bool, fileCount, lineCount int) bool {
	_ = fileCount
	_ = lineCount
	return explicitRequest
}
