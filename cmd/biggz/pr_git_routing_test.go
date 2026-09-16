package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Fixture: scratch repository with a stable identity and one seed commit.
// ---------------------------------------------------------------------------

// gitTestCmd runs one git command in dir and fails the test on error.
func gitTestCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// newGitTestRepo creates a temp repo with one seed commit and returns its root.
func newGitTestRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not available: %v", err)
	}
	root := t.TempDir()
	gitTestCmd(t, root, "init", "-q", "-b", "main")
	gitTestCmd(t, root, "config", "user.email", "guard@test.local")
	gitTestCmd(t, root, "config", "user.name", "Guard Test")
	if err := os.WriteFile(filepath.Join(root, "seed.txt"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTestCmd(t, root, "add", "seed.txt")
	gitTestCmd(t, root, "commit", "-q", "-m", "chore: seed")
	return root
}

// ---------------------------------------------------------------------------
// TM-3 — commit state: write ops argv verbatim; write-tree=index,
// HEAD^{tree}=commit; unstaged mutation invisible to write-tree.
// ---------------------------------------------------------------------------

func TestPRWriteOpsCommitState(t *testing.T) {
	root := newGitTestRepo(t)
	t.Chdir(root)

	// checkout -b <branch> creates exactly the requested branch at HEAD.
	if err := gitCreateBranch("fix/tm3"); err != nil {
		t.Fatalf("gitCreateBranch: %v", err)
	}
	if got := gitTestCmd(t, root, "rev-parse", "--abbrev-ref", "HEAD"); got != "fix/tm3" {
		t.Fatalf("HEAD branch = %q, want fix/tm3", got)
	}
	if got, want := gitTestCmd(t, root, "rev-parse", "HEAD"), gitTestCmd(t, root, "rev-parse", "main"); got != want {
		t.Fatalf("checkout -b must start at HEAD: %s != main %s", got, want)
	}

	// Empty index: nothing staged, so write-tree equals HEAD^{tree}.
	if got := gitTestCmd(t, root, "diff", "--cached", "--name-only"); got != "" {
		t.Fatalf("index should be empty, got %q", got)
	}
	if got, want := gitTestCmd(t, root, "write-tree"), gitTestCmd(t, root, "rev-parse", "HEAD^{tree}"); got != want {
		t.Fatalf("clean write-tree = %s, want HEAD^{tree} = %s", got, want)
	}

	// add -A plus commit: the untracked addition and the deletion both land.
	if err := os.WriteFile(filepath.Join(root, "added.txt"), []byte("added\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "seed.txt")); err != nil {
		t.Fatal(err)
	}
	if err := gitStageAll(); err != nil {
		t.Fatalf("gitStageAll: %v", err)
	}
	if err := gitCommit("fix: tm3 commit"); err != nil {
		t.Fatalf("gitCommit: %v", err)
	}
	if got := gitTestCmd(t, root, "log", "-1", "--format=%s"); got != "fix: tm3 commit" {
		t.Fatalf("commit subject = %q, want the verbatim message", got)
	}
	if got, want := gitTestCmd(t, root, "write-tree"), gitTestCmd(t, root, "rev-parse", "HEAD^{tree}"); got != want {
		t.Fatalf("committed write-tree = %s, want HEAD^{tree} = %s", got, want)
	}
	if tree := gitTestCmd(t, root, "ls-tree", "--name-only", "HEAD"); tree != "added.txt" {
		t.Fatalf("add -A must stage the addition and the deletion, tree = %q", tree)
	}

	// Unstaged mutation: invisible to write-tree, visible to the workspace snapshot.
	if err := os.WriteFile(filepath.Join(root, "added.txt"), []byte("mutated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, want := gitTestCmd(t, root, "write-tree"), gitTestCmd(t, root, "rev-parse", "HEAD^{tree}"); got != want {
		t.Fatalf("unstaged mutation leaked into the index tree: %s != %s", got, want)
	}
	if files := getChangedFiles(root); !slices.Contains(files, "added.txt") {
		t.Fatalf("workspace snapshot must see the unstaged mutation, got %v", files)
	}
}

// ---------------------------------------------------------------------------
// TM-4 — push state: bare remote; first push sets upstream; repeat behaves;
// failure exposes git's raw stderr through the returned *exec.ExitError.
// ---------------------------------------------------------------------------

func TestGitPushUpstreamBareRemote(t *testing.T) {
	root := newGitTestRepo(t)
	bare := filepath.Join(t.TempDir(), "origin.git")
	gitTestCmd(t, filepath.Dir(bare), "init", "-q", "--bare", "-b", "main", bare)
	gitTestCmd(t, root, "remote", "add", "origin", filepath.ToSlash(bare))
	t.Chdir(root)

	if err := gitCreateBranch("fix/tm4"); err != nil {
		t.Fatalf("gitCreateBranch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "first.txt"), []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gitStageAll(); err != nil {
		t.Fatal(err)
	}
	if err := gitCommit("fix: tm4 first"); err != nil {
		t.Fatal(err)
	}

	// First push: push -u origin <branch> sets the upstream.
	if err := gitPushUpstream("fix/tm4"); err != nil {
		t.Fatalf("first push: %v", err)
	}
	if got := gitTestCmd(t, root, "rev-parse", "--abbrev-ref", "fix/tm4@{upstream}"); got != "origin/fix/tm4" {
		t.Fatalf("upstream = %q, want origin/fix/tm4", got)
	}

	// Repeat behaves: a no-op push succeeds and the remote ref stays in sync.
	if err := gitPushUpstream("fix/tm4"); err != nil {
		t.Fatalf("repeat push: %v", err)
	}
	if got, want := gitTestCmd(t, root, "rev-parse", "origin/fix/tm4"), gitTestCmd(t, root, "rev-parse", "fix/tm4"); got != want {
		t.Fatalf("remote ref %s out of sync with local %s", got, want)
	}

	// Failure: git's raw stderr reaches the caller, unmodified.
	hook := filepath.Join(bare, "hooks", "pre-receive")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho PUSH-REJECTED-BY-HOOK >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "second.txt"), []byte("second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gitStageAll(); err != nil {
		t.Fatal(err)
	}
	if err := gitCommit("fix: tm4 second"); err != nil {
		t.Fatal(err)
	}
	err := gitPushUpstream("fix/tm4")
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("push failure = %v (%T), want *exec.ExitError", err, err)
	}
	if raw := string(exitErr.Stderr); !strings.Contains(raw, "PUSH-REJECTED-BY-HOOK") {
		t.Fatalf("raw stderr lost, got %q", raw)
	}
}

// ---------------------------------------------------------------------------
// TM-5 — PR commands: getChangedFiles and gh argv goldens; gh untouched.
// ---------------------------------------------------------------------------

// getChangedFiles returns the workspace snapshot staged, then unstaged, then
// untracked, in git's own order.
func TestGetChangedFilesGolden(t *testing.T) {
	root := newGitTestRepo(t)
	t.Chdir(root)

	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTestCmd(t, root, "add", "tracked.txt") // staged
	if err := os.WriteFile(filepath.Join(root, "seed.txt"), []byte("mutated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := strings.Join(getChangedFiles(root), "\n")
	want := "tracked.txt\nseed.txt\nuntracked.txt"
	if got != want {
		t.Fatalf("getChangedFiles golden mismatch\ngot:  %q\nwant: %q", got, want)
	}
}

// ghPRCreateArgs pins the gh argv: gh is the documented non-git boundary
// (TM-5) and is never routed through internal/git; labels stay trimmed.
func TestGHPRCreateArgsGolden(t *testing.T) {
	cases := []struct{ name, labels, want string }{
		{"no labels", "", "pr\ncreate\n--title\nTitle T\n--body\nBody B"},
		{"labels trimmed", "type:refactor, size:exception", "pr\ncreate\n--title\nTitle T\n--body\nBody B\n--label\ntype:refactor\n--label\nsize:exception"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := strings.Join(ghPRCreateArgs("Title T", "Body B", tc.labels), "\n"); got != tc.want {
				t.Errorf("gh pr create argv golden mismatch\ngot:  %q\nwant: %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// cli_util — detectGitDirs stays byte-compatible with the single owner.
// ---------------------------------------------------------------------------

func TestDetectGitDirsByteCompatible(t *testing.T) {
	root := newGitTestRepo(t)
	t.Chdir(root)
	common, worktree := detectGitDirs()
	if want := gitTestCmd(t, root, "rev-parse", "--git-common-dir"); common != want {
		t.Fatalf("commonDir = %q, want %q (same order, same bytes)", common, want)
	}
	if want := gitTestCmd(t, root, "rev-parse", "--git-dir"); worktree != want {
		t.Fatalf("worktreeDir = %q, want %q (same order, same bytes)", worktree, want)
	}

	t.Chdir(t.TempDir())
	if common, worktree := detectGitDirs(); common != "" || worktree != "" {
		t.Fatalf("outside a repo detectGitDirs = (%q, %q), want the empty pair", common, worktree)
	}
}
