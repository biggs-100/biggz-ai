package screens

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Task 4.3 RED: banner-grep test (REQ-WIZ-005). Ported wizard code MUST NOT
// reference branding banners. The check is scoped to code: line (//) and
// block (/* */) comments are stripped before matching so prose mentions in
// comments do not fail the gate. Banned tokens are built by concatenation
// so this file itself stays clean under the same grep.
func TestWizardBannerGrepClean(t *testing.T) {
	banned := []string{
		"Render" + "Logo",
		"Tag" + "line",
		"update" + "Banner",
		"advi" + "sory",
	}

	// Wizard screens + per-agent pickers + router. installing.go keeps the
	// shared progress bar (no banner) and is covered by the same gate.
	files, err := filepath.Glob("wizard_*.go")
	if err != nil {
		t.Fatalf("glob wizard screens: %v", err)
	}
	files = append(files,
		"claude_picker.go",
		"codex_picker.go",
		"kiro_picker.go",
		"opencode_picker.go",
		"installing.go",
		"../router.go",
	)
	if len(files) == 0 {
		t.Fatalf("no wizard files found")
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		code := stripComments(string(raw))
		for _, b := range banned {
			if strings.Contains(code, b) {
				t.Errorf("%s contains banned banner token %q in code", f, b)
			}
		}
	}
}

// stripComments removes // line comments and /* */ block comments so the
// banner gate only inspects real code, not prose in comments.
func stripComments(src string) string {
	var b strings.Builder
	inBlock := false
	for _, line := range strings.Split(src, "\n") {
		if inBlock {
			if end := strings.Index(line, "*/"); end >= 0 {
				line = line[end+2:]
				inBlock = false
			} else {
				continue
			}
		}
		for {
			start := strings.Index(line, "/*")
			if start < 0 {
				break
			}
			if end := strings.Index(line[start+2:], "*/"); end >= 0 {
				line = line[:start] + line[start+2+end+2:]
				continue
			}
			line = line[:start]
			inBlock = true
			break
		}
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = line[:idx]
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}
