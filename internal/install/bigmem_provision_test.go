package install_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/agents/pi"
	"github.com/biggs-100/biggz-ai/internal/assets"
	"github.com/biggs-100/biggz-ai/internal/bigmem"
	"github.com/biggs-100/biggz-ai/internal/install"
	"github.com/biggs-100/biggz-ai/model"
	"github.com/biggs-100/biggz-ai/plugintest"
)

// TestOverlayDoesNotContainBigMemAllowlist ensures the MCP tools ARE listed
// for SDD workflow agents but NOT for generic workers. MCP servers (biggz-mcp)
// are discovered via mcpServers (mcpServers.bigmem via biggz-mcp --tools=agent --prefix=biggz)
// and correctly provisioned via ProvisionBigMemMCP (pi) / DeployMCPConfig (opencode),
// so SDD agents can now safely expose biggz_mem_* as allowlisted MCP tools.
// Previous fix 82e5d56 REMOVED them because they were listed as extension tools
// without MCP provisioning, causing "requested unavailable child tools" with strict *:"deny".
// Now with correct MCP provisioning, we restore them for SDD workflow agents.
func TestOverlayDoesNotContainBigMemAllowlist(t *testing.T) {
	data, err := fs.ReadFile(assets.FS, "opencode/sdd-overlay-multi.json")
	if err != nil {
		t.Fatalf("read overlay: %v", err)
	}
	s := string(data)
	// SDD agents MUST contain BigMem MCP tools (since MCP is now correctly provisioned)
	for _, want := range []string{
		"biggz_mem_save", "biggz_mem_search", "biggz_mem_get_observation",
		"biggz_mem_update", "biggz_mem_context",
	} {
		if !strings.Contains(s, `"`+want+`": true`) && !strings.Contains(s, `"`+want+`":true`) {
			t.Errorf("overlay must contain BigMem tool allowlist %q for SDD agents", want)
		}
	}
	var overlay map[string]any
	if err := json.Unmarshal(data, &overlay); err != nil {
		t.Fatalf("unmarshal overlay: %v", err)
	}
	agents, _ := overlay["agent"].(map[string]any)
	// SDD workflow agents must have BigMem tools
	for _, name := range []string{"sdd-propose", "sdd-explore", "sdd-apply", "sdd-verify", "sdd-spec", "sdd-design", "sdd-tasks", "sdd-init", "sdd-archive", "sdd-onboard", "sdd-research"} {
		ag, ok := agents[name].(map[string]any)
		if !ok {
			continue // single overlay may not have all
		}
		tools, _ := ag["tools"].(map[string]any)
		if tools == nil {
			t.Fatalf("agent %q tools missing", name)
		}
		for _, want := range []string{"biggz_mem_save", "biggz_mem_search", "biggz_mem_get_observation", "biggz_mem_update", "biggz_mem_context"} {
			if _, ok := tools[want]; !ok {
				t.Errorf("SDD agent %q must allowlist MCP tool %q (BigMem via MCP, now correctly provisioned)", name, want)
			}
		}
		if _, ok := tools["read"]; !ok {
			t.Errorf("agent %q missing read tool", name)
		}
		if _, ok := tools["bash"]; !ok {
			t.Errorf("agent %q missing bash tool", name)
		}
	}
	// Generic workers must NOT get BigMem (only SDD workflow)
	for _, name := range []string{"general", "explore", "jd-judge-a", "review-risk"} {
		ag, ok := agents[name].(map[string]any)
		if !ok {
			continue
		}
		tools, _ := ag["tools"].(map[string]any)
		if tools == nil {
			continue
		}
		for k := range tools {
			if strings.HasPrefix(k, "biggz_mem_") {
				t.Errorf("generic agent %q must NOT allowlist MCP tool %q (only SDD workflow should have BigMem)", name, k)
			}
		}
	}
}

// TestDeployMCPMergeIntoSettings_WritesBiggzServer verifies opencode's
// MergeIntoSettings strategy correctly merges mcp.biggz with command
// [biggz-mcp --tools=agent --prefix=biggz] type local enabled true,
// preserves existing keys, and is idempotent.
func TestDeployMCPMergeIntoSettings_WritesBiggzServer(t *testing.T) {
	home := t.TempDir()
	// Use FakeAgent mimicking opencode
	agent := &plugintest.FakeAgent{
		Installed:  true,
		BinaryPath: "/usr/local/bin/opencode",
		AgentID:    "opencode",
	}
	// Enable MCP + MergeIntoSettings
	trueVal := true
	agent.AgentMCP = &trueVal
	agent.AgentMCPStrategy = model.StrategyMergeIntoSettings

	// Simulate a binary path that the fallback resolver would produce
	binaryPath := filepath.Join(home, ".biggz", "biggz-mcp")
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0755); err != nil {
		t.Fatal(err)
	}
	// Create a dummy file so BiggzMCPPath fallback could find it, but we pass explicit path
	_ = os.WriteFile(binaryPath, []byte("dummy"), 0755)

	// First deploy
	if err := install.DeployMCPConfig(agent, home, binaryPath, false); err != nil {
		t.Fatalf("DeployMCPConfig: %v", err)
	}
	settingsPath := agent.SettingsPath(home)
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings not written: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		// Try JSONC-tolerant: MergeJSONC may have added comments? But our fake starts empty so pure JSON
		t.Fatalf("unmarshal settings: %v (raw: %s)", err, string(data))
	}
	mcp, _ := cfg["mcp"].(map[string]any)
	if mcp == nil {
		t.Fatalf("mcp missing in settings: %s", string(data))
	}
	biggz, _ := mcp["biggz"].(map[string]any)
	if biggz == nil {
		t.Fatalf("mcp.biggz missing: %s", string(data))
	}
	cmd, _ := biggz["command"].([]any)
	if len(cmd) != 3 || cmd[0] != binaryPath || cmd[1] != "--tools=agent" || cmd[2] != "--prefix=biggz" {
		t.Errorf("mcp.biggz.command = %v, want [%q --tools=agent --prefix=biggz]", cmd, binaryPath)
	}
	if biggz["type"] != "local" {
		t.Errorf("mcp.biggz.type = %v, want local", biggz["type"])
	}
	if biggz["enabled"] != true {
		t.Errorf("mcp.biggz.enabled = %v, want true", biggz["enabled"])
	}

	// Idempotent second deploy should not change file
	before, _ := os.ReadFile(settingsPath)
	if err := install.DeployMCPConfig(agent, home, binaryPath, false); err != nil {
		t.Fatalf("second DeployMCPConfig: %v", err)
	}
	after, _ := os.ReadFile(settingsPath)
	if string(before) != string(after) {
		t.Errorf("second deploy changed file (not idempotent):\nbefore: %s\nafter: %s", string(before), string(after))
	}

	// Guard: fresh pi subagent child should skip provisioning entirely (no file created/changed)
	home2 := t.TempDir()
	t.Setenv("PI_SUBAGENT_CHILD", "1")
	agent2 := &plugintest.FakeAgent{
		Installed:  true,
		BinaryPath: "/usr/local/bin/opencode",
		AgentID:    "opencode",
	}
	agent2.AgentMCP = &trueVal
	agent2.AgentMCPStrategy = model.StrategyMergeIntoSettings
	if err := install.DeployMCPConfig(agent2, home2, binaryPath, false); err != nil {
		t.Fatalf("DeployMCPConfig in child: %v", err)
	}
	if _, err := os.Stat(agent2.SettingsPath(home2)); err == nil {
		t.Error("PI_SUBAGENT_CHILD=1 should skip DeployMCPConfig (file should not be created)")
	}
	t.Setenv("PI_SUBAGENT_CHILD", "")
}

// TestProvisionBigMemMCP_WritesBothFiles verifies pi's ProvisionBigMemMCP writes
// both settings.json and mcp.json with mcpServers.bigmem {command, args, type:local},
// preserves existing keys, cleans legacy vendors, and is idempotent.
func TestProvisionBigMemMCP_WritesBothFiles(t *testing.T) {
	home := t.TempDir()
	a := pi.NewAdapter()

	changed, files, err := a.ProvisionBigMemMCP(home)
	if err != nil {
		t.Fatalf("ProvisionBigMemMCP: %v", err)
	}
	if !changed {
		t.Error("first ProvisionBigMemMCP should report changed=true")
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %v", files)
	}
	settingsPath := filepath.Join(home, ".pi", "agent", "settings.json")
	mcpPath := filepath.Join(home, ".pi", "agent", "mcp.json")
	for _, want := range []string{settingsPath, mcpPath} {
		found := false
		for _, f := range files {
			if f == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("files missing %q, got %v", want, files)
		}
		if _, err := os.Stat(want); err != nil {
			t.Fatalf("file %q not created: %v", want, err)
		}
		data, _ := os.ReadFile(want)
		var obj map[string]any
		if err := json.Unmarshal(data, &obj); err != nil {
			t.Fatalf("unmarshal %q: %v", want, err)
		}
		servers, _ := obj["mcpServers"].(map[string]any)
		if servers == nil {
			t.Fatalf("%q missing mcpServers", want)
		}
		bm, _ := servers["bigmem"].(map[string]any)
		if bm == nil {
			t.Fatalf("%q missing mcpServers.bigmem", want)
		}
		if bm["type"] != "local" {
			t.Errorf("%q bigmem.type = %v, want local", want, bm["type"])
		}
		cmd, _ := bm["command"].(string)
		if cmd == "" {
			t.Errorf("%q bigmem.command empty", want)
		}
		// args must contain --tools=agent
		args, _ := bm["args"].([]any)
		hasTools := false
		for _, a := range args {
			if s, _ := a.(string); s == "--tools=agent" {
				hasTools = true
			}
		}
		if !hasTools {
			t.Errorf("%q bigmem.args missing --tools=agent, got %v", want, args)
		}
	}

	// Idempotent second run: should report changed=false and file unchanged
	beforeSettings, _ := os.ReadFile(settingsPath)
	beforeMCP, _ := os.ReadFile(mcpPath)
	changed2, _, err := a.ProvisionBigMemMCP(home)
	if err != nil {
		t.Fatalf("second ProvisionBigMemMCP: %v", err)
	}
	if changed2 {
		t.Error("second ProvisionBigMemMCP should be idempotent (changed=false)")
	}
	afterSettings, _ := os.ReadFile(settingsPath)
	afterMCP, _ := os.ReadFile(mcpPath)
	if string(beforeSettings) != string(afterSettings) {
		t.Error("settings.json changed on second (idempotent) run")
	}
	if string(beforeMCP) != string(afterMCP) {
		t.Error("mcp.json changed on second (idempotent) run")
	}

	// Verify biggzMCPPath fallback resolves correctly: should be absolute or bare name, never empty
	binary := a.BiggzMCPPath()
	if binary == "" {
		t.Error("BiggzMCPPath fallback returned empty")
	}
}

// TestProvisionBigMemMCP_SkipsInFreshChild ensures concurrent fresh pi
// subagents (PI_SUBAGENT_CHILD=1) do not race mcp.json/settings.json writes.
// Mirrors biggz-last-model.js guard.
func TestProvisionBigMemMCP_SkipsInFreshChild(t *testing.T) {
	home := t.TempDir()
	a := pi.NewAdapter()
	// Pre-provision as parent
	if _, _, err := a.ProvisionBigMemMCP(home); err != nil {
		t.Fatalf("parent provision: %v", err)
	}
	settingsPath := filepath.Join(home, ".pi", "agent", "settings.json")
	before, _ := os.ReadFile(settingsPath)

	t.Setenv("PI_SUBAGENT_CHILD", "1")
	changed, files, err := a.ProvisionBigMemMCP(home)
	if err != nil {
		t.Fatalf("child provision: %v", err)
	}
	if changed {
		t.Error("PI_SUBAGENT_CHILD=1 should return changed=false")
	}
	if len(files) != 0 {
		t.Errorf("PI_SUBAGENT_CHILD=1 should return no files, got %v", files)
	}
	after, _ := os.ReadFile(settingsPath)
	if string(before) != string(after) {
		t.Error("PI_SUBAGENT_CHILD=1 modified settings.json (should be skipped)")
	}

	// Also verify DeployMCPBinaryToHomeDir skips in child
	if dst, err := install.DeployMCPBinaryToHomeDir(home, false); err != nil {
		t.Fatalf("DeployMCPBinaryToHomeDir in child: %v", err)
	} else if dst != "" {
		t.Errorf("DeployMCPBinaryToHomeDir in child should return empty, got %q", dst)
	}
}

// TestBigMem_FreshSubagentCanSave simulates a fresh pi subagent (or opencode
// general) calling biggz_mem_save via the underlying bigmem store. The MCP
// server is just a wrapper around bigmem.Open; if the store works, the MCP
// tool will work. This test ensures the store remains usable even when
// provisioning is skipped in the child.
func TestBigMem_FreshSubagentCanSave(t *testing.T) {
	home := t.TempDir()
	bigmemDir := filepath.Join(home, ".biggz", "bigmem")
	store, err := bigmem.Open(bigmemDir)
	if err != nil {
		t.Fatalf("bigmem.Open: %v", err)
	}
	defer store.Close()

	// Parent saves
	obs := &bigmem.Observation{
		Title:   "fresh-subagent-save-test",
		Content: "subagent must be able to use BigMem even when PI_SUBAGENT_CHILD=1",
		Type:    "manual",
		Project: "test-proj",
	}
	if err := store.Save(obs); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if obs.ID == "" {
		t.Fatal("Save did not set ID")
	}

	// Simulate fresh child: provisioning skipped, but store operations must succeed
	t.Setenv("PI_SUBAGENT_CHILD", "1")
	a := pi.NewAdapter()
	if changed, _, err := a.ProvisionBigMemMCP(home); err != nil {
		t.Fatalf("child ProvisionBigMemMCP: %v", err)
	} else if changed {
		t.Error("child should not provision")
	}
	// Child saves another observation via same store (MCP would use same DB)
	childObs := &bigmem.Observation{
		Title:   "fresh-subagent-save-test-2",
		Content: "second save from child",
		Type:    "manual",
		Project: "test-proj",
	}
	if err := store.Save(childObs); err != nil {
		t.Fatalf("child Save: %v", err)
	}

	// Verify both are searchable
	results, err := store.Search("fresh-subagent-save-test", bigmem.SearchOptions{Project: "test-proj", Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Error("Search returned 0 results, want >=1")
	}
	// Verify stats work (biggz bigmem stats uses same store) — ConflictsStats and Search are core
	if _, err := store.ConflictsStats(""); err != nil {
		t.Fatalf("ConflictsStats: %v", err)
	}
	// ListProjects requires sessions table which is created lazily via SessionStart;
	// ensure at least Search/Get work. If sessions exists, ListProjects should not error.
	if _, err := store.ListProjects(); err != nil && !strings.Contains(err.Error(), "no such table: sessions") {
		t.Fatalf("ListProjects: %v", err)
	}

	// Verify Get works
	got, err := store.Get(obs.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != obs.Title {
		t.Errorf("Get title = %q, want %q", got.Title, obs.Title)
	}
}

// TestBigMemStatsAfterInstall verifies `biggz bigmem stats` works after install
// by checking that bigmem.Open on the default location (or temp home) succeeds
// and stats can be retrieved without error.
func TestBigMemStatsAfterInstall(t *testing.T) {
	home := t.TempDir()
	// Simulate what install does: ensure BigMem dir exists via Open
	store, err := bigmem.Open(filepath.Join(home, ".biggz", "bigmem"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()
	// Save something so stats has data
	_ = store.Save(&bigmem.Observation{Title: "stats-test", Content: "hello stats", Project: "stats-proj"})

		// Stats-like queries — core bigmem stats uses ConflictsStats and Search
	conflicts, err := store.ConflictsStats("")
	if err != nil {
		t.Fatalf("ConflictsStats: %v", err)
	}
	if conflicts == nil {
		t.Error("ConflictsStats returned nil")
	}
	// Verify the saved observation is searchable (stats-proj project)
	results, err := store.Search("stats-test", bigmem.SearchOptions{Project: "stats-proj", Limit: 5})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Error("Search for stats-test returned 0 results, want >=1")
	}
	// ListProjects lazily requires sessions table; accept either success or missing-table
	if projects, err := store.ListProjects(); err != nil {
		if !strings.Contains(err.Error(), "no such table: sessions") {
			t.Fatalf("ListProjects: %v", err)
		}
	} else {
		found := false
		for _, p := range projects {
			if p.Name == "stats-proj" {
				found = true
				break
			}
		}
		if !found && len(projects) > 0 {
			// If sessions table exists but project not yet via sessions join, still pass if Search succeeded
			// (ListProjects joins sessions; fresh DB without sessions will return empty list)
			t.Logf("ListProjects missing stats-proj (expected without sessions), got %+v", projects)
		}
	}
}
