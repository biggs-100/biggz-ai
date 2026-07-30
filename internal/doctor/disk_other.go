//go:build !windows

package doctor

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
)

const (
	// DiskCheckID is the check identifier for free disk space.
	DiskCheckID CheckID = "disk"
)

// DiskCheck verifies that the biggz data partition has sufficient free disk
// space. This is a stub implementation for non-Windows platforms.
type DiskCheck struct {
	biggzDir string
	diskFree func(string) (freeBytes, totalBytes int64, err error)
}

// NewDiskCheck creates a DiskCheck using the default ~/.biggz/ directory.
func NewDiskCheck() *DiskCheck {
	return &DiskCheck{
		biggzDir: "",
		diskFree: nil,
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

// Run returns a WARNING indicating the disk check is not supported on this platform.
func (c *DiskCheck) Run(ctx context.Context) *Result {
	return &Result{
		ID:       DiskCheckID,
		Status:   StatusWarn,
		Message:  "Disk space check not supported on this platform",
		Severity: SeverityWarning,
		Error:    fmt.Sprintf("platform %s/%s not supported", runtime.GOOS, runtime.GOARCH),
	}
}

// Remedy returns nil.
func (c *DiskCheck) Remedy() *Remedy { return nil }
