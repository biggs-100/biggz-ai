// Package cursor implements the AgentAdapter interface for Cursor
// (AI-native code editor by Anysphere).
package cursor

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/biggs-100/biggz-ai/internal/agents"
	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugin"
)

func init() {
	agents.Register(agents.AgentCursor, func() plugin.AgentAdapter { return NewAdapter() })
}

// Adapter implements plugin.AgentAdapter for Cursor.
type Adapter struct {
	lookPath func(name string) (string, error)
}

// NewAdapter creates a new Adapter.
func NewAdapter() *Adapter {
	return &Adapter{lookPath: exec.LookPath}
}

func (a *Adapter) ID() model.AgentID                       { return agents.AgentCursor }
func (a *Adapter) Name() string                             { return "Cursor" }
func (a *Adapter) Tier() model.SupportTier                  { return agents.TierFull }

func (a *Adapter) Detect(ctx context.Context, homeDir string) (bool, string, string, bool, error) {
	binPath, err := a.lookPath("cursor")
	if err != nil {
		return false, "", "", false, fmt.Errorf("cursor: not found: %w", err)
	}
	return true, binPath, a.SettingsPath(homeDir), true, nil
}

func (a *Adapter) InstallCommand(profile interface{}) ([][]string, error) {
	return [][]string{
		{"curl", "-fsSL", "https://cursor.sh/install.sh", "|", "sh"},
	}, nil
}

func (a *Adapter) Capabilities() []string {
	return []string{
		plugin.CapSkills,
		plugin.CapMCP,
		plugin.CapSystemPrompt,
		plugin.CapSubAgents,
	}
}

func (a *Adapter) SupportsAutoInstall() bool    { return true }
func (a *Adapter) SupportsSkills() bool         { return true }
func (a *Adapter) SupportsSystemPrompt() bool   { return true }
func (a *Adapter) SupportsMCP() bool            { return true }
func (a *Adapter) SupportsOutputStyles() bool   { return false }
func (a *Adapter) SupportsSlashCommands() bool  { return false }
func (a *Adapter) SupportsSubAgents() bool      { return true }

func (a *Adapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return agents.StrategyFileReplace
}

func (a *Adapter) MCPStrategy() model.MCPStrategy {
	return agents.StrategyMergeIntoSettings
}

func (a *Adapter) GlobalConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ".cursor")
}

func (a *Adapter) SystemPromptDir(homeDir string) string {
	return a.GlobalConfigDir(homeDir)
}

func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(a.GlobalConfigDir(homeDir), "rules", "AGENTS.md")
}

func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".cursor", "skills")
}

func (a *Adapter) CommandsDir(homeDir string) string {
	return ""
}

func (a *Adapter) SubAgentsDir(homeDir string) string {
	return filepath.Join(homeDir, ".cursor", "agents")
}

func (a *Adapter) EmbeddedSubAgentsDir() string {
	return "agents/cursor"
}

func (a *Adapter) OutputStyleDir(homeDir string) string {
	return ""
}

func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(homeDir, ".cursor", "config.json")
}

func (a *Adapter) MCPConfigPath(homeDir string, serverName string) string {
	return filepath.Join(homeDir, ".cursor", "mcp.json")
}

func (a *Adapter) DeployConfig(ctx context.Context, cfg plugin.AgentConfig) error {
	_ = ctx
	_ = cfg
	return nil
}
