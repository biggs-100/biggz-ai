// Package assets_test verifies the embedded OpenCode plugin files keep the
// ported contract shapes from gentle-ai while preserving biggz's deliberate
// divergences (quarantine-to-file persistence).
package assets_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/assets"
)

func readModelVariantsPlugin(t *testing.T) string {
	t.Helper()
	data, err := assets.FS.ReadFile("opencode/plugins/model-variants.ts")
	if err != nil {
		t.Fatalf("Read(model-variants.ts) error = %v", err)
	}
	return string(data)
}

// TestModelVariantsPluginContract mirrors gentle-ai's contract: atomic write
// via tmp+rename, always-write semantics, per-invocation randomized tmp path,
// own-temp cleanup, and visible error logging.
func TestModelVariantsPluginContract(t *testing.T) {
	source := readModelVariantsPlugin(t)

	if !strings.Contains(source, "rename") {
		t.Errorf("model-variants.ts must use rename() for atomic write")
	}
	if !strings.Contains(source, ".tmp") {
		t.Errorf("model-variants.ts must write to a .tmp file before rename()")
	}
	if strings.Contains(source, "Object.keys(variants).length") {
		t.Errorf("model-variants.ts must not gate the write on variants length (allows stale cache to survive)")
	}
	if !strings.Contains(source, "JSON.stringify(variants") {
		t.Errorf("model-variants.ts must serialize the variants object — even when empty — to overwrite stale cache")
	}
	if strings.Contains(source, "} catch {") {
		t.Errorf("model-variants.ts must not have a parameterless `catch {}` block (silences ENOSPC/EACCES)")
	}
	if !strings.Contains(source, "console.error") {
		t.Errorf("model-variants.ts must log errors via console.error so users see failures")
	}
	for _, want := range []string{
		`const MODEL_VARIANTS_CACHE_FILE = "model-variants.json"`,
		"const finalPath = path.join(cacheDir, MODEL_VARIANTS_CACHE_FILE)",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("model-variants.ts missing constant-based cache path contract %q", want)
		}
	}
	tmpPathPattern := regexp.MustCompile("tmpPath\\s*=\\s*path\\.join\\(\\s*cacheDir\\s*,\\s*`\\$\\{\\s*MODEL_VARIANTS_CACHE_FILE\\s*\\}\\.\\$\\{\\s*randomBytes\\([^)]*\\)\\s*\\.\\s*toString\\(\\s*[\"']hex[\"']\\s*\\)\\s*\\}\\.tmp`\\s*\\)")
	if !tmpPathPattern.MatchString(source) {
		t.Errorf("model-variants.ts tmp path must use path.join(cacheDir, randomized basename) to be unique across plugin double-loads within the same process")
	}
	for _, want := range []string{
		"finally",
		"removeOwnTempFile(tmpPath)",
		"await rm(tmpPath, { force: true })",
		"tmpPath = undefined",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("model-variants.ts missing own-temp cleanup contract %q", want)
		}
	}
	for _, forbidden := range []string{
		"removeStaleModelVariantsTempFiles",
		"STALE_TEMP_FILE_AGE_MS",
		"mtimeMs",
		"Date.now()",
	} {
		if strings.Contains(source, forbidden) {
			t.Errorf("model-variants.ts must not use stale temp cleanup by age; found %q", forbidden)
		}
	}
	if strings.Contains(source, "setTimeout") {
		t.Errorf("model-variants.ts must not use setTimeout for temp cleanup")
	}
}
