package pi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/sdd"
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

// Twin parity: the pi renderer is a thin delegate to the sdd owner, so the
// disabled/unmanaged notice and the warning type match the owner exactly.
func TestRenderBackgroundSubagentsReport_DisabledUnmanagedTwin(t *testing.T) {
	r := BackgroundSubagentsResolution{Policy: "off", Source: BackgroundSourceDefault}
	report := RenderBackgroundSubagentsReport(r, "absent", nil)
	for _, token := range []string{"policy: off", "capability: absent", "disabled/unmanaged"} {
		if !strings.Contains(report.Message, token) {
			t.Fatalf("twin report missing %q:\n%s", token, report.Message)
		}
	}
	if report.Type != "warning" {
		t.Fatalf("twin report type = %q, want warning", report.Type)
	}
	if owner := sdd.RenderBackgroundSubagentsReport(r, "absent", nil); report != owner {
		t.Fatalf("twin drifted from the sdd owner:\ntwin:  %#v\nowner: %#v", report, owner)
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
	t.Setenv("PI_CODING_AGENT_DIR", t.TempDir()) // no runtime marker: pre-cutover list
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
	// Hermetic: AgentConfigPath honors PI_CODING_AGENT_DIR, so clear it or the
	// provision would write into a real agent home instead of t.TempDir().
	t.Setenv("PI_CODING_AGENT_DIR", "")
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

	// settings.json: native Pi has no mcpServers/directTools keys; only the
	// packages reconcile survives there.
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var sObj map[string]any
	if err := json.Unmarshal(data, &sObj); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}
	if _, ok := sObj["mcpServers"]; ok {
		t.Errorf("settings.json must not carry mcpServers in the native contract: %v", sObj["mcpServers"])
	}
	if _, ok := sObj["directTools"]; ok {
		t.Errorf("settings.json must not carry directTools in the native contract: %v", sObj["directTools"])
	}
	if _, ok := sObj["packages"]; !ok {
		t.Error("settings.json packages reconcile must remain")
	}

	// mcp.json: the native bigmem shape (decisions D6/D8).
	data2, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("read mcp.json: %v", err)
	}
	var mObj map[string]any
	if err := json.Unmarshal(data2, &mObj); err != nil {
		t.Fatalf("unmarshal mcp: %v", err)
	}
	if imports, ok := mObj["imports"]; ok {
		t.Errorf("mcp.json must not carry top-level imports: %v", imports)
	}
	if dt, ok := mObj["directTools"]; ok {
		t.Errorf("mcp.json must not carry top-level directTools: %v", dt)
	}
	servers2, _ := mObj["mcpServers"].(map[string]any)
	if servers2 == nil {
		t.Fatal("mcp mcpServers missing")
	}
	bm2, _ := servers2["bigmem"].(map[string]any)
	if bm2 == nil {
		t.Fatal("mcp mcpServers.bigmem missing")
	}
	if _, ok := bm2["type"]; ok {
		t.Errorf("native bigmem must omit type (D6), got %v", bm2["type"])
	}
	if bm2["exposure"] != "codemode" {
		t.Errorf("mcp bigmem exposure = %v want codemode", bm2["exposure"])
	}
	if desc, _ := bm2["description"].(string); desc == "" {
		t.Error("mcp bigmem description missing")
	}
	if cmd, _ := bm2["command"].(string); cmd == "" {
		t.Error("mcp bigmem command empty")
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
	// toolExposure maps exactly the 11 piDirectTools server-declared names to "direct".
	want11 := map[string]struct{}{
		"biggz_mem_save": {}, "biggz_mem_search": {}, "biggz_mem_get_observation": {},
		"biggz_mem_context": {}, "biggz_mem_session_summary": {}, "biggz_mem_save_prompt": {},
		"biggz_mem_update": {}, "biggz_mem_timeline": {}, "biggz_mem_review": {},
		"biggz_mem_judge": {}, "biggz_mem_current_project": {},
	}
	te, _ := bm2["toolExposure"].(map[string]any)
	if len(te) != len(want11) {
		t.Errorf("fresh toolExposure len = %d, want exactly %d: %v", len(te), len(want11), te)
	}
	for name := range want11 {
		if te[name] != "direct" {
			t.Errorf("fresh toolExposure[%q] = %v, want direct", name, te[name])
		}
	}
	for name := range te {
		if _, ok := want11[name]; !ok {
			t.Errorf("fresh toolExposure has unexpected entry %q: %v", name, te)
		}
	}
}

func TestProvisionBigMemMCP_MergePreservesOthersAtomically(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "")
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	a := NewAdapter()
	settingsPath := filepath.Join(home, ".pi", "agent", "settings.json")
	mcpPath := filepath.Join(home, ".pi", "agent", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	// Pre-create with another server, a biggz-owned legacy bigmem entry and
	// adapter-era keys, plus a foreign top-level key that must survive.
	preSettings := map[string]any{
		"mcpServers": map[string]any{
			"other":  map[string]any{"command": "other-cmd", "args": []string{}, "type": "local"},
			"bigmem": map[string]any{"command": "old-bigmem", "type": "local"},
		},
		"directTools":  []string{"existing_tool"},
		"defaultTools": []string{"read"},
	}
	bs, _ := json.Marshal(preSettings)
	if err := os.WriteFile(settingsPath, bs, 0o644); err != nil {
		t.Fatal(err)
	}
	preMCP := map[string]any{
		"mcpServers": map[string]any{
			"other": map[string]any{"command": "other-cmd", "args": []string{}, "type": "local"},
		},
		"imports":     []string{"existing"},
		"directTools": []string{"existing_tool"},
	}
	bm, _ := json.Marshal(preMCP)
	if err := os.WriteFile(mcpPath, bm, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.ProvisionBigMemMCP(home); err != nil {
		t.Fatalf("ProvisionBigMemMCP merge: %v", err)
	}
	// settings.json: foreign server + foreign key survive; biggz-owned legacy keys pruned.
	data, _ := os.ReadFile(settingsPath)
	var sObj map[string]any
	_ = json.Unmarshal(data, &sObj)
	servers, _ := sObj["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Error("settings merge lost mcpServers.other")
	}
	if _, ok := servers["bigmem"]; ok {
		t.Error("settings merge must prune biggz-owned mcpServers.bigmem")
	}
	if _, ok := sObj["directTools"]; ok {
		t.Error("settings merge must prune biggz-owned directTools")
	}
	if _, ok := sObj["defaultTools"]; !ok {
		t.Error("settings merge lost foreign defaultTools")
	}
	// mcp.json: foreign server survives; adapter-era keys pruned; native bigmem added.
	data2, _ := os.ReadFile(mcpPath)
	var mObj map[string]any
	_ = json.Unmarshal(data2, &mObj)
	if _, ok := mObj["imports"]; ok {
		t.Error("mcp merge must prune imports")
	}
	if _, ok := mObj["directTools"]; ok {
		t.Error("mcp merge must prune directTools")
	}
	servers2, _ := mObj["mcpServers"].(map[string]any)
	if _, ok := servers2["other"]; !ok {
		t.Error("mcp merge lost mcpServers.other")
	}
	bm2, _ := servers2["bigmem"].(map[string]any)
	if bm2 == nil {
		t.Fatal("mcp merge missing bigmem")
	}
	if _, ok := bm2["type"]; ok {
		t.Errorf("native bigmem must omit type, got %v", bm2["type"])
	}
	if bm2["exposure"] != "codemode" {
		t.Errorf("mcp bigmem exposure = %v want codemode", bm2["exposure"])
	}
	// Failed write leaves target unchanged: invalid JSON returns error and file untouched.
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
	t.Setenv("PI_CODING_AGENT_DIR", "")
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
		"imports":     []string{"opencode"},
		"directTools": []string{"biggz_mem_save"},
	}
	bs, _ := json.Marshal(pre)
	if err := os.WriteFile(path, bs, 0o644); err != nil {
		t.Fatal(err)
	}
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
	if _, ok := obj["imports"]; ok {
		t.Error("merge must prune top-level imports")
	}
	if _, ok := obj["directTools"]; ok {
		t.Error("merge must prune top-level directTools")
	}
	servers, _ := obj["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Error("merge lost other")
	}
	bm, _ := servers["bigmem"].(map[string]any)
	if bm == nil {
		t.Fatal("merge missing bigmem")
	}
	if _, ok := bm["type"]; ok {
		t.Errorf("native bigmem must omit type, got %v", bm["type"])
	}
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

func TestMergePiMCPFileBigMem_PrunesLegacyKeysPreservesForeign(t *testing.T) {
	home := t.TempDir()
	a := NewAdapter()
	path := filepath.Join(home, "mcp.json")
	// Legacy adapter-era install: imports, directTools, a foreign top-level key
	// and a foreign server must all be reconciled without collateral damage.
	pre := map[string]any{
		"mcpServers": map[string]any{
			"other": map[string]any{"command": "other", "type": "local"},
		},
		"imports":     []string{"opencode", "existing"},
		"directTools": []string{"existing_tool", "biggz_mem_save"},
		"customKey":   "keep-me",
	}
	bs, _ := json.Marshal(pre)
	if err := os.WriteFile(path, bs, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.mergePiMCPFileBigMem(path, "/tmp/biggz-mcp"); err != nil {
		t.Fatalf("mergePiMCPFileBigMem: %v", err)
	}
	data, _ := os.ReadFile(path)
	var obj map[string]any
	_ = json.Unmarshal(data, &obj)
	if _, ok := obj["imports"]; ok {
		t.Error("legacy imports not pruned")
	}
	if _, ok := obj["directTools"]; ok {
		t.Error("legacy directTools not pruned")
	}
	if obj["customKey"] != "keep-me" {
		t.Errorf("foreign top-level key lost: %v", obj["customKey"])
	}
	servers, _ := obj["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Error("foreign server lost")
	}
	// Idempotent: re-merging the merged file is a fixed point.
	before, _ := os.ReadFile(path)
	res, err := a.mergePiMCPFileBigMem(path, "/tmp/biggz-mcp")
	if err != nil {
		t.Fatalf("re-merge: %v", err)
	}
	if res.Changed {
		t.Error("re-merge should be Changed=false")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Error("re-merge not idempotent")
	}
}

func TestProvisionBigMemMCP_ReinstallPrunesLegacyAdapterKeys(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "")
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	a := NewAdapter()
	mcpPath := filepath.Join(home, ".pi", "agent", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(mcpPath), 0o755); err != nil {
		t.Fatal(err)
	}
	// Existing adapter-era install: legacy bigmem shape + adapter keys + foreign server.
	legacy := map[string]any{
		"mcpServers": map[string]any{
			"other":  map[string]any{"command": "other-cmd", "args": []string{}, "type": "local"},
			"bigmem": map[string]any{"command": "old-biggz-mcp", "args": []string{"--prefix=biggz"}, "type": "local"},
		},
		"imports":     []string{"opencode"},
		"directTools": []string{"biggz_mem_save", "biggz_mem_search"},
	}
	bs, _ := json.Marshal(legacy)
	if err := os.WriteFile(mcpPath, bs, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.ProvisionBigMemMCP(home); err != nil {
		t.Fatalf("reinstall provision: %v", err)
	}
	data, _ := os.ReadFile(mcpPath)
	var mObj map[string]any
	_ = json.Unmarshal(data, &mObj)
	if _, ok := mObj["imports"]; ok {
		t.Error("reinstall kept legacy imports")
	}
	if _, ok := mObj["directTools"]; ok {
		t.Error("reinstall kept legacy directTools")
	}
	servers, _ := mObj["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Error("reinstall lost mcpServers.other")
	}
	bm, _ := servers["bigmem"].(map[string]any)
	if bm == nil {
		t.Fatal("reinstall lost mcpServers.bigmem")
	}
	if _, ok := bm["type"]; ok {
		t.Errorf("reinstall kept legacy type: %v", bm["type"])
	}
	te, _ := bm["toolExposure"].(map[string]any)
	if len(te) != 11 {
		t.Errorf("reinstall toolExposure len = %d, want 11: %v", len(te), te)
	}
}

func TestProvisionBigMemMCP_SettingsPrunesLegacyBigMemKeys(t *testing.T) {
	t.Setenv("PI_SUBAGENT_CHILD", "")
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	a := NewAdapter()
	settingsPath := filepath.Join(home, ".pi", "agent", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	// Legacy settings.json with a biggz-owned bigmem entry, legacy directTools,
	// a foreign server and a foreign top-level key.
	legacy := map[string]any{
		"mcpServers": map[string]any{
			"other":  map[string]any{"command": "other-cmd", "args": []string{}, "type": "local"},
			"bigmem": map[string]any{"command": "old-biggz-mcp", "type": "local"},
		},
		"directTools":  []string{"biggz_mem_save", "existing_tool"},
		"defaultTools": []string{"read"},
	}
	bs, _ := json.Marshal(legacy)
	if err := os.WriteFile(settingsPath, bs, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.ProvisionBigMemMCP(home); err != nil {
		t.Fatalf("provision with legacy settings: %v", err)
	}
	data, _ := os.ReadFile(settingsPath)
	var sObj map[string]any
	_ = json.Unmarshal(data, &sObj)
	if _, ok := sObj["directTools"]; ok {
		t.Error("settings kept legacy directTools")
	}
	servers, _ := sObj["mcpServers"].(map[string]any)
	if _, ok := servers["bigmem"]; ok {
		t.Error("settings kept legacy mcpServers.bigmem")
	}
	if _, ok := servers["other"]; !ok {
		t.Error("settings lost foreign mcpServers.other")
	}
	if _, ok := sObj["defaultTools"]; !ok {
		t.Error("settings lost foreign defaultTools")
	}
	if _, ok := sObj["packages"]; !ok {
		t.Error("settings packages reconcile must remain")
	}
}
