package opencode

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/biggz-ai/biggz/plugin"
)

func TestNewAdapter(t *testing.T) {
	a := NewAdapter()
	if a.ID() != "opencode" {
		t.Errorf("ID() = %q, want %q", a.ID(), "opencode")
	}
	if a.Name() != "OpenCode" {
		t.Errorf("Name() = %q, want %q", a.Name(), "OpenCode")
	}
}

func TestDetect_Found(t *testing.T) {
	a := &Adapter{
		lookPath: func(name string) (string, error) {
			return "/usr/local/bin/" + name, nil
		},
	}
	path, err := a.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}
	if path != "/usr/local/bin/opencode" {
		t.Errorf("Detect() = %q, want %q", path, "/usr/local/bin/opencode")
	}
}

func TestDetect_NotFound(t *testing.T) {
	a := &Adapter{
		lookPath: func(name string) (string, error) {
			return "", exec.ErrNotFound
		},
	}
	_, err := a.Detect(context.Background())
	if err == nil {
		t.Fatal("Detect() expected error, got nil")
	}
	if !errors.Is(err, exec.ErrNotFound) {
		t.Errorf("Detect() error = %v, want exec.ErrNotFound", err)
	}
}

func TestCapabilities(t *testing.T) {
	a := NewAdapter()
	caps := a.Capabilities()
	if len(caps) == 0 {
		t.Fatal("Capabilities() returned empty slice")
	}

	// Verify expected capabilities are present
	expected := map[string]bool{
		"skills":         false,
		"mcp":            false,
		"sub_agents":     false,
		"system_prompt":  false,
		"slash_commands": false,
	}
	for _, c := range caps {
		if _, ok := expected[string(c)]; ok {
			expected[string(c)] = true
		}
	}
	for capName, found := range expected {
		if !found {
			t.Errorf("Capabilities() missing %q", capName)
		}
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

func TestSkillsDir(t *testing.T) {
	a := NewAdapter()
	dir := a.SkillsDir("/home/user")
	want := filepath.Join("/home/user", ".config", "opencode", "skills")
	if dir != want {
		t.Errorf("SkillsDir() = %q, want %q", dir, want)
	}
}

func TestSettingsPath(t *testing.T) {
	a := NewAdapter()
	path := a.SettingsPath("/home/user")
	want := filepath.Join("/home/user", ".config", "opencode", "opencode.jsonc")
	if path != want {
		t.Errorf("SettingsPath() = %q, want %q", path, want)
	}
}

func TestDeployConfig_Noop(t *testing.T) {
	a := NewAdapter()
	err := a.DeployConfig(context.Background(), plugin.AgentConfig{})
	if err != nil {
		t.Errorf("DeployConfig() = %v, want nil", err)
	}
}
