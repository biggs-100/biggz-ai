package filemerge

import (
	"strings"
	"testing"
)

func TestInjectSection_EmptyContent(t *testing.T) {
	result, err := InjectSection("", "test", []byte("hello"))
	if err != nil {
		t.Fatalf("InjectSection() returned error: %v", err)
	}

	expected := "<!-- section:test -->\nhello\n<!-- /section -->"
	if string(result) != expected {
		t.Errorf("InjectSection() = %q, want %q", string(result), expected)
	}
}

func TestInjectSection_ExistingSections(t *testing.T) {
	content := `# README

<!-- section:existing -->
existing content
<!-- /section -->
`
	newSection := []byte("new content")

	result, err := InjectSection(content, "new-section", newSection)
	if err != nil {
		t.Fatalf("InjectSection() returned error: %v", err)
	}

	if !strings.Contains(string(result), "<!-- section:existing -->") {
		t.Error("InjectSection() removed existing section marker")
	}
	if !strings.Contains(string(result), "<!-- section:new-section -->") {
		t.Error("InjectSection() did not add new section marker")
	}
	if !strings.Contains(string(result), "new content") {
		t.Error("InjectSection() did not include new section content")
	}
	if !strings.Contains(string(result), "existing content") {
		t.Error("InjectSection() removed existing content")
	}
}

func TestReplaceSection_Existing(t *testing.T) {
	content := `# README

<!-- section:config -->
old config
<!-- /section -->

More text.
`
	newSection := []byte("new config value")

	result, err := ReplaceSection(content, "config", newSection)
	if err != nil {
		t.Fatalf("ReplaceSection() returned error: %v", err)
	}

	if strings.Contains(string(result), "old config") {
		t.Error("ReplaceSection() did not replace old content")
	}
	if !strings.Contains(string(result), "new config value") {
		t.Error("ReplaceSection() did not include new content")
	}
	if !strings.Contains(string(result), "<!-- section:config -->") {
		t.Error("ReplaceSection() removed opening marker")
	}
	if !strings.Contains(string(result), "<!-- /section -->") {
		t.Error("ReplaceSection() removed closing marker")
	}
	if !strings.Contains(string(result), "More text.") {
		t.Error("ReplaceSection() removed content after section")
	}
}

func TestReplaceSection_NotFound(t *testing.T) {
	content := "# Just a readme\nwith no sections\n"

	_, err := ReplaceSection(content, "missing", []byte("content"))
	if err == nil {
		t.Fatal("ReplaceSection() expected error for missing section, got nil")
	}
}

func TestReplaceSection_MissingClosingMarker(t *testing.T) {
	content := "<!-- section:broken -->\nno closing marker\n"

	_, err := ReplaceSection(content, "broken", []byte("content"))
	if err == nil {
		t.Fatal("ReplaceSection() expected error for missing closing marker, got nil")
	}
}

func TestInjectSection_NoTrailingNewline(t *testing.T) {
	// Content without trailing newline
	content := `# README
Some text without trailing newline`
	newSection := []byte("appended")

	result, err := InjectSection(content, "appendix", newSection)
	if err != nil {
		t.Fatalf("InjectSection() returned error: %v", err)
	}

	if !strings.Contains(string(result), "<!-- section:appendix -->") {
		t.Error("InjectSection() did not add section marker")
	}
	// Verify original content is preserved
	if !strings.HasPrefix(string(result), content) {
		t.Errorf("InjectSection() result does not start with original content:\n%s", string(result))
	}
}
