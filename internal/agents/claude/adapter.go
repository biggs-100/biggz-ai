// Package claude implements the AgentAdapter interface for Claude Code
// by Anthropic. Detection via exec.LookPath("claude"). Config under ~/.claude/.
package claude

import (
	"context"
	"os/exec"
	"path/filepath"

	"github.com/biggz-ai/biggz/plugin"
)

// Adapter implements plugin.AgentAdapter for Claude Code.
type Adapter struct {
	lookPath func(name string) (string, error)
}

func NewAdapter() *Adapter {
	return &Adapter{lookPath: exec.LookPath}
}

func (a *Adapter) ID() string                             { return "claude" }
func (a *Adapter) Name() string                           { return "Claude Code" }
func (a *Adapter) Detect(ctx context.Context) (string, error) { return a.lookPath("claude") }

func (a *Adapter) Capabilities() []plugin.Capability {
	return []plugin.Capability{
		plugin.CapSkills,
		plugin.CapSystemPrompt,
		plugin.CapSlashCommands,
	}
}

func (a *Adapter) GlobalConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ".claude")
}

func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".claude", "skills")
}

func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(homeDir, ".claude", "settings.json")
}

func (a *Adapter) DeployConfig(ctx context.Context, cfg plugin.AgentConfig) error {
	_ = ctx
	_ = cfg
	return nil
}
