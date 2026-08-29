package review

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func tmpRepo(t *testing.T, m map[string]string) string {
	t.Helper()
	r := t.TempDir()
	gitInit(t, r)
	for k, v := range m {
		p := filepath.Join(r, filepath.FromSlash(k))
		_ = os.MkdirAll(filepath.Dir(p), 0755)
		_ = os.WriteFile(p, []byte(v), 0644)
	}
	runGitInDir(t, r, "add", ".")
	runGitInDir(t, r, "commit", "-m", "base")
	return r
}
func TestShellGuard(t *testing.T) {
	r := tmpRepo(t, map[string]string{"base.txt": "base\n"})
	_ = os.WriteFile(filepath.Join(r, "a;rm -rf.txt"), []byte("x\n"), 0644)
	runGitInDir(t, r, "add", ".")
	runGitInDir(t, r, "commit", "-m", "dangerous")
	b := runGitInDir(t, r, "rev-parse", "HEAD~1^{tree}")
	c := runGitInDir(t, r, "rev-parse", "HEAD^{tree}")
	m, err := DeriveChangedPathManifest(r, b, c)
	if err != nil {
		t.Fatal(err)
	}
	ok := false
	for _, e := range m {
		if e.Path == "a;rm -rf.txt" {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("literal missing %+v", m)
	}
	if _, err := DeriveChangedPathManifest(r, "bad", "bad"); err == nil {
		t.Fatal("bad should fail")
	}
}
func TestParser(t *testing.T) {
	t.Run("rename", func(t *testing.T) {
		r := tmpRepo(t, map[string]string{"a.txt": "a\n"})
		runGitInDir(t, r, "mv", "a.txt", "b.txt")
		runGitInDir(t, r, "commit", "-m", "rename")
		m, _ := DeriveChangedPathManifest(r, runGitInDir(t, r, "rev-parse", "HEAD~1^{tree}"), runGitInDir(t, r, "rev-parse", "HEAD^{tree}"))
		if len(m) != 1 || m[0].Path != "b.txt" {
			t.Fatalf("rename %+v", m)
		}
	})
	t.Run("modeOnly", func(t *testing.T) {
		r := tmpRepo(t, map[string]string{"mode.txt": "same\n"})
		p := filepath.Join(r, "mode.txt")
		if runtime.GOOS == "windows" {
			runGitInDir(t, r, "update-index", "--chmod=+x", "mode.txt")
		} else {
			_ = os.Chmod(p, 0755)
			runGitInDir(t, r, "add", ".")
		}
		runGitInDir(t, r, "commit", "-m", "mode")
		m, _ := DeriveChangedPathManifest(r, runGitInDir(t, r, "rev-parse", "HEAD~1^{tree}"), runGitInDir(t, r, "rev-parse", "HEAD^{tree}"))
		for _, e := range m {
			if e.Path == "mode.txt" && (!e.ModeOnly || e.TypeChanged) {
				t.Fatalf("modeOnly %+v", e)
			}
		}
	})
	t.Run("typeChanged", func(t *testing.T) {
		r := tmpRepo(t, map[string]string{"type.txt": "hello\n"})
		_ = os.Remove(filepath.Join(r, "type.txt"))
		if err := os.Symlink("other", filepath.Join(r, "type.txt")); err != nil {
			if runtime.GOOS == "windows" {
				t.Skipf("symlink %v", err)
			}
			t.Fatal(err)
		}
		runGitInDir(t, r, "add", ".")
		runGitInDir(t, r, "commit", "-m", "type")
		m, _ := DeriveChangedPathManifest(r, runGitInDir(t, r, "rev-parse", "HEAD~1^{tree}"), runGitInDir(t, r, "rev-parse", "HEAD^{tree}"))
		for _, e := range m {
			if e.Path == "type.txt" && (!e.TypeChanged || e.Status != "T") {
				t.Fatalf("typeChanged %+v", e)
			}
		}
	})
}
func TestDigest(t *testing.T) {
	m := []ChangedPathEntry{{Path: "b.txt", Status: "M", OldMode: "100644", NewMode: "100644"}, {Path: "a.txt", Status: "A", OldMode: "000000", NewMode: "100644"}}
	d1 := DigestChangedPathManifest(m)
	if len(d1) != 71 || d1 != DigestChangedPathManifest([]ChangedPathEntry{m[1], m[0]}) {
		t.Fatalf("canonical %q", d1)
	}
	type ce struct {
		Path string `json:"path"`; Status string `json:"status"`; OldMode string `json:"old_mode"`; NewMode string `json:"new_mode"`; Deleted bool `json:"deleted"`; TypeChanged bool `json:"type_changed"`; ModeOnly bool `json:"mode_only"`
	}
	b, _ := json.Marshal([]ce{{Path: "a.txt", Status: "A", OldMode: "000000", NewMode: "100644"}, {Path: "b.txt", Status: "M", OldMode: "100644", NewMode: "100644"}})
	s := sha256.Sum256(b)
	if d1 != "sha256:"+hex.EncodeToString(s[:]) {
		t.Fatalf("mismatch %q", d1)
	}
}
func TestRO(t *testing.T) {
	if runtime.GOOS == "windows" {
		r := t.TempDir()
		p := filepath.Join(r, "a.txt")
		_ = os.WriteFile(p, []byte("d"), 0644)
		_ = os.WriteFile(filepath.Join(r, ".git"), []byte("g"), 0644)
		if err := MakeReadOnly(r, []ChangedPathEntry{{Path: "a.txt", Status: "A", OldMode: "000000", NewMode: "100644"}}); err != nil {
			t.Fatal(err)
		}
		f, err := os.OpenFile(p, os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
		_ = MakeWritableForCleanup(r)
		return
	}
	r := t.TempDir()
	m := []ChangedPathEntry{{Path: "a.txt", Status: "A", OldMode: "000000", NewMode: "100644"}, {Path: "sub/b.txt", Status: "A", OldMode: "000000", NewMode: "100755"}}
	for _, e := range m {
		p := filepath.Join(r, filepath.FromSlash(e.Path))
		_ = os.MkdirAll(filepath.Dir(p), 0755)
		_ = os.WriteFile(p, []byte("x"), 0644)
	}
	_ = os.WriteFile(filepath.Join(r, ".git"), []byte("g"), 0644)
	if err := MakeReadOnly(r, m); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(filepath.Join(r, "a.txt")); fi.Mode().Perm() != 0444 {
		t.Fatal("a.txt not 0444")
	}
	if fi, _ := os.Stat(filepath.Join(r, "sub/b.txt")); fi.Mode().Perm() != 0555 {
		t.Fatal("b.txt not 0555")
	}
	_ = MakeWritableForCleanup(r)
}
func TestTraversal(t *testing.T) {
	if !IsWithin("/tmp/root", "/tmp/root/sub") || IsWithin("/tmp/root", "/tmp/root") || IsWithin("/tmp/root", "../../etc/passwd") {
		t.Fatal("isWithin")
	}
	if _, err := decodePath([]byte("../../etc/passwd")); err == nil {
		t.Fatal("traversal")
	}
	if !IsSafeCandidatePath("a/b.txt") || IsSafeCandidatePath("../../etc/passwd") {
		t.Fatal("safe")
	}
	rt := t.TempDir()
	_ = os.WriteFile(filepath.Join(rt, ".git"), []byte("x"), 0644)
	if err := ValidateSymlinkTarget(rt, "a/b.txt", "../../etc/passwd"); err == nil {
		t.Fatal("escape")
	}
	if err := ValidateSymlinkTarget(rt, "a/b.txt", "../c.txt"); err != nil {
		t.Fatal(err)
	}
}
