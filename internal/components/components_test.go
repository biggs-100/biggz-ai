package components

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugin"
)

// -- mocks -------------------------------------------------------------------

type mockAdapter struct {
	id string
}

func (m *mockAdapter) ID() model.AgentID                                   { return model.AgentID(m.id) }
func (m *mockAdapter) Name() string                                        { return "Mock " + m.id }
func (m *mockAdapter) Tier() model.SupportTier                             { return model.TierFull }
func (m *mockAdapter) Detect(_ context.Context, _ string) (bool, string, string, bool, error) {
	return true, "/bin/" + m.id, "", false, nil
}
func (m *mockAdapter) InstallCommand(_ interface{}) ([][]string, error)    { return nil, nil }
func (m *mockAdapter) Capabilities() []string                              { return nil }
func (m *mockAdapter) SupportsAutoInstall() bool                           { return false }
func (m *mockAdapter) SupportsSkills() bool                                { return false }
func (m *mockAdapter) SupportsSystemPrompt() bool                          { return false }
func (m *mockAdapter) SupportsMCP() bool                                   { return false }
func (m *mockAdapter) SupportsOutputStyles() bool                          { return false }
func (m *mockAdapter) SupportsSlashCommands() bool                         { return false }
func (m *mockAdapter) SupportsSubAgents() bool                             { return false }
func (m *mockAdapter) SystemPromptStrategy() model.SystemPromptStrategy    { return 0 }
func (m *mockAdapter) MCPStrategy() model.MCPStrategy                      { return 0 }
func (m *mockAdapter) DeployConfig(_ context.Context, _ plugin.AgentConfig) error { return nil }
func (m *mockAdapter) GlobalConfigDir(homeDir string) string               { return filepath.Join(homeDir, ".config", m.id) }
func (m *mockAdapter) SystemPromptDir(homeDir string) string               { return filepath.Join(homeDir, ".config", m.id) }
func (m *mockAdapter) SystemPromptFile(homeDir string) string              { return filepath.Join(homeDir, ".config", m.id, "AGENTS.md") }
func (m *mockAdapter) SkillsDir(homeDir string) string                     { return filepath.Join(homeDir, ".config", m.id, "skills") }
func (m *mockAdapter) CommandsDir(homeDir string) string                   { return filepath.Join(homeDir, ".config", m.id, "commands") }
func (m *mockAdapter) SubAgentsDir(homeDir string) string                  { return "" }
func (m *mockAdapter) EmbeddedSubAgentsDir() string                        { return "" }
func (m *mockAdapter) OutputStyleDir(homeDir string) string                { return "" }
func (m *mockAdapter) SettingsPath(homeDir string) string                  { return filepath.Join(homeDir, ".config", m.id, "settings.jsonc") }
func (m *mockAdapter) MCPConfigPath(homeDir string, _ string) string       { return filepath.Join(homeDir, ".config", m.id, "settings.jsonc") }

// -- tests -------------------------------------------------------------------

func TestSkillsComponent_ID(t *testing.T) {
	c := NewSkillsComponent(t.TempDir(), fstest.MapFS{})
	if c.ID() != "skills" {
		t.Errorf("ID() = %q, want %q", c.ID(), "skills")
	}
}

func TestSkillsComponent_Deploy_UsesAdapterPath(t *testing.T) {
	homeDir := t.TempDir()
	fsys := fstest.MapFS{
		"skills/test.md": &fstest.MapFile{Data: []byte("# test skill")},
	}

	// Create skills dir expected by the adapter
	skillsDir := filepath.Join(homeDir, ".config", "mock", "skills")
	os.MkdirAll(skillsDir, 0755)

	c := NewSkillsComponent(homeDir, fsys)
	adapter := &mockAdapter{id: "mock"}

	result, err := c.Deploy(context.Background(), adapter)
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}

	if !result.Changed {
		t.Error("expected Changed=true")
	}

	// Verify the file was actually written to the right place
	targetPath := filepath.Join(skillsDir, "test.md")
	if _, err := os.Stat(targetPath); err != nil {
		t.Errorf("expected file at %s: %v", targetPath, err)
	}
}

func TestSkillsComponent_Deploy_NoFiles(t *testing.T) {
	homeDir := t.TempDir()
	// Real empty skills directory to avoid fstest.MapFS infinite recursion
	os.MkdirAll(filepath.Join(homeDir, "skills"), 0755)
	fsys := os.DirFS(homeDir)

	c := NewSkillsComponent(homeDir, fsys)
	adapter := &mockAdapter{id: "mock"}

	result, err := c.Deploy(context.Background(), adapter)
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}

	if result.Changed {
		t.Error("expected Changed=false for empty skills dir")
	}
}

func TestSkillsComponent_Deploy_ReturnsFileList(t *testing.T) {
	homeDir := t.TempDir()
	fsys := fstest.MapFS{
		"skills/foo.md": &fstest.MapFile{Data: []byte("foo")},
		"skills/bar.md": &fstest.MapFile{Data: []byte("bar")},
	}

	c := NewSkillsComponent(homeDir, fsys)
	adapter := &mockAdapter{id: "mock"}

	result, err := c.Deploy(context.Background(), adapter)
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}

	if len(result.Files) != 2 {
		t.Errorf("got %d files, want 2: %v", len(result.Files), result.Files)
	}
}

func TestConfigComponent_ID(t *testing.T) {
	c := NewConfigComponent(t.TempDir(), fstest.MapFS{})
	if c.ID() != "config" {
		t.Errorf("ID() = %q, want %q", c.ID(), "config")
	}
}

func TestConfigComponent_Deploy_EmptyOverlay(t *testing.T) {
	homeDir := t.TempDir()
	// Empty JSON overlay (valid, but no content to merge)
	fsys := fstest.MapFS{
		"opencode/sdd-overlay-multi.json": &fstest.MapFile{
			Data: []byte(`{}`),
		},
	}

	c := NewConfigComponent(homeDir, fsys)
	adapter := &mockAdapter{id: "mock"}

	result, err := c.Deploy(context.Background(), adapter)
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}

	if !result.Changed {
		t.Error("expected Changed=true (config always writes on deploy)")
	}
}

func TestConfigComponent_Deploy_WithOverlay(t *testing.T) {
	homeDir := t.TempDir()
	fsys := fstest.MapFS{
		"opencode/sdd-overlay-multi.json": &fstest.MapFile{
			Data: []byte(`{"agent": {"biggz-orchestrator": {"prompt": "test"}}}`),
		},
	}

	c := NewConfigComponent(homeDir, fsys)
	adapter := &mockAdapter{id: "mock"}

	result, err := c.Deploy(context.Background(), adapter)
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}

	if !result.Changed {
		t.Error("expected Changed=true with overlay")
	}
}

func TestPromptsComponent_ID(t *testing.T) {
	c := NewPromptsComponent(t.TempDir(), fstest.MapFS{})
	if c.ID() != "prompts" {
		t.Errorf("ID() = %q, want %q", c.ID(), "prompts")
	}
}

func TestPromptsComponent_Deploy_NoFiles(t *testing.T) {
	homeDir := t.TempDir()
	// Real empty prompts/sdd directory to avoid fstest.MapFS infinite recursion
	os.MkdirAll(filepath.Join(homeDir, "prompts", "sdd"), 0755)
	fsys := os.DirFS(homeDir)

	c := NewPromptsComponent(homeDir, fsys)
	adapter := &mockAdapter{id: "mock"}

	result, err := c.Deploy(context.Background(), adapter)
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}

	if result.Changed {
		t.Error("expected Changed=false for empty prompts dir")
	}
}

func TestPromptsComponent_Deploy_WithFiles(t *testing.T) {
	homeDir := t.TempDir()
	fsys := fstest.MapFS{
		"prompts/sdd/spec.md": &fstest.MapFile{Data: []byte("# spec prompt")},
	}

	c := NewPromptsComponent(homeDir, fsys)
	adapter := &mockAdapter{id: "mock"}

	result, err := c.Deploy(context.Background(), adapter)
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}

	if !result.Changed {
		t.Error("expected Changed=true")
	}
	if len(result.Files) != 1 || result.Files[0] != "prompts/sdd/spec.md" {
		t.Errorf("Files = %v, want [prompts/sdd/spec.md]", result.Files)
	}
}

// Ensure mockAdapter implements plugin.AgentAdapter at compile time
var _ plugin.AgentAdapter = (*mockAdapter)(nil)

// Ensure component types implement Component at compile time
var _ Component = (*skillsComponent)(nil)
var _ Component = (*configComponent)(nil)
var _ Component = (*promptsComponent)(nil)

// Convenience import — testing/fstest only, never used at runtime
var _ fs.FS
