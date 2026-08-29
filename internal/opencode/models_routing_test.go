package opencode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModelRouting_ReadWriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "models.json")
	cfg := AgentModelConfig{
		"sdd-design": {Model: "claude-sonnet-4", Thinking: ThinkingHigh},
		"sdd-spec":   {Model: "gpt-5", Thinking: ThinkingLow},
	}
	if err := WriteModelConfig(path, cfg); err != nil {
		t.Fatalf("WriteModelConfig: %v", err)
	}
	got, err := ReadModelConfig(path)
	if err != nil {
		t.Fatalf("ReadModelConfig: %v", err)
	}
	if got["sdd-design"].Model != "claude-sonnet-4" || got["sdd-design"].Thinking != ThinkingHigh {
		t.Errorf("sdd-design = %+v, want claude-sonnet-4/high", got["sdd-design"])
	}
	if got["sdd-spec"].Thinking != ThinkingLow {
		t.Errorf("sdd-spec thinking = %v, want low", got["sdd-spec"].Thinking)
	}
}

func TestModelRouting_MergePrecedence(t *testing.T) {
	agents := AgentModelConfig{"sdd-design": {Model: "agent-model", Thinking: ThinkingHigh}}
	user := AgentModelConfig{"sdd-design": {Model: "user-model", Thinking: ThinkingLow}, "sdd-spec": {Model: "user-only", Thinking: ThinkingMedium}}
	builtin := AgentModelConfig{"sdd-design": {Model: "builtin-model"}, "sdd-spec": {Model: "builtin-spec"}, "sdd-init": {Model: "builtin-init"}}
	merged := MergeModelConfigs(agents, user, builtin)
	if merged["sdd-design"].Model != "agent-model" {
		t.Errorf("precedence agents>user>builtin failed for sdd-design: got %q want agent-model", merged["sdd-design"].Model)
	}
	if merged["sdd-spec"].Model != "user-only" {
		t.Errorf("precedence user>builtin failed for sdd-spec: got %q", merged["sdd-spec"].Model)
	}
	if merged["sdd-init"].Model != "builtin-init" {
		t.Errorf("builtin fallback failed: got %q", merged["sdd-init"].Model)
	}
}

func TestModelRouting_EffectiveThinking(t *testing.T) {
	if got := EffectiveThinking(ThinkingInherit, ThinkingHigh); got != ThinkingHigh {
		t.Errorf("inherit->high = %q, want high", got)
	}
	if got := EffectiveThinking(ThinkingOff, ThinkingHigh); got != ThinkingOff {
		t.Errorf("off with global high = %q, want off", got)
	}
	if got := EffectiveThinking(ThinkingLow, ThinkingHigh); got != ThinkingLow {
		t.Errorf("low = %q, want low", got)
	}
	if got := EffectiveThinking(ThinkingInherit, ThinkingLow); got != ThinkingLow {
		t.Errorf("inherit->low = %q", got)
	}
	if got := EffectiveThinking("", ThinkingHigh); got != ThinkingHigh {
		t.Errorf("empty -> high = %q", got)
	}
}

func TestModelRouting_EnvelopeRoundTrip(t *testing.T) {
	cfg := AgentModelConfig{
		"sdd-design": {Model: "claude-sonnet-4", Thinking: ThinkingHigh},
		"sdd-spec":   {Model: "gpt-5", Thinking: ThinkingInherit},
	}
	data, err := MarshalModelEnvelope(cfg)
	if err != nil {
		t.Fatalf("MarshalModelEnvelope: %v", err)
	}
	if !strings.Contains(string(data), ModelExportKind) {
		t.Errorf("envelope missing kind %q: %s", ModelExportKind, data)
	}
	got, err := ParseModelEnvelope(data)
	if err != nil {
		t.Fatalf("ParseModelEnvelope: %v", err)
	}
	if got["sdd-design"].Model != "claude-sonnet-4" || got["sdd-design"].Thinking != ThinkingHigh {
		t.Errorf("envelope roundtrip sdd-design = %+v", got["sdd-design"])
	}
	if got["sdd-spec"].Thinking != ThinkingInherit {
		t.Errorf("envelope sdd-spec thinking = %v, want inherit", got["sdd-spec"].Thinking)
	}
	// invalid kind
	bad := strings.Replace(string(data), ModelExportKind, "bad.kind", 1)
	if _, err := ParseModelEnvelope([]byte(bad)); err == nil {
		t.Error("ParseModelEnvelope bad kind: expected error")
	}
}

func TestModelRouting_Frontmatter(t *testing.T) {
	orig := "---\nname: sdd-design\ndescription: foo\n---\nbody\n"
	entry := &AgentRoutingEntry{Model: "claude-sonnet-4", Thinking: ThinkingHigh}
	got := UpdateFrontmatterRouting(orig, entry)
	if !strings.Contains(got, "model: claude-sonnet-4") {
		t.Errorf("frontmatter missing model: %q", got)
	}
	if !strings.Contains(got, "thinking: high") {
		t.Errorf("frontmatter missing thinking: %q", got)
	}
	if !strings.Contains(got, "description: foo") {
		t.Errorf("frontmatter lost description: %q", got)
	}
	// clearing
	got2 := UpdateFrontmatterRouting(got, nil)
	if strings.Contains(got2, "model:") || strings.Contains(got2, "thinking:") {
		t.Errorf("frontmatter clear failed: %q", got2)
	}
	// non-frontmatter passthrough
	plain := "no frontmatter"
	if got3 := UpdateFrontmatterRouting(plain, entry); got3 != plain {
		t.Errorf("non-frontmatter should passthrough, got %q", got3)
	}
}

func TestModelRouting_SetThinking(t *testing.T) {
	cfg := make(AgentModelConfig)
	SetThinking(cfg, "sdd-design", ThinkingHigh)
	if cfg["sdd-design"].Thinking != ThinkingHigh {
		t.Errorf("SetThinking high = %v", cfg["sdd-design"].Thinking)
	}
	SetThinking(cfg, "sdd-design", ThinkingInherit)
	if cfg["sdd-design"].Thinking != ThinkingInherit {
		t.Errorf("inherit = %v", cfg["sdd-design"].Thinking)
	}
	// effective
	if got := EffectiveThinking(cfg["sdd-design"].Thinking, ThinkingHigh); got != ThinkingHigh {
		t.Errorf("inherit effective = %v want high", got)
	}
}

func TestModelRouting_PickerFiles(t *testing.T) {
	files := PickerAgentFiles()
	if len(files) != 30 {
		t.Fatalf("PickerAgentFiles len = %d, want 30", len(files))
	}
	seen := map[string]bool{}
	for _, f := range files {
		if seen[f] {
			t.Errorf("duplicate picker file %q", f)
		}
		seen[f] = true
	}
}

func TestModelRouting_SaveReloadPrecedence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".biggz", "models.json")
	// simulate save/reload preserves precedence: write user config, read back, merge with builtin
	user := AgentModelConfig{"sdd-design": {Model: "user-model", Thinking: ThinkingHigh}}
	if err := WriteModelConfig(path, user); err != nil {
		t.Fatalf("write: %v", err)
	}
	loaded, _ := ReadModelConfig(path)
	builtin := AgentModelConfig{"sdd-design": {Model: "builtin-model", Thinking: ThinkingLow}, "sdd-init": {Model: "builtin-init"}}
	merged := MergeModelConfigs(loaded, builtin)
	if merged["sdd-design"].Model != "user-model" {
		t.Errorf("save/reload precedence failed: got %q", merged["sdd-design"].Model)
	}
	if merged["sdd-init"].Model != "builtin-init" {
		t.Errorf("builtin fallback after reload failed")
	}
}

func TestModelRouting_NormalizeInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "models.json")
	// write raw invalid model and thinking, ensure they are filtered
	raw := `{"sdd-design":{"model":"bad model with spaces","thinking":"ultra"},"sdd-spec":{"model":"claude-sonnet-4","thinking":"high"}}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write raw: %v", err)
	}
	got, err := ReadModelConfig(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if _, ok := got["sdd-design"]; ok {
		t.Errorf("invalid entry should be filtered, got %+v", got["sdd-design"])
	}
	if got["sdd-spec"].Model != "claude-sonnet-4" {
		t.Errorf("valid entry missing: %+v", got)
	}
}
