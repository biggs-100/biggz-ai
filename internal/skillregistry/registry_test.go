package skillregistry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRefresh(t *testing.T) {
	root := t.TempDir()

	// Create a skill
	skillsDir := filepath.Join(root, "skills", "test-skill")
	os.MkdirAll(skillsDir, 0755)
	skillContent := "---\nname: test-skill\ndescription: A test skill for unit testing\n---\n# Test Skill\n"
	os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte(skillContent), 0644)

	result, err := Refresh(root)
	if err != nil {
		t.Fatalf("Refresh() error: %v", err)
	}
	if !result.Regenerated {
		t.Error("expected Regenerated = true")
	}
	if result.SkillCount == 0 {
		t.Error("expected at least 1 skill")
	}

	// Read registry
	data, err := os.ReadFile(filepath.Join(root, ".atl", "skill-registry.md"))
	if err != nil {
		t.Fatalf("read registry: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "test-skill") {
		t.Errorf("registry should contain test-skill, got: %s", content)
	}
	if !strings.Contains(content, "A test skill for unit testing") {
		t.Errorf("registry should contain description")
	}
}

func TestRefresh_NoSkills(t *testing.T) {
	// Use a temp dir as both project root and "home" to isolate from real skills
	tmpHome := t.TempDir()
	oldHome := os.Getenv("HOME")
	oldProfile := os.Getenv("USERPROFILE")
	os.Setenv("HOME", tmpHome)
	os.Setenv("USERPROFILE", tmpHome)
	defer func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("USERPROFILE", oldProfile)
	}()

	result, err := Refresh(tmpHome)
	if err != nil {
		t.Fatalf("Refresh() error: %v", err)
	}
	if result.SkillCount != 0 {
		t.Errorf("expected 0 skills in empty project, got %d", result.SkillCount)
	}
}

func TestExtractDescription(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "SKILL.md")

	// With frontmatter
	os.WriteFile(path, []byte("---\nname: test\ndescription: Custom description\n---\n# Content"), 0644)
	if got := extractDescription(path); got != "Custom description" {
		t.Errorf("expected 'Custom description', got %q", got)
	}

	// Without frontmatter
	os.WriteFile(path, []byte("# Just a title\n\nSome content"), 0644)
	if got := extractDescription(path); got != "Just a title" {
		t.Errorf("expected 'Just a title', got %q", got)
	}
}

func TestScanDir_Excluded(t *testing.T) {
	root := t.TempDir()
	seen := map[string]bool{}

	// _shared should be excluded
	os.MkdirAll(filepath.Join(root, "_shared"), 0755)
	os.WriteFile(filepath.Join(root, "_shared", "SKILL.md"), []byte("# shared"), 0644)

	entries := scanDir(root, seen, root)
	for _, e := range entries {
		if e.Name == "_shared" {
			t.Error("_shared should be excluded")
		}
	}
}
