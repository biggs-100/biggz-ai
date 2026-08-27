package install_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/assets"
	"github.com/biggs-100/biggz-ai/internal/install"
)

func TestDeployPiWebSearch(t *testing.T) {
	home := t.TempDir()
	res, err := install.DeployPiWebSearch(context.Background(), home)
	if err != nil {
		t.Fatalf("DeployPiWebSearch: %v", err)
	}
	if !res.Created && !res.Changed {
		// first deploy should create
		if _, err := os.Stat(filepath.Join(home, ".pi", "agent", "extensions", "biggz-web-search.js")); err != nil {
			t.Fatalf("extension not created: %v", err)
		}
	}
	target := filepath.Join(home, ".pi", "agent", "extensions", "biggz-web-search.js")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	want, _ := fs.ReadFile(assets.FS, "pi/biggz-web-search.js")
	if string(data) != string(want) {
		t.Fatalf("content mismatch: got %d bytes, want %d bytes", len(data), len(want))
	}
	if len(data) == 0 || !strings.Contains(string(data), "isPrivateIP") {
		t.Fatalf("extension missing SSRF guard")
	}
}

func TestDeployPiWebSearch_Idempotent(t *testing.T) {
	home := t.TempDir()
	if _, err := install.DeployPiWebSearch(context.Background(), home); err != nil {
		t.Fatalf("first deploy: %v", err)
	}
	target := filepath.Join(home, ".pi", "agent", "extensions", "biggz-web-search.js")
	first, _ := os.ReadFile(target)
	res, err := install.DeployPiWebSearch(context.Background(), home)
	if err != nil {
		t.Fatalf("second deploy: %v", err)
	}
	if res.Changed || res.Created {
		t.Fatalf("second deploy should be idempotent, got Created=%v Changed=%v", res.Created, res.Changed)
	}
	second, _ := os.ReadFile(target)
	if string(first) != string(second) {
		t.Fatalf("idempotent deploy changed content")
	}
}

func TestDeployPiWebSearch_TempDir(t *testing.T) {
	home := t.TempDir()
	legacyHome := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(legacyHome, "override"))
	// Deploy with home=TempDir should not touch override when PI_CODING_AGENT_DIR not matching home
	// But piExtensionsDir respects PI_CODING_AGENT_DIR env, so we test isolated home without env
	t.Setenv("PI_CODING_AGENT_DIR", "")
	if _, err := install.DeployPiWebSearch(context.Background(), home); err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".pi", "agent", "extensions", "biggz-web-search.js")); err != nil {
		t.Fatalf("not in TempDir: %v", err)
	}
	// Ensure no file outside TempDir was modified (check that legacyHome is still empty)
	if entries, _ := os.ReadDir(legacyHome); len(entries) != 0 {
		t.Fatalf("unexpected file outside TempDir: %v", entries)
	}
}

func TestDeployPiWebSearch_LegacyCleanup(t *testing.T) {
	home := t.TempDir()
	extDir := filepath.Join(home, ".pi", "agent", "extensions")
	_ = os.MkdirAll(extDir, 0755)
	_ = os.WriteFile(filepath.Join(extDir, "biggz-web-search-legacy.js"), []byte("old"), 0644)
	if _, err := install.DeployPiWebSearch(context.Background(), home); err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if _, err := os.Stat(filepath.Join(extDir, "biggz-web-search-legacy.js")); err == nil {
		t.Fatalf("legacy file not removed")
	}
}
