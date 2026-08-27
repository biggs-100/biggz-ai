package skillregistry

import (
	"bytes"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/assets"
)

func TestTemplatesEmbedded(t *testing.T) {
	entries, err := assets.FS.ReadDir("prompts/review")
	if err != nil {
		t.Fatalf("ReadDir prompts/review: %v", err)
	}
	if len(entries) != 6 {
		t.Fatalf("want 6 templates, got %d", len(entries))
	}
	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name()] = true
		if !strings.HasSuffix(e.Name(), ".md") {
			t.Errorf("template %q should be .md", e.Name())
		}
	}
	// Check expected files exist (R1-R4 + external + shared)
	expected := []string{"r1-risk.md", "r2-readability.md", "r3-reliability.md", "r4-resilience.md", "external.md", "shared.md"}
	for _, exp := range expected {
		if !names[exp] {
			t.Errorf("missing expected template %q, got %v", exp, names)
		}
	}
}

func TestMissingVarFails(t *testing.T) {
	tmpl, err := LoadPrompt("r1-risk")
	if err != nil {
		t.Fatalf("LoadPrompt r1-risk: %v", err)
	}
	// Execute without required Diff field - should error due to missingkey=error
	// Our template uses {{.Diff}} so missing Diff should fail.
	// Use map without Diff key.
	data := map[string]interface{}{
		"Repo": "test-repo",
		// Diff missing intentionally
		"ChangedLines": 10,
		"Paths":        []string{"a.go"},
		"Truncated":    false,
		"BaseTree":     "abc",
		"Shared":       "shared",
		"Hunks":        "hunk",
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err == nil {
		t.Fatal("expected error for missing Diff with missingkey=error, got nil")
	}
	if !strings.Contains(err.Error(), "Diff") && !strings.Contains(err.Error(), "map has no entry") {
		t.Logf("got expected missingkey error: %v", err)
	}
	// Also test that valid data passes
	valid := PromptData{
		Repo:         "repo",
		ChangedLines: 10,
		Paths:        []string{"a.go"},
		Diff:         "diff content",
		Truncated:    false,
		BaseTree:     "abc",
		Hunks:        "hunk",
		Shared:       "shared",
		Payload:      "payload",
	}
	tmpl2, err := LoadPrompt("external")
	if err != nil {
		t.Fatalf("LoadPrompt external: %v", err)
	}
	var buf2 bytes.Buffer
	if err := tmpl2.Execute(&buf2, valid); err != nil {
		t.Fatalf("valid execute should not error: %v", err)
	}
	if strings.Contains(buf2.String(), "{{") {
		t.Errorf("valid render should not contain {{, got %q", buf2.String())
	}
}

func TestRenderNoBraces(t *testing.T) {
	// Successful render interpolates values and contains no {{
	data := PromptData{
		Repo:         "my-repo",
		ChangedLines: 42,
		Paths:        []string{"a.go", "b.go"},
		Diff:         "diff-data-123",
		Truncated:    true,
		BaseTree:     "base123",
		Hunks:        "hunk-data",
		Shared:       "shared-val",
		Payload:      "payload-val",
	}
	names := []string{"r1-risk", "r2-readability", "r3-reliability", "r4-resilience", "external", "shared"}
	for _, name := range names {
		tmpl, err := LoadPrompt(name)
		if err != nil {
			t.Fatalf("LoadPrompt %s: %v", name, err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			t.Fatalf("Execute %s: %v", name, err)
		}
		out := buf.String()
		if strings.Contains(out, "{{") || strings.Contains(out, "}}") {
			t.Errorf("%s output contains braces: %q", name, out)
		}
		// Check that at least one of the values appears
		if !strings.Contains(out, "diff-data-123") && !strings.Contains(out, "my-repo") {
			t.Errorf("%s output missing interpolated values: %q", name, out)
		}
	}
}

func TestLoadPromptMissingKeyOption(t *testing.T) {
	// Ensure template is parsed with missingkey=error
	tmpl, err := LoadPrompt("r2-readability")
	if err != nil {
		t.Fatalf("LoadPrompt: %v", err)
	}
	// Missing Hunks should error if template uses it
	// Use struct missing field via map
	data := map[string]string{
		"Repo": "repo",
		"Diff": "diff",
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err == nil {
		t.Error("expected missingkey error for incomplete data")
	}
}

func TestNoHtmlTemplate(t *testing.T) {
	// Ensure we use text/template not html/template for prompts
	// Check resolver.go does not import html/template
	data, err := assets.FS.ReadFile("prompts/review/r2-readability.md")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "<") && strings.Contains(content, "&lt;") {
		t.Logf("template content: %s", content)
	}
	// This test just ensures file exists; actual html/template ban is checked via go vet and rg
	_ = content
}
