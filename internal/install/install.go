// Package install implements the install command for biggz-ai.
//
// It discovers an AI coding agent via plugin.AgentAdapter, deploys embedded
// skill files and commands, and merges configuration overlays into the agent's
// settings file. All file operations target the agent's config directory under
// the user's home directory or a custom HomeDir for testing.
package install

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/biggz-ai/biggz/internal/assets"
	"github.com/biggz-ai/biggz/internal/filemerge"
	"github.com/biggz-ai/biggz/plugin"
)

// Config controls the install behavior.
type Config struct {
	// HomeDir overrides os.UserHomeDir() for testing. When set, all agent
	// config paths are resolved relative to this directory.
	HomeDir string

	// DryRun reports what would be done without writing any files.
	DryRun bool
}

// Result describes what happened during an install run.
type Result struct {
	AgentDetected   bool   // whether the agent binary was found on PATH
	BinaryPath      string // full path to the agent binary (empty if not detected)
	SkillsDeployed  int    // number of skill files written (or would be written in dry-run)
	ConfigMerged    bool   // whether the config file was merged and written
	CommandsWritten int    // number of command files written (or would be written in dry-run)
	DryRun          bool   // whether this was a dry-run (no files written)
}

// Run discovers the agent via adapter, deploys skills, merges config, and
// writes command files. If cfg.DryRun is true it reports counts without
// modifying the filesystem.
func Run(ctx context.Context, adapter plugin.AgentAdapter, cfg Config) (*Result, error) {
	binaryPath, err := adapter.Detect(ctx)
	if err != nil {
		return &Result{AgentDetected: false}, fmt.Errorf("agent not detected: %w", err)
	}

	homeDir := cfg.HomeDir
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}

	result := &Result{
		AgentDetected: true,
		BinaryPath:    binaryPath,
		DryRun:        cfg.DryRun,
	}

	if cfg.DryRun {
		result.SkillsDeployed = countDirFiles(assets.FS, "skills")
		result.ConfigMerged = true
		result.CommandsWritten = countDirFiles(assets.FS, "opencode/commands")
		return result, nil
	}

	skillsDir := adapter.SkillsDir(homeDir)
	deployed, err := deploySkills(skillsDir, assets.FS)
	if err != nil {
		return result, fmt.Errorf("deploy skills: %w", err)
	}
	result.SkillsDeployed = deployed

	settingsPath := adapter.SettingsPath(homeDir)
	merged, err := mergeConfig(settingsPath, assets.FS)
	if err != nil {
		return result, fmt.Errorf("merge config: %w", err)
	}
	result.ConfigMerged = merged

	commandsDir := filepath.Join(adapter.GlobalConfigDir(homeDir), "commands")
	written, err := writeCommands(commandsDir, assets.FS)
	if err != nil {
		return result, fmt.Errorf("write commands: %w", err)
	}
	result.CommandsWritten = written

	return result, nil
}

// deploySkills copies all embedded skill files from ffs (under the "skills/"
// prefix) into skillsDir, creating parent directories as needed.
func deploySkills(skillsDir string, ffs fs.FS) (int, error) {
	count := 0
	err := fs.WalkDir(ffs, "skills", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(ffs, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		relPath := strings.TrimPrefix(path, "skills/")
		targetPath := filepath.Join(skillsDir, relPath)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(targetPath), err)
		}
		if err := filemerge.WriteFile(targetPath, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", targetPath, err)
		}
		count++
		return nil
	})
	return count, err
}

// mergeConfig reads the agent's existing settings file (if any), merges it
// with the embedded overlay from ffs, and writes the result atomically.
// Returns true when a file was written (new or updated).
func mergeConfig(settingsPath string, ffs fs.FS) (bool, error) {
	overlayData, err := fs.ReadFile(ffs, "opencode/sdd-overlay-multi.json")
	if err != nil {
		return false, fmt.Errorf("read overlay: %w", err)
	}

	var existingData []byte
	if _, err := os.Stat(settingsPath); err == nil {
		existingData, err = os.ReadFile(settingsPath)
		if err != nil {
			return false, fmt.Errorf("read existing config: %w", err)
		}
	}

	var merged []byte
	if len(existingData) > 0 {
		merged, err = filemerge.MergeJSONC(existingData, overlayData)
	} else {
		merged, err = filemerge.MergeJSONC([]byte("{}"), overlayData)
	}
	if err != nil {
		return false, fmt.Errorf("merge config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return false, fmt.Errorf("mkdir config dir: %w", err)
	}
	if err := filemerge.WriteFile(settingsPath, merged, 0644); err != nil {
		return false, fmt.Errorf("write config: %w", err)
	}
	return true, nil
}

// writeCommands copies all embedded command .md files from ffs (under the
// "opencode/commands/" prefix) into commandsDir, creating parent dirs as needed.
func writeCommands(commandsDir string, ffs fs.FS) (int, error) {
	count := 0
	err := fs.WalkDir(ffs, "opencode/commands", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(ffs, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		relPath := strings.TrimPrefix(path, "opencode/commands/")
		targetPath := filepath.Join(commandsDir, relPath)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(targetPath), err)
		}
		if err := filemerge.WriteFile(targetPath, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", targetPath, err)
		}
		count++
		return nil
	})
	return count, err
}

// countDirFiles counts all non-directory entries below dir in ffs.
func countDirFiles(ffs fs.FS, dir string) int {
	var count int
	fs.WalkDir(ffs, dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible entries in dry-run counting
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	return count
}
