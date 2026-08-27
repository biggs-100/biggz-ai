package lens

import (
	"bytes"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/assets"
)

func TestPromptTemplates_Embedded(t *testing.T) {
	entries, err := assets.FS.ReadDir("prompts/review")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 6 {
		t.Fatalf("want 6 templates, got %d", len(entries))
	}
}

func TestPromptMissingVarFails(t *testing.T) {
	tmpl, err := LoadPrompt("r1-risk")
	if err != nil {
		t.Fatalf("LoadPrompt: %v", err)
	}
	// Data missing Diff should error
	data := map[string]interface{}{
		"Repo": "repo",
		// Diff missing
		"ChangedLines": 5,
		"Paths":        []string{"a.go"},
		"Truncated":    false,
		"BaseTree":     "abc",
		"Shared":       "shared",
		"Hunks":        "h",
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err == nil {
		t.Fatal("expected missingkey error for missing Diff")
	}
}

func TestPromptRenderNoBraces(t *testing.T) {
	data := PromptData{
		Repo:         "repo1",
		ChangedLines: 10,
		Paths:        []string{"a.go"},
		Diff:         "my-diff",
		Truncated:    false,
		BaseTree:     "base",
		Hunks:        "hunks",
		Shared:       "shared",
		Payload:      "payload",
	}
	for _, name := range []string{"r1-risk", "r2-readability", "r3-reliability", "r4-resilience", "external", "shared"} {
		out, err := RenderPrompt(name, data)
		if err != nil {
			t.Fatalf("RenderPrompt %s: %v", name, err)
		}
		if strings.Contains(out, "{{") {
			t.Errorf("%s still contains {{: %q", name, out)
		}
		if !strings.Contains(out, "my-diff") && !strings.Contains(out, "repo1") {
			t.Errorf("%s missing interpolated values", name)
		}
	}
}

func TestPromptSuccessfulInterpolates(t *testing.T) {
	tmpl, err := LoadPrompt("shared")
	if err != nil {
		t.Fatalf("LoadPrompt shared: %v", err)
	}
	data := PromptData{
		Repo:         "repoX",
		ChangedLines: 99,
		Paths:        []string{"x.go"},
		Diff:         "diffX",
		Truncated:    true,
		Shared:       "sharedX",
		Hunks:        "hunkX",
		BaseTree:     "baseX",
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "diffX") {
		t.Errorf("output missing diffX: %q", out)
	}
	if !strings.Contains(out, "repoX") {
		t.Errorf("output missing repoX")
	}
	if strings.Contains(out, "{{") {
		t.Errorf("output contains braces")
	}
}
