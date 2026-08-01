package review

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

// ---------------------------------------------------------------------------
// File-based locking (per-lineage)
// ---------------------------------------------------------------------------

// FileLock provides cross-process exclusive locking via a lock file.
// The lock file is stored at:
//
//	.git/biggz/review-transactions/<lineage>/.lock
//
// Acquire creates the lock file atomically using O_EXCL | O_CREATE.
// Release removes the lock file.
//
// FileLock is NOT recursive. Calling Acquire twice without Release
// will deadlock the second caller until the first caller's process
// releases it or the file is stale.
type FileLock struct {
	dir  string
	name string
}

// NewFileLock creates a FileLock for the given store directory.
// The .lock file will be created at <dir>/.lock.
func NewFileLock(dir string) *FileLock {
	return &FileLock{dir: dir, name: ".lock"}
}

// NewNamedFileLock creates a FileLock whose lock file is named name
// (e.g. "LOCK") instead of ".lock", for stores that publish a LOCK file
// as part of their on-disk contract (the SDD runtime ledger).
func NewNamedFileLock(dir, name string) *FileLock {
	return &FileLock{dir: dir, name: name}
}

// LockFilePath returns the full path to the lock file.
func (fl *FileLock) LockFilePath() string {
	return filepath.Join(fl.dir, fl.name)
}

// Acquire acquires the exclusive file lock. It creates the .lock file
// atomically. If the file already exists, the lock is held by another
// process and Acquire returns an error.
//
// The caller MUST call Release when done, even on error paths, to
// prevent stale lock files in crash scenarios.
func (fl *FileLock) Acquire() error {
	// Ensure the directory exists.
	if err := os.MkdirAll(fl.dir, 0755); err != nil {
		return fmt.Errorf("file lock acquire: create dir: %w", err)
	}

	path := fl.LockFilePath()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("file lock acquire: lock held at %s", path)
		}
		return fmt.Errorf("file lock acquire: %w", err)
	}
	f.Close()
	return nil
}

// Release releases the file lock by removing the .lock file.
func (fl *FileLock) Release() error {
	path := fl.LockFilePath()
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			// Already released — idempotent.
			return nil
		}
		return fmt.Errorf("file lock release: %w", err)
	}
	return nil
}

// WithFileLock executes f with an exclusive file lock held.
// The lock is released when f returns.
func WithFileLock(dir string, f func() error) error {
	return WithNamedFileLock(dir, ".lock", f)
}

// WithNamedFileLock executes f with an exclusive lock on the named lock
// file held. The lock is released when f returns.
func WithNamedFileLock(dir, name string, f func() error) error {
	fl := NewNamedFileLock(dir, name)
	if err := fl.Acquire(); err != nil {
		return err
	}
	defer fl.Release()
	return f()
}
