package install_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"io/fs"

	"github.com/biggs-100/biggz-ai/internal/assets"
)

func TestQuietToolsRendering_Node(t *testing.T) {
	// Verify quiet collapsed rendering via Node — mirrors gentle-pi
	// quiet-tool-rendering.test.ts collapsed contracts.
	// NOTE: the inline JS below avoids backslash escapes entirely (NL/ESC
	// constants) so no transport layer can mangle newlines.
	node, err := exec.LookPath("node")
	if err != nil {
		node, err = exec.LookPath("node.exe")
		if err != nil {
			t.Skip("node not found, skipping rendering test")
		}
	}
	quietData, err := fs.ReadFile(assets.FS, "pi/biggz-quiet-tools.js")
	if err != nil {
		t.Fatalf("read asset: %v", err)
	}
	chromeData, err := fs.ReadFile(assets.FS, "pi/biggz-memory-chrome.js")
	if err != nil {
		t.Fatalf("read chrome asset: %v", err)
	}
	tmpDir := t.TempDir()
	// quiet-tools.js imports ./biggz-memory-chrome.js — stage both with exact names.
	if err := os.WriteFile(filepath.Join(tmpDir, "biggz-quiet-tools.mjs"), quietData, 0644); err != nil {
		t.Fatalf("write tmp: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "biggz-memory-chrome.js"), chromeData, 0644); err != nil {
		t.Fatalf("write tmp: %v", err)
	}
	testScript := filepath.Join(tmpDir, "test.mjs")
	script := `
import quietDefault, { QUIET_TOOLS, quietResultText, countNonEmptyLines, extractText, isQuietEnabled } from "./biggz-quiet-tools.mjs";
const NL = String.fromCharCode(10);
const ESC = String.fromCharCode(27);
function assert(cond, msg) { if (!cond) { console.error("FAIL: "+msg); process.exit(1); } }
const textRes = (text, details) => ({ content: [{type:"text", text}], details });
function check() {
  for (const need of ["read","bash","powershell","grep","find","ls","edit","write"]) {
    assert(QUIET_TOOLS.includes(need), "QUIET_TOOLS missing "+need);
  }
  const grepOut = ["a:1:x","b:2:y","c:3:z","d:4:w","e:5:v"].join(NL);
  const g = quietResultText("grep", textRes(grepOut), {});
  assert(g.indexOf("5 matches") >= 0, "grep count: "+JSON.stringify(g));
  assert(g.indexOf("a:1:x") < 0 && g.indexOf("e:5:v") >= 0, "grep tail 3: "+JSON.stringify(g));
  assert(quietResultText("grep", textRes("No matches found"), {}) === "", "grep empty silent");
  assert(quietResultText("ls", textRes("Directory is empty"), {}) === "", "ls empty silent");
  assert(quietResultText("find", textRes(["a","b",""].join(NL)), {}).indexOf("2 files") >= 0, "find count");
  const r = quietResultText("read", textRes(["1","2","3","4","5"].join(NL)), {});
  assert(r === ["1","2","3"].join(NL), "read head 3: "+JSON.stringify(r));
  const b = quietResultText("bash", textRes(["1","2","3","4"].join(NL)), {});
  assert(b === ["2","3","4"].join(NL), "bash tail 3: "+JSON.stringify(b));
  const bj = quietResultText("bash", textRes(JSON.stringify([{id:"1",title:"hello"}])), {});
  assert(bj.indexOf("hello") >= 0, "bash json semantic: "+JSON.stringify(bj));
  const gitLines = [];
  for (let i = 0; i < 15; i++) gitLines.push("line"+i);
  const gg = quietResultText("bash", textRes(gitLines.join(NL)), {args:{command:"git status"}});
  assert(gg.split(NL).length === 10 && gg.indexOf("line14") >= 0 && gg.indexOf("line0"+NL) < 0, "git tail 10");
  const e = quietResultText("edit", textRes("ok", {diff:["+a","+b","-c",""].join(NL)}), {});
  assert(e === "✓ +2 / -1", "edit stats: "+e);
  assert(quietResultText("write", textRes("Successfully wrote 42 bytes"), {}) === "✓ wrote 42 bytes", "write bytes");
  const er = quietResultText("bash", textRes(["a","","b","c","d"].join(NL)), {isError:true});
  assert(er === ["b","c","d"].join(NL), "error tail: "+JSON.stringify(er));
  const s = quietResultText("read", textRes([ESC+"[31mred"+ESC+"[0m","plain"].join(NL)), {});
  assert(s.indexOf(ESC) < 0, "sanitized: "+JSON.stringify(s));
  const registered = new Map();
  const executions = new Map();
  const fakePi = {
    getToolDefinition: (name) => {
      const execute = async () => textRes(name+" result");
      executions.set(name, execute);
      return { name, description: name, execute, renderCall: () => "call", renderResult: () => "NATIVE" };
    },
    registerTool: (def) => { registered.set(def.name, def); },
  };
  quietDefault(fakePi);
  assert(registered.size === QUIET_TOOLS.length, "registered all: "+registered.size);
  for (const name of QUIET_TOOLS) {
    const def = registered.get(name);
    assert(def.execute === executions.get(name), name+" execute preserved");
    const collapsed = def.renderResult(textRes("a"+NL+"b"), {expanded:false}, {fg: (c, t) => t}, {});
    assert(typeof collapsed.render === "function", name+" returns component");
    assert(collapsed.render().join(NL).indexOf("NATIVE") < 0, name+" collapsed quiet");
    const expanded = def.renderResult(textRes("a"), {expanded:true}, {}, {});
    assert(expanded === "NATIVE", name+" expanded delegates");
  }
  process.env.BIGGZ_QUIET_TOOLS = "0";
  assert(isQuietEnabled() === false, "opt-out flag");
  let n = 0;
  quietDefault({ registerTool: () => { n++; }, getToolDefinition: () => ({execute: async () => ({})}) });
  assert(n === 0, "opt-out registers nothing");
  delete process.env.BIGGZ_QUIET_TOOLS;
  console.log("PASS");
}
check();
`
	if err := os.WriteFile(testScript, []byte(script), 0644); err != nil {
		t.Fatalf("write tmp script: %v", err)
	}
	cmd := exec.Command(node, testScript)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node test failed: %s", string(out))
	}
	if !strings.Contains(string(out), "PASS") {
		t.Fatalf("unexpected output: %s", string(out))
	}
}
