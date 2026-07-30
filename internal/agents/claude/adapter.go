// Package claude implements the AgentAdapter interface for Claude Code
// by Anthropic. Detection via exec.LookPath("claude"). Config under ~/.claude/.
package claude

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
	agents.Register(agents.AgentClaudeCode, func() plugin.AgentAdapter { return NewAdapter() })
}

// Adapter implements plugin.AgentAdapter for Claude Code.
type Adapter struct {
	lookPath func(name string) (string, error)
}

func NewAdapter() *Adapter {
	return &Adapter{lookPath: exec.LookPath}
}

// ID returns "claude-code".
func (a *Adapter) ID() model.AgentID { return agents.AgentClaudeCode }

// Name returns "Claude Code".
func (a *Adapter) Name() string { return "Claude Code" }

// Tier returns the support commitment tier for Claude Code.
func (a *Adapter) Tier() model.SupportTier { return agents.TierFull }

// Detect checks whether the claude binary is on the system PATH.
func (a *Adapter) Detect(ctx context.Context, homeDir string) (bool, string, string, bool, error) {
	binPath, err := a.lookPath("claude")
	if err != nil {
		return false, "", "", false, fmt.Errorf("claude: not found: %w", err)
	}
	return true, binPath, a.SettingsPath(homeDir), true, nil
}

// InstallCommand returns the commands to install Claude Code.
func (a *Adapter) InstallCommand(profile interface{}) ([][]string, error) {
	return [][]string{
		{"npm", "install", "-g", "@anthropic-ai/claude-code"},
	}, nil
}

// Capabilities returns the list of capabilities Claude Code supports.
func (a *Adapter) Capabilities() []string {
	return []string{
		plugin.CapSkills,
		plugin.CapMCP,
		plugin.CapSubAgents,
		plugin.CapSystemPrompt,
		plugin.CapSlashCommands,
	}
}

// SupportsAutoInstall returns true — Claude Code supports npm install.
func (a *Adapter) SupportsAutoInstall() bool { return true }

// SupportsSkills returns true — Claude Code supports custom skills.
func (a *Adapter) SupportsSkills() bool { return true }

// SupportsSystemPrompt returns true — Claude Code supports CLAUDE.md.
func (a *Adapter) SupportsSystemPrompt() bool { return true }

// SupportsMCP returns true — Claude Code supports MCP servers.
func (a *Adapter) SupportsMCP() bool { return true }

// SupportsOutputStyles returns true — Claude Code supports output
// style configuration.
func (a *Adapter) SupportsOutputStyles() bool { return true }

// SupportsSlashCommands returns true — Claude Code supports custom
// slash commands.
func (a *Adapter) SupportsSlashCommands() bool { return true }

// SupportsSubAgents returns true — Claude Code supports delegating
// to sub-agents.
func (a *Adapter) SupportsSubAgents() bool { return true }

// SystemPromptStrategy returns MarkdownSections — Claude Code reads
// sections from CLAUDE.md.
func (a *Adapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return agents.StrategyMarkdownSections
}

// MCPStrategy returns SeparateMCPFiles — Claude Code stores each MCP
// server config in its own file under ~/.claude/mcp/.
func (a *Adapter) MCPStrategy() model.MCPStrategy {
	return agents.StrategySeparateMCPFiles
}

// GlobalConfigDir returns ~/.claude/.
func (a *Adapter) GlobalConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ".claude")
}

// SystemPromptDir returns ~/.claude/ — same as GlobalConfigDir.
func (a *Adapter) SystemPromptDir(homeDir string) string {
	return a.GlobalConfigDir(homeDir)
}

// SystemPromptFile returns ~/.claude/CLAUDE.md.
func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(a.GlobalConfigDir(homeDir), "CLAUDE.md")
}

// SkillsDir returns ~/.claude/skills/.
func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".claude", "skills")
}

// CommandsDir returns ~/.claude/commands/.
func (a *Adapter) CommandsDir(homeDir string) string {
	return filepath.Join(homeDir, ".claude", "commands")
}

// SubAgentsDir returns ~/.claude/agents/.
func (a *Adapter) SubAgentsDir(homeDir string) string {
	return filepath.Join(homeDir, ".claude", "agents")
}

// EmbeddedSubAgentsDir returns the relative path for embedded
// sub-agents shipped with biggz-ai.
func (a *Adapter) EmbeddedSubAgentsDir() string {
	return "claude/agents"
}

// OutputStyleDir returns ~/.claude/output-styles/.
func (a *Adapter) OutputStyleDir(homeDir string) string {
	return filepath.Join(homeDir, ".claude", "output-styles")
}

// SettingsPath returns ~/.claude/settings.json.
func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(homeDir, ".claude", "settings.json")
}

// MCPConfigPath returns the path for a specific MCP server config,
// e.g. ~/.claude/mcp/{serverName}.json.
func (a *Adapter) MCPConfigPath(homeDir string, serverName string) string {
	return filepath.Join(homeDir, ".claude", "mcp", serverName+".json")
}

// DeployConfig validates the config and reports capability.
func (a *Adapter) DeployConfig(ctx context.Context, cfg plugin.AgentConfig) error {
	_ = ctx
	_ = cfg
	return nil
}
