package review

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
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

// BusyError is returned when a FileLock cannot be acquired because another
// process holds it and the lock is not stale. Callers can use IsBusy to
// distinguish this from other I/O errors and retry with a bounded wait.
type BusyError struct {
	Path string
}

func (e *BusyError) Error() string {
	return fmt.Sprintf("file lock acquire: lock held at %s", e.Path)
}

// IsBusy reports whether err is a BusyError (possibly wrapped).
func IsBusy(err error) bool {
	var busy *BusyError
	return errors.As(err, &busy)
}

// staleLockAge is the maximum age after which a lock file is considered
// stale and may be removed, even if the owning PID appears alive. This
// prevents deadlocks when a holder crashes without cleaning up.
const staleLockAge = 5 * time.Minute

// FileLock provides cross-process exclusive locking via a lock file.
// The lock file is stored at:
//
//	.git/biggz/review-transactions/<lineage>/.lock
//
// Acquire creates the lock file atomically using O_EXCL | O_CREATE and
// writes the current PID and timestamp for stale detection. If the file
// already exists, Acquire checks whether the holder is still alive or the
// file is older than staleLockAge; stale locks are removed and acquisition
// is retried once.
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

// LockPathFor returns a deterministic lock file path for a target under
// lockRoot, hashing the target with SHA-256 (first 16 hex chars). It is a
// minimal helper for callers that need per-target lock files without
// creating subdirectories per target.
func LockPathFor(lockRoot, target string) string {
	sum := sha256.Sum256([]byte(target))
	return filepath.Join(lockRoot, hex.EncodeToString(sum[:])[:16]+".lock")
}

// Acquire acquires the exclusive file lock. It creates the lock file
// atomically. If the file already exists and is not stale, it returns a
// *BusyError. The lock file contains the PID and timestamp for debugging
// and stale detection.
func (fl *FileLock) Acquire() error {
	// Ensure the directory exists.
	if err := os.MkdirAll(fl.dir, 0755); err != nil {
		return fmt.Errorf("file lock acquire: create dir: %w", err)
	}

	path := fl.LockFilePath()
	// Try up to two attempts: first try, then one retry after removing a
	// stale lock.
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			// Write PID + timestamp for debugging and stale detection.
			_, _ = fmt.Fprintf(f, "%d\n%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano))
			f.Close()
			return nil
		}
		if !os.IsExist(err) {
			return fmt.Errorf("file lock acquire: %w", err)
		}
		// File exists — check if stale.
		stale, statErr := fl.isStale(path)
		if statErr != nil {
			// If we cannot determine staleness, treat as busy.
			return &BusyError{Path: path}
		}
		if !stale {
			return &BusyError{Path: path}
		}
		// Stale — remove and retry.
		_ = os.Remove(path)
	}
	return &BusyError{Path: fl.LockFilePath()}
}

// AcquireWithTimeout polls Acquire every 100ms until timeout elapses.
// It is a minimal cooperative wait matching gentle-ai's bounded lock wait.
func (fl *FileLock) AcquireWithTimeout(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		err := fl.Acquire()
		if err == nil {
			return nil
		}
		if !IsBusy(err) {
			return err
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// isStale reports whether the lock file at path is stale. A lock is stale
// if its age exceeds staleLockAge or its owning PID no longer exists.
func (fl *FileLock) isStale(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if time.Since(info.ModTime()) > staleLockAge {
		return true, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		return false, nil
	}
	pidStr := strings.TrimSpace(lines[0])
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		// No valid PID — rely on age only.
		return false, nil
	}
	if !isProcessAlive(pid) {
		return true, nil
	}
	return false, nil
}

// isProcessAlive reports whether a process with the given pid is alive.
// On Windows it always returns true (age-based staleness is the fallback)
// because Signal(0) is not reliable there; on Unix it uses kill(pid,0).
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	return true
}

// Release releases the file lock by removing the lock file.
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

// WithNamedFileLockTimeout executes f with an exclusive lock held, polling
// for up to timeout. It is the timeout-aware variant for callers like the SDD
// attempt CAS store that need bounded cooperative waits.
func WithNamedFileLockTimeout(dir, name string, timeout time.Duration, f func() error) error {
	fl := NewNamedFileLock(dir, name)
	if err := fl.AcquireWithTimeout(timeout); err != nil {
		return err
	}
	defer fl.Release()
	return f()
}
