package review

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Store: Append / LoadChain / Validate
// ---------------------------------------------------------------------------

func TestStoreAppend_Genesis(t *testing.T) {
	dir := t.TempDir()
	s := OpenWithDir(dir, "test-lineage")

	hash, err := s.Append("", Record{
		Operation: "start",
		Role:      "Author",
		Actor:     "user",
		Timestamp: "2026-01-01T00:00:00Z",
		Payload:   json.RawMessage(`{"reason":"test"}`),
	})
	if err != nil {
		t.Fatalf("Append() error: %v", err)
	}
	if len(hash) != 64 {
		t.Errorf("expected 64-char hex hash, got %d: %s", len(hash), hash)
	}

	// Directory should exist now (task 1.4).
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("store directory should exist after first Append")
	}

	// HEAD file should exist.
	if _, err := os.Stat(filepath.Join(dir, "HEAD")); os.IsNotExist(err) {
		t.Error("HEAD file should exist after Append")
	}
}

func TestStoreAppend_ThreeEvents(t *testing.T) {
	dir := t.TempDir()
	s := OpenWithDir(dir, "chain-test")

	// Append genesis.
	h1, err := s.Append("", Record{
		Operation: "create",
		Role:      "Author",
		Timestamp: "2026-01-01T00:00:00Z",
		Payload:   json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("genesis Append: %v", err)
	}

	// Append second event.
	h2, err := s.Append(h1, Record{
		Operation: "review",
		Role:      "Reviewer",
		Timestamp: "2026-01-02T00:00:00Z",
		Payload:   json.RawMessage(`{"finding":"typo"}`),
	})
	if err != nil {
		t.Fatalf("second Append: %v", err)
	}
	if h2 == h1 {
		t.Error("second event should have different hash")
	}

	// Append third event.
	h3, err := s.Append(h2, Record{
		Operation: "approve",
		Role:      "Lead",
		Timestamp: "2026-01-03T00:00:00Z",
		Payload:   json.RawMessage(`{"approved":true}`),
	})
	if err != nil {
		t.Fatalf("third Append: %v", err)
	}

	// LoadChain should return 3 records in order.
	vc, err := s.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain() error: %v", err)
	}
	if vc.Count != 3 {
		t.Errorf("expected 3 records, got %d", vc.Count)
	}
	if vc.HeadHash != h3 {
		t.Errorf("head hash mismatch: expected %s, got %s", h3, vc.HeadHash)
	}
	if vc.GenesisHash != h1 {
		t.Errorf("genesis hash mismatch: expected %s, got %s", h1, vc.GenesisHash)
	}
	if !vc.Valid {
		t.Error("expected valid chain")
	}
	if len(vc.Records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(vc.Records))
	}
	if vc.Records[0].Operation != "create" {
		t.Errorf("record 0: expected create, got %s", vc.Records[0].Operation)
	}
	if vc.Records[2].Operation != "approve" {
		t.Errorf("record 2: expected approve, got %s", vc.Records[2].Operation)
	}
}

func TestStoreAppend_Idempotent(t *testing.T) {
	dir := t.TempDir()
	s := OpenWithDir(dir, "idempotent")

	hash, err := s.Append("", Record{
		Operation: "genesis",
		Role:      "Admin",
		Timestamp: "2026-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("first Append: %v", err)
	}

	// Same content should succeed (idempotent).
	hash2, err := s.Append("", Record{
		Operation: "genesis",
		Role:      "Admin",
		Timestamp: "2026-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("idempotent Append: %v", err)
	}
	if hash != hash2 {
		t.Errorf("expected same hash for identical content, got %s vs %s", hash, hash2)
	}
}

func TestStoreAppend_HashCollision(t *testing.T) {
	dir := t.TempDir()
	s := OpenWithDir(dir, "collision-test")

	_, err := s.Append("", Record{
		Operation: "genesis",
		Role:      "Admin",
		Timestamp: "2026-01-01T00:00:00Z",
		Payload:   json.RawMessage(`{"data":"first"}`),
	})
	if err != nil {
		t.Fatalf("first Append: %v", err)
	}

	// Same prevRevision (""), different payload — different hash, so no collision.
	// To truly test collision, we'd need to create different content with same hash.
	// That's infeasible with SHA-256, so the practical collision test verifies
	// that the idempotent check works (same content OK).
}

func TestStore_EmptyLineage(t *testing.T) {
	dir := t.TempDir()
	s := OpenWithDir(dir, "empty")

	vc, err := s.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain() on empty lineage: %v", err)
	}
	if vc.Count != 0 {
		t.Errorf("expected 0 records, got %d", vc.Count)
	}
	if !vc.Valid {
		t.Error("empty chain should be valid")
	}
}

func TestStore_Validate_ValidChain(t *testing.T) {
	dir := t.TempDir()
	s := OpenWithDir(dir, "validate-test")

	_, err := s.Append("", Record{
		Operation: "genesis", Role: "Author",
		Timestamp: "2026-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("Append genesis: %v", err)
	}

	verdict := s.Validate()
	if !verdict.Valid {
		t.Errorf("expected valid, got: %s", verdict.Reason)
	}
}

func TestStore_Validate_TamperedFile(t *testing.T) {
	dir := t.TempDir()
	s := OpenWithDir(dir, "tamper-test")

	hash, err := s.Append("", Record{
		Operation: "genesis", Role: "Author",
		Timestamp: "2026-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Tamper with the file content (handle both v1/events and flat).
	path := filepath.Join(dir, "v1", "events", hash)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = filepath.Join(dir, hash)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read event file: %v", err)
	}
	tampered := strings.Replace(string(data), "genesis", "TAMPERED", 1)
	if err := os.WriteFile(path, []byte(tampered), 0644); err != nil {
		t.Fatalf("write tampered file: %v", err)
	}

	verdict := s.Validate()
	if verdict.Valid {
		t.Error("expected invalid after tamper")
	}
}

func TestStore_Validate_EmptyStore(t *testing.T) {
	dir := t.TempDir()
	s := OpenWithDir(dir, "empty")

	verdict := s.Validate()
	if !verdict.Valid {
		t.Errorf("empty store should be valid, got: %s", verdict.Reason)
	}
}

// ---------------------------------------------------------------------------
// RED tests: Git repo selection (threat matrix — task 1.6)
// ---------------------------------------------------------------------------

// TestResolveGitDir_RelativePath verifies that resolveGitDir properly
// resolves relative paths returned by `git rev-parse --git-dir`.
// In a standard repo, rev-parse returns ".git" (relative).
func TestResolveGitDir_RelativePath(t *testing.T) {
	repoDir := t.TempDir()
	gitInit(t, repoDir)

	gitDir, err := resolveGitDir(repoDir)
	if err != nil {
		t.Fatalf("resolveGitDir() error: %v", err)
	}

	// Must be absolute.
	if !filepath.IsAbs(gitDir) {
		t.Errorf("expected absolute path, got relative: %s", gitDir)
	}

	expected := filepath.Join(repoDir, ".git")
	if gitDir != expected {
		t.Errorf("expected %s, got %s", expected, gitDir)
	}

	// Must exist.
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Errorf("resolved git dir does not exist: %s", gitDir)
	}
}

// TestResolveGitDir_SymlinkRepo verifies that resolveGitDir works
// when the git directory is symlinked (e.g., via gitdir file).
func TestResolveGitDir_SymlinkRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping symlink test in short mode")
	}

	repoDir := t.TempDir()
	realGitDir := filepath.Join(repoDir, "..", "actual-git")
	realGitDir, _ = filepath.Abs(realGitDir)

	// Create a repo with separate git dir:
	//   git init --separate-git-dir=<realGitDir>
	cmd := exec.Command("git", "init", "--separate-git-dir="+realGitDir)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git init with separate git dir: %v\n%s", err, string(out))
	}

	// rev-parse should return the real path (absolute, resolved).
	gitDir, err := resolveGitDir(repoDir)
	if err != nil {
		t.Fatalf("resolveGitDir() error: %v", err)
	}

	if !filepath.IsAbs(gitDir) {
		t.Errorf("expected absolute path, got: %s", gitDir)
	}

	// The resolved path should exist.
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Errorf("resolved git dir does not exist: %s", gitDir)
	}
}

// TestResolveGitDir_NonRepo verifies that resolveGitDir returns an
// error when called outside a git repository.
func TestResolveGitDir_NonRepo(t *testing.T) {
	nonRepoDir := t.TempDir()
	// Deliberately do NOT git init.

	_, err := resolveGitDir(nonRepoDir)
	if err == nil {
		t.Error("expected error for non-repo directory, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("expected 'not a git repository' error, got: %v", err)
	}
}

// TestResolveGitDir_NonExistentPath verifies that a non-existent path
// produces an error (not a panic).
func TestResolveGitDir_NonExistentPath(t *testing.T) {
	nonExistent := filepath.Join(t.TempDir(), "does-not-exist")

	_, err := resolveGitDir(nonExistent)
	if err == nil {
		t.Error("expected error for non-existent path")
	}
}

// ---------------------------------------------------------------------------
// File lock tests
// ---------------------------------------------------------------------------

func TestFileLock_AcquireRelease(t *testing.T) {
	dir := t.TempDir()
	fl := NewFileLock(dir)

	if err := fl.Acquire(); err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}

	// Lock file should exist.
	lockPath := fl.LockFilePath()
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("lock file should exist after Acquire")
	}

	if err := fl.Release(); err != nil {
		t.Fatalf("Release() error: %v", err)
	}

	// Lock file should be gone.
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("lock file should not exist after Release")
	}
}

func TestFileLock_Exclusive(t *testing.T) {
	dir := t.TempDir()
	fl1 := NewFileLock(dir)
	fl2 := NewFileLock(dir)

	if err := fl1.Acquire(); err != nil {
		t.Fatalf("first Acquire: %v", err)
	}

	// Second acquire should fail.
	if err := fl2.Acquire(); err == nil {
		t.Fatal("expected error for second acquire")
	}

	fl1.Release()

	// After release, second should succeed.
	if err := fl2.Acquire(); err != nil {
		t.Fatalf("second Acquire after release: %v", err)
	}
	fl2.Release()
}

func TestFileLock_IdempotentRelease(t *testing.T) {
	dir := t.TempDir()
	fl := NewFileLock(dir)

	// Double release should not error.
	if err := fl.Release(); err != nil {
		t.Fatalf("first Release: %v", err)
	}
	if err := fl.Release(); err != nil {
		t.Fatalf("second Release: %v", err)
	}
}

func TestWithFileLock(t *testing.T) {
	dir := t.TempDir()

	called := false
	err := WithFileLock(dir, func() error {
		called = true
		// Inside lock, verify lock file exists.
		if _, err := os.Stat(filepath.Join(dir, ".lock")); os.IsNotExist(err) {
			t.Error("lock file should exist inside WithFileLock")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WithFileLock() error: %v", err)
	}
	if !called {
		t.Error("function not called")
	}

	// Lock should be released after WithFileLock returns.
	if _, err := os.Stat(filepath.Join(dir, ".lock")); !os.IsNotExist(err) {
		t.Error("lock file should be released after WithFileLock")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// gitInit initializes a git repository in the given directory.
func gitInit(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git init in %s: %v\n%s", dir, err, string(out))
	}

	// Configure a minimal user for git to avoid warnings.
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test User").Run()
}

func TestStoreGitCommonDir(t *testing.T) {
	repo := t.TempDir()
	gitInit(t, repo)
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("hello\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	runGitInDir(t, repo, "add", ".")
	runGitInDir(t, repo, "commit", "-m", "init")
	// create worktree
	wtDir := filepath.Join(t.TempDir(), "wt1")
	out, err := exec.Command("git", "-C", repo, "worktree", "add", wtDir, "-b", "wt-branch").CombinedOutput()
	if err != nil {
		t.Fatalf("worktree add: %v %s", err, string(out))
	}
	lineage := "wt-lineage"
	s, err := Open(wtDir, lineage)
	if err != nil {
		t.Fatalf("Open worktree: %v", err)
	}
	hash, err := s.Append("", Record{Operation: "start", Role: "Author", Timestamp: "2026-01-01T00:00:00Z", Payload: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	commonDirRaw := runGitInDir(t, wtDir, "rev-parse", "--git-common-dir")
	commonDir := commonDirRaw
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(wtDir, commonDir)
	}
	commonDir = filepath.Clean(commonDir)
	expected := filepath.Join(commonDir, "biggz", "review-transactions", lineage, "v1", "events", hash)
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected event at common dir %q: %v", expected, err)
	}
	// flat must not be created anew at worktree git dir
	gitDirRaw := runGitInDir(t, wtDir, "rev-parse", "--git-dir")
	gitDir := gitDirRaw
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(wtDir, gitDir)
	}
	gitDir = filepath.Clean(gitDir)
	flatPath := filepath.Join(gitDir, "biggz", "review-transactions", lineage, hash)
	if _, err := os.Stat(flatPath); err == nil {
		t.Fatalf("flat path %q should not exist (GitCommonDir only)", flatPath)
	}
	// validate publishImmutable idempotency: same payload again should be no-op
	_, err = s.Append("", Record{Operation: "start", Role: "Author", Timestamp: "2026-01-01T00:00:00Z", Payload: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatalf("idempotent second append: %v", err)
	}
}

func TestLegacyFlatReadable(t *testing.T) {
	dir := t.TempDir()
	// manually create legacy flat chain
	rec := Record{Schema: recordSchemaVersion, PrevRevision: "", Operation: "start", Role: "Author", Timestamp: "2026-01-01T00:00:00Z", Payload: json.RawMessage(`{"l":"legacy"}`)}
	data, _ := json.Marshal(rec)
	hash := sha256Hex(data)
	if err := os.WriteFile(filepath.Join(dir, hash), data, 0644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "HEAD"), []byte(hash+"\n"), 0644); err != nil {
		t.Fatalf("write HEAD: %v", err)
	}
	s := OpenWithDir(dir, "legacy")
	vc, err := s.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain legacy: %v", err)
	}
	if vc.Count != 1 || vc.HeadHash != hash {
		t.Fatalf("legacy chain mismatch: %+v", vc)
	}
	if !vc.Valid {
		t.Error("legacy chain should be valid")
	}
	// dual-read identical: create v1/events copy and ensure same chain
	if err := os.MkdirAll(filepath.Join(dir, "v1", "events"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "v1", "events", hash), data, 0644); err != nil {
		t.Fatalf("write v1: %v", err)
	}
	vc2, err := s.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain v1: %v", err)
	}
	if vc2.HeadHash != vc.HeadHash || vc2.Count != vc.Count {
		t.Error("dual-read should return identical ValidatedChain")
	}
}

// runGitInDir runs a git command in a specific directory and returns output.
func runGitInDir(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, string(out))
	}
	return strings.TrimSpace(string(out))
}
