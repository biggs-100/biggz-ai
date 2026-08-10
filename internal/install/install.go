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

	"github.com/biggs-100/biggz-ai/internal/assets"
	"github.com/biggs-100/biggz-ai/internal/filemerge"
	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugin"
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
	PluginsDeployed int    // number of plugin files written (or would be written in dry-run)
	PromptsDeployed int    // number of prompt files written (or would be written in dry-run)
	MCPDeployed     bool   // whether the MCP server binary and config were deployed
	DryRun          bool   // whether this was a dry-run (no files written)
}

// AssetRelPaths returns the relative paths (embedded asset layout) of every
// file install deploys to agent-owned locations, so callers such as
// uninstall can enumerate exactly what install wrote. AgentSkills excludes
// skills shared with gentle-ai (those only live in ~/.biggz/skills/).
type AssetRelPaths struct {
	AgentSkills []string // relative paths under the agent skills dir
	Prompts     []string // relative paths under prompts/sdd
	Commands    []string // relative paths under the opencode/commands dir
	Plugins     []string // relative paths under the opencode/plugins dir
}

// AgentAssetPaths walks the embedded assets and returns the relative path of
// every file install deploys, mirroring the exact skip rules of the deploy
// functions (shared skills are not copied to the agent skills dir).
func AgentAssetPaths() (AssetRelPaths, error) {
	var out AssetRelPaths

	err := fs.WalkDir(assets.FS, "skills", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		relPath := strings.TrimPrefix(path, "skills/")
		if sharedSkillNames[filepath.Dir(relPath)] {
			return nil
		}
		out.AgentSkills = append(out.AgentSkills, relPath)
		return nil
	})
	if err != nil {
		return out, err
	}

	err = fs.WalkDir(assets.FS, "prompts/sdd", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			out.Prompts = append(out.Prompts, strings.TrimPrefix(path, "prompts/sdd/"))
		}
		return nil
	})
	if err != nil {
		return out, err
	}

	err = fs.WalkDir(assets.FS, "opencode/commands", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			out.Commands = append(out.Commands, strings.TrimPrefix(path, "opencode/commands/"))
		}
		return nil
	})
	if err != nil {
		return out, err
	}

	err = fs.WalkDir(assets.FS, "opencode/plugins", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			out.Plugins = append(out.Plugins, strings.TrimPrefix(path, "opencode/plugins/"))
		}
		return nil
	})
	if err != nil {
		return out, err
	}

	return out, nil
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
	fs.WalkDir(assets.FS, "prompts/sdd", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			result.PromptsDeployed++
		}
		return nil
	})

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

	// Deploy OpenCode plugins to the agent's plugin directory
	// (~/.config/opencode/plugins/ for OpenCode). Local plugin files are
	// auto-discovered at startup and need no `plugin: []` config registration
	// (that array is only for npm packages).
	pluginsDir := filepath.Join(adapter.GlobalConfigDir(homeDir), "plugins")
	pluginsDeployed, err := DeployPlugins(pluginsDir, assets.FS, cfg.DryRun)
	if err != nil {
		return result, fmt.Errorf("deploy plugins: %w", err)
	}
	result.PluginsDeployed = pluginsDeployed

	// Inject persona into system prompt file (AGENTS.md or equivalent)
	if err := DeployPersona(adapter, homeDir, cfg.DryRun); err != nil {
		return result, fmt.Errorf("deploy persona: %w", err)
	}

	// Inject BigMem protocol into system prompt file (AGENTS.md or equivalent)
	// Making BigMem instructions available to ALL agents (including sub-agents)
	// without depending on the orchestrator to forward them at delegation time.
	if err := DeployBigMemProtocol(adapter, homeDir, cfg.DryRun); err != nil {
		return result, fmt.Errorf("deploy bigmem protocol: %w", err)
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
	result.MCPDeployed = mcpBinPath != ""

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
	"branch-pr":            true,
	"chained-pr":           true,
	"cognitive-doc-design": true,
	"comment-writer":       true,
	"issue-creation":       true,
	"work-unit-commits":    true,
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

// DeployPlugins copies all embedded OpenCode plugin files (under the
// "opencode/plugins/" prefix) into pluginsDir — the agent's global plugin
// directory (for OpenCode: ~/.config/opencode/plugins/). OpenCode auto-loads
// every local plugin file there at startup, so no `plugin: []` config
// registration is needed (that array is only for npm packages).
// When dryRun is true, counts files without writing.
func DeployPlugins(pluginsDir string, ffs fs.FS, dryRun bool) (int, error) {
	count := 0
	err := fs.WalkDir(ffs, "opencode/plugins", func(path string, d fs.DirEntry, err error) error {
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
		relPath := strings.TrimPrefix(path, "opencode/plugins/")
		targetPath := filepath.Join(pluginsDir, relPath)
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

	updated := InjectByMarker(string(existing), personaContent, "biggz:persona")

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

// DeployBigMemProtocol injects the BigMem persistent memory protocol into
// the agent's AGENTS.md (or equivalent system prompt file) as a managed section
// with <!-- biggz:bigmem-protocol --> markers.
//
// This ensures ALL agents (including sub-agents) receive the memory protocol
// instructions automatically, without depending on the orchestrator to forward
// them at delegation time.
func DeployBigMemProtocol(adapter plugin.AgentAdapter, homeDir string, dryRun bool) error {
	if !adapter.SupportsSystemPrompt() {
		return nil
	}
	promptFile := adapter.SystemPromptFile(homeDir)
	if promptFile == "" {
		return nil
	}

	// Read BigMem protocol content from embedded assets
	// The file includes <!-- biggz:bigmem-protocol --> markers so InjectByMarker
	// can update it idempotently on subsequent installs.
	protocolData, err := fs.ReadFile(assets.FS, "biggz/bigmem-protocol.md")
	if err != nil {
		return nil // protocol file not embedded, skip
	}
	protocolContent := string(protocolData)

	// Read existing system prompt file
	var existing []byte
	if _, err := os.Stat(promptFile); err == nil {
		existing, err = os.ReadFile(promptFile)
		if err != nil {
			return fmt.Errorf("read %s: %w", promptFile, err)
		}
	}

	updated := InjectByMarker(string(existing), protocolContent, "biggz:bigmem-protocol")

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

// DeployStrictTDDMode injects or removes the <!-- biggz:strict-tdd-mode --> marker
// in the agent's system prompt file. When enabled, the marker tells sub-agents
// that Strict TDD Mode is active. The orchestator reads this marker and forwards
// the instruction to sdd-apply and sdd-verify.
func DeployStrictTDDMode(adapter plugin.AgentAdapter, homeDir string, enabled bool, dryRun bool) error {
	if !adapter.SupportsSystemPrompt() {
		return nil
	}
	promptFile := adapter.SystemPromptFile(homeDir)
	if promptFile == "" {
		return nil
	}

	content := "Strict TDD Mode: enabled\n\nWhen active, sdd-apply writes tests FIRST (RED → GREEN → REFACTOR).\nThe orchestrator MUST forward this to sub-agents via:\n\"STRICT TDD MODE IS ACTIVE. You MUST follow strict-tdd.md.\""

	var existing []byte
	if _, err := os.Stat(promptFile); err == nil {
		existing, err = os.ReadFile(promptFile)
		if err != nil {
			return fmt.Errorf("read %s: %w", promptFile, err)
		}
	}

	var updated string
	if enabled {
		updated = InjectByMarker(string(existing), content, "biggz:strict-tdd-mode")
	} else {
		// Remove the marker section by replacing it with nothing
		updated = InjectByMarker(string(existing), "", "biggz:strict-tdd-mode")
		// If InjectByMarker returns content with empty markers, strip them
		openMarker := "<!-- biggz:strict-tdd-mode -->"
		closeMarker := "<!-- /biggz:strict-tdd-mode -->"
		if strings.Contains(updated, openMarker) {
			updated = strings.ReplaceAll(updated, openMarker+"\n\n"+closeMarker, "")
			updated = strings.ReplaceAll(updated, openMarker+closeMarker, "")
		}
	}

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

// InjectByMarker inserts or updates managed content in a markdown file using
// HTML comment markers (<!-- name --> ... <!-- /name -->).
//
// Before injection, it runs stripOrphanMarkers to repair files that accumulated
// orphan or duplicate markers from previous runs (e.g., repeated syncs could
// leave multiple copies of the same managed block).
//
// If content has not changed since the last injection (content fingerprint match),
// the file is returned unchanged to avoid unnecessary writes.
func InjectByMarker(existing, content, name string) string {
	existing = stripOrphanMarkers(existing, name)

	openMarker := "<!-- " + name + " -->"
	closeMarker := "<!-- /" + name + " -->"

	openIdx := strings.Index(existing, openMarker)
	if openIdx == -1 {
		// No marker pair found — append at end with proper markers
		if existing != "" && !strings.HasSuffix(existing, "\n") {
			existing += "\n"
		}
		return existing + "\n" + openMarker + "\n" + content + "\n" + closeMarker + "\n"
	}

	closeIdx := strings.Index(existing[openIdx+len(openMarker):], closeMarker)
	if closeIdx == -1 {
		// Opening marker found but no closing marker — replace from open to end
		return existing[:openIdx] + openMarker + "\n" + content + "\n" + closeMarker + "\n"
	}
	closeIdx += openIdx + len(openMarker)

	// Check if content is unchanged (content fingerprint via marker)
	existingContent := existing[openIdx+len(openMarker) : closeIdx]
	if strings.TrimSpace(existingContent) == strings.TrimSpace(content) {
		return existing
	}

	// Replace content between markers
	return existing[:openIdx] + openMarker + "\n" + content + "\n" + closeMarker + existing[closeIdx+len(closeMarker):]
}

// stripOrphanMarkers removes orphan and duplicate marker pairs for the given
// name. It collapses any number of duplicate blocks into a clean state:
//
//  1. Removes closing markers that appear before their matching opening marker
//  2. Removes orphan opening markers without a matching closer
//  3. Removes orphan closing markers without a matching opener
//  4. Removes ALL content between the first opener and last closer, keeping
//     only the FIRST valid pair and discarding subsequent duplicates
func stripOrphanMarkers(input, name string) string {
	openMarker := "<!-- " + name + " -->"
	closeMarker := "<!-- /" + name + " -->"

	// Find ALL occurrences of both markers
	var openIdxs []int
	var closeIdxs []int

	for i := 0; i < len(input); {
		if idx := strings.Index(input[i:], openMarker); idx >= 0 {
			openIdxs = append(openIdxs, i+idx)
			i += idx + len(openMarker)
		} else {
			break
		}
	}

	for i := 0; i < len(input); {
		if idx := strings.Index(input[i:], closeMarker); idx >= 0 {
			closeIdxs = append(closeIdxs, i+idx)
			i += idx + len(closeMarker)
		} else {
			break
		}
	}

	// If there are 0 or 1 pairs, no cleanup needed
	if len(openIdxs) <= 1 && len(closeIdxs) <= 1 {
		// Still need to balance: remove orphans
		if len(openIdxs) == 1 && len(closeIdxs) == 0 {
			// Remove the orphan opener
			return input[:openIdxs[0]] + input[openIdxs[0]+len(openMarker):]
		}
		if len(openIdxs) == 0 && len(closeIdxs) == 1 {
			// Remove the orphan closer
			return input[:closeIdxs[0]] + input[closeIdxs[0]+len(closeMarker):]
		}
		return input
	}

	// Multiple markers found — collapse to first valid pair, discard the rest.

	// Find first valid pair: first opener followed by a closer
	firstOpen := -1
	firstClose := -1
	for _, oi := range openIdxs {
		for _, ci := range closeIdxs {
			if ci > oi {
				firstOpen = oi
				firstClose = ci
				break
			}
		}
		if firstOpen >= 0 {
			break
		}
	}

	if firstOpen < 0 {
		// No valid pair — remove all markers
		result := input
		for _, oi := range openIdxs {
			result = result[:oi] + result[oi+len(openMarker):]
		}
		for _, ci := range closeIdxs {
			result = result[:ci] + result[ci+len(closeMarker):]
		}
		return result
	}

	// Keep everything before the first opener, and content between first pair.
	// Discard everything after first closer (removes all duplicate blocks).
	return input[:firstOpen] + openMarker + input[firstOpen+len(openMarker):firstClose] + closeMarker
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
