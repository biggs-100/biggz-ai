package install_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/biggz-ai/biggz/internal/install"
	"github.com/biggz-ai/biggz/plugintest"
)

func TestInstall_AgentDetected(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	agent := &plugintest.FakeAgent{
		Installed:  true,
		BinaryPath: "/usr/local/bin/opencode",
	}
	agent.SetTempDir(tmpDir)

	result, err := install.Run(ctx, agent, install.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.AgentDetected {
		t.Error("expected AgentDetected=true")
	}
	if result.BinaryPath != "/usr/local/bin/opencode" {
		t.Errorf("expected BinaryPath=/usr/local/bin/opencode, got %q", result.BinaryPath)
	}
	if result.SkillsDeployed == 0 {
		t.Error("expected SkillsDeployed > 0")
	}
	if !result.ConfigMerged {
		t.Error("expected ConfigMerged=true")
	}
	if result.CommandsWritten == 0 {
		t.Error("expected CommandsWritten > 0")
	}
	if result.DryRun {
		t.Error("expected DryRun=false")
	}

	// Verify files were actually written to the temp dir
	if _, err := os.Stat(filepath.Join(tmpDir, "skills")); os.IsNotExist(err) {
		t.Error("skills directory was not created on disk")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "opencode.jsonc")); os.IsNotExist(err) {
		t.Error("config file was not created on disk")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "commands")); os.IsNotExist(err) {
		t.Error("commands directory was not created on disk")
	}

	// Verify at least one skill file exists
	skillFiles, _ := filepath.Glob(filepath.Join(tmpDir, "skills", "**", "SKILL.md"))
	if len(skillFiles) == 0 {
		t.Error("no SKILL.md files were deployed")
	}

	// Verify at least one command file exists
	cmdFiles, _ := filepath.Glob(filepath.Join(tmpDir, "commands", "*.md"))
	if len(cmdFiles) == 0 {
		t.Error("no command .md files were deployed")
	}
}

func TestInstall_AgentNotDetected(t *testing.T) {
	ctx := context.Background()

	agent := &plugintest.FakeAgent{
		Installed: false,
	}

	result, err := install.Run(ctx, agent, install.Config{})
	if err == nil {
		t.Fatal("expected error for undetected agent, got nil")
	}
	if result == nil {
		t.Fatal("expected non-nil result even on error")
	}
	if result.AgentDetected {
		t.Error("expected AgentDetected=false for undetected agent")
	}
}

func TestInstall_DryRun(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	agent := &plugintest.FakeAgent{
		Installed:  true,
		BinaryPath: "/usr/local/bin/opencode",
	}
	agent.SetTempDir(tmpDir)

	result, err := install.Run(ctx, agent, install.Config{DryRun: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.DryRun {
		t.Error("expected DryRun=true")
	}
	if result.SkillsDeployed == 0 {
		t.Error("expected SkillsDeployed > 0 in dry-run")
	}
	if !result.ConfigMerged {
		t.Error("expected ConfigMerged=true in dry-run")
	}
	if result.CommandsWritten == 0 {
		t.Error("expected CommandsWritten > 0 in dry-run")
	}

	// Verify NO files were actually written during dry-run
	if _, err := os.Stat(filepath.Join(tmpDir, "skills")); !os.IsNotExist(err) {
		t.Error("skills directory should NOT exist after dry-run")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "opencode.jsonc")); !os.IsNotExist(err) {
		t.Error("config file should NOT exist after dry-run")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "commands")); !os.IsNotExist(err) {
		t.Error("commands directory should NOT exist after dry-run")
	}
}

func TestInstall_Idempotent(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	agent := &plugintest.FakeAgent{
		Installed:  true,
		BinaryPath: "/usr/local/bin/opencode",
	}
	agent.SetTempDir(tmpDir)

	// First run
	first, err := install.Run(ctx, agent, install.Config{})
	if err != nil {
		t.Fatalf("first run unexpected error: %v", err)
	}

	// Second run
	second, err := install.Run(ctx, agent, install.Config{})
	if err != nil {
		t.Fatalf("second run unexpected error: %v", err)
	}

	if first.SkillsDeployed != second.SkillsDeployed {
		t.Errorf("SkillsDeployed changed: first=%d second=%d", first.SkillsDeployed, second.SkillsDeployed)
	}
	if first.CommandsWritten != second.CommandsWritten {
		t.Errorf("CommandsWritten changed: first=%d second=%d", first.CommandsWritten, second.CommandsWritten)
	}
	if first.ConfigMerged != second.ConfigMerged {
		t.Errorf("ConfigMerged changed: first=%v second=%v", first.ConfigMerged, second.ConfigMerged)
	}
}

func TestInstall_CustomHomeDir(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	agent := &plugintest.FakeAgent{
		Installed:  true,
		BinaryPath: "/usr/local/bin/opencode",
	}

	// Use Config.HomeDir instead of SetTempDir — paths resolve via homeDir
	result, err := install.Run(ctx, agent, install.Config{HomeDir: tmpDir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.AgentDetected {
		t.Error("expected AgentDetected=true")
	}
	if result.SkillsDeployed == 0 {
		t.Error("expected SkillsDeployed > 0")
	}
	if !result.ConfigMerged {
		t.Error("expected ConfigMerged=true")
	}
	if result.CommandsWritten == 0 {
		t.Error("expected CommandsWritten > 0")
	}

	// Files should be under homeDir/.config/opencode/ since we didn't use SetTempDir
	configDir := filepath.Join(tmpDir, ".config", "opencode")
	if _, err := os.Stat(filepath.Join(configDir, "skills")); os.IsNotExist(err) {
		t.Error("skills directory was not created under homeDir/.config/opencode/")
	}
	if _, err := os.Stat(filepath.Join(configDir, "opencode.jsonc")); os.IsNotExist(err) {
		t.Error("config file was not created under homeDir/.config/opencode/")
	}
	if _, err := os.Stat(filepath.Join(configDir, "commands")); os.IsNotExist(err) {
		t.Error("commands directory was not created under homeDir/.config/opencode/")
	}
}
