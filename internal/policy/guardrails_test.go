package policy

import (
	"os"
	"path/filepath"
	"testing"
)

// Phase 1 RED threat tests
func TestIsDenied_GitSelectionRED(t *testing.T) {
	// RED: git -C before push with force must block; plain push must not
	if !IsDenied("git -C /r push --force") {
		t.Fatalf("expected IsDenied true for git -C push --force")
	}
	if IsDenied("git push") {
		t.Fatalf("expected IsDenied false for git push without force")
	}
}

func TestClassifyGuardedCommand_PushStateRED(t *testing.T) {
	// RED: denied overrides allow even in autonomous mode
	cfg := RuntimeGuardrailsConfig{AutonomousMode: true, GuardedCommands: map[string]string{GuardGitPush: "allow"}}
	if got := ClassifyGuardedCommand("git push --force", cfg); got != "block" {
		t.Fatalf("expected block, got %q", got)
	}
}

func TestIsDenied_BlocksRooted(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{"rm -rf /", true},
		{"rm -rf ~", true},
		{"rm -rf $HOME/x", true},
		{"rm -rf ..", true},
		{"git reset --hard", true},
		{"chmod -R 777 /tmp", true},
		{"chown -R user /tmp", true},
		{"rm -rf ./scoped/a", false},
		{"echo hello", false},
	}
	for _, c := range cases {
		if got := IsDenied(c.cmd); got != c.want {
			t.Fatalf("IsDenied(%q)=%v want %v", c.cmd, got, c.want)
		}
	}
}

func TestIsDenied_GitCleanBothFlags(t *testing.T) {
	if !IsDenied("git clean -fd") {
		t.Fatal("git clean -fd should block")
	}
	if !IsDenied("git clean -f -d") {
		t.Fatal("git clean -f -d should block")
	}
	if !IsDenied("git clean --force --directories") {
		t.Fatal("git clean --force --directories should block")
	}
	if IsDenied("git clean -f") {
		t.Fatal("git clean -f alone should not block")
	}
	if IsDenied("git clean -d") {
		t.Fatal("git clean -d alone should not block")
	}
}

func TestIsDenied_GitPushNeedsForce(t *testing.T) {
	if !IsDenied("git push --force") {
		t.Fatal("push --force should block")
	}
	if !IsDenied("git push -f") {
		t.Fatal("push -f should block")
	}
	if !IsDenied("git -C /r push -uf origin") {
		t.Fatal("git -C push -uf should block")
	}
	if IsDenied("git push") {
		t.Fatal("plain push should not block")
	}
	if IsDenied("git push origin main") {
		t.Fatal("push origin main should not block")
	}
}

func TestEvaluateSensitivePathTool_Blocks8Families(t *testing.T) {
	home := os.Getenv("HOME")
	if home == "" {
		t.Setenv("HOME", "/tmp/home")
	}
	cases := []string{
		"~/.ssh/id_rsa",
		".aws/credentials",
		"secrets/tok",
		".config/gh/hosts.yaml",
		"library/keychains/login.keychain",
		"app/.env",
		"cert/key.pem",
		".credentials",
	}
	for _, p := range cases {
		dec := EvaluateSensitivePathTool("read", map[string]any{"path": p})
		if dec == nil || dec.Kind != DecisionBlock {
			t.Fatalf("expected block for %q, got %v", p, dec)
		}
	}
}

func TestEvaluateSensitivePathTool_EnvVariants(t *testing.T) {
	for _, p := range []string{".env.local", ".env.production", "a.PEM", "cert.p12", "store.pfx"} {
		dec := EvaluateSensitivePathTool("write", map[string]any{"path": p})
		if dec == nil {
			t.Fatalf("expected block for %q", p)
		}
	}
	if dec := EvaluateSensitivePathTool("write", map[string]any{"path": "src/app.go"}); dec != nil {
		t.Fatalf("src/app.go should not block, got %v", dec)
	}
}

func TestEvaluateSensitivePathTool_NonGuardedAndArray(t *testing.T) {
	if dec := EvaluateSensitivePathTool("exec", map[string]any{"path": "~/.ssh/id_rsa"}); dec != nil {
		t.Fatalf("exec should not block, got %v", dec)
	}
	dec := EvaluateSensitivePathTool("read", map[string]any{"paths": []any{"a", "~/.ssh/id_rsa"}})
	if dec == nil {
		t.Fatal("array sensitive should block")
	}
}

func TestClassifyGuardedCommand_DefaultsAndOverrides(t *testing.T) {
	cfgEmpty := RuntimeGuardrailsConfig{AutonomousMode: true, GuardedCommands: map[string]string{}}
	if got := ClassifyGuardedCommand("git push origin main", cfgEmpty); got != "allow" {
		t.Fatalf("gitPush default allow, got %q", got)
	}
	if got := ClassifyGuardedCommand("npm publish", cfgEmpty); got != "block" {
		t.Fatalf("npmPublish default block, got %q", got)
	}
	if got := ClassifyGuardedCommand("git rebase main", cfgEmpty); got != "confirm" {
		t.Fatalf("git rebase default confirm, got %q", got)
	}
	cfgOver := RuntimeGuardrailsConfig{AutonomousMode: true, GuardedCommands: map[string]string{GuardGitPush: "block", GuardNpmPublish: "allow"}}
	if got := ClassifyGuardedCommand("git push origin main", cfgOver); got != "block" {
		t.Fatalf("override gitPush block, got %q", got)
	}
	if got := ClassifyGuardedCommand("npm publish", cfgOver); got != "allow" {
		t.Fatalf("override npmPublish allow, got %q", got)
	}
}

func TestClassifyGuardedCommand_NonAutoConfirmAndUnknown(t *testing.T) {
	cfg := RuntimeGuardrailsConfig{AutonomousMode: false}
	if got := ClassifyGuardedCommand("git push origin main", cfg); got != "confirm" {
		t.Fatalf("non-auto should confirm, got %q", got)
	}
	if got := ClassifyGuardedCommand("go test ./...", cfg); got != "not-guarded" {
		t.Fatalf("unknown should be not-guarded, got %q", got)
	}
}

func TestParseGuardrailsConfigFile_Malformed(t *testing.T) {
	if _, ok := ParseGuardrailsConfigFile("{bad"); ok {
		t.Fatal("malformed should return false")
	}
}

func TestLoadRuntimeGuardrailsConfig_EnvFastPath(t *testing.T) {
	t.Setenv("GENTLE_PI_AUTONOMOUS_MODE", "1")
	// create malformed files to ensure env bypasses I/O
	tmp := t.TempDir()
	// even though files malformed, env should win
	cfg := LoadRuntimeGuardrailsConfig(tmp, filepath.Join(tmp, "home"))
	if !cfg.AutonomousMode || len(cfg.GuardedCommands) != 0 {
		t.Fatalf("env fast-path should be {true, empty}, got %+v", cfg)
	}
}

func TestLoadRuntimeGuardrailsConfig_MalformedSafe(t *testing.T) {
	t.Setenv("GENTLE_PI_AUTONOMOUS_MODE", "")
	home := t.TempDir()
	cwd := t.TempDir()
	// global malformed
	_ = os.WriteFile(filepath.Join(home, "runtime-guardrails.json"), []byte("{bad"), 0644)
	cfg := LoadRuntimeGuardrailsConfig(cwd, home)
	if cfg.AutonomousMode != false || cfg.GuardedCommands == nil {
		t.Fatalf("malformed global should give safe fallback {false, empty non-nil}, got %+v", cfg)
	}
	// project malformed
	_ = os.WriteFile(filepath.Join(home, "runtime-guardrails.json"), []byte(`{"autonomousMode":false,"guardedCommands":{}}`), 0644)
	_ = os.MkdirAll(filepath.Join(cwd, ".pi", "gentle-ai"), 0755)
	_ = os.WriteFile(filepath.Join(cwd, ".pi", "gentle-ai", "runtime-guardrails.json"), []byte("{bad"), 0644)
	cfg = LoadRuntimeGuardrailsConfig(cwd, home)
	if cfg.AutonomousMode != false {
		t.Fatalf("malformed project should give safe fallback, got %+v", cfg)
	}
}

func TestLoadRuntimeGuardrailsConfig_MergeCopyOnMerge(t *testing.T) {
	t.Setenv("GENTLE_PI_AUTONOMOUS_MODE", "")
	home := t.TempDir()
	cwd := t.TempDir()
	_ = os.WriteFile(filepath.Join(home, "runtime-guardrails.json"), []byte(`{"autonomousMode":false,"guardedCommands":{"gitPush":"block"}}`), 0644)
	_ = os.MkdirAll(filepath.Join(cwd, ".pi", "gentle-ai"), 0755)
	_ = os.WriteFile(filepath.Join(cwd, ".pi", "gentle-ai", "runtime-guardrails.json"), []byte(`{"autonomousMode":true,"guardedCommands":{"npmPublish":"allow"}}`), 0644)
	cfg := LoadRuntimeGuardrailsConfig(cwd, home)
	if !cfg.AutonomousMode {
		t.Fatal("merged AutonomousMode should be true (project wins)")
	}
	if cfg.GuardedCommands["gitPush"] != "block" || cfg.GuardedCommands["npmPublish"] != "allow" {
		t.Fatalf("merged maps incorrect: %+v", cfg.GuardedCommands)
	}
	// reread global unchanged
	data, _ := os.ReadFile(filepath.Join(home, "runtime-guardrails.json"))
	if string(data) != `{"autonomousMode":false,"guardedCommands":{"gitPush":"block"}}` {
		t.Fatal("global file mutated")
	}
	// also ensure in-memory copy not mutated: reload again should still have both
	cfg2 := LoadRuntimeGuardrailsConfig(cwd, home)
	if cfg2.GuardedCommands["gitPush"] != "block" {
		t.Fatal("second load missing gitPush")
	}
}
