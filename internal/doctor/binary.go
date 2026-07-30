package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	// BinaryCheckID is the check identifier for MCP binary presence.
	BinaryCheckID CheckID = "binary"
)

// BinaryCheck verifies that the biggz-mcp binary exists at the expected
// location and has executable permissions.
type BinaryCheck struct {
	biggzDir string
	statFn   func(string) (os.FileInfo, error)
}

// NewBinaryCheck creates a BinaryCheck that checks the default ~/.biggz/ directory.
func NewBinaryCheck() *BinaryCheck {
	return &BinaryCheck{
		biggzDir: "",
		statFn:   os.Stat,
	}
}

// NewBinaryCheckWithCustom creates a BinaryCheck with custom paths for testing.
func NewBinaryCheckWithCustom(biggzDir string, statFn func(string) (os.FileInfo, error)) *BinaryCheck {
	return &BinaryCheck{
		biggzDir: biggzDir,
		statFn:   statFn,
	}
}

// ID returns the check identifier.
func (c *BinaryCheck) ID() CheckID { return BinaryCheckID }

// Run checks that biggz-mcp exists at the expected location.
func (c *BinaryCheck) Run(ctx context.Context) *Result {
	biggzDir := c.biggzDir
	if biggzDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return &Result{
				ID:       BinaryCheckID,
				Status:   StatusFail,
				Message:  "Cannot determine home directory",
				Severity: SeverityCritical,
				Error:    err.Error(),
			}
		}
		biggzDir = filepath.Join(home, ".biggz")
	}

	binaryName := "biggz-mcp"
	if runtime.GOOS == "windows" {
		binaryName = "biggz-mcp.exe"
	}

	binaryPath := filepath.Join(biggzDir, binaryName)

	info, err := c.statFn(binaryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Result{
				ID:       BinaryCheckID,
				Status:   StatusFail,
				Message:  fmt.Sprintf("MCP binary not found at %s", binaryPath),
				Severity: SeverityCritical,
				Error:    err.Error(),
			}
		}
		return &Result{
			ID:       BinaryCheckID,
			Status:   StatusFail,
			Message:  fmt.Sprintf("Cannot stat MCP binary at %s", binaryPath),
			Severity: SeverityCritical,
			Error:    err.Error(),
		}
	}

	// Verify it's not a directory.
	if info.IsDir() {
		return &Result{
			ID:       BinaryCheckID,
			Status:   StatusFail,
			Message:  fmt.Sprintf("Expected binary but found directory at %s", binaryPath),
			Severity: SeverityCritical,
		}
	}

	// On Unix, check executable permission bit.
	if runtime.GOOS != "windows" {
		mode := info.Mode()
		if mode&0o111 == 0 {
			return &Result{
				ID:       BinaryCheckID,
				Status:   StatusWarn,
				Message:  fmt.Sprintf("MCP binary at %s is not executable (mode: %o)", binaryPath, mode.Perm()),
				Severity: SeverityWarning,
			}
		}
	}

	return &Result{
		ID:       BinaryCheckID,
		Status:   StatusPass,
		Message:  fmt.Sprintf("MCP binary found at %s", binaryPath),
		Severity: SeverityInfo,
	}
}

// Remedy returns nil — reinstalling is handled externally.
func (c *BinaryCheck) Remedy() *Remedy { return nil }
