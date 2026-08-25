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
	"time"

	"github.com/biggs-100/biggz-ai/internal/agents"
	piadapter "github.com/biggs-100/biggz-ai/internal/agents/pi"
	"github.com/biggs-100/biggz-ai/internal/assets"
	"github.com/biggs-100/biggz-ai/internal/filemerge"
	"github.com/biggs-100/biggz-ai/internal/platform"
	"github.com/biggs-100/biggz-ai/internal/review"
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
	AgentDetected     bool   // whether the agent binary was found on PATH
	BinaryPath        string // full path to the agent binary (empty if not detected)
	SkillsDeployed    int    // number of skill files written (or would be written in dry-run)
	ConfigMerged      bool   // whether the config file was merged and written
	CommandsWritten   int    // number of command files written (or would be written in dry-run)
	PluginsDeployed   int    // number of plugin files written (or would be written in dry-run)
	PromptsDeployed   int    // number of prompt files written (or would be written in dry-run)
	MCPDeployed       bool   // whether the MCP server binary and config were deployed
	PiAgentsDeployed  int    // number of pi-native SDD agent files written (or would be written in dry-run)
	PiWebSearch       bool   // whether the pi web-search extension was deployed
	PiQuestionMouse   bool   // whether the pi question-mouse extension was deployed
	DryRun            bool   // whether this was a dry-run (no files written)
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
		skillName := filepath.Dir(relPath)
		if skillName == "_shared" || strings.HasPrefix(skillName, "_") {
			return nil
		}
		if sharedSkillNames[skillName] {
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

	// Also copy skills to the agent's skills directory so the agent's native
	// skill discovery can find them (opencode: ~/.config/opencode/skills, pi: ~/.pi/agent/skills).
	// Pi now has parity with opencode via SupportsSkills=true and SkillsDir=~/.pi/agent/skills
	// (per pi docs packages/coding-agent/docs/skills.md).
	skillsDir := adapter.SkillsDir(homeDir)
	if skillsDir == "" {
		// skip when agent has no skills dir
	} else if _, err := DeploySkillsToAgentDir(skillsDir, assets.FS, cfg.DryRun); err != nil {
		return result, fmt.Errorf("deploy skills to agent: %w", err)
	}
	// Self-heal: remove legacy _shared skill that had invalid name `_shared` (pi strict validation).
	// It was previously deployed before the frontmatter fix and now correctly skipped.
	if !cfg.DryRun {
		_ = os.RemoveAll(filepath.Join(homeDir, ".pi", "agent", "skills", "_shared"))
		_ = os.RemoveAll(filepath.Join(homeDir, ".biggz", "skills", "_shared"))
		// Also clean project-local legacy copy if present
		if cwd, err := os.Getwd(); err == nil {
			_ = os.RemoveAll(filepath.Join(cwd, "skills", "_shared"))
		}
	}

	// Deploy SDD prompts (used by OpenCode to delegate to sub-agents)
	// Pi uses native agents (~/.pi/agent/agents/), not prompts — skip for pi.
	if adapter.ID() == agents.AgentPi {
		result.PromptsDeployed = 0
	} else {
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
	}

	// Deploy config overlay with absolute paths to ~/.biggz/
	merged, err := DeployBiggzConfig(adapter, homeDir, assets.FS, cfg.DryRun)
	if err != nil {
		return result, fmt.Errorf("deploy config: %w", err)
	}
	result.ConfigMerged = merged

	// For pi, sync defaultModel/defaultProvider from last session so new
	// sessions start with the last used model, not the hardcoded kimi-k2.6.
	// This complements the biggz-last-model.js extension which does the same
	// at TUI startup; the Go sync ensures the very first install after a
	// model switch is already correct and also populates last-model.json.
	if adapter.ID() == agents.AgentPi && !cfg.DryRun {
		_ = syncPiLastModel(homeDir)
	}

	// Pi has SupportsSlashCommands=false — skip commands (0 files).
	if adapter.ID() == agents.AgentPi && !adapter.SupportsSlashCommands() {
		result.CommandsWritten = 0
	} else {
		commandsDir := filepath.Join(adapter.GlobalConfigDir(homeDir), "commands")
		written, err := DeployCommands(commandsDir, assets.FS, cfg.DryRun)
		if err != nil {
			return result, fmt.Errorf("write commands: %w", err)
		}
		result.CommandsWritten = written
	}

	// Deploy OpenCode plugins to the agent's plugin directory
	// (~/.config/opencode/plugins/ for OpenCode). Pi uses extensions (~/.pi/agent/extensions/) — skip for pi.
	if adapter.ID() == agents.AgentPi {
		result.PluginsDeployed = 0
	} else {
		pluginsDir := filepath.Join(adapter.GlobalConfigDir(homeDir), "plugins")
		pluginsDeployed, err := DeployPlugins(pluginsDir, assets.FS, cfg.DryRun)
		if err != nil {
			return result, fmt.Errorf("deploy plugins: %w", err)
		}
		result.PluginsDeployed = pluginsDeployed
	}

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

	// Write MCP config entry using the adapter's strategy — always, with fallback
	// when the binary is not next to the exe (dev `go run`).
	mcpPathForConfig := mcpBinPath
	if mcpPathForConfig == "" {
		// Fallback via pi adapter's exported resolver if available, else ~/.biggz/biggz-mcp fallback.
		if fallbacker, ok := adapter.(interface{ BiggzMCPPath() string }); ok {
			mcpPathForConfig = fallbacker.BiggzMCPPath()
		} else {
			// generic fallback: ~/.biggz/biggz-mcp(.exe) or bare name
			binName := "biggz-mcp"
			if runtime.GOOS == "windows" {
				binName = "biggz-mcp.exe"
			}
			cand := filepath.Join(homeDir, ".biggz", binName)
			if _, err := os.Stat(cand); err == nil {
				mcpPathForConfig = cand
			} else {
				mcpPathForConfig = binName
			}
		}
	}
	if err := DeployMCPConfig(adapter, homeDir, mcpPathForConfig, cfg.DryRun); err != nil {
		return result, fmt.Errorf("deploy mcp config: %w", err)
	}
	result.MCPDeployed = true

	// For pi, unify via ProvisionBigMemMCP which writes both settings.json and mcp.json
	// atomically with type:local and cleans legacy pi-subagents* entries.
	if adapter.ID() == agents.AgentPi {
		if provisioner, ok := adapter.(interface {
			ProvisionBigMemMCP(string) (bool, []string, error)
		}); ok {
			if !cfg.DryRun {
				if _, _, err := provisioner.ProvisionBigMemMCP(homeDir); err != nil {
					return result, fmt.Errorf("provision bigmem mcp: %w", err)
				}
			}
		}
	}

	// Deploy pi-native SDD agents (like gentle-pi's npm:gentle-pi subagents).
	// Pi lists sdd-apply, sdd-research, sdd-spec, etc. as native agents via
	// ~/.pi/agent/agents/*.md and /agents. This is biggz's equivalent of
	// gentle-pi's `~/.pi/agent/node_modules/gentle-pi/subagents/*` overlay
	// to `~/.pi/agent/agents/` or `~/.pi/agent/subagents/`.
	if adapter.ID() == agents.AgentPi {
		if n, err := DeployPiSubAgents(homeDir, assets.FS, cfg.DryRun); err != nil {
			return result, fmt.Errorf("deploy pi agents: %w", err)
		} else {
			result.PiAgentsDeployed = n
		}
		if _, err := DeployPiThinkingWrap(homeDir, assets.FS, cfg.DryRun); err != nil {
			return result, fmt.Errorf("deploy pi thinking wrap: %w", err)
		}
		if _, err := DeployPiLastModel(homeDir, assets.FS, cfg.DryRun); err != nil {
			return result, fmt.Errorf("deploy pi last model: %w", err)
		}
		if cfg.DryRun {
			result.PiWebSearch = true
		} else if res, err := DeployPiWebSearch(ctx, homeDir); err != nil {
			return result, fmt.Errorf("deploy pi web search: %w", err)
		} else {
			result.PiWebSearch = res.Created || res.Changed || true
			_ = res
		}
		if cfg.DryRun {
			result.PiQuestionMouse = true
		} else if res, err := DeployPiQuestionMouse(ctx, homeDir); err != nil {
			return result, fmt.Errorf("deploy pi question mouse: %w", err)
		} else {
			result.PiQuestionMouse = res.Created || res.Changed || true
			_ = res
		}
		// pi-pretty is auto-loaded via its package `pi.extensions` (npm:pi-pretty/dist/index.js).
		// Do NOT deploy a custom wrapper — it would duplicate tool registrations (read/bash/find/grep)
		// and cause "Tool conflicts" on pi startup. The FleetView itself comes from pi-subagents,
		// not pi-pretty, so pretty rendering works without a wrapper.
		// Self-heal: remove legacy conflicting wrapper left by installs before e54da84.
		if !cfg.DryRun {
			legacyWrapper := filepath.Join(piExtensionsDir(homeDir), "biggz-pi-pretty.js")
			_ = os.Remove(legacyWrapper)
		}
	}

	// Auto-install pi subagent dispatcher when pi is the target.
	// Ensures `biggz install --agent pi` provisions pi-subagents
	// (nicobailon/pi-subagents) so pi has delegate tool, not just
	// read/bash/edit/write. Without it pi cannot launch sub-agents.
	// Gentle's 9-step pi install includes pi-subagents-j0k3r; biggz uses
	// the 2-package npm install (idempotent) plus BigMem MCP separately.
	// Mouse parity for ask_user_question (biggz-question-mouse.js) is not
	// installed via `pi install`; it is deployed via file copy to
	// ~/.pi/agent/extensions/ (DeployPiQuestionMouse) so clicks select options.
	if adapter.ID() == agents.AgentPi {
		if cmds, err := adapter.InstallCommand(nil); err == nil && len(cmds) > 0 {
			if !cfg.DryRun {
				for _, cmd := range cmds {
					if len(cmd) == 0 {
						continue
					}
					c := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
					platform.EnsureCommandDir(c)
					if out, err := c.CombinedOutput(); err != nil {
						return result, fmt.Errorf("install pi subagents (%s): %w (output: %s)", strings.Join(cmd, " "), err, strings.TrimSpace(string(out)))
					}
				}
			}
		}
	}

	// Ensure biggz binary is on PATH for terminal use
	if !cfg.DryRun {
		if err := deploySelfToPath(homeDir); err != nil {
			return result, fmt.Errorf("deploy to path: %w", err)
		}
	}

	// Guarantee RDD enabled by default and clear any stale clone/worktree disables.
	// Fresh installs must come up trusted without manual `biggz rdd enable`.
	// This is idempotent and never fails the install (logs warning on error).
	if !cfg.DryRun {
		ensureRDDEnabled(homeDir)
	}

	return result, nil
}

// ensureRDDEnabled guarantees global RDD is enabled and clears stale
// clone/worktree generation overrides in the current repository.
// It is idempotent and never fails the caller; errors are logged as warnings.
//
// Clone/worktree clearing requires the actual git directories: RDDEnable with
// empty strings only writes the global file and cannot clear a stale
// `clone: disabled` generation that lives under .git/biggz/rdd-mode/.
// Detection is best-effort via `git rev-parse` and never fails the install.
//
// When cfg.HomeDir overrides the real home (tests), HOME and USERPROFILE are
// temporarily pointed at homeDir so review's globalStatePath (which uses
// os.UserHomeDir) writes under the temp home for test isolation.
func ensureRDDEnabled(homeDir string) {
	var worktreeGitDir, commonGitDir string
	if out, err := exec.Command("git", "rev-parse", "--git-dir").Output(); err == nil {
		worktreeGitDir = strings.TrimSpace(string(out))
	}
	if out, err := exec.Command("git", "rev-parse", "--git-common-dir").Output(); err == nil {
		commonGitDir = strings.TrimSpace(string(out))
	}

	// Temporarily override HOME/USERPROFILE when HomeDir is a test temp dir
	// so the global rdd-mode.json lands under the isolated home.
	var restoreHOME, restoreUSERPROFILE string
	var hadHOME, hadUSERPROFILE bool
	if homeDir != "" {
		if v, ok := os.LookupEnv("HOME"); ok {
			restoreHOME = v
			hadHOME = true
		}
		if v, ok := os.LookupEnv("USERPROFILE"); ok {
			restoreUSERPROFILE = v
			hadUSERPROFILE = true
		}
		_ = os.Setenv("HOME", homeDir)
		_ = os.Setenv("USERPROFILE", homeDir)
		defer func() {
			if hadHOME {
				_ = os.Setenv("HOME", restoreHOME)
			} else {
				_ = os.Unsetenv("HOME")
			}
			if hadUSERPROFILE {
				_ = os.Setenv("USERPROFILE", restoreUSERPROFILE)
			} else {
				_ = os.Unsetenv("USERPROFILE")
			}
		}()
	}

	if _, err := review.RDDEnable(worktreeGitDir, commonGitDir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to ensure RDD enabled: %v\n", err)
	}
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
		relPath := strings.TrimPrefix(path, "skills/")
		skillName := filepath.Dir(relPath)
		if skillName == "_shared" || strings.HasPrefix(skillName, "_") {
			return nil
		}
		data, err := fs.ReadFile(ffs, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
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
// skill registry. Pi is isolated under ~/.pi (not ~/.config/opencode) so it
// never conflicts with gentle-ai — for pi, all skills including shared ones are
// deployed to ~/.pi/agent/skills for native discovery.
func DeploySkillsToAgentDir(skillsDir string, ffs fs.FS, dryRun bool) (int, error) {
	// Pi's skills dir is isolated (~/.pi/agent/skills), no gentle-ai conflict.
	isPi := strings.Contains(skillsDir, ".pi")
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
		if skillName == "_shared" || strings.HasPrefix(skillName, "_") {
			return nil
		}
		if sharedSkillNames[skillName] && !isPi {
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
// For pi (which has no agent.biggz-orchestrator JSON), it also inlines
// biggz-orchestrator.md into the same APPEND_SYSTEM.md via <!-- biggz:orchestrator -->.
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

	// For pi, also inline the orchestrator instructions into APPEND_SYSTEM.md
	// since pi has no settings.json agent.biggz-orchestrator prompt to read.
	if adapter.ID() == agents.AgentPi {
		if orchData, err := fs.ReadFile(assets.FS, "biggz/biggz-orchestrator.md"); err == nil {
			orchContent := string(orchData)
			// Replace background policy token with live capability probe at install time.
			// This mirrors gentle-pi's renderOrchestratorPrompt which injects
			// Background subagent policy: on|off (capability: ready|absent).
			// If pi-subagents is not yet installed, render off/absent with hint.
			if strings.Contains(orchContent, "{{BIGGZ_BACKGROUND_POLICY}}") {
				bgLine := piadapter.RenderBackgroundSubagentsStatusLine(homeDir)
				orchContent = strings.ReplaceAll(orchContent, "{{BIGGZ_BACKGROUND_POLICY}}", bgLine)
			}
			updated = InjectByMarker(updated, orchContent, "biggz:orchestrator")
		}
		// Document web tools explicitly so the model never self-refuses browsing
		// based on its own training beliefs (pi has no built-in web tools; they
		// come from the biggz-web-search extension and work with any model).
		if webData, err := fs.ReadFile(assets.FS, "biggz/web-tools.md"); err == nil {
			updated = InjectByMarker(updated, string(webData), "biggz:web-tools")
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
//   - MCPConfigFile: dedicated mcp.json (Pi — ~/.pi/agent/mcp.json)
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
	case model.StrategyMCPConfigFile:
		return deployMCPConfigFile(adapter, homeDir, mcpBinaryPath, dryRun)
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

// deployMCPConfigFile writes the biggz MCP server config into the dedicated
// mcp.json file used by the MCPConfigFile strategy (Pi — ~/.pi/agent/mcp.json).
// It merges mcpServers.bigmem into the existing JSON, preserving other servers.
func deployMCPConfigFile(adapter plugin.AgentAdapter, homeDir, mcpBinaryPath string, dryRun bool) error {
	configPath := adapter.MCPConfigPath(homeDir, "bigmem")
	if configPath == "" {
		configPath = adapter.MCPConfigPath(homeDir, "")
	}
	if configPath == "" {
		return nil
	}
	var existingData []byte
	if _, err := os.Stat(configPath); err == nil {
		existingData, err = os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("read mcp config %s: %w", configPath, err)
		}
	}
	var existing map[string]any
	if len(existingData) == 0 {
		existing = map[string]any{}
	} else {
		if err := json.Unmarshal(existingData, &existing); err != nil {
			// Malformed existing file — start fresh but preserve backup via write
			existing = map[string]any{}
		}
		if existing == nil {
			existing = map[string]any{}
		}
	}
	servers, _ := existing["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers["bigmem"] = map[string]any{
		"command": mcpBinaryPath,
		"args":    []string{"--tools=agent"},
		"type":    "local",
	}
	existing["mcpServers"] = servers
	merged, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal mcp config: %w", err)
	}
	merged = append(merged, '\n')
	if dryRun {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(configPath), err)
	}
	if _, err := filemerge.WriteFileAtomic(configPath, merged, 0644); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}
	return nil
}

// piAgentsDir returns the pi-native agents directory (~/.pi/agent/agents),
// respecting PI_CODING_AGENT_DIR like pi.Adapter AgentConfigPath does.
func piAgentsDir(homeDir string) string {
	if v := strings.TrimSpace(os.Getenv("PI_CODING_AGENT_DIR")); v != "" {
		return filepath.Join(v, "agents")
	}
	return filepath.Join(homeDir, ".pi", "agent", "agents")
}

// piExtensionsDir returns the pi-native extensions directory (~/.pi/agent/extensions),
// respecting PI_CODING_AGENT_DIR like pi.Adapter AgentConfigPath does.
// Extensions are auto-discovered from ~/.pi/agent/extensions/*.ts (*.js also works)
func piExtensionsDir(homeDir string) string {
	if v := strings.TrimSpace(os.Getenv("PI_CODING_AGENT_DIR")); v != "" {
		return filepath.Join(v, "extensions")
	}
	return filepath.Join(homeDir, ".pi", "agent", "extensions")
}

// DeployPiThinkingWrap deploys the biggz thinking-wrap extension to
// ~/.pi/agent/extensions/biggz-thinking-wrap.js. It provides wrap +
// collapsible thinking (like gentle-pi single column with theme colors but
// with wrap desplegable). Pi's thinking is rendered via pi-tui Markdown at
// termWidth; this extension makes collapse via Ctrl+T discoverable and
// ensures wrap is active.
//
// It reads from ffs at pi/biggz-thinking-wrap.js (embedded via assets.FS
// all:pi) and writes atomically via filemerge.WriteFileAtomic. Dry-run
// counts without writing.
func DeployPiThinkingWrap(homeDir string, ffs fs.FS, dryRun ...bool) (bool, error) {
	isDry := len(dryRun) > 0 && dryRun[0]
	extensionsDir := piExtensionsDir(homeDir)
	targetPath := filepath.Join(extensionsDir, "biggz-thinking-wrap.js")

	// Try provided FS first, then assets.FS fallback.
	var data []byte
	var err error
	if ffs != nil {
		data, err = fs.ReadFile(ffs, "pi/biggz-thinking-wrap.js")
	}
	if err != nil || len(data) == 0 {
		data, err = fs.ReadFile(assets.FS, "pi/biggz-thinking-wrap.js")
		if err != nil {
			return false, fmt.Errorf("read pi thinking wrap asset: %w", err)
		}
	}
	if isDry {
		return true, nil
	}
	if err := os.MkdirAll(extensionsDir, 0755); err != nil {
		return false, fmt.Errorf("mkdir %s: %w", extensionsDir, err)
	}
	if _, err := filemerge.WriteFileAtomic(targetPath, data, 0644); err != nil {
		return false, fmt.Errorf("write %s: %w", targetPath, err)
	}
	return true, nil
}

// DeployPiLastModel deploys the biggz last-model extension to
// ~/.pi/agent/extensions/biggz-last-model.js. It syncs new sessions to the
// last used model (provider/modelId) so `pi` does not always start with the
// hardcoded kimi-k2.6 from settings.json. Reads from embedded FS at
// pi/biggz-last-model.js and writes atomically. Dry-run counts without writing.
func DeployPiLastModel(homeDir string, ffs fs.FS, dryRun ...bool) (bool, error) {
	isDry := len(dryRun) > 0 && dryRun[0]
	extensionsDir := piExtensionsDir(homeDir)
	targetPath := filepath.Join(extensionsDir, "biggz-last-model.js")

	var data []byte
	var err error
	if ffs != nil {
		data, err = fs.ReadFile(ffs, "pi/biggz-last-model.js")
	}
	if err != nil || len(data) == 0 {
		data, err = fs.ReadFile(assets.FS, "pi/biggz-last-model.js")
		if err != nil {
			return false, fmt.Errorf("read pi last model asset: %w", err)
		}
	}
	if isDry {
		return true, nil
	}
	if err := os.MkdirAll(extensionsDir, 0755); err != nil {
		return false, fmt.Errorf("mkdir %s: %w", extensionsDir, err)
	}
	if _, err := filemerge.WriteFileAtomic(targetPath, data, 0644); err != nil {
		return false, fmt.Errorf("write %s: %w", targetPath, err)
	}
	return true, nil
}

// piLastModelCachePath returns the pi last-model cache file path.
func piLastModelCachePath(homeDir string) string {
	return filepath.Join(piadapter.AgentConfigPath(homeDir), "last-model.json")
}

// piSessionsDir returns the pi sessions directory.
func piSessionsDir(homeDir string) string {
	return filepath.Join(piadapter.AgentConfigPath(homeDir), "sessions")
}

// findPiLastModel finds the most recent model/provider used in pi sessions.
// It checks last-model.json cache first, then scans sessions/*.jsonl by mtime
// and parses the last model_change or assistant message.
func findPiLastModel(homeDir string) (string, string, bool) {
	// Check cache first — fast path maintained by the extension and installer.
	cachePath := piLastModelCachePath(homeDir)
	if data, err := os.ReadFile(cachePath); err == nil && len(data) > 0 {
		var cached struct {
			Model    string `json:"model"`
			Provider string `json:"provider"`
		}
		if err := json.Unmarshal(data, &cached); err == nil {
			if cModel := strings.TrimSpace(cached.Model); cModel != "" {
				return cModel, strings.TrimSpace(cached.Provider), true
			}
		}
	}

	sessionsDir := piSessionsDir(homeDir)
	if _, err := os.Stat(sessionsDir); err != nil {
		return "", "", false
	}

	var latestPath string
	var latestMod time.Time
	foundAny := false
	_ = filepath.WalkDir(sessionsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".jsonl") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		mod := info.ModTime()
		if !foundAny || mod.After(latestMod) {
			latestPath = path
			latestMod = mod
			foundAny = true
		}
		return nil
	})
	if !foundAny || latestPath == "" {
		return "", "", false
	}

	data, err := os.ReadFile(latestPath)
	if err != nil || len(data) == 0 {
		return "", "", false
	}
	lines := strings.Split(string(data), "\n")
	// Scan reverse for last model_change or assistant message.
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}
		t, _ := obj["type"].(string)
		if t == "model_change" {
			provider, _ := obj["provider"].(string)
			modelID, _ := obj["modelId"].(string)
			if mid := strings.TrimSpace(modelID); mid != "" {
				return mid, strings.TrimSpace(provider), true
			}
		}
		if t == "message" {
			if msg, ok := obj["message"].(map[string]any); ok {
				role, _ := msg["role"].(string)
				if role == "assistant" {
					if m, ok := msg["model"].(string); ok {
						if mid := strings.TrimSpace(m); mid != "" {
							provider, _ := msg["provider"].(string)
							if provider == "" {
								provider, _ = msg["modelProvider"].(string)
							}
							return mid, strings.TrimSpace(provider), true
						}
					}
					if mid, ok := msg["modelId"].(string); ok {
						if trimmed := strings.TrimSpace(mid); trimmed != "" {
							if prov, ok := msg["provider"].(string); ok && strings.TrimSpace(prov) != "" {
								return trimmed, strings.TrimSpace(prov), true
							}
						}
					}
				}
			}
		}
	}
	return "", "", false
}

// syncPiLastModel updates settings.json defaultModel/defaultProvider from the
// last session's model, and refreshes last-model.json cache. Best-effort;
// errors are swallowed to not fail install.
func syncPiLastModel(homeDir string) error {
	modelID, provider, ok := findPiLastModel(homeDir)
	if !ok || strings.TrimSpace(modelID) == "" {
		return nil
	}
	modelID = strings.TrimSpace(modelID)
	provider = strings.TrimSpace(provider)

	settingsPath := piadapter.NewAdapter().SettingsPath(homeDir)
	if settingsPath == "" {
		return nil
	}
	// Read existing settings (may not exist yet).
	var existingData []byte
	if data, err := os.ReadFile(settingsPath); err == nil {
		existingData = data
	} else if !os.IsNotExist(err) {
		return nil
	}
	if len(existingData) == 0 {
		existingData = []byte("{}")
	}
	// Fast check via JSON decode to avoid unnecessary write.
	var existing map[string]any
	if err := json.Unmarshal(existingData, &existing); err != nil {
		// Try JSONC-tolerant decode via filemerge.
		if m, err2 := filemerge.UnmarshalJSONObject(existingData); err2 == nil {
			existing = m
		} else {
			existing = map[string]any{}
		}
	}
	if existing == nil {
		existing = map[string]any{}
	}
	currModel, _ := existing["defaultModel"].(string)
	currProvider, _ := existing["defaultProvider"].(string)
	if currModel == modelID && (provider == "" || currProvider == provider) {
		// Already synced — still ensure cache is fresh.
		cachePath := piLastModelCachePath(homeDir)
		if _, err := os.Stat(cachePath); os.IsNotExist(err) {
			cacheObj := map[string]any{
				"model":     modelID,
				"provider":  provider,
				"updatedAt": time.Now().UTC().Format(time.RFC3339),
			}
			if encoded, err := json.MarshalIndent(cacheObj, "", "  "); err == nil {
				encoded = append(encoded, '\n')
				_ = os.MkdirAll(filepath.Dir(cachePath), 0755)
				_, _ = filemerge.WriteFileAtomic(cachePath, encoded, 0644)
			}
		}
		return nil
	}
	// Validate model id — must be non-empty and not contain whitespace-only.
	if modelID == "" {
		return nil
	}
	overlay := map[string]any{
		"defaultModel": modelID,
	}
	if provider != "" {
		overlay["defaultProvider"] = provider
	}
	overlayBytes, err := json.Marshal(overlay)
	if err != nil {
		return nil
	}
	merged, err := filemerge.MergeJSONC(existingData, overlayBytes)
	if err != nil {
		// Fallback: direct map merge.
		for k, v := range overlay {
			existing[k] = v
		}
		if encoded, err := json.MarshalIndent(existing, "", "  "); err == nil {
			merged = append(encoded, '\n')
		} else {
			return nil
		}
	}
	// Ensure merged is indented with newline (MergeJSONC returns no newline,
	// but WriteFileAtomic handles raw bytes; add newline for consistency).
	if len(merged) > 0 && merged[len(merged)-1] != '\n' {
		merged = append(merged, '\n')
	}
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return nil
	}
	if _, err := filemerge.WriteFileAtomic(settingsPath, merged, 0644); err != nil {
		return nil
	}
	// Update cache for future startups.
	cachePath := piLastModelCachePath(homeDir)
	cacheObj := map[string]any{
		"model":     modelID,
		"provider":  provider,
		"updatedAt": time.Now().UTC().Format(time.RFC3339),
	}
	if encoded, err := json.MarshalIndent(cacheObj, "", "  "); err == nil {
		encoded = append(encoded, '\n')
		_ = os.MkdirAll(filepath.Dir(cachePath), 0755)
		_, _ = filemerge.WriteFileAtomic(cachePath, encoded, 0644)
	}
	return nil
}

// DeployPiPrettyWrapper deploys the FleetView pretty extension to
// ~/.pi/agent/extensions/biggz-pi-pretty.js. It mirrors gentle-pi's
// extensions/pi-pretty.ts: uses realpathSync + createRequire to resolve
// pnpm symlinks for @heyhuynhgiabuu/pi-pretty, then delegates
// piPrettyModule(pi, deps) so pi reports subagent_run capability ready
// and renders delegation as FleetView (multi-pane) instead of single-column
// native task fallback.
//
// Keep alongside biggz-thinking-wrap.js (Ctrl+T wrap); do not replace it.
// Reads from ffs at pi/biggz-pi-pretty.js (embedded via assets.FS all:pi)
// and writes atomically. Dry-run counts without writing.
func DeployPiPrettyWrapper(homeDir string, ffs fs.FS, dryRun ...bool) (bool, error) {
	isDry := len(dryRun) > 0 && dryRun[0]
	extensionsDir := piExtensionsDir(homeDir)
	targetPath := filepath.Join(extensionsDir, "biggz-pi-pretty.js")

	var data []byte
	var err error
	if ffs != nil {
		data, err = fs.ReadFile(ffs, "pi/biggz-pi-pretty.js")
	}
	if err != nil || len(data) == 0 {
		data, err = fs.ReadFile(assets.FS, "pi/biggz-pi-pretty.js")
		if err != nil {
			return false, fmt.Errorf("read pi pretty wrapper asset: %w", err)
		}
	}
	if isDry {
		return true, nil
	}
	if err := os.MkdirAll(extensionsDir, 0755); err != nil {
		return false, fmt.Errorf("mkdir %s: %w", extensionsDir, err)
	}
	if _, err := filemerge.WriteFileAtomic(targetPath, data, 0644); err != nil {
		return false, fmt.Errorf("write %s: %w", targetPath, err)
	}
	return true, nil
}

// DeployPiSubAgents deploys biggz SDD skills as pi-native subagents at
// ~/.pi/agent/agents/sdd-*.md, mirroring gentle-pi's model where SDD
// phases are native pi agents visible via `/agents`.
//
// It walks assets.FS for skills/sdd-*/SKILL.md, converts each SKILL.md
// frontmatter (name, description) and body (without frontmatter) into pi
// agent markdown with YAML frontmatter `name`, `description`, `tools`, and
// writes via filemerge.WriteFileAtomic. Only sdd-* skills are deployed
// (not all 47 skills), like gentle-pi's sdd chain.
//
// Dry-run is variadic to support both DeployPiSubAgents(home, fs) and
// DeployPiSubAgents(home, fs, true) call shapes; when dryRun is true it
// counts without writing. This keeps the 2-arg example in the task and
// the DryRun requirement both satisfied.
func DeployPiSubAgents(homeDir string, ffs fs.FS, dryRun ...bool) (int, error) {
	isDry := len(dryRun) > 0 && dryRun[0]
	agentsDir := piAgentsDir(homeDir)
	// Load BigMem protocol content for injection into each pi subagent.
	// Makes every sdd-* agent self-contained, not relying on APPEND_SYSTEM.md inheritance.
	protocolContent := ""
	if pData, err := fs.ReadFile(assets.FS, "biggz/bigmem-protocol.md"); err == nil {
		raw := string(pData)
		startMarker := "<!-- biggz:bigmem-protocol -->"
		endMarker := "<!-- /biggz:bigmem-protocol -->"
		if s := strings.Index(raw, startMarker); s != -1 {
			if e := strings.Index(raw[s+len(startMarker):], endMarker); e != -1 {
				protocolContent = strings.TrimSpace(raw[s+len(startMarker) : s+len(startMarker)+e])
			} else {
				protocolContent = strings.TrimSpace(raw)
			}
		} else {
			protocolContent = strings.TrimSpace(raw)
		}
		if protocolContent == "" {
			protocolContent = strings.TrimSpace(raw)
		}
	} else if pData, err := fs.ReadFile(ffs, "biggz/bigmem-protocol.md"); err == nil {
		raw := string(pData)
		startMarker := "<!-- biggz:bigmem-protocol -->"
		endMarker := "<!-- /biggz:bigmem-protocol -->"
		if s := strings.Index(raw, startMarker); s != -1 {
			if e := strings.Index(raw[s+len(startMarker):], endMarker); e != -1 {
				protocolContent = strings.TrimSpace(raw[s+len(startMarker) : s+len(startMarker)+e])
			} else {
				protocolContent = strings.TrimSpace(raw)
			}
		} else {
			protocolContent = strings.TrimSpace(raw)
		}
	}
	count := 0
	err := fs.WalkDir(ffs, "skills", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(path, "skills/")
		dir := filepath.Dir(rel)
		if !strings.HasPrefix(dir, "sdd-") {
			return nil
		}
		if filepath.Base(path) != "SKILL.md" {
			return nil
		}
		data, err := fs.ReadFile(ffs, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		name, desc, body, err := parsePiSkillFrontmatter(string(data))
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		if name == "" {
			name = dir
		}
		if desc == "" {
			desc = name + " SDD phase"
		}
		if strings.TrimSpace(body) == "" {
			body = "See ~/.biggz/skills/" + dir + "/SKILL.md for full instructions."
		}
		tools := []string{"read", "edit", "bash", "write"}
		if name == "sdd-explore" || name == "sdd-research" {
			tools = []string{"read", "grep", "find", "ls"}
		}
		if name == "sdd-research" {
			tools = append(tools, "web_search", "web_fetch")
		}
		// ask_user_question is pi's parity for opencode's `question` (grouped TUI
		// with single/multi-select + "Type something." + "Chat about this").
		tools = append(tools, "ask_user_question")
		count++
		if isDry {
			return nil
		}
		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", agentsDir, err)
		}
		escapedDesc := strings.ReplaceAll(desc, `"`, `\"`)
		// Trim newlines from description for single-line YAML
		escapedDesc = strings.ReplaceAll(escapedDesc, "\n", " ")
		var sb strings.Builder
		sb.WriteString("---\n")
		sb.WriteString(fmt.Sprintf("name: %s\n", name))
		sb.WriteString(fmt.Sprintf("description: \"%s\"\n", escapedDesc))
		sb.WriteString("tools:\n")
		for _, t := range tools {
			sb.WriteString(fmt.Sprintf("  - %s\n", t))
		}
		sb.WriteString("---\n\n")
		sb.WriteString(strings.TrimSpace(body))
		sb.WriteString("\n")
		if protocolContent != "" {
			sb.WriteString("\n<!-- biggz:bigmem-protocol -->\n")
			sb.WriteString(strings.TrimSpace(protocolContent))
			sb.WriteString("\n<!-- /biggz:bigmem-protocol -->\n")
		}
		targetPath := filepath.Join(agentsDir, name+".md")
		if _, err := filemerge.WriteFileAtomic(targetPath, []byte(sb.String()), 0644); err != nil {
			return fmt.Errorf("write %s: %w", targetPath, err)
		}
		return nil
	})
	return count, err
}

// parsePiSkillFrontmatter extracts name, description, and body from a
// biggz SKILL.md file. It handles the dual-model capable/small wrapper:
// if <!-- section:model-capable --> exists, it extracts only that section's
// frontmatter/body; otherwise it parses the whole file as a single
// frontmatter+body document.
//
// Pi's strict YAML skill loader requires frontmatter at byte 0, so SDD
// skills now have frontmatter at the top and the capable marker AFTER it
// (e.g. "---\nname: sdd-apply\n---\n<!-- section:model-capable -->\nbody").
// In that layout the markers' interior contains only the body without
// frontmatter. This parser handles both layouts:
//   - legacy: marker wraps frontmatter+body ("<!-- -->\\n---\\nname:..\\n---\\nbody")
//   - new: frontmatter at file top, marker wraps body only.
func parsePiSkillFrontmatter(data string) (string, string, string, error) {
	section := data
	hasCapable := false
	capableStart := -1
	if start := strings.Index(data, "<!-- section:model-capable -->"); start != -1 {
		capableStart = start
		if end := strings.Index(data, "<!-- /section:model-capable -->"); end != -1 && end > start {
			section = data[start+len("<!-- section:model-capable -->") : end]
			hasCapable = true
		}
	}
	lines := strings.Split(section, "\n")
	startIdx := -1
	endIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			if startIdx == -1 {
				startIdx = i
			} else {
				endIdx = i
				break
			}
		}
	}
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		if hasCapable {
			// New layout: frontmatter is at file top before the capable marker,
			// section interior has no frontmatter. Extract name/desc from top.
			prefix := data[:capableStart]
			plines := strings.Split(strings.TrimSpace(prefix), "\n")
			pStart := -1
			pEnd := -1
			for i, line := range plines {
				if strings.TrimSpace(line) == "---" {
					if pStart == -1 {
						pStart = i
					} else {
						pEnd = i
						break
					}
				}
			}
			if pStart != -1 && pEnd != -1 && pEnd > pStart {
				pFrontLines := plines[pStart+1 : pEnd]
				body := strings.TrimSpace(section)
				if idx := strings.Index(body, "<!-- section:model-small -->"); idx != -1 {
					body = strings.TrimSpace(body[:idx])
				}
				var name, desc string
				for i, line := range pFrontLines {
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "name:") {
						val := strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
						val = strings.Trim(val, "\"'")
						name = val
					} else if strings.HasPrefix(trimmed, "description:") {
						val := strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
						val = strings.Trim(val, "\"'")
						if val == ">" || val == "|" {
							var parts []string
							for j := i + 1; j < len(pFrontLines); j++ {
								next := pFrontLines[j]
								if strings.HasPrefix(next, "  ") || strings.HasPrefix(next, "\t") {
									parts = append(parts, strings.TrimSpace(next))
								} else {
									break
								}
							}
							if len(parts) > 0 {
								desc = strings.Join(parts, " ")
								desc = strings.Trim(desc, "\"'")
							}
						} else {
							desc = val
						}
					}
				}
				return name, desc, body, nil
			}
		}
		return "", "", strings.TrimSpace(section), nil
	}
	frontLines := lines[startIdx+1 : endIdx]
	bodyLines := lines[endIdx+1:]
	body := strings.TrimSpace(strings.Join(bodyLines, "\n"))
	// Remove trailing small-model section that may have leaked if we didn't
	// use the capable extraction (fallback).
	if idx := strings.Index(body, "<!-- section:model-small -->"); idx != -1 {
		body = strings.TrimSpace(body[:idx])
	}
	var name, desc string
	for i, line := range frontLines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "name:") {
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
			val = strings.Trim(val, "\"'")
			name = val
		} else if strings.HasPrefix(trimmed, "description:") {
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
			val = strings.Trim(val, "\"'")
			if val == ">" || val == "|" {
				var parts []string
				for j := i + 1; j < len(frontLines); j++ {
					next := frontLines[j]
					if strings.HasPrefix(next, "  ") || strings.HasPrefix(next, "\t") {
						parts = append(parts, strings.TrimSpace(next))
					} else {
						break
					}
				}
				if len(parts) > 0 {
					desc = strings.Join(parts, " ")
					desc = strings.Trim(desc, "\"'")
				}
			} else {
				desc = val
			}
		}
	}
	return name, desc, body, nil
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
