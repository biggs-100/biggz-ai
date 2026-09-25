package install

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/assets"
)

// TestDefaultPiThemeIsDeployed keeps DefaultPiTheme and the deployed theme
// assets in sync. A fresh install writes this name into settings.json, so a
// rename on one side without the other leaves pi pointing at a theme it cannot
// resolve.
func TestDefaultPiThemeIsDeployed(t *testing.T) {
	entries, err := fs.ReadDir(assets.FS, "pi/themes")
	if err != nil {
		t.Fatalf("read pi/themes: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := fs.ReadFile(assets.FS, "pi/themes/"+e.Name())
		if err != nil {
			t.Fatalf("read theme %s: %v", e.Name(), err)
		}
		var th struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(data, &th); err != nil {
			t.Fatalf("theme %s is not valid JSON: %v", e.Name(), err)
		}
		if th.Name == DefaultPiTheme {
			return
		}
	}
	t.Fatalf("DefaultPiTheme %q has no asset under internal/assets/pi/themes", DefaultPiTheme)
}

// TestEnsurePiThemeWritesDefaultAndPreservesOtherSettings pins both halves of
// ensurePiTheme: it sets the theme, and it leaves unrelated settings intact.
// PI_CODING_AGENT_DIR is redirected to a temp dir so the test never rewrites
// the developer's real pi settings.
func TestEnsurePiThemeWritesDefaultAndPreservesOtherSettings(t *testing.T) {
	home := t.TempDir()
	agentDir := filepath.Join(home, "pi-agent")
	t.Setenv("PI_CODING_AGENT_DIR", agentDir)

	settingsPath := filepath.Join(agentDir, "settings.json")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"theme":"dark","defaultModel":"opencode-go"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ensurePiTheme(home, DefaultPiTheme); err != nil {
		t.Fatalf("ensurePiTheme: %v", err)
	}

	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("settings.json is not valid JSON after the merge: %v", err)
	}
	if got["theme"] != DefaultPiTheme {
		t.Errorf("theme = %v, want %q", got["theme"], DefaultPiTheme)
	}
	if got["defaultModel"] != "opencode-go" {
		t.Errorf("defaultModel = %v, want it preserved as opencode-go", got["defaultModel"])
	}
}
