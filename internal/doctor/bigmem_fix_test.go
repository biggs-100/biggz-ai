package doctor

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/biggs-100/biggz-ai/internal/bigmem"
	"github.com/biggs-100/biggz-ai/internal/skillregistry"
	_ "modernc.org/sqlite"
)

func TestBigmemRemedy_NonNil(t *testing.T) {
	dir := t.TempDir()
	c := NewBigmemCheckWithOpener(func() (*bigmem.Store, error) {
		return bigmem.Open(dir)
	})
	remedy := c.Remedy()
	if remedy == nil {
		t.Fatal("Bigmem Remedy() returned nil, want non-nil")
	}
	if remedy.Description == "" {
		t.Error("Remedy Description empty")
	}
	if remedy.Action == nil {
		t.Error("Remedy Action nil")
	}
}

func TestBigmemRemedy_ReindexHealthy(t *testing.T) {
	dir := t.TempDir()
	s, err := bigmem.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Insert an observation to ensure FTS has data.
	if err := s.Save(&bigmem.Observation{Title: "hello", Content: "world", Type: "note"}); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()

	c := NewBigmemCheckWithOpener(func() (*bigmem.Store, error) {
		return bigmem.Open(dir)
	})
	remedy := c.Remedy()
	ctx := context.Background()
	if err := remedy.Action(ctx); err != nil {
		t.Fatalf("Action failed on healthy DB: %v", err)
	}
	// Idempotent second run.
	if err := remedy.Action(ctx); err != nil {
		t.Fatalf("second Action failed: %v", err)
	}
	// Verify FTS table exists.
	dbPath := filepath.Join(dir, "bigmem.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='observations_fts'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("observations_fts table count = %d, want 1", count)
	}
	// Verify integrity still ok.
	result := c.Run(ctx)
	if result.Status != StatusPass {
		t.Errorf("after remedy, Run status = %v, want pass (msg=%q err=%q)", result.Status, result.Message, result.Error)
	}
}

func TestBigmemRemedy_RecreateMissingFTS(t *testing.T) {
	dir := t.TempDir()
	s, err := bigmem.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save(&bigmem.Observation{Title: "reindex-test", Content: "content", Type: "note"}); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()

	dbPath := filepath.Join(dir, "bigmem.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DROP TABLE IF EXISTS observations_fts"); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	c := NewBigmemCheckWithOpener(func() (*bigmem.Store, error) {
		return bigmem.Open(dir)
	})
	remedy := c.Remedy()
	if err := remedy.Action(context.Background()); err != nil {
		t.Fatalf("Action failed after dropping FTS: %v", err)
	}
	// Verify FTS recreated.
	db2, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	var count int
	if err := db2.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='observations_fts'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("after remedy, observations_fts count = %d, want 1", count)
	}
	// Search should still work (best-effort repopulated).
	s2, err := bigmem.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	results, err := s2.Search("reindex-test", bigmem.SearchOptions{})
	if err != nil {
		t.Fatalf("search after remedy: %v", err)
	}
	// At least the DB is usable; search may return 0 or 1 depending on repopulation, but must not error.
	_ = results
}

func TestBigmemRemedy_MissingDBCreatesFresh(t *testing.T) {
	dir := t.TempDir()
	// Do not create DB beforehand.
	c := NewBigmemCheckWithOpener(func() (*bigmem.Store, error) {
		return bigmem.Open(dir)
	})
	remedy := c.Remedy()
	if err := remedy.Action(context.Background()); err != nil {
		t.Fatalf("Action failed on missing DB: %v", err)
	}
	dbPath := filepath.Join(dir, "bigmem.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("DB not created at %s: %v", dbPath, err)
	}
	// Idempotent second run.
	if err := remedy.Action(context.Background()); err != nil {
		t.Fatalf("second Action failed: %v", err)
	}
}

func TestBigmemRemedy_ContextCanceled(t *testing.T) {
	dir := t.TempDir()
	s, _ := bigmem.Open(dir)
	_ = s.Close()
	c := NewBigmemCheckWithOpener(func() (*bigmem.Store, error) {
		return bigmem.Open(dir)
	})
	remedy := c.Remedy()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := remedy.Action(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Review stale locks
// ---------------------------------------------------------------------------

func TestReviewRemedy_NonNil(t *testing.T) {
	c := NewReviewCheck()
	if r := c.Remedy(); r == nil {
		t.Fatal("Review Remedy() returned nil")
	}
}

func TestReviewRemedy_RemovesStaleLocks(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(filepath.Join(gitDir, "biggz", "review-transactions", "lineage1"), 0755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-10 * time.Minute)

	// Stale .lock via mtime
	staleLock := filepath.Join(gitDir, "biggz", "review-transactions", "lineage1", ".lock")
	if err := os.WriteFile(staleLock, []byte("999999\n2020-01-01T00:00:00Z\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(staleLock, old, old); err != nil {
		t.Fatal(err)
	}
	// Fresh LOCK (current PID, recent mtime) — must NOT be removed
	freshLock := filepath.Join(gitDir, "biggz", "review-transactions", "lineage1", "LOCK")
	if err := os.WriteFile(freshLock, []byte(fmt.Sprintf("%d\n%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano))), 0644); err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	// Stale in home .biggz/sdd-runtime
	stale2 := filepath.Join(home, ".biggz", "sdd-runtime", "v1", "test-change", "LOCK")
	if err := os.MkdirAll(filepath.Dir(stale2), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale2, []byte("999999\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(stale2, old, old); err != nil {
		t.Fatal(err)
	}
	// Extra *.lock file under home/.biggz
	extraStale := filepath.Join(home, ".biggz", "extra.lock")
	if err := os.WriteFile(extraStale, []byte("999999\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(extraStale, old, old); err != nil {
		t.Fatal(err)
	}
	// Fresh file under home that should stay
	freshHome := filepath.Join(home, ".biggz", "fresh.lock")
	if err := os.WriteFile(freshHome, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644); err != nil {
		t.Fatal(err)
	}
	// Do not Chtimes freshHome — stays recent

	c := NewReviewCheckWithCustom(
		func() (string, error) { return root, nil },
		os.Stat,
		os.ReadDir,
		func(string, ...string) ([]byte, error) { return nil, fmt.Errorf("no git") },
	)
	remedy := c.Remedy()
	if remedy == nil {
		t.Fatal("Remedy nil")
	}
	if err := remedy.Action(context.Background()); err != nil {
		t.Fatalf("Action failed: %v", err)
	}
	if _, err := os.Stat(staleLock); !os.IsNotExist(err) {
		t.Errorf("stale .lock not removed: %s exists, err=%v", staleLock, err)
	}
	if _, err := os.Stat(stale2); !os.IsNotExist(err) {
		t.Errorf("stale LOCK under home not removed: %s", stale2)
	}
	if _, err := os.Stat(extraStale); !os.IsNotExist(err) {
		t.Errorf("extra stale lock not removed: %s", extraStale)
	}
	if _, err := os.Stat(freshLock); err != nil {
		t.Errorf("fresh LOCK incorrectly removed: %v", err)
	}
	if _, err := os.Stat(freshHome); err != nil {
		t.Errorf("fresh home lock incorrectly removed: %v", err)
	}

	// Idempotent second run — should succeed even when stale already gone.
	if err := remedy.Action(context.Background()); err != nil {
		t.Fatalf("second Action failed: %v", err)
	}
}

func TestReviewRemedy_IdempotentNoLocks(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".git"), 0755)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	_ = os.MkdirAll(filepath.Join(home, ".biggz"), 0755)

	c := NewReviewCheckWithCustom(
		func() (string, error) { return root, nil },
		os.Stat,
		os.ReadDir,
		nil,
	)
	remedy := c.Remedy()
	if err := remedy.Action(context.Background()); err != nil {
		t.Fatalf("Action on empty roots failed: %v", err)
	}
	if err := remedy.Action(context.Background()); err != nil {
		t.Fatalf("second Action on empty roots failed: %v", err)
	}
}

func TestReviewRemedy_ContextCanceled(t *testing.T) {
	c := NewReviewCheck()
	remedy := c.Remedy()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := remedy.Action(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("expected Canceled, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Platform skill-registry
// ---------------------------------------------------------------------------

func TestPlatformRemedy_NonNil(t *testing.T) {
	c := NewPlatformCheck()
	if r := c.Remedy(); r == nil {
		t.Fatal("Platform Remedy() nil")
	}
	if r := c.Remedy(); r.Description == "" {
		t.Error("Platform Remedy Description empty")
	}
}

func TestPlatformRemedy_RefreshesRegistry(t *testing.T) {
	projectRoot := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	pluginDir := filepath.Join(home, ".config", "opencode", "plugins")
	_ = os.RemoveAll(pluginDir)

	c := NewPlatformCheckWithCustom(
		func() (string, error) { return projectRoot, nil },
		func() (string, error) { return home, nil },
		nil,
	)
	remedy := c.Remedy()
	if err := remedy.Action(context.Background()); err != nil {
		t.Fatalf("Action failed: %v", err)
	}
	// Verify plugin dir created
	if _, err := os.Stat(pluginDir); err != nil {
		t.Errorf("plugin dir not created: %v", err)
	}
	// Verify registry created
	regPath := filepath.Join(projectRoot, ".atl", "skill-registry.md")
	if _, err := os.Stat(regPath); err != nil {
		t.Errorf("registry not created at %s: %v", regPath, err)
	}
	// Idempotent second call
	if err := remedy.Action(context.Background()); err != nil {
		t.Fatalf("second Action failed: %v", err)
	}
}

func TestPlatformRemedy_ContextCanceled(t *testing.T) {
	c := NewPlatformCheck()
	remedy := c.Remedy()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := remedy.Action(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("expected Canceled, got %v", err)
	}
}

func TestPlatformRemedy_RefreshFailureFallback(t *testing.T) {
	projectRoot := t.TempDir()
	home := t.TempDir()
	calls := 0
	c := NewPlatformCheckWithCustom(
		func() (string, error) { return projectRoot, nil },
		func() (string, error) { return home, nil },
		func(s string, b bool) (*skillregistry.Result, error) {
			calls++
			return nil, fmt.Errorf("refresh fail %d", calls)
		},
	)
	remedy := c.Remedy()
	err := remedy.Action(context.Background())
	if err == nil {
		t.Error("expected error when refresh fails twice, got nil")
	}
	if calls != 2 {
		t.Errorf("expected 2 refresh calls (retry), got %d", calls)
	}
}

func TestPlatformRemedy_RefreshRetrySucceeds(t *testing.T) {
	projectRoot := t.TempDir()
	home := t.TempDir()
	calls := 0
	c := NewPlatformCheckWithCustom(
		func() (string, error) { return projectRoot, nil },
		func() (string, error) { return home, nil },
		func(s string, b bool) (*skillregistry.Result, error) {
			calls++
			if calls == 1 {
				return nil, fmt.Errorf("first fail")
			}
			return &skillregistry.Result{Regenerated: true, Registry: filepath.Join(s, ".atl", "skill-registry.md")}, nil
		},
	)
	remedy := c.Remedy()
	if err := remedy.Action(context.Background()); err != nil {
		t.Fatalf("expected success on retry, got %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}
