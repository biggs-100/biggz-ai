package sdd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ArchiveChange moves an SDD change from openspec/changes/<change> to
// openspec/changes/archive/<change>. If the destination already exists it
// is suffixed with a UTC timestamp (20060102-150405) to avoid silently
// clobbering via os.Rename, mirroring gentle-ai's 56fdd57c fix.
//
// The timestamp suffix uses UTC and second precision; callers that archive
// twice within the same second will still collide on the suffixed name and
// receive an error, which is safer than overwriting.
func ArchiveChange(openspecRoot, change string) (string, error) {
	if err := validateChangeName(change); err != nil {
		return "", err
	}
	src := filepath.Join(openspecRoot, "changes", change)
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("archive: source change %q not found at %s", change, src)
		}
		return "", fmt.Errorf("archive: stat source: %w", err)
	}
	archiveDir := filepath.Join(openspecRoot, "changes", "archive")
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return "", fmt.Errorf("archive: create archive dir: %w", err)
	}
	dst := filepath.Join(archiveDir, change)
	if _, err := os.Stat(dst); err == nil {
		dst = dst + "-" + time.Now().UTC().Format("20060102-150405")
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("archive: stat destination: %w", err)
	}
	// Re-check the suffixed destination to avoid clobbering if two archives
	// race within the same second.
	if _, err := os.Stat(dst); err == nil {
		return "", fmt.Errorf("archive destination collision: %s already exists; resolve the collision before archiving", dst)
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("archive: stat destination: %w", err)
	}
	// never auto-disable RDD
	if err := os.Rename(src, dst); err != nil {
		return "", fmt.Errorf("archive: move %s to %s: %w", src, dst, err)
	}
	return dst, nil
}

// validateChangeName is a local copy of the ledger validation to avoid
// importing sddattempt. Keep it in sync with sddattempt.validateChangeName.
func validateChangeName(change string) error {
	if change == "" {
		return fmt.Errorf("SDD change name is required")
	}
	if change == "." || change == ".." {
		return fmt.Errorf("invalid SDD change name %q", change)
	}
	for _, c := range change {
		if c == '/' || c == '\\' || c == 0 {
			return fmt.Errorf("invalid SDD change name %q", change)
		}
	}
	return nil
}
