package platform

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrUnsupportedOS          = errors.New("unsupported operating system")
	ErrUnsupportedLinuxDistro = errors.New("unsupported linux distro")
)

// IsSupportedOS reports whether goos is darwin, linux, or windows.
func IsSupportedOS(goos string) bool {
	return goos == "darwin" || goos == "linux" || goos == "windows"
}

// EnsureSupportedOS validates that goos is a supported operating system.
func EnsureSupportedOS(goos string) error {
	if IsSupportedOS(goos) {
		return nil
	}
	return fmt.Errorf("%w: only macOS, Linux, and Windows are supported (detected %s)", ErrUnsupportedOS, goos)
}

// EnsureSupported validates that the detected platform is supported.
// On Linux, Supported must be true (a package manager was found).
func EnsureSupported(p Profile) error {
	if err := EnsureSupportedOS(p.OS); err != nil {
		return err
	}
	if p.OS == "linux" && !p.Supported {
		distro := strings.TrimSpace(p.LinuxDistro)
		if distro == "" {
			distro = "unknown"
		}
		return fmt.Errorf(
			"%w: no package manager found on PATH (detected distro %s).\n"+
				"biggz-ai looks for these, in order: %s\n"+
				"See which ones this machine already has:\n"+
				"  for m in %s; do command -v \"$m\"; done\n"+
				"Install one of them, or add the directory holding it to PATH, then run biggz-ai again.",
			ErrUnsupportedLinuxDistro,
			distro,
			strings.Join(linuxPMs, ", "),
			strings.Join(linuxPMs, " "),
		)
	}
	return nil
}
