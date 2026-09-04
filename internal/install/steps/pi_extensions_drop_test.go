package steps

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/agents"
	"github.com/biggs-100/biggz-ai/internal/pipeline"
	"github.com/biggs-100/biggz-ai/plugintest"
)

// TestPiExtensionsStep_DropsUnportableExtensions ensures the step neither
// deploys nor leaves behind extensions that import from ../lib/* (a tree
// never ported into internal/assets/pi), which crash pi on startup with
// "Cannot find module '../lib/sdd-preflight.ts'".
func TestPiExtensionsStep_DropsUnportableExtensions(t *testing.T) {
	tmp := t.TempDir()
	agent := &plugintest.FakeAgent{Installed: true, AgentID: agents.AgentPi}
	agent.SetTempDir(tmp)
	// Pre-seed stale copies as left behind by older installs.
	extDir := piExtensionsDir(tmp)
	if err := os.MkdirAll(extDir, 0o755); err != nil {
		t.Fatalf("mkdir extDir: %v", err)
	}
	stale := []string{"gentle-ai.ts", "quiet-tools.ts", "sdd-init.ts", "startup-banner.ts"}
	for _, name := range stale {
		if err := os.WriteFile(filepath.Join(extDir, name), []byte("stale"), 0o644); err != nil {
			t.Fatalf("seed stale %s: %v", name, err)
		}
	}
	p := NewPiExtensionsStep(tmp, agent, false)
	if err := p.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	ch := make(pipeline.ProgressChan, 32)
	if err := p.Apply(context.Background(), ch); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for _, name := range stale {
		if _, err := os.Stat(filepath.Join(extDir, name)); !os.IsNotExist(err) {
			t.Errorf("stale extension %s still present after Apply (err=%v)", name, err)
		}
	}
	// Portable extensions must still be deployed.
	for _, name := range []string{
		"ask-user-choice.ts", "codegraph-tools.ts",
		"skill-registry.ts",
		"biggz-thinking-wrap.js", "biggz-web-search.js",
	} {
		if _, err := os.Stat(filepath.Join(extDir, name)); err != nil {
			t.Errorf("expected deployed extension %s missing: %v", name, err)
		}
	}
}
