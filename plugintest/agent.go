package plugintest

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugin"
)

// FakeAgent is an AgentAdapter for testing.
// It simulates a detected AI coding agent with configurable capabilities.
type FakeAgent struct {
	// Identity
	AgentID   model.AgentID
	AgentName string

	// Installation
	Installed         bool
	BinaryPath        string
	ConfigPath        string
	AutoInstall       bool
	AgentCapabilities []string

	// Feature toggles (nil = default false)
	AgentAutoInstall    *bool
	AgentSkills         *bool
	AgentSystemPrompt   *bool
	AgentMCP            *bool
	AgentOutputStyles   *bool
	AgentSlashCommands  *bool
	AgentSubAgents      *bool

	// Strategy overrides
	AgentMCPStrategy          model.MCPStrategy
	AgentSystemPromptStrategy model.SystemPromptStrategy

	// Temp dir for path methods
	tempDir string

	// Error injectors
	InjectDetectError error
}

// ID returns the unique identifier for this agent adapter.
func (a *FakeAgent) ID() model.AgentID {
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

// Tier returns the support commitment tier.
func (a *FakeAgent) Tier() model.SupportTier {
	return model.TierFull
}

// Detect checks whether this agent is installed.
func (a *FakeAgent) Detect(ctx context.Context, homeDir string) (bool, string, string, bool, error) {
	if a.InjectDetectError != nil {
		return false, "", "", false, a.InjectDetectError
	}
	if !a.Installed {
		return false, "", "", false, fmt.Errorf("fake-agent: %q not found", a.ID())
	}
	binPath := a.BinaryPath
	if binPath == "" {
		binPath = "/usr/local/bin/" + string(a.ID())
	}
	configPath := a.ConfigPath
	if configPath == "" {
		configPath = a.SettingsPath(homeDir)
	}
	return true, binPath, configPath, a.AutoInstall, nil
}

// InstallCommand returns the install commands for this fake agent.
func (a *FakeAgent) InstallCommand(profile interface{}) ([][]string, error) {
	return [][]string{
		{"echo", "install", string(a.ID())},
	}, nil
}

// Capabilities returns the capabilities this agent supports.
func (a *FakeAgent) Capabilities() []string {
	if a.AgentCapabilities == nil {
		return []string{plugin.CapSkills, plugin.CapMCP}
	}
	return a.AgentCapabilities
}

// SupportsAutoInstall returns whether this fake agent supports auto-install.
func (a *FakeAgent) SupportsAutoInstall() bool {
	if a.AgentAutoInstall != nil {
		return *a.AgentAutoInstall
	}
	return a.AutoInstall
}

// SupportsSkills returns whether this agent supports skills.
func (a *FakeAgent) SupportsSkills() bool {
	if a.AgentSkills != nil {
		return *a.AgentSkills
	}
	return false
}

// SupportsSystemPrompt returns whether this agent supports system prompts.
func (a *FakeAgent) SupportsSystemPrompt() bool {
	if a.AgentSystemPrompt != nil {
		return *a.AgentSystemPrompt
	}
	return false
}

// SupportsMCP returns whether this agent supports MCP.
func (a *FakeAgent) SupportsMCP() bool {
	if a.AgentMCP != nil {
		return *a.AgentMCP
	}
	return false
}

// SupportsOutputStyles returns whether this agent supports output styles.
func (a *FakeAgent) SupportsOutputStyles() bool {
	if a.AgentOutputStyles != nil {
		return *a.AgentOutputStyles
	}
	return false
}

// SupportsSlashCommands returns whether this agent supports slash commands.
func (a *FakeAgent) SupportsSlashCommands() bool {
	if a.AgentSlashCommands != nil {
		return *a.AgentSlashCommands
	}
	return false
}

// SupportsSubAgents returns whether this agent supports sub-agents.
func (a *FakeAgent) SupportsSubAgents() bool {
	if a.AgentSubAgents != nil {
		return *a.AgentSubAgents
	}
	return false
}

// SystemPromptStrategy returns the configured system prompt strategy.
func (a *FakeAgent) SystemPromptStrategy() model.SystemPromptStrategy {
	return a.AgentSystemPromptStrategy
}

// MCPStrategy returns the configured MCP strategy.
func (a *FakeAgent) MCPStrategy() model.MCPStrategy {
	return a.AgentMCPStrategy
}

// GlobalConfigDir returns the agent's global config directory.
func (a *FakeAgent) GlobalConfigDir(homeDir string) string {
	if a.tempDir != "" {
		return a.tempDir
	}
	return filepath.Join(homeDir, ".config", "opencode")
}

// SystemPromptDir returns the system prompt directory.
func (a *FakeAgent) SystemPromptDir(homeDir string) string {
	return a.GlobalConfigDir(homeDir)
}

// SystemPromptFile returns the system prompt file path.
func (a *FakeAgent) SystemPromptFile(homeDir string) string {
	return filepath.Join(a.GlobalConfigDir(homeDir), "AGENTS.md")
}

// SkillsDir returns the agent's skills directory.
func (a *FakeAgent) SkillsDir(homeDir string) string {
	if a.tempDir != "" {
		return filepath.Join(a.tempDir, "skills")
	}
	return filepath.Join(homeDir, ".config", "opencode", "skills")
}

// CommandsDir returns the agent's commands directory.
func (a *FakeAgent) CommandsDir(homeDir string) string {
	if a.tempDir != "" {
		return filepath.Join(a.tempDir, "commands")
	}
	return filepath.Join(homeDir, ".config", "opencode", "commands")
}

// SubAgentsDir returns the agent's sub-agents directory.
func (a *FakeAgent) SubAgentsDir(homeDir string) string {
	return ""
}

// EmbeddedSubAgentsDir returns the relative embedded sub-agents path.
func (a *FakeAgent) EmbeddedSubAgentsDir() string {
	return ""
}

// OutputStyleDir returns the agent's output style directory.
func (a *FakeAgent) OutputStyleDir(homeDir string) string {
	return ""
}

// SettingsPath returns the agent's settings file path.
func (a *FakeAgent) SettingsPath(homeDir string) string {
	if a.tempDir != "" {
		return filepath.Join(a.tempDir, "opencode.jsonc")
	}
	return filepath.Join(homeDir, ".config", "opencode", "opencode.jsonc")
}

// MCPConfigPath returns the path where this agent stores MCP config.
func (a *FakeAgent) MCPConfigPath(homeDir string, serverName string) string {
	return a.SettingsPath(homeDir)
}

// DeployConfig simulates deploying configuration to the agent.
func (a *FakeAgent) DeployConfig(ctx context.Context, cfg plugin.AgentConfig) error {
	if !a.Installed && a.InjectDetectError == nil {
		return fmt.Errorf("fake-agent: cannot deploy config, %q not installed", a.ID())
	}
	return nil
}

// SetTempDir sets a temporary directory root for all config path methods.
func (a *FakeAgent) SetTempDir(dir string) {
	a.tempDir = dir
}
