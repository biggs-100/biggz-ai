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
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/biggz-ai/biggz/internal/assets"
	"github.com/biggz-ai/biggz/internal/filemerge"
	"github.com/biggz-ai/biggz/model"
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
	homeDir := cfg.HomeDir
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}

	installed, binaryPath, _, _, err := adapter.Detect(ctx, homeDir)
	if err != nil || !installed {
		return &Result{AgentDetected: false}, fmt.Errorf("agent not detected: %w", err)
	}

	result := &Result{
		AgentDetected: true,
		BinaryPath:    binaryPath,
		DryRun:        cfg.DryRun,
	}

	// Deploy skills to ~/.biggz/skills/ (canonical source — no gentle-ai conflict)
	deployed, err := DeploySkillsToBiggzDir(homeDir, assets.FS, cfg.DryRun)
	if err != nil {
		return result, fmt.Errorf("deploy skills: %w", err)
	}
	result.SkillsDeployed = deployed

	// Also copy skills to the agent's skills directory so OpenCode's native
	// skill discovery (~/.config/opencode/skills/<name>/SKILL.md) can find them.
	skillsDir := adapter.SkillsDir(homeDir)
	if _, err := DeploySkillsToAgentDir(skillsDir, assets.FS, cfg.DryRun); err != nil {
		return result, fmt.Errorf("deploy skills to agent: %w", err)
	}

	// Deploy SDD prompts (used by OpenCode to delegate to sub-agents)
	promptsDir := filepath.Join(adapter.GlobalConfigDir(homeDir), "prompts", "sdd")
	if err := DeployPrompts(promptsDir, assets.FS, cfg.DryRun); err != nil {
		return result, fmt.Errorf("deploy prompts: %w", err)
	}

	// Deploy config overlay with absolute paths to ~/.biggz/
	merged, err := DeployBiggzConfig(adapter, homeDir, assets.FS, cfg.DryRun)
	if err != nil {
		return result, fmt.Errorf("deploy config: %w", err)
	}
	result.ConfigMerged = merged

	commandsDir := filepath.Join(adapter.GlobalConfigDir(homeDir), "commands")
	written, err := DeployCommands(commandsDir, assets.FS, cfg.DryRun)
	if err != nil {
		return result, fmt.Errorf("write commands: %w", err)
	}
	result.CommandsWritten = written

	// Inject persona into system prompt file (AGENTS.md or equivalent)
	if err := DeployPersona(adapter, homeDir, cfg.DryRun); err != nil {
		return result, fmt.Errorf("deploy persona: %w", err)
	}

	// Deploy MCP binary to ~/.biggz/
	mcpBinPath, err := DeployMCPBinaryToHomeDir(homeDir, cfg.DryRun)
	if err != nil {
		return result, fmt.Errorf("deploy mcp binary: %w", err)
	}

	// Write MCP config entry using the adapter's strategy
	if mcpBinPath != "" {
		if err := DeployMCPConfig(adapter, homeDir, mcpBinPath, cfg.DryRun); err != nil {
			return result, fmt.Errorf("deploy mcp config: %w", err)
		}
	}

	// Ensure biggz binary is on PATH for terminal use
	if !cfg.DryRun {
		if err := deploySelfToPath(homeDir); err != nil {
			return result, fmt.Errorf("deploy to path: %w", err)
		}
	}

	return result, nil
}

// DeploySkillsToBiggzDir copies all embedded skill files from ffs (under the
// "skills/" prefix) into ~/.biggz/skills/, creating parent directories as needed.
// This is the canonical source — agent configs reference these paths absolutely.
// When dryRun is true, counts files without writing.
func DeploySkillsToBiggzDir(homeDir string, ffs fs.FS, dryRun bool) (int, error) {
	biggzSkillsDir := filepath.Join(homeDir, ".biggz", "skills")
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
		targetPath := filepath.Join(biggzSkillsDir, relPath)
		if dryRun {
			count++
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(targetPath), err)
		}
		if _, err := filemerge.WriteFileAtomic(targetPath, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", targetPath, err)
		}
		count++
		return nil
	})
	return count, err
}

// DeploySkills copies all embedded skill files to the agent's skills directory.
// Backward-compatible alias: delegates to DeploySkillsToAgentDir.
func DeploySkills(skillsDir string, ffs fs.FS, dryRun bool) (int, error) {
	return DeploySkillsToAgentDir(skillsDir, ffs, dryRun)
}

// sharedSkillNames are skill names that exist in both biggz-ai and gentle-ai.
// These are deployed to ~/.biggz/skills/ (canonical store) but NOT to the
// agent's skills directory to avoid overwriting gentle-ai's versions.
var sharedSkillNames = map[string]bool{
	"branch-pr":             true,
	"chained-pr":            true,
	"cognitive-doc-design":  true,
	"comment-writer":        true,
	"issue-creation":        true,
	"work-unit-commits":     true,
}

// DeploySkillsToAgentDir copies biggz-only embedded skill files to the agent's
// skills directory. Skills that also exist in gentle-ai are skipped to avoid
// conflicts — they are only stored in ~/.biggz/skills/ and resolved via the
// skill registry.
func DeploySkillsToAgentDir(skillsDir string, ffs fs.FS, dryRun bool) (int, error) {
	count := 0
	err := fs.WalkDir(ffs, "skills", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		relPath := strings.TrimPrefix(path, "skills/")
		skillName := filepath.Dir(relPath)
		if sharedSkillNames[skillName] {
			return nil
		}
		data, err := fs.ReadFile(ffs, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		targetPath := filepath.Join(skillsDir, relPath)
		if dryRun {
			count++
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(targetPath), err)
		}
		if _, err := filemerge.WriteFileAtomic(targetPath, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", targetPath, err)
		}
		count++
		return nil
	})
	return count, err
}

// DeployBiggzConfig reads the agent's existing settings file (if any), generates
// a config overlay with absolute skill paths pointing to ~/.biggz/skills/, and
// merges it into the settings file atomically.
// Returns true when a file was written (new or updated).
// When dryRun is true, returns whether a merge would occur without writing.
func DeployBiggzConfig(adapter plugin.AgentAdapter, homeDir string, ffs fs.FS, dryRun bool) (bool, error) {
	settingsPath := adapter.SettingsPath(homeDir)
	if settingsPath == "" {
		return false, nil
	}

	// Generate overlay with absolute paths to ~/.biggz/
	overlay, err := generateBiggzOverlay(ffs, homeDir)
	if err != nil {
		return false, fmt.Errorf("generate overlay: %w", err)
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
		merged, err = filemerge.MergeJSONC(existingData, overlay)
		if err != nil {
			merged, err = filemerge.MergeJSONC([]byte("{}"), overlay)
			if err != nil {
				return false, fmt.Errorf("merge config: %w", err)
			}
		}
	} else {
		merged, err = filemerge.MergeJSONC([]byte("{}"), overlay)
		if err != nil {
			return false, fmt.Errorf("merge config: %w", err)
		}
	}

	if dryRun {
		return true, nil
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return false, fmt.Errorf("mkdir config dir: %w", err)
	}
	if _, err := filemerge.WriteFileAtomic(settingsPath, merged, 0644); err != nil {
		return false, fmt.Errorf("write config: %w", err)
	}
	return true, nil
}

// DeployConfig is a backward-compatible function that merges the static config
// overlay (with relative paths) into the agent's settings file. Used by sync
// command and components system.
func DeployConfig(settingsPath string, ffs fs.FS, dryRun bool) (bool, error) {
	overlayData, err := fs.ReadFile(ffs, "opencode/sdd-overlay-multi.json")
	if err != nil {
		return false, err
	}

	var existingData []byte
	if _, err := os.Stat(settingsPath); err == nil {
		existingData, err = os.ReadFile(settingsPath)
		if err != nil {
			return false, err
		}
	}

	var merged []byte
	if len(existingData) > 0 {
		merged, err = filemerge.MergeJSONC(existingData, overlayData)
		if err != nil {
			merged, err = filemerge.MergeJSONC([]byte("{}"), overlayData)
			if err != nil {
				return false, err
			}
		}
	} else {
		merged, err = filemerge.MergeJSONC([]byte("{}"), overlayData)
		if err != nil {
			return false, err
		}
	}

	if dryRun {
		return true, nil
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return false, err
	}
	if _, err := filemerge.WriteFileAtomic(settingsPath, merged, 0644); err != nil {
		return false, err
	}
	return true, nil
}

// generateBiggzOverlay reads the embedded overlay template and:
//  1. Replaces __ORCHESTRATOR_PROMPT__ with the actual orchestrator prompt content
//  2. Transforms skill paths from relative to absolute, pointing at ~/.biggz/skills/
func generateBiggzOverlay(ffs fs.FS, homeDir string) ([]byte, error) {
	data, err := fs.ReadFile(ffs, "opencode/sdd-overlay-multi.json")
	if err != nil {
		return nil, err
	}

	var overlay map[string]any
	if err := json.Unmarshal(data, &overlay); err != nil {
		return nil, fmt.Errorf("parse overlay: %w", err)
	}

	// Replace __ORCHESTRATOR_PROMPT__ with the actual orchestrator prompt content
	if agents, ok := overlay["agent"].(map[string]any); ok {
		if orch, ok := agents["biggz-orchestrator"].(map[string]any); ok {
			if prompt, ok := orch["prompt"].(string); ok && prompt == "__ORCHESTRATOR_PROMPT__" {
				promptData, err := fs.ReadFile(ffs, "biggz/biggz-orchestrator.md")
				if err == nil {
					orch["prompt"] = string(promptData)
				}
			}
		}
	}

	return json.MarshalIndent(overlay, "", "  ")
}

// DeployCommands copies all embedded command .md files from ffs (under the
// "opencode/commands/" prefix) into commandsDir, creating parent dirs as needed.
// When dryRun is true, counts files without writing.
func DeployCommands(commandsDir string, ffs fs.FS, dryRun bool) (int, error) {
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
		if dryRun {
			count++
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(targetPath), err)
		}
		if _, err := filemerge.WriteFileAtomic(targetPath, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", targetPath, err)
		}
		count++
		return nil
	})
	return count, err
}

// DeployPrompts copies embedded SDD prompt files (under "prompts/sdd/") into
// the agent's prompts directory. These prompts tell OpenCode's orchestrator
// to delegate SDD phases to sub-agents rather than executing them inline.
// When dryRun is true, counts files without writing.
func DeployPrompts(promptsDir string, ffs fs.FS, dryRun bool) error {
	return fs.WalkDir(ffs, "prompts/sdd", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if !dryRun {
				return os.MkdirAll(filepath.Join(promptsDir, d.Name()), 0755)
			}
			return nil
		}
		data, err := fs.ReadFile(ffs, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		relPath := strings.TrimPrefix(path, "prompts/sdd/")
		targetPath := filepath.Join(promptsDir, relPath)
		if dryRun {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(targetPath), err)
		}
		if _, err := filemerge.WriteFileAtomic(targetPath, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", targetPath, err)
		}
		return nil
	})
}

// DeployPersona injects the biggz-ai persona content into the agent's AGENTS.md
// (or equivalent system prompt file). Uses <!-- biggz:persona --> markers for
// idempotent injection — subsequent runs replace content within the markers.
// Compatible with gentle-ai's <!-- gentle-ai:persona --> markers (both can coexist).
func DeployPersona(adapter plugin.AgentAdapter, homeDir string, dryRun bool) error {
	if !adapter.SupportsSystemPrompt() {
		return nil
	}
	promptFile := adapter.SystemPromptFile(homeDir)
	if promptFile == "" {
		return nil
	}

	// Read persona content from embedded assets
	personaData, err := fs.ReadFile(assets.FS, "biggz/biggz-persona.md")
	if err != nil {
		return nil // persona file not embedded, skip
	}
	personaContent := string(personaData)

	// Read existing system prompt file
	var existing []byte
	if _, err := os.Stat(promptFile); err == nil {
		existing, err = os.ReadFile(promptFile)
		if err != nil {
			return fmt.Errorf("read %s: %w", promptFile, err)
		}
	}

	updated := injectByMarker(string(existing), personaContent, "biggz:persona")

	if dryRun {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(promptFile), 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(promptFile), err)
	}
	if _, err := filemerge.WriteFileAtomic(promptFile, []byte(updated), 0644); err != nil {
		return fmt.Errorf("write %s: %w", promptFile, err)
	}
	return nil
}

// injectByMarker inserts content into a markdown file using HTML comment markers.
// If the opening marker <!-- <name> --> exists, content between markers is replaced.
// Otherwise, content is appended at the end with the marker pair.
func injectByMarker(existing, content, name string) string {
	openMarker := "<!-- " + name + " -->"
	closeMarker := "<!-- /" + name + " -->"

	openIdx := strings.Index(existing, openMarker)
	if openIdx == -1 {
		// No marker found — append at end
		if existing != "" && !strings.HasSuffix(existing, "\n") {
			existing += "\n"
		}
		return existing + "\n" + content + "\n"
	}

	closeIdx := strings.Index(existing, closeMarker)
	if closeIdx == -1 {
		// Opening marker found but no closing marker — replace from open to end
		return existing[:openIdx] + content + "\n"
	}

	// Both markers found — replace content between them
	return existing[:openIdx] + content + existing[closeIdx+len(closeMarker):]
}

// DeployMCPBinaryToHomeDir copies the MCP server binary to ~/.biggz/ so it
// works even if the biggz-ai source directory is moved or deleted.
// Returns the destination path, or empty string if no source binary was found.
// When dryRun is true, no files are written.
func DeployMCPBinaryToHomeDir(homeDir string, dryRun bool) (string, error) {
	biggzDir := filepath.Join(homeDir, ".biggz")

	// Find the source binary — look next to the running binary first
	srcPath := "biggz-mcp.exe"
	exe, err := os.Executable()
	if err == nil {
		srcPath = filepath.Join(filepath.Dir(exe), "biggz-mcp.exe")
	}

	// Check if source exists
	if _, err := os.Stat(srcPath); err != nil {
		return "", nil // source binary not found, skip
	}

	dstPath := filepath.Join(biggzDir, "biggz-mcp.exe")

	if dryRun {
		return dstPath, nil
	}

	if err := os.MkdirAll(biggzDir, 0755); err != nil {
		return "", fmt.Errorf("mkdir .biggz: %w", err)
	}

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("read mcp binary: %w", err)
	}

	if _, err := filemerge.WriteFileAtomic(dstPath, data, 0755); err != nil {
		return "", fmt.Errorf("write mcp binary: %w", err)
	}

	return dstPath, nil
}

// DeployMCPConfig writes the MCP server configuration into the agent's config
// using the adapter's MCPStrategy. Three strategies are currently supported:
//
//   - MergeIntoSettings: merge into the agent's settings file (OpenCode, Qwen)
//   - SeparateMCPFiles: write a per-server JSON file (Claude Code)
//
// Other strategies are a no-op until their adapters are implemented.
func DeployMCPConfig(adapter plugin.AgentAdapter, homeDir, mcpBinaryPath string, dryRun bool) error {
	if !adapter.SupportsMCP() {
		return nil
	}

	switch adapter.MCPStrategy() {
	case model.StrategyMergeIntoSettings:
		return deployMCPMergeIntoSettings(adapter, homeDir, mcpBinaryPath, dryRun)
	case model.StrategySeparateMCPFiles:
		return deployMCPSeparateFile(adapter, homeDir, mcpBinaryPath, dryRun)
	default:
		return nil
	}
}

// deployMCPMergeIntoSettings merges the biggz MCP server entry into the agent's
// settings file (opencode.jsonc for OpenCode, settings.json for Qwen, etc.).
// Uses filemerge.MergeJSONC to preserve JSONC comments if present.
func deployMCPMergeIntoSettings(adapter plugin.AgentAdapter, homeDir, mcpBinaryPath string, dryRun bool) error {
	settingsPath := adapter.SettingsPath(homeDir)
	if settingsPath == "" {
		return nil
	}

	// Build the MCP overlay JSON
	mcpOverlay := map[string]any{
		"mcp": map[string]any{
			"biggz": map[string]any{
			"command": []string{mcpBinaryPath, "--tools=agent", "--prefix=biggz"},
				"type":    "local",
				"enabled": true,
			},
		},
	}
	overlay, err := json.Marshal(mcpOverlay)
	if err != nil {
		return fmt.Errorf("marshal mcp overlay: %w", err)
	}

	// Read existing settings (may be JSONC with comments)
	var existingData []byte
	if _, err := os.Stat(settingsPath); err == nil {
		existingData, err = os.ReadFile(settingsPath)
		if err != nil {
			return fmt.Errorf("read settings %s: %w", settingsPath, err)
		}
	}

	if len(existingData) == 0 {
		existingData = []byte("{}")
	}

	// Merge using JSONC-aware function — handles comments and trailing commas
	merged, err := filemerge.MergeJSONC(existingData, overlay)
	if err != nil {
		return fmt.Errorf("merge mcp config into %s: %w", settingsPath, err)
	}

	if dryRun {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(settingsPath), err)
	}
	if _, err := filemerge.WriteFileAtomic(settingsPath, merged, 0644); err != nil {
		return fmt.Errorf("write %s: %w", settingsPath, err)
	}

	return nil
}

// deployMCPSeparateFile writes the biggz MCP server config as a standalone JSON
// file, using the path returned by adapter.MCPConfigPath().
// This is the pattern used by Claude Code (~/.claude/mcp/biggz.json).
func deployMCPSeparateFile(adapter plugin.AgentAdapter, homeDir, mcpBinaryPath string, dryRun bool) error {
	configPath := adapter.MCPConfigPath(homeDir, "biggz")
	if configPath == "" {
		return nil
	}

	serverConfig := map[string]any{
		"biggz": map[string]any{
			"command": []string{mcpBinaryPath, "--tools=agent"},
			"type":    "local",
			"enabled": true,
		},
	}

	out, err := json.MarshalIndent(serverConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal mcp server config: %w", err)
	}

	if dryRun {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(configPath), err)
	}
	if _, err := filemerge.WriteFileAtomic(configPath, out, 0644); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}

	return nil
}

// deploySelfToPath copies the running biggz binary to ~/.biggz/biggz.exe
// and ensures ~/.biggz/ is on the user PATH (persistent, per-user).
// Only runs on Windows and only when homeDir matches the real user home.
func deploySelfToPath(homeDir string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	realHome, _ := os.UserHomeDir()
	if realHome == "" || !strings.EqualFold(homeDir, realHome) {
		return nil
	}
	biggzDir := filepath.Join(homeDir, ".biggz")
	selfPath := filepath.Join(biggzDir, "biggz.exe")

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	// Copy running binary to ~/.biggz/
	needsCopy := true
	if existing, err := os.ReadFile(selfPath); err == nil {
		if current, err := os.ReadFile(exe); err == nil && len(existing) == len(current) {
			needsCopy = false
		}
	}
	if needsCopy {
		data, err := os.ReadFile(exe)
		if err != nil {
			return fmt.Errorf("read %s: %w", exe, err)
		}
		if err := os.MkdirAll(biggzDir, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", biggzDir, err)
		}
		if _, err := filemerge.WriteFileAtomic(selfPath, data, 0755); err != nil {
			return fmt.Errorf("write %s: %w", selfPath, err)
		}
	}

	// Add ~/.biggz/ to persistent user PATH via PowerShell (Windows)
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		`$p = [Environment]::GetEnvironmentVariable('Path','User');
		 if ($p -split ';' -notcontains '`+biggzDir+`') {
		   [Environment]::SetEnvironmentVariable('Path', $p + ';`+biggzDir+`', 'User')
		 }`)
	if err := cmd.Run(); err != nil {
		return err
	}
	// Also update current process PATH so it's usable immediately
	os.Setenv("PATH", biggzDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return nil
}


