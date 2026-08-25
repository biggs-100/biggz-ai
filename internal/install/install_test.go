package install_test

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/assets"
	"github.com/biggs-100/biggz-ai/internal/install"
	"github.com/biggs-100/biggz-ai/internal/review"
	"github.com/biggs-100/biggz-ai/plugintest"
)

func TestInstall_AgentDetected(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	agent := &plugintest.FakeAgent{
		Installed:  true,
		BinaryPath: "/usr/local/bin/opencode",
	}

	result, err := install.Run(ctx, agent, install.Config{HomeDir: tmpDir})
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
	if result.PluginsDeployed == 0 {
		t.Error("expected PluginsDeployed > 0")
	}
	if result.DryRun {
		t.Error("expected DryRun=false")
	}

	// Skills live in ~/.biggz/skills/ — canonical source, no agent-dir conflict
	biggzSkillsDir := filepath.Join(tmpDir, ".biggz", "skills")
	if _, err := os.Stat(biggzSkillsDir); os.IsNotExist(err) {
		t.Error("biggs skills directory was not created under .biggz/skills/")
	}

	// Config goes to agent's settings file via adapter.SettingsPath(homeDir)
	configFile := filepath.Join(tmpDir, ".config", "opencode", "opencode.jsonc")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("config file was not created under .config/opencode/")
	}

	// Commands still deploy to agent's commands dir
	commandsDir := filepath.Join(tmpDir, ".config", "opencode", "commands")
	if _, err := os.Stat(commandsDir); os.IsNotExist(err) {
		t.Error("commands directory was not created on disk")
	}

	// Plugins deploy to the agent's global plugin dir (OpenCode auto-loads it)
	pluginsDir := filepath.Join(tmpDir, ".config", "opencode", "plugins")
	pluginFiles, _ := filepath.Glob(filepath.Join(pluginsDir, "*.ts"))
	if len(pluginFiles) == 0 {
		t.Error("no plugin .ts files were deployed to the agent plugins directory")
	}
	reviewPlugin := filepath.Join(pluginsDir, "review-result-artifacts.ts")
	if _, err := os.Stat(reviewPlugin); os.IsNotExist(err) {
		t.Error("review-result-artifacts.ts was not deployed to the agent plugins directory")
	}
	// All 3 OpenCode plugins ship (parity with gentle-ai): the reviewer
	// transport + SDD phase hooks, the startup skill-registry refresh, and
	// the model-variants cache writer.
	for _, name := range []string{
		"review-result-artifacts.ts",
		"skill-registry.ts",
		"model-variants.ts",
	} {
		if _, err := os.Stat(filepath.Join(pluginsDir, name)); os.IsNotExist(err) {
			t.Errorf("%s was not deployed to the agent plugins directory", name)
		}
	}
	if got := result.PluginsDeployed; got != len(pluginFiles) {
		t.Errorf("PluginsDeployed = %d, want %d plugin files on disk", got, len(pluginFiles))
	}

	// Verify at least one skill file exists under .biggz/skills/
	skillFiles, _ := filepath.Glob(filepath.Join(biggzSkillsDir, "**", "SKILL.md"))
	if len(skillFiles) == 0 {
		t.Error("no SKILL.md files were deployed to .biggz/skills/")
	}

	// Verify at least one command file exists
	cmdFiles, _ := filepath.Glob(filepath.Join(commandsDir, "*.md"))
	if len(cmdFiles) == 0 {
		t.Error("no command .md files were deployed")
	}

	// Skills are deployed to the agent's skills directory for OpenCode discovery
	agentSkillsDir := filepath.Join(tmpDir, ".config", "opencode", "skills")
	if _, err := os.Stat(agentSkillsDir); os.IsNotExist(err) {
		t.Error("agent skills directory was not created under .config/opencode/skills/")
	}
	agentSkillFiles, _ := filepath.Glob(filepath.Join(agentSkillsDir, "**", "SKILL.md"))
	if len(agentSkillFiles) == 0 {
		t.Error("no SKILL.md files were deployed to agent skills directory")
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

	result, err := install.Run(ctx, agent, install.Config{DryRun: true, HomeDir: tmpDir})
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
	if result.PluginsDeployed == 0 {
		t.Error("expected PluginsDeployed > 0 in dry-run")
	}

	// Verify NO files were actually written during dry-run
	if _, err := os.Stat(filepath.Join(tmpDir, ".biggz", "skills")); !os.IsNotExist(err) {
		t.Error("biggz skills directory should NOT exist after dry-run")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".config", "opencode", "opencode.jsonc")); !os.IsNotExist(err) {
		t.Error("config file should NOT exist after dry-run")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".config", "opencode", "commands")); !os.IsNotExist(err) {
		t.Error("commands directory should NOT exist after dry-run")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".config", "opencode", "plugins")); !os.IsNotExist(err) {
		t.Error("plugins directory should NOT exist after dry-run")
	}
}

func TestDeployPlugins_EmbeddedAssetWritten(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, ".config", "opencode", "plugins")

	// Dry-run: counts the embedded plugin without writing anything.
	count, err := install.DeployPlugins(pluginsDir, assets.FS, true)
	if err != nil {
		t.Fatalf("dry-run DeployPlugins error = %v", err)
	}
	if count == 0 {
		t.Fatal("dry-run DeployPlugins counted 0 plugins")
	}
	if _, err := os.Stat(pluginsDir); !os.IsNotExist(err) {
		t.Error("plugins directory should NOT exist after dry-run")
	}

	// Real deploy: every embedded plugin file lands with identical bytes.
	count, err = install.DeployPlugins(pluginsDir, assets.FS, false)
	if err != nil {
		t.Fatalf("DeployPlugins error = %v", err)
	}
	if count == 0 {
		t.Fatal("DeployPlugins deployed 0 plugins")
	}
	embedded, err := fs.Glob(assets.FS, "opencode/plugins/*.ts")
	if err != nil {
		t.Fatalf("glob embedded plugins: %v", err)
	}
	if count != len(embedded) {
		t.Errorf("DeployPlugins count = %d, want %d embedded plugins", count, len(embedded))
	}
	for _, name := range embedded {
		want, err := fs.ReadFile(assets.FS, name)
		if err != nil {
			t.Fatalf("read embedded %s: %v", name, err)
		}
		got, err := os.ReadFile(filepath.Join(pluginsDir, filepath.Base(name)))
		if err != nil {
			t.Fatalf("deployed %s missing: %v", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("deployed %s content differs from the embedded asset", name)
		}
	}
}

// TestDeployPlugins_AllThreeParityPluginsEmbedded verifies the full 3/3
// OpenCode plugin parity set with gentle-ai is embedded and deployed with
// identical bytes: review-result-artifacts, skill-registry, model-variants.
func TestDeployPlugins_AllThreeParityPluginsEmbedded(t *testing.T) {
	tmpDir := t.TempDir()
	pluginsDir := filepath.Join(tmpDir, ".config", "opencode", "plugins")

	count, err := install.DeployPlugins(pluginsDir, assets.FS, false)
	if err != nil {
		t.Fatalf("DeployPlugins error = %v", err)
	}
	embedded, err := fs.Glob(assets.FS, "opencode/plugins/*.ts")
	if err != nil {
		t.Fatalf("glob embedded plugins: %v", err)
	}
	if count != len(embedded) {
		t.Errorf("DeployPlugins count = %d, want %d embedded plugins", count, len(embedded))
	}
	names := make(map[string]bool)
	for _, name := range embedded {
		names[filepath.Base(name)] = true
	}
	for _, want := range []string{"review-result-artifacts.ts", "skill-registry.ts", "model-variants.ts"} {
		if !names[want] {
			t.Errorf("embedded plugins missing %q", want)
		}
	}
}

func TestInstall_Idempotent(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	agent := &plugintest.FakeAgent{
		Installed:  true,
		BinaryPath: "/usr/local/bin/opencode",
	}

	// First run
	first, err := install.Run(ctx, agent, install.Config{HomeDir: tmpDir})
	if err != nil {
		t.Fatalf("first run unexpected error: %v", err)
	}

	// Second run
	second, err := install.Run(ctx, agent, install.Config{HomeDir: tmpDir})
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

	// Skills now live in ~/.biggz/skills/ (canonical source)
	biggzSkillsDir := filepath.Join(tmpDir, ".biggz", "skills")
	if _, err := os.Stat(biggzSkillsDir); os.IsNotExist(err) {
		t.Error("skills directory was not created under .biggz/skills/")
	}

	// Config goes to agent's settings file via adapter.SettingsPath(homeDir)
	configFile := filepath.Join(tmpDir, ".config", "opencode", "opencode.jsonc")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("config file was not created under .config/opencode/")
	}

	// Commands still deploy to agent's commands dir
	commandsDir := filepath.Join(tmpDir, ".config", "opencode", "commands")
	if _, err := os.Stat(commandsDir); os.IsNotExist(err) {
		t.Error("commands directory was not created under .config/opencode/")
	}

	// Skills are deployed to the agent's skills directory for OpenCode discovery
	agentSkillsDir := filepath.Join(tmpDir, ".config", "opencode", "skills")
	if _, err := os.Stat(agentSkillsDir); os.IsNotExist(err) {
		t.Error("agent skills directory was not created under .config/opencode/skills/")
	}
	agentSkillFiles, _ := filepath.Glob(filepath.Join(agentSkillsDir, "**", "SKILL.md"))
	if len(agentSkillFiles) == 0 {
		t.Error("no SKILL.md files were deployed to agent skills directory")
	}
}

func TestInstall_EnsuresRDDEnabled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	// Start from disabled to prove install re-enables.
	if _, err := review.RDDDisable("", "", "global"); err != nil {
		t.Fatalf("pre-disable: %v", err)
	}
	status, err := review.RDDStatus("", "")
	if err != nil {
		t.Fatalf("RDDStatus pre: %v", err)
	}
	if status.EffectiveMode != review.RDDModeDisabled {
		t.Fatalf("pre: expected disabled, got %s", status.EffectiveMode)
	}

	ctx := context.Background()
	agent := &plugintest.FakeAgent{Installed: true, BinaryPath: "/usr/local/bin/opencode"}
	result, err := install.Run(ctx, agent, install.Config{HomeDir: home})
	if err != nil {
		t.Fatalf("install Run: %v", err)
	}
	if !result.AgentDetected {
		t.Fatal("expected AgentDetected=true")
	}

	// Global mode must be enabled after install.
	status, err = review.RDDStatus("", "")
	if err != nil {
		t.Fatalf("RDDStatus post: %v", err)
	}
	if status.EffectiveMode != review.RDDModeEnabled {
		t.Errorf("post: expected enabled, got %s (source=%s)", status.EffectiveMode, status.Source)
	}
	if status.GlobalMode != review.RDDModeEnabled {
		t.Errorf("post: expected global enabled, got %s", status.GlobalMode)
	}

	// File check: ~/.biggz/rdd-mode.json contains enabled.
	data, err := os.ReadFile(filepath.Join(home, ".biggz", "rdd-mode.json"))
	if err != nil {
		t.Fatalf("read rdd-mode.json: %v", err)
	}
	if !strings.Contains(string(data), `"mode": "enabled"`) {
		t.Errorf("rdd-mode.json missing enabled: %s", string(data))
	}

	// Idempotent second run still enabled.
	if _, err := install.Run(ctx, agent, install.Config{HomeDir: home}); err != nil {
		t.Fatalf("second Run: %v", err)
	}
	status, _ = review.RDDStatus("", "")
	if status.EffectiveMode != review.RDDModeEnabled {
		t.Errorf("second run: expected still enabled, got %s", status.EffectiveMode)
	}
}

func TestInstall_EnsuresRDDEnabled_DryRunDoesNotTouchRDD(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	if _, err := review.RDDDisable("", "", "global"); err != nil {
		t.Fatalf("pre-disable: %v", err)
	}

	ctx := context.Background()
	agent := &plugintest.FakeAgent{Installed: true, BinaryPath: "/usr/local/bin/opencode"}
	if _, err := install.Run(ctx, agent, install.Config{HomeDir: home, DryRun: true}); err != nil {
		t.Fatalf("dryRun: %v", err)
	}

	status, err := review.RDDStatus("", "")
	if err != nil {
		t.Fatalf("RDDStatus: %v", err)
	}
	if status.EffectiveMode != review.RDDModeDisabled {
		t.Errorf("dryRun should not enable RDD, got %s", status.EffectiveMode)
	}
}

func TestInstall_EnsuresRDDEnabled_ClearsStaleCloneDisable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	// Create a fake git dir and disable at clone scope.
	gitDir := filepath.Join(home, "repo.git")
	if err := os.MkdirAll(filepath.Join(gitDir, "objects"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := review.RDDDisable(gitDir, gitDir, "clone"); err != nil {
		t.Fatalf("clone disable: %v", err)
	}
	status, _ := review.RDDStatus(gitDir, gitDir)
	if status.EffectiveMode != review.RDDModeDisabled {
		t.Fatalf("expected clone disabled, got %s", status.EffectiveMode)
	}
	// Simulate what install does: clear via RDDEnable with the same git dirs.
	if _, err := review.RDDEnable(gitDir, gitDir); err != nil {
		t.Fatalf("RDDEnable clear: %v", err)
	}
	status, _ = review.RDDStatus(gitDir, gitDir)
	if status.EffectiveMode != review.RDDModeEnabled {
		t.Errorf("after clear: expected enabled, got %s", status.EffectiveMode)
	}
	if status.CloneMode != review.RDDModeUnset {
		t.Errorf("after clear: expected clone unset, got %s", status.CloneMode)
	}
}

func TestInstall_VerifiesOrchestratorCheckpoint(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	agent := &plugintest.FakeAgent{Installed: true, BinaryPath: "/usr/local/bin/opencode"}
	if _, err := install.Run(ctx, agent, install.Config{HomeDir: home}); err != nil {
		t.Fatalf("install Run: %v", err)
	}
	// Verify OpenCode settings contain checkpoint and permissions
	configFile := filepath.Join(home, ".config", "opencode", "opencode.jsonc")
	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	s := string(data)
	for _, want := range []string{"Post-Delegation Human Checkpoint", "Synthesize a concise summary"} {
		if !strings.Contains(s, want) {
			t.Errorf("config missing %q", want)
		}
	}
	if !strings.Contains(s, "ask_user_question") {
		t.Errorf("config missing ask_user_question permission")
	}
	if !strings.Contains(s, `"question"`) && !strings.Contains(s, "'question'") {
		t.Errorf("config missing question permission")
	}
	// Verify checkpoint at top 40 lines (hardening) — check first 4000 chars contains checkpoint
	prefix := s
	if len(prefix) > 4000 {
		prefix = prefix[:4000]
	}
	if !strings.Contains(prefix, "Post-Delegation Human Checkpoint") && !strings.Contains(s, "Post-Delegation Human Checkpoint") {
		t.Errorf("config should contain checkpoint near top")
	}
}

func TestInstall_VerifyCheckpointSynthesisHook(t *testing.T) {
	if err := install.VerifyCheckpointSynthesis("missing"); err == nil {
		t.Error("expected error for missing markers")
	}
	good := "Post-Delegation Human Checkpoint\nSynthesize a concise summary\nartifacts/paths\nrisks\nnext\n"
	if err := install.VerifyCheckpointSynthesis(good); err != nil {
		t.Errorf("expected pass for good synthesis: %v", err)
	}
	if err := install.VerifyAskUserQuestionPrecededBySynthesis(good); err != nil {
		t.Errorf("hook should pass for good synthesis: %v", err)
	}
	if err := install.VerifyAskUserQuestionPrecededBySynthesis("no checkpoint"); err == nil {
		t.Error("hook should fail for missing synthesis")
	}
}
