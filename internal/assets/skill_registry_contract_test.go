// Package assets_test verifies the embedded OpenCode plugin files keep the
// ported contract shapes from gentle-ai while preserving biggz's deliberate
// divergences (quarantine-to-file persistence).
package assets_test

import (
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/assets"
)

func readSkillRegistryPlugin(t *testing.T) string {
	t.Helper()
	data, err := assets.FS.ReadFile("opencode/plugins/skill-registry.ts")
	if err != nil {
		t.Fatalf("Read(skill-registry.ts) error = %v", err)
	}
	return string(data)
}

// TestSkillRegistryPluginContract mirrors gentle-ai's contract for the
// skill-registry startup plugin, adapted to the biggz binary name.
func TestSkillRegistryPluginContract(t *testing.T) {
	source := readSkillRegistryPlugin(t)

	for _, want := range []string{
		`"biggz"`,
		"execFile",
		"skill-registry",
		"refresh",
		"--quiet",
		"--no-gitignore",
		"--cwd",
		"input.directory",
		"input.worktree",
		"timeout: 30_000",
		"console.error",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("skill-registry.ts missing %q", want)
		}
	}
	if strings.Contains(source, "exec(") {
		t.Fatal("skill-registry.ts must use execFile, not shell exec")
	}
	if strings.Contains(source, "gentle-ai") {
		t.Fatal("skill-registry.ts must invoke the biggz binary, not gentle-ai")
	}
}
