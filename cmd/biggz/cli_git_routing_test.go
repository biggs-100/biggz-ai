package main

import (
	"os"
	"path/filepath"
	"testing"
)

// canonicalDir returns the symlink-resolved absolute form of dir, matching the
// contract of the git wrapper (git.TopLevel) this package now delegates to.
func canonicalDir(t *testing.T, dir string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", dir, err)
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		t.Fatalf("Abs(%q): %v", resolved, err)
	}
	return abs
}

// TestDetectProjectRootKeepsCwdFallback pins the error semantics preserved from
// the pre-migration shape: when git cannot answer, detectProjectRoot returns the
// process working directory instead of failing.
func TestDetectProjectRootKeepsCwdFallback(t *testing.T) {
	t.Run("outside a repository", func(t *testing.T) {
		t.Chdir(t.TempDir())
		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd: %v", err)
		}
		if got := detectProjectRoot(); got != wd {
			t.Fatalf("detectProjectRoot() = %q, want cwd %q", got, wd)
		}
	})

	t.Run("git unavailable", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd: %v", err)
		}
		if got := detectProjectRoot(); got != wd {
			t.Fatalf("detectProjectRoot() = %q, want cwd %q", got, wd)
		}
	})
}

// TestDetectProjectRootSelectsGitTopLevel proves the migrated site still
// prefers the repository root over the caller's cwd.
func TestDetectProjectRootSelectsGitTopLevel(t *testing.T) {
	root := newGitTestRepo(t)
	sub := filepath.Join(root, "nested", "deeper")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	t.Chdir(sub)

	if got, want := detectProjectRoot(), canonicalDir(t, root); got != want {
		t.Fatalf("detectProjectRoot() = %q, want git top level %q", got, want)
	}
}

// TestCodegraphGitTopLevelSelectsExplicitPath proves the explicit path wins over
// the caller's cwd and that the error still propagates outside a repository.
func TestCodegraphGitTopLevelSelectsExplicitPath(t *testing.T) {
	root := newGitTestRepo(t)
	want := canonicalDir(t, root)

	got, err := codegraphGitTopLevel(root)
	if err != nil {
		t.Fatalf("codegraphGitTopLevel(%q): %v", root, err)
	}
	if got != want {
		t.Fatalf("codegraphGitTopLevel(%q) = %q, want %q", root, got, want)
	}

	sub := filepath.Join(root, "nested")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	got, err = codegraphGitTopLevel(sub)
	if err != nil {
		t.Fatalf("codegraphGitTopLevel(%q): %v", sub, err)
	}
	if got != want {
		t.Fatalf("codegraphGitTopLevel(%q) = %q, want %q", sub, got, want)
	}

	if _, err := codegraphGitTopLevel(t.TempDir()); err == nil {
		t.Fatal("codegraphGitTopLevel outside a repository: want error, got nil")
	}
}

// TestExportChangelogFailsWithoutGit pins the preserved failure semantics of the
// migrated `git log` argv: unreachable git exits 1 instead of panicking or
// succeeding with an empty changelog.
func TestExportChangelogFailsWithoutGit(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if code := exportChangelog("", "txt"); code != 1 {
		t.Fatalf("exportChangelog without git = %d, want 1", code)
	}
}
