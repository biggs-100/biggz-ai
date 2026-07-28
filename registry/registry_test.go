package registry

import (
	"context"
	"fmt"
	"testing"

	"github.com/biggz-ai/biggz/model"
	"github.com/biggz-ai/biggz/plugin"
)

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
	id   string
	name string
	installed bool
}

func (m *mockAgent) ID() string                            { return m.id }
func (m *mockAgent) Name() string                          { return m.name }
func (m *mockAgent) Detect(_ context.Context) (string, error) {
	if !m.installed {
		return "", fmt.Errorf("not found")
	}
	return "/usr/local/bin/" + m.id, nil
}
func (m *mockAgent) Capabilities() []plugin.Capability {
	return []plugin.Capability{plugin.CapSkills, plugin.CapMCP}
}
func (m *mockAgent) DeployConfig(_ context.Context, _ plugin.AgentConfig) error {
	if !m.installed {
		return fmt.Errorf("cannot deploy, not installed")
	}
	return nil
}
func (m *mockAgent) GlobalConfigDir(homeDir string) string {
	return homeDir + "/.config/" + m.id
}
func (m *mockAgent) SkillsDir(homeDir string) string {
	return homeDir + "/.config/" + m.id + "/skills"
}
func (m *mockAgent) SettingsPath(homeDir string) string {
	return homeDir + "/.config/" + m.id + "/" + m.id + ".json"
}

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

func TestRegisterAndGetAgent(t *testing.T) {
	r := New()
	agent := &mockAgent{id: "test-agent", name: "Test Agent", installed: true}

	err := r.RegisterAgent(agent)
	if err != nil {
		t.Fatalf("RegisterAgent returned unexpected error: %v", err)
	}

	got := r.GetAgent("test-agent")
	if got == nil {
		t.Fatal("GetAgent returned nil for registered agent")
	}
	if got.ID() != "test-agent" {
		t.Errorf("GetAgent ID = %q, want %q", got.ID(), "test-agent")
	}
	if got.Name() != "Test Agent" {
		t.Errorf("GetAgent Name = %q, want %q", got.Name(), "Test Agent")
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

func TestDuplicateAgentRegistration(t *testing.T) {
	r := New()
	agent := &mockAgent{id: "dup-agent", name: "Dup Agent"}

	if err := r.RegisterAgent(agent); err != nil {
		t.Fatalf("First RegisterAgent returned unexpected error: %v", err)
	}
	if err := r.RegisterAgent(agent); err == nil {
		t.Fatal("Expected error on duplicate agent registration, got nil")
	}
}

func TestGetUnknownLensReturnsNil(t *testing.T) {
	r := New()
	got := r.GetLens("nonexistent")
	if got != nil {
		t.Fatalf("GetLens for unknown ID should return nil, got %v", got)
	}
}

func TestGetUnknownAgentReturnsNil(t *testing.T) {
	r := New()
	got := r.GetAgent("nonexistent")
	if got != nil {
		t.Fatalf("GetAgent for unknown ID should return nil, got %v", got)
	}
}

func TestEmptyRegistry(t *testing.T) {
	r := New()
	if got := r.GetLens("anything"); got != nil {
		t.Error("Expected nil for lens on empty registry")
	}
	if got := r.GetAgent("anything"); got != nil {
		t.Error("Expected nil for agent on empty registry")
	}
}
