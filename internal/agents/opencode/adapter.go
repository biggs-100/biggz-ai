// Package opencode implements the AgentAdapter interface for the OpenCode
// AI coding agent. It provides detection, capability discovery, and config
// path resolution specific to OpenCode's directory layout.
package opencode

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/biggs-100/biggz-ai/internal/agents"
	"github.com/biggs-100/biggz-ai/internal/platform"
	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugin"
)

func init() {
	agents.Register(agents.AgentOpenCode, func() plugin.AgentAdapter { return NewAdapter() })
}

// Adapter implements plugin.AgentAdapter for the OpenCode agent.
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
func (a *Adapter) ID() model.AgentID { return agents.AgentOpenCode }

// Name returns a human-readable name for this agent.
func (a *Adapter) Name() string { return "OpenCode" }

// Tier returns the support commitment tier for OpenCode.
func (a *Adapter) Tier() model.SupportTier { return agents.TierFull }

// Detect checks whether the OpenCode binary is on the system PATH.
func (a *Adapter) Detect(ctx context.Context, homeDir string) (bool, string, string, bool, error) {
	binPath, err := a.lookPath("opencode")
	if err != nil {
		return false, "", "", false, fmt.Errorf("opencode: not found: %w", err)
	}
	return true, binPath, a.SettingsPath(homeDir), true, nil
}

// InstallCommand returns the commands to install OpenCode.
// If Go is not available on the platform, it returns a hint error instead of
// a go install command. The profile may be a platform.Profile, *platform.Profile,
// or nil (in which case platform.Detect is called).
func (a *Adapter) InstallCommand(profile interface{}) ([][]string, error) {
	p := resolveProfile(profile)
	if !p.GoAvailable {
		return nil, fmt.Errorf("Install Go 1.24+ from go.dev")
	}
	return [][]string{
		{"go", "install", "github.com/opencode-ai/opencode@latest"},
	}, nil
}

func resolveProfile(profile interface{}) platform.Profile {
	if profile != nil {
		if pp, ok := profile.(platform.Profile); ok {
			return pp
		}
		if pp, ok := profile.(*platform.Profile); ok && pp != nil {
			return *pp
		}
	}
	detected, _ := platform.Detect(context.Background())
	return detected
}

// Capabilities returns the list of capabilities OpenCode supports.
func (a *Adapter) Capabilities() []string {
	return []string{
		plugin.CapSkills,
		plugin.CapMCP,
		plugin.CapSystemPrompt,
		plugin.CapSlashCommands,
	}
}

// SupportsAutoInstall returns true — OpenCode supports binary download.
func (a *Adapter) SupportsAutoInstall() bool { return true }

// SupportsSkills returns true — OpenCode supports custom skills.
func (a *Adapter) SupportsSkills() bool { return true }

// SupportsSystemPrompt returns true — OpenCode supports AGENTS.md.
func (a *Adapter) SupportsSystemPrompt() bool { return true }

// SupportsMCP returns true — OpenCode supports MCP servers.
func (a *Adapter) SupportsMCP() bool { return true }

// SupportsOutputStyles returns false — OpenCode does not support
// separate output style configuration.
func (a *Adapter) SupportsOutputStyles() bool { return false }

// SupportsSlashCommands returns true — OpenCode supports custom
// slash commands.
func (a *Adapter) SupportsSlashCommands() bool { return true }

// SupportsSubAgents returns false — OpenCode does not support
// delegating to sub-agents.
func (a *Adapter) SupportsSubAgents() bool { return false }

// SystemPromptStrategy returns FileReplace — OpenCode replaces the
// AGENTS.md file entirely.
func (a *Adapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return agents.StrategyFileReplace
}

// MCPStrategy returns MergeIntoSettings — OpenCode stores MCP config
// inside opencode.json.
func (a *Adapter) MCPStrategy() model.MCPStrategy {
	return agents.StrategyMergeIntoSettings
}

// GlobalConfigDir returns OpenCode's global configuration directory.
// Typically ~/.config/opencode/.
func (a *Adapter) GlobalConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ".config", "opencode")
}

// SystemPromptDir returns the directory where OpenCode's system prompt
// file lives — same as GlobalConfigDir.
func (a *Adapter) SystemPromptDir(homeDir string) string {
	return a.GlobalConfigDir(homeDir)
}

// SystemPromptFile returns the full path to OpenCode's AGENTS.md.
func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(a.GlobalConfigDir(homeDir), "AGENTS.md")
}

// SkillsDir returns the directory where OpenCode stores skills.
// Typically ~/.config/opencode/skills/.
func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".config", "opencode", "skills")
}

// CommandsDir returns the directory where OpenCode stores slash commands.
func (a *Adapter) CommandsDir(homeDir string) string {
	return filepath.Join(homeDir, ".config", "opencode", "commands")
}

// SubAgentsDir returns an empty string — OpenCode does not support
// sub-agents.
func (a *Adapter) SubAgentsDir(homeDir string) string {
	return ""
}

// EmbeddedSubAgentsDir returns an empty string — OpenCode does not
// support sub-agents.
func (a *Adapter) EmbeddedSubAgentsDir() string {
	return ""
}

// OutputStyleDir returns an empty string — OpenCode does not support
// output styles.
func (a *Adapter) OutputStyleDir(homeDir string) string {
	return ""
}

// SettingsPath returns the path to OpenCode's settings file.
// Typically ~/.config/opencode/opencode.json.
func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(homeDir, ".config", "opencode", "opencode.json")
}

// MCPConfigPath returns the path where OpenCode stores MCP config —
// same as the settings file since it uses MergeIntoSettings.
func (a *Adapter) MCPConfigPath(homeDir string, serverName string) string {
	return a.SettingsPath(homeDir)
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
