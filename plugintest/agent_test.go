package plugintest

import (
	"path/filepath"
	"testing"
)

func TestFakeAgentDefaults(t *testing.T) {
	a := &FakeAgent{}
	if a.ID() != "fake-agent" {
		t.Errorf("ID() = %q, want %q", a.ID(), "fake-agent")
	}
	if a.Name() != "Fake Agent" {
		t.Errorf("Name() = %q, want %q", a.Name(), "Fake Agent")
	}
}

func TestFakeAgentDetect_Installed(t *testing.T) {
	a := &FakeAgent{Installed: true}
	path, err := a.Detect(nil)
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}
	if path == "" {
		t.Fatal("Detect() returned empty path")
	}
}

func TestFakeAgentDetect_NotInstalled(t *testing.T) {
	a := &FakeAgent{Installed: false}
	_, err := a.Detect(nil)
	if err == nil {
		t.Fatal("Detect() expected error for not installed agent")
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
