package git

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestDetectGitDirs_Parity(t *testing.T) {
	c, g := DetectGitDirs()
	var ec, eg string
	if out, _ := exec.Command("git", "rev-parse", "--git-common-dir").Output(); len(out) > 0 {
		ec = strings.TrimSpace(string(out))
	}
	if out, _ := exec.Command("git", "rev-parse", "--git-dir").Output(); len(out) > 0 {
		eg = strings.TrimSpace(string(out))
	}
	if c != ec || g != eg {
		t.Fatalf("DetectGitDirs %q %q want %q %q", c, g, ec, eg)
	}
}
func TestGitWrapper_IsNotExist_NoPanic(t *testing.T) {
	tmp := t.TempDir()
	orig := os.Getenv("PATH")
	t.Setenv("PATH", tmp)
	defer t.Setenv("PATH", orig)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic %v", r)
		}
	}()
	if c, g := DetectGitDirs(); c != "" || g != "" {
		t.Logf("missing git returned %q %q", c, g)
	}
	if _, err := GitStatus(""); err == nil {
		t.Logf("GitStatus missing git expected error")
	} else if !isNotExist(err) && !strings.Contains(err.Error(), "not found") {
		t.Logf("GitStatus err %v", err)
	}
	if _, err := GitDiff("", "HEAD"); err == nil {
		t.Logf("GitDiff missing git expected error")
	}
}
func TestGitStatusAndDiff(t *testing.T) {
	if out, err := GitStatus(""); err != nil && isNotExist(err) {
		t.Skip("git not available")
	} else {
		_ = out
		_ = err
	}
	if out, err := GitDiff("", "--stat"); err != nil && isNotExist(err) {
		t.Skip("git not available")
	} else {
		_ = out
	}
}
func TestIsNotExist(t *testing.T) {
	if !isNotExist(&os.PathError{Op: "fork/exec", Path: "git", Err: os.ErrNotExist}) {
		t.Fatal("isNotExist")
	}
	if isNotExist(nil) {
		t.Fatal("nil should be false")
	}
}
