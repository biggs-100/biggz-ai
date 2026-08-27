// Package codex implements the AgentAdapter for Codex CLI (by OpenAI).
package codex

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
	agents.Register(agents.AgentCodexCLI, func() plugin.AgentAdapter { return NewAdapter() })
}

type Adapter struct {
	lookPath func(name string) (string, error)
}

func NewAdapter() *Adapter { return &Adapter{lookPath: exec.LookPath} }

func (a *Adapter) ID() model.AgentID       { return agents.AgentCodexCLI }
func (a *Adapter) Name() string            { return "Codex CLI" }
func (a *Adapter) Tier() model.SupportTier { return agents.TierFull }

func (a *Adapter) Detect(ctx context.Context, homeDir string) (bool, string, string, bool, error) {
	binPath, err := a.lookPath("codex")
	if err != nil {
		return false, "", "", false, fmt.Errorf("codex: not found: %w", err)
	}
	return true, binPath, a.SettingsPath(homeDir), true, nil
}

func (a *Adapter) InstallCommand(profile interface{}) ([][]string, error) {
	return [][]string{{"npm", "install", "-g", "@openai/codex"}}, nil
}

func (a *Adapter) Capabilities() []string {
	return []string{plugin.CapSkills, plugin.CapSystemPrompt}
}

func (a *Adapter) SupportsAutoInstall() bool   { return true }
func (a *Adapter) SupportsSkills() bool        { return true }
func (a *Adapter) SupportsSystemPrompt() bool  { return true }
func (a *Adapter) SupportsMCP() bool           { return false }
func (a *Adapter) SupportsOutputStyles() bool  { return false }
func (a *Adapter) SupportsSlashCommands() bool { return false }
func (a *Adapter) SupportsSubAgents() bool     { return false }
func (a *Adapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return agents.StrategyFileReplace
}
func (a *Adapter) MCPStrategy() model.MCPStrategy { return agents.StrategyMergeIntoSettings }
func (a *Adapter) GlobalConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ".config", "codex")
}
func (a *Adapter) SystemPromptDir(homeDir string) string { return a.GlobalConfigDir(homeDir) }
func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(a.GlobalConfigDir(homeDir), "CODEX.md")
}
func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".config", "codex", "skills")
}
func (a *Adapter) CommandsDir(homeDir string) string    { return "" }
func (a *Adapter) SubAgentsDir(homeDir string) string   { return "" }
func (a *Adapter) EmbeddedSubAgentsDir() string         { return "" }
func (a *Adapter) OutputStyleDir(homeDir string) string { return "" }
func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(homeDir, ".config", "codex", "settings.json")
}
func (a *Adapter) MCPConfigPath(homeDir string, serverName string) string         { return "" }
func (a *Adapter) DeployConfig(ctx context.Context, cfg plugin.AgentConfig) error { return nil }
