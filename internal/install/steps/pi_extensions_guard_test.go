package steps

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/biggs-100/biggz-ai/internal/agents"
	"github.com/biggs-100/biggz-ai/internal/assets"
	"github.com/biggs-100/biggz-ai/internal/pipeline"
	"github.com/biggs-100/biggz-ai/plugintest"
)

// TestPiExtensionsGuard_FactoryExport ensures every deployed pi extension asset
// exports a valid pi factory. Regression guard for the biggz-session-guard.js
// crash: pi loads every direct extensions/*.js|*.ts as an extension and throws
// "Extension does not export a valid factory function" when one has no
// `export default function(pi)`.
//
// The mirror list carries the recorded factory flag and this test checks all 17
// factories, not just the 13 JS ones: ask-user-choice.ts, codegraph-tools.ts and
// skill-registry.ts were deployed and unchecked before this change, so the guard
// is stricter, not looser.
func TestPiExtensionsGuard_FactoryExport(t *testing.T) {
	// Must mirror the deploy list in pi_extensions.go (17 factories).
	deployList := []piExtension{
		{asset: "pi/biggz-thinking-wrap.js", target: "biggz-thinking-wrap.js", factory: true},
		{asset: "pi/biggz-memory-chrome.js", target: "biggz-memory-chrome.js", factory: true},
		{asset: "pi/biggz-tool-interception.js", target: "biggz-tool-interception.js", factory: true},
		{asset: "pi/biggz-extension-api.js", target: "biggz-extension-api.js", factory: true},
		{asset: "pi/biggz-session-guard.js", target: "biggz-session-guard.js", factory: true},
		{asset: "pi/biggz-ask-guard.js", target: "biggz-ask-guard.js", factory: true},
		{asset: "pi/biggz-last-model.js", target: "biggz-last-model.js", factory: true},
		{asset: "pi/biggz-subagent-runtime.js", target: "biggz-subagent-runtime.js", factory: true},
		{asset: "pi/biggz-shell.ts", target: "biggz-shell.ts", factory: true},
		{asset: "pi/biggz-todo.ts", target: "biggz-todo.ts", factory: true},
		{asset: "pi/biggz-tool-pills.js", target: "biggz-tool-pills.js", factory: true},
		{asset: "pi/biggz-web-search.js", target: "biggz-web-search.js", factory: true},
		{asset: "pi/biggz-question-mouse.js", target: "biggz-question-mouse.js", factory: true},
		{asset: "pi/biggz-quiet-tools.js", target: "biggz-quiet-tools.js", factory: true},
		{asset: "pi/ask-user-choice.ts", target: "ask-user-choice.ts", factory: true},
		{asset: "pi/codegraph-tools.ts", target: "codegraph-tools.ts", factory: true},
		{asset: "pi/skill-registry.ts", target: "skill-registry.ts", factory: true},
	}
	factoryCount := 0
	for _, e := range deployList {
		if !e.factory {
			continue
		}
		factoryCount++
		data, err := fs.ReadFile(assets.FS, e.asset)
		if err != nil {
			t.Fatalf("missing pi extension asset %s: %v", e.asset, err)
		}
		content := string(data)
		if !strings.Contains(content, "export default") {
			t.Errorf("pi extension %s missing factory: must contain 'export default' (pi requires valid factory) — first 200 bytes: %q", e.asset, truncateForTest(content, 200))
		}
		if !strings.Contains(content, "export default function") {
			t.Errorf("pi extension %s missing factory function: must contain 'export default function' (pi loader expects factory function) — asset %s", e.asset, e.asset)
		}
	}
	if factoryCount != 17 {
		t.Errorf("deploy list drift: expected 17 pi extension factories, got %d — the three TS factories were deployed and unchecked before this change, so the guard is stricter, not looser; sync with pi_extensions.go and biggz-pi-extensions-factory.test.mjs", factoryCount)
	}
	// Also verify our canonical helper lists the same factory count — catches
	// drift where pi_extensions.go was updated but this test wasn't (or vice
	// versa).
	if helperCount := countFactories(piExtensionsDeployList()); helperCount != factoryCount {
		t.Errorf("helper drift: piExtensionsDeployList() has %d factories, test list has %d — keep them in sync", helperCount, factoryCount)
	}
}

// TestPiExtensionsDeployList_ShellBeforeTodo pins the load-order dependency that
// keeps the sidebar-visibility race in the shell/todo extensions benign.
//
// biggz-shell.ts writes sidebarState(tui).visibility = { todo: ... } inside its
// footer factory, which pi runs while the extension loads. biggz-todo.ts
// registers its card from its own session_start, which only runs after every
// extension has loaded, so the visibility write happens first and any todo
// widget can only ever read the value already set. That is why the race is
// benign today. Nothing else pins the order: if biggz-todo.ts were deployed
// first, its widget could exist before the shell sets visibility, silently
// turning the benign race into a visible defect. This assertion fails loudly on
// such a reordering.
func TestPiExtensionsDeployList_ShellBeforeTodo(t *testing.T) {
	shellIdx, todoIdx := -1, -1
	for i, e := range piExtensionsDeployList() {
		switch e.asset {
		case "pi/biggz-shell.ts":
			shellIdx = i
		case "pi/biggz-todo.ts":
			todoIdx = i
		}
	}
	if shellIdx == -1 || todoIdx == -1 {
		t.Fatalf("deploy list must contain both pi/biggz-shell.ts (index %d) and pi/biggz-todo.ts (index %d) to evaluate the ordering guard", shellIdx, todoIdx)
	}
	if shellIdx > todoIdx {
		t.Errorf("pi/biggz-shell.ts (index %d) must be deployed before pi/biggz-todo.ts (index %d): the shell footer factory sets sidebarState(tui).visibility.todo during load and the todo card is registered later from session_start, so loading todo first would expose the visibility write after the widget exists", shellIdx, todoIdx)
	}
}

// TestPiExtensionsStep_RemovesRetiredWaitPretty ensures the stale self-heal
// removes every name in retiredPiExtensionTargets — biggz-wait-pretty.js
// (retired with the j0k3r cutover, replaced by the deployed subagent runtime),
// biggz-synthesis-gate.js (the native Go gate is the only enforcement path) and
// biggz-footer.js (retired in unit 2.4, replaced by the deployed shell
// extension) — and that a second Apply with no stale files is a silent no-op.
// The seed list is derived from that variable instead of repeated here, so a new
// retired target cannot be removed by Apply without this test exercising it.
func TestPiExtensionsStep_RemovesRetiredWaitPretty(t *testing.T) {
	tmp := t.TempDir()
	agent := &plugintest.FakeAgent{Installed: true, AgentID: agents.AgentPi}
	agent.SetTempDir(tmp)
	extDir := piExtensionsDir(tmp)
	if err := os.MkdirAll(extDir, 0o755); err != nil {
		t.Fatalf("mkdir extDir: %v", err)
	}
	stale := append([]string(nil), retiredPiExtensionTargets...)
	if len(stale) == 0 {
		t.Fatal("retiredPiExtensionTargets is empty: the seed list this test proves against would be empty too")
	}
	for _, name := range stale {
		if err := os.WriteFile(filepath.Join(extDir, name), []byte("stale"), 0o644); err != nil {
			t.Fatalf("seed stale %s: %v", name, err)
		}
	}
	p := NewPiExtensionsStep(tmp, agent, false)
	if err := p.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if err := p.Apply(context.Background(), make(pipeline.ProgressChan, 32)); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for _, name := range stale {
		if _, err := os.Stat(filepath.Join(extDir, name)); !os.IsNotExist(err) {
			t.Errorf("retired extension %s still present after Apply (err=%v)", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(extDir, "biggz-subagent-runtime.js")); err != nil {
		t.Errorf("subagent runtime must be deployed in place of the retired wait shim: %v", err)
	}
	// Missing stale files are silent no-ops (os.Remove error ignored).
	if err := p.Apply(context.Background(), make(pipeline.ProgressChan, 32)); err != nil {
		t.Errorf("second Apply with no stale files must succeed silently, got %v", err)
	}
}

func countFactories(list []piExtension) int {
	n := 0
	for _, e := range list {
		if e.factory {
			n++
		}
	}
	return n
}

func truncateForTest(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// TestPiExtensionsPrepare_RejectsBrokenFactory ensures validatePiExtensionsFactory
// fails fast on a broken factory asset rather than deploying it and crashing pi.
// The role comes from the recorded factory flag, so a broken TS factory is now
// rejected too — the regression this unit fixes.
func TestPiExtensionsPrepare_RejectsBrokenFactory(t *testing.T) {
	validStub := "export default function valid(pi){ if(typeof pi?.on==='function') pi.on('session_stop', async()=>{}); }\n"
	helperStub := "export const SHELL_WIDTH = 80;\n"
	brokenContent := "// no factory here\nconsole.log('broken');\n"

	validFS := fstest.MapFS{}
	brokenJS := fstest.MapFS{}
	brokenTS := fstest.MapFS{}
	for _, e := range piExtensionsDeployList() {
		if !e.factory {
			// Helpers must not look like extensions.
			validFS[e.asset] = &fstest.MapFile{Data: []byte(helperStub)}
			brokenJS[e.asset] = &fstest.MapFile{Data: []byte(helperStub)}
			brokenTS[e.asset] = &fstest.MapFile{Data: []byte(helperStub)}
			continue
		}
		validFS[e.asset] = &fstest.MapFile{Data: []byte(validStub)}
		if e.asset == "pi/biggz-session-guard.js" {
			brokenJS[e.asset] = &fstest.MapFile{Data: []byte(brokenContent)}
		} else {
			brokenJS[e.asset] = &fstest.MapFile{Data: []byte(validStub)}
		}
		if e.asset == "pi/skill-registry.ts" {
			brokenTS[e.asset] = &fstest.MapFile{Data: []byte(brokenContent)}
		} else {
			brokenTS[e.asset] = &fstest.MapFile{Data: []byte(validStub)}
		}
	}

	if err := validatePiExtensionsFactory(validFS); err != nil {
		t.Fatalf("valid synthetic FS should pass factory validation, got: %v", err)
	}
	if err := validatePiExtensionsFactory(assets.FS); err != nil {
		t.Fatalf("valid embedded assets should pass factory validation, got: %v", err)
	}

	jsErr := validatePiExtensionsFactory(brokenJS)
	if jsErr == nil {
		t.Fatalf("expected validatePiExtensionsFactory to reject broken JS factory, got nil")
	}
	if !strings.Contains(jsErr.Error(), "biggz-session-guard.js") || !strings.Contains(jsErr.Error(), "export default") {
		t.Fatalf("error should mention missing factory and asset name, got: %v", jsErr)
	}

	// Regression: a broken TS factory must be rejected even though the target is
	// not .js. Before this unit the suffix rule skipped it, so a broken TS
	// factory would crash pi on startup with nothing in the installer catching it.
	tsErr := validatePiExtensionsFactory(brokenTS)
	if tsErr == nil {
		t.Fatalf("expected validatePiExtensionsFactory to reject broken TS factory pi/skill-registry.ts, got nil — the factory role must not be inferred from the file extension")
	}
	if !strings.Contains(tsErr.Error(), "skill-registry.ts") || !strings.Contains(tsErr.Error(), "export default") {
		t.Fatalf("error should mention the broken TS factory and missing export, got: %v", tsErr)
	}
}

// TestPiExtensionsLayout_Targets covers the pure target rules: helpers must be
// under the shell subdirectory, factories must be flat, no helper may be an
// index file (which would make pi load the subdirectory as an extension), and
// targets must be unique.
func TestPiExtensionsLayout_Targets(t *testing.T) {
	if err := validatePiExtensionsTargets(piExtensionsDeployList()); err != nil {
		t.Fatalf("canonical deploy list must satisfy the layout rules, got: %v", err)
	}
	hybrid := []piExtension{
		{asset: "pi/biggz-footer.js", target: "biggz-footer.js", factory: true},
		{asset: "pi/biggz-shell/shell-card.js", target: piShellSubdir + "/shell-card.js", factory: false},
	}
	if err := validatePiExtensionsTargets(hybrid); err != nil {
		t.Fatalf("hybrid list of one flat factory and one nested helper must pass, got: %v", err)
	}

	cases := []struct {
		name string
		list []piExtension
		want string
	}{
		{
			name: "helper at the extensions root",
			list: []piExtension{{asset: "pi/shell-card.js", target: "shell-card.js", factory: false}},
			want: piShellSubdir,
		},
		{
			name: "helper index file in the subdirectory",
			list: []piExtension{{asset: "pi/" + piShellSubdir + "/index.ts", target: piShellSubdir + "/index.ts", factory: false}},
			want: "index",
		},
		{
			name: "factory with a nested target",
			list: []piExtension{{asset: "pi/biggz-footer.js", target: "nested/biggz-footer.js", factory: true}},
			want: "direct child",
		},
		{
			name: "duplicate target",
			list: []piExtension{
				{asset: "pi/biggz-footer.js", target: "biggz-footer.js", factory: true},
				{asset: "pi/biggz-footer-copy.js", target: "biggz-footer.js", factory: true},
			},
			want: "duplicate",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePiExtensionsTargets(tc.list)
			if err == nil {
				t.Fatalf("validatePiExtensionsTargets(%s) = nil, want an error mentioning %q", tc.name, tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error for %s should mention %q, got: %v", tc.name, tc.want, err)
			}
		})
	}
}

// TestPiExtensionsLayout_ShellSubdirManifest covers the manifest rule: a
// pi/biggz-shell/package.json that declares a `pi` field makes pi load the
// subdirectory as an extension, so it must be rejected. A manifest without the
// field, and an absent manifest, both pass.
func TestPiExtensionsLayout_ShellSubdirManifest(t *testing.T) {
	manifest := "pi/" + piShellSubdir + "/package.json"
	withPi := fstest.MapFS{
		manifest: &fstest.MapFile{Data: []byte(`{"name":"biggz-shell","pi":{"extensions":["index.js"]}}`)},
	}
	if err := validatePiShellSubdirManifest(withPi); err == nil {
		t.Fatalf("manifest %s declaring a `pi` field must be rejected: pi would load %s/ as an extension", manifest, piShellSubdir)
	} else if !strings.Contains(err.Error(), "`pi` field") {
		t.Fatalf("error should name the offending `pi` field, got: %v", err)
	}
	withoutPi := fstest.MapFS{
		manifest: &fstest.MapFile{Data: []byte(`{"name":"biggz-shell","private":true}`)},
	}
	if err := validatePiShellSubdirManifest(withoutPi); err != nil {
		t.Fatalf("manifest without a `pi` field must pass, got: %v", err)
	}
	if err := validatePiShellSubdirManifest(fstest.MapFS{}); err != nil {
		t.Fatalf("absent manifest must pass, got: %v", err)
	}
	if err := validatePiExtensionsLayout(assets.FS); err != nil {
		t.Fatalf("embedded assets must satisfy the layout rules, got: %v", err)
	}
}

// TestPiExtensionsGuard_ValidateHelperCoversAll ensures the helper and the
// factory test agree on the asset set. If pi_extensions.go adds a new extension
// without updating the factory test, either this or the factory test will fail.
func TestPiExtensionsGuard_ValidateHelperCoversAll(t *testing.T) {
	assetsList := piExtensionsDeployList()
	// Ensure every entry in helper is readable from embedded FS (except when
	// testing with synthetic FS) — here we check real embedded FS.
	for _, e := range assetsList {
		if _, err := fs.Stat(assets.FS, e.asset); err != nil {
			t.Errorf("helper lists %s but embedded FS missing it: %v — add file or remove from deploy list", e.asset, err)
		}
	}
}
