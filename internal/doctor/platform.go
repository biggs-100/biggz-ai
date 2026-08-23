package doctor

import (
	"context"
	"fmt"

	"github.com/biggs-100/biggz-ai/internal/platform"
)

const PlatformCheckID CheckID = "platform"

// PlatformCheck validates that the current OS and Linux distribution are supported.
type PlatformCheck struct{}

// NewPlatformCheck creates a PlatformCheck.
func NewPlatformCheck() *PlatformCheck { return &PlatformCheck{} }

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

// Remedy returns nil — platform support requires manual setup.
func (c *PlatformCheck) Remedy() *Remedy { return nil }
