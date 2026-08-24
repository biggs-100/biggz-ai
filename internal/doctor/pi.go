package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/platform"
)

const (
	// PiSubagentsCheckID is the check identifier for Pi subagent dispatcher.
	PiSubagentsCheckID CheckID = "pi-subagents"
)

// PiSubagentsCheck verifies that pi-subagents (nicobailon/pi-subagents) is
// installed when pi is present. Without it pi has only read/bash/edit/write
// and cannot delegate to subagents (scout, researcher, worker, reviewer, etc.).
//
// It checks in order:
//  1. `npm list -g pi-subagents` succeeds
//  2. `pi-subagents` binary is on PATH (where/which)
//  3. `~/.pi/agent/node_modules/pi-subagents` exists
//  4. `~/.pi/node_modules/pi-subagents` exists (fallback)
//
// If pi itself is not installed, the check is informational (pass) — pi-subagents
// is only relevant when pi is in use.
type PiSubagentsCheck struct {
	lookPath  func(string) (string, error)
	execFn    func(string, ...string) ([]byte, error)
	statFn    func(string) (os.FileInfo, error)
	homeDirFn func() (string, error)
}

// NewPiSubagentsCheck creates a PiSubagentsCheck using the default environment.
func NewPiSubagentsCheck() *PiSubagentsCheck {
	return &PiSubagentsCheck{
		lookPath:  exec.LookPath,
		execFn:    execCommand,
		statFn:    os.Stat,
		homeDirFn: os.UserHomeDir,
	}
}

// NewPiSubagentsCheckWithCustom creates a PiSubagentsCheck with injected functions for testing.
func NewPiSubagentsCheckWithCustom(
	lookPath func(string) (string, error),
	execFn func(string, ...string) ([]byte, error),
	statFn func(string) (os.FileInfo, error),
	homeDirFn func() (string, error),
) *PiSubagentsCheck {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if execFn == nil {
		execFn = execCommand
	}
	if statFn == nil {
		statFn = os.Stat
	}
	if homeDirFn == nil {
		homeDirFn = os.UserHomeDir
	}
	return &PiSubagentsCheck{
		lookPath:  lookPath,
		execFn:    execFn,
		statFn:    statFn,
		homeDirFn: homeDirFn,
	}
}

// ID returns the check identifier.
func (c *PiSubagentsCheck) ID() CheckID { return PiSubagentsCheckID }

// Run verifies pi-subagents installation.
func (c *PiSubagentsCheck) Run(ctx context.Context) *Result {
	// If pi itself is not installed, skip — not relevant.
	if _, err := c.lookPath("pi"); err != nil {
		return &Result{
			ID:       PiSubagentsCheckID,
			Status:   StatusPass,
			Message:  "pi not installed — skipping pi-subagents check",
			Severity: SeverityInfo,
		}
	}

	// 1. pi's loader path is authoritative: ~/.pi/agent/npm/node_modules/pi-subagents
	// (pnpm symlinks via `pi install`), not the global npm prefix. Check this first — `pi install`
	// writes to npm/node_modules, and gentle-pi's FleetView capability probe uses package.json
	// presence, not `npm list -g`.
	home, err := c.homeDirFn()
	if err == nil && home != "" {
		candidates := []string{
			filepath.Join(home, ".pi", "agent", "npm", "node_modules", "pi-subagents"),
			filepath.Join(home, ".pi", "agent", "node_modules", "pi-subagents"),
			filepath.Join(home, ".pi", "node_modules", "pi-subagents"),
		}
		// Also respect PI_CODING_AGENT_DIR if set (mirrors pi adapter ConfigPath)
		if v := strings.TrimSpace(os.Getenv("PI_CODING_AGENT_DIR")); v != "" {
			candidates = append(candidates, filepath.Join(v, "npm", "node_modules", "pi-subagents"), filepath.Join(v, "node_modules", "pi-subagents"))
		}
		for _, cand := range candidates {
			if info, err := c.statFn(cand); err == nil && info.IsDir() {
				return &Result{
					ID:       PiSubagentsCheckID,
					Status:   StatusPass,
					Message:  fmt.Sprintf("pi-subagents found at %s", cand),
					Severity: SeverityInfo,
				}
			}
		}
	}

	// 2. npm list -g pi-subagents (legacy global check — not where pi loads from,
	// but kept as fallback for older installs).
	if out, err := c.execFn("npm", "list", "-g", "pi-subagents"); err == nil {
		// npm list exits 0 when found; output contains package name
		if strings.Contains(string(out), "pi-subagents") {
			return &Result{
				ID:       PiSubagentsCheckID,
				Status:   StatusPass,
				Message:  "pi-subagents installed (npm list -g)",
				Severity: SeverityInfo,
			}
		}
		// Even if output doesn't contain name but exit 0, treat as installed
		return &Result{
			ID:       PiSubagentsCheckID,
			Status:   StatusPass,
			Message:  "pi-subagents installed (npm list -g)",
			Severity: SeverityInfo,
		}
	}

	// 3. pi-subagents binary on PATH (some installs expose a binary)
	if _, err := c.lookPath("pi-subagents"); err == nil {
		return &Result{
			ID:       PiSubagentsCheckID,
			Status:   StatusPass,
			Message:  "pi-subagents binary found on PATH",
			Severity: SeverityInfo,
		}
	}

	return &Result{
		ID:       PiSubagentsCheckID,
		Status:   StatusWarn,
		Message:  "pi-subagents not installed — pi subagent dispatcher missing (run: pi install npm:pi-subagents)",
		Severity: SeverityWarning,
		Error:    "pi-subagents not found via npm list -g, PATH, or ~/.pi/agent/npm/node_modules/pi-subagents",
	}
}

// Remedy returns a repair action that installs pi-subagents via pi.
func (c *PiSubagentsCheck) Remedy() *Remedy {
	return &Remedy{
		ID:          string(PiSubagentsCheckID),
		Description: "Install pi-subagents dispatcher (pi install npm:pi-subagents)",
		Action: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			cmd := exec.CommandContext(ctx, "pi", "install", "npm:pi-subagents")
			platform.EnsureCommandDir(cmd)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("pi install npm:pi-subagents: %w (output: %s)", err, strings.TrimSpace(string(out)))
			}
			return nil
		},
	}
}
