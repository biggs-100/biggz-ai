package agents

import (
	"context"
	"os"

	"github.com/biggs-100/biggz-ai/model"
)

// InstalledAgent describes a detected AI coding agent on the system.
type InstalledAgent struct {
	// ID is the typed agent identifier from the catalog.
	ID model.AgentID

	// BinaryPath is the full path to the agent's executable binary.
	BinaryPath string

	// ConfigDir is the agent's global configuration directory, resolved
	// against the provided homeDir. This directory is checked for
	// existence on disk as evidence of installation.
	ConfigDir string
}

// DetectInstalled iterates all registered factories in the given Registry
// and returns every agent whose GlobalConfigDir exists on disk AND whose
// Detect() succeeds. Unlike the v1 behaviour (first-match), this returns
// ALL installed agents, enabling multi-agent workflows.
//
// The homeDir parameter is used to resolve the agent's config directory.
// It should typically be os.UserHomeDir().
func DetectInstalled(ctx context.Context, reg *Registry, homeDir string) []InstalledAgent {
	var found []InstalledAgent
	for _, entry := range reg.ListAll() {
		a, ok := reg.Get(model.AgentID(entry.ID))
		if !ok {
			continue
		}

		installed, binaryPath, _, _, err := a.Detect(ctx, homeDir)
		if err != nil || !installed {
			// Fallback: check if the GlobalConfigDir exists on disk
			configDir := a.GlobalConfigDir(homeDir)
			if _, statErr := os.Stat(configDir); statErr != nil {
				continue // not detected and no config dir
			}
			found = append(found, InstalledAgent{
				ID:         model.AgentID(entry.ID),
				BinaryPath: binaryPath,
				ConfigDir:  configDir,
			})
			continue
		}

		found = append(found, InstalledAgent{
			ID:         model.AgentID(entry.ID),
			BinaryPath: binaryPath,
			ConfigDir:  a.GlobalConfigDir(homeDir),
		})
	}
	return found
}
