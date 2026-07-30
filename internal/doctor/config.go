package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// ConfigCheckID is the check identifier for config directory structure.
	ConfigCheckID CheckID = "config"
)

// ConfigCheck verifies that the ~/.biggz/ config directory and its required
// subdirectories exist.
type ConfigCheck struct {
	biggzDir  string
	statFn    func(string) (os.FileInfo, error)
	readDirFn func(string) ([]os.DirEntry, error)
}

// NewConfigCheck creates a ConfigCheck that checks the default ~/.biggz/ directory.
func NewConfigCheck() *ConfigCheck {
	return &ConfigCheck{
		biggzDir:  "",
		statFn:    os.Stat,
		readDirFn: os.ReadDir,
	}
}

// NewConfigCheckWithCustom creates a ConfigCheck with custom paths for testing.
func NewConfigCheckWithCustom(biggzDir string, statFn func(string) (os.FileInfo, error), readDirFn func(string) ([]os.DirEntry, error)) *ConfigCheck {
	return &ConfigCheck{
		biggzDir:  biggzDir,
		statFn:    statFn,
		readDirFn: readDirFn,
	}
}

// ID returns the check identifier.
func (c *ConfigCheck) ID() CheckID { return ConfigCheckID }

// requiredSubdirs lists the subdirectories expected under ~/.biggz/.
var requiredSubdirs = []string{
	"bigmem",
	"backups",
}

// Run verifies the config directory tree exists.
func (c *ConfigCheck) Run(ctx context.Context) *Result {
	biggzDir := c.biggzDir
	if biggzDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return &Result{
				ID:       ConfigCheckID,
				Status:   StatusFail,
				Message:  "Cannot determine home directory",
				Severity: SeverityCritical,
				Error:    err.Error(),
			}
		}
		biggzDir = filepath.Join(home, ".biggz")
	}

	// Check the root config directory.
	info, err := c.statFn(biggzDir)
	if err != nil {
		if os.IsNotExist(err) {
			return &Result{
				ID:       ConfigCheckID,
				Status:   StatusFail,
				Message:  fmt.Sprintf("Config directory not found at %s", biggzDir),
				Severity: SeverityCritical,
				Error:    err.Error(),
			}
		}
		return &Result{
			ID:       ConfigCheckID,
			Status:   StatusFail,
			Message:  fmt.Sprintf("Cannot stat config directory %s", biggzDir),
			Severity: SeverityCritical,
			Error:    err.Error(),
		}
	}

	if !info.IsDir() {
		return &Result{
			ID:       ConfigCheckID,
			Status:   StatusFail,
			Message:  fmt.Sprintf("Config path %s exists but is not a directory", biggzDir),
			Severity: SeverityCritical,
		}
	}

	// Check each required subdirectory.
	var missing []string
	for _, sub := range requiredSubdirs {
		subPath := filepath.Join(biggzDir, sub)
		subInfo, err := c.statFn(subPath)
		if err != nil || !subInfo.IsDir() {
			missing = append(missing, sub)
		}
	}

	if len(missing) > 0 {
		return &Result{
			ID:       ConfigCheckID,
			Status:   StatusFail,
			Message:  fmt.Sprintf("Missing required subdirectories under %s: %v", biggzDir, missing),
			Severity: SeverityCritical,
			Error:    fmt.Sprintf("missing: %v", missing),
		}
	}

	return &Result{
		ID:       ConfigCheckID,
		Status:   StatusPass,
		Message:  fmt.Sprintf("Config directory structure OK at %s", biggzDir),
		Severity: SeverityInfo,
	}
}

// Remedy returns nil — manual reinstall is required.
func (c *ConfigCheck) Remedy() *Remedy { return nil }
