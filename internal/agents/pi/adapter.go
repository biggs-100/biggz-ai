// Package pi implements the AgentAdapter for Pi (by Inflection AI).
package pi

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
	agents.Register(agents.AgentPi, func() plugin.AgentAdapter { return NewAdapter() })
}

type Adapter struct {
	lookPath func(name string) (string, error)
}

func NewAdapter() *Adapter { return &Adapter{lookPath: exec.LookPath} }

func (a *Adapter) ID() model.AgentID                        { return agents.AgentPi }
func (a *Adapter) Name() string                               { return "Pi" }
func (a *Adapter) Tier() model.SupportTier                    { return agents.TierFull }

func (a *Adapter) Detect(ctx context.Context, homeDir string) (bool, string, string, bool, error) {
	binPath, err := a.lookPath("pi")
	if err != nil {
		return false, "", "", false, fmt.Errorf("pi: not found: %w", err)
	}
	return true, binPath, a.SettingsPath(homeDir), true, nil
}

func (a *Adapter) InstallCommand(profile interface{}) ([][]string, error) {
	return [][]string{{"npm", "install", "-g", "@inflection/pi"}}, nil
}

func (a *Adapter) Capabilities() []string {
	return []string{plugin.CapSkills, plugin.CapSystemPrompt, plugin.CapMCP}
}

func (a *Adapter) SupportsAutoInstall() bool    { return true }
func (a *Adapter) SupportsSkills() bool         { return true }
func (a *Adapter) SupportsSystemPrompt() bool   { return true }
func (a *Adapter) SupportsMCP() bool            { return true }
func (a *Adapter) SupportsOutputStyles() bool   { return false }
func (a *Adapter) SupportsSlashCommands() bool  { return false }
func (a *Adapter) SupportsSubAgents() bool      { return false }
func (a *Adapter) SystemPromptStrategy() model.SystemPromptStrategy { return agents.StrategyFileReplace }
func (a *Adapter) MCPStrategy() model.MCPStrategy                    { return agents.StrategyMergeIntoSettings }
func (a *Adapter) GlobalConfigDir(homeDir string) string             { return filepath.Join(homeDir, ".config", "pi") }
func (a *Adapter) SystemPromptDir(homeDir string) string             { return a.GlobalConfigDir(homeDir) }
func (a *Adapter) SystemPromptFile(homeDir string) string            { return filepath.Join(a.GlobalConfigDir(homeDir), "PI.md") }
func (a *Adapter) SkillsDir(homeDir string) string                   { return filepath.Join(homeDir, ".config", "pi", "skills") }
func (a *Adapter) CommandsDir(homeDir string) string                 { return "" }
func (a *Adapter) SubAgentsDir(homeDir string) string                { return "" }
func (a *Adapter) EmbeddedSubAgentsDir() string                      { return "" }
func (a *Adapter) OutputStyleDir(homeDir string) string              { return "" }
func (a *Adapter) SettingsPath(homeDir string) string                { return filepath.Join(homeDir, ".config", "pi", "settings.json") }
func (a *Adapter) MCPConfigPath(homeDir string, serverName string) string { return a.SettingsPath(homeDir) }
func (a *Adapter) DeployConfig(ctx context.Context, cfg plugin.AgentConfig) error { return nil }
