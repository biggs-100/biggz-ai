// Package components provides deployable component implementations that
// wrap install-package functions behind a uniform Component interface.
// Each component knows how to deploy itself given a plugin.AgentAdapter.
package components

import (
	"context"

	"github.com/biggz-ai/biggz/plugin"
)

// DeploymentResult describes what happened during a component deploy.
type DeploymentResult struct {
	Changed bool
	Files   []string
}

// Component is the interface for a deployable unit. Each implementation
// wraps a specific install-package function (e.g. DeploySkills, DeployConfig)
// and adapts it to the plugin.AgentAdapter interface.
type Component interface {
	// ID returns the unique component identifier (e.g. "skills", "config").
	ID() string

	// Deploy deploys this component for the given agent adapter.
	// Returns a DeploymentResult describing what changed, or an error.
	Deploy(ctx context.Context, adapter plugin.AgentAdapter) (*DeploymentResult, error)
}
