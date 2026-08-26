package filecoord

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func canonicalTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	canon, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return dir
	}
	return canon
}

func TestAcquireGrantsExclusiveUntilRelease(t *testing.T) {
	base := canonicalTemp(t)
	root := filepath.Join(base, "locks")
	target := filepath.Join(base, "target.txt")
	_ = os.WriteFile(target, []byte("x"), 0644)

	lease, err := Acquire(context.Background(), target, root)
	if err != nil || lease == nil {
		t.Fatalf("Acquire first: %v", err)
	}
	_, busy := Acquire(context.Background(), target, root)
	if !errors.Is(busy, ErrBusy) {
		t.Fatalf("second Acquire should be BusyError, got %v", busy)
	}
	if err := lease.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	lease2, err := Acquire(context.Background(), target, root)
	if err != nil || lease2 == nil {
		t.Fatalf("Acquire after release: %v", err)
	}
	_ = lease2.Release()
}

func TestAcquireRejectsSymlinkedLockPath(t *testing.T) {
	base := canonicalTemp(t)
	root := filepath.Join(base, "locks")
	target := filepath.Join(base, "target.txt")
	_ = os.MkdirAll(root, 0755)
	_ = os.WriteFile(target, []byte("x"), 0644)
	path, _ := LockPath(root, target)
	// create symlink at lock path
	_ = os.WriteFile(filepath.Join(base, "victim.txt"), []byte("victim"), 0644)
	if err := os.Symlink(filepath.Join(base, "victim.txt"), path); err != nil {
		t.Skip("symlink not available")
	}
	lease, err := Acquire(context.Background(), target, root)
	if lease != nil || !errors.Is(err, ErrOperational) {
		t.Fatalf("symlinked lock path should be operational error, got %v %v", lease, err)
	}
}

func TestAcquireHonorsCancelledContext(t *testing.T) {
	base := canonicalTemp(t)
	root := filepath.Join(base, "never")
	target := filepath.Join(base, "target.txt")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	lease, err := Acquire(ctx, target, root)
	if lease != nil || !errors.Is(err, ErrOperational) {
		t.Fatalf("cancelled context should be operational, got %v %v", lease, err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("cancelled Acquire touched filesystem")
	}
}
