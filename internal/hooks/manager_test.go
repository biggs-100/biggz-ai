package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager("/tmp/test")
	if m == nil {
		t.Fatal("NewManager() returned nil")
	}
	if m.root != "/tmp/test" {
		t.Errorf("root = %q, want %q", m.root, "/tmp/test")
	}
}

func TestLoad_NoFile(t *testing.T) {
	m := NewManager(t.TempDir())
	if err := m.Load(); err != nil {
		t.Fatalf("Load() error with no file: %v", err)
	}
	if m.config == nil {
		t.Fatal("config should not be nil after Load()")
	}
}

func TestLoad_WithFile(t *testing.T) {
	dir := t.TempDir()
	hookDir := filepath.Join(dir, ".biggz")
	os.MkdirAll(hookDir, 0755)
	yamlContent := []byte(`hooks:
  on_review_start:
    - command: "echo hello"
      description: "Test hook"
      timeout: 10
      continue_on_error: true
`)
	os.WriteFile(filepath.Join(hookDir, "hooks.yaml"), yamlContent, 0644)

	m := NewManager(dir)
	if err := m.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	hooks, ok := m.config.Hooks["on_review_start"]
	if !ok {
		t.Fatal("expected on_review_start hooks")
	}
	if len(hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks))
	}
	if hooks[0].Command != "echo hello" {
		t.Errorf("command = %q, want %q", hooks[0].Command, "echo hello")
	}
	if hooks[0].Timeout != 10 {
		t.Errorf("timeout = %d, want 10", hooks[0].Timeout)
	}
	if !hooks[0].ContinueOnError {
		t.Error("expected continue_on_error = true")
	}
}

func TestLoad_WithYmlExtension(t *testing.T) {
	dir := t.TempDir()
	hookDir := filepath.Join(dir, ".biggz")
	os.MkdirAll(hookDir, 0755)
	os.WriteFile(filepath.Join(hookDir, "hooks.yml"), []byte(`hooks:
  on_apply_done:
    - command: "echo done"
`), 0644)

	m := NewManager(dir)
	if err := m.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if _, ok := m.config.Hooks["on_apply_done"]; !ok {
		t.Error("expected on_apply_done hooks from .yml file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	hookDir := filepath.Join(dir, ".biggz")
	os.MkdirAll(hookDir, 0755)
	os.WriteFile(filepath.Join(hookDir, "hooks.yaml"), []byte("invalid: [yaml: broken"), 0644)

	m := NewManager(dir)
	if err := m.Load(); err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestDispatch_NoConfig(t *testing.T) {
	m := NewManager(t.TempDir())
	results := m.Dispatch(EventReviewStart)
	if results == nil {
		t.Fatal("Dispatch() returned nil")
	}
	if results.Event != EventReviewStart {
		t.Errorf("event = %q, want %q", results.Event, EventReviewStart)
	}
	if results.Blocked {
		t.Error("expected not blocked")
	}
}

func TestDispatch_NoHooksForEvent(t *testing.T) {
	m := NewManager(t.TempDir())
	m.Load()
	results := m.Dispatch("nonexistent_event")
	if results.Blocked {
		t.Error("expected not blocked for missing event")
	}
}

func TestDispatch_EmptyCommand(t *testing.T) {
	m := NewManager(t.TempDir())
	m.config = &HookConfig{
		Hooks: map[string][]HookDef{
			EventReviewStart: {{Command: ""}},
		},
	}
	results := m.Dispatch(EventReviewStart)
	if len(results.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results.Results))
	}
	if results.Results[0].Success {
		t.Error("expected failure for empty command")
	}
	if results.Results[0].Error != "empty command" {
		t.Errorf("error = %q, want %q", results.Results[0].Error, "empty command")
	}
}

func TestDispatch_BlockedByFailure(t *testing.T) {
	m := NewManager(t.TempDir())
	m.config = &HookConfig{
		Hooks: map[string][]HookDef{
			EventReviewStart: {{Command: "false", ContinueOnError: false}},
		},
	}
	results := m.Dispatch(EventReviewStart)
	if !results.Blocked {
		t.Error("expected blocked when hook fails with continue_on_error=false")
	}
}

func TestDispatch_ContinuesOnError(t *testing.T) {
	m := NewManager(t.TempDir())
	m.config = &HookConfig{
		Hooks: map[string][]HookDef{
			EventReviewStart: {
				{Command: "false", ContinueOnError: true},
				{Command: "echo ok", ContinueOnError: true},
			},
		},
	}
	results := m.Dispatch(EventReviewStart)
	if results.Blocked {
		t.Error("expected not blocked when continue_on_error=true")
	}
	if len(results.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results.Results))
	}
}

func TestDispatch_MultipleHooks(t *testing.T) {
	m := NewManager(t.TempDir())
	m.config = &HookConfig{
		Hooks: map[string][]HookDef{
			EventApplyDone: {
				{Command: "echo first", ContinueOnError: true},
				{Command: "echo second", ContinueOnError: true},
			},
		},
	}
	results := m.Dispatch(EventApplyDone)
	if len(results.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results.Results))
	}
}

func TestResults_Success(t *testing.T) {
	r := &HookResults{Event: "test", Results: []HookResult{{Success: true}}}
	if !r.Success() {
		t.Error("expected Success() = true")
	}
	r.Blocked = true
	if r.Success() {
		t.Error("expected Success() = false when blocked")
	}
}

func TestDefaultHooksYAML(t *testing.T) {
	yaml := DefaultHooksYAML()
	if !strings.Contains(yaml, "hooks:") {
		t.Error("expected hooks: in default YAML")
	}
	if !strings.Contains(yaml, "on_review_complete") {
		t.Error("expected on_review_complete in default YAML")
	}
	if !strings.Contains(yaml, "on_apply_done") {
		t.Error("expected on_apply_done in default YAML")
	}
	if !strings.Contains(yaml, "on_pr_created") {
		t.Error("expected on_pr_created in default YAML")
	}
}

func TestEventConstants(t *testing.T) {
	events := []string{EventReviewStart, EventReviewComplete, EventApplyDone, EventPRCreated, EventInstallDone}
	for _, e := range events {
		if e == "" {
			t.Error("event constant should not be empty")
		}
	}
}

func TestHookDefDefaults(t *testing.T) {
	h := HookDef{}
	if h.Timeout != 0 {
		t.Errorf("expected timeout 0, got %d", h.Timeout)
	}
	if h.ContinueOnError {
		t.Error("expected continue_on_error false")
	}
}
