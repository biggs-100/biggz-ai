package platform

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Profile holds minimal platform detection results.
type Profile struct {
	OS             string
	Arch           string
	Shell          string
	LinuxDistro    string
	PackageManager string
	NpmWritable    bool
	GoAvailable    bool
	Supported      bool
}

// injectable dependencies for testing.
var (
	lookPath    = exec.LookPath
	osReadFile  = os.ReadFile
	execOutput  = func(name string, args ...string) ([]byte, error) { return exec.Command(name, args...).Output() }
	userHomeDir = os.UserHomeDir
	linuxPMs    = []string{"brew", "apt", "dnf", "pacman", "apk"}
)

// Detect gathers platform information.
// It reads runtime.GOOS/GOARCH, SHELL, Linux distro, package manager,
// npm writable, and Go availability.
func Detect(ctx context.Context) (Profile, error) {
	_ = ctx

	goos := runtime.GOOS
	arch := runtime.GOARCH

	shell := os.Getenv("SHELL")
	if shell == "" {
		if goos == "windows" {
			shell = "powershell"
		} else {
			shell = "unknown"
		}
	}

	var linuxDistro string
	if goos == "linux" {
		data, _ := osReadFile("/etc/os-release")
		linuxDistro = osReleaseID(string(data))
	}

	pm := detectPackageManager(goos)

	home, _ := userHomeDir()
	npmWritable := detectNpmWritable(home)

	goAvailable := detectGoAvailable()

	supported := IsSupportedOS(goos)
	if supported && goos == "linux" {
		supported = pm != ""
	}

	return Profile{
		OS:             goos,
		Arch:           arch,
		Shell:          shell,
		LinuxDistro:    linuxDistro,
		PackageManager: pm,
		NpmWritable:    npmWritable,
		GoAvailable:    goAvailable,
		Supported:      supported,
	}, nil
}

// detectPackageManager probes for a package manager in order.
// On darwin it returns brew without probing, on windows it probes winget,
// on linux it probes brew,apt,dnf,pacman,apk.
func detectPackageManager(goos string) string {
	switch goos {
	case "darwin":
		return "brew"
	case "linux":
		for _, m := range linuxPMs {
			if _, err := lookPath(m); err == nil {
				return m
			}
		}
		return ""
	case "windows":
		if _, err := lookPath("winget"); err == nil {
			return "winget"
		}
		return ""
	default:
		return ""
	}
}

// detectNpmWritable checks if npm global prefix is under homeDir.
func detectNpmWritable(homeDir string) bool {
	if homeDir == "" {
		return false
	}
	out, err := execOutput("npm", "config", "get", "prefix")
	if err != nil {
		return false
	}
	prefix := strings.TrimSpace(string(out))
	return strings.HasPrefix(prefix, homeDir)
}

// detectGoAvailable checks if go is on PATH.
func detectGoAvailable() bool {
	_, err := lookPath("go")
	return err == nil
}

// osReleaseID parses /etc/os-release content and returns the lower-cased ID field.
// It handles quoted values and is case-insensitive for the key.
func osReleaseID(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found || strings.ToUpper(strings.TrimSpace(key)) != "ID" {
			continue
		}
		id := strings.ToLower(strings.Trim(strings.TrimSpace(value), `"'`))
		if id == "" {
			continue
		}
		return id
	}
	return "unknown"
}
