package hashline

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/filemerge"
)

func mustSHA(t *testing.T, s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func TestComputeHash(t *testing.T) {
	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, strings.Repeat("x", 8)+strings.Repeat("-", i%5)+"\n")
	}
	whole := strings.Join(lines, "")
	seg := strings.Join(lines[9:20], "")
	wholeHash := filemerge.ComputeHash([]byte(whole))
	rangeHash := filemerge.ComputeHash([]byte(seg))
	if wholeHash == rangeHash {
		t.Fatal("range == whole")
	}
	if rangeHash != mustSHA(t, seg) || wholeHash != mustSHA(t, whole) {
		t.Fatal("hash mismatch")
	}
	empty := filemerge.ComputeHash(nil)
	if empty != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" || Hash4(empty) != "E3B0" {
		t.Fatal("empty hash")
	}
}

func TestSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	orig := []byte("original\nline2\n")
	os.WriteFile(path, orig, 0644)
	var s Store
	s.Capture(path, orig)
	os.WriteFile(path, []byte("modified\n"), 0644)
	if err := s.Restore(path); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(orig) {
		t.Fatalf("restore %q", string(got))
	}
	s.Capture("a.txt", []byte("a"))
	s.Capture("b.txt", []byte("b"))
	if s.Size() != 3 {
		t.Fatalf("size want 3 got %d", s.Size())
	}
	s.Clear()
	if s.Size() != 0 {
		t.Fatal("clear")
	}
	if err := s.Restore("/nope"); err == nil {
		t.Fatal("missing should err")
	}
}

func TestSnapshot_Bounded(t *testing.T) {
	var s Store
	for i := 0; i < 5; i++ {
		s.Capture("/tmp/bnd_"+string(rune('0'+i))+".txt", []byte("c"))
	}
	if s.Size() != 5 {
		t.Fatalf("size %d", s.Size())
	}
	s.Capture("/tmp/bnd_0.txt", []byte("new"))
	if s.Size() != 5 {
		t.Fatal("overwrite")
	}
}
