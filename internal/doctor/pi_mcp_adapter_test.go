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

// helpers for PiMCPAdapter tests — distinct names to avoid colliding with
// existing pi_web_search_test / pi_subagents_test helpers in same package.

func piMCPFoundLookPath(name string) (string, error) {
	if name == "pi" {
		return filepath.Join("C:", "pi", "pi.cmd"), nil
	}
	if name == "biggz-mcp" || name == "biggz-mcp.exe" {
		return filepath.Join("C:", "biggz", "biggz-mcp"), nil
	}
	return "", errors.New("not found")
}

func piMCPStatWithDirs(dirs ...string) func(string) (os.FileInfo, error) {
	set := make(map[string]bool, len(dirs))
	for _, d := range dirs {
		set[filepath.Clean(d)] = true
	}
	return func(path string) (os.FileInfo, error) {
		if set[filepath.Clean(path)] {
			return fakeFileInfo{isDir: true}, nil
		}
		if filepath.Clean(path) == filepath.Clean(dirs[0]) {
			return nil, os.ErrNotExist
		}
		return nil, os.ErrNotExist
	}
}

func piMCPStatFileExists(path string) (os.FileInfo, error) {
	return fakeFileInfo{isDir: false}, nil
}

func TestPiMCPAdapterCheck_HealthyPasses(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	t.Setenv("BIGGZ_MCP_TIMEOUT", "30000")
	home := t.TempDir()
	adapterDir := filepath.Join(home, ".pi", "agent", "npm", "node_modules", "pi-mcp-adapter")
	// create real adapter dir for stat check via os.Stat path, but we inject stat mock so need dir existence via stat mock
	// Also ensure biggz-mcp binary exists for stat check
	biggzMCP := filepath.Join(home, ".biggz", "biggz-mcp")
	if err := os.MkdirAll(filepath.Dir(biggzMCP), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(biggzMCP, []byte("binary"), 0755); err != nil {
		t.Fatal(err)
	}
	// Mock stat: adapter dir is dir, biggz-mcp is file
	statFn := func(path string) (os.FileInfo, error) {
		clean := filepath.Clean(path)
		if clean == filepath.Clean(adapterDir) {
			return fakeFileInfo{isDir: true}, nil
		}
		if clean == filepath.Clean(biggzMCP) {
			return fakeFileInfo{isDir: false}, nil
		}
		if clean == filepath.Clean(filepath.Join(home, ".biggz", "biggz-mcp.exe")) {
			return nil, os.ErrNotExist
		}
		// For other checks like mcp.json paths, let os.Stat handle? but we use injected readFile, so return not exist for others
		return nil, os.ErrNotExist
	}
	readFileFn := func(path string) ([]byte, error) {
		clean := filepath.Clean(path)
		if strings.HasSuffix(clean, "package.json") && strings.Contains(clean, "pi-mcp-adapter") {
			return json.Marshal(map[string]any{"name": "pi-mcp-adapter", "version": "2.32.1"})
		}
		if strings.HasSuffix(clean, "settings.json") || strings.HasSuffix(clean, "mcp.json") {
			obj := map[string]any{
				"mcpServers": map[string]any{
					"bigmem": map[string]any{"command": biggzMCP, "args": []string{"--tools=agent", "--prefix=biggz"}, "type": "local"},
				},
			}
			return json.Marshal(obj)
		}
		return nil, os.ErrNotExist
	}
	execFn := func(name string, args ...string) ([]byte, error) {
		// npm list fallback should not be called for healthy (adapter found via stat) — but handle
		if name == "npm" {
			return []byte("pi-mcp-adapter@2.32.1"), nil
		}
		// biggz-mcp probe: return help containing biggz_mem
		if strings.Contains(name, "biggz-mcp") {
			return []byte("biggz_mem_save biggz_mem_search"), nil
		}
		return nil, errors.New("unknown exec")
	}
	c := NewPiMCPAdapterCheckWithCustom(piMCPFoundLookPath, statFn, readFileFn, execFn, os.Getenv, func() (string, error) { return home, nil })
	res := c.Run(context.Background())
	if res.Status != StatusPass {
		t.Fatalf("Healthy should pass, got %v Status=%v Message=%q Error=%q", res.ID, res.Status, res.Message, res.Error)
	}
	if res.Severity != SeverityInfo {
		t.Errorf("healthy severity want INFO got %s", res.Severity)
	}
	if !strings.Contains(res.Message, "healthy") && !strings.Contains(res.Message, "biggz-mcp") {
		t.Errorf("healthy message should mention healthy/biggz-mcp, got %q", res.Message)
	}
}

func TestPiMCPAdapterCheck_MissingWarnsWithHint(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	// No adapter dir — stat returns not exist
	statFn := func(path string) (os.FileInfo, error) {
		// biggz-mcp binary exists to isolate adapter missing
		if strings.Contains(path, ".biggz") && strings.Contains(path, "biggz-mcp") {
			return fakeFileInfo{isDir: false}, nil
		}
		return nil, os.ErrNotExist
	}
	// MCP JSON exists to trigger "but MCP JSON exists" scenario
	readFileFn := func(path string) ([]byte, error) {
		if strings.HasSuffix(path, "settings.json") || strings.HasSuffix(path, "mcp.json") {
			obj := map[string]any{
				"mcpServers": map[string]any{
					"bigmem": map[string]any{"command": "/tmp/biggz-mcp", "args": []string{"--tools=agent", "--prefix=biggz"}, "type": "local"},
				},
			}
			return json.Marshal(obj)
		}
		return nil, os.ErrNotExist
	}
	execFn := func(name string, args ...string) ([]byte, error) {
		if name == "npm" {
			return nil, errors.New("not found via npm")
		}
		return nil, errors.New("not found")
	}
	c := NewPiMCPAdapterCheckWithCustom(piMCPFoundLookPath, statFn, readFileFn, execFn, os.Getenv, func() (string, error) { return home, nil })
	res := c.Run(context.Background())
	if res.Status != StatusWarn {
		t.Fatalf("Missing should warn, got %v Message=%q", res.Status, res.Message)
	}
	if res.Severity != SeverityWarning {
		t.Errorf("missing severity want WARNING got %s", res.Severity)
	}
	if !strings.Contains(res.Message, "pi-mcp-adapter") {
		t.Errorf("missing message should name adapter, got %q", res.Message)
	}
	if !strings.Contains(res.Message, "pi install npm:pi-mcp-adapter") {
		t.Errorf("missing message should hint pi install, got %q", res.Message)
	}
}

func TestPiMCPAdapterCheck_VersionDriftWarns(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	adapterDir := filepath.Join(home, ".pi", "agent", "npm", "node_modules", "pi-mcp-adapter")
	statFn := func(path string) (os.FileInfo, error) {
		clean := filepath.Clean(path)
		if clean == filepath.Clean(adapterDir) {
			return fakeFileInfo{isDir: true}, nil
		}
		if strings.Contains(clean, "biggz-mcp") {
			return fakeFileInfo{isDir: false}, nil
		}
		return nil, os.ErrNotExist
	}
	readFileFn := func(path string) ([]byte, error) {
		clean := filepath.Clean(path)
		if strings.HasSuffix(clean, "package.json") && strings.Contains(clean, "pi-mcp-adapter") {
			return json.Marshal(map[string]any{"name": "pi-mcp-adapter", "version": "3.0.0"})
		}
		if strings.HasSuffix(clean, "settings.json") || strings.HasSuffix(clean, "mcp.json") {
			obj := map[string]any{
				"mcpServers": map[string]any{
					"bigmem": map[string]any{"command": "/tmp/biggz-mcp", "args": []string{"--tools=agent", "--prefix=biggz"}, "type": "local"},
				},
			}
			return json.Marshal(obj)
		}
		return nil, os.ErrNotExist
	}
	execFn := func(name string, args ...string) ([]byte, error) {
		if strings.Contains(name, "biggz-mcp") {
			return []byte("biggz_mem_save"), nil
		}
		if name == "npm" {
			return nil, errors.New("npm fail")
		}
		return nil, errors.New("unknown")
	}
	c := NewPiMCPAdapterCheckWithCustom(piMCPFoundLookPath, statFn, readFileFn, execFn, os.Getenv, func() (string, error) { return home, nil })
	res := c.Run(context.Background())
	if res.Status != StatusWarn {
		t.Fatalf("Version drift should warn, got %v Message=%q", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "3.0.0") {
		t.Errorf("drift message should contain 3.0.0, got %q", res.Message)
	}
	if !strings.Contains(res.Message, "^2") {
		t.Errorf("drift message should mention ^2, got %q", res.Message)
	}
}

func TestPiMCPAdapterCheck_CrashWarnsWithTimeout(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	t.Setenv("BIGGZ_MCP_TIMEOUT", "45000")
	home := t.TempDir()
	adapterDir := filepath.Join(home, ".pi", "agent", "npm", "node_modules", "pi-mcp-adapter")
	biggzMCP := filepath.Join(home, ".biggz", "biggz-mcp")
	statFn := func(path string) (os.FileInfo, error) {
		clean := filepath.Clean(path)
		if clean == filepath.Clean(adapterDir) {
			return fakeFileInfo{isDir: true}, nil
		}
		if clean == filepath.Clean(biggzMCP) {
			return fakeFileInfo{isDir: false}, nil
		}
		return nil, os.ErrNotExist
	}
	readFileFn := func(path string) ([]byte, error) {
		clean := filepath.Clean(path)
		if strings.HasSuffix(clean, "package.json") && strings.Contains(clean, "pi-mcp-adapter") {
			return json.Marshal(map[string]any{"name": "pi-mcp-adapter", "version": "2.32.1"})
		}
		if strings.HasSuffix(clean, "settings.json") || strings.HasSuffix(clean, "mcp.json") {
			obj := map[string]any{
				"mcpServers": map[string]any{
					"bigmem": map[string]any{"command": biggzMCP, "args": []string{"--tools=agent", "--prefix=biggz"}, "type": "local"},
				},
			}
			return json.Marshal(obj)
		}
		return nil, os.ErrNotExist
	}
	execFn := func(name string, args ...string) ([]byte, error) {
		if strings.Contains(name, "biggz-mcp") {
			return nil, errors.New("stdio crash: connection reset")
		}
		if name == "npm" {
			return nil, errors.New("npm fail")
		}
		return nil, errors.New("unknown")
	}
	c := NewPiMCPAdapterCheckWithCustom(piMCPFoundLookPath, statFn, readFileFn, execFn, os.Getenv, func() (string, error) { return home, nil })
	res := c.Run(context.Background())
	if res.Status != StatusWarn && res.Status != StatusFail {
		t.Fatalf("Crash should warn or fail, got %v Message=%q", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "BIGGZ_MCP_TIMEOUT") {
		t.Errorf("crash message should contain BIGGZ_MCP_TIMEOUT, got %q", res.Message)
	}
	if !strings.Contains(res.Error, "crash") && !strings.Contains(res.Message, "crash") {
		t.Logf("warning: crash message/error should mention crash, got Message=%q Error=%q", res.Message, res.Error)
	}
	// Ensure timeout value from env appears or default
	if !strings.Contains(res.Message, "45000") && !strings.Contains(res.Message, "BIGGZ_MCP_TIMEOUT") {
		t.Errorf("crash message should mention timeout guidance, got %q", res.Message)
	}
}

func TestPiMCPAdapterCheck_PanicIsolation(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "")
	home := t.TempDir()
	// Panicking statFn
	panicStat := func(path string) (os.FileInfo, error) { panic("test panic in stat") }
	cPanicking := NewPiMCPAdapterCheckWithCustom(piMCPFoundLookPath, panicStat, os.ReadFile, func(string, ...string) ([]byte, error) { return nil, errors.New("fail") }, os.Getenv, func() (string, error) { return home, nil })
	healthyStat := func(path string) (os.FileInfo, error) {
		if strings.Contains(path, "pi-mcp-adapter") {
			return fakeFileInfo{isDir: true}, nil
		}
		if strings.Contains(path, "biggz-mcp") {
			return fakeFileInfo{isDir: false}, nil
		}
		return nil, os.ErrNotExist
	}
	healthyRead := func(path string) ([]byte, error) {
		if strings.Contains(path, "package.json") {
			return json.Marshal(map[string]any{"version": "2.32.1"})
		}
		if strings.Contains(path, "settings.json") || strings.Contains(path, "mcp.json") {
			return json.Marshal(map[string]any{"mcpServers": map[string]any{"bigmem": map[string]any{"command": "/tmp/biggz-mcp", "type": "local"}}})
		}
		return nil, os.ErrNotExist
	}
	healthyExec := func(name string, args ...string) ([]byte, error) {
		if strings.Contains(name, "biggz-mcp") {
			return []byte("biggz_mem_save"), nil
		}
		return nil, errors.New("fail")
	}
	cHealthy := NewPiMCPAdapterCheckWithCustom(piMCPFoundLookPath, healthyStat, healthyRead, healthyExec, os.Getenv, func() (string, error) { return home, nil })
	runner := &Runner{Checks: []Check{cPanicking, cHealthy}}
	report := runner.RunAll(context.Background())
	// Panicking check should be captured as Critical or Warning but not abort healthy
	if len(report.All()) != 2 {
		t.Fatalf("expected 2 results, got %d", len(report.All()))
	}
	foundHealthy := false
	for _, r := range report.All() {
		if r.ID == PiMCPAdapterCheckID && r.Status == StatusPass {
			foundHealthy = true
		}
	}
	if !foundHealthy {
		t.Errorf("healthy check should still pass despite preceding panic; report: %+v", report.All())
	}
}

func TestPiMCPAdapterCheck_RealFS_TmpHomeHealthy(t *testing.T) {
	home := t.TempDir()
	adapterDir := filepath.Join(home, ".pi", "agent", "npm", "node_modules", "pi-mcp-adapter")
	if err := os.MkdirAll(adapterDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(adapterDir, "package.json"), []byte(`{"name":"pi-mcp-adapter","version":"2.32.1"}`), 0644); err != nil {
		t.Fatal(err)
	}
	biggzMCP := filepath.Join(home, ".biggz", "biggz-mcp")
	if err := os.MkdirAll(filepath.Dir(biggzMCP), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(biggzMCP, []byte("dummy"), 0755); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(home, ".pi", "agent", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatal(err)
	}
	mcpPath := filepath.Join(home, ".pi", "agent", "mcp.json")
	mcpObj := map[string]any{
		"mcpServers": map[string]any{"bigmem": map[string]any{"command": biggzMCP, "args": []string{"--tools=agent", "--prefix=biggz"}, "type": "local"}},
		"imports":    []string{"opencode"},
	}
	bs, _ := json.Marshal(mcpObj)
	if err := os.WriteFile(mcpPath, bs, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, bs, 0644); err != nil {
		t.Fatal(err)
	}
	c := NewPiMCPAdapterCheckWithCustom(piMCPFoundLookPath, os.Stat, os.ReadFile, func(name string, args ...string) ([]byte, error) {
		if strings.Contains(name, "biggz-mcp") {
			return []byte("biggz_mem_save tools/list"), nil
		}
		if name == "npm" {
			return nil, errors.New("npm not needed")
		}
		return nil, errors.New("unknown")
	}, os.Getenv, func() (string, error) { return home, nil })
	res := c.Run(context.Background())
	if res.Status != StatusPass {
		t.Fatalf("RealFS healthy should pass, got %v %q Err=%q", res.Status, res.Message, res.Error)
	}
}

func TestPiMCPAdapterCheck_Remedy(t *testing.T) {
	c := NewPiMCPAdapterCheck()
	rem := c.Remedy()
	if rem == nil || rem.ID != string(PiMCPAdapterCheckID) {
		t.Fatalf("remedy missing or wrong ID, got %+v", rem)
	}
	if !strings.Contains(rem.Description, "pi install npm:pi-mcp-adapter") {
		t.Errorf("remedy description should mention pi install, got %q", rem.Description)
	}
	if rem.Action == nil {
		t.Fatalf("remedy Action nil")
	}
}

func TestPiMCPAdapterCheck_SkipsWhenPiNotInstalled(t *testing.T) {
	home := t.TempDir()
	c := NewPiMCPAdapterCheckWithCustom(
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
