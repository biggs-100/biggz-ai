package qwen

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/biggz-ai/biggz/plugin"
)

func TestID(t *testing.T) {
	a := NewAdapter()
	if got := a.ID(); got != "qwen" {
		t.Errorf("ID() = %q, want %q", got, "qwen")
	}
}

func TestName(t *testing.T) {
	a := NewAdapter()
	if got := a.Name(); got != "Qwen" {
		t.Errorf("Name() = %q, want %q", got, "Qwen")
	}
}

func TestDetect_Found(t *testing.T) {
	a := &Adapter{lookPath: func(name string) (string, error) {
		return "/usr/local/bin/" + name, nil
	}}
	path, err := a.Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() unexpected error: %v", err)
	}
	if path != "/usr/local/bin/qwen" {
		t.Errorf("Detect() = %q, want %q", path, "/usr/local/bin/qwen")
	}
}

func TestDetect_NotFound(t *testing.T) {
	a := &Adapter{lookPath: func(name string) (string, error) {
		return "", errors.New("not found")
	}}
	_, err := a.Detect(context.Background())
	if err == nil {
		t.Fatal("Detect() expected error, got nil")
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

func TestGlobalConfigDir(t *testing.T) {
	a := NewAdapter()
	got := a.GlobalConfigDir("/home/user")
	want := filepath.Join("/home/user", ".qwen")
	if got != want {
		t.Errorf("GlobalConfigDir() = %q, want %q", got, want)
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

func TestSettingsPath(t *testing.T) {
	a := NewAdapter()
	got := a.SettingsPath("/home/user")
	want := filepath.Join("/home/user", ".qwen", "settings.json")
	if got != want {
		t.Errorf("SettingsPath() = %q, want %q", got, want)
	}
}

func TestDeployConfig(t *testing.T) {
	a := NewAdapter()
	err := a.DeployConfig(context.Background(), plugin.AgentConfig{})
	if err != nil {
		t.Fatalf("DeployConfig() unexpected error: %v", err)
	}
}
