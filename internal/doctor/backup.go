package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// BackupCheckID is the check identifier for backup freshness.
	BackupCheckID CheckID = "backup"
)

// maxBackupAge is the maximum allowed age for the most recent backup.
const maxBackupAge = 7 * 24 * time.Hour // 7 days

// BackupCheck verifies that at least one backup exists and that the newest
// backup is within the acceptable age threshold (7 days).
type BackupCheck struct {
	backupDir string
	statFn    func(string) (os.FileInfo, error)
	readDirFn func(string) ([]os.DirEntry, error)
}

// backupEntry represents a discovered backup file with its modification time.
type backupEntry struct {
	path    string
	modTime time.Time
}

// NewBackupCheck creates a BackupCheck using the default backup directory.
func NewBackupCheck() *BackupCheck {
	return &BackupCheck{
		backupDir: "",
		statFn:    os.Stat,
		readDirFn: os.ReadDir,
	}
}

// NewBackupCheckWithCustom creates a BackupCheck with custom paths for testing.
func NewBackupCheckWithCustom(
	backupDir string,
	statFn func(string) (os.FileInfo, error),
	readDirFn func(string) ([]os.DirEntry, error),
) *BackupCheck {
	return &BackupCheck{
		backupDir: backupDir,
		statFn:    statFn,
		readDirFn: readDirFn,
	}
}

// ID returns the check identifier.
func (c *BackupCheck) ID() CheckID { return BackupCheckID }

// Run lists backups and checks the newest backup timestamp.
func (c *BackupCheck) Run(ctx context.Context) *Result {
	backupDir := c.backupDir
	if backupDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return &Result{
				ID:       BackupCheckID,
				Status:   StatusWarn,
				Message:  "Cannot determine home directory for backup check",
				Severity: SeverityWarning,
				Error:    err.Error(),
			}
		}
		backupDir = filepath.Join(home, ".biggz", "backups")
	}

	// Check if backups directory exists.
	_, err := c.statFn(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return &Result{
				ID:       BackupCheckID,
				Status:   StatusWarn,
				Message:  "No backups directory found — no backups have been created",
				Severity: SeverityWarning,
				Error:    err.Error(),
			}
		}
		return &Result{
			ID:       BackupCheckID,
			Status:   StatusWarn,
			Message:  "Cannot access backups directory",
			Severity: SeverityWarning,
			Error:    err.Error(),
		}
	}

	// Enumerate backup files.
	entries, err := c.readDirFn(backupDir)
	if err != nil {
		return &Result{
			ID:       BackupCheckID,
			Status:   StatusWarn,
			Message:  "Cannot list backup files",
			Severity: SeverityWarning,
			Error:    err.Error(),
		}
	}

	var backups []backupEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		backups = append(backups, backupEntry{
			path:    e.Name(),
			modTime: info.ModTime(),
		})
	}

	if len(backups) == 0 {
		return &Result{
			ID:       BackupCheckID,
			Status:   StatusWarn,
			Message:  "No backup files found in backups directory",
			Severity: SeverityWarning,
		}
	}

	// Find the newest backup.
	newest := backups[0]
	for _, b := range backups[1:] {
		if b.modTime.After(newest.modTime) {
			newest = b
		}
	}

	age := time.Since(newest.modTime)
	if age > maxBackupAge {
		days := int(age.Hours() / 24)
		return &Result{
			ID:       BackupCheckID,
			Status:   StatusWarn,
			Message:  fmt.Sprintf("Newest backup (%s) is %d days old — exceeds 7-day threshold", newest.path, days),
			Severity: SeverityWarning,
			Error:    fmt.Sprintf("age: %.0f hours, threshold: 168 hours", age.Hours()),
		}
	}

	hoursAgo := int(age.Hours())
	if hoursAgo < 1 {
		return &Result{
			ID:       BackupCheckID,
			Status:   StatusPass,
			Message:  fmt.Sprintf("Newest backup (%s) is less than 1 hour old", newest.path),
			Severity: SeverityInfo,
		}
	}

	return &Result{
		ID:       BackupCheckID,
		Status:   StatusPass,
		Message:  fmt.Sprintf("Backup OK — newest backup (%s) is %d hours old", newest.path, hoursAgo),
		Severity: SeverityInfo,
	}
}

// Remedy returns nil — creating backups is the user's responsibility.
func (c *BackupCheck) Remedy() *Remedy { return nil }
