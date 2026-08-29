package doctor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/assets"
)

func TestManagedAssetHash(t *testing.T) {
	// SHA256 of "hello" is known
	got := assets.ManagedAssetHash([]byte("hello"))
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Errorf("ManagedAssetHash(hello) = %q, want %q", got, want)
	}
	if got2 := assets.ManagedAssetHash([]byte("")); got2 == "" {
		t.Error("empty hash should not be empty")
	}
	// Deterministic
	if got != assets.ManagedAssetHash([]byte("hello")) {
		t.Error("hash not deterministic")
	}
}

func TestManagedAssetHashFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	content := "biggz drift test"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	hex1, err := assets.ManagedAssetHashFile(path)
	if err != nil {
		t.Fatal(err)
	}
	hex2 := assets.ManagedAssetHash([]byte(content))
	if hex1 != hex2 {
		t.Errorf("ManagedAssetHashFile = %q, want %q", hex1, hex2)
	}
	if _, err := assets.ManagedAssetHashFile(filepath.Join(dir, "nope.txt")); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestGlobalDrift_PassZero(t *testing.T) {
	dir := t.TempDir()
	installedRoot := filepath.Join(dir, "installed")
	if err := os.MkdirAll(installedRoot, 0755); err != nil {
		t.Fatal(err)
	}
	content := "sdd content v1"
	// Compute hash
	hash := assets.ManagedAssetHash([]byte(content))
	manifest := `{"schemaVersion":1,"assets":{"agents/sdd-foo.md":"` + hash + `"}}`
	manifestPath := filepath.Join(dir, "managed-assets.json")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	// Write installed file matching hash
	installedPath := filepath.Join(installedRoot, "agents", "sdd-foo.md")
	if err := os.MkdirAll(filepath.Dir(installedPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installedPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	check := NewGlobalDriftCheckWithCustom(manifestPath, installedRoot, os.ReadFile, os.Stat, os.ReadDir)
	res := check.Run(context.Background())
	if res.Status != StatusPass {
		t.Errorf("status = %v, want pass (msg=%q)", res.Status, res.Message)
	}
	if res.Severity != SeverityInfo {
		t.Errorf("severity = %s, want INFO", res.Severity)
	}
	if !strings.Contains(res.Message, "OK") {
		t.Errorf("message = %q, want OK", res.Message)
	}
	if d, ok := res.Details.(map[string]int); ok {
		if d["sddGlobalAssetDriftCount"] != 0 {
			t.Errorf("drift count = %d, want 0", d["sddGlobalAssetDriftCount"])
		}
	}
	if check.Remedy() != nil {
		t.Error("drift check should have no --fix (Remedy nil)")
	}
}

func TestGlobalDrift_WarnOne(t *testing.T) {
	dir := t.TempDir()
	installedRoot := filepath.Join(dir, "installed")
	if err := os.MkdirAll(installedRoot, 0755); err != nil {
		t.Fatal(err)
	}
	hash := assets.ManagedAssetHash([]byte("packaged"))
	manifest := `{"schemaVersion":1,"assets":{"agents/sdd-foo.md":"` + hash + `"}}`
	manifestPath := filepath.Join(dir, "managed-assets.json")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	installedPath := filepath.Join(installedRoot, "agents", "sdd-foo.md")
	if err := os.MkdirAll(filepath.Dir(installedPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installedPath, []byte("installed DIFFERENT"), 0644); err != nil {
		t.Fatal(err)
	}
	check := NewGlobalDriftCheckWithCustom(manifestPath, installedRoot, os.ReadFile, os.Stat, os.ReadDir)
	res := check.Run(context.Background())
	if res.Status != StatusWarn {
		t.Errorf("status = %v, want warn (msg=%q)", res.Status, res.Message)
	}
	if res.Severity != SeverityWarning {
		t.Errorf("severity = %s, want WARNING", res.Severity)
	}
	if !strings.Contains(res.Message, "Global SDD asset drift 1") {
		t.Errorf("message = %q, want 'Global SDD asset drift 1'", res.Message)
	}
	if !strings.Contains(res.Message, "warn:") {
		t.Errorf("message = %q, should contain 'warn:'", res.Message)
	}
}

func TestGlobalDrift_MissingFileWarn(t *testing.T) {
	dir := t.TempDir()
	installedRoot := filepath.Join(dir, "installed")
	os.MkdirAll(installedRoot, 0755)
	hash := assets.ManagedAssetHash([]byte("x"))
	manifest := `{"schemaVersion":1,"assets":{"agents/sdd-foo.md":"` + hash + `"}}`
	manifestPath := filepath.Join(dir, "managed-assets.json")
	os.WriteFile(manifestPath, []byte(manifest), 0644)
	// No installed file -> should be warn 1
	check := NewGlobalDriftCheckWithCustom(manifestPath, installedRoot, os.ReadFile, os.Stat, os.ReadDir)
	res := check.Run(context.Background())
	if res.Status != StatusWarn {
		t.Errorf("status = %v, want warn for missing", res.Status)
	}
}

func TestLocalOverride_PassZero(t *testing.T) {
	dir := t.TempDir()
	check := NewLocalOverrideCheckWithCustom(dir, nil, os.ReadDir)
	res := check.Run(context.Background())
	if res.Status != StatusPass {
		t.Errorf("status = %v, want pass (msg=%q)", res.Status, res.Message)
	}
	if check.Remedy() != nil {
		t.Error("local override should have no --fix")
	}
}

func TestLocalOverride_WarnOne(t *testing.T) {
	dir := t.TempDir()
	agentsDir := filepath.Join(dir, ".pi", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "sdd-foo.md"), []byte("override"), 0644); err != nil {
		t.Fatal(err)
	}
	check := NewLocalOverrideCheckWithCustom(dir, nil, os.ReadDir)
	res := check.Run(context.Background())
	if res.Status != StatusWarn {
		t.Errorf("status = %v, want warn (msg=%q)", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "Project-local") {
		t.Errorf("message = %q, should mention Project-local", res.Message)
	}
	if d, ok := res.Details.(map[string]int); ok {
		if d["sddLocalAgentOverrideCount"] != 1 {
			t.Errorf("count = %d, want 1", d["sddLocalAgentOverrideCount"])
		}
	}
}

func TestDrift_RunnerPanicIsolation(t *testing.T) {
	// Panicking drift check must be isolated and other checks unaffected
	panicCheck := &testCheck{id: GlobalDriftCheckID, panic: true}
	okCheck := &testCheck{id: "ok", status: StatusPass, message: "ok"}
	runner := &Runner{Checks: []Check{panicCheck, okCheck}}
	report := runner.RunAll(context.Background())
	// Panicked drift should be Warning (warn) not Critical
	if len(report.Warning) != 1 {
		t.Errorf("Warning count = %d, want 1 (panic drift)", len(report.Warning))
	} else {
		if report.Warning[0].Status != StatusWarn {
			t.Errorf("panic drift status = %v, want warn", report.Warning[0].Status)
		}
		if !strings.Contains(report.Warning[0].Error, "panic") {
			t.Errorf("panic error = %q, should contain panic", report.Warning[0].Error)
		}
	}
	if len(report.Info) != 1 {
		t.Errorf("Info count = %d, want 1 (ok check)", len(report.Info))
	}
	if len(report.Critical) != 0 {
		t.Errorf("Critical count = %d, want 0 (drift panic is warn not critical)", len(report.Critical))
	}
}

func TestDrift_RunnerOtherChecksUnaffected(t *testing.T) {
	dir := t.TempDir()
	// One good drift check (0) + one panicking + one local override
	goodManifest := filepath.Join(dir, "manifest.json")
	os.WriteFile(goodManifest, []byte(`{"schemaVersion":1,"assets":{}}`), 0644)
	good := NewGlobalDriftCheckWithCustom(goodManifest, filepath.Join(dir, "installed"), os.ReadFile, os.Stat, os.ReadDir)
	panicCheck := &testCheck{id: LocalOverrideCheckID, panic: true}
	local := NewLocalOverrideCheckWithCustom(dir, nil, os.ReadDir)
	runner := &Runner{Checks: []Check{good, panicCheck, local}}
	report := runner.RunAll(context.Background())
	if len(report.All()) != 3 {
		t.Errorf("total = %d, want 3", len(report.All()))
	}
	// good and local should still be pass (info)
	if len(report.Info) != 2 {
		t.Errorf("Info = %d, want 2 (good+local)", len(report.Info))
	}
}

func TestDrift_JSON_RO(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "m.json")
	os.WriteFile(manifest, []byte(`{"schemaVersion":1,"assets":{}}`), 0644)
	global := NewGlobalDriftCheckWithCustom(manifest, filepath.Join(dir, "inst"), os.ReadFile, os.Stat, os.ReadDir)
	local := NewLocalOverrideCheckWithCustom(dir, nil, os.ReadDir)
	runner := &Runner{Checks: []Check{global, local}}
	report := runner.RunAll(context.Background())
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "sddGlobalAssetDriftCount") {
		t.Error("json should contain sddGlobalAssetDriftCount")
	}
	// Ensure report is read-only: re-running doesn't change filesystem
	if len(report.Critical) != 0 || len(report.Warning) != 0 {
		t.Errorf("expected 0 drift, got %d critical %d warning", len(report.Critical), len(report.Warning))
	}
	// Verify Remedy nil for RO (no --fix)
	for _, c := range runner.Checks {
		if c.Remedy() != nil {
			t.Errorf("check %s should have no remedy (--fix absent)", c.ID())
		}
	}
}

func TestDrift_NoFixRejected(t *testing.T) {
	// Drift checks must not expose --fix remedy; CLI is RO
	cases := []Check{
		NewGlobalDriftCheckWithCustom(t.TempDir()+"/m.json", t.TempDir(), os.ReadFile, os.Stat, os.ReadDir),
		NewLocalOverrideCheckWithCustom(t.TempDir(), nil, os.ReadDir),
	}
	for _, c := range cases {
		if c.Remedy() != nil {
			t.Errorf("%s Remedy should be nil (no --fix)", c.ID())
		}
	}
}
