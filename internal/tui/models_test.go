package tui

import (
	"path/filepath"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/opencode"
)

func TestTUI_ModelRouting_Picker30(t *testing.T) {
	m := NewModelRoutingModel()
	files := m.PickerFiles()
	if len(files) != 30 {
		t.Fatalf("PickerFiles len = %d, want 30", len(files))
	}
	if PickerCount() != 30 {
		t.Errorf("PickerCount = %d, want 30", PickerCount())
	}
	// ensure PickerAgentFiles also 30
	if got := opencode.PickerAgentFiles(); len(got) != 30 {
		t.Errorf("opencode.PickerAgentFiles len = %d, want 30", len(got))
	}
}

func TestTUI_ModelRouting_ThinkingInherit(t *testing.T) {
	m := NewModelRoutingModel()
	m.globalThinking = opencode.ThinkingHigh
	m.config = opencode.AgentModelConfig{
		"sdd-design": {Model: "claude-sonnet-4", Thinking: opencode.ThinkingInherit},
		"sdd-spec":   {Model: "x", Thinking: opencode.ThinkingLow},
	}
	e := m.ResolveEffective("sdd-design")
	if e.Thinking != opencode.ThinkingHigh {
		t.Errorf("inherit effective = %v, want high", e.Thinking)
	}
	e2 := m.ResolveEffective("sdd-spec")
	if e2.Thinking != opencode.ThinkingLow {
		t.Errorf("low effective = %v, want low", e2.Thinking)
	}
}

func TestTUI_ModelRouting_PrecedenceAgentsUserBuiltin(t *testing.T) {
	agents := opencode.AgentModelConfig{"sdd-design": {Model: "agent-model", Thinking: opencode.ThinkingHigh}}
	user := opencode.AgentModelConfig{"sdd-design": {Model: "user-model", Thinking: opencode.ThinkingLow}, "sdd-spec": {Model: "user-only"}}
	builtin := opencode.AgentModelConfig{"sdd-design": {Model: "builtin-model"}, "sdd-spec": {Model: "builtin-spec"}, "sdd-init": {Model: "builtin-init"}}
	// via helper
	e := ResolvePrecedence(agents, user, builtin, "sdd-design", opencode.ThinkingHigh)
	if e.Model != "agent-model" {
		t.Errorf("agents wins: got %q", e.Model)
	}
	e2 := ResolvePrecedence(agents, user, builtin, "sdd-spec", opencode.ThinkingHigh)
	if e2.Model != "user-only" {
		t.Errorf("user wins over builtin: got %q", e2.Model)
	}
	e3 := ResolvePrecedence(agents, user, builtin, "sdd-init", opencode.ThinkingHigh)
	if e3.Model != "builtin-init" {
		t.Errorf("builtin fallback: got %q", e3.Model)
	}
}

func TestTUI_ModelRouting_ThinkingOptions(t *testing.T) {
	levels := ThinkingLevels()
	if len(levels) != 5 {
		t.Fatalf("ThinkingLevels len = %d, want 5 (off/low/medium/high/inherit)", len(levels))
	}
	expect := map[opencode.ThinkingLevel]bool{
		opencode.ThinkingOff:     true,
		opencode.ThinkingLow:     true,
		opencode.ThinkingMedium:  true,
		opencode.ThinkingHigh:    true,
		opencode.ThinkingInherit: true,
	}
	for _, lvl := range levels {
		if !expect[lvl] {
			t.Errorf("unexpected thinking level %q", lvl)
		}
	}
}

func TestTUI_ModelRouting_SaveReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "models.json")
	// simulate TUI save via opencode.WriteModelConfig
	// override path via temp: directly use opencode funcs
	cfg := opencode.AgentModelConfig{"sdd-design": {Model: "claude-sonnet-4", Thinking: opencode.ThinkingHigh}}
	if err := opencode.WriteModelConfig(path, cfg); err != nil {
		t.Fatalf("write: %v", err)
	}
	loaded, err := opencode.ReadModelConfig(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if loaded["sdd-design"].Model != "claude-sonnet-4" || loaded["sdd-design"].Thinking != opencode.ThinkingHigh {
		t.Errorf("reload = %+v, want claude-sonnet-4/high", loaded["sdd-design"])
	}
	// ensure precedence preserved after reload
	builtin := opencode.AgentModelConfig{"sdd-design": {Model: "builtin"}, "sdd-init": {Model: "builtin-init"}}
	merged := opencode.MergeModelConfigs(loaded, builtin)
	if merged["sdd-design"].Model != "claude-sonnet-4" {
		t.Errorf("precedence after reload: got %q", merged["sdd-design"].Model)
	}
	// ensure model persists via SetModel
	m2 := NewModelRoutingModel()
	m2.SetModel("sdd-design", "test-model")
	if m2.config["sdd-design"].Model != "test-model" {
		t.Errorf("SetModel failed: %+v", m2.config["sdd-design"])
	}
}

func TestTUI_ModelRouting_EnvelopeFromTUI(t *testing.T) {
	cfg := opencode.AgentModelConfig{"sdd-design": {Model: "claude-sonnet-4", Thinking: opencode.ThinkingHigh}}
	data, err := opencode.MarshalModelEnvelope(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := opencode.ParseModelEnvelope(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got["sdd-design"].Model != "claude-sonnet-4" {
		t.Errorf("envelope via tui: %+v", got["sdd-design"])
	}
}
