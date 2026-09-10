// Package sdd — pre-SDD route check.
//
// EvaluateRoute answers ONE question before implementation starts: should the
// orchestrator recommend SDD (explicit question + STOP) or proceed direct?
// It encodes the SDD recommendation triggers from the orchestrator delegation
// contract as a deterministic function so agents run the check instead of
// eyeballing estimates.
//
// Trigger semantics (disambiguated):
//   - Hard triggers ask unconditionally: new public interface, new domain,
//     new external dependency, golden tests, visual theme, full UI rewrite,
//     critical packages, cross-cutting change, or ambiguous "done".
//   - Scale triggers (>400 lines, 5+ files, 2+ non-trivial files) ask UNLESS
//     the carve-out holds: acceptance clear AND single domain AND
//     verification planned (the quiet-tools precedent: 7 files, clear
//     acceptance, tests green — direct was correct).
//   - Ambiguity trigger: unclear acceptance asks once the work is
//     substantial (>100 lines, 3+ files, or any non-trivial file); tiny
//     unclear items stay direct (clarify inline, not via SDD ceremony).
package sdd

import (
	"fmt"
	"strings"
)

// RouteDecision is the pre-SDD routing verdict.
type RouteDecision string

const (
	// RouteDirect means proceed without SDD (inline or delegated direct).
	RouteDirect RouteDecision = "direct"
	// RouteAskSDD means MUST recommend SDD via explicit question + STOP.
	// SDD is still never auto-launched; a decline proceeds direct.
	RouteAskSDD RouteDecision = "ask-sdd"
)

// Scale thresholds mirrored from the delegation contract.
const (
	RouteScaleLines      = 400
	RouteScaleFiles      = 5
	RouteScaleNontrivial = 2
	RouteAmbiguousLines  = 100
	RouteAmbiguousFiles  = 3
)

// RouteInput carries the orchestrator's pre-implementation estimates.
// Booleans default to false (unknown/absent); counts default to 0.
type RouteInput struct {
	EstimatedLines  int
	EstimatedFiles  int
	NontrivialFiles int
	// AcceptanceClear means acceptance criteria are known and verifiable.
	AcceptanceClear bool
	// SingleDomain means one domain area, no cross-cutting behavior.
	SingleDomain bool
	// VerificationPlanned means tests/verification run with the change.
	VerificationPlanned bool
	// Hard triggers (each asks unconditionally).
	NewPublicInterface bool
	NewDomain          bool
	NewDependency      bool
	TouchesGolden      bool
	TouchesTheme       bool
	UIRewrite          bool
	CriticalPackages   bool
	CrossCutting       bool
	AmbiguousDone      bool
}

// RouteVerdict is the deterministic routing outcome.
type RouteVerdict struct {
	Route    RouteDecision `json:"route"`
	Triggers []string      `json:"triggers,omitempty"`
	Reason   string        `json:"reason"`
}

// clearAndVerified reports the scale carve-out: acceptance clear, single
// domain, verification planned.
func (in RouteInput) clearAndVerified() bool {
	return in.AcceptanceClear && in.SingleDomain && in.VerificationPlanned
}

// substantial reports whether unclear work is big enough for SDD ceremony.
func (in RouteInput) substantial() bool {
	return in.EstimatedLines > RouteAmbiguousLines ||
		in.EstimatedFiles >= RouteAmbiguousFiles ||
		in.NontrivialFiles >= 1
}

// EvaluateRoute applies the trigger list deterministically. Matched trigger
// codes are stable identifiers for tests and receipts.
func EvaluateRoute(in RouteInput) RouteVerdict {
	var triggers []string
	hard := []struct {
		code string
		hit  bool
	}{
		{"new-public-interface", in.NewPublicInterface},
		{"new-domain", in.NewDomain},
		{"new-dependency", in.NewDependency},
		{"golden-tests", in.TouchesGolden},
		{"visual-theme", in.TouchesTheme},
		{"ui-rewrite", in.UIRewrite},
		{"critical-packages", in.CriticalPackages},
		{"cross-cutting", in.CrossCutting},
		{"ambiguous-done", in.AmbiguousDone},
	}
	for _, h := range hard {
		if h.hit {
			triggers = append(triggers, h.code)
		}
	}
	if !in.clearAndVerified() {
		if in.EstimatedLines > RouteScaleLines {
			triggers = append(triggers, "scale-lines")
		}
		if in.EstimatedFiles >= RouteScaleFiles {
			triggers = append(triggers, "scale-files")
		}
		if in.NontrivialFiles >= RouteScaleNontrivial {
			triggers = append(triggers, "scale-nontrivial")
		}
	}
	if !in.AcceptanceClear && in.substantial() {
		triggers = append(triggers, "unclear-acceptance")
	}
	if len(triggers) == 0 {
		return RouteVerdict{Route: RouteDirect, Reason: "no SDD triggers matched; proceed direct"}
	}
	return RouteVerdict{
		Route:    RouteAskSDD,
		Triggers: triggers,
		Reason:   fmt.Sprintf("recommend SDD via explicit question + STOP (triggers: %s); a decline proceeds direct without nagging", strings.Join(triggers, ", ")),
	}
}
