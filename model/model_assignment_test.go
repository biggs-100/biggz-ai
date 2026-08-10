package model

import "testing"

func TestSplitModelSpec_Slash(t *testing.T) {
	providerID, modelID, ok := SplitModelSpec("anthropic/claude-sonnet-4-20250514")
	if !ok {
		t.Fatal("SplitModelSpec() slash: ok = false, want true")
	}
	if providerID != "anthropic" || modelID != "claude-sonnet-4-20250514" {
		t.Errorf("SplitModelSpec() = %q, %q; want anthropic, claude-sonnet-4-20250514", providerID, modelID)
	}
}

func TestSplitModelSpec_Colon(t *testing.T) {
	providerID, modelID, ok := SplitModelSpec("anthropic:claude-sonnet-4-20250514")
	if !ok {
		t.Fatal("SplitModelSpec() colon: ok = false, want true")
	}
	if providerID != "anthropic" || modelID != "claude-sonnet-4-20250514" {
		t.Errorf("SplitModelSpec() = %q, %q; want anthropic, claude-sonnet-4-20250514", providerID, modelID)
	}
}

func TestSplitModelSpec_SplitsAtFirstSeparator(t *testing.T) {
	providerID, modelID, ok := SplitModelSpec("openrouter/qwen/qwen3.6-plus:free")
	if !ok {
		t.Fatal("SplitModelSpec() = ok false, want true")
	}
	if providerID != "openrouter" || modelID != "qwen/qwen3.6-plus:free" {
		t.Errorf("SplitModelSpec() = %q, %q; want openrouter, qwen/qwen3.6-plus:free", providerID, modelID)
	}
}

func TestSplitModelSpec_Invalid(t *testing.T) {
	for _, spec := range []string{"", "no-separator", "/leading", "trailing/", ":"} {
		if providerID, modelID, ok := SplitModelSpec(spec); ok {
			t.Errorf("SplitModelSpec(%q) = ok true (%q, %q), want false", spec, providerID, modelID)
		}
	}
}

func TestModelAssignment_FullID_UsesSlash(t *testing.T) {
	a := ModelAssignment{ProviderID: "anthropic", ModelID: "claude-sonnet-4-20250514", Effort: "high"}
	if got := a.FullID(); got != "anthropic/claude-sonnet-4-20250514" {
		t.Errorf("FullID() = %q, want canonical slash form", got)
	}
}
