//go:build windows

package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

const (
	// DiskCheckID is the check identifier for free disk space.
	DiskCheckID CheckID = "disk"
)

// DiskCheck verifies that the biggz data partition has sufficient free disk
// space. Below 500 MB free produces a WARNING.
type DiskCheck struct {
	biggzDir string
	// diskFree allows injection for testing.
	diskFree func(string) (freeBytes, totalBytes int64, err error)
}

// NewDiskCheck creates a DiskCheck using the default ~/.biggz/ directory.
func NewDiskCheck() *DiskCheck {
	return &DiskCheck{
		biggzDir: "",
		diskFree: getDiskFreeSpace,
	}
}

// NewDiskCheckWithCustom creates a DiskCheck with custom paths for testing.
func NewDiskCheckWithCustom(biggzDir string, diskFree func(string) (int64, int64, error)) *DiskCheck {
	return &DiskCheck{
		biggzDir: biggzDir,
		diskFree: diskFree,
	}
}

// ID returns the check identifier.
func (c *DiskCheck) ID() CheckID { return DiskCheckID }

// Run checks for sufficient free disk space.
func (c *DiskCheck) Run(ctx context.Context) *Result {
	biggzDir := c.biggzDir
	if biggzDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return &Result{
				ID:       DiskCheckID,
				Status:   StatusWarn,
				Message:  "Cannot determine home directory for disk check",
				Severity: SeverityWarning,
				Error:    err.Error(),
			}
		}
		biggzDir = filepath.Join(home, ".biggz")
	}

	// Get the root path of the drive containing the biggz directory.
	absPath, err := filepath.Abs(biggzDir)
	if err != nil {
		return &Result{
			ID:       DiskCheckID,
			Status:   StatusWarn,
			Message:  "Cannot resolve biggz directory path for disk check",
			Severity: SeverityWarning,
			Error:    err.Error(),
		}
	}

	// On Windows, use the drive root (e.g., "C:\").
	volumeRoot := filepath.VolumeName(absPath) + "\\"
	if volumeRoot == "\\" {
		volumeRoot = absPath
	}

	freeBytes, totalBytes, err := c.diskFree(volumeRoot)
	if err != nil {
		return &Result{
			ID:       DiskCheckID,
			Status:   StatusWarn,
			Message:  "Cannot check free disk space",
			Severity: SeverityWarning,
			Error:    err.Error(),
		}
	}

	const warnThreshold int64 = 500 * 1024 * 1024 // 500 MB

	freeMB := freeBytes / (1024 * 1024)
	totalMB := totalBytes / (1024 * 1024)

	if freeBytes < warnThreshold {
		return &Result{
			ID:       DiskCheckID,
			Status:   StatusWarn,
			Message:  fmt.Sprintf("Low disk space: %d MB free of %d MB on %s", freeMB, totalMB, volumeRoot),
			Severity: SeverityWarning,
			Error:    fmt.Sprintf("free: %d bytes, threshold: %d bytes", freeBytes, warnThreshold),
		}
	}

	return &Result{
		ID:       DiskCheckID,
		Status:   StatusPass,
		Message:  fmt.Sprintf("Disk space OK: %d MB free of %d MB on %s", freeMB, totalMB, volumeRoot),
		Severity: SeverityInfo,
	}
}

// getDiskFreeSpace retrieves free disk space using the Windows API
// via golang.org/x/sys/windows.GetDiskFreeSpaceEx.
func getDiskFreeSpace(path string) (freeBytes, totalBytes int64, err error) {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}

	var free, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(pathPtr, &free, &total, &totalFree); err != nil {
		return 0, 0, err
	}

	return int64(free), int64(total), nil
}

// Remedy returns nil — freeing disk space is manual.
func (c *DiskCheck) Remedy() *Remedy { return nil }
