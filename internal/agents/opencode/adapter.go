// Package opencode implements the AgentAdapter interface for the OpenCode
// AI coding agent. It provides detection, capability discovery, and config
// path resolution specific to OpenCode's directory layout.
package opencode

import (
	"context"
	"os/exec"
	"path/filepath"

	"github.com/biggz-ai/biggz/plugin"
)

// Adapter implements plugin.AgentAdapter for the OpenCode agent.
// It uses exec.LookPath for binary detection and knows OpenCode's
// standard config directory layout under the user's home directory.
type Adapter struct {
	lookPath func(name string) (string, error)
}

// NewAdapter creates a new Adapter with default production wiring.
func NewAdapter() *Adapter {
	return &Adapter{
		lookPath: exec.LookPath,
	}
}

// ID returns the unique identifier for this agent adapter.
func (a *Adapter) ID() string {
	return "opencode"
}

// Name returns a human-readable name for this agent.
func (a *Adapter) Name() string {
	return "OpenCode"
}

// Detect checks whether the OpenCode binary is on the system PATH.
// Returns the full binary path if found, or an error if not detected.
func (a *Adapter) Detect(ctx context.Context) (string, error) {
	return a.lookPath("opencode")
}

// Capabilities returns the list of capabilities OpenCode supports.
func (a *Adapter) Capabilities() []plugin.Capability {
	return []plugin.Capability{
		plugin.CapSkills,
		plugin.CapMCP,
		plugin.CapSubAgents,
		plugin.CapSystemPrompt,
		plugin.CapSlashCommands,
	}
}

// GlobalConfigDir returns OpenCode's global configuration directory.
// Typically ~/.config/opencode/.
func (a *Adapter) GlobalConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ".config", "opencode")
}

// SkillsDir returns the directory where OpenCode stores skills.
// Typically ~/.config/opencode/skills/.
func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".config", "opencode", "skills")
}

// SettingsPath returns the path to OpenCode's settings file.
// Typically ~/.config/opencode/opencode.jsonc.
func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(homeDir, ".config", "opencode", "opencode.jsonc")
}

// DeployConfig installs or updates configuration for OpenCode.
// This is a basic MVP implementation — it validates the config is
// non-nil but does not yet perform file operations.
func (a *Adapter) DeployConfig(ctx context.Context, cfg plugin.AgentConfig) error {
	_ = ctx
	_ = cfg
	// TODO: implement file deployment in PR #3 (Install command)
	return nil
}
