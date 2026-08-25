package install_test

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/biggs-100/biggz-ai/internal/assets"
	"github.com/biggs-100/biggz-ai/internal/install"
)

func TestMemoryChromeAssetExists(t *testing.T) {
	data, err := fs.ReadFile(assets.FS, "pi/biggz-memory-chrome.js")
	if err != nil {
		t.Fatalf("read pi/biggz-memory-chrome.js via assets.FS: %v", err)
	}
	s := string(data)
	// Exported signatures required by task
	for _, need := range []string{
		"SUPPORTED_MEMORY_TOOLS",
		"humanToolName",
		"compactToolArg",
		"compactResultStatus",
		"renderCallText",
		"renderResultText",
		"TOOL_LABELS",
		"ARG_KEYS",
		"normalizeToolName",
		"biggz-memory-chrome",
	} {
		if !strings.Contains(s, need) {
			t.Errorf("biggz-memory-chrome.js missing %q", need)
		}
	}
	// Prefix stripping
	if !strings.Contains(s, "replace(/^biggz_/") {
		t.Errorf("missing prefix stripping via name.replace(/^biggz_/")
	}
	// Cover biggz's 22 tools
	for _, tool := range []string{
		"mem_save", "mem_search", "mem_get_observation", "mem_update", "mem_delete",
		"mem_context", "mem_session_summary", "mem_session_start", "mem_session_end",
		"mem_save_prompt", "mem_current_project", "mem_suggest_topic_key", "mem_timeline",
		"mem_stats", "mem_pin", "mem_unpin", "mem_doctor", "mem_compare", "mem_judge",
		"mem_capture_passive", "mem_merge_projects", "mem_review",
	} {
		if !strings.Contains(s, tool) {
			t.Errorf("TOOL_LABELS/ARG_KEYS missing tool %q", tool)
		}
	}
	// Biggz-specific ARG_KEYS
	for _, key := range []string{"title", "content", "topic_key", "project", "scope", "session_id", "tool_name", "pinned"} {
		if !strings.Contains(s, key) {
			t.Errorf("ARG_KEYS missing biggz-specific key %q", key)
		}
	}
	// Text/json duality
	for _, need := range []string{"firstTextContent", "resultData", "isError", "isPartial"} {
		if !strings.Contains(s, need) {
			t.Errorf("missing duality handling %q", need)
		}
	}
	// Pi extension wrapper
	if !strings.Contains(s, "export default function") || !strings.Contains(s, "pi.on") {
		t.Errorf("missing pi extension wrapper export default function + pi.on")
	}
	if !strings.Contains(s, "PI_SUBAGENT_CHILD") {
		t.Errorf("missing PI_SUBAGENT_CHILD guard")
	}
	// Lightweight check
	if len(s) > 20000 {
		t.Errorf("extension too large %d bytes, want <20000 (<200 LOC)", len(s))
	}
}

func TestDeployPiMemoryChrome(t *testing.T) {
	home := t.TempDir()
	// Normal deploy
	ok, err := install.DeployPiMemoryChrome(home, nil)
	if err != nil {
		t.Fatalf("DeployPiMemoryChrome: %v", err)
	}
	if !ok {
		t.Fatalf("expected true")
	}
	target := filepath.Join(home, ".pi", "agent", "extensions", "biggz-memory-chrome.js")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("file not created at %q: %v", target, err)
	}
	s := string(data)
	if !strings.Contains(s, "renderCallText") || !strings.Contains(s, "biggz_mem_") || !strings.Contains(s, "normalizeToolName") {
		t.Errorf("deployed file content incorrect: %q", s[:500])
	}
	// Dry-run should not write
	home2 := t.TempDir()
	ok, err = install.DeployPiMemoryChrome(home2, nil, true)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !ok {
		t.Fatalf("dry-run expected true")
	}
	if _, err := os.Stat(filepath.Join(home2, ".pi", "agent", "extensions", "biggz-memory-chrome.js")); err == nil {
		t.Errorf("dry-run should not write file")
	}
	// Custom FS fallback — provide empty FS, should fallback to assets.FS
	home3 := t.TempDir()
	emptyFS := fstest.MapFS{}
	ok, err = install.DeployPiMemoryChrome(home3, emptyFS)
	if err != nil {
		t.Fatalf("fallback: %v", err)
	}
	if !ok {
		t.Fatalf("fallback expected true")
	}
	if _, err := os.Stat(filepath.Join(home3, ".pi", "agent", "extensions", "biggz-memory-chrome.js")); err != nil {
		t.Fatalf("fallback should still write via assets.FS: %v", err)
	}
}

func TestDeployPiMemoryChrome_PIEnvOverride(t *testing.T) {
	tmpHome := t.TempDir()
	override := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", override)
	ok, err := install.DeployPiMemoryChrome(tmpHome, nil)
	if err != nil {
		t.Fatalf("env override: %v", err)
	}
	if !ok {
		t.Fatalf("expected true")
	}
	overridePath := filepath.Join(override, "extensions", "biggz-memory-chrome.js")
	if _, err := os.Stat(overridePath); err != nil {
		t.Fatalf("expected file at PI_CODING_AGENT_DIR override %q: %v", overridePath, err)
	}
	defaultPath := filepath.Join(tmpHome, ".pi", "agent", "extensions", "biggz-memory-chrome.js")
	if _, err := os.Stat(defaultPath); err == nil {
		t.Errorf("should not write to default path when PI_CODING_AGENT_DIR is set")
	}
}

func TestMemoryChromeRendering_Node(t *testing.T) {
	// Verify rendering via Node if available — mimics Pi collapsed/expanded TUI
	node, err := exec.LookPath("node")
	if err != nil {
		node, err = exec.LookPath("node.exe")
		if err != nil {
			t.Skip("node not found, skipping rendering test")
		}
	}
	// Build JS snippet that imports the chrome and tests key renderings
	// Use dynamic import via file path — need to write a temp mjs that imports the asset
	assetData, err := fs.ReadFile(assets.FS, "pi/biggz-memory-chrome.js")
	if err != nil {
		t.Fatalf("read asset: %v", err)
	}
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "chrome.mjs")
	if err := os.WriteFile(tmpFile, assetData, 0644); err != nil {
		t.Fatalf("write tmp: %v", err)
	}
	testScript := filepath.Join(tmpDir, "test.mjs")
	script := `
import { humanToolName, compactToolArg, compactResultStatus, renderCallText, renderResultText } from "./chrome.mjs";
function assert(cond, msg) { if (!cond) { console.error("FAIL: "+msg); process.exit(1); } }
function check() {
  // prefix stripping: biggz_mem_save should map same as mem_save
  assert(humanToolName("biggz_mem_save") === "save", "humanToolName prefix");
  assert(humanToolName("mem_save") === "save", "humanToolName base");
  assert(compactToolArg("biggz_mem_save", {title:"hello world"}) === "“hello world”", "compactToolArg prefix");
  assert(compactToolArg("mem_save", {title:"hello world"}) === "“hello world”", "compactToolArg base");
  // collapsed call
  const call = renderCallText("biggz_mem_save", {title:"My Task", type:"decision"});
  assert(call.includes("🧠") && call.includes("save") && call.includes("My Task"), "renderCallText collapsed: "+call);
  const call2 = renderCallText("mem_search", {query:"foo bar"});
  assert(call2.includes("search") && call2.includes("foo bar"), "renderCall search: "+call2);
  // collapsed result for textResult Saved: ... (biggz-mcp)
  const res = { content: [{type:"text", text:"Saved: My Task (id: obs-abc123)"}], details: {} };
  const status = compactResultStatus("biggz_mem_save", res);
  assert(status.includes("✓") && status.includes("Saved"), "compactResultStatus text: "+status);
  const expanded = renderResultText("biggz_mem_save", res, {expanded:true});
  assert(expanded.includes("↳") && expanded.includes("Saved: My Task"), "renderResultText expanded: "+expanded);
  const collapsed = renderResultText("biggz_mem_save", res, {expanded:false});
  assert(collapsed.startsWith("↳") && !collapsed.includes("Saved: My Task") || collapsed.includes("✓"), "collapsed should not show full text beyond status: "+collapsed);
  // json result for search
  const searchRes = { content: [{type:"json", json: [{id:"1"},{id:"2"}]}], details: { data: [{id:"1"},{id:"2"}] } };
  // Simulate json path via details.data
  const searchStatus = compactResultStatus("mem_search", { details: { data: [{id:"1"},{id:"2"}] } });
  assert(searchStatus === "✓ 2 results", "search count: "+searchStatus);
  // get_observation
  const getRes = { details: { data: {id:"obs-xyz"} } };
  const getStatus = compactResultStatus("mem_get_observation", getRes);
  assert(getStatus === "✓ observation #obs-xyz", "get_observation: "+getStatus);
  console.log("PASS");
}
check();
`
	if err := os.WriteFile(testScript, []byte(script), 0644); err != nil {
		t.Fatalf("write script: %v", err)
	}
	cmd := exec.Command(node, testScript)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node rendering test failed: %v\noutput: %s", err, string(out))
	}
	if !strings.Contains(string(out), "PASS") {
		t.Fatalf("unexpected output: %s", string(out))
	}
}
