package uninstall_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/agents/claude"
	"github.com/biggs-100/biggz-ai/internal/install"
	"github.com/biggs-100/biggz-ai/internal/uninstall"
	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugin"
	"github.com/biggs-100/biggz-ai/plugintest"
)

func boolPtr(b bool) *bool { return &b }

// newFakeAgent builds an adapter that mirrors OpenCode's layout
// (~/.config/opencode/...) with every install-relevant capability on.
func newFakeAgent(id string) *plugintest.FakeAgent {
	return &plugintest.FakeAgent{
		AgentID:           model.AgentID(id),
		AgentName:         "Fake " + id,
		Installed:         true,
		BinaryPath:        "/usr/local/bin/" + id,
		AgentSkills:       boolPtr(true),
		AgentSystemPrompt: boolPtr(true),
		AgentMCP:          boolPtr(true),
		AgentMCPStrategy:  model.StrategyMergeIntoSettings,
	}
}

func adapters(agents ...plugin.AgentAdapter) map[string]plugin.AgentAdapter {
	m := make(map[string]plugin.AgentAdapter, len(agents))
	for _, a := range agents {
		m[string(a.ID())] = a
	}
	return m
}

// installedHome runs a full install and seeds user content that must survive
// an uninstall: bigmem store, backups, custom-agents.json, a user skill, a
// user settings key and user AGENTS.md content.
func installedHome(t *testing.T, agent *plugintest.FakeAgent) string {
	t.Helper()
	home := t.TempDir()

	if _, err := install.Run(context.Background(), agent, install.Config{HomeDir: home}); err != nil {
		t.Fatalf("install: %v", err)
	}

	// User data that uninstall must NEVER delete (unless --purge for the
	// first two).
	for _, p := range []string{
		filepath.Join(home, ".biggz", "bigmem", "bigmem.db"),
		filepath.Join(home, ".biggz", "backups", "backup-1.json"),
		filepath.Join(home, ".config", "biggz", "custom-agents.json"),
	} {
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
		}
		if err := os.WriteFile(p, []byte(`{"user": true}`), 0644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}

	// A user skill in the agent skills dir that is not biggz-owned.
	userSkill := filepath.Join(agent.SkillsDir(home), "my-user-skill", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(userSkill), 0755); err != nil {
		t.Fatalf("mkdir user skill: %v", err)
	}
	if err := os.WriteFile(userSkill, []byte("# my user skill\n"), 0644); err != nil {
		t.Fatalf("write user skill: %v", err)
	}

	// User content inside the settings file (comments + user keys) and
	// AGENTS.md (user sections) are seeded by the install fixtures below.
	return home
}

// userSettings returns a settings file containing biggz-owned keys plus user
// content that must survive byte-identically.
const userSettings = `{
  // user comment that must survive
  "default_agent": "biggz-orchestrator",
  "theme": "dark",
  "agent": {
    "biggz-orchestrator": { "mode": "primary", "description": "biggz orchestrator" },
    "user-agent": { "mode": "subagent", "description": "mine" }
  },
  "mcp": {
    "biggz": { "command": ["C:/home/u/.biggz/biggz-mcp.exe"], "type": "local" },
    "github": { "command": ["gh"], "type": "local" }
  }
}
`

const userSettingsExpected = `{
  // user comment that must survive
  "theme": "dark",
  "agent": {
    "user-agent": { "mode": "subagent", "description": "mine" }
  },
  "mcp": {
    "github": { "command": ["gh"], "type": "local" }
  }
}
`

// userAgentsMD contains biggz marker sections interleaved with user content.
const userAgentsMD = `# User Header
Keep this line.

<!-- biggz:persona -->
Biggz persona content.
<!-- /biggz:persona -->

User middle section.

<!-- biggz:bigmem-protocol -->
BigMem protocol content.
<!-- /biggz:bigmem-protocol -->

<!-- biggz:strict-tdd-mode -->
Strict TDD Mode: enabled
<!-- /biggz:strict-tdd-mode -->

# User Footer
Keep this line.
`

const userAgentsMDExpected = `# User Header
Keep this line.

User middle section.

# User Footer
Keep this line.
`

func TestUninstall_RemovesEverythingKeepsUserContent(t *testing.T) {
	agent := newFakeAgent("fake")
	home := installedHome(t, agent)

	settingsPath := filepath.Join(home, ".config", "opencode", "opencode.jsonc")
	if err := os.WriteFile(settingsPath, []byte(userSettings), 0644); err != nil {
		t.Fatalf("write settings fixture: %v", err)
	}
	agentsPath := filepath.Join(home, ".config", "opencode", "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte(userAgentsMD), 0644); err != nil {
		t.Fatalf("write AGENTS.md fixture: %v", err)
	}

	res, err := uninstall.Run(context.Background(), adapters(agent), uninstall.Config{HomeDir: home, Yes: true})
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if len(res.Failed) > 0 {
		t.Fatalf("unexpected failures: %v", res.Failed)
	}
	if res.RemovedFiles == 0 {
		t.Error("expected RemovedFiles > 0")
	}
	if res.RewrittenConfigs != 2 {
		t.Errorf("RewrittenConfigs = %d, want 2 (settings + AGENTS.md)", res.RewrittenConfigs)
	}

	// Agent-owned artifact dirs are gone or empty.
	assertMissing(t, filepath.Join(home, ".biggz", "skills"))
	assertMissing(t, filepath.Join(home, ".config", "opencode", "prompts"))
	assertMissing(t, filepath.Join(home, ".config", "opencode", "commands"))
	assertMissing(t, filepath.Join(home, ".config", "opencode", "plugins"))

	// Biggz skill files removed from the agent skills dir; the user skill
	// survives.
	assertMissing(t, filepath.Join(home, ".config", "opencode", "skills", "sdd-init", "SKILL.md"))
	assertExists(t, filepath.Join(home, ".config", "opencode", "skills", "my-user-skill", "SKILL.md"))

	// Settings: biggz keys removed, everything else byte-identical.
	gotSettings, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings after uninstall: %v", err)
	}
	if !bytes.Equal(gotSettings, []byte(userSettingsExpected)) {
		t.Errorf("settings not byte-identical:\ngot:\n%s\nwant:\n%s", gotSettings, userSettingsExpected)
	}

	// AGENTS.md: biggz sections removed, everything else byte-identical.
	gotAgents, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read AGENTS.md after uninstall: %v", err)
	}
	if !bytes.Equal(gotAgents, []byte(userAgentsMDExpected)) {
		t.Errorf("AGENTS.md not byte-identical:\ngot:\n%s\nwant:\n%s", gotAgents, userAgentsMDExpected)
	}

	// User data kept: bigmem, backups, custom-agents.json.
	assertExists(t, filepath.Join(home, ".biggz", "bigmem", "bigmem.db"))
	assertExists(t, filepath.Join(home, ".biggz", "backups", "backup-1.json"))
	assertExists(t, filepath.Join(home, ".config", "biggz", "custom-agents.json"))

	if !strings.Contains(res.Summary, "1 agents uninstalled, 0 failed, kept: bigmem, backups, custom-agents") {
		t.Errorf("summary = %q", res.Summary)
	}
}

func TestUninstall_DryRunChangesNothing(t *testing.T) {
	agent := newFakeAgent("fake")
	home := installedHome(t, agent)

	settingsPath := filepath.Join(home, ".config", "opencode", "opencode.jsonc")
	before, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	agentsPath := filepath.Join(home, ".config", "opencode", "AGENTS.md")
	agentsBefore, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}

	res, err := uninstall.Run(context.Background(), adapters(agent), uninstall.Config{HomeDir: home, Yes: true, DryRun: true})
	if err != nil {
		t.Fatalf("dry-run uninstall: %v", err)
	}
	if res.RemovedFiles == 0 {
		t.Error("dry-run should still report removals")
	}

	// Nothing changed on disk.
	assertExists(t, filepath.Join(home, ".biggz", "skills"))
	assertExists(t, filepath.Join(home, ".config", "opencode", "skills", "sdd-init", "SKILL.md"))
	assertExists(t, filepath.Join(home, ".config", "opencode", "prompts", "sdd", "sdd-init.md"))
	assertExists(t, filepath.Join(home, ".config", "opencode", "commands", "sdd-init.md"))
	assertExists(t, filepath.Join(home, ".config", "opencode", "plugins", "model-variants.ts"))
	assertExists(t, filepath.Join(home, ".biggz", "bigmem", "bigmem.db"))

	after, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings after dry-run: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Error("settings changed during dry-run")
	}
	agentsAfter, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read AGENTS.md after dry-run: %v", err)
	}
	if !bytes.Equal(agentsBefore, agentsAfter) {
		t.Error("AGENTS.md changed during dry-run")
	}
}

// TestUninstall_FailureInjection makes one removal impossible (a skill file
// is replaced by a non-empty directory, so os.Remove fails with "directory
// not empty" on every platform) and asserts every OTHER operation is still
// attempted and reported.
func TestUninstall_FailureInjectionContinuesOtherOps(t *testing.T) {
	agent := newFakeAgent("fake")
	home := installedHome(t, agent)

	blocked := filepath.Join(home, ".config", "opencode", "skills", "sdd-init", "SKILL.md")
	if err := os.RemoveAll(blocked); err != nil {
		t.Fatalf("remove skill file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(blocked, "sub"), 0755); err != nil {
		t.Fatalf("replace skill file with dir: %v", err)
	}

	res, err := uninstall.Run(context.Background(), adapters(agent), uninstall.Config{HomeDir: home, Yes: true})
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if len(res.Failed) == 0 {
		t.Fatal("expected failures from the injected broken path")
	}
	for _, f := range res.Failed {
		if f.Agent != "fake" {
			t.Errorf("failure agent = %q, want fake", f.Agent)
		}
		if !strings.Contains(f.Op, "skill") {
			t.Errorf("failure op = %q, want a skill removal", f.Op)
		}
	}

	// Every other operation still ran: prompts, commands, plugins, settings,
	// AGENTS.md, and the shared ~/.biggz store.
	assertMissing(t, filepath.Join(home, ".config", "opencode", "prompts"))
	assertMissing(t, filepath.Join(home, ".config", "opencode", "commands"))
	assertMissing(t, filepath.Join(home, ".config", "opencode", "plugins"))
	assertMissing(t, filepath.Join(home, ".biggz", "skills"))
	gotSettings, err := os.ReadFile(filepath.Join(home, ".config", "opencode", "opencode.jsonc"))
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if strings.Contains(string(gotSettings), "biggz-orchestrator") {
		t.Error("settings still contain biggz-orchestrator after uninstall")
	}
	assertExists(t, filepath.Join(home, ".biggz", "bigmem", "bigmem.db"))

	if !strings.Contains(res.Summary, "1 agents uninstalled, 1 failed") {
		t.Errorf("summary = %q", res.Summary)
	}
}

func TestUninstall_PurgeRemovesBigmemAndBackups(t *testing.T) {
	agent := newFakeAgent("fake")
	home := installedHome(t, agent)

	res, err := uninstall.Run(context.Background(), adapters(agent), uninstall.Config{HomeDir: home, Yes: true, Purge: true})
	if err != nil {
		t.Fatalf("purge uninstall: %v", err)
	}
	if len(res.Failed) > 0 {
		t.Fatalf("unexpected failures: %v", res.Failed)
	}

	assertMissing(t, filepath.Join(home, ".biggz"))
	assertMissing(t, filepath.Join(home, ".biggz", "bigmem"))
	assertMissing(t, filepath.Join(home, ".biggz", "backups"))
	// custom-agents.json lives outside ~/.biggz and is never deleted.
	assertExists(t, filepath.Join(home, ".config", "biggz", "custom-agents.json"))

	if !strings.Contains(res.Summary, "kept: custom-agents") {
		t.Errorf("summary = %q", res.Summary)
	}
}

func TestUninstall_AgentRestriction(t *testing.T) {
	agentA := newFakeAgent("agent-a")
	agentB := newFakeAgent("agent-b")

	// Two agents sharing one home (different config roots via SetTempDir).
	home := t.TempDir()
	rootA := filepath.Join(home, "config-a")
	rootB := filepath.Join(home, "config-b")
	agentA.SetTempDir(rootA)
	agentB.SetTempDir(rootB)
	_, err := install.Run(context.Background(), agentA, install.Config{HomeDir: home})
	if err != nil {
		t.Fatalf("install A: %v", err)
	}
	_, err = install.Run(context.Background(), agentB, install.Config{HomeDir: home})
	if err != nil {
		t.Fatalf("install B: %v", err)
	}

	res, err := uninstall.Run(context.Background(), adapters(agentA, agentB), uninstall.Config{
		HomeDir: home, Yes: true, AgentID: "agent-a",
	})
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if len(res.Failed) > 0 {
		t.Fatalf("unexpected failures: %v", res.Failed)
	}

	assertMissing(t, filepath.Join(rootA, "skills", "sdd-init", "SKILL.md"))
	assertExists(t, filepath.Join(rootB, "skills", "sdd-init", "SKILL.md"))
	// The shared canonical store is removed even with one agent selected.
	assertMissing(t, filepath.Join(home, ".biggz", "skills"))
	if !strings.Contains(res.Summary, "1 agents uninstalled") {
		t.Errorf("summary = %q", res.Summary)
	}
}

func TestUninstall_Confirmation(t *testing.T) {
	agent := newFakeAgent("fake")
	home := installedHome(t, agent)

	// Declined confirmation cancels without touching anything.
	_, err := uninstall.Run(context.Background(), adapters(agent), uninstall.Config{
		HomeDir: home, Confirm: strings.NewReader("n\n"),
	})
	if err != uninstall.ErrCancelled {
		t.Fatalf("err = %v, want ErrCancelled", err)
	}
	assertExists(t, filepath.Join(home, ".biggz", "skills"))

	// Accepted confirmation proceeds.
	res, err := uninstall.Run(context.Background(), adapters(agent), uninstall.Config{
		HomeDir: home, Confirm: strings.NewReader("y\n"),
	})
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if len(res.Failed) > 0 {
		t.Fatalf("unexpected failures: %v", res.Failed)
	}
	assertMissing(t, filepath.Join(home, ".biggz", "skills"))
}

func TestUninstall_RequiresYesNonInteractive(t *testing.T) {
	stat, err := os.Stdin.Stat()
	if err != nil || (stat.Mode()&os.ModeCharDevice) != 0 {
		t.Skip("stdin is a TTY; the non-interactive path cannot be exercised")
	}
	_, err = uninstall.Run(context.Background(), adapters(newFakeAgent("fake")), uninstall.Config{
		HomeDir: t.TempDir(),
	})
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("err = %v, want a --yes requirement error", err)
	}
}

func TestUninstall_UnknownAgent(t *testing.T) {
	_, err := uninstall.Run(context.Background(), adapters(newFakeAgent("fake")), uninstall.Config{
		HomeDir: t.TempDir(), Yes: true, AgentID: "nope",
	})
	if err == nil || !strings.Contains(err.Error(), "unknown agent") {
		t.Fatalf("err = %v, want unknown agent error", err)
	}
}

// TestUninstall_ClaudeSeparateMCPFile verifies the SeparateMCPFiles strategy:
// ~/.claude/mcp/biggz.json is removed while ~/.claude/user-file stays. The
// real claude adapter is used because its MCPConfigPath resolves to
// ~/.claude/mcp/<name>.json (FakeAgent always returns the settings path).
func TestUninstall_ClaudeSeparateMCPFile(t *testing.T) {
	claudeAgent := claude.NewAdapter()
	home := t.TempDir()

	claudeDir := filepath.Join(home, ".claude")
	mcpDir := filepath.Join(claudeDir, "mcp")
	if err := os.MkdirAll(mcpDir, 0755); err != nil {
		t.Fatalf("mkdir mcp: %v", err)
	}
	biggzMcp := filepath.Join(mcpDir, "biggz.json")
	if err := os.WriteFile(biggzMcp, []byte(`{"biggz": {"command": ["/home/u/.biggz/biggz-mcp.exe"]}}`), 0600); err != nil {
		t.Fatalf("write biggz mcp: %v", err)
	}
	userFile := filepath.Join(claudeDir, "user-file.txt")
	if err := os.WriteFile(userFile, []byte("mine"), 0644); err != nil {
		t.Fatalf("write user file: %v", err)
	}

	res, err := uninstall.Run(context.Background(), adapters(claudeAgent), uninstall.Config{HomeDir: home, Yes: true})
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if len(res.Failed) > 0 {
		t.Fatalf("unexpected failures: %v", res.Failed)
	}
	assertMissing(t, biggzMcp)
	assertExists(t, userFile)
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected %s to exist: %v", path, err)
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("expected %s to be removed", path)
	} else if !os.IsNotExist(err) {
		t.Errorf("stat %s: %v", path, err)
	}
}
