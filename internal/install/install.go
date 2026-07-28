// Package install implements the install command for biggz-ai.
//
// It discovers an AI coding agent via plugin.AgentAdapter, deploys embedded
// skill files and commands, and merges configuration overlays into the agent's
// settings file. All file operations target the agent's config directory under
// the user's home directory or a custom HomeDir for testing.
package install

import (
	"context"
	"encoding/json"
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

	// Deploy SDD prompts (used by OpenCode to delegate to sub-agents)
	promptsDir := filepath.Join(adapter.GlobalConfigDir(homeDir), "prompts", "sdd")
	if err := deployPrompts(promptsDir, assets.FS); err != nil {
		return result, fmt.Errorf("deploy prompts: %w", err)
	}

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

	// Update biggz-orchestrator prompt in opencode.json
	agentConfigPath := filepath.Join(adapter.GlobalConfigDir(homeDir), "opencode.json")
	if err := updateOrchestratorPrompt(agentConfigPath); err != nil {
		return result, fmt.Errorf("update orchestrator prompt: %w", err)
	}

	// Deploy MCP binary to ~/.biggz/ and update config
	if err := deployMCPBinary(homeDir, agentConfigPath); err != nil {
		return result, fmt.Errorf("deploy mcp: %w", err)
	}

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
		if err != nil {
			// If existing config can't be parsed (e.g. Windows paths with
			// unescaped backslashes), write overlay only.
			merged, err = filemerge.MergeJSONC([]byte("{}"), overlayData)
			if err != nil {
				return false, fmt.Errorf("merge config: %w", err)
			}
		}
	} else {
		merged, err = filemerge.MergeJSONC([]byte("{}"), overlayData)
		if err != nil {
			return false, fmt.Errorf("merge config: %w", err)
		}
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

// deployPrompts copies embedded SDD prompt files (under "prompts/sdd/") into
// the agent's prompts directory. These prompts tell OpenCode's orchestrator
// to delegate SDD phases to sub-agents rather than executing them inline.
func deployPrompts(promptsDir string, ffs fs.FS) error {
	return fs.WalkDir(ffs, "prompts/sdd", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(promptsDir, d.Name()), 0755)
		}
		data, err := fs.ReadFile(ffs, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		relPath := strings.TrimPrefix(path, "prompts/sdd/")
		targetPath := filepath.Join(promptsDir, relPath)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(targetPath), err)
		}
		if err := filemerge.WriteFile(targetPath, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", targetPath, err)
		}
		return nil
	})
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

// updateOrchestratorPrompt reads the biggz orchestrator prompt from embedded
// assets and updates the biggz-orchestrator entry in the agent's opencode.json.
// Uses proper JSON manipulation (encoding/json), never string replacement.
// This is safe — it only modifies the prompt field of the biggz-orchestrator
// agent and preserves all other config.
func updateOrchestratorPrompt(agentConfigPath string) error {
	// Read the prompt from embedded assets
	promptData, err := fs.ReadFile(assets.FS, "biggz/biggz-orchestrator.md")
	if err != nil {
		// If the prompt file doesn't exist, skip silently
		return nil
	}
	prompt := string(promptData)

	// Read the existing agent config
	data, err := os.ReadFile(agentConfigPath)
	if err != nil {
		// If opencode.json doesn't exist, skip silently
		return nil
	}

	// Parse with encoding/json — handles all edge cases
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parse opencode.json: %w", err)
	}

	// Navigate to agent > biggz-orchestrator > prompt
	agents, ok := config["agent"].(map[string]any)
	if !ok {
		return nil // no agents section
	}

	biggzAgent, ok := agents["biggz-orchestrator"].(map[string]any)
	if !ok {
		return nil // no biggz-orchestrator entry
	}

	// Update the prompt
	biggzAgent["prompt"] = prompt

	// Write back with proper JSON marshaling
	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(agentConfigPath, out, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// deployMCPBinary copies biggz-mcp.exe to ~/.biggz/ and updates the MCP config
// in opencode.json to point to the deployed binary. This ensures the MCP server
// works even if the biggz-ai source directory is moved or deleted.
func deployMCPBinary(homeDir, agentConfigPath string) error {
	// Determine the deployed binary path
	biggzDir := filepath.Join(homeDir, ".biggz")
	if err := os.MkdirAll(biggzDir, 0755); err != nil {
		return fmt.Errorf("mkdir .biggz: %w", err)
	}

	// Find the source binary — look next to the running binary first
	srcPath := "biggz-mcp.exe"
	exe, err := os.Executable()
	if err == nil {
		srcPath = filepath.Join(filepath.Dir(exe), "biggz-mcp.exe")
	}

	// Check if source exists
	if _, err := os.Stat(srcPath); err != nil {
		// Source binary not found, skip MCP deploy
		return nil
	}

	dstPath := filepath.Join(biggzDir, "biggz-mcp.exe")

	// Read source
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read mcp binary: %w", err)
	}

	// Write destination atomically
	if err := filemerge.WriteFile(dstPath, data, 0755); err != nil {
		return fmt.Errorf("write mcp binary: %w", err)
	}

	// Update MCP config in opencode.json
	data, err = os.ReadFile(agentConfigPath)
	if err != nil {
		return nil // opencode.json doesn't exist, skip
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return nil
	}

	mcp, ok := config["mcp"].(map[string]any)
	if !ok {
		return nil
	}

	engram, ok := mcp["engram"].(map[string]any)
	if !ok {
		return nil
	}

	// Update the command path
	engram["command"] = []string{dstPath, "--tools=agent"}

	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(agentConfigPath, out, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}
