// Package project provides 5-case project detection ported from Engram.
// Quick-win port for biggz-ai BigMem: config → git_remote → git_root → git_child → ambiguous → dir_basename.
package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Errors.
var (
	ErrAmbiguousProject = errors.New("ambiguous project: multiple git repos found in cwd")
	ErrInvalidConfig    = errors.New("invalid config project_name")
)

// Sources.
const (
	SourceConfig                               = "config"
	SourceGitRemote                            = "git_remote"
	SourceGitRoot                              = "git_root"
	SourceGitChild                             = "git_child"
	SourceDirBasename                          = "dir_basename"
	SourceAmbiguous                            = "ambiguous"
	SourceExplicitOverride                     = "explicit_override"
	SourceUserSelectedAfterAmbiguousProject    = "user_selected_after_ambiguous_project"
	SourceSessionProject                       = "session"
	SourceRequestBody                          = "request_body"
	SourceAllProjects                          = "all_projects"
	SourceProcessOverride                      = "process_override"
)

// Env precedence for SourceConfig: BIGMEM_PROJECT > BIGGZ_PROJECT > ENGRAM_PROJECT.
var envConfigKeys = []string{"BIGMEM_PROJECT", "BIGGZ_PROJECT", "ENGRAM_PROJECT"}

// EnvProjectOverride is the legacy single key, kept for compatibility.
const EnvProjectOverride = "ENGRAM_PROJECT"

// noiseSet lists directory names that are skipped during child-repo scanning.
var noiseSet = map[string]bool{
	"node_modules": true,
	"vendor":       true,
	".venv":        true,
	"__pycache__":  true,
	"target":       true,
	"dist":         true,
	"build":        true,
	".idea":        true,
	".vscode":      true,
}

// ProjectInfo carries the full output of DetectProjectFull.
type ProjectInfo struct {
	Project           string
	Source            string
	Path              string
	Warning           string
	Error             error
	AvailableProjects []string
}

// DetectionResult is an alias for ProjectInfo for Engram compatibility.
type DetectionResult = ProjectInfo

// ProcessOverride returns the process-level project override.
// Precedence: explicit arg → BIGMEM_PROJECT → BIGGZ_PROJECT → ENGRAM_PROJECT.
func ProcessOverride(explicit string) (string, bool) {
	if trimmed := strings.TrimSpace(explicit); trimmed != "" {
		return trimmed, true
	}
	for _, key := range envConfigKeys {
		if trimmed := strings.TrimSpace(os.Getenv(key)); trimmed != "" {
			return trimmed, true
		}
	}
	return "", false
}

// IsNoiseDir reports whether name is a known noise directory (skip during scan).
func IsNoiseDir(name string) bool {
	return noiseSet[name]
}

// CanonicalizePath resolves symlinks via EvalSymlinks and cleans the result.
// Falls back to Clean when EvalSymlinks fails.
func CanonicalizePath(path string) string {
	if path == "" {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(resolved)
}

// canonicalizePath is the unexported alias used internally.
func canonicalizePath(path string) string { return CanonicalizePath(path) }

// isNoiseDir unexported alias.
func isNoiseDir(name string) bool { return IsNoiseDir(name) }

// NormalizeProjectName normalizes a project name to kebab lowercase.
// Steps: trim, lowercase, replace spaces/underscores with hyphen, collapse --, trim hyphens.
func NormalizeProjectName(name string) string {
	n := strings.TrimSpace(strings.ToLower(name))
	if n == "" {
		return "unknown"
	}
	n = strings.ReplaceAll(n, " ", "-")
	n = strings.ReplaceAll(n, "_", "-")
	for strings.Contains(n, "--") {
		n = strings.ReplaceAll(n, "--", "-")
	}
	n = strings.Trim(n, "-")
	if n == "" {
		return "unknown"
	}
	return n
}

// normalize is internal helper matching Engram's normalize semantics (delegates to NormalizeProjectName).
func normalize(name string) string { return NormalizeProjectName(name) }

// normalizeConfigProjectName validates and normalizes a config project_name (file or env).
func normalizeConfigProjectName(projectName string) (string, error) {
	trimmed := strings.TrimSpace(projectName)
	if trimmed == "" {
		return "", fmt.Errorf("%w: project_name is required", ErrInvalidConfig)
	}
	if strings.ContainsAny(trimmed, `/\`) {
		return "", fmt.Errorf("%w: project_name must be a name, not a path", ErrInvalidConfig)
	}
	for _, r := range trimmed {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("%w: project_name contains control characters", ErrInvalidConfig)
		}
	}
	return NormalizeProjectName(trimmed), nil
}

// DetectProject resolves project for cwd and returns ProjectInfo + error.
// Error is non-nil only for ErrAmbiguousProject or ErrInvalidConfig; callers may fallback to basename.
func DetectProject(cwd string) (ProjectInfo, error) {
	res := DetectProjectFull(cwd)
	if res.Error != nil {
		return res, res.Error
	}
	return res, nil
}

// DetectProjectFull resolves the project for dir using a 5-case algorithm:
//
//  0. config     — BIGMEM_PROJECT/BIGGZ_PROJECT/ENGRAM_PROJECT env OR nearest .biggz/.engram/config.json within enclosing repo/root
//  1. git_remote — cwd is a git root with a remote → derive name from remote URL
//  2. git_root   — cwd is inside a git repo → use repo root basename
//  3. git_child  — cwd has exactly one git-repo child → auto-promote it
//  4. ambiguous  — cwd has multiple git-repo children → return ErrAmbiguousProject
//  5. dir_basename — none of the above → use filepath.Base(dir)
func DetectProjectFull(dir string) ProjectInfo {
	if dir == "" {
		dir = "."
	}
	if strings.HasPrefix(dir, "-") {
		dir = "./" + dir
	}

	if res, ok := detectFromEnv(dir); ok {
		return res
	}
	if res, ok := detectFromConfig(dir); ok {
		return res
	}

	// Case 1: git_remote
	if name := detectFromGitRemote(dir); name != "" {
		path := detectGitRootDir(dir)
		if path == "" {
			path, _ = filepath.Abs(dir)
		}
		return ProjectInfo{
			Project: NormalizeProjectName(name),
			Source:  SourceGitRemote,
			Path:    path,
		}
	}

	// Case 2: git_root (includes subdir case)
	if root := detectGitRootDir(dir); root != "" {
		return ProjectInfo{
			Project: NormalizeProjectName(filepath.Base(root)),
			Source:  SourceGitRoot,
			Path:    root,
		}
	}

	// Cases 3 & 4: scan child directories
	children, timedOut := scanChildren(dir)
	if timedOut {
		goto basename
	}
	switch len(children) {
	case 1:
		child := children[0]
		childName := NormalizeProjectName(filepath.Base(child))
		absChild, _ := filepath.Abs(child)
		return ProjectInfo{
			Project: childName,
			Source:  SourceGitChild,
			Path:    absChild,
			Warning: "auto-promoted child repository: " + childName,
		}
	default:
		if len(children) > 1 {
			names := make([]string, len(children))
			for i, c := range children {
				names[i] = NormalizeProjectName(filepath.Base(c))
			}
			absDir, _ := filepath.Abs(dir)
			return ProjectInfo{
				Project:           "",
				Source:            SourceAmbiguous,
				Path:              absDir,
				Error:             ErrAmbiguousProject,
				AvailableProjects: names,
			}
		}
	}

basename:
	absDir, _ := filepath.Abs(dir)
	base := filepath.Base(dir)
	if base == "" || base == "." {
		base = "unknown"
	}
	return ProjectInfo{
		Project: NormalizeProjectName(base),
		Source:  SourceDirBasename,
		Path:    absDir,
	}
}

// detectFromEnv checks BIGMEM_PROJECT/BIGGZ_PROJECT/ENGRAM_PROJECT in order.
// Returns (info, true) when env is set (even if invalid, Error is set).
func detectFromEnv(dir string) (ProjectInfo, bool) {
	for _, key := range envConfigKeys {
		if raw := strings.TrimSpace(os.Getenv(key)); raw != "" {
			absDir, _ := filepath.Abs(dir)
			absDir = CanonicalizePath(absDir)
			normalized, err := normalizeConfigProjectName(raw)
			if err != nil {
				return ProjectInfo{
					Project: "",
					Source:  SourceConfig,
					Path:    absDir,
					Error:   fmt.Errorf("%w: %v", ErrInvalidConfig, err),
				}, true
			}
			return ProjectInfo{
				Project: normalized,
				Source:  SourceConfig,
				Path:    absDir,
			}, true
		}
	}
	return ProjectInfo{}, false
}

type configFile struct {
	ProjectName string `json:"project_name"`
}

func detectFromConfig(dir string) (ProjectInfo, bool) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = dir
	}
	absDir = CanonicalizePath(absDir)

	if gitRoot := CanonicalizePath(detectGitRootDir(absDir)); gitRoot != "" {
		return readNearestConfigAtOrBelow(absDir, gitRoot)
	}
	return readConfigAt(absDir)
}

func readNearestConfigAtOrBelow(startDir, stopDir string) (ProjectInfo, bool) {
	current := filepath.Clean(startDir)
	stop := filepath.Clean(stopDir)
	for {
		if res, ok := readConfigAt(current); ok {
			return res, true
		}
		if current == stop {
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return ProjectInfo{}, false
}

func readConfigAt(projectDir string) (ProjectInfo, bool) {
	// Check .biggz first, then .engram for biggz-ai compatibility (both map to SourceConfig).
	candidates := []string{
		filepath.Join(projectDir, ".biggz", "config.json"),
		filepath.Join(projectDir, ".engram", "config.json"),
	}
	var data []byte
	var err error
	var chosen string
	for _, p := range candidates {
		data, err = os.ReadFile(p)
		if err == nil {
			chosen = p
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return invalidConfigResult(projectDir, fmt.Errorf("read %s: %w", p, err)), true
		}
	}
	if chosen == "" {
		return ProjectInfo{}, false
	}
	var cfg configFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return invalidConfigResult(projectDir, fmt.Errorf("parse %s: %w", chosen, err)), true
	}
	projectName, err := normalizeConfigProjectName(cfg.ProjectName)
	if err != nil {
		return invalidConfigResult(projectDir, err), true
	}
	return ProjectInfo{Project: projectName, Source: SourceConfig, Path: projectDir}, true
}

func invalidConfigResult(path string, err error) ProjectInfo {
	return ProjectInfo{
		Project: "",
		Source:  SourceConfig,
		Path:    path,
		Error:   fmt.Errorf("%w: %v", ErrInvalidConfig, err),
	}
}

// detectGitRootDir returns the git repository root for dir, or "" if not in a repo.
func detectGitRootDir(dir string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := newProjectCommandContext(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	root := strings.TrimSpace(string(out))
	return root
}

// scanChildren scans dir at depth=1 for git repositories, skipping noise dirs,
// hidden dirs, enforcing a 200ms timeout, a 20-entry cap, and short-circuiting
// as soon as more than 1 repo is found.
func scanChildren(dir string) (repos []string, timedOut bool) {
	deadline := time.Now().Add(200 * time.Millisecond)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, false
	}
	scanned := 0
	for _, entry := range entries {
		if time.Now().After(deadline) {
			return repos, true
		}
		if scanned >= 20 {
			break
		}
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if isNoiseDir(name) {
			continue
		}
		scanned++
		childPath := filepath.Join(dir, name)
		gitPath := filepath.Join(childPath, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			repos = append(repos, childPath)
			if len(repos) > 1 {
				return repos, false
			}
		}
	}
	return repos, false
}

// newProjectCommandContext creates a git command with context.
// Uses exec.CommandContext; on Windows the caller may hide window via SysProcAttr.
func newProjectCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

// detectFromGitRemote attempts to determine the project name from the git remote origin URL.
func detectFromGitRemote(dir string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := newProjectCommandContext(ctx, "git", "-C", dir, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	url := strings.TrimSpace(string(out))
	return extractRepoName(url)
}

// extractRepoName parses a git remote URL and returns just the repository name.
// Supported: SSH git@github.com:user/repo.git, HTTPS https://github.com/user/repo.git, with or without .git.
func extractRepoName(url string) string {
	url = strings.TrimSuffix(url, ".git")
	parts := strings.FieldsFunc(url, func(r rune) bool { return r == '/' || r == ':' })
	if len(parts) == 0 {
		return ""
	}
	name := parts[len(parts)-1]
	return strings.TrimSpace(name)
}

// DetectProjectLegacy is a backward-compatible wrapper returning only the name string (like Engram's DetectProject).
// On ErrAmbiguousProject it falls back to basename so callers never receive empty string.
func DetectProjectLegacy(dir string) string {
	res := DetectProjectFull(dir)
	if errors.Is(res.Error, ErrAmbiguousProject) {
		if dir == "" {
			return "unknown"
		}
		base := filepath.Base(dir)
		if base == "" || base == "." {
			return "unknown"
		}
		return normalize(base)
	}
	if res.Project == "" {
		return "unknown"
	}
	return res.Project
}
