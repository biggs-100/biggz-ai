package agents

import (
	"context"
	"testing"

	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugin"
)

// -- mocks -------------------------------------------------------------------

type mockAdapter struct {
	id   string
	name string
}

func (m *mockAdapter) ID() model.AgentID                                   { return model.AgentID(m.id) }
func (m *mockAdapter) Name() string                                        { return m.name }
func (m *mockAdapter) Tier() model.SupportTier                             { return model.TierFull }
func (m *mockAdapter) Detect(_ context.Context, _ string) (bool, string, string, bool, error) {
	return true, "/bin/" + m.id, "", false, nil
}
func (m *mockAdapter) InstallCommand(_ interface{}) ([][]string, error)    { return nil, nil }
func (m *mockAdapter) Capabilities() []string                              { return []string{"skills"} }
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
func (m *mockAdapter) GlobalConfigDir(homeDir string) string               { return homeDir + "/." + m.id }
func (m *mockAdapter) SystemPromptDir(homeDir string) string               { return homeDir + "/." + m.id }
func (m *mockAdapter) SystemPromptFile(homeDir string) string              { return homeDir + "/." + m.id + "/AGENTS.md" }
func (m *mockAdapter) SkillsDir(homeDir string) string                     { return homeDir + "/." + m.id + "/skills" }
func (m *mockAdapter) CommandsDir(homeDir string) string                   { return homeDir + "/." + m.id + "/commands" }
func (m *mockAdapter) SubAgentsDir(homeDir string) string                  { return "" }
func (m *mockAdapter) EmbeddedSubAgentsDir() string                        { return "" }
func (m *mockAdapter) OutputStyleDir(homeDir string) string                { return "" }
func (m *mockAdapter) SettingsPath(homeDir string) string                  { return homeDir + "/." + m.id + "/settings.json" }
func (m *mockAdapter) MCPConfigPath(homeDir string, _ string) string       { return homeDir + "/." + m.id + "/settings.json" }

// validAgentID returns an AgentID that exists in the canonical manifest.
// Repeated calls return distinct IDs so tests can register multiple adapters.
var validAgentID = func() func() model.AgentID {
	ids := []model.AgentID{"opencode", "claude-code", "qwen-code", "cursor"}
	i := 0
	return func() model.AgentID {
		id := ids[i%len(ids)]
		i++
		return id
	}
}()

func TestNewDefaultRegistry(t *testing.T) {
	r, err := NewDefaultRegistry()
	if err != nil {
		t.Fatalf("NewDefaultRegistry() returned error: %v", err)
	}
	entries := r.ListAll()
	if entries == nil {
		t.Error("ListAll() returned nil, want empty slice")
	}
}

func TestRegistryRegisterAndList(t *testing.T) {
	r := NewRegistry()
	id := validAgentID()
	err := r.Register(id, func() plugin.AgentAdapter {
		return &mockAdapter{id: string(id), name: "Test Agent"}
	})
	if err != nil {
		t.Fatalf("Register() returned error: %v", err)
	}

	entries := r.ListAll()
	if len(entries) != 1 {
		t.Fatalf("ListAll() = %d entries, want 1", len(entries))
	}
	if entries[0].ID != string(id) {
		t.Errorf("ListAll()[0].ID = %q, want %q", entries[0].ID, string(id))
	}
}

func TestRegistryDuplicateOverwrites(t *testing.T) {
	r := NewRegistry()
	id := validAgentID()
	if err := r.Register(id, func() plugin.AgentAdapter {
		return &mockAdapter{id: string(id), name: "First"}
	}); err != nil {
		t.Fatalf("Register() returned error: %v", err)
	}
	if err := r.Register(id, func() plugin.AgentAdapter {
		return &mockAdapter{id: string(id), name: "Second"}
	}); err != nil {
		t.Fatalf("Register() returned error: %v", err)
	}

	entries := r.ListAll()
	if len(entries) != 1 {
		t.Fatalf("ListAll() = %d entries, want 1", len(entries))
	}
	a, ok := r.Get(id)
	if !ok {
		t.Fatal("Get() returned false for registered ID")
	}
	if a.Name() != "Second" {
		t.Errorf("Get().Name() = %q, want %q", a.Name(), "Second")
	}
}

func TestRegistryGetReturnsFalseForUnknown(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Get("unknown")
	if ok {
		t.Error("Get(\"unknown\") returned true, want false")
	}
}

func TestRegistryRegister_UnknownAgentID(t *testing.T) {
	r := NewRegistry()
	err := r.Register("unknown-agent", func() plugin.AgentAdapter {
		return &mockAdapter{id: "unknown-agent", name: "Unknown"}
	})
	if err == nil {
		t.Error("Register(\"unknown-agent\") expected error, got nil")
	}
}

func TestNewAdapter(t *testing.T) {
	id := validAgentID()
	Register(id, func() plugin.AgentAdapter {
		return &mockAdapter{id: string(id), name: "Mock Agent"}
	})

	a, err := NewAdapter(id)
	if err != nil {
		t.Fatalf("NewAdapter(%q) returned error: %v", id, err)
	}
	if a.ID() != id {
		t.Errorf("ID() = %q, want %q", a.ID(), id)
	}
}

func TestNewAdapterUnknown(t *testing.T) {
	_, err := NewAdapter("nonexistent")
	if err == nil {
		t.Fatal("NewAdapter(\"nonexistent\") expected error, got nil")
	}
}

func TestDetectInstalled(t *testing.T) {
	r := NewRegistry()
	id := validAgentID()
	_ = r.Register(id, func() plugin.AgentAdapter {
		return &mockAdapter{id: string(id), name: "Detectable"}
	})

	agents := DetectInstalled(context.Background(), r, "/home/user")
	if len(agents) == 0 {
		t.Fatal("DetectInstalled() returned empty slice, want at least 1")
	}
	if agents[0].ID != id {
		t.Errorf("DetectInstalled()[0].ID = %q, want %q", agents[0].ID, id)
	}
}

func TestDetectInstalledEmptyRegistry(t *testing.T) {
	r := NewRegistry()
	agents := DetectInstalled(context.Background(), r, "/home/user")
	if len(agents) != 0 {
		t.Errorf("DetectInstalled() on empty registry = %d agents, want 0", len(agents))
	}
}
