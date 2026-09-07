package steps

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/biggs-100/biggz-ai/internal/assets"
)

// TestPiExtensionsGuard_FactoryExport ensures every JS pi extension asset
// deployed via PiExtensionsStep exports a valid pi factory. Regression guard
// for biggz-session-guard.js crash: pi loader requires every .js in
// ~/.pi/agent/extensions to export `export default function(pi)` — missing
// factory crashes pi with "Extension does not export a valid factory function".
func TestPiExtensionsGuard_FactoryExport(t *testing.T) {
	// Must mirror the deploy list in pi_extensions.go (12 JS + 3 TS).
	// TS entries are not pi ExtensionAPI factories and are intentionally skipped.
	deployList := []struct{ asset, target string }{
		{"pi/biggz-thinking-wrap.js", "biggz-thinking-wrap.js"},
		{"pi/biggz-memory-chrome.js", "biggz-memory-chrome.js"},
		{"pi/biggz-tool-interception.js", "biggz-tool-interception.js"},
		{"pi/biggz-extension-api.js", "biggz-extension-api.js"},
		{"pi/biggz-session-guard.js", "biggz-session-guard.js"},
		{"pi/biggz-last-model.js", "biggz-last-model.js"},
		{"pi/biggz-synthesis-gate.js", "biggz-synthesis-gate.js"},
		{"pi/biggz-wait-pretty.js", "biggz-wait-pretty.js"},
		{"pi/biggz-footer.js", "biggz-footer.js"},
		{"pi/biggz-tool-pills.js", "biggz-tool-pills.js"},
		{"pi/biggz-web-search.js", "biggz-web-search.js"},
		{"pi/biggz-question-mouse.js", "biggz-question-mouse.js"},
		{"pi/ask-user-choice.ts", "ask-user-choice.ts"},
		{"pi/codegraph-tools.ts", "codegraph-tools.ts"},
		{"pi/skill-registry.ts", "skill-registry.ts"},
	}
	jsCount := 0
	for _, e := range deployList {
		if strings.HasSuffix(e.target, ".js") {
			jsCount++
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
	}
	if jsCount != 12 {
		t.Errorf("deploy list drift: expected 12 JS extensions, got %d — sync with pi_extensions.go and biggz-pi-extensions-factory.test.mjs", jsCount)
	}
	// Also verify our canonical helper lists the same JS count — catches drift
	// where pi_extensions.go was updated but this test wasn't (or vice versa).
	if helperCount := countJS(piExtensionsDeployList()); helperCount != jsCount {
		t.Errorf("helper drift: piExtensionsDeployList() has %d JS, test list has %d — keep them in sync", helperCount, jsCount)
	}
}

func countJS(list []struct{ asset, target string }) int {
	n := 0
	for _, e := range list {
		if strings.HasSuffix(e.target, ".js") {
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
// fails fast on a broken JS asset rather than deploying it and crashing pi.
func TestPiExtensionsPrepare_RejectsBrokenFactory(t *testing.T) {
	// Build a synthetic FS with all 12 JS assets valid except one broken.
	validStub := "export default function broken(pi){ if(typeof pi?.on==='function') pi.on('session_stop', async()=>{}); }\n"
	brokenContent := "// no factory here\nconsole.log('broken');\n"

	validFS := fstest.MapFS{}
	brokenFS := fstest.MapFS{}
	for _, e := range piExtensionsDeployList() {
		if strings.HasSuffix(e.target, ".js") {
			if e.asset == "pi/biggz-session-guard.js" {
				validFS[e.asset] = &fstest.MapFile{Data: []byte(validStub)}
				brokenFS[e.asset] = &fstest.MapFile{Data: []byte(brokenContent)}
			} else {
				validFS[e.asset] = &fstest.MapFile{Data: []byte(validStub)}
				brokenFS[e.asset] = &fstest.MapFile{Data: []byte(validStub)}
			}
		} else {
			// TS assets — content irrelevant, but include empty to satisfy FS reads if any
			validFS[e.asset] = &fstest.MapFile{Data: []byte("export const dummy=1;")}
			brokenFS[e.asset] = &fstest.MapFile{Data: []byte("export const dummy=1;")}
		}
	}

	if err := validatePiExtensionsFactory(validFS); err != nil {
		t.Fatalf("valid synthetic FS should pass factory validation, got: %v", err)
	}
	if err := validatePiExtensionsFactory(assets.FS); err != nil {
		t.Fatalf("valid embedded assets should pass factory validation, got: %v", err)
	}
	err := validatePiExtensionsFactory(brokenFS)
	if err == nil {
		t.Fatalf("expected validatePiExtensionsFactory to reject broken JS asset, got nil")
	}
	if !strings.Contains(err.Error(), "biggz-session-guard.js") || !strings.Contains(err.Error(), "export default") {
		t.Fatalf("error should mention missing factory and asset name, got: %v", err)
	}
}

// TestPiExtensionsGuard_ValidateHelperCoversAll ensures the helper and the
// JS factory test agree on the asset set. If pi_extensions.go adds a new JS
// extension without updating the JS test, either this or the JS test will fail.
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
