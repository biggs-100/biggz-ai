// Package agents provides the adapter system for AI agent discovery and interaction.
package agents

import "context"

// EffectiveCodeGraphWiringDetector is an optional interface that adapters can
// implement to provide semantic MCP wiring validation. When a detector is
// available, the installer uses it to verify that MCP servers are correctly
// wired into the agent's configuration before marking a deployment complete.
type EffectiveCodeGraphWiringDetector interface {
	// EffectiveCodeGraphWiring returns the path to the resolved MCP wiring
	// configuration and whether the agent reports it as properly configured.
	EffectiveCodeGraphWiring(ctx context.Context, homeDir string) (path string, configured bool)
}
