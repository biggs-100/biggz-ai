// Package uninstall implements the uninstall command for biggz-ai.
//
// It reverses the install artifact inventory per agent (skills, prompts,
// commands, plugins, settings keys, MCP config, AGENTS.md marker sections)
// plus the shared ~/.biggz store, following gentle's per-operation
// error-collection contract: every operation is attempted, failures are
// collected per agent, and the run never aborts at the first error.
package uninstall

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/filemerge"
	"github.com/biggs-100/biggz-ai/internal/install"
	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugin"
)

// ErrCancelled is returned when the user declines the confirmation prompt.
var ErrCancelled = errors.New("uninstall cancelled")

// Config controls the uninstall behavior.
type Config struct {
	// HomeDir overrides os.UserHomeDir() for testing. When set, all paths
	// are resolved relative to this directory.
	HomeDir string

	// AgentID restricts the run to one adapter. Empty processes every
	// registered adapter and removes only artifacts that exist.
	AgentID string

	// Yes skips the confirmation prompt. Without it the CLI prompts on a
	// TTY and fails with a clear message in non-interactive contexts.
	Yes bool

	// DryRun reports exactly what would be removed without changing anything.
	DryRun bool

	// Purge additionally deletes ~/.biggz/bigmem and ~/.biggz/backups.
	Purge bool

	// Confirm overrides the confirmation input (defaults to os.Stdin).
	Confirm io.Reader
}

// AgentFailure describes one operation that failed for one agent (or the
// shared store, where Agent is "shared").
type AgentFailure struct {
	Agent string // adapter ID, or "shared"
	Op    string // operation that failed
	Err   error
}

// AgentResult summarizes the uninstall outcome for one adapter.
type AgentResult struct {
	AgentID          string
	Name             string
	RemovedFiles     int
	RewrittenConfigs int
}

// Result describes what happened during an uninstall run.
type Result struct {
	AgentResults     []AgentResult
	RemovedFiles     int
	RewrittenConfigs int
	Failed           []AgentFailure
	Summary          string
	DryRun           bool
}

// Run uninstalls biggz-ai artifacts for every registered adapter (or only
// the one selected via cfg.AgentID) and the shared ~/.biggz store. Every
// operation is attempted; failures are collected per agent. Run returns
// ErrCancelled when the confirmation prompt is declined, and an error when
// no confirmation can be obtained in a non-interactive context.
func Run(ctx context.Context, adapters map[string]plugin.AgentAdapter, cfg Config) (*Result, error) {
	if err := confirm(cfg); err != nil {
		return nil, err
	}

	home := cfg.HomeDir
	if home == "" {
		home, _ = os.UserHomeDir()
	}

	res := &Result{DryRun: cfg.DryRun}

	ids := make([]string, 0, len(adapters))
	for id := range adapters {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if cfg.AgentID != "" {
		if _, ok := adapters[cfg.AgentID]; !ok {
			return nil, fmt.Errorf("unknown agent %q", cfg.AgentID)
		}
		ids = []string{cfg.AgentID}
	}

	for _, id := range ids {
		ar := AgentResult{AgentID: id, Name: adapters[id].Name()}
		uninstallAgent(ctx, adapters[id], home, cfg, &ar, res)
		res.AgentResults = append(res.AgentResults, ar)
	}

	shared := sharedOps{cfg: cfg, home: home}
	shared.run(res)

	for _, ar := range res.AgentResults {
		res.RemovedFiles += ar.RemovedFiles
		res.RewrittenConfigs += ar.RewrittenConfigs
	}
	res.RemovedFiles += shared.removed
	res.RewrittenConfigs += shared.rewritten
	res.Summary = summarize(res, cfg)
	return res, nil
}

// confirm enforces the confirmation contract: --yes skips it, dry-run needs
// no confirmation (it changes nothing), and non-interactive contexts require
// --yes with a clear error.
func confirm(cfg Config) error {
	if cfg.Yes || cfg.DryRun {
		return nil
	}
	reader := cfg.Confirm
	if reader == nil {
		stat, _ := os.Stdin.Stat()
		if stat == nil || (stat.Mode()&os.ModeCharDevice) == 0 {
			return fmt.Errorf("uninstall requires --yes in non-interactive mode")
		}
		reader = os.Stdin
	}
	fmt.Fprint(os.Stdout, "Uninstall biggz-ai? This removes biggz-managed files from your AI agent configs and ~/.biggz. [y/N] ")
	line, err := bufio.NewReader(reader).ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) && strings.TrimSpace(line) == "" {
			// No input available (closed stdin, NUL device on Windows):
			// this is a non-interactive context, require --yes.
			return fmt.Errorf("uninstall requires --yes in non-interactive mode")
		}
		if !errors.Is(err, io.EOF) {
			return fmt.Errorf("read confirmation: %w", err)
		}
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return nil
	}
	return ErrCancelled
}

// uninstallAgent removes every biggz-owned artifact of one adapter. Failures
// are appended to res.Failed; the remaining operations still run.
func uninstallAgent(ctx context.Context, adapter plugin.AgentAdapter, home string, cfg Config, ar *AgentResult, res *Result) {
	fail := func(op string, err error) {
		res.Failed = append(res.Failed, AgentFailure{Agent: ar.AgentID, Op: op, Err: err})
	}
	removeIfExists := func(path string, op string) {
		if cfg.DryRun {
			if _, err := os.Lstat(path); err == nil {
				ar.RemovedFiles++
			}
			return
		}
		if err := os.Remove(path); err == nil {
			ar.RemovedFiles++
		} else if !os.IsNotExist(err) {
			fail(op, err)
		}
	}

	assets, err := install.AgentAssetPaths()
	if err != nil {
		fail("enumerate assets", err)
	}

	// Agent skills dir: remove exactly the files install deploys (reverse of
	// DeploySkillsToAgentDir, including the shared-skill skip rule).
	if skillsDir := adapter.SkillsDir(home); skillsDir != "" {
		for _, rel := range assets.AgentSkills {
			removeIfExists(filepath.Join(skillsDir, filepath.FromSlash(rel)), "remove skill "+rel)
		}
		sweepEmptyDirs(skillsDir)
		cleanupEmptyDirsUpTo(skillsDir, adapter.GlobalConfigDir(home))
	}

	// Prompts dir (GlobalConfigDir/prompts/sdd).
	if promptsDir := filepath.Join(adapter.GlobalConfigDir(home), "prompts", "sdd"); promptsDir != "" {
		for _, rel := range assets.Prompts {
			removeIfExists(filepath.Join(promptsDir, filepath.FromSlash(rel)), "remove prompt "+rel)
		}
		sweepEmptyDirs(promptsDir)
		cleanupEmptyDirsUpTo(promptsDir, adapter.GlobalConfigDir(home))
	}

	// Commands dir.
	if commandsDir := filepath.Join(adapter.GlobalConfigDir(home), "commands"); commandsDir != "" {
		for _, rel := range assets.Commands {
			removeIfExists(filepath.Join(commandsDir, filepath.FromSlash(rel)), "remove command "+rel)
		}
		sweepEmptyDirs(commandsDir)
		cleanupEmptyDirsUpTo(commandsDir, adapter.GlobalConfigDir(home))
	}

	// Plugins dir.
	if pluginsDir := filepath.Join(adapter.GlobalConfigDir(home), "plugins"); pluginsDir != "" {
		for _, rel := range assets.Plugins {
			removeIfExists(filepath.Join(pluginsDir, filepath.FromSlash(rel)), "remove plugin "+rel)
		}
		sweepEmptyDirs(pluginsDir)
		cleanupEmptyDirsUpTo(pluginsDir, adapter.GlobalConfigDir(home))
	}

	// Settings JSONC: remove only the biggz-owned keys, keep every other
	// byte of the file (comments, formatting, user keys) untouched.
	if settingsPath := adapter.SettingsPath(home); settingsPath != "" {
		rewriteConfigKeys(settingsPath, cfg, &ar.RewrittenConfigs, func(op string, err error) {
			fail(op, err)
		})
	}

	// MCP separate file (Claude: ~/.claude/mcp/biggz.json — biggz-owned).
	if adapter.SupportsMCP() && adapter.MCPStrategy() == model.StrategySeparateMCPFiles {
		if mcpPath := adapter.MCPConfigPath(home, "biggz"); mcpPath != "" {
			removeIfExists(mcpPath, "remove MCP config")
			cleanupEmptyDirsUpTo(filepath.Dir(mcpPath), adapter.GlobalConfigDir(home))
		}
	}

	// AGENTS.md (or equivalent): remove only the `<!-- biggz:... -->`
	// managed sections, keep the rest of the file byte-identical.
	if adapter.SupportsSystemPrompt() {
		if promptFile := adapter.SystemPromptFile(home); promptFile != "" {
			rewriteMarkdownSections(promptFile, cfg, &ar.RewrittenConfigs, func(op string, err error) {
				fail(op, err)
			})
		}
	}

	// Remove the agent config root itself when biggz emptied it.
	cleanupEmptyDirsUpTo(adapter.GlobalConfigDir(home), adapter.GlobalConfigDir(home))
	if root := adapter.GlobalConfigDir(home); root != "" && root != home {
		removeIfEmptyDir(root)
	}
}

// rewriteConfigKeys removes the biggz-owned keys from a JSONC settings file,
// rewriting it only when something actually changed. The file's permission
// bits are preserved (e.g. ~/.claude/settings.json stays 0600).
func rewriteConfigKeys(path string, cfg Config, rewritten *int, fail func(op string, err error)) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		fail("read settings "+path, err)
		return
	}

	paths := []string{"agent.biggz-orchestrator", "mcp.biggz"}
	// default_agent points at the orchestrator install created; only remove
	// it when it still references biggz (a user-chosen default must survive).
	if normalized, err := filemerge.MergeJSONC(data, []byte("{}")); err == nil {
		var m map[string]any
		if json.Unmarshal(normalized, &m) == nil {
			if m["default_agent"] == "biggz-orchestrator" {
				paths = append(paths, "default_agent")
			}
		}
	}

	merged, err := filemerge.RemoveKeysJSONC(data, paths...)
	if err != nil {
		fail("clean settings "+path, err)
		return
	}
	if string(merged) == string(data) {
		return
	}
	if cfg.DryRun {
		*rewritten++
		return
	}
	mode := os.FileMode(0644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if _, err := filemerge.WriteFileAtomic(path, merged, mode); err != nil {
		fail("write settings "+path, err)
		return
	}
	*rewritten++
}

// rewriteMarkdownSections removes every `<!-- biggz:... -->` managed section
// from the system prompt file, leaving the rest byte-identical.
func rewriteMarkdownSections(path string, cfg Config, rewritten *int, fail func(op string, err error)) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		fail("read system prompt "+path, err)
		return
	}
	stripped := stripBiggzSections(string(data))
	if stripped == string(data) {
		return
	}
	if cfg.DryRun {
		*rewritten++
		return
	}
	mode := os.FileMode(0644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if _, err := filemerge.WriteFileAtomic(path, []byte(stripped), mode); err != nil {
		fail("write system prompt "+path, err)
		return
	}
	*rewritten++
}

// stripBiggzSections removes every `<!-- biggz:<name> -->` managed section
// from content, leaving every other byte untouched. A section whose closing
// marker is missing is left intact (conservative: never eat user content).
func stripBiggzSections(content string) string {
	const openPrefix = "<!-- biggz:"
	const closePrefix = "<!-- /biggz:"

	var b strings.Builder
	b.Grow(len(content))
	rest := content
	for {
		o := strings.Index(rest, openPrefix)
		if o < 0 {
			b.WriteString(rest)
			return b.String()
		}
		nameEnd := strings.Index(rest[o+len(openPrefix):], "-->")
		if nameEnd < 0 {
			b.WriteString(rest)
			return b.String()
		}
		nameEnd += o + len(openPrefix)
		name := strings.TrimSpace(rest[o+len(openPrefix) : nameEnd])
		closer := closePrefix + name + " -->"
		c := strings.Index(rest[nameEnd+3:], closer)
		if c < 0 {
			b.WriteString(rest)
			return b.String()
		}
		c += nameEnd + 3
		b.WriteString(rest[:o])
		rest = rest[c+len(closer):]
		// Drop the section's own trailing line break, then collapse the
		// blank line that typically separated the section from its
		// neighbors (sections are injected surrounded by blank lines).
		if strings.HasPrefix(rest, "\r\n") {
			rest = rest[2:]
		} else if strings.HasPrefix(rest, "\n") {
			rest = rest[1:]
		}
		out := b.String()
		if strings.HasPrefix(rest, "\n") && strings.HasSuffix(out, "\n") {
			rest = rest[1:]
		}
	}
}

// sharedOps removes the shared ~/.biggz store artifacts exactly once,
// regardless of how many adapters were processed.
type sharedOps struct {
	cfg       Config
	home      string
	removed   int
	rewritten int
}

func (s *sharedOps) run(res *Result) {
	fail := func(op string, err error) {
		res.Failed = append(res.Failed, AgentFailure{Agent: "shared", Op: op, Err: err})
	}
	biggzDir := filepath.Join(s.home, ".biggz")

	removeAllIfExists := func(path, op string) {
		if _, err := os.Lstat(path); err != nil {
			if os.IsNotExist(err) {
				return
			}
			fail(op, err)
			return
		}
		if s.cfg.DryRun {
			s.removed++
			return
		}
		if err := os.RemoveAll(path); err != nil {
			fail(op, err)
			return
		}
		s.removed++
	}

	// The canonical skill store is entirely biggz-owned.
	removeAllIfExists(filepath.Join(biggzDir, "skills"), "remove ~/.biggz/skills")

	// biggz-owned binaries under ~/.biggz.
	removeAllIfExists(filepath.Join(biggzDir, "biggz-mcp.exe"), "remove ~/.biggz/biggz-mcp.exe")
	removeAllIfExists(filepath.Join(biggzDir, "biggz.exe"), "remove ~/.biggz/biggz.exe")

	// biggz-owned state/cache (RDD mode, model-variants cache). Remove always
	// so plain uninstall doesn't leave orphaned state; purge also clears them.
	removeAllIfExists(filepath.Join(biggzDir, "rdd-mode.json"), "remove ~/.biggz/rdd-mode.json")
	removeAllIfExists(filepath.Join(biggzDir, "cache"), "remove ~/.biggz/cache")

	// --purge: also delete the user-created bigmem store and backups.
	if s.cfg.Purge {
		removeAllIfExists(filepath.Join(biggzDir, "bigmem"), "remove ~/.biggz/bigmem")
		removeAllIfExists(filepath.Join(biggzDir, "backups"), "remove ~/.biggz/backups")
		removeAllIfExists(filepath.Join(biggzDir, "config.json"), "remove ~/.biggz/config.json")
	}

	// Remove ~/.biggz itself only when empty — bigmem, backups and
	// config.json (kept unless --purge) make it non-empty and it survives.
	removeIfEmptyDir(biggzDir)

	// Windows: remove the ~/.biggz entry from the persistent User PATH,
	// mirroring install's deploySelfToPath exactly in reverse.
	if !s.cfg.DryRun {
		if err := removeSelfFromPath(s.home); err != nil {
			fail("remove ~/.biggz from User PATH", err)
		}
	}
}

// sweepEmptyDirs removes empty subdirectories bottom-up (if-empty only),
// including nested trees such as skills/<name>/references/. The top-level
// directory itself is left for cleanupEmptyDirsUpTo.
func sweepEmptyDirs(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		child := filepath.Join(dir, e.Name())
		sweepEmptyDirs(child)
		if sub, err := os.ReadDir(child); err == nil && len(sub) == 0 {
			_ = os.Remove(child)
		}
	}
}

// cleanupEmptyDirsUpTo removes dir and its empty ancestors (if-empty only),
// stopping at stop (which is never removed here).
func cleanupEmptyDirsUpTo(dir, stop string) {
	if dir == "" {
		return
	}
	for dir != stop && dir != "" {
		if dir == filepath.Dir(dir) {
			return // filesystem root
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		_ = os.Remove(dir)
		dir = filepath.Dir(dir)
	}
}

// removeIfEmptyDir removes dir when it exists, is a directory and is empty.
func removeIfEmptyDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) > 0 {
		return
	}
	_ = os.Remove(dir)
}

// removeSelfFromPath removes the ~/.biggz entry from the persistent per-user
// PATH (Windows only), mirroring install.deploySelfToPath exactly in reverse:
// same platform guard, same real-home check, same PowerShell mechanism.
func removeSelfFromPath(home string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	realHome, _ := os.UserHomeDir()
	if realHome == "" || !strings.EqualFold(home, realHome) {
		return nil
	}
	biggzDir := filepath.Join(home, ".biggz")
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		`$p = [Environment]::GetEnvironmentVariable('Path','User');
		 $entries = ($p -split ';') | Where-Object { $_ -ne '`+biggzDir+`' };
		 [Environment]::SetEnvironmentVariable('Path', ($entries -join ';'), 'User')`)
	return cmd.Run()
}

// summarize renders the final report line. The kept list names everything
// that survives the run (unless --purge).
func summarize(res *Result, cfg Config) string {
	uninstalled := 0
	for _, ar := range res.AgentResults {
		if ar.RemovedFiles > 0 || ar.RewrittenConfigs > 0 {
			uninstalled++
		}
	}
	kept := []string{"custom-agents"}
	if !cfg.Purge {
		kept = append([]string{"bigmem", "backups"}, kept...)
	}
	if res.DryRun {
		return fmt.Sprintf("Dry-run: would uninstall %d agents, %d failed, kept: %s",
			uninstalled, len(res.Failed), strings.Join(kept, ", "))
	}
	return fmt.Sprintf("%d agents uninstalled, %d failed, kept: %s",
		uninstalled, len(res.Failed), strings.Join(kept, ", "))
}
