package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/biggs-100/biggz-ai/internal/platform"
	"github.com/biggs-100/biggz-ai/internal/skillregistry"
)

const PlatformCheckID CheckID = "platform"

// PlatformCheck validates that the current OS and Linux distribution are supported.
type PlatformCheck struct {
	getwdFn   func() (string, error)
	homeDirFn func() (string, error)
	refreshFn func(string, bool) (*skillregistry.Result, error)
}

// NewPlatformCheck creates a PlatformCheck.
func NewPlatformCheck() *PlatformCheck {
	return &PlatformCheck{
		getwdFn:   os.Getwd,
		homeDirFn: os.UserHomeDir,
		refreshFn: skillregistry.Refresh,
	}
}

// NewPlatformCheckWithCustom creates a PlatformCheck with injected functions for testing.
func NewPlatformCheckWithCustom(
	getwdFn func() (string, error),
	homeDirFn func() (string, error),
	refreshFn func(string, bool) (*skillregistry.Result, error),
) *PlatformCheck {
	if getwdFn == nil {
		getwdFn = os.Getwd
	}
	if homeDirFn == nil {
		homeDirFn = os.UserHomeDir
	}
	if refreshFn == nil {
		refreshFn = skillregistry.Refresh
	}
	return &PlatformCheck{
		getwdFn:   getwdFn,
		homeDirFn: homeDirFn,
		refreshFn: refreshFn,
	}
}

// ID returns the check identifier.
func (c *PlatformCheck) ID() CheckID { return PlatformCheckID }

// Run detects the platform and checks if it is supported.
func (c *PlatformCheck) Run(ctx context.Context) *Result {
	profile, err := platform.Detect(ctx)
	if err != nil {
		return &Result{
			ID:       PlatformCheckID,
			Status:   StatusFail,
			Message:  "Platform detection failed",
			Severity: SeverityCritical,
			Error:    err.Error(),
		}
	}
	if err := platform.EnsureSupported(profile); err != nil {
		return &Result{
			ID:       PlatformCheckID,
			Status:   StatusFail,
			Message:  err.Error(),
			Severity: SeverityCritical,
			Error:    err.Error(),
		}
	}
	return &Result{
		ID:       PlatformCheckID,
		Status:   StatusPass,
		Message:  fmt.Sprintf("Platform supported: %s/%s shell=%s pm=%s", profile.OS, profile.Arch, profile.Shell, profile.PackageManager),
		Severity: SeverityInfo,
	}
}

// Remedy returns a repair action that refreshes the skill-registry cache
// and ensures the opencode plugin directory exists. Safe and idempotent.
func (c *PlatformCheck) Remedy() *Remedy {
	return &Remedy{
		ID:          string(PlatformCheckID),
		Description: "Refresh skill-registry cache",
		Action: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			// Ensure the plugin/config directory exists.
			homeFn := c.homeDirFn
			if homeFn == nil {
				homeFn = os.UserHomeDir
			}
			if home, err := homeFn(); err == nil && home != "" {
				pluginDir := filepath.Join(home, ".config", "opencode", "plugins")
				_ = os.MkdirAll(pluginDir, 0755)
				_ = os.MkdirAll(filepath.Join(home, ".config", "opencode"), 0755)
				// Demonstrate EnsureCommandDir usage: ensure a dummy command has a valid dir.
				dummy := &exec.Cmd{}
				platform.EnsureCommandDir(dummy)
			}
			getwd := c.getwdFn
			if getwd == nil {
				getwd = os.Getwd
			}
			cwd, err := getwd()
			if err != nil || cwd == "" {
				cwd = "."
			}
			refresh := c.refreshFn
			if refresh == nil {
				refresh = skillregistry.Refresh
			}
			// Refresh the registry; idempotent — second run is a cache hit.
			if _, err := refresh(cwd, false); err != nil {
				// Try forced refresh as fallback.
				if _, ferr := refresh(cwd, true); ferr != nil {
					return fmt.Errorf("skill-registry refresh: %w", err)
				}
			}
			return nil
		},
	}
}
