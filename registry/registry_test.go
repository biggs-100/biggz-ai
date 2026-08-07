package registry

import (
	"context"
	"fmt"
	"testing"

	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugin"
)

// compile-time check: *mockLens implements plugin.LensPlugin
var _ plugin.LensPlugin = (*mockLens)(nil)

// compile-time check: *mockAgent implements plugin.AgentAdapter
var _ plugin.AgentAdapter = (*mockAgent)(nil)

// -- mocks ---------------------------------------------------------------

type mockLens struct {
	id      string
	name    string
	version string
}

func (m *mockLens) ID() string                             { return m.id }
func (m *mockLens) Name() string                           { return m.name }
func (m *mockLens) Version() string                        { return m.version }
func (m *mockLens) Analyze(_ context.Context, _ model.ReviewSubject) (*plugin.LensResult, error) {
	return &plugin.LensResult{LensID: m.id}, nil
}
func (m *mockLens) Policies() []plugin.Policy {
	return []plugin.Policy{{Name: "mock-policy", Description: "A mock policy"}}
}

type mockAgent struct {
	id        string
	name      string
	installed bool
}

func (m *mockAgent) ID() model.AgentID                                   { return model.AgentID(m.id) }
func (m *mockAgent) Name() string                                        { return m.name }
func (m *mockAgent) Tier() model.SupportTier                             { return model.TierFull }
func (m *mockAgent) Detect(_ context.Context, _ string) (bool, string, string, bool, error) {
	if !m.installed {
		return false, "", "", false, fmt.Errorf("not found")
	}
	return true, "/usr/local/bin/" + m.id, "", false, nil
}
func (m *mockAgent) InstallCommand(_ interface{}) ([][]string, error)    { return nil, nil }
func (m *mockAgent) Capabilities() []string {
	return []string{plugin.CapSkills, plugin.CapMCP}
}
func (m *mockAgent) SupportsAutoInstall() bool                           { return false }
func (m *mockAgent) SupportsSkills() bool                                { return false }
func (m *mockAgent) SupportsSystemPrompt() bool                          { return false }
func (m *mockAgent) SupportsMCP() bool                                   { return false }
func (m *mockAgent) SupportsOutputStyles() bool                          { return false }
func (m *mockAgent) SupportsSlashCommands() bool                         { return false }
func (m *mockAgent) SupportsSubAgents() bool                             { return false }
func (m *mockAgent) SystemPromptStrategy() model.SystemPromptStrategy    { return 0 }
func (m *mockAgent) MCPStrategy() model.MCPStrategy                      { return 0 }
func (m *mockAgent) DeployConfig(_ context.Context, _ plugin.AgentConfig) error {
	if !m.installed {
		return fmt.Errorf("cannot deploy, not installed")
	}
	return nil
}
func (m *mockAgent) GlobalConfigDir(homeDir string) string               { return homeDir + "/.config/" + m.id }
func (m *mockAgent) SystemPromptDir(homeDir string) string               { return m.GlobalConfigDir(homeDir) }
func (m *mockAgent) SystemPromptFile(homeDir string) string              { return m.GlobalConfigDir(homeDir) + "/AGENTS.md" }
func (m *mockAgent) SkillsDir(homeDir string) string                     { return homeDir + "/.config/" + m.id + "/skills" }
func (m *mockAgent) CommandsDir(homeDir string) string                   { return homeDir + "/.config/" + m.id + "/commands" }
func (m *mockAgent) SubAgentsDir(homeDir string) string                  { return "" }
func (m *mockAgent) EmbeddedSubAgentsDir() string                        { return "" }
func (m *mockAgent) OutputStyleDir(homeDir string) string                { return "" }
func (m *mockAgent) SettingsPath(homeDir string) string                  { return homeDir + "/.config/" + m.id + "/" + m.id + ".json" }
func (m *mockAgent) MCPConfigPath(homeDir string, _ string) string       { return homeDir + "/.config/" + m.id + "/" + m.id + ".json" }

// -- tests ----------------------------------------------------------------

func TestRegisterAndGetLens(t *testing.T) {
	r := New()
	lens := &mockLens{id: "test-lens", name: "Test Lens", version: "1.0.0"}

	err := r.RegisterLens(lens)
	if err != nil {
		t.Fatalf("RegisterLens returned unexpected error: %v", err)
	}

	got := r.GetLens("test-lens")
	if got == nil {
		t.Fatal("GetLens returned nil for registered lens")
	}
	if got.ID() != "test-lens" {
		t.Errorf("GetLens ID = %q, want %q", got.ID(), "test-lens")
	}
	if got.Name() != "Test Lens" {
		t.Errorf("GetLens Name = %q, want %q", got.Name(), "Test Lens")
	}
}

func TestRegisterAndGetAdapter(t *testing.T) {
	r := New()
	agent := &mockAgent{id: "test-agent", name: "Test Agent", installed: true}

	err := r.RegisterAdapter(agent)
	if err != nil {
		t.Fatalf("RegisterAdapter returned unexpected error: %v", err)
	}

	got := r.GetAdapter("test-agent")
	if got == nil {
		t.Fatal("GetAdapter returned nil for registered agent")
	}
	if got.ID() != model.AgentID("test-agent") {
		t.Errorf("GetAdapter ID = %q, want %q", got.ID(), "test-agent")
	}
	if got.Name() != "Test Agent" {
		t.Errorf("GetAdapter Name = %q, want %q", got.Name(), "Test Agent")
	}
}

func TestDuplicateLensRegistration(t *testing.T) {
	r := New()
	lens := &mockLens{id: "dup-lens", name: "Dup Lens"}

	if err := r.RegisterLens(lens); err != nil {
		t.Fatalf("First RegisterLens returned unexpected error: %v", err)
	}
	if err := r.RegisterLens(lens); err == nil {
		t.Fatal("Expected error on duplicate lens registration, got nil")
	}
}

func TestDuplicateAdapterRegistration(t *testing.T) {
	r := New()
	agent := &mockAgent{id: "dup-agent", name: "Dup Agent"}

	if err := r.RegisterAdapter(agent); err != nil {
		t.Fatalf("First RegisterAdapter returned unexpected error: %v", err)
	}
	if err := r.RegisterAdapter(agent); err == nil {
		t.Fatal("Expected error on duplicate adapter registration, got nil")
	}
}

func TestGetUnknownLensReturnsNil(t *testing.T) {
	r := New()
	got := r.GetLens("nonexistent")
	if got != nil {
		t.Fatalf("GetLens for unknown ID should return nil, got %v", got)
	}
}

func TestGetUnknownAdapterReturnsNil(t *testing.T) {
	r := New()
	got := r.GetAdapter("nonexistent")
	if got != nil {
		t.Fatalf("GetAdapter for unknown ID should return nil, got %v", got)
	}
}

func TestListAllAdapters(t *testing.T) {
	r := New()
	r.RegisterAdapter(&mockAgent{id: "agent-a", name: "Agent A", installed: true})
	r.RegisterAdapter(&mockAgent{id: "agent-b", name: "Agent B", installed: false})

	entries := r.ListAll()
	if len(entries) != 2 {
		t.Fatalf("ListAll() = %d entries, want 2", len(entries))
	}

	ids := make(map[string]bool)
	for _, e := range entries {
		ids[e.ID] = true
	}
	if !ids["agent-a"] {
		t.Error("ListAll() missing agent-a")
	}
	if !ids["agent-b"] {
		t.Error("ListAll() missing agent-b")
	}
}

func TestListAllAdapters_Empty(t *testing.T) {
	r := New()
	entries := r.ListAll()
	if entries == nil {
		t.Error("ListAll() on empty registry returned nil, want empty slice")
	}
	if len(entries) != 0 {
		t.Errorf("ListAll() on empty registry = %d entries, want 0", len(entries))
	}
}

func TestEmptyRegistry(t *testing.T) {
	r := New()
	if got := r.GetLens("anything"); got != nil {
		t.Error("Expected nil for lens on empty registry")
	}
	if got := r.GetAdapter("anything"); got != nil {
		t.Error("Expected nil for adapter on empty registry")
	}
}
