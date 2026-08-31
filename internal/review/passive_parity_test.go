package review

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func TestPassive(t *testing.T) {
	dir := t.TempDir()
	// Plain md should be passive
	plain := writeTempFile(t, dir, "readme.md", "# Hello\n\nThis is plain markdown.\n")
	if !isPassiveContentFile(plain) {
		t.Fatal("plain md should be passive")
	}
	// Shebang should be not passive
	shebang := writeTempFile(t, dir, "with-shebang.md", "#!/usr/bin/env python\nprint('hi')\n")
	if isPassiveContentFile(shebang) {
		t.Fatal("shebang md should be not passive")
	}
	// BOM + whitespace + shebang
	bomShebang := writeTempFile(t, dir, "bom-shebang.md", "\xEF\xBB\xBF   #!/bin/sh\necho hi\n")
	if isPassiveContentFile(bomShebang) {
		t.Fatal("bom shebang should be not passive")
	}
	// NUL
	nul := writeTempFile(t, dir, "nul.md", "hello\x00world\n")
	if isPassiveContentFile(nul) {
		t.Fatal("NUL should be not passive")
	}
	// Invalid UTF8
	invalid := filepath.Join(dir, "invalid.md")
	os.WriteFile(invalid, []byte{0xff, 0xfe, 0xfd}, 0644)
	if isPassiveContentFile(invalid) {
		t.Fatal("invalid utf8 should be not passive")
	}
	// MDX import
	mdx := writeTempFile(t, dir, "comp.mdx", "import x from \"y\"\n\n# Title\n")
	if isPassiveContentFile(mdx) {
		t.Fatal("MDX import mdx should be not passive")
	}
	mdx2 := writeTempFile(t, dir, "comp2.md", "import x from \"y\"\n")
	if isPassiveContentFile(mdx2) {
		t.Fatal("MDX import should be not passive")
	}
	// exec substring
	execFile := writeTempFile(t, dir, "note.md", "This uses subprocess.call to run\n")
	if isPassiveContentFile(execFile) {
		t.Fatal("exec should be not passive")
	}
	// Over budget >8MiB
	large := filepath.Join(dir, "large.md")
	f, _ := os.Create(large)
	// Write 8MiB +1
	data := strings.Repeat("a", 8<<20+1)
	f.Write([]byte(data))
	f.Close()
	if isPassiveContentFile(large) {
		t.Fatal("large >8MiB should be not passive")
	}
	// Unreadable / missing
	if isPassiveContentFile(filepath.Join(dir, "missing.md")) {
		t.Fatal("missing should be not passive")
	}
	// Extension gate: .go with exec should not be considered via isPassiveContentFile, but triviallyInert should not be passive due to source check
	goFile := writeTempFile(t, dir, "tool.go", "package main\n// exec something\n")
	if isPassiveDocumentExtension(goFile) {
		t.Fatal(".go should not be allowlisted")
	}
	if isPassiveContentFile(goFile) {
		t.Fatal(".go is not allowlisted, so isPassiveContentFile should be false (gate)")
	}
	// Pure passive stays low via ClassifyRisk
	// Create a real file docs/readme.md in temp dir and test ClassifyRisk
	readme := writeTempFile(t, dir, "docs/readme.md", "# Title\n\nPlain text.\n")
	// Use isPassive check directly instead of ClassifyRisk which would need file at that path relative
	_ = readme
	// triviallyInert with plain should be true when file exists and lines ok
	// Use absolute paths for this test
	plainAbs := filepath.Join(dir, "plain2.md")
	writeTempFile(t, dir, "plain2.md", "plain markdown\n")
	if !isPassiveContentFile(plainAbs) {
		t.Fatal("plain2 should be passive")
	}
	// Test ClassifyRisk with triviallyInert gated
	// For docs/readme.md plain (allowlisted), triviallyInert should be true, but we need file at that path
	// Create docs/readme.md under dir and chdir?
	// Simplify: test triviallyInert with absolute path that is passive
	if !triviallyInert([]string{plainAbs}, map[string]int{plainAbs: 10}) {
		t.Fatal("triviallyInert plainAbs should be true")
	}
	if triviallyInert([]string{shebang}, map[string]int{shebang: 10}) {
		t.Fatal("triviallyInert shebang should be false")
	}
}

func TestIsPassiveDocumentExtension(t *testing.T) {
	cases := map[string]bool{
		"readme.md": true, "doc.markdown": true, "a.mdown": true, "b.rst": true, "c.adoc": true, "d.txt": true,
		"e.png": true, "f.jpg": true, "g.jpeg": true, "h.gif": true, "i.mdx": true,
		"i.go": false, "j.js": false, "k.json": false,
	}
	for p, want := range cases {
		if got := isPassiveDocumentExtension(p); got != want {
			t.Errorf("isPassiveDocumentExtension(%q)=%v want %v", p, got, want)
		}
	}
}
