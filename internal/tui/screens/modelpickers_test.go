package screens

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/filemerge"
	"github.com/biggs-100/biggz-ai/internal/opencode"
)

// pickerCache is a small models cache with a variant-rich sonnet, a
// variant-less haiku, and a non-tool-call opus.
const pickerCache = `{
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
        "cost": {"input": 3, "output": 15},
        "limit": {"context": 200000, "output": 64000}
      },
      "claude-haiku-4-20250514": {
        "id": "claude-haiku-4-20250514",
        "name": "Claude Haiku 4",
        "family": "claude-haiku-4",
        "tool_call": true,
        "reasoning": false
      },
      "claude-opus-4-20250514": {
        "id": "claude-opus-4-20250514",
        "name": "Claude Opus 4",
        "family": "claude-opus-4",
        "tool_call": false,
        "reasoning": true
      }
    }
  },
  "openai": {
    "id": "openai",
    "name": "OpenAI",
    "models": {
      "gpt-5": { "id": "gpt-5", "name": "GPT-5", "family": "gpt-5", "tool_call": true, "reasoning": true }
    }
  }
}`

const pickerVariants = `{
  "anthropic": { "claude-sonnet-4-20250514": ["high", "low", "medium"] }
}`

// pickerFixture writes cache, variants, and settings files into a temp dir and
// returns the screen plus the settings path.
func pickerFixture(t *testing.T, settingsContent string) (ModelPickerScreen, string) {
	t.Helper()
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "models.json")
	variantsPath := filepath.Join(dir, "model-variants.json")
	settingsPath := filepath.Join(dir, "opencode.json")
	if err := os.WriteFile(cachePath, []byte(pickerCache), 0o644); err != nil {
		t.Fatalf("write cache: %v", err)
	}
	if err := os.WriteFile(variantsPath, []byte(pickerVariants), 0o644); err != nil {
		t.Fatalf("write variants: %v", err)
	}
	if settingsContent != "" {
		if err := os.WriteFile(settingsPath, []byte(settingsContent), 0o644); err != nil {
			t.Fatalf("write settings: %v", err)
		}
	}
	return NewModelPickerScreenWithPaths(cachePath, variantsPath, settingsPath), settingsPath
}

func updatePicker(m ModelPickerScreen, key string) ModelPickerScreen {
	updated, _ := m.Update(keyMsg(key))
	return updated.(ModelPickerScreen)
}

func TestPicker_InitialState_PhaseList(t *testing.T) {
	m, _ := pickerFixture(t, "")
	if m.view != mpPhaseList {
		t.Errorf("initial view = %v, want mpPhaseList", m.view)
	}
	if len(m.assignments) != 0 {
		t.Errorf("assignments = %v, want empty", m.assignments)
	}
	// 20 agents + 3 separators (review-validator added).
	if len(mpAgentRows) != 23 {
		t.Errorf("mpAgentRows len = %d, want 23", len(mpAgentRows))
	}
}

func TestPicker_EmptyCache_Warning(t *testing.T) {
	dir := t.TempDir()
	m := NewModelPickerScreenWithPaths(
		filepath.Join(dir, "missing-models.json"),
		filepath.Join(dir, "missing-variants.json"),
		filepath.Join(dir, "missing-settings.json"),
	)
	if m.warning == "" {
		t.Error("warning = empty, want cache-missing warning")
	}
	if !strings.Contains(m.View(), "Model cache not found") {
		t.Error("View() does not render the cache-missing warning")
	}
	// Picker stays navigable: enter on the orchestrator row goes nowhere
	// (no providers) without panicking.
	updatePicker(m, "enter")
}

func TestPicker_PhaseList_EnterOpensProviderMode(t *testing.T) {
	m, _ := pickerFixture(t, "")
	m = updatePicker(m, "enter")
	if m.view != mpProvider {
		t.Errorf("view = %v, want mpProvider", m.view)
	}
	if m.selectedAgent != opencode.OrchestratorAgent {
		t.Errorf("selectedAgent = %q, want %q", m.selectedAgent, opencode.OrchestratorAgent)
	}
	if len(m.providerEntries) != 2 {
		t.Errorf("providerEntries = %v, want 2 (Anthropic, OpenAI)", m.providerEntries)
	}
}

func TestPicker_ProviderSelect_EnterOpensModelMode(t *testing.T) {
	m, _ := pickerFixture(t, "")
	m = updatePicker(m, "enter") // orchestrator → providers
	m = updatePicker(m, "enter") // Anthropic (sorted first) → models
	if m.view != mpModel {
		t.Errorf("view = %v, want mpModel", m.view)
	}
	if m.selectedProvider != "anthropic" {
		t.Errorf("selectedProvider = %q, want anthropic", m.selectedProvider)
	}
	// Only tool_call models, sorted by name: Haiku 4, Sonnet 4 (opus excluded).
	if len(m.models) != 2 {
		t.Fatalf("models = %v, want 2 tool-call models", m.models)
	}
	if m.models[0].ID != "claude-haiku-4-20250514" || m.models[1].ID != "claude-sonnet-4-20250514" {
		t.Errorf("model order = [%s %s], want haiku, sonnet", m.models[0].ID, m.models[1].ID)
	}
}

func TestPicker_ModelSelect_NoVariants_AppliesAndSaves(t *testing.T) {
	m, settingsPath := pickerFixture(t, "")
	m = updatePicker(m, "enter") // orchestrator → providers
	m = updatePicker(m, "enter") // anthropic → models
	m = updatePicker(m, "enter") // haiku (no variants) → apply + save

	if m.view != mpPhaseList {
		t.Errorf("view = %v, want mpPhaseList after apply", m.view)
	}
	if m.status == "" {
		t.Error("status = empty, want save confirmation")
	}

	agents := readSettingsAgents(t, settingsPath)
	def, ok := agents[opencode.OrchestratorAgent].(map[string]any)
	if !ok {
		t.Fatalf("orchestrator agent missing from saved settings: %v", agents)
	}
	if def["model"] != "anthropic/claude-haiku-4-20250514" {
		t.Errorf("saved model = %v, want anthropic/claude-haiku-4-20250514", def["model"])
	}
	if def["variant"] != "" {
		t.Errorf("saved variant = %v, want empty", def["variant"])
	}
}

func TestPicker_ModelSelect_WithVariants_OpensEffortMode(t *testing.T) {
	m, _ := pickerFixture(t, "")
	m = updatePicker(m, "enter") // orchestrator → providers
	m = updatePicker(m, "enter") // anthropic → models
	m = updatePicker(m, "down")  // cursor 1: sonnet (has variants)
	m = updatePicker(m, "enter")
	if m.view != mpEffort {
		t.Errorf("view = %v, want mpEffort", m.view)
	}
	opts := mpEffortOptions(m.effortLevels)
	want := []string{"default", "high", "low", "medium"}
	if len(opts) != len(want) {
		t.Fatalf("effort options = %v, want %v", opts, want)
	}
	for i := range want {
		if opts[i] != want[i] {
			t.Errorf("effort options[%d] = %q, want %q", i, opts[i], want[i])
		}
	}
}

func TestPicker_EffortSelect_AppliesEffortAndSaves(t *testing.T) {
	m, settingsPath := pickerFixture(t, "")
	m = updatePicker(m, "enter") // orchestrator → providers
	m = updatePicker(m, "enter") // anthropic → models
	m = updatePicker(m, "down")  // sonnet
	m = updatePicker(m, "enter") // → effort
	m = updatePicker(m, "down")  // cursor 1: high
	m = updatePicker(m, "enter") // apply + save

	if m.view != mpPhaseList {
		t.Errorf("view = %v, want mpPhaseList after effort apply", m.view)
	}

	agents := readSettingsAgents(t, settingsPath)
	def := agents[opencode.OrchestratorAgent].(map[string]any)
	if def["model"] != "anthropic/claude-sonnet-4-20250514" {
		t.Errorf("saved model = %v, want anthropic/claude-sonnet-4-20250514", def["model"])
	}
	if def["variant"] != "high" {
		t.Errorf("saved variant = %v, want high", def["variant"])
	}
}

func TestPicker_EffortSelect_DefaultMapsToEmpty(t *testing.T) {
	m, settingsPath := pickerFixture(t, "")
	m = updatePicker(m, "enter") // orchestrator → providers
	m = updatePicker(m, "enter") // anthropic → models
	m = updatePicker(m, "down")  // sonnet
	m = updatePicker(m, "enter") // → effort
	m = updatePicker(m, "enter") // default → apply + save

	agents := readSettingsAgents(t, settingsPath)
	def := agents[opencode.OrchestratorAgent].(map[string]any)
	if def["variant"] != "" {
		t.Errorf("saved variant = %v, want empty for default effort", def["variant"])
	}
}

func TestPicker_ExistingAgentKey_PreservedWhenUnassigned(t *testing.T) {
	settings := `{
  "agent": {
    "sdd-init": { "mode": "subagent", "model": "openai/gpt-5" }
  }
}`
	m, settingsPath := pickerFixture(t, settings)

	// Assign sonnet to the orchestrator only.
	m = updatePicker(m, "enter") // orchestrator → providers
	m = updatePicker(m, "enter") // anthropic
	m = updatePicker(m, "down")  // sonnet
	m = updatePicker(m, "enter") // → effort
	m = updatePicker(m, "enter") // default → save

	agents := readSettingsAgents(t, settingsPath)
	if got := agents["sdd-init"].(map[string]any)["model"]; got != "openai/gpt-5" {
		t.Errorf("sdd-init model = %v, want user value preserved", got)
	}
	if got := agents["sdd-init"].(map[string]any)["mode"]; got != "subagent" {
		t.Errorf("sdd-init mode = %v, want preserved", got)
	}
	if _, ok := agents["biggz-orchestrator"]; !ok {
		t.Error("biggz-orchestrator agent missing after merge")
	}
}

func TestPicker_ExistingAssignment_PrePopulated(t *testing.T) {
	settings := `{
  "agent": {
    "biggz-orchestrator": { "model": "anthropic:claude-sonnet-4-20250514", "variant": "high" }
  }
}`
	m, _ := pickerFixture(t, settings)
	a, ok := m.assignments[opencode.OrchestratorAgent]
	if !ok {
		t.Fatalf("assignments missing orchestrator: %v", m.assignments)
	}
	if a.ModelID != "claude-sonnet-4-20250514" || a.Effort != "high" {
		t.Errorf("orchestrator assignment = %+v, want sonnet with effort high", a)
	}
}

func TestPicker_StaleEffort_Sanitized(t *testing.T) {
	settings := `{
  "agent": {
    "biggz-orchestrator": { "model": "anthropic:claude-sonnet-4-20250514", "variant": "ultra" }
  }
}`
	m, _ := pickerFixture(t, settings)
	a := m.assignments[opencode.OrchestratorAgent]
	if a.Effort != "" {
		t.Errorf("stale effort = %q, want cleared (not among current variants)", a.Effort)
	}
}

func TestPicker_Esc_BacksOutOfModes(t *testing.T) {
	m, _ := pickerFixture(t, "")

	m = updatePicker(m, "enter") // → provider
	m = updatePicker(m, "esc")
	if m.view != mpPhaseList {
		t.Errorf("view after esc from provider = %v, want mpPhaseList", m.view)
	}

	m = updatePicker(m, "enter") // → provider
	m = updatePicker(m, "enter") // → model
	m = updatePicker(m, "esc")
	if m.view != mpProvider {
		t.Errorf("view after esc from model = %v, want mpProvider", m.view)
	}

	m = updatePicker(m, "enter") // → model
	m = updatePicker(m, "down")  // sonnet
	m = updatePicker(m, "enter") // → effort
	m = updatePicker(m, "esc")
	if m.view != mpModel {
		t.Errorf("view after esc from effort = %v, want mpModel", m.view)
	}
}

func TestPicker_Esc_FromPhaseList_NavigatesDashboard(t *testing.T) {
	m, _ := pickerFixture(t, "")
	updated, cmd := m.Update(keyMsg("esc"))
	m2 := updated.(ModelPickerScreen)
	if m2.view != mpPhaseList {
		t.Errorf("view = %v, want mpPhaseList", m2.view)
	}
	if cmd == nil {
		t.Fatal("expected navigation command on esc from phase list")
	}
	msg := cmd()
	nav, ok := msg.(NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.Screen != 0 {
		t.Errorf("navigate screen = %d, want 0 (dashboard)", nav.Screen)
	}
}

func TestPicker_PhaseList_SkipsSeparators(t *testing.T) {
	m, _ := pickerFixture(t, "")
	// From biggz-orchestrator (0), down must skip "--- SDD phases ---" (1)
	// and land on sdd-init (2).
	m = updatePicker(m, "down")
	if m.cursor != 2 || mpAgentRows[m.cursor] != "sdd-init" {
		t.Errorf("cursor = %d (%q), want 2 (sdd-init)", m.cursor, mpAgentRows[m.cursor])
	}
}

func TestPicker_View_RendersRowsWithoutPanic(t *testing.T) {
	m, _ := pickerFixture(t, "")
	view := m.View()
	for _, want := range []string{"biggz-orchestrator", "SDD phases"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() missing %q", want)
		}
	}
	// Scroll to the bottom: the window must reveal the JD and review groups.
	for i := 0; i < len(mpAgentRows); i++ {
		m = updatePicker(m, "down")
	}
	view = m.View()
	for _, want := range []string{"Judgment Day", "Review agents", "review-refuter"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() at bottom missing %q", want)
		}
	}
}

// readSettingsAgents parses the saved opencode.json (JSONC-safe) and returns
// the "agent" map.
func readSettingsAgents(t *testing.T, settingsPath string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read saved settings: %v", err)
	}
	root, err := filemerge.UnmarshalJSONObject(data)
	if err != nil {
		t.Fatalf("parse saved settings: %v\n%s", err, data)
	}
	agents, ok := root["agent"].(map[string]any)
	if !ok {
		t.Fatalf("saved settings have no agent map: %v", root)
	}
	return agents
}
