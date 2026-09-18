package doctor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// statWithFiles reports the given paths as existing regular files and
// everything else as missing.
func statWithFiles(files ...string) func(string) (os.FileInfo, error) {
	set := make(map[string]bool, len(files))
	for _, f := range files {
		set[filepath.Clean(f)] = true
	}
	return func(path string) (os.FileInfo, error) {
		if set[filepath.Clean(path)] {
			return fakeFileInfo{isDir: false}, nil
		}
		return nil, os.ErrNotExist
	}
}

func piFoundLookPath(name string) (string, error) {
	if name == "pi" {
		return filepath.Join("C:", "pi", "pi.cmd"), nil
	}
	return "", errors.New("not found")
}

func TestPiSubagentsCheck_RuntimeMarkerPasses(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	marker := filepath.Join(home, ".pi", "agent", "extensions", "biggz-subagent-runtime.js")
	c := NewPiSubagentsCheckWithCustom(piFoundLookPath,
		statWithFiles(marker),
		func() (string, error) { return home, nil })
	res := c.Run(context.Background())
	if res.Status != StatusPass {
		t.Fatalf("Status = %v, want pass (result: %s)", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "subagent runtime") {
		t.Errorf("Message %q should name the subagent runtime", res.Message)
	}
}

func TestPiSubagentsCheck_MissingMarkerWarnsWithInstallHint(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	c := NewPiSubagentsCheckWithCustom(piFoundLookPath,
		statWithFiles(),
		func() (string, error) { return home, nil })
	res := c.Run(context.Background())
	if res.Status != StatusWarn {
		t.Fatalf("Status = %v, want warn when the runtime marker is missing", res.Status)
	}
	if !strings.Contains(res.Message, "biggz install --agent pi") {
		t.Errorf("Message %q should point at the install remedy", res.Message)
	}
}

func TestPiSubagentsCheck_PiMissingSkips(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	c := NewPiSubagentsCheckWithCustom(
		func(string) (string, error) { return "", errors.New("no pi") },
		statWithFiles(),
		func() (string, error) { return t.TempDir(), nil })
	res := c.Run(context.Background())
	if res.Status != StatusPass {
		t.Fatalf("Status = %v, want pass skip when pi is absent", res.Status)
	}
}

func TestPiSubagentsCheck_HonorsPICodingAgentDirOverride(t *testing.T) {
	override := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", override)
	marker := filepath.Join(override, "extensions", "biggz-subagent-runtime.js")
	c := NewPiSubagentsCheckWithCustom(piFoundLookPath,
		statWithFiles(marker),
		func() (string, error) { return t.TempDir(), nil })
	res := c.Run(context.Background())
	if res.Status != StatusPass {
		t.Fatalf("Status = %v, want pass (PI_CODING_AGENT_DIR marker)", res.Status)
	}
}

// TestPiSubagentsRemedy_RedeploysRuntime ensures the remedy runs the standard
// pi install flow (biggz install --agent pi) — the runtime is the delegation
// owner after the j0k3r cutover, so the repair is a redeploy, not a fork install.
func TestPiSubagentsRemedy_RedeploysRuntime(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	c := NewPiSubagentsCheckWithCustom(piFoundLookPath,
		statWithFiles(),
		func() (string, error) { return t.TempDir(), nil })
	remedy := c.Remedy()
	if remedy == nil {
		t.Fatal("Remedy() returned nil")
	}
	if remedy.Action == nil {
		t.Fatal("Remedy() action nil")
	}
	if !strings.Contains(remedy.Description, "biggz install --agent pi") {
		t.Errorf("Remedy description %q must show the redeploy command", remedy.Description)
	}
}
