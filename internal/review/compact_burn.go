package review

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrAuthorityLockTimeout = errors.New("review authority lock timeout")
	storeResetLockTimeout   = 2 * time.Second
)

type ReviewAuthorityBurnStateError struct {
	LineageID string
	Version   string
	State     string
	Required  string
}

func (e *ReviewAuthorityBurnStateError) Error() string {
	return fmt.Sprintf("review authority burn refused for %s lineage %q: state %q is not %q", e.Version, e.LineageID, e.State, e.Required)
}

type ReviewAuthorityBurnIncompleteError struct {
	LineageID string
	Residue   []string
	Cause     error
}

func (e *ReviewAuthorityBurnIncompleteError) Error() string {
	return fmt.Sprintf("review authority burn for lineage %q is incomplete: owned residue remains at %s: %v", e.LineageID, strings.Join(e.Residue, ", "), e.Cause)
}
func (e *ReviewAuthorityBurnIncompleteError) Unwrap() error { return e.Cause }

type CompactRevisionConflictError struct {
	LineageID string
	Expected  string
	Current   string
}

func (e *CompactRevisionConflictError) Error() string {
	return fmt.Sprintf("compact revision conflict for lineage %q: expected %q, got %q", e.LineageID, e.Expected, e.Current)
}

var storeResetRemoveTreeFn = os.RemoveAll

func validateLineageID(id string) error {
	if id == "" || len(id) > 128 {
		return errors.New("invalid lineage ID")
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') && c != '-' && c != '_' {
			return fmt.Errorf("invalid lineage ID %q", id)
		}
	}
	return nil
}

func reviewAuthorityRoot(ctx context.Context, repo string) (string, string, error) {
	gitDir, err := resolveGitDir(repo)
	if err != nil {
		if repo == "" {
			repo, _ = os.Getwd()
		}
		gitDir = filepath.Join(repo, ".git")
	}
	base := filepath.Join(gitDir, "biggz", "review-compact")
	_ = ctx
	return base, gitDir, nil
}

func storeResetAcquireLease(ctx context.Context, repo string) (*LeaseHandle, error) {
	base, _, err := reviewAuthorityRoot(ctx, repo)
	if err != nil {
		return nil, err
	}
	lockPath := filepath.Join(base, "LOCK")
	dir := filepath.Dir(lockPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(storeResetLockTimeout)
	for {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			_, _ = f.WriteString(fmt.Sprintf("%d\n", os.Getpid()))
			f.Close()
			return &LeaseHandle{path: lockPath}, nil
		}
		if !os.IsExist(err) {
			return nil, &ReviewAuthorityBurnIncompleteError{LineageID: "", Residue: []string{lockPath}, Cause: err}
		}
		if time.Now().After(deadline) {
			return nil, ErrAuthorityLockTimeout
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("acquire review maintenance lease: %w", ErrAuthorityLockTimeout)
		case <-time.After(50 * time.Millisecond):
		}
	}
}

type LeaseHandle struct {
	path string
}

func (l *LeaseHandle) Release() {
	if l == nil {
		return
	}
	_ = os.Remove(l.path)
}

func ensureNoPreparedCompactBatchReconciliation(base string) error {
	_ = base
	return nil
}

func acquireLocalStoreLock(path string) (*LeaseHandle, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			return nil, ErrAuthorityLockTimeout
		}
		return nil, err
	}
	_, _ = f.WriteString(fmt.Sprintf("%d\n", os.Getpid()))
	f.Close()
	return &LeaseHandle{path: path}, nil
}

func (l *LeaseHandle) release() { l.Release() }

type CompactStoreFile struct {
	Dir       string
	lineageID string
}

func (s CompactStoreFile) loadCompactRecordLocked() (CompactRecordFile, error) {
	path := filepath.Join(s.Dir, "record.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return CompactRecordFile{}, fmt.Errorf("load compact authority: %w: %v", fs.ErrNotExist, err)
		}
		return CompactRecordFile{}, fmt.Errorf("load compact authority: %w", err)
	}
	var rec CompactRecordFile
	if err := json.Unmarshal(data, &rec); err != nil {
		return CompactRecordFile{}, fmt.Errorf("parse compact record: %w", err)
	}
	return rec, nil
}

type CompactRecordFile struct {
	Revision  string `json:"revision"`
	State     string `json:"state"`
	LineageID string `json:"lineage_id"`
}

func BurnApprovedCompactAuthority(ctx context.Context, repo, lineageID, expectedRevision string) error {
	if err := validateLineageID(lineageID); err != nil {
		return err
	}
	base, _, err := reviewAuthorityRoot(ctx, repo)
	if err != nil {
		return err
	}
	lockCtx, cancel := context.WithTimeout(ctx, storeResetLockTimeout)
	defer cancel()
	maintenance, err := storeResetAcquireLease(lockCtx, repo)
	if err != nil {
		return fmt.Errorf("acquire review maintenance lease: %w", err)
	}
	defer maintenance.Release()
	if err := ensureNoPreparedCompactBatchReconciliation(base); err != nil {
		return err
	}
	versionLock, err := acquireLocalStoreLock(filepath.Join(base, "v2", "LOCK"))
	if err != nil {
		return fmt.Errorf("acquire compact authority version lock: %w", err)
	}
	defer versionLock.release()

	store := CompactStoreFile{Dir: filepath.Join(base, "v2", lineageID), lineageID: lineageID}
	record, err := store.loadCompactRecordLocked()
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("load compact authority: %w: lineage not found", err)
		}
		return fmt.Errorf("load compact authority: %w", err)
	}
	if record.Revision != expectedRevision {
		return &CompactRevisionConflictError{LineageID: lineageID, Expected: expectedRevision, Current: record.Revision}
	}
	if record.State != "approved" {
		return &ReviewAuthorityBurnStateError{LineageID: lineageID, Version: "v2", State: record.State, Required: "approved"}
	}
	for _, path := range []string{
		filepath.Join(base, "effect-markers", "v1", lineageID),
		filepath.Join(base, "incidents", lineageID),
		store.Dir,
	} {
		if err := removeExactCompactBurnPath(lineageID, path); err != nil {
			return err
		}
	}
	return nil
}

func removeExactCompactBurnPath(lineageID, path string) error {
	if err := storeResetRemoveTreeFn(path); err != nil {
		return &ReviewAuthorityBurnIncompleteError{LineageID: lineageID, Residue: []string{path}, Cause: err}
	}
	if _, err := os.Lstat(path); err == nil {
		return &ReviewAuthorityBurnIncompleteError{
			LineageID: lineageID,
			Residue:   []string{path},
			Cause:     errors.New("owned burn path remains after deletion"),
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return &ReviewAuthorityBurnIncompleteError{LineageID: lineageID, Residue: []string{path}, Cause: err}
	}
	return nil
}
