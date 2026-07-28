package review

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// LockManager provides concurrent access protection for review operations.
// Ported from gentle-ai's store_lock.go — simplified for in-memory use.
// Supports shared (read) and exclusive (write) locking per review ID.
type LockManager struct {
	mu    sync.Mutex
	locks map[string]*reviewLock
}

type lockKind int

const (
	lockShared    lockKind = 0
	lockExclusive lockKind = 1
)

type reviewLock struct {
	kind      lockKind
	readers   int
	createdAt time.Time
	mu        sync.Mutex
}

// NewLockManager creates a new lock manager.
func NewLockManager() *LockManager {
	return &LockManager{locks: make(map[string]*reviewLock)}
}

// AcquireRead acquires a shared (read) lock for the given review ID.
// Multiple readers can hold the lock simultaneously.
func (lm *LockManager) AcquireRead(ctx context.Context, reviewID string) error {
	return lm.acquire(ctx, reviewID, lockShared)
}

// AcquireWrite acquires an exclusive (write) lock for the given review ID.
// Only one writer can hold the lock at a time; blocks readers.
func (lm *LockManager) AcquireWrite(ctx context.Context, reviewID string) error {
	return lm.acquire(ctx, reviewID, lockExclusive)
}

// Release releases any lock held for the given review ID.
func (lm *LockManager) Release(reviewID string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lk, ok := lm.locks[reviewID]
	if !ok {
		return
	}

	lk.mu.Lock()
	defer lk.mu.Unlock()

	if lk.kind == lockShared {
		lk.readers--
		if lk.readers <= 0 {
			delete(lm.locks, reviewID)
		}
	} else {
		delete(lm.locks, reviewID)
	}
}

func (lm *LockManager) acquire(ctx context.Context, reviewID string, kind lockKind) error {
	timeout := time.After(5 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("lock acquisition timed out for review %q", reviewID)
		default:
		}

		lm.mu.Lock()
		existing, exists := lm.locks[reviewID]

		if !exists {
			// No existing lock — create new
			lk := &reviewLock{kind: kind, createdAt: time.Now()}
			if kind == lockShared {
				lk.readers = 1
			}
			lm.locks[reviewID] = lk
			lm.mu.Unlock()
			return nil
		}

		if kind == lockShared && existing.kind == lockShared {
			// Shared locks are compatible
			existing.readers++
			lm.mu.Unlock()
			return nil
		}

		// Conflict — need to wait
		lm.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
}

// WithReadLock executes a function with a shared lock held.
func WithReadLock(ctx context.Context, lm *LockManager, reviewID string, fn func() error) error {
	if err := lm.AcquireRead(ctx, reviewID); err != nil {
		return err
	}
	defer lm.Release(reviewID)
	return fn()
}

// WithWriteLock executes a function with an exclusive lock held.
func WithWriteLock(ctx context.Context, lm *LockManager, reviewID string, fn func() error) error {
	if err := lm.AcquireWrite(ctx, reviewID); err != nil {
		return err
	}
	defer lm.Release(reviewID)
	return fn()
}
