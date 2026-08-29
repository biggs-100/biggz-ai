package pi

import (
	"os"
	"path/filepath"
	"testing"
)

func writePolicyFile(t *testing.T, path, policy string) {
	t.Helper()
	content := `{"schema":"gentle-pi.background-subagents/v1","policy":"` + policy + `"}` + "\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseBackgroundSubagentsPolicyFile(t *testing.T) {
	if p, ok := ParseBackgroundSubagentsPolicyFile(`{"schema":"gentle-pi.background-subagents/v1","policy":"on"}`); !ok || p != "on" {
		t.Fatalf("expected on, got %q ok=%v", p, ok)
	}
	if _, ok := ParseBackgroundSubagentsPolicyFile(`{"schema":"wrong","policy":"on"}`); ok {
		t.Fatal("wrong schema should fail")
	}
	if _, ok := ParseBackgroundSubagentsPolicyFile(`{"schema":"gentle-pi.background-subagents/v1","policy":"on","extra":1}`); ok {
		t.Fatal("extra keys should fail")
	}
	if _, ok := ParseBackgroundSubagentsPolicyFile(`not json`); ok {
		t.Fatal("malformed json should fail")
	}
}

func TestResolveBackgroundSubagentsPolicy_ProjectOverrides(t *testing.T) {
	cwd := t.TempDir()
	cfg := t.TempDir()
	writePolicyFile(t, filepath.Join(cwd, ".pi", "gentle-ai", BackgroundSubagentsFile), "on")
	writePolicyFile(t, filepath.Join(cfg, BackgroundSubagentsFile), "off")
	res := ResolveBackgroundSubagentsPolicy(cwd, LoadBackgroundSubagentsOptions{GentleAiConfigHome: cfg, Env: map[string]string{"GENTLE_PI_BACKGROUND_SUBAGENTS": "on"}})
	if res.Policy != "on" || res.Source != BackgroundSourceProject {
		t.Fatalf("expected project on, got %q %q", res.Policy, res.Source)
	}
	if res.Malformed {
		t.Fatal("should not be malformed")
	}
}

func TestResolveBackgroundSubagentsPolicy_MalformedFailsClosed(t *testing.T) {
	cwd := t.TempDir()
	cfg := t.TempDir()
	p := filepath.Join(cwd, ".pi", "gentle-ai", BackgroundSubagentsFile)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{ malformed`), 0o644); err != nil {
		t.Fatal(err)
	}
	writePolicyFile(t, filepath.Join(cfg, BackgroundSubagentsFile), "on")
	res := ResolveBackgroundSubagentsPolicy(cwd, LoadBackgroundSubagentsOptions{GentleAiConfigHome: cfg, Env: map[string]string{"GENTLE_PI_BACKGROUND_SUBAGENTS": "on"}})
	if res.Policy != "off" {
		t.Fatalf("malformed should fail closed to off, got %q", res.Policy)
	}
	if !res.Malformed || res.Source != BackgroundSourceProject {
		t.Fatalf("expected malformed project_file, got malformed=%v source=%q", res.Malformed, res.Source)
	}
}

func TestResolveBackgroundSubagentsPolicy_GlobalOverridesEnv(t *testing.T) {
	cwd := t.TempDir()
	cfg := t.TempDir()
	writePolicyFile(t, filepath.Join(cfg, BackgroundSubagentsFile), "off")
	res := ResolveBackgroundSubagentsPolicy(cwd, LoadBackgroundSubagentsOptions{GentleAiConfigHome: cfg, Env: map[string]string{"GENTLE_PI_BACKGROUND_SUBAGENTS": "on"}})
	if res.Policy != "off" || res.Source != BackgroundSourceGlobal {
		t.Fatalf("expected global off, got %q %q", res.Policy, res.Source)
	}
}

func TestResolveBackgroundSubagentsPolicy_EnvFallbackAndDefault(t *testing.T) {
	cwd := t.TempDir()
	cfg := t.TempDir()
	res := ResolveBackgroundSubagentsPolicy(cwd, LoadBackgroundSubagentsOptions{GentleAiConfigHome: cfg, Env: map[string]string{"GENTLE_PI_BACKGROUND_SUBAGENTS": "on"}})
	if res.Policy != "on" || res.Source != BackgroundSourceEnvironment {
		t.Fatalf("expected env on, got %q %q", res.Policy, res.Source)
	}
	res2 := ResolveBackgroundSubagentsPolicy(cwd, LoadBackgroundSubagentsOptions{GentleAiConfigHome: cfg, Env: map[string]string{}})
	if res2.Policy != "off" || res2.Source != BackgroundSourceDefault {
		t.Fatalf("expected default off, got %q %q", res2.Policy, res2.Source)
	}
}

func TestRenderBackgroundSubagentsReport_Malformed(t *testing.T) {
	r := BackgroundSubagentsResolution{Policy: "off", Source: BackgroundSourceProject, Malformed: true, ProjectFile: "/tmp/.pi/gentle-ai/bg.json", GlobalFile: "/tmp/.pi/gentle-ai/global.json"}
	report := RenderBackgroundSubagentsReport(r, "ready", nil)
	if report.Type != "warning" {
		t.Fatalf("expected warning for malformed, got %q", report.Type)
	}
	if len(report.Message) == 0 {
		t.Fatal("empty message")
	}
}

func TestGentleAiConfigHome_EnvOverride(t *testing.T) {
	t.Setenv("GENTLE_PI_CONFIG_HOME", "/tmp/custom")
	if got := GentleAiConfigHome(); got != "/tmp/custom" {
		t.Fatalf("expected custom, got %q", got)
	}
}
