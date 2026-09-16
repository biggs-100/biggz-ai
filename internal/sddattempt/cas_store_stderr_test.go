package sddattempt

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestResolveCloneStoreNotGitRepoStaysClassified pins the stderr-wording
// classification isNotGitRepoError relies on: the migrated spawn must still
// surface git's raw stderr on the exit error, because only that failure class
// may fall back to the machine-scoped ledger.
func TestResolveCloneStoreNotGitRepoStaysClassified(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not available: %v", err)
	}
	dir := t.TempDir() // deliberately outside any git repository
	_, err := resolveCloneStore("ch-classify", dir, filepath.Join(dir, "legacy.json"))
	if err == nil {
		t.Fatal("resolveCloneStore must fail outside a git repository")
	}
	if !isNotGitRepoError(err) {
		t.Fatalf("not-a-git-repository failure must stay classified by git's raw stderr, got: %v", err)
	}
}
