package review

import (
	"fmt"

	"github.com/biggz-ai/biggz/model"
)

// GateKind represents a validation point in the development workflow.
type GateKind string

const (
	GatePreCommit GateKind = "pre-commit"
	GatePrePush   GateKind = "pre-push"
	GateRelease   GateKind = "release"
)

// GateConfig defines the rules for a specific gate.
type GateConfig struct {
	Kind              GateKind
	RequireReceipt    bool // requires a valid receipt
	RequireNoFindings bool // requires zero CRITICAL/WARNING findings
	RequireApproved   bool // requires human approval
}

// DefaultGateConfigs returns the default configuration for each gate.
func DefaultGateConfigs() map[GateKind]GateConfig {
	return map[GateKind]GateConfig{
		GatePreCommit: {
			Kind:              GatePreCommit,
			RequireReceipt:    true,
			RequireNoFindings: false,
			RequireApproved:   false,
		},
		GatePrePush: {
			Kind:              GatePrePush,
			RequireReceipt:    true,
			RequireNoFindings: true,
			RequireApproved:   false,
		},
		GateRelease: {
			Kind:              GateRelease,
			RequireReceipt:    true,
			RequireNoFindings: true,
			RequireApproved:   true,
		},
	}
}

// GateResult describes the outcome of a gate validation.
type GateResult struct {
	Gate    GateKind `json:"gate"`
	Allowed bool     `json:"allowed"`
	Reason  string   `json:"reason,omitempty"`
}

// ValidateCheck runs a single gate check against the review state.
// It does not run lenses or modify state — it validates that conditions are met.
func ValidateCheck(kind GateKind, state *model.ReviewState, cfg GateConfig, receipt *Receipt) *GateResult {
	// Check receipt if required
	if cfg.RequireReceipt {
		if receipt == nil {
			return &GateResult{
				Gate:    kind,
				Allowed: false,
				Reason:  fmt.Sprintf("%s: no receipt found", kind),
			}
		}
		if !VerifyReceipt(receipt, state) {
			return &GateResult{
				Gate:    kind,
				Allowed: false,
				Reason:  fmt.Sprintf("%s: receipt does not match review state", kind),
			}
		}
	}

	// Check state
	if state.Status != model.StatusCompleted {
		return &GateResult{
			Gate:    kind,
			Allowed: false,
			Reason:  fmt.Sprintf("%s: review status is %s, expected completed", kind, state.Status),
		}
	}

	return &GateResult{
		Gate:    kind,
		Allowed: true,
		Reason:  fmt.Sprintf("%s: passed", kind),
	}
}
