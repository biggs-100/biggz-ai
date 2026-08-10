package update_test

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/assets"
	"github.com/biggs-100/biggz-ai/internal/update"
	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugintest"
)

func boolPtr(b bool) *bool { return &b }

func fakeAgent() *plugintest.FakeAgent {
	return &plugintest.FakeAgent{
		AgentID:           "fake",
		AgentName:         "Fake",
		Installed:         true,
		BinaryPath:        "/usr/local/bin/fake",
		AgentSystemPrompt: boolPtr(true),
		AgentMCP:          boolPtr(true),
		AgentMCPStrategy:  model.StrategyMergeIntoSettings,
	}
}

// TestReconcile_DeploysFreshAssets asserts the full install-equivalent set
// is redeployed and that written bytes match the embedded assets.
func TestReconcile_DeploysFreshAssets(t *testing.T) {
	agent := fakeAgent()
	home := t.TempDir()

	rr, err := update.Reconcile(context.Background(), agent, home)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if rr.Skills == 0 {
		t.Error("expected Skills > 0")
	}
	if rr.Commands == 0 {
		t.Error("expected Commands > 0")
	}
	if rr.Plugins == 0 {
		t.Error("expected Plugins > 0")
	}
	if rr.Prompts == 0 {
		t.Error("expected Prompts > 0")
	}
	if !rr.ConfigMerged {
		t.Error("expected ConfigMerged=true")
	}
	// No biggz-mcp.exe source binary next to the test executable, so the
	// MCP binary/config are not deployed in tests.
	if rr.MCPDeployed {
		t.Error("expected MCPDeployed=false without a source binary")
	}

	// A couple of deployed files must be byte-identical to the embedded
	// assets: one skill in the canonical store and one command in the agent
	// commands dir.
	skillData, err := fs.ReadFile(assets.FS, "skills/sdd-apply/SKILL.md")
	if err != nil {
		t.Fatalf("read embedded skill: %v", err)
	}
	gotSkill, err := os.ReadFile(filepath.Join(home, ".biggz", "skills", "sdd-apply", "SKILL.md"))
	if err != nil {
		t.Fatalf("read deployed skill: %v", err)
	}
	if !bytes.Equal(gotSkill, skillData) {
		t.Error("deployed skill bytes differ from the embedded asset")
	}

	cmdData, err := fs.ReadFile(assets.FS, "opencode/commands/sdd-apply.md")
	if err != nil {
		t.Fatalf("read embedded command: %v", err)
	}
	gotCmd, err := os.ReadFile(filepath.Join(home, ".config", "opencode", "commands", "sdd-apply.md"))
	if err != nil {
		t.Fatalf("read deployed command: %v", err)
	}
	if !bytes.Equal(gotCmd, cmdData) {
		t.Error("deployed command bytes differ from the embedded asset")
	}
}

// TestReconcile_AgentNotInstalled asserts Reconcile surfaces detection
// failures instead of silently succeeding.
func TestReconcile_AgentNotInstalled(t *testing.T) {
	agent := fakeAgent()
	agent.Installed = false

	_, err := update.Reconcile(context.Background(), agent, t.TempDir())
	if err == nil {
		t.Fatal("Reconcile with an undetected agent should fail")
	}
}
