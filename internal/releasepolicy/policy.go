// Package releasepolicy validates the run-marker attestation that binds a
// snapshot build to its CI run identity. This is a minimal port of
// gentle-ai's internal/releasepolicy/policy.go (validateRunMarker,
// directoryContains, validateSnapshotFile) intended as a pre-verify attestation
// step.
package releasepolicy

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// Validate proves the snapshot marker binds the expected run identity and,
// when dist/artifacts.json exists, that the snapshot predates no artifact.
// For minimal scope it always validates the marker itself; snapshot validation
// is available via ValidateSnapshotFile for callers that need it.
func Validate(root, markerPath, runID string) error {
	root, err := canonicalDirectory(root)
	if err != nil {
		return fmt.Errorf("resolve repository root: %w", err)
	}
	markerTime, err := validateRunMarker(root, markerPath, runID)
	if err != nil {
		return err
	}
	// Best-effort snapshot check for the canonical artifact. Missing snapshot
	// is not an attestation failure for the minimal marker-only flow; callers
	// that require snapshot freshness call ValidateSnapshotFile explicitly.
	snapshot := filepath.Join(root, "dist", "artifacts.json")
	if _, err := os.Lstat(snapshot); err == nil {
		if err := validateSnapshotFile(root, "dist/artifacts.json", markerTime); err != nil {
			return fmt.Errorf("snapshot metadata: %w", err)
		}
	}
	return nil
}

// ValidateSnapshotFile validates that artifactPath (slash-separated, under dist/)
// is a regular non-symlink file whose ModTime is >= markerTime.
func ValidateSnapshotFile(root, artifactPath string, markerTime time.Time) error {
	root, err := canonicalDirectory(root)
	if err != nil {
		return err
	}
	return validateSnapshotFile(root, artifactPath, markerTime)
}

func canonicalDirectory(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("path is not a directory")
	}
	return resolved, nil
}

func validateRunMarker(root, markerPath, runID string) (time.Time, error) {
	if runID == "" || len(runID) > 512 || strings.ContainsAny(runID, "\x00\r\n") {
		return time.Time{}, errors.New("snapshot run identity is invalid")
	}
	if markerPath == "" || !filepath.IsAbs(markerPath) {
		return time.Time{}, errors.New("snapshot marker must be an absolute path")
	}
	markerPath = filepath.Clean(markerPath)
	resolvedMarker, err := filepath.EvalSymlinks(markerPath)
	if err != nil {
		return time.Time{}, fmt.Errorf("resolve snapshot marker: %w", err)
	}
	if directoryContains(filepath.Join(root, "dist"), resolvedMarker) {
		return time.Time{}, errors.New("snapshot marker must remain outside the clean snapshot directory")
	}
	info, err := os.Lstat(markerPath)
	if err != nil {
		return time.Time{}, fmt.Errorf("read snapshot marker: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return time.Time{}, errors.New("snapshot marker must be a regular non-symlink file")
	}
	payload, err := os.ReadFile(markerPath)
	if err != nil {
		return time.Time{}, fmt.Errorf("read snapshot marker: %w", err)
	}
	if !bytes.Equal(payload, []byte(runID+"\n")) {
		return time.Time{}, errors.New("snapshot marker does not bind the current run identity")
	}
	return info.ModTime(), nil
}

// directoryContains reports whether path is directory itself or lies beneath
// it, deciding identity by device and inode rather than by comparing strings.
//
// This repeats internal/pathidentity.Contains on purpose, and it is the only
// copy that is allowed to exist. This package is compiled in isolation by the
// release-policy verifier, which copies policy.go into a bare module so the
// thing that validates a release cannot depend on the tree it is validating.
// An import would break that isolation, so the rule is duplicated rather than
// the policy being changed. Keep the two in step: internal/pathidentity
// states the policy and its limits in full.
func directoryContains(directory, path string) bool {
	directoryInfo, err := os.Stat(directory)
	if err != nil || !directoryInfo.IsDir() {
		return false
	}
	current, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for {
		if info, statErr := os.Stat(current); statErr == nil && info.IsDir() && os.SameFile(directoryInfo, info) {
			return true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false
		}
		current = parent
	}
}

func validateSnapshotFile(root, artifactPath string, markerTime time.Time) error {
	clean := path.Clean(artifactPath)
	if artifactPath == "" || clean != artifactPath || path.IsAbs(artifactPath) || !strings.HasPrefix(artifactPath, "dist/") {
		return errors.New("path must be canonical and remain under dist")
	}
	relative := strings.TrimPrefix(artifactPath, "dist/")
	if relative == "" || strings.HasPrefix(relative, "../") {
		return errors.New("path escapes the clean snapshot directory")
	}
	dist := filepath.Join(root, "dist")
	info, err := os.Lstat(dist)
	if err != nil {
		return fmt.Errorf("read clean snapshot directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("clean snapshot directory must be a real directory")
	}
	current := dist
	parts := strings.Split(relative, "/")
	for index, part := range parts {
		if part == "" || part == "." || part == ".." {
			return errors.New("path contains an invalid component")
		}
		current = filepath.Join(current, filepath.FromSlash(part))
		info, err = os.Lstat(current)
		if err != nil {
			return fmt.Errorf("snapshot output is missing: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("snapshot output path contains a symlink")
		}
		if index < len(parts)-1 && !info.IsDir() {
			return errors.New("snapshot output parent is not a directory")
		}
	}
	if !info.Mode().IsRegular() {
		return errors.New("snapshot output is not a regular file")
	}
	if info.ModTime().Before(markerTime) {
		return errors.New("snapshot output predates the current run marker")
	}
	return nil
}
