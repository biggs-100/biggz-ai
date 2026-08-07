// Package plugintest provides reusable test implementations of plugin interfaces.
//
// It contains DummyLens (a LensPlugin that returns static analysis findings)
// and FakeAgent (an AgentAdapter for testing agent discovery scenarios).
package plugintest

import (
	"context"
	"fmt"

	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugin"
)

// DummyLens is a LensPlugin that returns a static finding.
// It is used as a test/development lens for CLI wiring validation.
type DummyLens struct{}

// ID returns the unique identifier for this lens.
func (l *DummyLens) ID() string { return "dummy-lens" }

// Name returns a human-readable name for this lens.
func (l *DummyLens) Name() string { return "Dummy Lens" }

// Version returns the version string for this lens.
func (l *DummyLens) Version() string { return "1.0.0" }

// Analyze runs the lens analysis against the given subject.
// It validates that the subject is non-empty (Repository must be set)
// and returns a LensResult with a single static finding.
func (l *DummyLens) Analyze(ctx context.Context, subject model.ReviewSubject) (*plugin.LensResult, error) {
	if subject.Repository == "" {
		return nil, fmt.Errorf("dummy-lens: subject repository is required")
	}

	return &plugin.LensResult{
		LensID: l.ID(),
		Findings: []plugin.Finding{
			{
				ID:       "dummy-001",
				Severity: "info",
				Message:  "Dummy analysis complete",
				File:     subject.Repository,
				Line:     1,
			},
		},
	}, nil
}

// Policies returns the list of policies associated with this lens.
func (l *DummyLens) Policies() []plugin.Policy {
	return []plugin.Policy{
		{
			Name:        "minimum-evidence",
			Description: "Ensures at least one evidence entry exists",
		},
	}
}
