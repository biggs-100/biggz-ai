package doctor

import (
	"context"
	"fmt"
	"strings"
)

const (
	// VersionCheckID is the check identifier for version comparison.
	VersionCheckID CheckID = "version"
)

// VersionCheck compares the embedded build version (set via ldflags)
// against the latest git tag. A mismatch produces an INFO result with
// both versions displayed.
//
// The embedded version is injected at build time via -ldflags:
//
//	go build -ldflags="-X github.com/biggs-100/biggz-ai/internal/doctor.BuildVersion=v1.0.0"
//
// If BuildVersion is empty, the check uses "dev" as the installed version.
var BuildVersion string

// VersionCheck compares the installed version against the latest git tag.
type VersionCheck struct {
	execFn func(string, ...string) ([]byte, error)
}

// NewVersionCheck creates a VersionCheck using the default environment.
func NewVersionCheck() *VersionCheck {
	return &VersionCheck{
		execFn: execCommand,
	}
}

// NewVersionCheckWithCustom creates a VersionCheck with injected functions for testing.
func NewVersionCheckWithCustom(execFn func(string, ...string) ([]byte, error)) *VersionCheck {
	return &VersionCheck{
		execFn: execFn,
	}
}

// ID returns the check identifier.
func (c *VersionCheck) ID() CheckID { return VersionCheckID }

// Run compares the installed version against the latest git tag.
func (c *VersionCheck) Run(ctx context.Context) *Result {
	installed := BuildVersion
	if installed == "" {
		installed = "dev"
	}

	// Get the latest git tag.
	latest, err := c.getLatestTag()
	if err != nil {
		// If we can't get the latest tag, report what we have as INFO.
		return &Result{
			ID:       VersionCheckID,
			Status:   StatusPass,
			Message:  fmt.Sprintf("Installed version: %s (could not determine latest tag)", installed),
			Severity: SeverityInfo,
			Error:    err.Error(),
		}
	}

	if installed == "dev" || installed != latest {
		return &Result{
			ID:       VersionCheckID,
			Status:   StatusPass,
			Message:  fmt.Sprintf("Installed: %s, Latest: %s", installed, latest),
			Severity: SeverityInfo,
		}
	}

	return &Result{
		ID:       VersionCheckID,
		Status:   StatusPass,
		Message:  fmt.Sprintf("Up to date: %s", installed),
		Severity: SeverityInfo,
	}
}

// getLatestTag returns the latest git tag using `git describe --tags --abbrev=0`.
func (c *VersionCheck) getLatestTag() (string, error) {
	out, err := c.execFn("git", "describe", "--tags", "--abbrev=0")
	if err != nil {
		return "", fmt.Errorf("git describe: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Remedy returns nil — version updates are manual.
func (c *VersionCheck) Remedy() *Remedy { return nil }
