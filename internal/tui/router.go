// Package tui wizard router: linear 10-stage order for the installer wizard.
//
// The router is pure navigation state — no rendering, no animation, no style
// tokens. It mirrors the install.go wizard step enum without importing the
// screens package (screens imports tui's parent would cycle), so stage order
// is defined once here and consumed via NextStage/PrevStage.
package tui

import "os"

// WizardStage is one step of the linear installer wizard.
type WizardStage int

// Wizard stages in traversal order.
const (
	StageWelcome WizardStage = iota
	StageDetection
	StageAgents
	StagePersona
	StagePreset
	StageDepTree
	StageSkillPicker
	StageReview
	StageInstalling
	StageComplete
)

// wizardOrder fixes the linear traversal order.
var wizardOrder = []WizardStage{
	StageWelcome,
	StageDetection,
	StageAgents,
	StagePersona,
	StagePreset,
	StageDepTree,
	StageSkillPicker,
	StageReview,
	StageInstalling,
	StageComplete,
}

// Route describes the linear neighbours of a wizard stage.
type Route struct {
	Forward    WizardStage
	Backward   WizardStage
	HasForward bool
	HasBack    bool
}

// linearRoutes maps every stage to its forward/backward neighbours.
// Only adjacent moves are representable; out-of-order jumps are rejected
// by NextStage/PrevStage returning ok=false.
var linearRoutes = func() map[WizardStage]Route {
	routes := make(map[WizardStage]Route, len(wizardOrder))
	for i, s := range wizardOrder {
		r := Route{}
		if i+1 < len(wizardOrder) {
			r.Forward = wizardOrder[i+1]
			r.HasForward = true
		}
		if i-1 >= 0 {
			r.Backward = wizardOrder[i-1]
			r.HasBack = true
		}
		routes[s] = r
	}
	return routes
}()

// NextStage returns the stage after s, or ok=false at Complete / unknown stage.
func NextStage(s WizardStage) (next WizardStage, ok bool) {
	r, known := linearRoutes[s]
	if !known || !r.HasForward {
		return s, false
	}
	return r.Forward, true
}

// PrevStage returns the stage before s, or ok=false at Welcome / unknown stage.
func PrevStage(s WizardStage) (prev WizardStage, ok bool) {
	r, known := linearRoutes[s]
	if !known || !r.HasBack {
		return s, false
	}
	return r.Backward, true
}

// LegacyInstall reports whether the lean 6-state installer flow is forced
// via BIGGZ_LEGACY_INSTALL=1. When true, install.go keeps Idle→Detect→
// Select→Review→Running→Done and skips the wizard stages.
func LegacyInstall() bool {
	return os.Getenv("BIGGZ_LEGACY_INSTALL") == "1"
}
