package opencode

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/biggz-ai/biggz/internal/agents"
	"github.com/biggz-ai/biggz/model"
	"github.com/biggz-ai/biggz/plugin"
)

func TestNewAdapter(t *testing.T) {
	a := NewAdapter()
	if a.ID() != agents.AgentOpenCode {
		t.Errorf("ID() = %q, want %q", a.ID(), agents.AgentOpenCode)
	}
	if a.Name() != "OpenCode" {
		t.Errorf("Name() = %q, want %q", a.Name(), "OpenCode")
	}
}

func TestTier(t *testing.T) {
	a := NewAdapter()
	if got := a.Tier(); got != model.TierFull {
		t.Errorf("Tier() = %q, want %q", got, model.TierFull)
	}
}

func TestDetect_Found(t *testing.T) {
	a := &Adapter{
		lookPath: func(name string) (string, error) {
			return "/usr/local/bin/" + name, nil
		},
	}
	ok, binPath, configPath, autoCapable, err := a.Detect(context.Background(), "/home/user")
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}
	if !ok {
		t.Fatal("Detect() returned installed=false, want true")
	}
	if binPath != "/usr/local/bin/opencode" {
		t.Errorf("Detect() binPath = %q, want %q", binPath, "/usr/local/bin/opencode")
	}
	if configPath == "" {
		t.Error("Detect() configPath is empty")
	}
	if !autoCapable {
		t.Error("Detect() autoInstallCapable=false, want true")
	}
}

func TestDetect_NotFound(t *testing.T) {
	a := &Adapter{
		lookPath: func(name string) (string, error) {
			return "", exec.ErrNotFound
		},
	}
	ok, binPath, configPath, autoCapable, err := a.Detect(context.Background(), "/home/user")
	if err == nil {
		t.Fatal("Detect() expected error, got nil")
	}
	if ok {
		t.Fatal("Detect() returned installed=true, want false")
	}
	if binPath != "" {
		t.Errorf("Detect() binPath = %q, want empty", binPath)
	}
	if configPath != "" {
		t.Errorf("Detect() configPath = %q, want empty", configPath)
	}
	if autoCapable {
		t.Error("Detect() autoInstallCapable=true, want false")
	}
	if !errors.Is(err, exec.ErrNotFound) {
		t.Errorf("Detect() error = %v, want exec.ErrNotFound", err)
	}
}

func TestInstallCommand(t *testing.T) {
	a := NewAdapter()
	cmds, err := a.InstallCommand(nil)
	if err != nil {
		t.Fatalf("InstallCommand() returned error: %v", err)
	}
	if len(cmds) == 0 {
		t.Fatal("InstallCommand() returned empty commands")
	}
}

func TestCapabilities(t *testing.T) {
	a := NewAdapter()
	caps := a.Capabilities()
	if len(caps) == 0 {
		t.Fatal("Capabilities() returned empty slice")
	}
	expected := map[string]bool{
		"skills":         false,
		"mcp":            false,
		"system_prompt":  false,
		"slash_commands": false,
	}
	for _, c := range caps {
		if _, ok := expected[c]; ok {
			expected[c] = true
		}
	}
	for capName, found := range expected {
		if !found {
			t.Errorf("Capabilities() missing %q", capName)
		}
	}
}

func TestSupportsMethods(t *testing.T) {
	a := NewAdapter()
	tests := []struct {
		name string
		got  bool
		want bool
	}{
		{"SupportsAutoInstall", a.SupportsAutoInstall(), true},
		{"SupportsSkills", a.SupportsSkills(), true},
		{"SupportsSystemPrompt", a.SupportsSystemPrompt(), true},
		{"SupportsMCP", a.SupportsMCP(), true},
		{"SupportsOutputStyles", a.SupportsOutputStyles(), false},
		{"SupportsSlashCommands", a.SupportsSlashCommands(), true},
		{"SupportsSubAgents", a.SupportsSubAgents(), false},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s() = %v, want %v", tt.name, tt.got, tt.want)
		}
	}
}

func TestStrategies(t *testing.T) {
	a := NewAdapter()
	if got := a.SystemPromptStrategy(); got != agents.StrategyFileReplace {
		t.Errorf("SystemPromptStrategy() = %v, want %v", got, agents.StrategyFileReplace)
	}
	if got := a.MCPStrategy(); got != agents.StrategyMergeIntoSettings {
		t.Errorf("MCPStrategy() = %v, want %v", got, agents.StrategyMergeIntoSettings)
	}
}

func TestGlobalConfigDir(t *testing.T) {
	a := NewAdapter()
	dir := a.GlobalConfigDir("/home/user")
	want := filepath.Join("/home/user", ".config", "opencode")
	if dir != want {
		t.Errorf("GlobalConfigDir() = %q, want %q", dir, want)
	}
}

func TestSystemPromptDir(t *testing.T) {
	a := NewAdapter()
	dir := a.SystemPromptDir("/home/user")
	want := a.GlobalConfigDir("/home/user")
	if dir != want {
		t.Errorf("SystemPromptDir() = %q, want %q", dir, want)
	}
}

func TestSystemPromptFile(t *testing.T) {
	a := NewAdapter()
	got := a.SystemPromptFile("/home/user")
	want := filepath.Join("/home/user", ".config", "opencode", "AGENTS.md")
	if got != want {
		t.Errorf("SystemPromptFile() = %q, want %q", got, want)
	}
}

func TestSkillsDir(t *testing.T) {
	a := NewAdapter()
	dir := a.SkillsDir("/home/user")
	want := filepath.Join("/home/user", ".config", "opencode", "skills")
	if dir != want {
		t.Errorf("SkillsDir() = %q, want %q", dir, want)
	}
}

func TestCommandsDir(t *testing.T) {
	a := NewAdapter()
	dir := a.CommandsDir("/home/user")
	want := filepath.Join("/home/user", ".config", "opencode", "commands")
	if dir != want {
		t.Errorf("CommandsDir() = %q, want %q", dir, want)
	}
}

func TestSubAgentsDir(t *testing.T) {
	a := NewAdapter()
	if got := a.SubAgentsDir("/home/user"); got != "" {
		t.Errorf("SubAgentsDir() = %q, want empty", got)
	}
}

func TestEmbeddedSubAgentsDir(t *testing.T) {
	a := NewAdapter()
	if got := a.EmbeddedSubAgentsDir(); got != "" {
		t.Errorf("EmbeddedSubAgentsDir() = %q, want empty", got)
	}
}

func TestOutputStyleDir(t *testing.T) {
	a := NewAdapter()
	if got := a.OutputStyleDir("/home/user"); got != "" {
		t.Errorf("OutputStyleDir() = %q, want empty", got)
	}
}

func TestSettingsPath(t *testing.T) {
	a := NewAdapter()
	path := a.SettingsPath("/home/user")
	want := filepath.Join("/home/user", ".config", "opencode", "opencode.json")
	if path != want {
		t.Errorf("SettingsPath() = %q, want %q", path, want)
	}
}

func TestMCPConfigPath(t *testing.T) {
	a := NewAdapter()
	got := a.MCPConfigPath("/home/user", "my-server")
	want := a.SettingsPath("/home/user")
	if got != want {
		t.Errorf("MCPConfigPath() = %q, want %q", got, want)
	}
}

func TestDeployConfig_Noop(t *testing.T) {
	a := NewAdapter()
	err := a.DeployConfig(context.Background(), plugin.AgentConfig{})
	if err != nil {
		t.Errorf("DeployConfig() = %v, want nil", err)
	}
}
