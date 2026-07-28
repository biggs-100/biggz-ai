package review

import (
	"context"
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
