package doctor

import (
	"context"
	"fmt"

	"github.com/biggs-100/biggz-ai/internal/git"
)

const StaleBranchesCheckID CheckID = "stale-branches"

// StaleBranchesCheck reports stale [gone] branches as INFO, never fail.
type StaleBranchesCheck struct {
	cwd    string
	listFn func(context.Context, string) ([]git.GoneBranch, error)
}

// NewStaleBranchesCheck creates a check using the default git helper.
func NewStaleBranchesCheck() *StaleBranchesCheck {
	return &StaleBranchesCheck{
		cwd:    ".",
		listFn: git.ListGoneBranches,
	}
}

// NewStaleBranchesCheckWithCustom creates a check with injected list function for testing.
func NewStaleBranchesCheckWithCustom(cwd string, listFn func(context.Context, string) ([]git.GoneBranch, error)) *StaleBranchesCheck {
	return &StaleBranchesCheck{cwd: cwd, listFn: listFn}
}

// ID returns the check identifier.
func (c *StaleBranchesCheck) ID() CheckID { return StaleBranchesCheckID }

// Run lists gone branches and reports count as INFO, never failing.
func (c *StaleBranchesCheck) Run(ctx context.Context) *Result {
	fn := c.listFn
	if fn == nil {
		fn = git.ListGoneBranches
	}
	cwd := c.cwd
	if cwd == "" {
		cwd = "."
	}
	branches, err := fn(ctx, cwd)
	if err != nil {
		return &Result{
			ID:       StaleBranchesCheckID,
			Status:   StatusPass,
			Message:  "staleBranches: 0 (git unavailable)",
			Severity: SeverityInfo,
			Error:    err.Error(),
			Details:  map[string]int{"staleBranches": 0},
		}
	}
	n := len(branches)
	return &Result{
		ID:       StaleBranchesCheckID,
		Status:   StatusPass,
		Message:  fmt.Sprintf("staleBranches: %d", n),
		Severity: SeverityInfo,
		Details:  map[string]int{"staleBranches": n},
	}
}

// Remedy returns nil — stale branch cleanup is via biggz cleanup.
func (c *StaleBranchesCheck) Remedy() *Remedy { return nil }
