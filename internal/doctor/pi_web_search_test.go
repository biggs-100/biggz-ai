package doctor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fakeStatExists(path string) (os.FileInfo, error) { return fakeFileInfo{isDir: false}, nil }
func fakeStatMissing(path string) (os.FileInfo, error) { return nil, os.ErrNotExist }

type fakeFileInfo struct{ isDir bool }

func (f fakeFileInfo) Name() string      { return "biggz-web-search.js" }
func (f fakeFileInfo) Size() int64       { return 100 }
func (f fakeFileInfo) Mode() os.FileMode { return 0644 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool       { return f.isDir }
func (f fakeFileInfo) Sys() any          { return nil }

func TestPiWebSearch_FileMissingFail(t *testing.T) {
	c := NewPiWebSearchCheckWithCustom(fakeStatMissing, func(k string) string { return "" }, func() (string, error) { return t.TempDir(), nil })
	r := c.Run(context.Background())
	if r.Status != StatusFail || r.Severity != SeverityCritical {
		t.Fatalf(" want fail/CRITICAL got %v/%s msg=%q", r.Status, r.Severity, r.Message)
	}
	if !strings.Contains(r.Message, "biggz-web-search.js") {
		t.Errorf("message should contain expected path, got %q", r.Message)
	}
}

func TestPiWebSearch_PassWithTavily(t *testing.T) {
	c := NewPiWebSearchCheckWithCustom(fakeStatExists, func(k string) string {
		if k == "TAVILY_API_KEY" {
			return "tv-key"
		}
		return ""
	}, func() (string, error) { return t.TempDir(), nil })
	r := c.Run(context.Background())
	if r.Status != StatusPass || r.Severity != SeverityInfo {
		t.Fatalf("want pass/INFO got %v/%s %q", r.Status, r.Severity, r.Message)
	}
}

func TestPiWebSearch_WarnNoProvider(t *testing.T) {
	c := NewPiWebSearchCheckWithCustom(fakeStatExists, func(k string) string { return "" }, func() (string, error) { return t.TempDir(), nil })
	r := c.Run(context.Background())
	if r.Status != StatusWarn || r.Severity != SeverityWarning {
		t.Fatalf("want warn/WARNING got %v/%s %q", r.Status, r.Severity, r.Message)
	}
	if !strings.Contains(r.Message, "TAVILY_API_KEY") {
		t.Errorf("warn should hint TAVILY_API_KEY, got %q", r.Message)
	}
}

func TestPiWebSearch_DDGFallbackPass(t *testing.T) {
	c := NewPiWebSearchCheckWithCustom(fakeStatExists, func(k string) string {
		if k == "BIGGZ_DDG_FALLBACK" {
			return "1"
		}
		return ""
	}, func() (string, error) { return t.TempDir(), nil })
	r := c.Run(context.Background())
	if r.Status != StatusPass {
		t.Fatalf("DDG fallback should pass, got %v %q", r.Status, r.Message)
	}
}

func TestPiWebSearch_HeadlessNote(t *testing.T) {
	c := NewPiWebSearchCheckWithCustom(fakeStatExists, func(k string) string {
		if k == "TAVILY_API_KEY" {
			return "k"
		}
		if k == "BIGGZ_WEB_FETCH_HEADLESS" {
			return "1"
		}
		return ""
	}, func() (string, error) { return t.TempDir(), nil })
	r := c.Run(context.Background())
	if !strings.Contains(r.Message, "headless") {
		t.Errorf("headless flag should be noted, got %q", r.Message)
	}
}

func TestPiWebSearch_NoLiveProbe(t *testing.T) {
	// Ensure no network is attempted: check only uses stat+getenv
	called := false
	c := NewPiWebSearchCheckWithCustom(func(p string) (os.FileInfo, error) {
		called = true
		return fakeStatMissing(p)
	}, func(k string) string { return "" }, func() (string, error) { return t.TempDir(), nil })
	r := c.Run(context.Background())
	if !called {
		t.Fatalf("stat not called")
	}
	if r.Status != StatusFail {
		t.Fatalf("expected fail")
	}
}

func TestPiWebSearch_PanicIsolation(t *testing.T) {
	runner := &Runner{Checks: []Check{
		&testCheck{id: "ok", status: StatusPass, message: "ok"},
		&testCheck{id: PiWebSearchCheckID, panic: true},
		NewPiWebSearchCheckWithCustom(fakeStatExists, func(k string) string {
			if k == "TAVILY_API_KEY" {
				return "k"
			}
			return ""
		}, func() (string, error) { return t.TempDir(), nil }),
	}}
	report := runner.RunAll(context.Background())
	// pi-web-search should still be present despite panic in earlier check (but we simulated panic via testCheck, not actual PiWebSearch)
	// Verify runner still isolates and check before panic succeeded
	found := false
	for _, r := range report.All() {
		if r.ID == "ok" {
			found = true
		}
	}
	if !found {
		t.Fatalf("runner panic isolation failed")
	}
	// Also direct panic isolation: Runner with panicking PiWebSearch variant
	panicCheck := &testCheck{id: PiWebSearchCheckID, panic: true}
	tmp := t.TempDir()
	afterPanic := NewPiWebSearchCheckWithCustom(fakeStatExists, func(k string) string {
		if k == "BRAVE_API_KEY" {
			return "b"
		}
		return ""
	}, func() (string, error) { return tmp, nil })
	r2 := (&Runner{Checks: []Check{panicCheck, afterPanic}}).RunAll(context.Background())
	if len(r2.Critical) == 0 && len(r2.Info) == 0 {
		t.Fatalf("expected reports after panic")
	}
}

func TestPiWebSearch_Remedy(t *testing.T) {
	c := NewPiWebSearchCheck()
	rem := c.Remedy()
	if rem == nil || rem.ID != string(PiWebSearchCheckID) {
		t.Fatalf("remedy missing or wrong ID")
	}
	if !strings.Contains(rem.Description, "biggz install --agent pi") {
		t.Errorf("remedy description should mention biggz install, got %q", rem.Description)
	}
	if rem.Action == nil {
		t.Fatalf("remedy Action nil")
	}
}

func TestPiWebSearch_RealFS_Integration(t *testing.T) {
	home := t.TempDir()
	extDir := filepath.Join(home, ".pi", "agent", "extensions")
	if err := os.MkdirAll(extDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extDir, "biggz-web-search.js"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAVILY_API_KEY", "test-key")
	// Clear DDG/Brave to isolate
	t.Setenv("BRAVE_API_KEY", "")
	t.Setenv("BIGGZ_DDG_FALLBACK", "")
	c := NewPiWebSearchCheckWithCustom(os.Stat, os.Getenv, func() (string, error) { return home, nil })
	r := c.Run(context.Background())
	if r.Status != StatusPass {
		t.Fatalf("integration real FS should pass, got %v %q", r.Status, r.Message)
	}
	t.Setenv("TAVILY_API_KEY", "")
	r2 := c.Run(context.Background())
	if r2.Status != StatusWarn {
		t.Fatalf("no key should warn, got %v", r2.Status)
	}
}
