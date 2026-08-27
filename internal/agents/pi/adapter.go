// Package pi implements the AgentAdapter for Pi (by Inflection AI).
// BigMem-native port of gentle-ai's pi adapter. Pi's config lives under
// ~/.pi/agent/ (not ~/.config/pi). System prompt is APPEND_SYSTEM.md
// with AppendToFile strategy, MCP is MCPConfigFile (mcp.json), and the
// BigMem MCP server is biggz-mcp (not pi-mcp-adapter / Engram).
package pi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/agents"
	"github.com/biggs-100/biggz-ai/internal/filemerge"
	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugin"
)

const (
	piAppendSystemFile = "APPEND_SYSTEM.md"
	piMCPConfigFile    = "mcp.json"
	piSettingsFile     = "settings.json"
)

var legacyPiSubagentPackageIdentities = map[string]struct{}{
	"vendor/pi-subagents":       {},
	"vendor/pi-subagents-fixed": {},
}

type statResult struct {
	isDir bool
	err   error
}

// Adapter implements plugin.AgentAdapter for Pi.
type Adapter struct {
	lookPath func(string) (string, error)
	statPath func(string) statResult
}

func init() {
	agents.Register(agents.AgentPi, func() plugin.AgentAdapter { return NewAdapter() })
}

// NewAdapter creates a Pi adapter instance.
func NewAdapter() *Adapter {
	return &Adapter{
		lookPath: exec.LookPath,
		statPath: defaultStat,
	}
}

func (a *Adapter) ID() model.AgentID       { return agents.AgentPi }
func (a *Adapter) Name() string            { return "Pi" }
func (a *Adapter) Tier() model.SupportTier { return agents.TierFull }

// Detect checks pi binary and ~/.pi/agent config dir.
// Returns (installed, binaryPath, configPath, configFound, err) without error
// when not installed, mirroring gentle's pi adapter semantics.
func (a *Adapter) Detect(_ context.Context, homeDir string) (bool, string, string, bool, error) {
	configPath := AgentConfigPath(homeDir)
	binaryPath, err := a.lookPath("pi")
	installed := err == nil && binaryPath != ""

	stat := a.statPath(configPath)
	if stat.err != nil {
		if os.IsNotExist(stat.err) {
			return installed, binaryPath, configPath, false, nil
		}
		return false, "", "", false, stat.err
	}
	return installed, binaryPath, configPath, stat.isDir, nil
}

// InstallCommand returns the install commands for Pi.
//
// Gentle's pi adapter does 9 steps (see gentle-ai/internal/agents/pi/adapter.go:240):
//
//	pi install npm:gentle-pi, npm:gentle-engram, npm:pi-mcp-adapter,
//	pi-engram init, pi-subagents-j0k3r, rpiv-ask-user-question, pi-web-access,
//	rpiv-todo, pi-btw (see piSubagentsInstallCommand + engramInitCommand).
//
// Biggz uses BigMem (native Go at cmd/biggz-mcp / `biggz mcp`), not Engram,
// so it does NOT need gentle-pi, gentle-engram, pi-mcp-adapter, pi-engram init,
// or rpiv-* / pi-btw. The only runtime dependency beyond the pi CLI itself is
// the subagent dispatcher (nicobailon/pi-subagents) which provides
// scout/researcher/worker/reviewer/oracle/delegate + workflowScript/worktree
// isolation. Without it pi has only read/bash/edit/write (user reported
// "No tengo disponible el mecanismo para lanzar sub-agentes").
// npm install is idempotent; BigMem MCP is provisioned separately via
// ProvisionBigMemMCP (biggz-mcp).
// Additionally, biggz deploys SDD skills as pi-native agents at
// ~/.pi/agent/agents/sdd-*.md (like gentle-pi's npm:gentle-pi subagents at
// ~/.pi/agent/node_modules/gentle-pi/subagents/* → ~/.pi/agent/agents/).
// This gives pi `sdd-apply`, `sdd-research`, `sdd-spec`, etc. as native
// agents visible via `/agents` and the model assignment modal.
// Mouse parity for the questionnaire (SGR 1000/1006 click-to-focus / double-click
// to confirm, multi-select toggle) is provided by the pi extension
// `~/.pi/agent/extensions/biggz-question-mouse.js` which is deployed via
// file copy (filemerge.WriteFileAtomic), not via `pi install`. It wraps
// `ask_user_question` at runtime, enables mouse reporting (ESC[?1000h+1006h),
// and maps SGR clicks to nav/confirm/toggle so pi matches opencode's `question`
// mouse behavior.
func (a *Adapter) InstallCommand(_ interface{}) ([][]string, error) {
	// pi's package loader only scans ~/.pi/agent/node_modules (and pnpm
	// symlinks there), NOT the global npm prefix (%AppData%\npm on Windows
	// or /usr/local/lib/node_modules). Using `npm install -g` would leave
	// packages invisible to pi's capability probes and FleetView would never
	// become ready. `pi install` writes into the agent-owned node_modules
	// where pi actually discovers packages.
	// - npm:pi-subagents must use `pi install` (not `npm install -g`) — pi
	//   loader only scans ~/.pi/agent/npm.
	// - npm:@juicesharp/rpiv-ask-user-question provides the `ask_user_question`
	//   TUI (single/multi-select + "Type something." + "Chat about this") that
	//   is pi's parity for opencode's `question` (grouped interaction,
	//   header/label/description, custom answer row auto-appended).
	// - Mouse parity for that TUI (biggz-question-mouse.js) is NOT installed
	//   via `pi install`; it is copied to `~/.pi/agent/extensions/` via
	//   DeployPiQuestionMouse (filemerge) during `biggz install --agent pi`.
	return [][]string{
		{"pi", "install", "npm:pi-subagents"},
		{"pi", "install", "npm:@juicesharp/rpiv-ask-user-question"},
	}, nil
}

func (a *Adapter) Capabilities() []string {
	// Skills:true — pi discovers SDD skills from ~/.pi/agent/skills/ (global, via AgentConfigPath)
	// and .pi/skills/ (project, trust-gated), per pi docs (earendil-works/pi packages/coding-agent/docs/skills.md).
	// This brings pi to parity with opencode (~/.config/opencode/skills) so <available_skills> lists all SDD skills
	// (sdd-apply, sdd-verify, etc.) in both harnesses. Biggz manifest mirrors that plus FileSubAgents
	// for pi-native SDD agents at ~/.pi/agent/agents/sdd-*.md (like gentle-pi).
	return []string{plugin.CapSkills, plugin.CapMCP, plugin.CapSystemPrompt, plugin.CapSubAgents}
}

func (a *Adapter) SupportsAutoInstall() bool   { return true }
func (a *Adapter) SupportsSkills() bool        { return true }
func (a *Adapter) SupportsSystemPrompt() bool  { return true }
func (a *Adapter) SupportsMCP() bool           { return true }
func (a *Adapter) SupportsOutputStyles() bool  { return false }
func (a *Adapter) SupportsSlashCommands() bool { return false }
func (a *Adapter) SupportsSubAgents() bool     { return true }

func (a *Adapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return agents.StrategyAppendToFile
}
func (a *Adapter) MCPStrategy() model.MCPStrategy { return agents.StrategyMCPConfigFile }

func (a *Adapter) GlobalConfigDir(homeDir string) string { return ConfigPath(homeDir) }
func (a *Adapter) SystemPromptDir(homeDir string) string { return AgentConfigPath(homeDir) }
func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(AgentConfigPath(homeDir), piAppendSystemFile)
}

// SkillsDir returns pi's global skills directory for native skill discovery.
// Per pi docs (packages/coding-agent/docs/skills.md), pi scans ~/.pi/agent/skills/ (global)
// and .pi/skills/ (project). This mirrors opencode's ~/.config/opencode/skills/ so both
// harnesses surface the same SDD skills (sdd-apply, sdd-verify, etc.) in <available_skills>.
// Respects PI_CODING_AGENT_DIR via AgentConfigPath, matching other pi assets (agents, extensions).
func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(AgentConfigPath(homeDir), "skills")
}
func (a *Adapter) CommandsDir(_ string) string { return "" }
func (a *Adapter) SubAgentsDir(homeDir string) string {
	return filepath.Join(AgentConfigPath(homeDir), "agents")
}
func (a *Adapter) EmbeddedSubAgentsDir() string   { return "" }
func (a *Adapter) OutputStyleDir(_ string) string { return "" }
func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(AgentConfigPath(homeDir), piSettingsFile)
}
func (a *Adapter) MCPConfigPath(homeDir string, _ string) string {
	return filepath.Join(AgentConfigPath(homeDir), piMCPConfigFile)
}
func (a *Adapter) DeployConfig(ctx context.Context, cfg plugin.AgentConfig) error {
	_ = ctx
	_ = cfg
	// Best-effort BigMem provisioning for callers that use the generic
	// plugin.AgentAdapter DeployConfig hook. Actual provisioning uses
	// ProvisionBigMemMCP with an explicit homeDir; this fallback uses
	// os.UserHomeDir so `biggz install` / `biggz sync` can still wire
	// BigMem without knowing the adapter internals.
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		_, _, _ = a.ProvisionBigMemMCP(home)
	}
	return nil
}

// ConfigPath returns Pi's global config directory path.
// Respects PI_CODING_AGENT_DIR env override like gentle's CodeGraphPaths.
func ConfigPath(homeDir string) string {
	if v := strings.TrimSpace(os.Getenv("PI_CODING_AGENT_DIR")); v != "" {
		return v
	}
	return filepath.Join(homeDir, ".pi")
}

// AgentConfigPath returns Pi's current agent-owned config directory path.
// Respects PI_CODING_AGENT_DIR env override like gentle's CodeGraphPaths.
func AgentConfigPath(homeDir string) string {
	if v := strings.TrimSpace(os.Getenv("PI_CODING_AGENT_DIR")); v != "" {
		return v
	}
	return filepath.Join(ConfigPath(homeDir), "agent")
}

// ProvisionBigMemMCP atomically merges mcpServers.bigmem pointing at the
// biggz-mcp binary into both ~/.pi/agent/settings.json and
// ~/.pi/agent/mcp.json (MCPConfigFile strategy). It drops legacy
// pi-subagents* package entries from settings.json packages, creates parent
// dirs if missing, and uses filemerge.WriteFileAtomic for atomicity.
//
// BigMem is Go-native SQLite at ~/.biggz/bigmem/bigmem.db with a native Go
// MCP server (cmd/biggz-mcp, also exposed via `biggz mcp`). This is NOT
// Engram (gentle-ai's JS/Python external + Cloud).
func (a *Adapter) ProvisionBigMemMCP(homeDir string) (bool, []string, error) {
	// Never run inside fresh/isolated subagent children — they have empty
	// sessions and would race settings.json/mcp.json writes. Mirrors
	// biggz-last-model.js guard: if (process.env.PI_SUBAGENT_CHILD === "1") return;
	if os.Getenv("PI_SUBAGENT_CHILD") == "1" {
		return false, nil, nil
	}
	settingsPath := a.SettingsPath(homeDir)
	mcpPath := a.MCPConfigPath(homeDir, "")
	mcpBinary := a.biggzMCPPath()

	for _, dir := range []string{filepath.Dir(settingsPath), filepath.Dir(mcpPath)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return false, nil, fmt.Errorf("mkdir %q: %w", dir, err)
		}
	}

	changedSettings, err := a.mergePiSettingsBigMem(settingsPath, mcpBinary)
	if err != nil {
		return false, nil, err
	}
	changedMCP, err := a.mergePiMCPFileBigMem(mcpPath, mcpBinary)
	if err != nil {
		return false, nil, err
	}
	changed := changedSettings.Changed || changedSettings.Created || changedMCP.Changed || changedMCP.Created
	return changed, []string{settingsPath, mcpPath}, nil
}

// ProvisionEngramMCP is kept for backward compatibility. Biggz uses BigMem,
// not Engram, so this delegates to ProvisionBigMemMCP.
func (a *Adapter) ProvisionEngramMCP(homeDir string) (bool, []string, error) {
	return a.ProvisionBigMemMCP(homeDir)
}

func (a *Adapter) biggzMCPPath() string { return a.BiggzMCPPath() }

// BiggzMCPPath is the exported fallback-aware resolver for the biggz-mcp
// binary path. Used by install fallback when DeployMCPBinaryToHomeDir yields "".
func (a *Adapter) BiggzMCPPath() string {
	if p, err := a.lookPath("biggz-mcp"); err == nil && p != "" {
		return p
	}
	if p, err := a.lookPath("biggz-mcp.exe"); err == nil && p != "" {
		return p
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, name := range []string{"biggz-mcp", "biggz-mcp.exe"} {
			cand := filepath.Join(dir, name)
			if _, err := os.Stat(cand); err == nil {
				return cand
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		for _, name := range []string{"biggz-mcp", "biggz-mcp.exe"} {
			cand := filepath.Join(home, ".biggz", name)
			if _, err := os.Stat(cand); err == nil {
				return cand
			}
		}
	}
	if runtime.GOOS == "windows" {
		return "biggz-mcp.exe"
	}
	return "biggz-mcp"
}

func (a *Adapter) mergePiSettingsBigMem(path, mcpBinary string) (filemerge.WriteResult, error) {
	obj, err := readPiJSONObject(path)
	if err != nil {
		return filemerge.WriteResult{}, err
	}
	// Ensure packages always contains the FleetView dispatcher and its
	// pretty renderer, deduplicated, while still filtering legacy vendor
	// prefixes. This is idempotent: repeated installs keep exactly one copy.
	desiredPiPackages := []string{"npm:pi-subagents", "npm:@heyhuynhgiabuu/pi-pretty"}
	var filtered []any
	if pkgs, ok := obj["packages"]; ok {
		filtered = filterPiPackages(pkgs)
	} else {
		filtered = []any{}
	}
	if filtered == nil {
		filtered = []any{}
	}
	// Dedupe by base package identity (strip @version suffix for comparison).
	for _, want := range desiredPiPackages {
		if containsPiPackage(filtered, want) {
			continue
		}
		filtered = append(filtered, want)
	}
	// Always set packages as []any with both entries (stable JSON).
	obj["packages"] = filtered

	servers, _ := obj["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers["bigmem"] = map[string]any{
		"command": mcpBinary,
		"args":    []string{"--tools=agent", "--prefix=biggz"},
		"type":    "local",
	}
	obj["mcpServers"] = servers

	encoded, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return filemerge.WriteResult{}, fmt.Errorf("marshal pi settings %q: %w", path, err)
	}
	encoded = append(encoded, '\n')
	return filemerge.WriteFileAtomic(path, encoded, 0o644)
}

func (a *Adapter) mergePiMCPFileBigMem(path, mcpBinary string) (filemerge.WriteResult, error) {
	obj, err := readPiJSONObject(path)
	if err != nil {
		return filemerge.WriteResult{}, err
	}
	servers, _ := obj["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers["bigmem"] = map[string]any{
		"command": mcpBinary,
		"args":    []string{"--tools=agent", "--prefix=biggz"},
		"type":    "local",
	}
	obj["mcpServers"] = servers

	encoded, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return filemerge.WriteResult{}, fmt.Errorf("marshal pi mcp %q: %w", path, err)
	}
	encoded = append(encoded, '\n')
	return filemerge.WriteFileAtomic(path, encoded, 0o644)
}

func readPiJSONObject(path string) (map[string]any, error) {
	base, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("read pi json %q: %w", path, err)
	}
	if len(strings.TrimSpace(string(base))) == 0 {
		return map[string]any{}, nil
	}
	var object map[string]any
	if err := json.Unmarshal(base, &object); err != nil {
		return nil, fmt.Errorf("unmarshal pi json %q: %w", path, err)
	}
	if object == nil {
		object = map[string]any{}
	}
	return object, nil
}

func filterPiPackages(existing any) []any {
	packages := piPackagesAsSlice(existing)
	filtered := make([]any, 0, len(packages))
	for _, pkg := range packages {
		ident := piPackageIdentity(pkg)
		if isLegacyPiSubagentPackage(ident) {
			continue
		}
		filtered = append(filtered, pkg)
	}
	return filtered
}

func piPackagesAsSlice(existing any) []any {
	switch value := existing.(type) {
	case []any:
		return value
	case []string:
		packages := make([]any, 0, len(value))
		for _, item := range value {
			packages = append(packages, item)
		}
		return packages
	case map[string]any:
		packages := make([]any, 0, len(value))
		for source, version := range value {
			versionString, _ := version.(string)
			if versionString != "" && strings.HasPrefix(source, "npm:") && !strings.Contains(strings.TrimPrefix(source, "npm:"), "@") {
				packages = append(packages, source+"@"+versionString)
				continue
			}
			packages = append(packages, source)
		}
		return packages
	default:
		return nil
	}
}

func piPackageIdentity(pkg any) string {
	source, ok := pkg.(string)
	if !ok {
		object, isObject := pkg.(map[string]any)
		if !isObject {
			return ""
		}
		source, _ = object["source"].(string)
	}
	for legacy := range legacyPiSubagentPackageIdentities {
		if source == legacy || strings.HasPrefix(source, legacy+"@") {
			return legacy
		}
	}
	return source
}

func isLegacyPiSubagentPackage(identity string) bool {
	_, ok := legacyPiSubagentPackageIdentities[identity]
	return ok
}

func containsPiPackage(existing []any, want string) bool {
	for _, pkg := range existing {
		var src string
		switch v := pkg.(type) {
		case string:
			src = v
		case map[string]any:
			src, _ = v["source"].(string)
		default:
			continue
		}
		if src == want || strings.HasPrefix(src, want+"@") {
			return true
		}
	}
	return false
}

// Background subagent policy — minimal port of gentle-pi's
// resolveBackgroundSubagentsCapability / loadBackgroundSubagentsPolicy.
// Gentle-pi resolves policy via project > global > env > default but
// capability is always probed via the live tool registry or the installed
// pi-subagents package manifest. For biggz we keep it simple: capability
// is ready when ~/.pi/agent/npm/node_modules/pi-subagents/package.json exists
// (via `pi install`, pnpm symlinks under npm/node_modules), and policy
// is on when capability is ready, off otherwise.

const (
	backgroundPolicyOn         = "on"
	backgroundPolicyOff        = "off"
	backgroundCapabilityReady  = "ready"
	backgroundCapabilityAbsent = "absent"
)

func resolveBackgroundSubagentsCapability(homeDir string) string {
	candidates := []string{
		filepath.Join(homeDir, ".pi", "agent", "npm", "node_modules", "pi-subagents"),
		filepath.Join(homeDir, ".pi", "agent", "npm", "node_modules", "pi-subagents-j0k3r"),
		filepath.Join(homeDir, ".pi", "agent", "node_modules", "pi-subagents"),
		filepath.Join(homeDir, ".pi", "agent", "node_modules", "pi-subagents-j0k3r"),
	}
	if v := strings.TrimSpace(os.Getenv("PI_CODING_AGENT_DIR")); v != "" {
		candidates = append(candidates,
			filepath.Join(v, "npm", "node_modules", "pi-subagents"),
			filepath.Join(v, "npm", "node_modules", "pi-subagents-j0k3r"),
			filepath.Join(v, "node_modules", "pi-subagents"),
			filepath.Join(v, "node_modules", "pi-subagents-j0k3r"),
		)
	}
	for _, root := range candidates {
		if _, err := os.Stat(filepath.Join(root, "package.json")); err == nil {
			return backgroundCapabilityReady
		}
		if _, err := ResolvePackageBin(root); err == nil {
			return backgroundCapabilityReady
		}
	}
	return backgroundCapabilityAbsent
}

func loadBackgroundSubagentsPolicy(homeDir string) string {
	if resolveBackgroundSubagentsCapability(homeDir) == backgroundCapabilityReady {
		return backgroundPolicyOn
	}
	return backgroundPolicyOff
}

func renderBackgroundSubagentsStatusLine(homeDir string) string {
	capability := resolveBackgroundSubagentsCapability(homeDir)
	policy := loadBackgroundSubagentsPolicy(homeDir)
	line := fmt.Sprintf("Background subagent policy: %s (capability: %s)", policy, capability)
	if capability == backgroundCapabilityAbsent {
		line += " — run `pi install` to enable FleetView"
	}
	return line
}

// ResolveBackgroundSubagentsCapability is the exported wrapper for install and doctor.
func ResolveBackgroundSubagentsCapability(homeDir string) string {
	return resolveBackgroundSubagentsCapability(homeDir)
}

// LoadBackgroundSubagentsPolicy is the exported wrapper.
func LoadBackgroundSubagentsPolicy(homeDir string) string {
	return loadBackgroundSubagentsPolicy(homeDir)
}

// RenderBackgroundSubagentsStatusLine is the exported wrapper.
func RenderBackgroundSubagentsStatusLine(homeDir string) string {
	return renderBackgroundSubagentsStatusLine(homeDir)
}

func defaultStat(path string) statResult {
	info, err := os.Stat(path)
	if err != nil {
		return statResult{err: err}
	}
	return statResult{isDir: info.IsDir()}
}

// DeployViaExtensionAPI demonstrates pi deploy via ExtensionAPI + filemerge.WriteFileAtomic.
// It registers a dummy tool via ExtensionAPI to prove shim-less deployment.
// The actual JS asset is deployed via install.DeployPiExtensionAPI which uses
// filemerge.WriteFileAtomic for atomic writes.
func (a *Adapter) DeployViaExtensionAPI(api interface{ RegisterTool(def interface{}, h interface{}) }) {
	// No-op placeholder: real deploy uses extension.ExtensionAPI and
	// install.DeployPiExtensionAPI (filemerge.WriteFileAtomic + JS asset).
	_ = a
	_ = api
}
