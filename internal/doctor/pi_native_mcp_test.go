package doctor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helpers for PiNativeMCP tests — distinct names to avoid colliding with the
// existing pi_web_search_test / pi_subagents_test helpers in the same package.

// piNativeLookPath resolves the two binaries the check runs: pi itself and the
// biggz-mcp fallback name. Anything else is reported missing.
func piNativeLookPath(name string) (string, error) {
	switch name {
	case "pi":
		return filepath.Join("C:", "pi", "pi.cmd"), nil
	case "biggz-mcp", "biggz-mcp.exe":
		return filepath.Join("C:", "biggz", "biggz-mcp"), nil
	default:
		return "", errors.New("not found")
	}
}

// piNativeFS is an in-memory filesystem seam: paths are cleaned before lookup
// and unknown paths read as os.ErrNotExist, so no test touches the real home.
type piNativeFS struct {
	files map[string][]byte
	dirs  map[string]bool
}

func newPiNativeFS() *piNativeFS {
	return &piNativeFS{files: map[string][]byte{}, dirs: map[string]bool{}}
}

func (f *piNativeFS) addDir(path string) *piNativeFS {
	f.dirs[filepath.Clean(path)] = true
	return f
}

func (f *piNativeFS) addJSON(t *testing.T, path string, obj map[string]any) *piNativeFS {
	t.Helper()
	raw, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	f.files[filepath.Clean(path)] = raw
	return f
}

func (f *piNativeFS) addRaw(path string, raw string) *piNativeFS {
	f.files[filepath.Clean(path)] = []byte(raw)
	return f
}

func (f *piNativeFS) stat(path string) (os.FileInfo, error) {
	clean := filepath.Clean(path)
	if f.dirs[clean] {
		return fakeFileInfo{isDir: true}, nil
	}
	if _, ok := f.files[clean]; ok {
		return fakeFileInfo{isDir: false}, nil
	}
	return nil, os.ErrNotExist
}

func (f *piNativeFS) readFile(path string) ([]byte, error) {
	raw, ok := f.files[filepath.Clean(path)]
	if !ok {
		return nil, os.ErrNotExist
	}
	return raw, nil
}

// piNativeExec answers the two probes the check runs: the biggz-mcp stdio
// liveness probe and the `pi mcp list` server enumeration.
func piNativeExec(mcpErr error, listOut string, listErr error) func(string, ...string) ([]byte, error) {
	return func(name string, args ...string) ([]byte, error) {
		if name == "pi" && len(args) > 0 && args[0] == "mcp" {
			return []byte(listOut), listErr
		}
		if mcpErr != nil {
			return nil, mcpErr
		}
		return []byte("biggz-mcp --help"), nil
	}
}

// piNativeBigMemEntry is the native mcp.json shape this repo writes: stdio
// inferred from command, codemode exposure, per-tool direct exposure.
func piNativeBigMemEntry(command string) map[string]any {
	return map[string]any{
		"command":      command,
		"args":         []string{"--tools=agent", "--prefix=biggz"},
		"description":  "BigMem",
		"exposure":     "codemode",
		"toolExposure": map[string]any{"biggz_mem_save": "direct"},
	}
}

func piNativeAgentDir(home string) string { return filepath.Join(home, ".pi", "agent") }

func piNativeMCPPath(home string) string { return filepath.Join(piNativeAgentDir(home), "mcp.json") }

func piNativeSettingsPath(home string) string {
	return filepath.Join(piNativeAgentDir(home), "settings.json")
}

func TestPiNativeMCP_HealthyPasses(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	t.Setenv("BIGGZ_MCP_TIMEOUT", "30000")
	home := t.TempDir()
	command := filepath.Join(home, ".biggz", "biggz-mcp")
	fs := newPiNativeFS()
	fs.addRaw(command, "dummy")
	fs.addJSON(t, piNativeMCPPath(home), map[string]any{
		"mcpServers": map[string]any{"bigmem": piNativeBigMemEntry(command)},
	})

	c := NewPiNativeMCPCheckWithCustom(
		piNativeLookPath, fs.stat, fs.readFile,
		piNativeExec(nil, "bigmem: connected\n", nil),
		os.Getenv, func() (string, error) { return home, nil },
	)
	res := c.Run(context.Background())
	if res.Status != StatusPass {
		t.Fatalf("healthy native config should pass, got %v Status=%v Message=%q Error=%q", res.ID, res.Status, res.Message, res.Error)
	}
	if res.Severity != SeverityInfo {
		t.Errorf("healthy severity want INFO got %s", res.Severity)
	}
	if !strings.Contains(res.Message, "bigmem") {
		t.Errorf("healthy message should mention bigmem, got %q", res.Message)
	}
}

func TestPiNativeMCP_SkipsWhenPiNotInstalled(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	c := NewPiNativeMCPCheckWithCustom(
		func(string) (string, error) { return "", errors.New("pi not found") },
		os.Stat, os.ReadFile,
		func(string, ...string) ([]byte, error) { return nil, errors.New("should not be called") },
		os.Getenv, func() (string, error) { return home, nil },
	)
	res := c.Run(context.Background())
	if res.Status != StatusPass {
		t.Fatalf("should skip when pi not installed, got %v %q", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "pi not installed") {
		t.Errorf("skip message should mention pi not installed, got %q", res.Message)
	}
}

func TestPiNativeMCP_MissingConfigWarnsWithInstallHint(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	fs := newPiNativeFS() // no mcp.json at all
	c := NewPiNativeMCPCheckWithCustom(
		piNativeLookPath, fs.stat, fs.readFile,
		piNativeExec(nil, "bigmem: connected\n", nil),
		os.Getenv, func() (string, error) { return home, nil },
	)
	res := c.Run(context.Background())
	if res.Status != StatusWarn {
		t.Fatalf("missing mcp.json should warn, got %v %q", res.Status, res.Message)
	}
	if res.Severity != SeverityWarning {
		t.Errorf("missing config severity want WARNING got %s", res.Severity)
	}
	if !strings.Contains(res.Message, "biggz install --agent pi") {
		t.Errorf("missing config message should hint biggz install, got %q", res.Message)
	}
}

func TestPiNativeMCP_MalformedConfigWarnsWithInstallHint(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	fs := newPiNativeFS().addRaw(piNativeMCPPath(home), "{ not json")
	c := NewPiNativeMCPCheckWithCustom(
		piNativeLookPath, fs.stat, fs.readFile,
		piNativeExec(nil, "bigmem: connected\n", nil),
		os.Getenv, func() (string, error) { return home, nil },
	)
	res := c.Run(context.Background())
	if res.Status != StatusWarn {
		t.Fatalf("malformed mcp.json should warn, got %v %q", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "biggz install --agent pi") {
		t.Errorf("malformed config message should hint biggz install, got %q", res.Message)
	}
}

func TestPiNativeMCP_AdapterStillInstalledWarnsWithRemoveRemedy(t *testing.T) {
	tests := []struct {
		name         string
		withOverride bool
		setup        func(fs *piNativeFS, home, override string)
	}{
		{
			name: "npm directory under home",
			setup: func(fs *piNativeFS, home, _ string) {
				fs.addDir(filepath.Join(home, ".pi", "agent", "npm", "node_modules", "pi-mcp-adapter"))
			},
		},
		{
			name:         "settings packages entry under PI_CODING_AGENT_DIR",
			withOverride: true,
			setup: func(_ *piNativeFS, _ string, override string) {
				// handled by the override settings file added below
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			override := t.TempDir()
			t.Setenv("PI_CODING_AGENT_DIR", "")
			if tt.withOverride {
				t.Setenv("PI_CODING_AGENT_DIR", override)
			}
			agentDir := piNativeAgentDir(home)
			mcpDir := agentDir
			if tt.withOverride {
				mcpDir = override
			}
			fs := newPiNativeFS()
			tt.setup(fs, home, override)
			if tt.withOverride {
				fs.addJSON(t, filepath.Join(override, "settings.json"), map[string]any{
					"packages": []any{"npm:gentle-pi", "npm:pi-mcp-adapter@^2"},
				})
			}
			// A fully valid native config: only the adapter must be reported.
			command := filepath.Join(home, ".biggz", "biggz-mcp")
			fs.addRaw(command, "dummy")
			fs.addJSON(t, filepath.Join(mcpDir, "mcp.json"), map[string]any{
				"mcpServers": map[string]any{"bigmem": piNativeBigMemEntry(command)},
			})

			c := NewPiNativeMCPCheckWithCustom(
				piNativeLookPath, fs.stat, fs.readFile,
				piNativeExec(nil, "bigmem: connected\n", nil),
				os.Getenv, func() (string, error) { return home, nil },
			)
			res := c.Run(context.Background())
			if res.Status != StatusWarn {
				t.Fatalf("installed adapter should warn, got %v %q", res.Status, res.Message)
			}
			if res.Severity != SeverityWarning {
				t.Errorf("adapter present severity want WARNING got %s", res.Severity)
			}
			if !strings.Contains(res.Message, "/mcp") || !strings.Contains(res.Message, "ignores mcp.json") {
				t.Errorf("adapter warning should name the consequence (an extension registering /mcp makes Pi ignore mcp.json), got %q", res.Message)
			}
			if !strings.Contains(res.Message, "pi remove npm:pi-mcp-adapter") {
				t.Errorf("adapter warning should carry the removal remedy, got %q", res.Message)
			}
		})
	}
}

func TestPiNativeMCP_TypeRules(t *testing.T) {
	tests := []struct {
		name      string
		typ       any
		present   bool
		wantWarn  bool
		wantInMsg string
	}{
		{name: "absent type is native stdio", present: false},
		{name: "stdio accepted", present: true, typ: "stdio"},
		{name: "streamable-http accepted", present: true, typ: "streamable-http"},
		{name: "http accepted", present: true, typ: "http"},
		{
			name: "legacy local type is a hard Pi config error", present: true, typ: "local",
			wantWarn:  true,
			wantInMsg: `server "bigmem" needs either "command" (stdio) or "url" (streamable HTTP)`,
		},
		{name: "non-string type rejected", present: true, typ: 1, wantWarn: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PI_CODING_AGENT_DIR", "")
			home := t.TempDir()
			command := filepath.Join(home, ".biggz", "biggz-mcp")
			entry := piNativeBigMemEntry(command)
			if tt.present {
				entry["type"] = tt.typ
			}
			fs := newPiNativeFS()
			fs.addRaw(command, "dummy")
			fs.addJSON(t, piNativeMCPPath(home), map[string]any{"mcpServers": map[string]any{"bigmem": entry}})

			c := NewPiNativeMCPCheckWithCustom(
				piNativeLookPath, fs.stat, fs.readFile,
				piNativeExec(nil, "bigmem: connected\n", nil),
				os.Getenv, func() (string, error) { return home, nil },
			)
			res := c.Run(context.Background())
			if tt.wantWarn {
				if res.Status != StatusWarn {
					t.Fatalf("type %v should warn, got %v %q", tt.typ, res.Status, res.Message)
				}
				if tt.wantInMsg != "" && !strings.Contains(res.Message, tt.wantInMsg) {
					t.Errorf("type warning should quote the Pi config error %q, got %q", tt.wantInMsg, res.Message)
				}
				return
			}
			if res.Status != StatusPass {
				t.Fatalf("type %v should pass, got %v %q Error=%q", tt.typ, res.Status, res.Message, res.Error)
			}
		})
	}
}

func TestPiNativeMCP_CommandUnresolvableWarns(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	missing := filepath.Join(home, ".biggz", "biggz-mcp")
	fs := newPiNativeFS() // command file never created
	fs.addJSON(t, piNativeMCPPath(home), map[string]any{
		"mcpServers": map[string]any{"bigmem": piNativeBigMemEntry(missing)},
	})
	c := NewPiNativeMCPCheckWithCustom(
		piNativeLookPath, fs.stat, fs.readFile,
		piNativeExec(nil, "bigmem: connected\n", nil),
		os.Getenv, func() (string, error) { return home, nil },
	)
	res := c.Run(context.Background())
	if res.Status != StatusWarn {
		t.Fatalf("unresolvable command should warn, got %v %q", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "biggz install --agent pi") {
		t.Errorf("unresolvable command message should hint biggz install, got %q", res.Message)
	}
}

func TestPiNativeMCP_ExposureRules(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(entry map[string]any)
		wantMessage string
	}{
		{name: "native exposure accepted", mutate: func(map[string]any) {}},
		{
			name:        "missing exposure",
			mutate:      func(entry map[string]any) { delete(entry, "exposure") },
			wantMessage: "exposure",
		},
		{
			name:        "unknown exposure",
			mutate:      func(entry map[string]any) { entry["exposure"] = "bogus" },
			wantMessage: "exposure",
		},
		{
			name:        "missing toolExposure",
			mutate:      func(entry map[string]any) { delete(entry, "toolExposure") },
			wantMessage: "toolExposure",
		},
		{
			name:        "unknown toolExposure value",
			mutate:      func(entry map[string]any) { entry["toolExposure"] = map[string]any{"biggz_mem_save": "bogus"} },
			wantMessage: "toolExposure",
		},
		{
			name:        "non-object toolExposure",
			mutate:      func(entry map[string]any) { entry["toolExposure"] = []any{"direct"} },
			wantMessage: "toolExposure",
		},
		{
			name:   "empty toolExposure map stays sane",
			mutate: func(entry map[string]any) { entry["toolExposure"] = map[string]any{} },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PI_CODING_AGENT_DIR", "")
			home := t.TempDir()
			command := filepath.Join(home, ".biggz", "biggz-mcp")
			entry := piNativeBigMemEntry(command)
			tt.mutate(entry)
			fs := newPiNativeFS()
			fs.addRaw(command, "dummy")
			fs.addJSON(t, piNativeMCPPath(home), map[string]any{"mcpServers": map[string]any{"bigmem": entry}})

			c := NewPiNativeMCPCheckWithCustom(
				piNativeLookPath, fs.stat, fs.readFile,
				piNativeExec(nil, "bigmem: connected\n", nil),
				os.Getenv, func() (string, error) { return home, nil },
			)
			res := c.Run(context.Background())
			if tt.wantMessage == "" {
				if res.Status != StatusPass {
					t.Fatalf("sane entry should pass, got %v %q Error=%q", res.Status, res.Message, res.Error)
				}
				return
			}
			if res.Status != StatusWarn {
				t.Fatalf("entry should warn, got %v %q", res.Status, res.Message)
			}
			if !strings.Contains(res.Message, tt.wantMessage) {
				t.Errorf("warning should name %q, got %q", tt.wantMessage, res.Message)
			}
			if !strings.Contains(res.Message, "biggz install --agent pi") {
				t.Errorf("warning should hint biggz install, got %q", res.Message)
			}
		})
	}
}

func TestPiNativeMCP_LegacyKeysWarn(t *testing.T) {
	for _, key := range []string{"imports", "directTools"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv("PI_CODING_AGENT_DIR", "")
			home := t.TempDir()
			command := filepath.Join(home, ".biggz", "biggz-mcp")
			cfg := map[string]any{"mcpServers": map[string]any{"bigmem": piNativeBigMemEntry(command)}}
			cfg[key] = []any{"legacy"}
			fs := newPiNativeFS()
			fs.addRaw(command, "dummy")
			fs.addJSON(t, piNativeMCPPath(home), cfg)

			c := NewPiNativeMCPCheckWithCustom(
				piNativeLookPath, fs.stat, fs.readFile,
				piNativeExec(nil, "bigmem: connected\n", nil),
				os.Getenv, func() (string, error) { return home, nil },
			)
			res := c.Run(context.Background())
			if res.Status != StatusWarn {
				t.Fatalf("legacy key %s should warn, got %v %q", key, res.Status, res.Message)
			}
			if !strings.Contains(res.Message, key) {
				t.Errorf("warning should name %s, got %q", key, res.Message)
			}
			if !strings.Contains(res.Message, "biggz install --agent pi") {
				t.Errorf("warning should hint biggz install, got %q", res.Message)
			}
		})
	}
}

func TestPiNativeMCP_SettingsMCPStateWarns(t *testing.T) {
	tests := []struct {
		name        string
		settings    map[string]any
		wantMessage string
	}{
		{
			name:        "bigmem server entry in settings.json",
			settings:    map[string]any{"mcpServers": map[string]any{"bigmem": piNativeBigMemEntry("/tmp/biggz-mcp")}},
			wantMessage: "mcpServers",
		},
		{
			name:        "top-level directTools in settings.json",
			settings:    map[string]any{"directTools": []any{"biggz_mem_save"}},
			wantMessage: "directTools",
		},
		{
			name:     "clean settings.json passes",
			settings: map[string]any{"packages": []any{"npm:gentle-pi"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PI_CODING_AGENT_DIR", "")
			home := t.TempDir()
			command := filepath.Join(home, ".biggz", "biggz-mcp")
			fs := newPiNativeFS()
			fs.addRaw(command, "dummy")
			fs.addJSON(t, piNativeMCPPath(home), map[string]any{
				"mcpServers": map[string]any{"bigmem": piNativeBigMemEntry(command)},
			})
			fs.addJSON(t, piNativeSettingsPath(home), tt.settings)

			c := NewPiNativeMCPCheckWithCustom(
				piNativeLookPath, fs.stat, fs.readFile,
				piNativeExec(nil, "bigmem: connected\n", nil),
				os.Getenv, func() (string, error) { return home, nil },
			)
			res := c.Run(context.Background())
			if tt.wantMessage == "" {
				if res.Status != StatusPass {
					t.Fatalf("clean settings should pass, got %v %q Error=%q", res.Status, res.Message, res.Error)
				}
				return
			}
			if res.Status != StatusWarn {
				t.Fatalf("settings MCP state should warn, got %v %q", res.Status, res.Message)
			}
			if !strings.Contains(res.Message, tt.wantMessage) {
				t.Errorf("warning should name %q, got %q", tt.wantMessage, res.Message)
			}
			if !strings.Contains(res.Message, "biggz install --agent pi") {
				t.Errorf("warning should hint biggz install, got %q", res.Message)
			}
		})
	}
}

func TestPiNativeMCP_CrashWarnsWithTimeout(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	t.Setenv("BIGGZ_MCP_TIMEOUT", "45000")
	home := t.TempDir()
	command := filepath.Join(home, ".biggz", "biggz-mcp")
	fs := newPiNativeFS()
	fs.addRaw(command, "dummy")
	fs.addJSON(t, piNativeMCPPath(home), map[string]any{
		"mcpServers": map[string]any{"bigmem": piNativeBigMemEntry(command)},
	})
	c := NewPiNativeMCPCheckWithCustom(
		piNativeLookPath, fs.stat, fs.readFile,
		piNativeExec(errors.New("stdio crash: connection reset"), "", nil),
		os.Getenv, func() (string, error) { return home, nil },
	)
	res := c.Run(context.Background())
	if res.Status != StatusWarn && res.Status != StatusFail {
		t.Fatalf("crash should warn or fail, got %v Message=%q", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "BIGGZ_MCP_TIMEOUT") {
		t.Errorf("crash message should name BIGGZ_MCP_TIMEOUT, got %q", res.Message)
	}
	if !strings.Contains(res.Message, "45000") {
		t.Errorf("crash message should carry the configured timeout, got %q", res.Message)
	}
	if !strings.Contains(res.Message, "crash") {
		t.Errorf("crash message should mention the crash, got %q", res.Message)
	}
}

func TestPiNativeMCP_PiMCPListUnavailableWarns(t *testing.T) {
	tests := []struct {
		name   string
		out    string
		listFn error
	}{
		{name: "command fails", listFn: errors.New("exit status 1")},
		{name: "output does not report bigmem", out: "no servers configured\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PI_CODING_AGENT_DIR", "")
			t.Setenv("BIGGZ_MCP_TIMEOUT", "45000")
			home := t.TempDir()
			command := filepath.Join(home, ".biggz", "biggz-mcp")
			fs := newPiNativeFS()
			fs.addRaw(command, "dummy")
			fs.addJSON(t, piNativeMCPPath(home), map[string]any{
				"mcpServers": map[string]any{"bigmem": piNativeBigMemEntry(command)},
			})
			c := NewPiNativeMCPCheckWithCustom(
				piNativeLookPath, fs.stat, fs.readFile,
				piNativeExec(nil, tt.out, tt.listFn),
				os.Getenv, func() (string, error) { return home, nil },
			)
			res := c.Run(context.Background())
			if res.Status != StatusWarn {
				t.Fatalf("pi mcp list liveness loss should warn, got %v %q", res.Status, res.Message)
			}
			if !strings.Contains(res.Message, "pi mcp list") {
				t.Errorf("warning should name the pi mcp list signal, got %q", res.Message)
			}
			if !strings.Contains(res.Message, "BIGGZ_MCP_TIMEOUT") {
				t.Errorf("warning should carry the timeout guidance, got %q", res.Message)
			}
		})
	}
}

func TestPiNativeMCP_PanicIsolation(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	panicStat := func(string) (os.FileInfo, error) { panic("test panic in stat") }
	cPanicking := NewPiNativeMCPCheckWithCustom(
		piNativeLookPath, panicStat, os.ReadFile,
		func(string, ...string) ([]byte, error) { return nil, errors.New("fail") },
		os.Getenv, func() (string, error) { return home, nil },
	)

	command := filepath.Join(home, ".biggz", "biggz-mcp")
	fs := newPiNativeFS()
	fs.addRaw(command, "dummy")
	fs.addJSON(t, piNativeMCPPath(home), map[string]any{
		"mcpServers": map[string]any{"bigmem": piNativeBigMemEntry(command)},
	})
	cHealthy := NewPiNativeMCPCheckWithCustom(
		piNativeLookPath, fs.stat, fs.readFile,
		piNativeExec(nil, "bigmem: connected\n", nil),
		os.Getenv, func() (string, error) { return home, nil },
	)

	runner := &Runner{Checks: []Check{cPanicking, cHealthy}}
	report := runner.RunAll(context.Background())
	if len(report.All()) != 2 {
		t.Fatalf("expected 2 results, got %d", len(report.All()))
	}
	foundHealthy := false
	for _, r := range report.All() {
		if r.ID == PiNativeMCPCheckID && r.Status == StatusPass {
			foundHealthy = true
		}
	}
	if !foundHealthy {
		t.Errorf("healthy check should still pass despite the preceding panic; report: %+v", report.All())
	}
}

func TestPiNativeMCP_RealFS_TmpHomeHealthy(t *testing.T) {
	home := t.TempDir()
	override := t.TempDir() // stands in for PI_CODING_AGENT_DIR, never the live one
	t.Setenv("PI_CODING_AGENT_DIR", override)
	t.Setenv("BIGGZ_MCP_TIMEOUT", "")

	command := filepath.Join(home, ".biggz", "biggz-mcp")
	if err := os.MkdirAll(filepath.Dir(command), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(command, []byte("dummy"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(override, 0o755); err != nil {
		t.Fatal(err)
	}
	mcpRaw, err := json.MarshalIndent(map[string]any{
		"mcpServers": map[string]any{"bigmem": piNativeBigMemEntry(command)},
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(override, "mcp.json"), mcpRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(override, "settings.json"), []byte(`{"packages":["npm:gentle-pi"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	c := NewPiNativeMCPCheckWithCustom(
		piNativeLookPath, os.Stat, os.ReadFile,
		piNativeExec(nil, "bigmem: connected\n", nil),
		os.Getenv, func() (string, error) { return home, nil },
	)
	res := c.Run(context.Background())
	if res.Status != StatusPass {
		t.Fatalf("RealFS temp home should pass, got %v %q Error=%q", res.Status, res.Message, res.Error)
	}
}

func TestPiNativeMCP_Remedy(t *testing.T) {
	c := NewPiNativeMCPCheck()
	if c.ID() != PiNativeMCPCheckID {
		t.Fatalf("ID = %q want %q", c.ID(), PiNativeMCPCheckID)
	}
	rem := c.Remedy()
	if rem == nil || rem.ID != string(PiNativeMCPCheckID) {
		t.Fatalf("remedy missing or wrong ID, got %+v", rem)
	}
	if !strings.Contains(rem.Description, "pi remove npm:pi-mcp-adapter") {
		t.Errorf("remedy should remove the adapter, got %q", rem.Description)
	}
	if !strings.Contains(rem.Description, "biggz install --agent pi") {
		t.Errorf("remedy should re-provision native config, got %q", rem.Description)
	}
	if rem.Action == nil {
		t.Fatalf("remedy Action nil")
	}
}
