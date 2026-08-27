package skillregistry

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTraversalDotDot(t *testing.T) {
	tmp := t.TempDir()
	skillRoot := filepath.Join(tmp, "foo")
	if err := os.MkdirAll(filepath.Join(skillRoot, "docs"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillRoot, "docs", "a.md"), []byte("hello"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	roots := map[string]string{"foo": skillRoot}
	_, err := ResolveSkillURI("skill://foo/../../etc/passwd", roots)
	if err == nil {
		t.Fatal("expected error for traversal ../, got nil")
	}
	if !strings.Contains(err.Error(), "traversal") && !strings.Contains(err.Error(), "..") {
		t.Errorf("error should mention traversal, got %q", err.Error())
	}
	// Ensure no file outside root was accessed (check /etc/passwd not read)
	// The error should be before FS access outside root, which we verify by error presence.
}

func TestSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test skipped on windows")
	}
	tmp := t.TempDir()
	skillRoot := filepath.Join(tmp, "foo")
	if err := os.MkdirAll(skillRoot, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Create real target outside root
	outside := filepath.Join(tmp, "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0644); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	linkPath := filepath.Join(skillRoot, "link")
	if err := os.Symlink(outside, linkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	roots := map[string]string{"foo": skillRoot}
	_, err := ResolveSkillURI("skill://foo/link", roots)
	if err == nil {
		t.Fatal("expected error for symlink escape, got nil")
	}
	if !strings.Contains(err.Error(), "escapes") && !strings.Contains(err.Error(), "symlink") {
		t.Logf("symlink error: %v", err)
		// Still should error; allow any error message as long as it errors.
	}
}

func TestAbsoluteRejected(t *testing.T) {
	tmp := t.TempDir()
	skillRoot := filepath.Join(tmp, "foo")
	if err := os.MkdirAll(skillRoot, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	roots := map[string]string{"foo": skillRoot}
	_, err := ResolveSkillURI("skill://foo//etc/passwd", roots)
	if err == nil {
		t.Fatal("expected error for absolute path //etc/passwd, got nil")
	}
	if !strings.Contains(err.Error(), "absolute") {
		t.Errorf("error should mention absolute, got %q", err.Error())
	}
}

func TestValidInside(t *testing.T) {
	tmp := t.TempDir()
	skillRoot := filepath.Join(tmp, "foo")
	if err := os.MkdirAll(filepath.Join(skillRoot, "docs"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := []byte("valid content")
	if err := os.WriteFile(filepath.Join(skillRoot, "docs", "a.md"), content, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	roots := map[string]string{"foo": skillRoot}
	data, err := ResolveSkillURI("skill://foo/docs/a.md", roots)
	if err != nil {
		t.Fatalf("ResolveSkillURI valid: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("data = %q, want %q", string(data), string(content))
	}
}

func TestResolveSkillURI_MissingSkill(t *testing.T) {
	roots := map[string]string{"foo": "/tmp/foo"}
	_, err := ResolveSkillURI("skill://bar/docs/a.md", roots)
	if err == nil {
		t.Fatal("expected error for missing skill")
	}
}
