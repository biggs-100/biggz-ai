package doctor

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/biggs-100/biggz-ai/internal/platform"
)

const PiNativeMCPCheckID CheckID = "pi-native-mcp"

// piNativeExposures are the exposure values Pi's native MCP schema accepts,
// both for a whole server (mcpServers.<name>.exposure) and for one entry of its
// tool map (mcpServers.<name>.toolExposure).
var piNativeExposures = []string{"direct", "codemode", "deferred", "hidden"}

// piNativeTransportTypes are the transport types Pi accepts for a server. An
// absent type is the native stdio form (Pi infers it from "command").
var piNativeTransportTypes = []string{"stdio", "http", "streamable-http"}

// piNativeTransportError is the message Pi prints when an entry carries no
// usable transport. Kept verbatim so the warning quotes the real Pi error.
const piNativeTransportError = `server "bigmem" needs either "command" (stdio) or "url" (streamable HTTP)`

// piNativeInstallHint is the repair every native-config warning points at.
const piNativeInstallHint = "biggz install --agent pi"

// PiNativeMCPCheck verifies Pi's native MCP contract now that the retired
// pi-mcp-adapter is gone: the adapter must be absent (while any extension
// registers /mcp, Pi ignores mcp.json and its own servers), mcp.json must
// declare a native mcpServers.bigmem entry (command resolvable, no non-native
// type, sane exposure/toolExposure, no legacy imports/directTools),
// settings.json must carry no MCP state, and biggz-mcp must answer the bounded
// liveness probes. Warns carry a machine-applicable Remedy; panic-isolated via
// Runner; read-only.
type PiNativeMCPCheck struct {
	lookPath   func(string) (string, error)
	statFn     func(string) (os.FileInfo, error)
	readFileFn func(string) ([]byte, error)
	execFn     func(string, ...string) ([]byte, error)
	getenv     func(string) string
	homeDirFn  func() (string, error)
}

// NewPiNativeMCPCheck creates a PiNativeMCPCheck using the default environment.
func NewPiNativeMCPCheck() *PiNativeMCPCheck {
	return &PiNativeMCPCheck{
		lookPath:   exec.LookPath,
		statFn:     os.Stat,
		readFileFn: os.ReadFile,
		execFn:     execCommand,
		getenv:     os.Getenv,
		homeDirFn:  os.UserHomeDir,
	}
}

// NewPiNativeMCPCheckWithCustom creates a PiNativeMCPCheck with injected functions for testing.
func NewPiNativeMCPCheckWithCustom(
	lookPath func(string) (string, error),
	statFn func(string) (os.FileInfo, error),
	readFileFn func(string) ([]byte, error),
	execFn func(string, ...string) ([]byte, error),
	getenv func(string) string,
	homeDirFn func() (string, error),
) *PiNativeMCPCheck {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if statFn == nil {
		statFn = os.Stat
	}
	if readFileFn == nil {
		readFileFn = os.ReadFile
	}
	if execFn == nil {
		execFn = execCommand
	}
	if getenv == nil {
		getenv = os.Getenv
	}
	if homeDirFn == nil {
		homeDirFn = os.UserHomeDir
	}
	return &PiNativeMCPCheck{
		lookPath:   lookPath,
		statFn:     statFn,
		readFileFn: readFileFn,
		execFn:     execFn,
		getenv:     getenv,
		homeDirFn:  homeDirFn,
	}
}

// ID returns the check identifier.
func (c *PiNativeMCPCheck) ID() CheckID { return PiNativeMCPCheckID }

// Run verifies Pi's native MCP contract.
func (c *PiNativeMCPCheck) Run(ctx context.Context) *Result {
	select {
	case <-ctx.Done():
		return &Result{ID: PiNativeMCPCheckID, Status: StatusWarn, Message: "pi-native-mcp check canceled", Severity: SeverityWarning, Error: ctx.Err().Error()}
	default:
	}
	if _, err := c.lookPath("pi"); err != nil {
		return &Result{ID: PiNativeMCPCheckID, Status: StatusPass, Message: "pi not installed — skipping pi-native-mcp check", Severity: SeverityInfo}
	}
	home, err := c.homeDirFn()
	if err != nil || strings.TrimSpace(home) == "" {
		return &Result{ID: PiNativeMCPCheckID, Status: StatusWarn, Message: "cannot determine home directory for pi-native-mcp check", Severity: SeverityWarning, Error: fmt.Sprintf("home: %v", err)}
	}
	if evidence := c.adapterInstalled(home); evidence != "" {
		return &Result{
			ID:       PiNativeMCPCheckID,
			Status:   StatusWarn,
			Severity: SeverityWarning,
			Message:  "pi-mcp-adapter is still installed — while an extension registers /mcp, Pi ignores mcp.json and its own MCP servers, so native bigmem never loads (run: pi remove npm:pi-mcp-adapter)",
			Error:    evidence,
		}
	}
	command, res := c.checkNativeConfig(home)
	if res != nil {
		return res
	}
	if res := c.checkSettingsHasNoMCPState(home); res != nil {
		return res
	}
	if res := c.checkBiggzMCPHealth(ctx, command); res != nil {
		return res
	}
	return &Result{
		ID:       PiNativeMCPCheckID,
		Status:   StatusPass,
		Severity: SeverityInfo,
		Message:  "pi-native-mcp healthy: no pi-mcp-adapter, mcp.json bigmem is native (resolvable command, no non-native type, sane exposure/toolExposure, no legacy keys), settings.json carries no MCP state, pi mcp list reports bigmem",
	}
}

// Remedy returns a repair action that removes the retired adapter and
// re-provisions the native MCP configuration (`biggz install --agent pi`).
func (c *PiNativeMCPCheck) Remedy() *Remedy {
	return &Remedy{
		ID:          string(PiNativeMCPCheckID),
		Description: "Remove the retired adapter and re-provision native MCP config (pi remove npm:pi-mcp-adapter && biggz install --agent pi)",
		Action: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			cmd := exec.CommandContext(ctx, "pi", "remove", "npm:pi-mcp-adapter")
			platform.EnsureCommandDir(cmd)
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("pi remove npm:pi-mcp-adapter: %w (output: %s)", err, strings.TrimSpace(string(out)))
			}
			return biggzInstallPi(ctx)
		},
	}
}

// piAgentDir returns Pi's agent config directory: the PI_CODING_AGENT_DIR
// override when set, otherwise <home>/.pi/agent.
func (c *PiNativeMCPCheck) piAgentDir(home string) string {
	if v := strings.TrimSpace(c.getenv("PI_CODING_AGENT_DIR")); v != "" {
		return v
	}
	return filepath.Join(home, ".pi", "agent")
}

// piAgentRoots returns the agent roots that can hold pi package state, matching
// install.PiMCPAdapterPresent: the caller's home root plus the
// PI_CODING_AGENT_DIR override when it is set.
func (c *PiNativeMCPCheck) piAgentRoots(home string) []string {
	roots := []string{filepath.Join(home, ".pi", "agent")}
	if v := strings.TrimSpace(c.getenv("PI_CODING_AGENT_DIR")); v != "" {
		roots = append(roots, v)
	}
	return roots
}

// adapterInstalled reports evidence that the retired pi-mcp-adapter extension
// is still installed, or "" when it is gone. Two signals count per agent root,
// the same pair install.PiMCPAdapterPresent uses: the package's npm directory
// and a leftover `npm:pi-mcp-adapter*` entry in that root's settings.json
// packages list.
func (c *PiNativeMCPCheck) adapterInstalled(home string) string {
	for _, root := range c.piAgentRoots(home) {
		dir := filepath.Join(root, "npm", "node_modules", "pi-mcp-adapter")
		if info, err := c.statFn(dir); err == nil && info.IsDir() {
			return fmt.Sprintf("pi-mcp-adapter npm directory present at %s", dir)
		}
		if c.settingsHasPackage(root, "npm:pi-mcp-adapter") {
			return fmt.Sprintf("settings.json packages entry npm:pi-mcp-adapter under %s", root)
		}
	}
	return ""
}

// settingsHasPackage reports whether root's settings.json packages list names a
// package spec starting with prefix. Missing or malformed settings are not
// evidence — the signal is best-effort.
func (c *PiNativeMCPCheck) settingsHasPackage(root, prefix string) bool {
	raw, err := c.readFileFn(filepath.Join(root, "settings.json"))
	if err != nil || len(raw) == 0 {
		return false
	}
	var data struct {
		Packages []any `json:"packages"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return false
	}
	for _, pkg := range data.Packages {
		if s, ok := pkg.(string); ok && strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

// checkNativeConfig validates ~/.pi/agent/mcp.json and returns the bigmem
// command on success, or a warn Result naming the first violation found.
func (c *PiNativeMCPCheck) checkNativeConfig(home string) (string, *Result) {
	mcpPath := filepath.Join(c.piAgentDir(home), "mcp.json")
	raw, err := c.readFileFn(mcpPath)
	if err != nil || len(strings.TrimSpace(string(raw))) == 0 {
		return "", &Result{
			ID:       PiNativeMCPCheckID,
			Status:   StatusWarn,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("native MCP config missing or empty at %s — native Pi has no bigmem server to load (run: %s)", mcpPath, piNativeInstallHint),
			Error:    fmt.Sprintf("read %s: %v", mcpPath, err),
		}
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", &Result{
			ID:       PiNativeMCPCheckID,
			Status:   StatusWarn,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("native MCP config malformed at %s — native Pi cannot read mcpServers.bigmem (run: %s)", mcpPath, piNativeInstallHint),
			Error:    err.Error(),
		}
	}
	// Legacy adapter-era top-level keys: native Pi ignores them, so their
	// presence means an earlier install was never converged.
	for _, key := range []string{"imports", "directTools"} {
		if _, present := obj[key]; present {
			return "", c.warnf(mcpPath, "native MCP config still carries the legacy top-level key %q — native Pi reads servers and tool exposure only from mcpServers (run: %s)", key, piNativeInstallHint)
		}
	}
	servers, _ := obj["mcpServers"].(map[string]any)
	bigmem, _ := servers["bigmem"].(map[string]any)
	if bigmem == nil {
		return "", c.warnf(mcpPath, "mcpServers.bigmem not configured — native Pi has no biggz-mcp server to load (run: %s)", piNativeInstallHint)
	}
	return c.checkBigMemEntry(mcpPath, bigmem)
}

// checkBigMemEntry validates one native bigmem server entry and returns its
// resolved command on success.
func (c *PiNativeMCPCheck) checkBigMemEntry(mcpPath string, bigmem map[string]any) (string, *Result) {
	if raw, present := bigmem["type"]; present {
		typ, isString := raw.(string)
		typ = strings.TrimSpace(typ)
		switch {
		case !isString:
			return "", c.warnf(mcpPath, "mcpServers.bigmem has a non-string \"type\" — %s", piNativeTransportError)
		case typ != "" && !slices.Contains(piNativeTransportTypes, typ):
			return "", c.warnf(mcpPath, "mcpServers.bigmem has the non-native type %q — %s", typ, piNativeTransportError)
		}
	}
	command, _ := bigmem["command"].(string)
	command = strings.TrimSpace(command)
	if command == "" {
		return "", c.warnf(mcpPath, "mcpServers.bigmem has no command — %s", piNativeTransportError)
	}
	if err := c.resolveCommand(command); err != nil {
		return "", c.warnf(mcpPath, "mcpServers.bigmem command %q is not resolvable (%v) — native Pi cannot spawn biggz-mcp (run: %s)", command, err, piNativeInstallHint)
	}
	if res := c.checkExposure(mcpPath, bigmem, "exposure"); res != nil {
		return "", res
	}
	if res := c.checkExposure(mcpPath, bigmem, "toolExposure"); res != nil {
		return "", res
	}
	return command, nil
}

// checkExposure validates one exposure field of the bigmem entry: "exposure"
// holds a single value, "toolExposure" a map of server-declared tool names to
// values. Every value must be one of Pi's exposure values.
func (c *PiNativeMCPCheck) checkExposure(mcpPath string, bigmem map[string]any, field string) *Result {
	raw, present := bigmem[field]
	if !present || raw == nil {
		return c.warnf(mcpPath, "mcpServers.bigmem is missing %q — native Pi needs it to decide the model-facing surface (run: %s)", field, piNativeInstallHint)
	}
	if field == "toolExposure" {
		tools, ok := raw.(map[string]any)
		if !ok {
			return c.warnf(mcpPath, "mcpServers.bigmem toolExposure must be an object mapping tool names to exposures (want one of %s) (run: %s)", strings.Join(piNativeExposures, "/"), piNativeInstallHint)
		}
		for name, value := range tools {
			exposure, ok := value.(string)
			if !ok || !slices.Contains(piNativeExposures, exposure) {
				return c.warnf(mcpPath, "mcpServers.bigmem toolExposure[%q] = %v is not a valid exposure (want one of %s) (run: %s)", name, value, strings.Join(piNativeExposures, "/"), piNativeInstallHint)
			}
		}
		return nil
	}
	exposure, ok := raw.(string)
	if !ok || !slices.Contains(piNativeExposures, exposure) {
		return c.warnf(mcpPath, "mcpServers.bigmem exposure = %v is not a valid exposure (want one of %s) (run: %s)", raw, strings.Join(piNativeExposures, "/"), piNativeInstallHint)
	}
	return nil
}

// resolveCommand reports whether the bigmem command exists: an absolute or
// path-qualified command is stat-ed, a bare name is resolved through PATH.
func (c *PiNativeMCPCheck) resolveCommand(command string) error {
	if filepath.IsAbs(command) || strings.ContainsAny(command, `/\`) {
		info, err := c.statFn(command)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return fmt.Errorf("%s is a directory", command)
		}
		return nil
	}
	if _, err := c.lookPath(command); err != nil {
		return err
	}
	return nil
}

// checkSettingsHasNoMCPState verifies settings.json carries no MCP state:
// native Pi reads servers only from mcp.json, so a biggz-owned
// mcpServers.bigmem entry or a top-level directTools list there is dead
// adapter-era state. Missing or unreadable settings are not evidence.
func (c *PiNativeMCPCheck) checkSettingsHasNoMCPState(home string) *Result {
	settingsPath := filepath.Join(c.piAgentDir(home), "settings.json")
	raw, err := c.readFileFn(settingsPath)
	if err != nil || len(strings.TrimSpace(string(raw))) == 0 {
		return nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	if servers, ok := obj["mcpServers"].(map[string]any); ok {
		if _, present := servers["bigmem"]; present {
			return c.warnf(settingsPath, "settings.json carries mcpServers.bigmem — native Pi reads MCP servers only from mcp.json, so this entry is dead (run: %s)", piNativeInstallHint)
		}
	}
	if _, present := obj["directTools"]; present {
		return c.warnf(settingsPath, "settings.json carries directTools — native Pi reads tool exposure from mcp.json toolExposure, so this key is dead (run: %s)", piNativeInstallHint)
	}
	return nil
}

// checkBiggzMCPHealth runs the bounded liveness probes: the biggz-mcp stdio
// probe (the binary that serves tools/list for biggz_mem_*) and the
// `pi mcp list` enumeration, which proves Pi's own client loads the server.
// A failing probe is a warn carrying BIGGZ_MCP_TIMEOUT guidance.
func (c *PiNativeMCPCheck) checkBiggzMCPHealth(ctx context.Context, command string) *Result {
	select {
	case <-ctx.Done():
		return &Result{ID: PiNativeMCPCheckID, Status: StatusWarn, Message: "biggz-mcp liveness check canceled", Severity: SeverityWarning, Error: ctx.Err().Error()}
	default:
	}
	timeout := c.mcpTimeout()
	if _, err := c.execFn(command, "--help"); err != nil {
		return &Result{
			ID:       PiNativeMCPCheckID,
			Status:   StatusWarn,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("biggz-mcp stdio crash — probe failed for %s (%v), so tools/list for biggz_mem_* cannot be served; try BIGGZ_MCP_TIMEOUT=%s or reinstall biggz-mcp", command, err, timeout),
			Error:    err.Error(),
		}
	}
	out, err := c.execFn("pi", "mcp", "list")
	if err != nil {
		return &Result{
			ID:       PiNativeMCPCheckID,
			Status:   StatusWarn,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("`pi mcp list` failed (%v) — Pi's native MCP client cannot enumerate mcp.json servers; try BIGGZ_MCP_TIMEOUT=%s, then re-run %s", err, timeout, piNativeInstallHint),
			Error:    err.Error(),
		}
	}
	if !strings.Contains(string(out), "bigmem") {
		return &Result{
			ID:       PiNativeMCPCheckID,
			Status:   StatusWarn,
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("`pi mcp list` does not report bigmem — Pi's native MCP client did not load mcpServers.bigmem (run: %s; try BIGGZ_MCP_TIMEOUT=%s)", piNativeInstallHint, timeout),
			Error:    fmt.Sprintf("pi mcp list output: %s", strings.TrimSpace(string(out))),
		}
	}
	return nil
}

// mcpTimeout returns the configured MCP timeout guidance value, defaulting to
// the 30000ms the install flow uses.
func (c *PiNativeMCPCheck) mcpTimeout() string {
	return cmp.Or(strings.TrimSpace(c.getenv("BIGGZ_MCP_TIMEOUT")), "30000")
}

// warnf builds a warning Result naming the file the violation was found in.
func (c *PiNativeMCPCheck) warnf(path, format string, args ...any) *Result {
	return &Result{
		ID:       PiNativeMCPCheckID,
		Status:   StatusWarn,
		Severity: SeverityWarning,
		Message:  fmt.Sprintf(format, args...),
		Error:    path,
	}
}
