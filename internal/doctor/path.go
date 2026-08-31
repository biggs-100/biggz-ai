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

// Run scans PATH for duplicate binaries.
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
