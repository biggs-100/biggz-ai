package skillregistry

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherGate(t *testing.T) {
	o := os.Args
	defer func() { os.Args = o }()
	for _, c := range []struct {
		env  map[string]string
		args []string
		want bool
	}{{nil, []string{"biggz"}, false}, {map[string]string{"BIGGZ_NO_SKILL_REGISTRY": "1"}, []string{"biggz"}, true}, {map[string]string{"GENTLE_PI_NO_SKILL_REGISTRY": "1"}, []string{"biggz"}, true}, {nil, []string{"biggz", "--no-skills"}, true}, {nil, []string{"biggz", "-ns"}, true}} {
		for k, v := range c.env {
			t.Setenv(k, v)
		}
		if c.env == nil || c.env["BIGGZ_NO_SKILL_REGISTRY"] == "" {
			t.Setenv("BIGGZ_NO_SKILL_REGISTRY", "")
		}
		if c.env == nil || c.env["GENTLE_PI_NO_SKILL_REGISTRY"] == "" {
			t.Setenv("GENTLE_PI_NO_SKILL_REGISTRY", "")
		}
		os.Args = c.args
		if shouldSkipWatcher() != c.want {
			t.Errorf("want %v", c.want)
		}
	}
	if !isSkillMD("a/SKILL.md") || isSkillMD("a/README.md") {
		t.Error("isSkillMD")
	}
	t.Setenv("BIGGZ_NO_SKILL_REGISTRY", "1")
	os.Args = []string{"biggz", "--no-skills"}
	root := t.TempDir()
	d := filepath.Join(root, "skills", "k")
	os.MkdirAll(d, 0755)
	os.WriteFile(filepath.Join(d, "SKILL.md"), []byte("# K"), 0644)
	if r, err := Refresh(root, true); err != nil || !r.Regenerated {
		t.Fatalf("Refresh %v", err)
	}
	if w, _ := Start(root, t.Context()); w != nil {
		t.Error("gated nil")
		w.Close()
	}
}
func TestUniqueAndConstants(t *testing.T) {
	h := t.TempDir()
	p := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("USERPROFILE", h)
	os.MkdirAll(filepath.Join(h, ".config", "opencode", "skills"), 0755)
	os.MkdirAll(filepath.Join(p, "skills"), 0755)
	if len(uniqueExistingDirs(p)) != 2 {
		t.Error("want2")
	}
	if WatchDebounceMS != 500*time.Millisecond || PollInterval != 30*time.Second {
		t.Error("constants")
	}
}
func TestWatchingPollLifecycle(t *testing.T) {
	h := t.TempDir()
	p := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("USERPROFILE", h)
	t.Setenv("BIGGZ_NO_SKILL_REGISTRY", "")
	t.Setenv("GENTLE_PI_NO_SKILL_REGISTRY", "")
	o := os.Args
	os.Args = []string{"biggz"}
	defer func() { os.Args = o }()
	s := filepath.Join(p, "skills", "a")
	os.MkdirAll(filepath.Join(s, "nested"), 0755)
	os.WriteFile(filepath.Join(s, "SKILL.md"), []byte("# A"), 0644)
	w, _ := Start(p, t.Context())
	if w == nil || !w.IsWatching() || w.IsPolling() {
		t.Fatalf("watch %v", w)
	}
	os.MkdirAll(filepath.Join(s, "newsub"), 0755)
	time.Sleep(200 * time.Millisecond)
	w.Close()
	w.Close()
	h3 := t.TempDir()
	t.Setenv("HOME", h3)
	t.Setenv("USERPROFILE", h3)
	w3, _ := Start(t.TempDir(), t.Context())
	if w3 == nil || !w3.IsPolling() {
		t.Fatalf("poll %v", w3)
	}
	w3.Close()
	ctx, cancel := context.WithCancel(t.Context())
	os.MkdirAll(filepath.Join(p, "skills", "x"), 0755)
	t.Setenv("HOME", h)
	t.Setenv("USERPROFILE", h)
	w4, _ := Start(p, ctx)
	cancel()
	time.Sleep(50 * time.Millisecond)
	w4.Close()
}
func TestFingerprintGate(t *testing.T) {
	r := t.TempDir()
	h2 := t.TempDir()
	t.Setenv("HOME", h2)
	t.Setenv("USERPROFILE", h2)
	d := filepath.Join(r, "skills", "fp")
	os.MkdirAll(d, 0755)
	pp := filepath.Join(d, "SKILL.md")
	os.WriteFile(pp, []byte("# A"), 0644)
	Refresh(r, true)
	fp1 := Fingerprint(r)
	if rr, _ := Refresh(r, false); !rr.Cached {
		t.Error("cached")
	}
	os.WriteFile(pp, []byte("# B diff"), 0644)
	if fp1 == Fingerprint(r) {
		t.Error("fp")
	}
	if rr, _ := Refresh(r, false); rr.Cached || !rr.Regenerated {
		t.Error("regen")
	}
}
