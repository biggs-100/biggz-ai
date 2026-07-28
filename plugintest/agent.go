package plugintest

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/biggz-ai/biggz/plugin"
)

// FakeAgent is an AgentAdapter for testing.
// It simulates a detected AI coding agent with configurable capabilities.
type FakeAgent struct {
	AgentID      string
	AgentName    string
	Installed    bool
	BinaryPath   string
	AgentCapabilities []plugin.Capability
	tempDir      string
}

// ID returns the unique identifier for this agent adapter.
func (a *FakeAgent) ID() string {
	if a.AgentID == "" {
		return "fake-agent"
	}
	return a.AgentID
}

// Name returns a human-readable name.
func (a *FakeAgent) Name() string {
	if a.AgentName == "" {
		return "Fake Agent"
	}
	return a.AgentName
}

// Detect checks whether this agent is installed.
func (a *FakeAgent) Detect(ctx context.Context) (string, error) {
	if !a.Installed {
		return "", fmt.Errorf("fake-agent: %q not found", a.ID())
	}
	path := a.BinaryPath
	if path == "" {
		path = "/usr/local/bin/" + a.ID()
	}
	return path, nil
}

// Capabilities returns the capabilities this agent supports.
func (a *FakeAgent) Capabilities() []plugin.Capability {
	if a.AgentCapabilities == nil {
		return []plugin.Capability{plugin.CapSkills, plugin.CapMCP}
	}
	return a.AgentCapabilities
}

// DeployConfig simulates deploying configuration to the agent.
func (a *FakeAgent) DeployConfig(ctx context.Context, cfg plugin.AgentConfig) error {
	if !a.Installed {
		return fmt.Errorf("fake-agent: cannot deploy config, %q not installed", a.ID())
	}
	return nil
}

// SetTempDir sets a temporary directory root for all config path methods.
// When set, GlobalConfigDir, SkillsDir, and SettingsPath resolve under
// this directory instead of using the provided homeDir.
func (a *FakeAgent) SetTempDir(dir string) {
	a.tempDir = dir
}

// GlobalConfigDir returns the agent's global config directory.
// If tempDir is set, returns it directly. Otherwise joins homeDir
// with the default config subpath.
func (a *FakeAgent) GlobalConfigDir(homeDir string) string {
	if a.tempDir != "" {
		return a.tempDir
	}
	return filepath.Join(homeDir, ".config", "opencode")
}

// SkillsDir returns the agent's skills directory.
// If tempDir is set, returns a "skills" subdirectory under it.
// Otherwise returns ~/.config/opencode/skills/.
func (a *FakeAgent) SkillsDir(homeDir string) string {
	if a.tempDir != "" {
		return filepath.Join(a.tempDir, "skills")
	}
	return filepath.Join(homeDir, ".config", "opencode", "skills")
}

// SettingsPath returns the agent's settings file path.
// If tempDir is set, returns opencode.jsonc under it.
// Otherwise returns ~/.config/opencode/opencode.jsonc.
func (a *FakeAgent) SettingsPath(homeDir string) string {
	if a.tempDir != "" {
		return filepath.Join(a.tempDir, "opencode.jsonc")
	}
	return filepath.Join(homeDir, ".config", "opencode", "opencode.jsonc")
}
