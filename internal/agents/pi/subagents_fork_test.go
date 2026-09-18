package pi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInstallCommand_UsesJ0k3rFork ensures pi installs the maintained
// j0k3r fork as its subagent dispatcher while the own runtime marker is NOT
// deployed, exact-pinned to @1.6.1, never the predecessor package and never
// floating. Pre-cutover path: the pin stays until the runtime is deployed.
func TestInstallCommand_UsesJ0k3rFork(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir()) // no runtime marker here
	a := NewAdapter()
	cmds, err := a.InstallCommand(nil)
	if err != nil {
		t.Fatalf("InstallCommand: %v", err)
	}
	foundFork := false
	for _, cmd := range cmds {
		joined := strings.Join(cmd, " ")
		if joined == "pi install npm:pi-subagents" {
			t.Errorf("InstallCommand installs predecessor package: %q", joined)
		}
		if joined == "pi install npm:pi-subagents-j0k3r" {
			t.Errorf("InstallCommand installs unpinned fork (must be exact @1.6.1): %q", joined)
		}
		if joined == "pi install npm:pi-subagents-j0k3r@1.6.1" {
			foundFork = true
		}
	}
	if !foundFork {
		t.Errorf("InstallCommand missing pinned fork install, got %v", cmds)
	}
}

// TestInstallCommand_CutoverDropsJ0k3r ensures that once the own
// subagent-runtime marker is deployed, the fork is dropped from InstallCommand
// (no dual registration) while the rest of the install list stays intact.
func TestInstallCommand_CutoverDropsJ0k3r(t *testing.T) {
	agentDir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", agentDir)
	writeSubagentRuntimeMarker(t, agentDir)
	a := NewAdapter()
	cmds, err := a.InstallCommand(nil)
	if err != nil {
		t.Fatalf("InstallCommand: %v", err)
	}
	joined := make([]string, 0, len(cmds))
	for _, cmd := range cmds {
		line := strings.Join(cmd, " ")
		if strings.Contains(line, "pi-subagents-j0k3r") {
			t.Errorf("cutover InstallCommand must not install the fork, got %q", line)
		}
		joined = append(joined, line)
	}
	for _, want := range []string{"pi install npm:pi-mcp-adapter@^2", "pi install npm:rpiv-todo", "pi install npm:pi-web-access", "pi install npm:pi-btw"} {
		found := false
		for _, got := range joined {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("cutover InstallCommand missing %q, got %v", want, joined)
		}
	}
	// Idempotent second call: same list, no fork.
	cmds2, _ := a.InstallCommand(nil)
	if len(cmds) != len(cmds2) {
		t.Error("cutover InstallCommand not idempotent length mismatch")
	}
}

// TestInstallCommand_IncludesTodoOverlay ensures pi installs the rpiv-todo
// visual task-tracking overlay (gentle-pi parity).
func TestInstallCommand_IncludesTodoOverlay(t *testing.T) {
	a := NewAdapter()
	cmds, err := a.InstallCommand(nil)
	if err != nil {
		t.Fatalf("InstallCommand: %v", err)
	}
	for _, cmd := range cmds {
		if strings.Join(cmd, " ") == "pi install npm:rpiv-todo" {
			return
		}
	}
	t.Errorf("InstallCommand missing todo overlay install, got %v", cmds)
}

// TestInstallCommand_IncludesWebAndBtw ensures pi installs the web-access
// capabilities and the /btw side-conversation channel (gentle-pi parity).
func TestInstallCommand_IncludesWebAndBtw(t *testing.T) {
	a := NewAdapter()
	cmds, err := a.InstallCommand(nil)
	if err != nil {
		t.Fatalf("InstallCommand: %v", err)
	}
	joined := make([]string, 0, len(cmds))
	for _, cmd := range cmds {
		joined = append(joined, strings.Join(cmd, " "))
	}
	for _, want := range []string{"pi install npm:pi-web-access", "pi install npm:pi-btw"} {
		found := false
		for _, got := range joined {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("InstallCommand missing %q, got %v", want, joined)
		}
	}
}

// TestFilterPiPackages_DropsPredecessor ensures settings.json reconciliation
// drops the predecessor dispatcher entry while keeping the fork, so pi never
// loads both (duplicate subagent_* tool registrations).
func TestFilterPiPackages_DropsPredecessor(t *testing.T) {
	in := []any{
		"npm:pi-subagents",
		"npm:pi-subagents@0.65.0",
		"npm:pi-subagents-j0k3r",
		"npm:@heyhuynhgiabuu/pi-pretty",
	}
	got := filterPiPackages(in)
	for _, pkg := range got {
		if s, _ := pkg.(string); s == "npm:pi-subagents" || s == "npm:pi-subagents@0.65.0" {
			t.Errorf("predecessor entry survived filter: %q", s)
		}
	}
	joined := make([]string, 0, len(got))
	for _, pkg := range got {
		if s, ok := pkg.(string); ok {
			joined = append(joined, s)
		}
	}
	for _, want := range []string{"npm:pi-subagents-j0k3r", "npm:@heyhuynhgiabuu/pi-pretty"} {
		if !containsPiPackage(got, want) {
			t.Errorf("expected %q to survive filter, got %v", want, joined)
		}
	}
}

// TestSettingsReconcile_PinsJ0k3rFork ensures settings reconciliation in
// ProvisionBigMemMCP replaces floating/older j0k3r entries with the exact
// @1.6.1 pin (never floating), preserves the pretty renderer, and stays
// idempotent on re-run.
func TestSettingsReconcile_PinsJ0k3rFork(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "")
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	agentDir := filepath.Join(home, ".pi", "agent")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	settingsPath := filepath.Join(agentDir, "settings.json")
	seed := `{"packages":["npm:pi-subagents-j0k3r","npm:pi-subagents-j0k3r@1.6.0","npm:@heyhuynhgiabuu/pi-pretty"]}`
	if err := os.WriteFile(settingsPath, []byte(seed), 0o644); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	a := NewAdapter()
	if _, _, err := a.ProvisionBigMemMCP(home); err != nil {
		t.Fatalf("ProvisionBigMemMCP: %v", err)
	}
	pkgs := readSettingsPackages(t, settingsPath)
	assertPinnedJ0k3rOnly(t, pkgs)

	// Idempotent second run: no duplication, pin intact.
	if _, _, err := a.ProvisionBigMemMCP(home); err != nil {
		t.Fatalf("second ProvisionBigMemMCP: %v", err)
	}
	again := readSettingsPackages(t, settingsPath)
	if len(again) != len(pkgs) {
		t.Fatalf("second run changed package count: %v -> %v", pkgs, again)
	}
	assertPinnedJ0k3rOnly(t, again)
}

// TestSettingsReconcile_CutoverDropsJ0k3r ensures that with the runtime marker
// deployed, ProvisionBigMemMCP purges every j0k3r spec from settings packages
// (no dual registration) while preserving the pretty renderer, stays
// idempotent, and that reverting the cutover (marker removed) restores the
// exact pin on the next install (rollback scenario).
func TestSettingsReconcile_CutoverDropsJ0k3r(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "")
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	agentDir := filepath.Join(home, ".pi", "agent")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	settingsPath := filepath.Join(agentDir, "settings.json")
	seed := `{"packages":["npm:pi-subagents-j0k3r@1.6.1","npm:pi-subagents-j0k3r","npm:@heyhuynhgiabuu/pi-pretty"]}`
	if err := os.WriteFile(settingsPath, []byte(seed), 0o644); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	writeSubagentRuntimeMarker(t, agentDir)

	a := NewAdapter()
	if _, _, err := a.ProvisionBigMemMCP(home); err != nil {
		t.Fatalf("ProvisionBigMemMCP: %v", err)
	}
	pkgs := readSettingsPackages(t, settingsPath)
	assertNoJ0k3r(t, pkgs)

	// Idempotent second run: no duplication, still fork-free.
	if _, _, err := a.ProvisionBigMemMCP(home); err != nil {
		t.Fatalf("second ProvisionBigMemMCP: %v", err)
	}
	again := readSettingsPackages(t, settingsPath)
	if len(again) != len(pkgs) {
		t.Fatalf("second run changed package count: %v -> %v", pkgs, again)
	}
	assertNoJ0k3r(t, again)

	// Rollback: remove the marker and re-run — the exact pin is restored.
	if err := os.Remove(filepath.Join(agentDir, "extensions", "biggz-subagent-runtime.js")); err != nil {
		t.Fatalf("remove marker: %v", err)
	}
	if _, _, err := a.ProvisionBigMemMCP(home); err != nil {
		t.Fatalf("rollback ProvisionBigMemMCP: %v", err)
	}
	assertPinnedJ0k3rOnly(t, readSettingsPackages(t, settingsPath))
}

// TestResolveBackgroundSubagentsCapability_DelegatesToMarkerOwner ensures the
// adapter probe follows the marker owner: marker present → ready, no marker →
// absent (third-party package presence alone never yields ready).
func TestResolveBackgroundSubagentsCapability_DelegatesToMarkerOwner(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	if got := ResolveBackgroundSubagentsCapability(home); got != "absent" {
		t.Fatalf("capability = %q, want absent without marker", got)
	}
	writeSubagentRuntimeMarker(t, filepath.Join(home, ".pi", "agent"))
	if got := ResolveBackgroundSubagentsCapability(home); got != "ready" {
		t.Fatalf("capability = %q, want ready with marker", got)
	}
}

// writeSubagentRuntimeMarker deploys the own subagent-runtime marker under
// <agentDir>/extensions/ — the cutover signal consulted by install/reconcile.
func writeSubagentRuntimeMarker(t *testing.T, agentDir string) {
	t.Helper()
	extDir := filepath.Join(agentDir, "extensions")
	if err := os.MkdirAll(extDir, 0o755); err != nil {
		t.Fatalf("mkdir extensions: %v", err)
	}
	marker := filepath.Join(extDir, "biggz-subagent-runtime.js")
	if err := os.WriteFile(marker, []byte("export default function biggzSubagentRuntime(pi) {}\n"), 0o644); err != nil {
		t.Fatalf("write runtime marker: %v", err)
	}
}

func readSettingsPackages(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var obj struct {
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}
	return obj.Packages
}

func assertPinnedJ0k3rOnly(t *testing.T, pkgs []string) {
	t.Helper()
	pinned, stale, pretty := 0, 0, false
	for _, p := range pkgs {
		switch p {
		case "npm:pi-subagents-j0k3r@1.6.1":
			pinned++
		case "npm:pi-subagents-j0k3r", "npm:pi-subagents-j0k3r@1.6.0":
			stale++
		case "npm:@heyhuynhgiabuu/pi-pretty":
			pretty = true
		}
	}
	if pinned != 1 {
		t.Errorf("expected exactly one pinned j0k3r entry, got %d in %v", pinned, pkgs)
	}
	if stale != 0 {
		t.Errorf("floating/older j0k3r entry survived pin reconcile: %v", pkgs)
	}
	if !pretty {
		t.Errorf("pi-pretty entry lost during reconcile: %v", pkgs)
	}
}

func assertNoJ0k3r(t *testing.T, pkgs []string) {
	t.Helper()
	pretty := false
	for _, p := range pkgs {
		if strings.Contains(p, "pi-subagents-j0k3r") {
			t.Errorf("cutover reconcile left j0k3r entry: %q in %v", p, pkgs)
		}
		if p == "npm:@heyhuynhgiabuu/pi-pretty" {
			pretty = true
		}
	}
	if !pretty {
		t.Errorf("pi-pretty entry lost during cutover reconcile: %v", pkgs)
	}
}
