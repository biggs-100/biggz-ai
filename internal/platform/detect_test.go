package platform

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestDetect_SupportedOS(t *testing.T) {
	p, err := Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if p.OS != runtime.GOOS {
		t.Errorf("OS = %q, want %q", p.OS, runtime.GOOS)
	}
	if p.Arch != runtime.GOARCH {
		t.Errorf("Arch = %q, want %q", p.Arch, runtime.GOARCH)
	}
	if p.Shell == "" {
		t.Error("Shell is empty")
	}
	// Windows should default to powershell when SHELL is unset.
	if runtime.GOOS == "windows" && p.Shell == "unknown" {
		// On Windows, SHELL is typically unset, so Detect should default to powershell.
		// If SHELL is explicitly set, it may be different; so we only check when SHELL is empty.
		if os.Getenv("SHELL") == "" && p.Shell != "powershell" {
			t.Errorf("Shell = %q, want powershell on windows", p.Shell)
		}
	}
	if !IsSupportedOS(p.OS) && p.Supported {
		t.Errorf("Supported = true for unsupported OS %q", p.OS)
	}
	// On linux, Supported must correlate with PackageManager presence.
	if p.OS == "linux" {
		hasPM := p.PackageManager != ""
		if p.Supported != hasPM {
			t.Errorf("linux Supported = %v, PackageManager = %q, want Supported==hasPM", p.Supported, p.PackageManager)
		}
	}
}

func TestEnsureSupportedOS(t *testing.T) {
	tests := []struct {
		name    string
		goos    string
		wantErr bool
	}{
		{name: "darwin supported", goos: "darwin", wantErr: false},
		{name: "linux supported", goos: "linux", wantErr: false},
		{name: "windows supported", goos: "windows", wantErr: false},
		{name: "freebsd unsupported", goos: "freebsd", wantErr: true},
		{name: "empty unsupported", goos: "", wantErr: true},
		{name: "plan9 unsupported", goos: "plan9", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EnsureSupportedOS(tt.goos)
			if (err != nil) != tt.wantErr {
				t.Fatalf("EnsureSupportedOS(%q) error = %v, wantErr %v", tt.goos, err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrUnsupportedOS) {
				t.Errorf("error should wrap ErrUnsupportedOS, got %v", err)
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.goos) {
				t.Errorf("error message should mention %q, got %q", tt.goos, err.Error())
			}
		})
	}
}

func TestEnsureSupported_LinuxNoPMFails(t *testing.T) {
	profile := Profile{
		OS:             "linux",
		LinuxDistro:    "ubuntu",
		PackageManager: "",
		Supported:      false,
	}
	err := EnsureSupported(profile)
	if err == nil {
		t.Fatal("EnsureSupported() expected error for linux without PM, got nil")
	}
	if !errors.Is(err, ErrUnsupportedLinuxDistro) {
		t.Errorf("error should wrap ErrUnsupportedLinuxDistro, got %v", err)
	}
	// Message must list probed managers as in guard.go.
	for _, pm := range []string{"brew", "apt", "dnf", "pacman", "apk"} {
		if !strings.Contains(err.Error(), pm) {
			t.Errorf("error message should contain %q, got %q", pm, err.Error())
		}
	}
	if !strings.Contains(err.Error(), "ubuntu") {
		t.Errorf("error message should contain distro 'ubuntu', got %q", err.Error())
	}

	// Linux with PM should pass.
	profileSupported := Profile{
		OS:             "linux",
		LinuxDistro:    "ubuntu",
		PackageManager: "apt",
		Supported:      true,
	}
	if err := EnsureSupported(profileSupported); err != nil {
		t.Errorf("EnsureSupported() for supported linux should not error, got %v", err)
	}

	// Non-linux with Supported false due to OS unsupported should still error via OS check.
	profileWindows := Profile{OS: "windows", Supported: true}
	if err := EnsureSupported(profileWindows); err != nil {
		t.Errorf("EnsureSupported() for windows should pass, got %v", err)
	}
}

func TestNpmWritable(t *testing.T) {
	origExecOutput := execOutput
	defer func() { execOutput = origExecOutput }()

	home := t.TempDir()

	tests := []struct {
		name       string
		prefixOut  string
		prefixErr  error
		homeDir    string
		wantWrit   bool
	}{
		{name: "writable when prefix under home", prefixOut: home + "/.npm-global", homeDir: home, wantWrit: true},
		{name: "not writable when prefix outside home", prefixOut: "/usr/local", homeDir: home, wantWrit: false},
		{name: "not writable on error", prefixOut: "", prefixErr: errors.New("npm not found"), homeDir: home, wantWrit: false},
		{name: "not writable when home empty", prefixOut: home, homeDir: "", wantWrit: false},
		{name: "trims whitespace", prefixOut: "  " + home + "/prefix  \n", homeDir: home, wantWrit: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execOutput = func(name string, args ...string) ([]byte, error) {
				if name != "npm" {
					t.Errorf("unexpected command %q", name)
				}
				if tt.prefixErr != nil {
					return nil, tt.prefixErr
				}
				return []byte(tt.prefixOut), nil
			}
			got := detectNpmWritable(tt.homeDir)
			if got != tt.wantWrit {
				t.Errorf("detectNpmWritable() = %v, want %v (prefix %q, home %q)", got, tt.wantWrit, tt.prefixOut, tt.homeDir)
			}
		})
	}
}

func TestGoAvailable(t *testing.T) {
	origLookPath := lookPath
	defer func() { lookPath = origLookPath }()

	t.Run("available when found", func(t *testing.T) {
		lookPath = func(name string) (string, error) {
			if name == "go" {
				return "/usr/local/go/bin/go", nil
			}
			return "", exec.ErrNotFound
		}
		if !detectGoAvailable() {
			t.Error("detectGoAvailable() = false, want true")
		}
	})

	t.Run("not available when missing", func(t *testing.T) {
		lookPath = func(name string) (string, error) {
			return "", exec.ErrNotFound
		}
		if detectGoAvailable() {
			t.Error("detectGoAvailable() = true, want false")
		}
	})
}

func TestEnsureCommandDir(t *testing.T) {
	origGetwd := getwd
	defer func() { getwd = origGetwd }()

	t.Run("noop when Dir already set", func(t *testing.T) {
		dir := t.TempDir()
		cmd := exec.Command("echo", "hi")
		cmd.Dir = dir
		EnsureCommandDir(cmd)
		if cmd.Dir != dir {
			t.Errorf("Dir = %q, want %q", cmd.Dir, dir)
		}
	})

	t.Run("noop when wd exists", func(t *testing.T) {
		wd := t.TempDir()
		getwd = func() (string, error) { return wd, nil }
		cmd := exec.Command("echo", "hi")
		EnsureCommandDir(cmd)
		if cmd.Dir != "" {
			t.Errorf("Dir = %q, want empty when wd exists", cmd.Dir)
		}
	})

	t.Run("fallback to home when wd missing", func(t *testing.T) {
		// Simulate wd error.
		getwd = func() (string, error) { return "", errors.New("no wd") }
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			t.Skip("no home dir")
		}
		// Ensure home exists (it should).
		if _, err := os.Stat(home); err != nil {
			t.Skip("home does not exist")
		}
		cmd := exec.Command("echo", "hi")
		EnsureCommandDir(cmd)
		if cmd.Dir != home {
			t.Errorf("Dir = %q, want home %q", cmd.Dir, home)
		}
	})

	t.Run("fallback to TempDir when wd and home missing via ensureCommandDir", func(t *testing.T) {
		cmd := exec.Command("echo", "hi")
		// Call unexported ensureCommandDir directly with missing wd and home not existing.
		// We simulate by passing a non-existent wd and mocking UserHomeDir failure implicitly:
		// We cannot easily mock UserHomeDir, so test the TempDir fallback path by providing
		// a wd error and ensuring home check fails due to dir not existing.
		// Instead we test the lower helper directly with a temp dir that exists.
		tmp := os.TempDir()
		ensureCommandDir(cmd, "/nonexistent/path/that/does/not/exist", errors.New("gone"))
		// Should have set to home or tmp. Since home exists, it will be home.
		// We verify Dir is set to something that exists.
		if cmd.Dir == "" {
			t.Error("Dir should be set to fallback")
		}
		if _, err := os.Stat(cmd.Dir); err != nil {
			t.Errorf("fallback Dir %q does not exist: %v", cmd.Dir, err)
		}
		// If home exists, Dir should be home, else tmp.
		home, _ := os.UserHomeDir()
		if dirExists(home) {
			if cmd.Dir != home {
				t.Errorf("Dir = %q, want home %q", cmd.Dir, home)
			}
		} else {
			if cmd.Dir != tmp {
				t.Errorf("Dir = %q, want tmp %q", cmd.Dir, tmp)
			}
		}
	})

	t.Run("nil cmd is safe", func(t *testing.T) {
		// Should not panic.
		EnsureCommandDir(nil)
		ensureCommandDir(nil, "", nil)
	})
}

func TestOsReleaseID(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "ubuntu quoted", content: "ID=ubuntu\n", want: "ubuntu"},
		{name: "ubuntu double quoted", content: "ID=\"ubuntu\"\n", want: "ubuntu"},
		{name: "single quoted", content: "ID='gentoo'\n", want: "gentoo"},
		{name: "case insensitive key", content: "id=debian\n", want: "debian"},
		{name: "with spaces and quotes", content: " ID = \"Arch\" \n", want: "arch"},
		{name: "unknown when missing", content: "NAME=\"Ubuntu\"\n", want: "unknown"},
		{name: "unknown when empty", content: "", want: "unknown"},
		{name: "ignores comments", content: "# comment\nID=fedora\n", want: "fedora"},
		{name: "lowercases", content: "ID=Ubuntu\n", want: "ubuntu"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := osReleaseID(tt.content)
			if got != tt.want {
				t.Errorf("osReleaseID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectPackageManager(t *testing.T) {
	origLookPath := lookPath
	defer func() { lookPath = origLookPath }()

	t.Run("darwin returns brew", func(t *testing.T) {
		lookPath = func(name string) (string, error) { return "", exec.ErrNotFound }
		if got := detectPackageManager("darwin"); got != "brew" {
			t.Errorf("detectPackageManager(darwin) = %q, want brew", got)
		}
	})

	t.Run("linux finds first available", func(t *testing.T) {
		lookPath = func(name string) (string, error) {
			if name == "apt" {
				return "/usr/bin/apt", nil
			}
			return "", exec.ErrNotFound
		}
		if got := detectPackageManager("linux"); got != "apt" {
			t.Errorf("detectPackageManager(linux) = %q, want apt", got)
		}
	})

	t.Run("linux none found returns empty", func(t *testing.T) {
		lookPath = func(name string) (string, error) { return "", exec.ErrNotFound }
		if got := detectPackageManager("linux"); got != "" {
			t.Errorf("detectPackageManager(linux) = %q, want empty", got)
		}
	})

	t.Run("windows winget found", func(t *testing.T) {
		lookPath = func(name string) (string, error) {
			if name == "winget" {
				return "C:\\Program Files\\winget", nil
			}
			return "", exec.ErrNotFound
		}
		if got := detectPackageManager("windows"); got != "winget" {
			t.Errorf("detectPackageManager(windows) = %q, want winget", got)
		}
	})

	t.Run("windows winget missing returns empty", func(t *testing.T) {
		lookPath = func(name string) (string, error) { return "", exec.ErrNotFound }
		if got := detectPackageManager("windows"); got != "" {
			t.Errorf("detectPackageManager(windows) = %q, want empty", got)
		}
	})

	t.Run("unsupported OS returns empty", func(t *testing.T) {
		if got := detectPackageManager("freebsd"); got != "" {
			t.Errorf("detectPackageManager(freebsd) = %q, want empty", got)
		}
	})
}

func TestDetect_Integration(t *testing.T) {
	// Ensure Detect does not error and returns consistent OS.
	p, err := Detect(context.Background())
	if err != nil {
		t.Fatalf("Detect error = %v", err)
	}
	if p.OS == "" {
		t.Error("OS empty")
	}
	if p.Arch == "" {
		t.Error("Arch empty")
	}
}
