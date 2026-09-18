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
	"slices"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/agents"
	"github.com/biggs-100/biggz-ai/internal/filemerge"
	"github.com/biggs-100/biggz-ai/internal/sdd"
	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugin"
)

const (
	piAppendSystemFile = "APPEND_SYSTEM.md"
	piMCPConfigFile    = "mcp.json"
	piSettingsFile     = "settings.json"
	// piSubagentsJ0k3rPinned is the maintained-fork spec, exact-pinned (never
	// floating) until the own subagent-runtime cutover retires it.
	piSubagentsJ0k3rPinned = "npm:pi-subagents-j0k3r@1.6.1"
	// piPrettyPackage renders j0k3r FleetView output; preserved across the
	// cutover because the runtime keeps the pretty layer.
	piPrettyPackage = "npm:@heyhuynhgiabuu/pi-pretty"
)

var legacyPiSubagentPackageIdentities = map[string]struct{}{
	"vendor/pi-subagents":       {},
	"vendor/pi-subagents-fixed": {},
	// Predecessor dispatcher, replaced by the maintained fork below.
	// Dropping it from settings.json packages keeps pi from loading both
	// dispatchers (duplicate subagent_* tool registrations).
	"npm:pi-subagents": {},
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
// the subagent dispatcher (j0k3r-dev-rgl/pi-subagents-j0k3r, maintained fork
// of nicobailon/pi-subagents) which provides subagent_run/continue,
// background delegation, task history and model profiles. Without it pi has
// only read/bash/edit/write (user reported "No tengo disponible el mecanismo
// para lanzar sub-agentes").
// npm install is idempotent; BigMem MCP is provisioned separately via
// ProvisionBigMemMCP (biggz-mcp).
// Additionally, biggz deploys SDD skills as pi-native agents at
// ~/.pi/agent/agents/sdd-*.md (like gentle-pi's npm:gentle-pi subagents at
// ~/.pi/agent/node_modules/gentle-pi/subagents/* → ~/.pi/agent/agents/).
// This gives pi `sdd-apply`, `sdd-research`, `sdd-spec`, etc. as native
// agents visible via `/agents` and the model assignment modal.
//
// Cutover (S3): once the own biggz subagent-runtime marker is deployed
// (`internal/sdd.SubagentRuntimeMarkerPath`), the runtime owns delegation and
// the j0k3r fork is dropped from this list — installing it would dual-register
// subagent tools. Until the marker is deployed the exact pin stays, so a
// revert + reinstall restores `npm:pi-subagents-j0k3r@1.6.1`.
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
	// - npm:pi-mcp-adapter@^2 (nicobailon/pi-mcp-adapter, 761k/mo, stdio +
	//   streamable-http/sse, imports:["opencode"], directTools) is the
	//   canonical MCP client for Pi — it spawns `biggz-mcp --tools=agent
	//   --prefix=biggz` and exposes `biggz_mem_*` as native Pi tools with
	//   `/mcp` health. Pinned `^2` (v2.32.1 shape), idempotent via `pi
	//   install`, offline-tolerant (MCP JSON remains harmless).
	// - npm:pi-subagents-j0k3r@1.6.1 (exact pin, never floating) must use
	//   `pi install` (not `npm install -g`) — pi loader only scans
	//   ~/.pi/agent/npm.
	// - npm:@juicesharp/rpiv-ask-user-question provides the `ask_user_question`
	//   TUI (single/multi-select + "Type something." + "Chat about this") that
	//   is pi's parity for opencode's `question` (grouped interaction,
	//   header/label/description, custom answer row auto-appended).
	// - npm:rpiv-todo provides the visual task-tracking overlay in pi's TUI
	//   (Claude-Code parity), the same one gentle-pi ships.
	// - npm:pi-web-access provides multi-provider web search, URL/PDF
	//   extraction, video understanding and GitHub cloning (zero-config
	//   via Exa; keys optional in ~/.pi/web-search.json).
	// - npm:pi-btw provides /btw parallel side conversations in a modal
	//   overlay without interrupting the main run.
	// - Mouse parity for that TUI (biggz-question-mouse.js) is NOT installed
	//   via `pi install`; it is copied to `~/.pi/agent/extensions/` via
	//   DeployPiQuestionMouse (filemerge) during `biggz install --agent pi`.
	cmds := [][]string{
		{"pi", "install", "npm:pi-mcp-adapter@^2"},
		{"pi", "install", piSubagentsJ0k3rPinned},
		{"pi", "install", "npm:@juicesharp/rpiv-ask-user-question"},
		{"pi", "install", "npm:rpiv-todo"},
		{"pi", "install", "npm:pi-web-access"},
		{"pi", "install", "npm:pi-btw"},
	}
	// Cutover: with the own subagent-runtime marker deployed, the runtime owns
	// delegation; the fork must not be (re)installed or pi would load two
	// dispatchers registering duplicate subagent tools.
	if subagentRuntimeDeployed("") {
		cmds = slices.DeleteFunc(cmds, func(cmd []string) bool {
			return strings.Contains(strings.Join(cmd, " "), sdd.SubagentRuntimeJ0k3rMarker)
		})
	}
	return cmds, nil
}

// subagentRuntimeDeployed reports whether the own subagent-runtime marker
// (owned by internal/sdd) is deployed — the S3 cutover signal. homeDir is the
// user home used when PI_CODING_AGENT_DIR is unset; pass "" to resolve it via
// os.UserHomeDir.
func subagentRuntimeDeployed(homeDir string) bool {
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	info, err := os.Stat(sdd.SubagentRuntimeMarkerPath(homeDir))
	return err == nil && !info.IsDir()
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

	changedSettings, err := a.mergePiSettingsBigMem(settingsPath, mcpBinary, homeDir)
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
// Priority: ~/.biggz first (stable home copy written by DeployMCPBinaryToHomeDir,
// survives git clean / repo moves), then PATH, then the running binary's dir.
// The exe-dir check comes last because repo-local bin/ is gitignored and fragile.
func (a *Adapter) BiggzMCPPath() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		for _, name := range []string{"biggz-mcp", "biggz-mcp.exe"} {
			cand := filepath.Join(home, ".biggz", name)
			if _, err := os.Stat(cand); err == nil {
				return cand
			}
		}
	}
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
	if runtime.GOOS == "windows" {
		return "biggz-mcp.exe"
	}
	return "biggz-mcp"
}

func (a *Adapter) mergePiSettingsBigMem(path, mcpBinary, homeDir string) (filemerge.WriteResult, error) {
	obj, err := readPiJSONObject(path)
	if err != nil {
		return filemerge.WriteResult{}, err
	}
	// Cutover-aware desired packages: the j0k3r fork stays exact-pinned only
	// until the own subagent-runtime marker is deployed. From then on the
	// runtime owns delegation, so the fork must be absent from settings
	// packages (loading both dispatchers registers duplicate subagent tools).
	cutover := subagentRuntimeDeployed(homeDir)
	desiredPiPackages := []string{piPrettyPackage}
	if !cutover {
		desiredPiPackages = append([]string{piSubagentsJ0k3rPinned}, desiredPiPackages...)
	}
	var filtered []any
	if pkgs, ok := obj["packages"]; ok {
		filtered = filterPiPackages(pkgs)
	} else {
		filtered = []any{}
	}
	if filtered == nil {
		filtered = []any{}
	}
	// Cutover reconcile: purge every j0k3r spec shape (floating or pinned) so
	// reinstall converges an existing install to runtime-only delegation.
	if cutover {
		filtered = dropPiPackagesMatching(filtered, sdd.SubagentRuntimeJ0k3rMarker)
	}
	// Exact-pin reconcile: drop stale differently-speced copies of a pinned
	// package (e.g. floating npm:pi-subagents-j0k3r from older installs) so
	// the pin replaces them instead of coexisting with a float.
	filtered = dropSupersededPiPins(filtered, desiredPiPackages)
	// Dedupe by exact spec.
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

	// directTools — same allowlist-prune as mcp.json: drop removed BigMem
	// names, preserve foreign entries. Atomic via WriteFileAtomic below.
	obj["directTools"] = mergePiDirectTools(obj["directTools"])

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

	// imports:["opencode"] — authoritative, deduped, preserves other imports.
	// pi-mcp-adapter reuses opencode.json biggez server via imports, while
	// bigmem remains authoritative in both layers. WriteFileAtomic preserves
	// other mcpServers and ensures no partials.
	obj["imports"] = mergePiImports(obj["imports"])

	// directTools — promote BigMem tools to top-level for pi-mcp-adapter.
	// Unconditional (adapter ignores unknown); mirrors MCP spec filtering.
	obj["directTools"] = mergePiDirectTools(obj["directTools"])

	encoded, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return filemerge.WriteResult{}, fmt.Errorf("marshal pi mcp %q: %w", path, err)
	}
	encoded = append(encoded, '\n')
	return filemerge.WriteFileAtomic(path, encoded, 0o644)
}

// piDirectTools are BigMem tools promoted to top-level via pi-mcp-adapter directTools.
// Slim 11-tool allowlist (pi-footprint-slim): relieves 429 pressure while keeping
// current_project (recommended first call per bigmem-protocol). Promotion-only
// trim — cmd/biggz-mcp ProfileAgent still exposes all 20 via --tools=agent;
// the 9 removed stay callable server-side.
// biggz_ prefix from --prefix=biggz.
var piDirectTools = []string{
	"biggz_mem_save",
	"biggz_mem_search",
	"biggz_mem_get_observation",
	"biggz_mem_context",
	"biggz_mem_session_summary",
	"biggz_mem_save_prompt",
	"biggz_mem_update",
	"biggz_mem_timeline",
	"biggz_mem_review",
	"biggz_mem_judge",
	"biggz_mem_current_project",
}

// removedPiDirectTools are the 9 BigMem tools dropped from promotion by
// pi-footprint-slim. mergePiDirectTools prunes exactly these names so
// reinstall converges existing installs; all other entries are preserved.
var removedPiDirectTools = map[string]struct{}{
	"biggz_mem_capture_passive":   {},
	"biggz_mem_compare":           {},
	"biggz_mem_delete":            {},
	"biggz_mem_pin":               {},
	"biggz_mem_session_end":       {},
	"biggz_mem_session_start":     {},
	"biggz_mem_stats":             {},
	"biggz_mem_suggest_topic_key": {},
	"biggz_mem_unpin":             {},
}

func mergePiImports(existing any) []any {
	var out []any
	seen := map[string]struct{}{}
	add := func(v string) {
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	// Preserve existing imports (handle []any or []string)
	switch v := existing.(type) {
	case []any:
		for _, e := range v {
			if s, ok := e.(string); ok && s != "" {
				add(s)
			}
		}
	case []string:
		for _, s := range v {
			if s != "" {
				add(s)
			}
		}
	case string:
		if v != "" {
			add(v)
		}
	}
	// Ensure opencode authoritative
	add("opencode")
	if out == nil {
		out = []any{"opencode"}
	}
	return out
}

func mergePiDirectTools(existing any) []any {
	seen := map[string]struct{}{}
	var out []any
	add := func(v string) {
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	// Allowlist-prune: drop the 9 removed BigMem names so reinstall
	// converges existing installs to the 11-tool allowlist; preserve all
	// foreign (non-BigMem or still-allowlisted) entries.
	keep := func(s string) bool {
		if _, dropped := removedPiDirectTools[s]; dropped {
			return false
		}
		return true
	}
	switch v := existing.(type) {
	case []any:
		for _, e := range v {
			if s, ok := e.(string); ok && s != "" && keep(s) {
				add(s)
			}
		}
	case []string:
		for _, s := range v {
			if s != "" && keep(s) {
				add(s)
			}
		}
	}
	for _, want := range piDirectTools {
		add(want)
	}
	if out == nil {
		out = []any{}
		for _, want := range piDirectTools {
			out = append(out, want)
		}
	}
	return out
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

// dropSupersededPiPins removes existing entries that share a package base
// with an exact-pinned desired spec but carry a different spec (floating or
// older version), so the pin replaces them instead of coexisting with them.
// Legacy vendor identities are already dropped by filterPiPackages before
// this runs, so piPackageIdentity returns the entry's own spec here.
func dropSupersededPiPins(existing []any, desired []string) []any {
	pins := make(map[string]string, len(desired))
	for _, want := range desired {
		if base := piPackageBase(want); base != want {
			pins[base] = want
		}
	}
	if len(pins) == 0 {
		return existing
	}
	kept := make([]any, 0, len(existing))
	for _, pkg := range existing {
		spec := piPackageIdentity(pkg)
		if pin, ok := pins[piPackageBase(spec)]; ok && spec != pin {
			continue
		}
		kept = append(kept, pkg)
	}
	return kept
}

// dropPiPackagesMatching removes package entries whose spec carries the given
// fragment, mirroring the runtime's `hasJ0k3rPackage` containment check. Used
// at cutover to purge every retired-dispatcher spec shape (floating or pinned).
func dropPiPackagesMatching(existing []any, fragment string) []any {
	kept := make([]any, 0, len(existing))
	for _, pkg := range existing {
		if strings.Contains(piPackageIdentity(pkg), fragment) {
			continue
		}
		kept = append(kept, pkg)
	}
	return kept
}

// piPackageBase strips a trailing @version from a package spec so entries can
// be compared by base identity; the leading @ of a scoped package is kept.
func piPackageBase(spec string) string {
	if i := strings.LastIndex(spec, "@"); i > 0 && spec[i-1] != ':' {
		return spec[:i]
	}
	return spec
}

// Background subagent policy — 4-source fail-closed port of gentle-pi's
// resolveBackgroundSubagentsPolicy. Resolution order: project > global >
// env > default off. Malformed file fails closed to off without fallback.

const (
	backgroundPolicyOn  = "on"
	backgroundPolicyOff = "off"
)

const (
	BackgroundSubagentsSchema = "gentle-pi.background-subagents/v1"
	BackgroundSubagentsFile   = "background-subagents.json"
)

// Thin delegates to the internal/sdd owner: the exported names stay for pi
// callers and tests, but type identity lives once in sdd.
type (
	BackgroundSubagentsPolicy     = sdd.BackgroundSubagentsPolicy
	BackgroundSubagentsSource     = sdd.BackgroundSubagentsSource
	BackgroundSubagentsCapability = string
)

const (
	BackgroundSourceProject     BackgroundSubagentsSource = "project_file"
	BackgroundSourceGlobal      BackgroundSubagentsSource = "global_file"
	BackgroundSourceEnvironment BackgroundSubagentsSource = "environment"
	BackgroundSourceDefault     BackgroundSubagentsSource = "default"
)

type (
	BackgroundSubagentsResolution  = sdd.BackgroundSubagentsResolution
	LoadBackgroundSubagentsOptions = sdd.LoadBackgroundSubagentsOptions
	BackgroundSubagentsReport      = sdd.BackgroundSubagentsReport
)

func parseBackgroundSubagentsPolicyFile(raw string) (BackgroundSubagentsPolicy, bool) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return "", false
	}
	if len(parsed) != 2 {
		return "", false
	}
	schema, ok := parsed["schema"].(string)
	if !ok || schema != BackgroundSubagentsSchema {
		return "", false
	}
	policy, ok := parsed["policy"].(string)
	if !ok || (policy != backgroundPolicyOn && policy != backgroundPolicyOff) {
		return "", false
	}
	return BackgroundSubagentsPolicy(policy), true
}

func ParseBackgroundSubagentsPolicyFile(raw string) (BackgroundSubagentsPolicy, bool) {
	return parseBackgroundSubagentsPolicyFile(raw)
}

func gentleAiConfigHome() string {
	if v := strings.TrimSpace(os.Getenv("BIGGZ_CONFIG_HOME")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("GENTLE_PI_CONFIG_HOME")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("GENTLE_AI_CONFIG_HOME")); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".biggz")
}

func GentleAiConfigHome() string { return gentleAiConfigHome() }

func resolveBackgroundSubagentsPolicy(cwd string, opts LoadBackgroundSubagentsOptions) BackgroundSubagentsResolution {
	configHome := opts.GentleAiConfigHome
	if configHome == "" {
		configHome = gentleAiConfigHome()
	}
	projectFile := filepath.Join(cwd, ".biggz", BackgroundSubagentsFile)
	legacyProjectFile := filepath.Join(cwd, ".pi", "gentle-ai", BackgroundSubagentsFile)
	if _, err := os.Stat(projectFile); os.IsNotExist(err) {
		if _, err2 := os.Stat(legacyProjectFile); err2 == nil {
			projectFile = legacyProjectFile
		}
	}
	globalFile := filepath.Join(configHome, BackgroundSubagentsFile)
	projectExists := false
	globalExists := false
	if _, err := os.Stat(projectFile); err == nil {
		projectExists = true
	}
	if _, err := os.Stat(globalFile); err == nil {
		globalExists = true
	}
	envVal, envSet := lookupBackgroundEnv(opts.Env)
	var envPtr *string
	if envSet {
		envPtr = &envVal
	}
	locations := BackgroundSubagentsResolution{
		ProjectFile:       projectFile,
		GlobalFile:        globalFile,
		ProjectFileExists: projectExists,
		GlobalFileExists:  globalExists,
		EnvValue:          envPtr,
	}
	for _, entry := range []struct {
		source BackgroundSubagentsSource
		path   string
		exists bool
	}{
		{BackgroundSourceProject, projectFile, projectExists},
		{BackgroundSourceGlobal, globalFile, globalExists},
	} {
		if !entry.exists {
			continue
		}
		raw, err := os.ReadFile(entry.path)
		if err != nil {
			locations.Policy = backgroundPolicyOff
			locations.Source = entry.source
			locations.Malformed = true
			return locations
		}
		if decoded, ok := parseBackgroundSubagentsPolicyFile(string(raw)); ok {
			locations.Policy = decoded
			locations.Source = entry.source
			locations.Malformed = false
			return locations
		}
		locations.Policy = backgroundPolicyOff
		locations.Source = entry.source
		locations.Malformed = true
		return locations
	}
	if envSet && (envVal == backgroundPolicyOn || envVal == backgroundPolicyOff) {
		locations.Policy = BackgroundSubagentsPolicy(envVal)
		locations.Source = BackgroundSourceEnvironment
		locations.Malformed = false
		return locations
	}
	locations.Policy = backgroundPolicyOff
	locations.Source = BackgroundSourceDefault
	locations.Malformed = false
	return locations
}

func ResolveBackgroundSubagentsPolicy(cwd string, opts LoadBackgroundSubagentsOptions) BackgroundSubagentsResolution {
	return resolveBackgroundSubagentsPolicy(cwd, opts)
}

func loadBackgroundSubagentsPolicy(cwd string) string {
	return resolveBackgroundSubagentsPolicy(cwd, LoadBackgroundSubagentsOptions{}).Policy.String()
}

func lookupBackgroundEnv(env map[string]string) (string, bool) {
	if env != nil {
		if v, ok := env["BIGGZ_BACKGROUND_SUBAGENTS"]; ok {
			return v, true
		}
		if v, ok := env["GENTLE_PI_BACKGROUND_SUBAGENTS"]; ok {
			return v, true
		}
		return "", false
	}
	if v, ok := os.LookupEnv("BIGGZ_BACKGROUND_SUBAGENTS"); ok {
		return v, true
	}
	if v, ok := os.LookupEnv("GENTLE_PI_BACKGROUND_SUBAGENTS"); ok {
		return v, true
	}
	return "", false
}

// renderBackgroundSubagentsReport delegates to the internal/sdd owner so the
// `disabled/unmanaged` notice and the report typing render identically from
// both entry points (never a second copy to drift).
func renderBackgroundSubagentsReport(r BackgroundSubagentsResolution, capability string, wrote *BackgroundSubagentsPolicy) BackgroundSubagentsReport {
	return sdd.RenderBackgroundSubagentsReport(r, capability, wrote)
}

func RenderBackgroundSubagentsReport(r BackgroundSubagentsResolution, capability string, wrote *BackgroundSubagentsPolicy) BackgroundSubagentsReport {
	return renderBackgroundSubagentsReport(r, capability, wrote)
}

// resolveBackgroundSubagentsCapability delegates to the marker owner in
// internal/sdd: `ready` only when the deployed subagent-runtime marker is
// present AND j0k3r is absent from settings packages. Third-party
// `pi-subagents*` package presence alone must not yield `ready`.
func resolveBackgroundSubagentsCapability(homeDir string) string {
	return sdd.ResolveBackgroundSubagentsCapability(homeDir)
}

func renderBackgroundSubagentsStatusLine(homeDir string) string {
	res := resolveBackgroundSubagentsPolicy(homeDir, LoadBackgroundSubagentsOptions{})
	capability := resolveBackgroundSubagentsCapability(homeDir)
	report := renderBackgroundSubagentsReport(res, capability, nil)
	return report.Message
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
func (a *Adapter) DeployViaExtensionAPI(api interface {
	RegisterTool(def interface{}, h interface{})
}) {
	// No-op placeholder: real deploy uses extension.ExtensionAPI and
	// install.DeployPiExtensionAPI (filemerge.WriteFileAtomic + JS asset).
	_ = a
	_ = api
}
