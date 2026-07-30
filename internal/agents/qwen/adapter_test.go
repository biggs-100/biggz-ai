package qwen

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/biggz-ai/biggz/internal/agents"
	"github.com/biggz-ai/biggz/model"
	"github.com/biggz-ai/biggz/plugin"
)

func TestID(t *testing.T) {
	a := NewAdapter()
	if got := a.ID(); got != agents.AgentQwenCode {
		t.Errorf("ID() = %q, want %q", got, agents.AgentQwenCode)
	}
}

func TestName(t *testing.T) {
	a := NewAdapter()
	if got := a.Name(); got != "Qwen" {
		t.Errorf("Name() = %q, want %q", got, "Qwen")
	}
}

func TestTier(t *testing.T) {
	a := NewAdapter()
	if got := a.Tier(); got != model.TierFull {
		t.Errorf("Tier() = %q, want %q", got, model.TierFull)
	}
}

func TestDetect_Found(t *testing.T) {
	a := &Adapter{lookPath: func(name string) (string, error) {
		return "/usr/local/bin/" + name, nil
	}}
	ok, binPath, configPath, autoCapable, err := a.Detect(context.Background(), "/home/user")
	if err != nil {
		t.Fatalf("Detect() unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("Detect() returned installed=false, want true")
	}
	if binPath != "/usr/local/bin/qwen" {
		t.Errorf("Detect() binPath = %q, want %q", binPath, "/usr/local/bin/qwen")
	}
	if configPath == "" {
		t.Error("Detect() configPath is empty")
	}
	if !autoCapable {
		t.Error("Detect() autoInstallCapable=false, want true")
	}
}

func TestDetect_NotFound(t *testing.T) {
	a := &Adapter{lookPath: func(name string) (string, error) {
		return "", errors.New("not found")
	}}
	ok, _, _, _, err := a.Detect(context.Background(), "/home/user")
	if err == nil {
		t.Fatal("Detect() expected error, got nil")
	}
	if ok {
		t.Fatal("Detect() returned installed=true, want false")
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

func TestCapabilities(t *testing.T) {
	a := NewAdapter()
	caps := a.Capabilities()
	if len(caps) == 0 {
		t.Fatal("Capabilities() returned empty")
	}
	hasSkills := false
	for _, c := range caps {
		if c == plugin.CapSkills {
			hasSkills = true
		}
	}
	if !hasSkills {
		t.Error("Capabilities() should include CapSkills")
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
	got := a.GlobalConfigDir("/home/user")
	want := filepath.Join("/home/user", ".qwen")
	if got != want {
		t.Errorf("GlobalConfigDir() = %q, want %q", got, want)
	}
}

func TestSystemPromptDir(t *testing.T) {
	a := NewAdapter()
	got := a.SystemPromptDir("/home/user")
	want := a.GlobalConfigDir("/home/user")
	if got != want {
		t.Errorf("SystemPromptDir() = %q, want %q", got, want)
	}
}

func TestSystemPromptFile(t *testing.T) {
	a := NewAdapter()
	got := a.SystemPromptFile("/home/user")
	want := filepath.Join("/home/user", ".qwen", "QWEN.md")
	if got != want {
		t.Errorf("SystemPromptFile() = %q, want %q", got, want)
	}
}

func TestSkillsDir(t *testing.T) {
	a := NewAdapter()
	got := a.SkillsDir("/home/user")
	want := filepath.Join("/home/user", ".qwen", "skills")
	if got != want {
		t.Errorf("SkillsDir() = %q, want %q", got, want)
	}
}

func TestCommandsDir(t *testing.T) {
	a := NewAdapter()
	got := a.CommandsDir("/home/user")
	want := filepath.Join("/home/user", ".qwen", "commands")
	if got != want {
		t.Errorf("CommandsDir() = %q, want %q", got, want)
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
	got := a.SettingsPath("/home/user")
	want := filepath.Join("/home/user", ".qwen", "settings.json")
	if got != want {
		t.Errorf("SettingsPath() = %q, want %q", got, want)
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

func TestDeployConfig(t *testing.T) {
	a := NewAdapter()
	err := a.DeployConfig(context.Background(), plugin.AgentConfig{})
	if err != nil {
		t.Fatalf("DeployConfig() unexpected error: %v", err)
	}
}
