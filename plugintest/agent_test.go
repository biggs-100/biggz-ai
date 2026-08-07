package plugintest

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/biggs-100/biggz-ai/model"
)

func TestFakeAgentDefaults(t *testing.T) {
	a := &FakeAgent{}
	if a.ID() != "fake-agent" {
		t.Errorf("ID() = %q, want %q", a.ID(), "fake-agent")
	}
	if a.Name() != "Fake Agent" {
		t.Errorf("Name() = %q, want %q", a.Name(), "Fake Agent")
	}
	if a.Tier() != model.TierFull {
		t.Errorf("Tier() = %q, want %q", a.Tier(), model.TierFull)
	}
}

func TestFakeAgentDetect_Installed(t *testing.T) {
	a := &FakeAgent{Installed: true}
	ok, binPath, configPath, _, err := a.Detect(context.Background(), "/home/user")
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}
	if !ok {
		t.Fatal("Detect() returned installed=false, want true")
	}
	if binPath == "" {
		t.Fatal("Detect() returned empty path")
	}
	if configPath == "" {
		t.Fatal("Detect() returned empty configPath")
	}
}

func TestFakeAgentDetect_NotInstalled(t *testing.T) {
	a := &FakeAgent{Installed: false}
	ok, _, _, _, err := a.Detect(context.Background(), "/home/user")
	if err == nil {
		t.Fatal("Detect() expected error for not installed agent")
	}
	if ok {
		t.Fatal("Detect() returned installed=true, want false")
	}
}

func TestFakeAgentDetect_ErrorInjection(t *testing.T) {
	ctx := context.Background()
	injectedErr := context.DeadlineExceeded
	a := &FakeAgent{InjectDetectError: injectedErr}
	ok, binPath, configPath, autoCapable, err := a.Detect(ctx, "/home/user")
	if err != injectedErr {
		t.Fatalf("Detect() error = %v, want injected %v", err, injectedErr)
	}
	if ok {
		t.Error("Detect() returned installed=true, want false")
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
}

func TestFakeAgentInstallCommand(t *testing.T) {
	a := &FakeAgent{}
	cmds, err := a.InstallCommand(nil)
	if err != nil {
		t.Fatalf("InstallCommand() returned error: %v", err)
	}
	if len(cmds) == 0 {
		t.Fatal("InstallCommand() returned empty commands")
	}
}

func TestFakeAgentSupportsDefaults(t *testing.T) {
	a := &FakeAgent{}
	if a.SupportsAutoInstall() {
		t.Error("SupportsAutoInstall() = true, want false")
	}
	if a.SupportsSkills() {
		t.Error("SupportsSkills() = true, want false")
	}
	if a.SupportsSystemPrompt() {
		t.Error("SupportsSystemPrompt() = true, want false")
	}
	if a.SupportsMCP() {
		t.Error("SupportsMCP() = true, want false")
	}
	if a.SupportsOutputStyles() {
		t.Error("SupportsOutputStyles() = true, want false")
	}
	if a.SupportsSlashCommands() {
		t.Error("SupportsSlashCommands() = true, want false")
	}
	if a.SupportsSubAgents() {
		t.Error("SupportsSubAgents() = true, want false")
	}
}

func TestFakeAgentSupportsOverrides(t *testing.T) {
	yes := true
	a := &FakeAgent{
		AgentSkills:        &yes,
		AgentSystemPrompt:  &yes,
		AgentMCP:           &yes,
		AgentSlashCommands: &yes,
	}
	if !a.SupportsSkills() {
		t.Error("SupportsSkills() = false, want true")
	}
	if !a.SupportsSystemPrompt() {
		t.Error("SupportsSystemPrompt() = false, want true")
	}
	if !a.SupportsMCP() {
		t.Error("SupportsMCP() = false, want true")
	}
	if !a.SupportsSlashCommands() {
		t.Error("SupportsSlashCommands() = false, want true")
	}
}

func TestFakeAgentGlobalConfigDir_WithTempDir(t *testing.T) {
	tmpDir := t.TempDir()
	a := &FakeAgent{}
	a.SetTempDir(tmpDir)
	got := a.GlobalConfigDir("/home/user")
	if got != tmpDir {
		t.Errorf("GlobalConfigDir() = %q, want tempDir %q", got, tmpDir)
	}
}

func TestFakeAgentGlobalConfigDir_WithoutTempDir(t *testing.T) {
	a := &FakeAgent{}
	got := a.GlobalConfigDir("/home/user")
	want := filepath.Join("/home/user", ".config", "opencode")
	if got != want {
		t.Errorf("GlobalConfigDir() = %q, want %q", got, want)
	}
}

func TestFakeAgentSkillsDir_WithTempDir(t *testing.T) {
	tmpDir := t.TempDir()
	a := &FakeAgent{}
	a.SetTempDir(tmpDir)
	got := a.SkillsDir("/home/user")
	want := filepath.Join(tmpDir, "skills")
	if got != want {
		t.Errorf("SkillsDir() = %q, want %q", got, want)
	}
}

func TestFakeAgentSkillsDir_WithoutTempDir(t *testing.T) {
	a := &FakeAgent{}
	got := a.SkillsDir("/home/user")
	want := filepath.Join("/home/user", ".config", "opencode", "skills")
	if got != want {
		t.Errorf("SkillsDir() = %q, want %q", got, want)
	}
}

func TestFakeAgentSettingsPath_WithTempDir(t *testing.T) {
	tmpDir := t.TempDir()
	a := &FakeAgent{}
	a.SetTempDir(tmpDir)
	got := a.SettingsPath("/home/user")
	want := filepath.Join(tmpDir, "opencode.jsonc")
	if got != want {
		t.Errorf("SettingsPath() = %q, want %q", got, want)
	}
}

func TestFakeAgentSettingsPath_WithoutTempDir(t *testing.T) {
	a := &FakeAgent{}
	got := a.SettingsPath("/home/user")
	want := filepath.Join("/home/user", ".config", "opencode", "opencode.jsonc")
	if got != want {
		t.Errorf("SettingsPath() = %q, want %q", got, want)
	}
}

func TestFakeAgentSetTempDir_TwiceUpdates(t *testing.T) {
	a := &FakeAgent{}
	a.SetTempDir("/first")
	a.SetTempDir("/second")
	got := a.GlobalConfigDir("/home/user")
	if got != "/second" {
		t.Errorf("GlobalConfigDir() = %q, want %q", got, "/second")
	}
}

func TestFakeAgentSystemPromptFile(t *testing.T) {
	a := &FakeAgent{}
	got := a.SystemPromptFile("/home/user")
	want := filepath.Join("/home/user", ".config", "opencode", "AGENTS.md")
	if got != want {
		t.Errorf("SystemPromptFile() = %q, want %q", got, want)
	}
}

func TestFakeAgentCommandsDir(t *testing.T) {
	a := &FakeAgent{}
	got := a.CommandsDir("/home/user")
	want := filepath.Join("/home/user", ".config", "opencode", "commands")
	if got != want {
		t.Errorf("CommandsDir() = %q, want %q", got, want)
	}
}

func TestFakeAgentMCPConfigPath(t *testing.T) {
	a := &FakeAgent{}
	got := a.MCPConfigPath("/home/user", "test-server")
	want := a.SettingsPath("/home/user")
	if got != want {
		t.Errorf("MCPConfigPath() = %q, want %q", got, want)
	}
}

func TestFakeAgentSupportsAutoInstallOverride(t *testing.T) {
	yes := true
	a := &FakeAgent{AutoInstall: true}
	if !a.SupportsAutoInstall() {
		t.Error("SupportsAutoInstall() = false, want true (from AutoInstall field)")
	}
	a2 := &FakeAgent{AgentAutoInstall: &yes}
	if !a2.SupportsAutoInstall() {
		t.Error("SupportsAutoInstall() = false, want true (from AgentAutoInstall override)")
	}
}
