package steps

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/biggs-100/biggz-ai/internal/agents"
	"github.com/biggs-100/biggz-ai/internal/pipeline"
	"github.com/biggs-100/biggz-ai/plugintest"
)

// TestPiExtensionsStep_DeploysNestedHelper proves the hybrid deploy shape end to
// end: a flat factory and a nested helper land in the right places under the pi
// agent's extensions/ directory, the helper's subdirectory is created, and the
// returned count includes both.
//
// The invariant is the D1 decision: helpers cannot sit flat because pi loads
// every direct extensions/*.js|*.ts as an extension and crashes on one that does
// not export a factory. deployExtensionFiles is driven with a synthetic list so
// the shape is provable before any helper module is actually ported.
func TestPiExtensionsStep_DeploysNestedHelper(t *testing.T) {
	tmp := t.TempDir()
	agent := &plugintest.FakeAgent{Installed: true, AgentID: agents.AgentPi}
	agent.SetTempDir(tmp)
	p := NewPiExtensionsStep(tmp, agent, false)

	factoryContent := "export default function biggzFooter(pi){ pi.on('session_start', async()=>{}); }\n"
	helperContent := "export const SHELL_WIDTH = 80;\n"
	fsys := fstest.MapFS{
		"pi/biggz-footer.js":           &fstest.MapFile{Data: []byte(factoryContent)},
		"pi/biggz-shell/shell-card.js": &fstest.MapFile{Data: []byte(helperContent)},
	}
	list := []piExtension{
		{asset: "pi/biggz-footer.js", target: "biggz-footer.js", factory: true},
		{asset: "pi/biggz-shell/shell-card.js", target: piShellSubdir + "/shell-card.js", factory: false},
	}

	n, err := p.deployExtensionFiles(context.Background(), fsys, list, make(pipeline.ProgressChan, 32))
	if err != nil {
		t.Fatalf("deployExtensionFiles: %v", err)
	}
	if n != 2 {
		t.Fatalf("deployExtensionFiles returned %d, want 2 (flat factory + nested helper)", n)
	}
	if p.Deployed != 2 {
		t.Fatalf("p.Deployed = %d, want 2", p.Deployed)
	}

	extDir := piExtensionsDir(tmp)
	// The factory lands flat in extensions/.
	if _, err := os.Stat(filepath.Join(extDir, "biggz-footer.js")); err != nil {
		t.Errorf("flat factory biggz-footer.js missing: %v", err)
	}
	// The helper lands in extensions/biggz-shell/, not flat.
	if _, err := os.Stat(filepath.Join(extDir, "shell-card.js")); !os.IsNotExist(err) {
		t.Errorf("nested helper must not be written flat at %s (err=%v)", filepath.Join(extDir, "shell-card.js"), err)
	}
	helperPath := filepath.Join(extDir, piShellSubdir, "shell-card.js")
	data, err := os.ReadFile(helperPath)
	if err != nil {
		t.Fatalf("nested helper %s missing: %v", helperPath, err)
	}
	if string(data) != helperContent {
		t.Errorf("nested helper content = %q, want %q", string(data), helperContent)
	}
	// The helper's subdirectory was created by the deploy.
	info, err := os.Stat(filepath.Join(extDir, piShellSubdir))
	if err != nil {
		t.Fatalf("helper subdirectory %s was not created: %v", filepath.Join(extDir, piShellSubdir), err)
	}
	if !info.IsDir() {
		t.Fatalf("%s exists but is not a directory", filepath.Join(extDir, piShellSubdir))
	}
}
