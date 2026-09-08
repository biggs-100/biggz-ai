package pi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePolicyFile(t *testing.T, path, policy string) {
	t.Helper()
	content := `{"schema":"gentle-pi.background-subagents/v1","policy":"` + policy + `"}` + "\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseBackgroundSubagentsPolicyFile(t *testing.T) {
	if p, ok := ParseBackgroundSubagentsPolicyFile(`{"schema":"gentle-pi.background-subagents/v1","policy":"on"}`); !ok || p != "on" {
		t.Fatalf("expected on, got %q ok=%v", p, ok)
	}
	if _, ok := ParseBackgroundSubagentsPolicyFile(`{"schema":"wrong","policy":"on"}`); ok {
		t.Fatal("wrong schema should fail")
	}
	if _, ok := ParseBackgroundSubagentsPolicyFile(`{"schema":"gentle-pi.background-subagents/v1","policy":"on","extra":1}`); ok {
		t.Fatal("extra keys should fail")
	}
	if _, ok := ParseBackgroundSubagentsPolicyFile(`not json`); ok {
		t.Fatal("malformed json should fail")
	}
}

func TestResolveBackgroundSubagentsPolicy_ProjectOverrides(t *testing.T) {
	cwd := t.TempDir()
	cfg := t.TempDir()
	writePolicyFile(t, filepath.Join(cwd, ".pi", "gentle-ai", BackgroundSubagentsFile), "on")
	writePolicyFile(t, filepath.Join(cfg, BackgroundSubagentsFile), "off")
	res := ResolveBackgroundSubagentsPolicy(cwd, LoadBackgroundSubagentsOptions{GentleAiConfigHome: cfg, Env: map[string]string{"GENTLE_PI_BACKGROUND_SUBAGENTS": "on"}})
	if res.Policy != "on" || res.Source != BackgroundSourceProject {
		t.Fatalf("expected project on, got %q %q", res.Policy, res.Source)
	}
	if res.Malformed {
		t.Fatal("should not be malformed")
	}
}

func TestResolveBackgroundSubagentsPolicy_MalformedFailsClosed(t *testing.T) {
	cwd := t.TempDir()
	cfg := t.TempDir()
	p := filepath.Join(cwd, ".pi", "gentle-ai", BackgroundSubagentsFile)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{ malformed`), 0o644); err != nil {
		t.Fatal(err)
	}
	writePolicyFile(t, filepath.Join(cfg, BackgroundSubagentsFile), "on")
	res := ResolveBackgroundSubagentsPolicy(cwd, LoadBackgroundSubagentsOptions{GentleAiConfigHome: cfg, Env: map[string]string{"GENTLE_PI_BACKGROUND_SUBAGENTS": "on"}})
	if res.Policy != "off" {
		t.Fatalf("malformed should fail closed to off, got %q", res.Policy)
	}
	if !res.Malformed || res.Source != BackgroundSourceProject {
		t.Fatalf("expected malformed project_file, got malformed=%v source=%q", res.Malformed, res.Source)
	}
}

func TestResolveBackgroundSubagentsPolicy_GlobalOverridesEnv(t *testing.T) {
	cwd := t.TempDir()
	cfg := t.TempDir()
	writePolicyFile(t, filepath.Join(cfg, BackgroundSubagentsFile), "off")
	res := ResolveBackgroundSubagentsPolicy(cwd, LoadBackgroundSubagentsOptions{GentleAiConfigHome: cfg, Env: map[string]string{"GENTLE_PI_BACKGROUND_SUBAGENTS": "on"}})
	if res.Policy != "off" || res.Source != BackgroundSourceGlobal {
		t.Fatalf("expected global off, got %q %q", res.Policy, res.Source)
	}
}

func TestResolveBackgroundSubagentsPolicy_EnvFallbackAndDefault(t *testing.T) {
	cwd := t.TempDir()
	cfg := t.TempDir()
	res := ResolveBackgroundSubagentsPolicy(cwd, LoadBackgroundSubagentsOptions{GentleAiConfigHome: cfg, Env: map[string]string{"GENTLE_PI_BACKGROUND_SUBAGENTS": "on"}})
	if res.Policy != "on" || res.Source != BackgroundSourceEnvironment {
		t.Fatalf("expected env on, got %q %q", res.Policy, res.Source)
	}
	res2 := ResolveBackgroundSubagentsPolicy(cwd, LoadBackgroundSubagentsOptions{GentleAiConfigHome: cfg, Env: map[string]string{}})
	if res2.Policy != "off" || res2.Source != BackgroundSourceDefault {
		t.Fatalf("expected default off, got %q %q", res2.Policy, res2.Source)
	}
}

func TestRenderBackgroundSubagentsReport_Malformed(t *testing.T) {
	r := BackgroundSubagentsResolution{Policy: "off", Source: BackgroundSourceProject, Malformed: true, ProjectFile: "/tmp/.pi/gentle-ai/bg.json", GlobalFile: "/tmp/.pi/gentle-ai/global.json"}
	report := RenderBackgroundSubagentsReport(r, "ready", nil)
	if report.Type != "warning" {
		t.Fatalf("expected warning for malformed, got %q", report.Type)
	}
	if len(report.Message) == 0 {
		t.Fatal("empty message")
	}
}

func TestGentleAiConfigHome_EnvOverride(t *testing.T) {
	t.Setenv("GENTLE_PI_CONFIG_HOME", "/tmp/custom")
	if got := GentleAiConfigHome(); got != "/tmp/custom" {
		t.Fatalf("expected custom, got %q", got)
	}
}

// --- PR1 pi-mcp-adapter migration tests ---

func TestInstallCommand_ContainsAdapterBeforeSubagents(t *testing.T) {
	a := NewAdapter()
	cmds, err := a.InstallCommand(nil)
	if err != nil {
		t.Fatalf("InstallCommand error: %v", err)
	}
	var idxAdapter, idxJ0k3r = -1, -1
	for i, cmd := range cmds {
		joined := strings.Join(cmd, " ")
		if strings.Contains(joined, "pi-mcp-adapter") {
			idxAdapter = i
			if !strings.Contains(joined, "@^2") {
				t.Errorf("adapter entry must pin ^2, got %q", joined)
			}
			if !strings.Contains(joined, "npm:pi-mcp-adapter@^2") {
				t.Errorf("adapter entry must be npm:pi-mcp-adapter@^2, got %q", joined)
			}
		}
		if strings.Contains(joined, "pi-subagents-j0k3r") {
			idxJ0k3r = i
		}
	}
	if idxAdapter == -1 {
		t.Fatal("InstallCommand missing npm:pi-mcp-adapter@^2")
	}
	if idxJ0k3r == -1 {
		t.Fatal("InstallCommand missing pi-subagents-j0k3r")
	}
	if idxAdapter >= idxJ0k3r {
		t.Fatalf("adapter must precede pi-subagents-j0k3r: adapter idx %d, j0k3r idx %d", idxAdapter, idxJ0k3r)
	}
	// Idempotent: second call same
	cmds2, _ := a.InstallCommand(nil)
	if len(cmds) != len(cmds2) {
		t.Error("InstallCommand not idempotent length mismatch")
	}
	// No duplication of adapter
	count := 0
	for _, cmd := range cmds {
		if strings.Contains(strings.Join(cmd, " "), "pi-mcp-adapter") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 pi-mcp-adapter entry, got %d", count)
	}
}

func TestProvisionBigMemMCP_FreshProvisionCorrectShape(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "")
	home := t.TempDir()
	a := NewAdapter()
	changed, files, err := a.ProvisionBigMemMCP(home)
	if err != nil {
		t.Fatalf("ProvisionBigMemMCP: %v", err)
	}
	if !changed {
		t.Error("first provision should report changed=true")
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %v", files)
	}
	settingsPath := filepath.Join(home, ".pi", "agent", "settings.json")
	mcpPath := filepath.Join(home, ".pi", "agent", "mcp.json")
	// settings.json MUST have bigmem with --prefix=biggz
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var sObj map[string]any
	if err := json.Unmarshal(data, &sObj); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}
	servers, _ := sObj["mcpServers"].(map[string]any)
	if servers == nil {
		t.Fatal("settings mcpServers missing")
	}
	bm, _ := servers["bigmem"].(map[string]any)
	if bm == nil {
		t.Fatal("settings mcpServers.bigmem missing")
	}
	if bm["type"] != "local" {
		t.Errorf("settings bigmem type = %v want local", bm["type"])
	}
	if cmd, _ := bm["command"].(string); cmd == "" {
		t.Error("settings bigmem command empty")
	}
	args, _ := bm["args"].([]any)
	hasPrefix, hasTools := false, false
	for _, a := range args {
		if s, _ := a.(string); s == "--prefix=biggz" {
			hasPrefix = true
		}
		if s, _ := a.(string); s == "--tools=agent" {
			hasTools = true
		}
	}
	if !hasPrefix {
		t.Errorf("settings bigmem args missing --prefix=biggz got %v", args)
	}
	if !hasTools {
		t.Errorf("settings bigmem args missing --tools=agent got %v", args)
	}
	// mcp.json MUST have bigmem + imports:["opencode"] + directTools
	data2, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("read mcp.json: %v", err)
	}
	var mObj map[string]any
	if err := json.Unmarshal(data2, &mObj); err != nil {
		t.Fatalf("unmarshal mcp: %v", err)
	}
	servers2, _ := mObj["mcpServers"].(map[string]any)
	if servers2 == nil {
		t.Fatal("mcp mcpServers missing")
	}
	bm2, _ := servers2["bigmem"].(map[string]any)
	if bm2 == nil {
		t.Fatal("mcp mcpServers.bigmem missing")
	}
	if bm2["type"] != "local" {
		t.Errorf("mcp bigmem type = %v want local", bm2["type"])
	}
	args2, _ := bm2["args"].([]any)
	hasPrefix2, hasTools2 := false, false
	for _, a := range args2 {
		if s, _ := a.(string); s == "--prefix=biggz" {
			hasPrefix2 = true
		}
		if s, _ := a.(string); s == "--tools=agent" {
			hasTools2 = true
		}
	}
	if !hasPrefix2 {
		t.Errorf("mcp bigmem args missing --prefix=biggz got %v", args2)
	}
	if !hasTools2 {
		t.Errorf("mcp bigmem args missing --tools=agent got %v", args2)
	}
	// imports
	imports, _ := mObj["imports"].([]any)
	foundOpencode := false
	for _, v := range imports {
		if s, _ := v.(string); s == "opencode" {
			foundOpencode = true
		}
	}
	if !foundOpencode {
		t.Errorf("mcp imports missing opencode got %v", imports)
	}
	// directTools
	dt, _ := mObj["directTools"].([]any)
	if len(dt) == 0 {
		t.Error("mcp directTools empty, want at least biggz_mem_save")
	}
	hasSave, hasSearch := false, false
	for _, v := range dt {
		if s, _ := v.(string); s == "biggz_mem_save" {
			hasSave = true
		}
		if s, _ := v.(string); s == "biggz_mem_search" {
			hasSearch = true
		}
	}
	if !hasSave {
		t.Errorf("directTools missing biggz_mem_save got %v", dt)
	}
	if !hasSearch {
		t.Errorf("directTools missing biggz_mem_search got %v", dt)
	}
}

func TestProvisionBigMemMCP_MergePreservesOthersAtomically(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "")
	home := t.TempDir()
	a := NewAdapter()
	settingsPath := filepath.Join(home, ".pi", "agent", "settings.json")
	mcpPath := filepath.Join(home, ".pi", "agent", "mcp.json")
	// Pre-create with other server and custom imports
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	preSettings := map[string]any{
		"mcpServers": map[string]any{
			"other": map[string]any{"command": "other-cmd", "args": []string{}, "type": "local"},
		},
	}
	bs, _ := json.Marshal(preSettings)
	if err := os.WriteFile(settingsPath, bs, 0o644); err != nil {
		t.Fatal(err)
	}
	preMCP := map[string]any{
		"mcpServers": map[string]any{
			"other": map[string]any{"command": "other-cmd", "args": []string{}, "type": "local"},
		},
		"imports": []string{"existing"},
		"directTools": []string{"existing_tool"},
	}
	bm, _ := json.Marshal(preMCP)
	if err := os.WriteFile(mcpPath, bm, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.ProvisionBigMemMCP(home); err != nil {
		t.Fatalf("ProvisionBigMemMCP merge: %v", err)
	}
	// settings should preserve other
	data, _ := os.ReadFile(settingsPath)
	var sObj map[string]any
	_ = json.Unmarshal(data, &sObj)
	servers, _ := sObj["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Error("settings merge lost mcpServers.other")
	}
	if _, ok := servers["bigmem"]; !ok {
		t.Error("settings merge missing bigmem after merge")
	}
	// mcp should preserve other + opencode + existing + directTools merge
	data2, _ := os.ReadFile(mcpPath)
	var mObj map[string]any
	_ = json.Unmarshal(data2, &mObj)
	servers2, _ := mObj["mcpServers"].(map[string]any)
	if _, ok := servers2["other"]; !ok {
		t.Error("mcp merge lost mcpServers.other")
	}
	imports, _ := mObj["imports"].([]any)
	hasOpencode, hasExisting := false, false
	for _, v := range imports {
		if s, _ := v.(string); s == "opencode" {
			hasOpencode = true
		}
		if s, _ := v.(string); s == "existing" {
			hasExisting = true
		}
	}
	if !hasOpencode {
		t.Error("mcp imports missing opencode after merge")
	}
	if !hasExisting {
		t.Error("mcp imports lost existing after merge")
	}
	dt, _ := mObj["directTools"].([]any)
	hasExistingTool, hasSave := false, false
	for _, v := range dt {
		if s, _ := v.(string); s == "existing_tool" {
			hasExistingTool = true
		}
		if s, _ := v.(string); s == "biggz_mem_save" {
			hasSave = true
		}
	}
	if !hasExistingTool {
		t.Error("mcp directTools lost existing_tool")
	}
	if !hasSave {
		t.Error("mcp directTools missing biggz_mem_save after merge")
	}
	// Failed write leaves target unchanged: invalid JSON in file returns error and file untouched
	invalidPath := filepath.Join(home, ".pi", "agent", "invalid_mcp.json")
	if err := os.WriteFile(invalidPath, []byte(`{ invalid json`), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(invalidPath)
	if _, err := a.mergePiMCPFileBigMem(invalidPath, "dummy"); err == nil {
		t.Error("expected error on invalid json")
	}
	after, _ := os.ReadFile(invalidPath)
	if string(before) != string(after) {
		t.Error("failed merge changed file, should leave unchanged")
	}
}

func TestProvisionBigMemMCP_Idempotent(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "")
	home := t.TempDir()
	a := NewAdapter()
	if _, _, err := a.ProvisionBigMemMCP(home); err != nil {
		t.Fatalf("first provision: %v", err)
	}
	settingsPath := filepath.Join(home, ".pi", "agent", "settings.json")
	mcpPath := filepath.Join(home, ".pi", "agent", "mcp.json")
	beforeS, _ := os.ReadFile(settingsPath)
	beforeM, _ := os.ReadFile(mcpPath)
	changed, _, err := a.ProvisionBigMemMCP(home)
	if err != nil {
		t.Fatalf("second provision: %v", err)
	}
	if changed {
		t.Error("second provision should be idempotent changed=false")
	}
	afterS, _ := os.ReadFile(settingsPath)
	afterM, _ := os.ReadFile(mcpPath)
	if string(beforeS) != string(afterS) {
		t.Error("settings changed on idempotent second run")
	}
	if string(beforeM) != string(afterM) {
		t.Error("mcp changed on idempotent second run")
	}
}

func TestBiggzMCPPath_PriorityHomeFirst(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Also need USERPROFILE on windows? Set both.
	t.Setenv("USERPROFILE", home)
	a := NewAdapter()
	// Create ~/.biggz/biggz-mcp
	cand := filepath.Join(home, ".biggz", "biggz-mcp")
	if err := os.MkdirAll(filepath.Dir(cand), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cand, []byte("dummy"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := a.BiggzMCPPath()
	if got != cand {
		t.Fatalf("BiggzMCPPath priority home first: got %q want %q", got, cand)
	}
}

func TestMergePiMCPFileBigMem_PreservesOtherAndAtomic(t *testing.T) {
	home := t.TempDir()
	a := NewAdapter()
	path := filepath.Join(home, "mcp.json")
	pre := map[string]any{
		"mcpServers": map[string]any{
			"other": map[string]any{"command": "other", "type": "local"},
		},
	}
	bs, _ := json.Marshal(pre)
	if err := os.WriteFile(path, bs, 0o644); err != nil {
		t.Fatal(err)
	}
	// Ensure BiggzMCPPath is not empty; use explicit binary
	res, err := a.mergePiMCPFileBigMem(path, "/tmp/biggz-mcp")
	if err != nil {
		t.Fatalf("mergePiMCPFileBigMem: %v", err)
	}
	if !res.Changed && !res.Created {
		t.Log("merge reported no change (maybe idempotent second)")
	}
	data, _ := os.ReadFile(path)
	var obj map[string]any
	_ = json.Unmarshal(data, &obj)
	servers, _ := obj["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Error("merge lost other")
	}
	if _, ok := servers["bigmem"]; !ok {
		t.Error("merge missing bigmem")
	}
	bm, _ := servers["bigmem"].(map[string]any)
	args, _ := bm["args"].([]any)
	hasPrefix := false
	for _, v := range args {
		if s, _ := v.(string); s == "--prefix=biggz" {
			hasPrefix = true
		}
	}
	if !hasPrefix {
		t.Errorf("bigmem args missing --prefix=biggz got %v", args)
	}
	// Atomic: second call with same content should be no-op via WriteFileAtomic
	before, _ := os.ReadFile(path)
	res2, err := a.mergePiMCPFileBigMem(path, "/tmp/biggz-mcp")
	if err != nil {
		t.Fatalf("second merge: %v", err)
	}
	if res2.Changed {
		t.Error("second merge with same content should be Changed=false")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Error("atomic second merge changed file unexpectedly")
	}
}
