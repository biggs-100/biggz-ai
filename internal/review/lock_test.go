package review

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"
)

func TestLockManager_SharedRead(t *testing.T) {
	lm := NewLockManager()
	ctx := context.Background()

	if err := lm.AcquireRead(ctx, "r1"); err != nil {
		t.Fatalf("AcquireRead() error: %v", err)
	}

	// Second read should succeed (shared lock)
	if err := lm.AcquireRead(ctx, "r1"); err != nil {
		t.Fatalf("Second AcquireRead() error: %v", err)
	}

	lm.Release("r1")
	lm.Release("r1") // second release
}

func TestLockManager_ExclusiveWrite(t *testing.T) {
	lm := NewLockManager()
	ctx := context.Background()

	if err := lm.AcquireWrite(ctx, "r1"); err != nil {
		t.Fatalf("AcquireWrite() error: %v", err)
	}

	// Second write should block (we can't test blocking easily)
	lm.Release("r1")

	// After release, should be able to acquire
	if err := lm.AcquireWrite(ctx, "r1"); err != nil {
		t.Fatalf("AcquireWrite() after release error: %v", err)
	}
	lm.Release("r1")
}

func TestLockManager_ReadBlocksWrite(t *testing.T) {
	lm := NewLockManager()
	ctx := context.Background()

	lm.AcquireRead(ctx, "r1")

	// Write should block (test with timeout context)
	timeoutCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	err := lm.AcquireWrite(timeoutCtx, "r1")
	if err == nil {
		t.Fatal("expected write to block when read held")
	}

	lm.Release("r1")
}

func TestLockManager_Concurrent(t *testing.T) {
	lm := NewLockManager()
	ctx := context.Background()
	var wg sync.WaitGroup

	// 10 concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if err := lm.AcquireRead(ctx, "concurrent"); err != nil {
				t.Errorf("goroutine %d: AcquireRead() error: %v", id, err)
				return
			}
			time.Sleep(5 * time.Millisecond)
			lm.Release("concurrent")
		}(i)
	}
	wg.Wait()
}

func TestWithReadLock(t *testing.T) {
	lm := NewLockManager()
	ctx := context.Background()

	called := false
	err := WithReadLock(ctx, lm, "test", func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("WithReadLock() error: %v", err)
	}
	if !called {
		t.Error("function not called")
	}
}

func TestWithWriteLock(t *testing.T) {
	lm := NewLockManager()
	ctx := context.Background()

	called := false
	err := WithWriteLock(ctx, lm, "test", func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("WithWriteLock() error: %v", err)
	}
	if !called {
		t.Error("function not called")
	}
}

func TestFlockBusyError(t *testing.T) {
	dir := t.TempDir()
	fl1 := NewFileLock(dir)
	if err := fl1.Acquire(); err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer fl1.Release()
	fl2 := NewFileLock(dir)
	err := fl2.Acquire()
	if err == nil {
		fl2.Release()
		t.Fatal("expected BusyError for second concurrent acquire")
	}
	if !IsBusy(err) {
		t.Fatalf("expected BusyError, got %v", err)
	}
	// AcquireWithTimeout should also busy with short timeout
	fl3 := NewFileLock(dir)
	err = fl3.AcquireWithTimeout(150 * time.Millisecond)
	if err == nil || !IsBusy(err) {
		t.Fatalf("AcquireWithTimeout should be BusyError, got %v", err)
	}
}

func TestStaleReaped(t *testing.T) {
	dir := t.TempDir()
	fl := NewFileLock(dir)
	path := fl.LockFilePath()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// write stale lock file with old mtime
	if err := os.WriteFile(path, []byte("999999\n2020-01-01T00:00:00Z\n"), 0644); err != nil {
		t.Fatalf("write stale: %v", err)
	}
	old := time.Now().Add(-6 * time.Minute)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	fl2 := NewFileLock(dir)
	if err := fl2.Acquire(); err != nil {
		t.Fatalf("stale lock should be reaped, got: %v", err)
	}
	fl2.Release()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("lock file should be removed after Release")
	}
}
