package skillregistry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPriorityDeterministic(t *testing.T) {
	tmpHome := t.TempDir()
	tmpProject := t.TempDir()
	oldHome := os.Getenv("HOME")
	oldProfile := os.Getenv("USERPROFILE")
	os.Setenv("HOME", tmpHome)
	os.Setenv("USERPROFILE", tmpHome)
	defer func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("USERPROFILE", oldProfile)
	}()

	// Create same skill name in provider 2 (user:biggz) and provider 5 (project:skills)
	// ProviderPriority[1] is user:biggz, [4] is project:skills
	provider2Dir := filepath.Join(tmpHome, ".biggz", "skills", "dup")
	provider5Dir := filepath.Join(tmpProject, "skills", "dup")
	if err := os.MkdirAll(provider2Dir, 0755); err != nil {
		t.Fatalf("mkdir p2: %v", err)
	}
	if err := os.MkdirAll(provider5Dir, 0755); err != nil {
		t.Fatalf("mkdir p5: %v", err)
	}
	if err := os.WriteFile(filepath.Join(provider2Dir, "SKILL.md"), []byte("# Dup P2\n"), 0644); err != nil {
		t.Fatalf("write p2: %v", err)
	}
	if err := os.WriteFile(filepath.Join(provider5Dir, "SKILL.md"), []byte("# Dup P5\n"), 0644); err != nil {
		t.Fatalf("write p5: %v", err)
	}
	// Use helper that respects ProviderPriority order
	entries := scanAllSkillsWithOpts(tmpProject, ScanOpts{})
	found := ""
	for _, e := range entries {
		if e.Name == "dup" {
			found = e.Path
			break
		}
	}
	if found == "" {
		t.Fatal("dup skill not found")
	}
	// Should be from provider 2 (user:biggz) not provider5
	if !containsPath(found, ".biggz") {
		t.Errorf("priority failed: dup path = %q, want .biggz (provider 2 wins over 5)", found)
	}
}

func containsPath(p, substr string) bool {
	return len(p) >= len(substr) && (p == substr || contains(p, substr))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && search(s, substr))
}

func search(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestNonRecursive(t *testing.T) {
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "a")
	nested := filepath.Join(skillDir, "nested")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# A\n"), 0644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "SKILL.md"), []byte("# Nested\n"), 0644); err != nil {
		t.Fatalf("write nested: %v", err)
	}
	entries, err := ScanSkillsFromDir(tmp, ScanOpts{})
	if err != nil {
		t.Fatalf("ScanSkillsFromDir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("non-recursive should return 1, got %d: %+v", len(entries), entries)
	}
	if len(entries) == 1 && entries[0].Name != "a" {
		t.Errorf("Name = %q, want a", entries[0].Name)
	}
	// Ensure nested not returned
	for _, e := range entries {
		if e.Name == "nested" {
			t.Error("nested should not be returned via non-recursive scan")
		}
	}
}

func TestDisabledExtensions(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{"foo", "bar"} {
		dir := filepath.Join(tmp, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name+"\n"), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	opts := ScanOpts{DisabledExtensions: []string{"skill:foo"}}
	entries, err := ScanSkillsFromDir(tmp, opts)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	for _, e := range entries {
		if e.Name == "foo" {
			t.Error("foo should be excluded via disabledExtensions skill:foo")
		}
	}
	foundBar := false
	for _, e := range entries {
		if e.Name == "bar" {
			foundBar = true
		}
	}
	if !foundBar {
		t.Error("bar should remain")
	}
}

func TestGlobFiltering(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{"bar", "bar_test"} {
		dir := filepath.Join(tmp, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name+"\n"), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	opts := ScanOpts{Ignored: []string{"*_test*"}}
	entries, err := ScanSkillsFromDir(tmp, opts)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	for _, e := range entries {
		if e.Name == "bar_test" {
			t.Error("bar_test should be excluded via ignored *_test*")
		}
	}
	foundBar := false
	for _, e := range entries {
		if e.Name == "bar" {
			foundBar = true
		}
	}
	if !foundBar {
		t.Error("bar should remain")
	}
	// Test include filtering
	opts2 := ScanOpts{Include: []string{"bar"}}
	entries2, err := ScanSkillsFromDir(tmp, opts2)
	if err != nil {
		t.Fatalf("Scan include: %v", err)
	}
	if len(entries2) != 1 || entries2[0].Name != "bar" {
		t.Errorf("include filter should return only bar, got %+v", entries2)
	}
}

func TestIncludeGlob(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{"keep", "skip"} {
		dir := filepath.Join(tmp, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	opts := ScanOpts{Include: []string{"keep"}}
	entries, _ := ScanSkillsFromDir(tmp, opts)
	if len(entries) != 1 || entries[0].Name != "keep" {
		t.Errorf("include keep only, got %v", entries)
	}
}
