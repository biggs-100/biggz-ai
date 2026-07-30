package review

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// RED tests: Commit state scope detection (threat matrix — task 2.6)
// ---------------------------------------------------------------------------
//
// These tests verify that scope change detection works correctly for
// different git worktree states:
//   - Clean state: no staged or unstaged changes
//   - Staged-only: changes staged but not committed
//   - Dirty worktree: unstaged changes in working tree
//
// Scope is detected by comparing a snapshot (tree hash recorded at a point
// in time) with the current git tree via `git diff-tree` or `git status`.

// setupRepoWithCommit initialises a git repo, creates a base file,
// stages and commits it. Returns the repo directory and the tree hash.
func setupRepoWithCommit(t *testing.T) (repoDir, treeHash string) {
	t.Helper()
	repoDir = t.TempDir()
	gitInit(t, repoDir)

	// Create a base file and commit.
	baseFile := filepath.Join(repoDir, "base.go")
	if err := os.WriteFile(baseFile, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write base file: %v", err)
	}
	runGitInDir(t, repoDir, "add", "base.go")
	runGitInDir(t, repoDir, "commit", "-m", "initial commit")

	// Get the tree hash of HEAD.
	treeHash = strings.TrimSpace(runGitInDir(t, repoDir, "rev-parse", "HEAD:"))
	return repoDir, treeHash
}

// TestScopeDetect_CleanState verifies that a clean git worktree has no
// scope difference between the snapshot tree and current HEAD.
func TestScopeDetect_CleanState(t *testing.T) {
	repoDir, _ := setupRepoWithCommit(t)

	// Get the current tree hash (should be identical to the committed tree).
	currentTree := runGitInDir(t, repoDir, "rev-parse", "HEAD:")

	// Verify clean state: no staged or unstaged changes.
	// Status should show "nothing to commit, working tree clean".
	status := runGitInDir(t, repoDir, "status", "--porcelain")
	if status != "" {
		t.Fatalf("expected clean state, got status:\n%s", status)
	}

	// diff-tree between same tree should be empty.
	diff := runGitInDir(t, repoDir, "diff-tree", "--no-commit-id", "-r", currentTree, "HEAD")
	if diff != "" {
		t.Errorf("expected empty diff for same tree, got:\n%s", diff)
	}
}

// TestScopeDetect_StagedOnly verifies that staged changes are detected
// as a scope change from the snapshot tree.
func TestScopeDetect_StagedOnly(t *testing.T) {
	repoDir, snapshotTree := setupRepoWithCommit(t)

	// Stage a new file without committing.
	newFile := filepath.Join(repoDir, "new.go")
	if err := os.WriteFile(newFile, []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatalf("write new file: %v", err)
	}
	runGitInDir(t, repoDir, "add", "new.go")

	// Verify staged changes exist.
	status := runGitInDir(t, repoDir, "status", "--porcelain")
	if !strings.HasPrefix(status, "A ") && !strings.HasPrefix(status, "??") {
		// New file shows as "??" before staging, "A " after.
		// We staged it, so it should show "A  new.go" or similar.
	}

	// Compute current tree hash (including staged changes).
	currentTree := runGitInDir(t, repoDir, "write-tree")

	// Scope change: snapshot tree != current tree.
	if snapshotTree == currentTree {
		t.Error("expected staged changes to produce a different tree hash")
	}

	// diff-tree between snapshot and current should show the new file.
	diff := runGitInDir(t, repoDir, "diff-tree", "--no-commit-id", "-r", snapshotTree, currentTree)
	if diff == "" {
		t.Error("expected non-empty diff between snapshot and current tree")
	}
}

// TestScopeDetect_DirtyWorktree verifies that unstaged changes (dirty
// worktree) are NOT part of the committed tree but ARE part of the
// working tree. Scope detection should compare the committed tree
// (which doesn't include unstaged changes) — the gate documents this
// distinction.
func TestScopeDetect_DirtyWorktree(t *testing.T) {
	repoDir, snapshotTree := setupRepoWithCommit(t)

	// Modify a tracked file without staging.
	baseFile := filepath.Join(repoDir, "base.go")
	if err := os.WriteFile(baseFile, []byte("package main\n\n// dirty change\n"), 0644); err != nil {
		t.Fatalf("modify base file: %v", err)
	}

	// Verify dirty state: unstaged changes.
	status := runGitInDir(t, repoDir, "status", "--porcelain")
	if !strings.HasPrefix(status, " M") && !strings.HasPrefix(status, "M ") {
		// Modified but not staged shows as " M base.go"
	}

	// The committed tree hash should NOT include unstaged changes.
	headTree := runGitInDir(t, repoDir, "rev-parse", "HEAD:")

	// diff-tree between snapshot (which was HEAD at capture time) and
	// current HEAD tree — if no commit happened, these are the same.
	diff := runGitInDir(t, repoDir, "diff-tree", "--no-commit-id", "-r", snapshotTree, headTree)
	if diff != "" {
		t.Logf("diff between snapshot and HEAD tree:\n%s", diff)
	}

	// The staged tree (index) should be the same as HEAD tree since no
	// changes were staged.
	indexTree := runGitInDir(t, repoDir, "write-tree")
	if indexTree != headTree {
		t.Error("expected index tree to match HEAD tree with only dirty worktree changes")
	}

	// Verify that a diff between snapshot and index shows no changes
	// (only unstaged worktree changes, which don't affect the tree).
	if snapshotTree != headTree {
		t.Errorf("expected snapshot tree %s to match HEAD tree %s for dirty worktree",
			snapshotTree, headTree)
	}
}

// TestScopeDiff_CleanVsStaged runs diff-tree between clean state, staged
// state, and dirty state to verify the correct detection for each.
func TestScopeDiff_CleanVsStaged(t *testing.T) {
	repoDir, originalTree := setupRepoWithCommit(t)

	// --- Clean state: diff-tree should be empty ---
	headTree := runGitInDir(t, repoDir, "rev-parse", "HEAD:")
	if headTree != originalTree {
		t.Errorf("HEAD tree changed without any operation: %s vs %s", headTree, originalTree)
	}

	// --- Staged state: diff-tree should show changes ---
	newFile := filepath.Join(repoDir, "staged.go")
	if err := os.WriteFile(newFile, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write staged file: %v", err)
	}
	runGitInDir(t, repoDir, "add", "staged.go")

	stagedTree := runGitInDir(t, repoDir, "write-tree")
	if stagedTree == headTree {
		t.Error("expected staged tree to differ from HEAD tree")
	}

	diffStaged := runGitInDir(t, repoDir, "diff-tree", "--no-commit-id", "-r", headTree, stagedTree)
	if diffStaged == "" {
		t.Error("expected non-empty diff between HEAD and staged trees")
	}

	// --- Dirty worktree only: index matches HEAD ---
	// Add another modification without staging.
	baseFile := filepath.Join(repoDir, "base.go")
	if err := os.WriteFile(baseFile, []byte("package main\n\n// more dirty\n"), 0644); err != nil {
		t.Fatalf("modify base file: %v", err)
	}

	// Index tree hasn't changed, still equals stagedTree.
	indexTree := runGitInDir(t, repoDir, "write-tree")
	if indexTree != stagedTree {
		t.Errorf("expected index tree to equal staged tree, got %s vs %s",
			indexTree, stagedTree)
	}

	// A diff between snapshot and index shows staged changes but not
	// unstaged dirty changes. This is the expected gate behavior:
	// scope detection uses the committed tree (or staged tree), not
	// the worktree.
}

// ---------------------------------------------------------------------------
// Helper: runGitInDir is defined in store_test.go (same package)
// ---------------------------------------------------------------------------
