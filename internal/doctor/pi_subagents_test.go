package doctor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// statWithDirs returns a statFn that reports the given paths as existing
// directories and everything else as missing.
func statWithDirs(dirs ...string) func(string) (os.FileInfo, error) {
	set := make(map[string]bool, len(dirs))
	for _, d := range dirs {
		set[filepath.Clean(d)] = true
	}
	return func(path string) (os.FileInfo, error) {
		if set[filepath.Clean(path)] {
			return fakeFileInfo{isDir: true}, nil
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

func TestPiSubagentsCheck_ForkPasses(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	forkDir := filepath.Join(home, ".pi", "agent", "npm", "node_modules", "pi-subagents-j0k3r")
	c := NewPiSubagentsCheckWithCustom(piFoundLookPath,
		func(string, ...string) ([]byte, error) { return nil, errors.New("no npm") },
		statWithDirs(forkDir),
		func() (string, error) { return home, nil })
	res := c.Run(context.Background())
	if res.Status != StatusPass {
		t.Fatalf("Status = %v, want pass (result: %s)", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "pi-subagents-j0k3r") {
		t.Errorf("Message %q should name the fork", res.Message)
	}
}

func TestPiSubagentsCheck_LegacyWarnsMigrate(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	legacyDir := filepath.Join(home, ".pi", "agent", "npm", "node_modules", "pi-subagents")
	c := NewPiSubagentsCheckWithCustom(piFoundLookPath,
		func(string, ...string) ([]byte, error) { return nil, errors.New("no npm") },
		statWithDirs(legacyDir),
		func() (string, error) { return home, nil })
	res := c.Run(context.Background())
	if res.Status != StatusWarn {
		t.Fatalf("Status = %v, want warn for legacy-only install", res.Status)
	}
	if !strings.Contains(res.Message, "migrate") {
		t.Errorf("Message %q should hint migration", res.Message)
	}
}

func TestPiSubagentsCheck_MissingWarns(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	c := NewPiSubagentsCheckWithCustom(piFoundLookPath,
		func(string, ...string) ([]byte, error) { return nil, errors.New("no npm") },
		statWithDirs(),
		func() (string, error) { return home, nil })
	res := c.Run(context.Background())
	if res.Status != StatusWarn {
		t.Fatalf("Status = %v, want warn when dispatcher missing", res.Status)
	}
	if !strings.Contains(res.Message, "npm:pi-subagents-j0k3r") {
		t.Errorf("Message %q should point at the fork install", res.Message)
	}
}
