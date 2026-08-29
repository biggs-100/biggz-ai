package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/assets"
)

const (
	GlobalDriftCheckID   CheckID = "sdd-global-asset-drift"
	LocalOverrideCheckID CheckID = "sdd-local-agent-override"
)

type managedAssetsManifest struct {
	SchemaVersion int               `json:"schemaVersion"`
	Assets        map[string]string `json:"assets"`
}

func readManagedAssetsManifest(path string, readFile func(string) ([]byte, error)) (managedAssetsManifest, error) {
	data, err := readFile(path)
	if err != nil {
		return managedAssetsManifest{Assets: map[string]string{}}, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return managedAssetsManifest{Assets: map[string]string{}}, nil
	}
	var m managedAssetsManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return managedAssetsManifest{Assets: map[string]string{}}, err
	}
	if m.Assets == nil {
		m.Assets = map[string]string{}
	}
	return m, nil
}

// SDDGlobalAssetDriftCount returns the number of global SDD assets whose
// installed content hash differs from the manifest. Read-only, no writes.
func SDDGlobalAssetDriftCount(manifestPath, installedRoot string) int {
	return sddGlobalAssetDriftCount(manifestPath, installedRoot, os.ReadFile, os.Stat)
}

// sddGlobalAssetDriftCount is the unexported core (injectable for tests).
func sddGlobalAssetDriftCount(manifestPath, installedRoot string, readFile func(string) ([]byte, error), statFn func(string) (os.FileInfo, error)) int {
	manifest, err := readManagedAssetsManifest(manifestPath, readFile)
	if err != nil {
		// No manifest or corrupt -> no drift (fresh install, pass)
		return 0
	}
	if len(manifest.Assets) == 0 {
		return 0
	}
	stale := 0
	for ownershipKey, expectedHex := range manifest.Assets {
		if expectedHex == "" {
			continue
		}
		installedPath := filepath.Join(installedRoot, filepath.FromSlash(ownershipKey))
		if _, err := statFn(installedPath); err != nil {
			if os.IsNotExist(err) {
				stale++
			} else {
				stale++
			}
			continue
		}
		data, err := readFile(installedPath)
		if err != nil {
			stale++
			continue
		}
		// Normalize frontmatter routing for agents like gentle-pi does:
		// strip model:/thinking: lines before hashing comparison.
		// We compare raw hash but also tolerate routing-only differences
		// by hashing after stripping those lines for agents/* keys.
		hash := assets.ManagedAssetHash(data)
		if hash != expectedHex {
			// For agent files, also compare after stripping routing to avoid false drift
			if strings.HasPrefix(ownershipKey, "agents/") {
				stripped := stripRoutingFrontmatter(string(data))
				if assets.ManagedAssetHash([]byte(stripped)) == expectedHex {
					continue
				}
			}
			stale++
		}
	}
	return stale
}

func stripRoutingFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---\n") {
		return content
	}
	end := strings.Index(content[4:], "\n---")
	if end == -1 {
		return content
	}
	end += 4
	front := content[4:end]
	body := content[end:]
	lines := strings.Split(front, "\n")
	filtered := make([]string, 0, len(lines))
	for _, l := range lines {
		if strings.HasPrefix(l, "model:") || strings.HasPrefix(l, "thinking:") {
			continue
		}
		filtered = append(filtered, l)
	}
	return "---\n" + strings.Join(filtered, "\n") + body
}

// SDDLocalAgentOverrideCount counts project-local SDD agent overrides under cwd.
func SDDLocalAgentOverrideCount(cwd string) int {
	return sddLocalAgentOverrideCount(cwd, os.ReadDir)
}

func sddLocalAgentOverrideCount(cwd string, readDirFn func(string) ([]os.DirEntry, error)) int {
	if cwd == "" {
		return 0
	}
	count := 0
	for _, sub := range []string{filepath.Join(cwd, ".pi", "agents"), filepath.Join(cwd, ".pi", "subagents")} {
		entries, err := readDirFn(sub)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasPrefix(name, "sdd-") && strings.HasSuffix(name, ".md") {
				count++
			}
		}
	}
	return count
}

// GlobalDriftCheck is the RO doctor check for global SDD asset drift.
type GlobalDriftCheck struct {
	manifestPath string
	installedDir string
	readFile     func(string) ([]byte, error)
	statFn       func(string) (os.FileInfo, error)
	readDirFn    func(string) ([]os.DirEntry, error)
	homeDirFn    func() (string, error)
}

// NewGlobalDriftCheck creates the production check using home-dir defaults.
func NewGlobalDriftCheck() *GlobalDriftCheck {
	return &GlobalDriftCheck{
		readFile:  os.ReadFile,
		statFn:    os.Stat,
		readDirFn: os.ReadDir,
		homeDirFn: os.UserHomeDir,
	}
}

// NewGlobalDriftCheckWithCustom creates a check with injected deps for tests.
func NewGlobalDriftCheckWithCustom(manifestPath, installedDir string, readFile func(string) ([]byte, error), statFn func(string) (os.FileInfo, error), readDirFn func(string) ([]os.DirEntry, error)) *GlobalDriftCheck {
	if readFile == nil {
		readFile = os.ReadFile
	}
	if statFn == nil {
		statFn = os.Stat
	}
	if readDirFn == nil {
		readDirFn = os.ReadDir
	}
	return &GlobalDriftCheck{
		manifestPath: manifestPath,
		installedDir: installedDir,
		readFile:     readFile,
		statFn:       statFn,
		readDirFn:    readDirFn,
	}
}

func (c *GlobalDriftCheck) ID() CheckID { return GlobalDriftCheckID }

func (c *GlobalDriftCheck) resolvePaths() (string, string) {
	manifestPath := c.manifestPath
	installedDir := c.installedDir
	if manifestPath == "" || installedDir == "" {
		homeFn := c.homeDirFn
		if homeFn == nil {
			homeFn = os.UserHomeDir
		}
		if home, err := homeFn(); err == nil && home != "" {
			if manifestPath == "" {
				manifestPath = filepath.Join(home, ".pi", "agent", "gentle-ai", "managed-assets.json")
			}
			if installedDir == "" {
				installedDir = filepath.Join(home, ".pi", "agent")
			}
		}
	}
	if manifestPath == "" {
		manifestPath = "managed-assets.json"
	}
	if installedDir == "" {
		installedDir = "."
	}
	return manifestPath, installedDir
}

func (c *GlobalDriftCheck) Run(ctx context.Context) *Result {
	manifestPath, installedDir := c.resolvePaths()
	readFile := c.readFile
	if readFile == nil {
		readFile = os.ReadFile
	}
	statFn := c.statFn
	if statFn == nil {
		statFn = os.Stat
	}
	count := sddGlobalAssetDriftCount(manifestPath, installedDir, readFile, statFn)
	if count > 0 {
		return &Result{
			ID:       GlobalDriftCheckID,
			Status:   StatusWarn,
			Message:  fmt.Sprintf("warn: Global SDD asset drift %d", count),
			Severity: SeverityWarning,
			Details:  map[string]int{"sddGlobalAssetDriftCount": count},
		}
	}
	return &Result{
		ID:       GlobalDriftCheckID,
		Status:   StatusPass,
		Message:  "Global SDD assets OK",
		Severity: SeverityInfo,
		Details:  map[string]int{"sddGlobalAssetDriftCount": 0},
	}
}

func (c *GlobalDriftCheck) Remedy() *Remedy { return nil }

// LocalOverrideCheck is the RO doctor check for project-local overrides.
type LocalOverrideCheck struct {
	cwd       string
	getwdFn   func() (string, error)
	readDirFn func(string) ([]os.DirEntry, error)
}

// NewLocalOverrideCheck creates the production check using cwd.
func NewLocalOverrideCheck() *LocalOverrideCheck {
	return &LocalOverrideCheck{
		getwdFn:   os.Getwd,
		readDirFn: os.ReadDir,
	}
}

// NewLocalOverrideCheckWithCustom creates a check with injected cwd for tests.
func NewLocalOverrideCheckWithCustom(cwd string, getwdFn func() (string, error), readDirFn func(string) ([]os.DirEntry, error)) *LocalOverrideCheck {
	if readDirFn == nil {
		readDirFn = os.ReadDir
	}
	return &LocalOverrideCheck{
		cwd:       cwd,
		getwdFn:   getwdFn,
		readDirFn: readDirFn,
	}
}

func (c *LocalOverrideCheck) ID() CheckID { return LocalOverrideCheckID }

func (c *LocalOverrideCheck) Run(ctx context.Context) *Result {
	cwd := c.cwd
	if cwd == "" {
		fn := c.getwdFn
		if fn == nil {
			fn = os.Getwd
		}
		if v, err := fn(); err == nil {
			cwd = v
		}
	}
	readDir := c.readDirFn
	if readDir == nil {
		readDir = os.ReadDir
	}
	count := sddLocalAgentOverrideCount(cwd, readDir)
	if count > 0 {
		return &Result{
			ID:       LocalOverrideCheckID,
			Status:   StatusWarn,
			Message:  fmt.Sprintf("warn: Project-local SDD agent overrides %d", count),
			Severity: SeverityWarning,
			Details:  map[string]int{"sddLocalAgentOverrideCount": count},
		}
	}
	return &Result{
		ID:       LocalOverrideCheckID,
		Status:   StatusPass,
		Message:  "No project-local SDD agent overrides",
		Severity: SeverityInfo,
		Details:  map[string]int{"sddLocalAgentOverrideCount": 0},
	}
}

func (c *LocalOverrideCheck) Remedy() *Remedy { return nil }
