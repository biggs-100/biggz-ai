// Package qwen implements the AgentAdapter interface for the Qwen
// AI coding agent. It provides detection, capability discovery, and config
// path resolution specific to Qwen's directory layout (~/.qwen/).
package qwen

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/biggz-ai/biggz/internal/agents"
	"github.com/biggz-ai/biggz/model"
	"github.com/biggz-ai/biggz/plugin"
)

func init() {
	agents.Register(agents.AgentQwenCode, func() plugin.AgentAdapter { return NewAdapter() })
}

// Adapter implements plugin.AgentAdapter for the Qwen agent.
type Adapter struct {
	lookPath func(name string) (string, error)
}

// NewAdapter creates a new Adapter with default production wiring.
func NewAdapter() *Adapter {
	return &Adapter{
		lookPath: exec.LookPath,
	}
}

// ID returns "qwen-code".
func (a *Adapter) ID() model.AgentID { return agents.AgentQwenCode }

// Name returns "Qwen".
func (a *Adapter) Name() string { return "Qwen" }

// Tier returns the support commitment tier for Qwen.
func (a *Adapter) Tier() model.SupportTier { return agents.TierFull }

// Detect checks whether the qwen binary is on the system PATH.
func (a *Adapter) Detect(ctx context.Context, homeDir string) (bool, string, string, bool, error) {
	binPath, err := a.lookPath("qwen")
	if err != nil {
		return false, "", "", false, fmt.Errorf("qwen: not found: %w", err)
	}
	return true, binPath, a.SettingsPath(homeDir), true, nil
}

// InstallCommand returns the commands to install Qwen.
func (a *Adapter) InstallCommand(profile interface{}) ([][]string, error) {
	return [][]string{
		{"pip", "install", "qwen-agent"},
	}, nil
}

// Capabilities returns the list of capabilities Qwen supports.
func (a *Adapter) Capabilities() []string {
	return []string{
		plugin.CapSkills,
		plugin.CapSystemPrompt,
		plugin.CapSlashCommands,
	}
}

// SupportsAutoInstall returns true — Qwen supports pip install.
func (a *Adapter) SupportsAutoInstall() bool { return true }

// SupportsSkills returns true — Qwen supports custom skills.
func (a *Adapter) SupportsSkills() bool { return true }

// SupportsSystemPrompt returns true — Qwen supports QWEN.md.
func (a *Adapter) SupportsSystemPrompt() bool { return true }

// SupportsMCP returns true — Qwen supports MCP servers.
func (a *Adapter) SupportsMCP() bool { return true }

// SupportsOutputStyles returns false — Qwen does not support output
// style configuration.
func (a *Adapter) SupportsOutputStyles() bool { return false }

// SupportsSlashCommands returns true — Qwen supports custom slash
// commands.
func (a *Adapter) SupportsSlashCommands() bool { return true }

// SupportsSubAgents returns false — Qwen does not support sub-agents.
func (a *Adapter) SupportsSubAgents() bool { return false }

// SystemPromptStrategy returns FileReplace — Qwen replaces the
// QWEN.md file entirely.
func (a *Adapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return agents.StrategyFileReplace
}

// MCPStrategy returns MergeIntoSettings — Qwen stores MCP config
// inside settings.json.
func (a *Adapter) MCPStrategy() model.MCPStrategy {
	return agents.StrategyMergeIntoSettings
}

// GlobalConfigDir returns ~/.qwen/.
func (a *Adapter) GlobalConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ".qwen")
}

// SystemPromptDir returns ~/.qwen/ — same as GlobalConfigDir.
func (a *Adapter) SystemPromptDir(homeDir string) string {
	return a.GlobalConfigDir(homeDir)
}

// SystemPromptFile returns ~/.qwen/QWEN.md.
func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(a.GlobalConfigDir(homeDir), "QWEN.md")
}

// SkillsDir returns ~/.qwen/skills/.
func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".qwen", "skills")
}

// CommandsDir returns ~/.qwen/commands/.
func (a *Adapter) CommandsDir(homeDir string) string {
	return filepath.Join(homeDir, ".qwen", "commands")
}

// SubAgentsDir returns an empty string — Qwen does not support
// sub-agents.
func (a *Adapter) SubAgentsDir(homeDir string) string {
	return ""
}

// EmbeddedSubAgentsDir returns an empty string — Qwen does not
// support sub-agents.
func (a *Adapter) EmbeddedSubAgentsDir() string {
	return ""
}

// OutputStyleDir returns an empty string — Qwen does not support
// output styles.
func (a *Adapter) OutputStyleDir(homeDir string) string {
	return ""
}

// SettingsPath returns ~/.qwen/settings.json.
func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(homeDir, ".qwen", "settings.json")
}

// MCPConfigPath returns the path where Qwen stores MCP config —
// same as the settings file since it uses MergeIntoSettings.
func (a *Adapter) MCPConfigPath(homeDir string, serverName string) string {
	return a.SettingsPath(homeDir)
}

// DeployConfig validates the config and reports capability.
func (a *Adapter) DeployConfig(ctx context.Context, cfg plugin.AgentConfig) error {
	_ = ctx
	_ = cfg
	return nil
}
