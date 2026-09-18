package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/platform"
	"github.com/biggs-100/biggz-ai/internal/sdd"
)

const (
	// PiSubagentsCheckID is the check identifier for the Pi delegation runtime.
	PiSubagentsCheckID CheckID = "pi-subagents"
	// PiLastModelCheckID reports whether last-model sync is active.
	PiLastModelCheckID CheckID = "pi-last-model"
)

// PiSubagentsCheck verifies that the biggz subagent-runtime extension
// (internal/assets/pi/biggz-subagent-runtime.js, deployed by
// `biggz install --agent pi`) is present for pi. The runtime owns delegation
// after the j0k3r cutover: without its marker pi has only read/bash/edit/write
// and cannot delegate to subagents.
//
// The marker path is owned by internal/sdd (SubagentRuntimeMarkerPath) and
// honors PI_CODING_AGENT_DIR. If pi itself is not installed, the check is
// informational (pass) — the runtime is only relevant when pi is in use.
type PiSubagentsCheck struct {
	lookPath  func(string) (string, error)
	statFn    func(string) (os.FileInfo, error)
	homeDirFn func() (string, error)
}

// NewPiSubagentsCheck creates a PiSubagentsCheck using the default environment.
func NewPiSubagentsCheck() *PiSubagentsCheck {
	return &PiSubagentsCheck{
		lookPath:  exec.LookPath,
		statFn:    os.Stat,
		homeDirFn: os.UserHomeDir,
	}
}

// NewPiSubagentsCheckWithCustom creates a PiSubagentsCheck with injected functions for testing.
func NewPiSubagentsCheckWithCustom(
	lookPath func(string) (string, error),
	statFn func(string) (os.FileInfo, error),
	homeDirFn func() (string, error),
) *PiSubagentsCheck {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if statFn == nil {
		statFn = os.Stat
	}
	if homeDirFn == nil {
		homeDirFn = os.UserHomeDir
	}
	return &PiSubagentsCheck{
		lookPath:  lookPath,
		statFn:    statFn,
		homeDirFn: homeDirFn,
	}
}

// ID returns the check identifier.
func (c *PiSubagentsCheck) ID() CheckID { return PiSubagentsCheckID }

// Run verifies the deployed subagent-runtime marker.
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

	home, err := c.homeDirFn()
	override := strings.TrimSpace(os.Getenv("PI_CODING_AGENT_DIR")) != ""
	if (err != nil || home == "") && !override {
		return &Result{
			ID:       PiSubagentsCheckID,
			Status:   StatusWarn,
			Message:  "cannot determine home directory for pi subagent runtime check",
			Severity: SeverityWarning,
		}
	}
	marker := sdd.SubagentRuntimeMarkerPath(home)
	if info, statErr := c.statFn(marker); statErr == nil && !info.IsDir() {
		return &Result{
			ID:       PiSubagentsCheckID,
			Status:   StatusPass,
			Message:  fmt.Sprintf("subagent runtime deployed (%s)", marker),
			Severity: SeverityInfo,
		}
	}
	return &Result{
		ID:       PiSubagentsCheckID,
		Status:   StatusWarn,
		Message:  "subagent runtime not deployed — pi delegation unavailable (run: biggz install --agent pi)",
		Severity: SeverityWarning,
		Error:    fmt.Sprintf("subagent runtime marker %s not found", marker),
	}
}

// Remedy returns a repair action that redeploys the runtime through the
// standard pi install flow (`biggz install --agent pi`).
func (c *PiSubagentsCheck) Remedy() *Remedy {
	return &Remedy{
		ID:          string(PiSubagentsCheckID),
		Description: "Redeploy the pi subagent runtime (biggz install --agent pi)",
		Action:      biggzInstallPi,
	}
}

// biggzInstallPi re-runs the pi install flow, redeploying every biggz-managed
// pi asset (subagent runtime, last-model extension, guards, ...).
func biggzInstallPi(ctx context.Context) error {
	// Use biggz binary via PATH or current executable directory.
	biggzBin := "biggz"
	if exe, err := os.Executable(); err == nil {
		cand := filepath.Join(filepath.Dir(exe), "biggz.exe")
		if _, err := os.Stat(cand); err == nil {
			biggzBin = cand
		} else {
			cand2 := filepath.Join(filepath.Dir(exe), "biggz")
			if _, err := os.Stat(cand2); err == nil {
				biggzBin = cand2
			}
		}
	}
	cmd := exec.CommandContext(ctx, biggzBin, "install", "--agent", "pi")
	platform.EnsureCommandDir(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("biggz install --agent pi: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// PiLastModelCheck verifies that the last-model sync extension is active.
type PiLastModelCheck struct {
	lookPath  func(string) (string, error)
	statFn    func(string) (os.FileInfo, error)
	homeDirFn func() (string, error)
}

// NewPiLastModelCheck creates a PiLastModelCheck using the default environment.
func NewPiLastModelCheck() *PiLastModelCheck {
	return &PiLastModelCheck{
		lookPath:  exec.LookPath,
		statFn:    os.Stat,
		homeDirFn: os.UserHomeDir,
	}
}

// NewPiLastModelCheckWithCustom creates a PiLastModelCheck with injected functions for testing.
func NewPiLastModelCheckWithCustom(
	lookPath func(string) (string, error),
	statFn func(string) (os.FileInfo, error),
	homeDirFn func() (string, error),
) *PiLastModelCheck {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if statFn == nil {
		statFn = os.Stat
	}
	if homeDirFn == nil {
		homeDirFn = os.UserHomeDir
	}
	return &PiLastModelCheck{
		lookPath:  lookPath,
		statFn:    statFn,
		homeDirFn: homeDirFn,
	}
}

// ID returns the check identifier.
func (c *PiLastModelCheck) ID() CheckID { return PiLastModelCheckID }

// Run checks whether the biggz-last-model extension is installed.
func (c *PiLastModelCheck) Run(ctx context.Context) *Result {
	// If pi itself is not installed, skip — not relevant.
	if _, err := c.lookPath("pi"); err != nil {
		return &Result{
			ID:       PiLastModelCheckID,
			Status:   StatusPass,
			Message:  "pi not installed — skipping pi-last-model check",
			Severity: SeverityInfo,
		}
	}
	home, err := c.homeDirFn()
	if err != nil || home == "" {
		return &Result{
			ID:       PiLastModelCheckID,
			Status:   StatusWarn,
			Message:  "cannot determine home directory for pi-last-model check",
			Severity: SeverityWarning,
		}
	}
	candidates := []string{
		filepath.Join(home, ".pi", "agent", "extensions", "biggz-last-model.js"),
	}
	if v := strings.TrimSpace(os.Getenv("PI_CODING_AGENT_DIR")); v != "" {
		candidates = append(candidates, filepath.Join(v, "extensions", "biggz-last-model.js"))
	}
	for _, cand := range candidates {
		if info, err := c.statFn(cand); err == nil && !info.IsDir() {
			return &Result{
				ID:       PiLastModelCheckID,
				Status:   StatusPass,
				Message:  fmt.Sprintf("pi last-model sync active (%s)", cand),
				Severity: SeverityInfo,
			}
		}
	}
	return &Result{
		ID:       PiLastModelCheckID,
		Status:   StatusWarn,
		Message:  "pi last-model extension not found — new sessions will start with defaultModel (run: biggz install --agent pi)",
		Severity: SeverityWarning,
		Error:    "biggz-last-model.js not found in ~/.pi/agent/extensions",
	}
}

// Remedy returns a repair action that reinstalls the last-model extension.
func (c *PiLastModelCheck) Remedy() *Remedy {
	return &Remedy{
		ID:          string(PiLastModelCheckID),
		Description: "Install pi last-model extension (biggz install --agent pi)",
		Action:      biggzInstallPi,
	}
}

// Ensure PiMCPAdapterCheck implements Check (registered in doctorRun via NewPiMCPAdapterCheck).
var _ Check = (*PiMCPAdapterCheck)(nil)
