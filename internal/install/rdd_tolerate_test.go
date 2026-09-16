package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/git"
)

// TestEnsureRDDEnabledToleratesMissingRepository pins the tolerance the two
// inline `git rev-parse` calls had: when the repository cannot be resolved
// (no git repository, no git binary), both git dirs stay empty and the
// install continues to the global-scope enable instead of aborting.
func TestEnsureRDDEnabledToleratesMissingRepository(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, _, err := git.ResolveGitDirs(t.Context(), ""); err == nil {
		t.Skip("temporary directory resolved inside a git repository; cannot exercise the tolerance path")
	}

	home := t.TempDir()
	ensureRDDEnabled(home)

	// The global mode file is written while HOME points at the temp home; its
	// existence proves the install continued past the tolerated git failure.
	global := filepath.Join(home, ".biggz", "rdd-mode.json")
	if _, err := os.Stat(global); err != nil {
		t.Fatalf("global RDD mode must still be written after the tolerated git failure: %v", err)
	}
}
