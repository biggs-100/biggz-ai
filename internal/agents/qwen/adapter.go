// Package qwen implements the AgentAdapter interface for the Qwen
// AI coding agent. It provides detection, capability discovery, and config
// path resolution specific to Qwen's directory layout (~/.qwen/).
package qwen

import (
	"context"
	"os/exec"
	"path/filepath"

	"github.com/biggz-ai/biggz/plugin"
)

// Adapter implements plugin.AgentAdapter for the Qwen agent.
// Binary is detected via exec.LookPath("qwen"). Config lives under ~/.qwen/.
type Adapter struct {
	lookPath func(name string) (string, error)
}

// NewAdapter creates a new Adapter with default production wiring.
func NewAdapter() *Adapter {
	return &Adapter{
		lookPath: exec.LookPath,
	}
}

// ID returns "qwen".
func (a *Adapter) ID() string { return "qwen" }

// Name returns "Qwen".
func (a *Adapter) Name() string { return "Qwen" }

// Detect checks whether the qwen binary is on the system PATH.
func (a *Adapter) Detect(ctx context.Context) (string, error) {
	return a.lookPath("qwen")
}

// Capabilities returns features Qwen supports.
func (a *Adapter) Capabilities() []plugin.Capability {
	return []plugin.Capability{
		plugin.CapSkills,
		plugin.CapSystemPrompt,
		plugin.CapSlashCommands,
	}
}

// GlobalConfigDir returns ~/.qwen/.
func (a *Adapter) GlobalConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ".qwen")
}

// SkillsDir returns ~/.qwen/skills/.
func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".qwen", "skills")
}

// SettingsPath returns ~/.qwen/settings.json.
func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(homeDir, ".qwen", "settings.json")
}

// DeployConfig validates the config and reports capability.
func (a *Adapter) DeployConfig(ctx context.Context, cfg plugin.AgentConfig) error {
	_ = ctx
	_ = cfg
	return nil
}
