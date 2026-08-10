package opencode

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestLoadVariants_Valid(t *testing.T) {
	path := writeTempFile(t, "model-variants.json", `{
  "anthropic": { "claude-sonnet-4-20250514": ["high", "low", "medium"] }
}`)

	got, err := LoadVariants(path)
	if err != nil {
		t.Fatalf("LoadVariants() error = %v", err)
	}
	want := map[string]map[string][]string{
		"anthropic": {"claude-sonnet-4-20250514": {"high", "low", "medium"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LoadVariants() = %v, want %v", got, want)
	}
}

func TestLoadVariants_Missing(t *testing.T) {
	_, err := LoadVariants(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Error("LoadVariants() missing file: expected error, got nil")
	}
}

func TestLoadVariants_Malformed(t *testing.T) {
	path := writeTempFile(t, "model-variants.json", `{ not json`)
	_, err := LoadVariants(path)
	if err == nil {
		t.Error("LoadVariants() malformed file: expected error, got nil")
	}
}

func TestEnrichWithVariants_Enriches(t *testing.T) {
	variantsPath := writeTempFile(t, "model-variants.json", `{
  "anthropic": { "claude-sonnet-4-20250514": ["high", "low"] }
}`)

	cached := map[string]Provider{
		"anthropic": {
			ID: "anthropic",
			Models: map[string]Model{
				"claude-sonnet-4-20250514": {ID: "claude-sonnet-4-20250514", ToolCall: true},
				"claude-haiku-4-20250514":  {ID: "claude-haiku-4-20250514", ToolCall: true},
			},
		},
	}

	EnrichWithVariants(cached, variantsPath)

	got := cached["anthropic"].Models["claude-sonnet-4-20250514"].Variants
	want := []string{"high", "low"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sonnet variants = %v, want %v", got, want)
	}
	if got := cached["anthropic"].Models["claude-haiku-4-20250514"].Variants; len(got) != 0 {
		t.Errorf("haiku variants = %v, want nil (no entry)", got)
	}
}

func TestEnrichWithVariants_NoOpOnMissingFile(t *testing.T) {
	cached := map[string]Provider{
		"anthropic": {
			ID:     "anthropic",
			Models: map[string]Model{"claude-sonnet-4-20250514": {ID: "claude-sonnet-4-20250514"}},
		},
	}
	before := cached["anthropic"].Models["claude-sonnet-4-20250514"].Variants

	EnrichWithVariants(cached, filepath.Join(t.TempDir(), "nope.json"))
	if got := cached["anthropic"].Models["claude-sonnet-4-20250514"].Variants; !reflect.DeepEqual(got, before) {
		t.Errorf("variants changed to %v after missing-file enrich", got)
	}
}

func TestEnrichWithVariants_NoOpOnMalformedFile(t *testing.T) {
	variantsPath := writeTempFile(t, "model-variants.json", `{{{`)
	cached := map[string]Provider{
		"anthropic": {ID: "anthropic", Models: map[string]Model{"m": {ID: "m"}}},
	}
	EnrichWithVariants(cached, variantsPath)
	if got := cached["anthropic"].Models["m"].Variants; len(got) != 0 {
		t.Errorf("variants = %v, want nil", got)
	}
}

func TestFilterModelsForSDD_FiltersAndSorts(t *testing.T) {
	provider := Provider{
		ID: "test",
		Models: map[string]Model{
			"zeta":     {ID: "zeta", Name: "Zeta Model", ToolCall: true},
			"no-tools": {ID: "no-tools", Name: "A. No Tools", ToolCall: false},
			"alpha":    {ID: "alpha", Name: "Alpha Model", ToolCall: true},
		},
	}

	got := FilterModelsForSDD(provider)
	if len(got) != 2 {
		t.Fatalf("FilterModelsForSDD() returned %d models, want 2", len(got))
	}
	if got[0].ID != "alpha" || got[1].ID != "zeta" {
		t.Errorf("FilterModelsForSDD() order = [%s %s], want [alpha zeta]", got[0].ID, got[1].ID)
	}
}

func TestFilterModelsForSDD_Empty(t *testing.T) {
	got := FilterModelsForSDD(Provider{ID: "x", Models: map[string]Model{}})
	if len(got) != 0 {
		t.Errorf("FilterModelsForSDD() = %v, want empty", got)
	}
}

func TestLoadModels_ParsesRealCacheShape(t *testing.T) {
	// Mirrors the nested cost/limit objects and env arrays OpenCode writes.
	path := writeTempFile(t, "models.json", `{
  "anthropic": {
    "id": "anthropic",
    "name": "Anthropic",
    "env": ["ANTHROPIC_API_KEY"],
    "models": {
      "claude-sonnet-4-20250514": {
        "id": "claude-sonnet-4-20250514",
        "name": "Claude Sonnet 4",
        "family": "claude-sonnet-4",
        "tool_call": true,
        "reasoning": true,
        "cost": {"input": 3, "output": 15, "cache_read": 0.3},
        "limit": {"context": 200000, "output": 64000}
      }
    }
  }
}`)

	got, err := LoadModels(path)
	if err != nil {
		t.Fatalf("LoadModels() error = %v", err)
	}
	p, ok := got["anthropic"]
	if !ok {
		t.Fatalf("LoadModels() missing provider anthropic: %v", got)
	}
	if p.Name != "Anthropic" || p.Env != "ANTHROPIC_API_KEY" {
		t.Errorf("provider = %+v, want name Anthropic, env ANTHROPIC_API_KEY", p)
	}
	m := p.Models["claude-sonnet-4-20250514"]
	if !m.ToolCall || !m.Reasoning {
		t.Errorf("model = %+v, want tool_call+reasoning", m)
	}
	if m.Cost != 3 {
		t.Errorf("model.Cost = %v, want 3 (input price)", m.Cost)
	}
	if m.Limit != 200000 {
		t.Errorf("model.Limit = %v, want 200000 (context)", m.Limit)
	}
}

func TestLoadModels_SkipsMalformedProviders(t *testing.T) {
	path := writeTempFile(t, "models.json", `{
  "good":   { "id": "good", "name": "Good", "models": { "m1": { "id": "m1", "name": "M1", "tool_call": true } } },
  "broken": "not-a-provider-object"
}`)

	got, err := LoadModels(path)
	if err != nil {
		t.Fatalf("LoadModels() error = %v", err)
	}
	if _, ok := got["good"]; !ok {
		t.Errorf("LoadModels() missing provider good: %v", got)
	}
	if _, ok := got["broken"]; ok {
		t.Errorf("LoadModels() kept malformed provider broken")
	}
}

func TestLoadModels_Missing(t *testing.T) {
	_, err := LoadModels(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Error("LoadModels() missing file: expected error, got nil")
	}
}

func TestLoadModelsOrEmpty_MissingReturnsEmpty(t *testing.T) {
	got, err := LoadModelsOrEmpty(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("LoadModelsOrEmpty() error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("LoadModelsOrEmpty() = %v, want empty map", got)
	}
}

func TestModel_EffortLevels(t *testing.T) {
	if got := (Model{ID: "m", Variants: []string{"high"}}).EffortLevels(); !reflect.DeepEqual(got, []string{"high"}) {
		t.Errorf("EffortLevels() with variants = %v", got)
	}
	if got := (Model{ID: "m"}).EffortLevels(); got != nil {
		t.Errorf("EffortLevels() without variants = %v, want nil", got)
	}
}

func TestPhaseLists(t *testing.T) {
	if len(SDDPhases()) != 10 {
		t.Errorf("SDDPhases() len = %d, want 10", len(SDDPhases()))
	}
	if len(JDPhases()) != 3 {
		t.Errorf("JDPhases() len = %d, want 3", len(JDPhases()))
	}
	if len(ReviewPhases()) != 5 {
		t.Errorf("ReviewPhases() len = %d, want 5", len(ReviewPhases()))
	}
	if len(ConfigurableAgentPhases()) != 19 {
		t.Errorf("ConfigurableAgentPhases() len = %d, want 19", len(ConfigurableAgentPhases()))
	}
}
