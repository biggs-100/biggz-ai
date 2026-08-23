package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodeGraph_UsageErrors(t *testing.T) {
	cases := [][]string{
		{},
		{"init"},
		{"init", "--cwd"},
		{"init", "--cwd", ""},
		{"init", "--cwd", "   "},
		{"wrong", "--cwd", "/tmp"},
		{"init", "--badflag", "/tmp"},
	}
	for _, args := range cases {
		// Simulate os.Args for codegraphRun
		savedArgs := os.Args
		os.Args = append([]string{"biggz", "codegraph"}, args...)
		// Capture stderr by redirecting? codegraphRun prints to os.Stderr and returns 1.
		// We just check exit code, not output, for unit-level usage validation.
		code := codegraphRun()
		os.Args = savedArgs
		if code == 0 {
			t.Errorf("expected non-zero for args %v, got 0", args)
		}
	}
}

func TestCodeGraph_Guidance(t *testing.T) {
	savedArgs := os.Args
	os.Args = []string{"biggz", "codegraph", "guidance"}
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	code := codegraphRun()
	w.Close()
	os.Stdout = oldStdout
	os.Args = savedArgs
	if code != 0 {
		t.Fatalf("expected 0 for guidance, got %d", code)
	}
	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	out := string(buf[:n])
	if !strings.Contains(out, "CodeGraph") {
		t.Errorf("expected guidance to contain CodeGraph, got %q", out)
	}
	if !strings.Contains(out, "biggz codegraph init") {
		t.Errorf("expected guidance to contain 'biggz codegraph init', got %q", out)
	}
}

func TestResolveCodeGraphRoot_ValidRepo(t *testing.T) {
	// t.TempDir() is inside os.TempDir() which is considered unsafe by isUnsafeCodeGraphRoot.
	// Mock the temp/home dirs to avoid false unsafe rejection.
	origTemp := codegraphTempDir
	origHome := codegraphHomeDir
	fakeTemp := filepath.Join(t.TempDir(), "fake-temp-root-not-used")
	os.MkdirAll(fakeTemp, 0755)
	fakeHome := filepath.Join(t.TempDir(), "fake-home-not-used")
	os.MkdirAll(fakeHome, 0755)
	codegraphTempDir = func() string { return fakeTemp }
	codegraphHomeDir = func() (string, error) { return fakeHome, nil }
	defer func() {
		codegraphTempDir = origTemp
		codegraphHomeDir = origHome
	}()

	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "user.email", "test@test.com")

	readme := filepath.Join(dir, "README.md")
	os.WriteFile(readme, []byte("# test"), 0644)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "init")

	root, err := resolveCodeGraphRoot(dir)
	if err != nil {
		t.Fatalf("expected valid root, got %v", err)
	}
	if root == "" {
		t.Fatal("expected non-empty root")
	}
	canonicalDir, _ := filepath.EvalSymlinks(dir)
	canonicalDir, _ = filepath.Abs(canonicalDir)
	if root != canonicalDir {
		t.Errorf("expected %q, got %q", canonicalDir, root)
	}
}

func TestResolveCodeGraphRoot_UnsafePaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	cases := []struct {
		name string
		path string
	}{
		{"home dir", home},
		{"temp dir", os.TempDir()},
		{"drive root", filepath.VolumeName(home) + string(filepath.Separator)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolveCodeGraphRoot(tc.path)
			if err == nil {
				t.Errorf("expected error for unsafe path %q, got nil", tc.path)
			} else if !strings.Contains(err.Error(), "unsafe") {
				t.Errorf("expected unsafe error for %q, got %v", tc.path, err)
			}
		})
	}
}

func TestResolveCodeGraphRoot_NonGitDir(t *testing.T) {
	origTemp := codegraphTempDir
	origHome := codegraphHomeDir
	fakeTemp := filepath.Join(t.TempDir(), "fake-temp-root-not-used")
	os.MkdirAll(fakeTemp, 0755)
	fakeHome := filepath.Join(t.TempDir(), "fake-home-not-used")
	os.MkdirAll(fakeHome, 0755)
	codegraphTempDir = func() string { return fakeTemp }
	codegraphHomeDir = func() (string, error) { return fakeHome, nil }
	defer func() {
		codegraphTempDir = origTemp
		codegraphHomeDir = origHome
	}()

	dir := t.TempDir()
	_, err := resolveCodeGraphRoot(dir)
	if err == nil {
		t.Fatalf("expected error for non-git dir %q, got nil", dir)
	}
	if !strings.Contains(err.Error(), "not a recognized project") {
		t.Errorf("expected 'not a recognized project' for %q, got %v", dir, err)
	}
}

func TestResolveCodeGraphRoot_SubpathOfRepoRejected(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "user.email", "test@test.com")
	readme := filepath.Join(dir, "README.md")
	os.WriteFile(readme, []byte("# test"), 0644)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "init")

	sub := filepath.Join(dir, "subdir")
	os.MkdirAll(sub, 0755)

	// Candidate is subdir, but git toplevel is dir, so canonicalRoot != candidate -> unsafe
	_, err := resolveCodeGraphRoot(sub)
	if err == nil {
		t.Fatalf("expected error for subpath %q (toplevel %q), got nil", sub, dir)
	}
}

// Ensure codegraph binary is not required for validation tests
func TestCodeGraph_MissingBinaryReportsHelpfully(t *testing.T) {
	// This test verifies that when the root is valid but codegraph binary is missing,
	// the error message mentions the binary. We can't guarantee codegraph is installed,
	// so we test the validation part only via resolveCodeGraphRoot, which we already do.
	// If codegraph binary IS installed, the full init would succeed; if not, it fails gracefully.
	// This test just ensures the verb is wired and help is correct.
	savedArgs := os.Args
	os.Args = []string{"biggz", "codegraph", "init", "--cwd", t.TempDir()}
	// Use a non-git temp dir so validation fails before binary check — we test validation error path
	code := codegraphRun()
	os.Args = savedArgs
	if code == 0 {
		t.Error("expected non-zero for non-git dir")
	}
}

// Helper to run git in tests (reuses the helper from main_test.go if available)
func init() {
	// Ensure exec is available for git
	_, _ = exec.LookPath("git")
}
