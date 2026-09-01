package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// PathCheckID is the check identifier for PATH shadowing.
	PathCheckID CheckID = "path"
	// PathSanityCheckID is the check identifier for PATH length and pollution.
	PathSanityCheckID CheckID = "path_sanity"
)

// PathCheck scans the PATH environment variable for duplicate biggz or
// biggz-mcp binaries. If the same binary name appears in multiple PATH
// directories, the first one wins — duplicates are a WARNING.
type PathCheck struct {
	getenvFn func(string) string
	statFn   func(string) (os.FileInfo, error)
}

// NewPathCheck creates a PathCheck using the default environment.
func NewPathCheck() *PathCheck {
	return &PathCheck{
		getenvFn: os.Getenv,
		statFn:   os.Stat,
	}
}

// NewPathCheckWithCustom creates a PathCheck with injected functions for testing.
func NewPathCheckWithCustom(getenvFn func(string) string, statFn func(string) (os.FileInfo, error)) *PathCheck {
	return &PathCheck{
		getenvFn: getenvFn,
		statFn:   statFn,
	}
}

// ID returns the check identifier.
func (c *PathCheck) ID() CheckID { return PathCheckID }

// targetNames returns the binary names to search for in PATH.
func (c *PathCheck) targetNames() []string {
	names := []string{"biggz", "biggz-mcp"}
	if runtime.GOOS == "windows" {
		for i, n := range names {
			names[i] = n + ".exe"
		}
	}
	return names
}

// findPollutedEntries returns PATH entries that look like Temp\TestInstall or Temp\biggz-check pollution.
func findPollutedEntries(path string) []string {
	if path == "" {
		return nil
	}
	var polluted []string
	for _, d := range filepath.SplitList(path) {
		trimmed := strings.TrimSpace(d)
		if trimmed == "" {
			continue
		}
		low := strings.ToLower(trimmed)
		if strings.Contains(low, "temp") && (strings.Contains(low, "testinstall") || strings.Contains(low, "biggz-check")) {
			polluted = append(polluted, trimmed)
		}
	}
	return polluted
}

// Run scans PATH for duplicate binaries and also warns on length/pollution for Git Bash robustness.
func (c *PathCheck) Run(ctx context.Context) *Result {
	path := c.getenvFn("PATH")
	if path == "" {
		return &Result{
			ID:       PathCheckID,
			Status:   StatusWarn,
			Message:  "PATH environment variable is empty",
			Severity: SeverityWarning,
		}
	}

	// PATH length check — Windows has ~2047/4095 char limits per variable and ~8191 for cmd; 8000 is a safe threshold for Git Bash.
	if len(path) > 8000 {
		return &Result{
			ID:       PathCheckID,
			Status:   StatusWarn,
			Message:  fmt.Sprintf("PATH length %d exceeds 8000 characters, may be truncated on Windows (Git Bash)", len(path)),
			Severity: SeverityWarning,
			Error:    fmt.Sprintf("PATH length %d > 8000", len(path)),
		}
	}
	if polluted := findPollutedEntries(path); len(polluted) > 0 {
		return &Result{
			ID:       PathCheckID,
			Status:   StatusWarn,
			Message:  fmt.Sprintf("PATH contains polluted Temp entries: %s", strings.Join(polluted, ", ")),
			Severity: SeverityWarning,
			Error:    strings.Join(polluted, ", "),
		}
	}

	// Split PATH into directories using filepath.SplitList which handles
	// both Unix (:) and Windows (;) separators automatically.
	// Deduplicate directories so the same dir appearing twice in PATH
	// (common when shell rc appends) does not count as duplicate binaries.
	rawDirs := filepath.SplitList(path)
	seenDir := make(map[string]bool)
	var dirs []string
	for _, d := range rawDirs {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		key := strings.ToLower(d)
		if seenDir[key] {
			continue
		}
		seenDir[key] = true
		dirs = append(dirs, d)
	}

	targets := c.targetNames()
	type location struct {
		dir  string
		name string
	}
	found := make(map[string][]location) // binary name → locations

	for _, dir := range dirs {
		for _, name := range targets {
			fullPath := filepath.Join(dir, name)
			_, err := c.statFn(fullPath)
			if err == nil {
				found[name] = append(found[name], location{dir: dir, name: name})
			}
		}
	}

	// Check for duplicates.
	var duplicates []string
	var missing []string

	for _, name := range targets {
		locs, ok := found[name]
		if !ok {
			missing = append(missing, name)
			continue
		}
		if len(locs) > 1 {
			paths := make([]string, len(locs))
			for i, loc := range locs {
				paths[i] = filepath.Join(loc.dir, loc.name)
			}
			duplicates = append(duplicates, fmt.Sprintf("%s found in %d locations: %s", name, len(locs), strings.Join(paths, ", ")))
		}
	}

	if len(duplicates) > 0 {
		return &Result{
			ID:       PathCheckID,
			Status:   StatusWarn,
			Message:  fmt.Sprintf("Duplicate binaries in PATH: %s", strings.Join(duplicates, "; ")),
			Severity: SeverityWarning,
			Error:    strings.Join(duplicates, "; "),
		}
	}

	// All clear — but note if some binaries aren't on PATH at all.
	msg := "PATH check OK"
	if len(missing) > 0 {
		msg = fmt.Sprintf("PATH check OK (%s not on PATH)", strings.Join(missing, ", "))
	}

	return &Result{
		ID:       PathCheckID,
		Status:   StatusPass,
		Message:  msg,
		Severity: SeverityInfo,
	}
}

// Remedy returns nil — PATH management is user-controlled.
func (c *PathCheck) Remedy() *Remedy { return nil }

// PATHSanityCheck warns if PATH length is excessive or contains polluted Temp entries.
// It is a dedicated Git Bash robustness check for Windows.
type PATHSanityCheck struct {
	getenvFn func(string) string
}

// NewPATHSanityCheck creates a PATHSanityCheck using the default environment.
func NewPATHSanityCheck() *PATHSanityCheck {
	return &PATHSanityCheck{
		getenvFn: os.Getenv,
	}
}

// NewPATHSanityCheckWithCustom creates a PATHSanityCheck with injected functions for testing.
func NewPATHSanityCheckWithCustom(getenvFn func(string) string) *PATHSanityCheck {
	return &PATHSanityCheck{
		getenvFn: getenvFn,
	}
}

// ID returns the check identifier.
func (c *PATHSanityCheck) ID() CheckID { return PathSanityCheckID }

// Run checks PATH length and pollution.
func (c *PATHSanityCheck) Run(ctx context.Context) *Result {
	path := c.getenvFn("PATH")
	if path == "" {
		return &Result{
			ID:       PathSanityCheckID,
			Status:   StatusWarn,
			Message:  "PATH environment variable is empty",
			Severity: SeverityWarning,
		}
	}
	if len(path) > 8000 {
		return &Result{
			ID:       PathSanityCheckID,
			Status:   StatusWarn,
			Message:  fmt.Sprintf("PATH length %d exceeds 8000 characters, may be truncated on Windows (Git Bash)", len(path)),
			Severity: SeverityWarning,
			Error:    fmt.Sprintf("PATH length %d > 8000", len(path)),
		}
	}
	if polluted := findPollutedEntries(path); len(polluted) > 0 {
		return &Result{
			ID:       PathSanityCheckID,
			Status:   StatusWarn,
			Message:  fmt.Sprintf("PATH contains polluted Temp entries: %s", strings.Join(polluted, ", ")),
			Severity: SeverityWarning,
			Error:    strings.Join(polluted, ", "),
		}
	}
	return &Result{
		ID:       PathSanityCheckID,
		Status:   StatusPass,
		Message:  "PATH sanity OK",
		Severity: SeverityInfo,
	}
}

// Remedy returns nil — PATH management is user-controlled.
func (c *PATHSanityCheck) Remedy() *Remedy { return nil }
