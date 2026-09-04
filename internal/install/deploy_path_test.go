package install_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/install"
)

// TestBinaryOnPath_Found ensures BinaryOnPath is true when a biggz binary
// resolves via PATH, so install skips adding ~/.biggz (no duplicate warning).
func TestBinaryOnPath_Found(t *testing.T) {
	dir := t.TempDir()
	// Write both spellings so the test works on Windows (PATHEXT .exe)
	// and Unix (exact name + exec bit).
	for _, name := range []string{"biggz", "biggz.exe"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("fake"), 0o755); err != nil {
			t.Fatalf("write fake binary: %v", err)
		}
	}
	t.Setenv("PATH", dir)
	if !install.BinaryOnPath() {
		t.Error("BinaryOnPath() = false, want true when biggz resolves via PATH")
	}
}

// TestBinaryOnPath_Missing ensures BinaryOnPath is false when nothing named
// biggz is on PATH, so install keeps the legacy behavior of adding ~/.biggz.
func TestBinaryOnPath_Missing(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // empty dir: nothing resolves
	if install.BinaryOnPath() {
		t.Error("BinaryOnPath() = true, want false when biggz is not on PATH")
	}
}

// TestBinaryOnPath_IgnoresWorkingDirectory is a regression test: a ./biggz.exe
// in the working directory (e.g. installing from a repo checkout) must not
// hide a biggz binary that resolves via PATH. exec.LookPath reports ErrDot in
// that situation (go.dev/issue/53536), so BinaryOnPath scans PATH directly.
func TestBinaryOnPath_IgnoresWorkingDirectory(t *testing.T) {
	decoy := t.TempDir() // cwd with a shadowing ./biggz.exe
	for _, name := range []string{"biggz", "biggz.exe"} {
		if err := os.WriteFile(filepath.Join(decoy, name), []byte("fake"), 0o755); err != nil {
			t.Fatalf("write decoy binary: %v", err)
		}
	}
	bindir := t.TempDir() // PATH dir with the real binary
	for _, name := range []string{"biggz", "biggz.exe"} {
		if err := os.WriteFile(filepath.Join(bindir, name), []byte("fake"), 0o755); err != nil {
			t.Fatalf("write PATH binary: %v", err)
		}
	}
	t.Setenv("PATH", bindir)
	t.Chdir(decoy)
	if !install.BinaryOnPath() {
		t.Error("BinaryOnPath() = false, want true: PATH binary must win over ./biggz.exe decoy")
	}
}
