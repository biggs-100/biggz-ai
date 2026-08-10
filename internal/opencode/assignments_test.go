package opencode

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/filemerge"
	"github.com/biggs-100/biggz-ai/model"
)

func TestReadCurrentModelAssignments_ValidJSONC(t *testing.T) {
	settings := writeTempFile(t, "opencode.json", `{
  // biggz model assignments
  "agent": {
    "biggz-orchestrator": { "model": "anthropic/claude-sonnet-4-20250514", "variant": "high" },
    "sdd-init": { "model": "anthropic:claude-haiku-4-20250514", },
    "sdd-spec": { "model": "google/gemini-2.5-pro" },
  },
}`)

	got, err := ReadCurrentModelAssignments(settings)
	if err != nil {
		t.Fatalf("ReadCurrentModelAssignments() error = %v", err)
	}
	want := map[string]model.ModelAssignment{
		OrchestratorAgent: {ProviderID: "anthropic", ModelID: "claude-sonnet-4-20250514", Effort: "high"},
		"sdd-init":        {ProviderID: "anthropic", ModelID: "claude-haiku-4-20250514"},
		"sdd-spec":        {ProviderID: "google", ModelID: "gemini-2.5-pro"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReadCurrentModelAssignments() = %v, want %v", got, want)
	}
}

func TestReadCurrentModelAssignments_MissingFile(t *testing.T) {
	got, err := ReadCurrentModelAssignments(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("ReadCurrentModelAssignments() missing file error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("ReadCurrentModelAssignments() = %v, want empty map", got)
	}
}

func TestReadCurrentModelAssignments_MalformedFile(t *testing.T) {
	settings := writeTempFile(t, "opencode.json", `{ not json`)
	got, err := ReadCurrentModelAssignments(settings)
	if err != nil {
		t.Fatalf("ReadCurrentModelAssignments() malformed file error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("ReadCurrentModelAssignments() = %v, want empty map", got)
	}
}

func TestReadCurrentModelAssignments_LegacyAlias(t *testing.T) {
	settings := writeTempFile(t, "opencode.json", `{
  "agent": {
    "sdd-orchestrator": { "model": "anthropic:claude-sonnet-4-20250514" }
  }
}`)

	got, err := ReadCurrentModelAssignments(settings)
	if err != nil {
		t.Fatalf("ReadCurrentModelAssignments() error = %v", err)
	}
	a, ok := got[OrchestratorAgent]
	if !ok {
		t.Fatalf("ReadCurrentModelAssignments() missing %s mapping: %v", OrchestratorAgent, got)
	}
	if a.ModelID != "claude-sonnet-4-20250514" {
		t.Errorf("orchestrator assignment = %+v, want claude-sonnet-4-20250514", a)
	}
	if _, ok := got["sdd-orchestrator"]; ok {
		t.Error("legacy sdd-orchestrator key leaked into result")
	}
}

func TestReadCurrentModelAssignments_RealOrchestratorWinsOverAlias(t *testing.T) {
	settings := writeTempFile(t, "opencode.json", `{
  "agent": {
    "biggz-orchestrator": { "model": "anthropic:claude-opus-4-20250514" },
    "sdd-orchestrator":   { "model": "anthropic:claude-haiku-4-20250514" }
  }
}`)

	got, err := ReadCurrentModelAssignments(settings)
	if err != nil {
		t.Fatalf("ReadCurrentModelAssignments() error = %v", err)
	}
	a := got[OrchestratorAgent]
	if a.ModelID != "claude-opus-4-20250514" {
		t.Errorf("orchestrator = %+v, want real key to win over alias", a)
	}
}

func TestReadCurrentModelAssignments_UnknownKeysDropped(t *testing.T) {
	settings := writeTempFile(t, "opencode.json", `{
  "agent": {
    "biggz-orchestrator": { "model": "anthropic:claude-sonnet-4-20250514" },
    "custom-agent":       { "model": "anthropic:claude-haiku-4-20250514" },
    "gentle-orchestrator": { "model": "anthropic:claude-opus-4-20250514" }
  }
}`)

	got, err := ReadCurrentModelAssignments(settings)
	if err != nil {
		t.Fatalf("ReadCurrentModelAssignments() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ReadCurrentModelAssignments() = %v, want only the orchestrator", got)
	}
}

func TestReadCurrentModelAssignments_MalformedModelDropped(t *testing.T) {
	settings := writeTempFile(t, "opencode.json", `{
  "agent": {
    "biggz-orchestrator": { "model": "no-separator" },
    "sdd-init":           { "model": 42 },
    "sdd-spec":           { }
  }
}`)

	got, err := ReadCurrentModelAssignments(settings)
	if err != nil {
		t.Fatalf("ReadCurrentModelAssignments() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ReadCurrentModelAssignments() = %v, want empty map", got)
	}
}

func TestReadCurrentModelAssignments_IncludesJDAndReviewAgents(t *testing.T) {
	settings := writeTempFile(t, "opencode.json", `{
  "agent": {
    "jd-judge-a":       { "model": "anthropic:claude-sonnet-4-20250514" },
    "review-risk":      { "model": "openai:gpt-5" },
    "review-refuter":   { "model": "google:gemini-2.5-flash" }
  }
}`)

	got, err := ReadCurrentModelAssignments(settings)
	if err != nil {
		t.Fatalf("ReadCurrentModelAssignments() error = %v", err)
	}
	if len(got) != 3 {
		t.Errorf("ReadCurrentModelAssignments() = %v, want 3 entries", got)
	}
}

// overlayFixture is a minimal agent overlay shaped like the embedded
// sdd-overlay-multi.json for injection tests.
const overlayFixture = `{
  "agent": {
    "biggz-orchestrator": { "mode": "primary" },
    "sdd-init": { "mode": "subagent" },
    "sdd-spec": { "mode": "subagent" },
    "jd-judge-a": { "mode": "subagent" },
    "jd-judge-b": { "mode": "subagent" }
  }
}`

func injectAndParse(t *testing.T, overlay []byte, assignments map[string]model.ModelAssignment, rootModelID string, existingKeys []string) map[string]any {
	t.Helper()
	out, err := InjectModelAssignments(overlay, assignments, rootModelID, existingKeys)
	if err != nil {
		t.Fatalf("InjectModelAssignments() error = %v", err)
	}
	root, err := filemerge.UnmarshalJSONObject(out)
	if err != nil {
		t.Fatalf("unmarshal injected overlay: %v\n%s", err, out)
	}
	return root["agent"].(map[string]any)
}

func agentModelField(t *testing.T, agents map[string]any, name, field string) string {
	t.Helper()
	def, ok := agents[name].(map[string]any)
	if !ok {
		t.Fatalf("agent %s missing from injected overlay", name)
	}
	v, _ := def[field].(string)
	return v
}

func TestInjectModelAssignments_AssignmentWins(t *testing.T) {
	agents := injectAndParse(t, []byte(overlayFixture),
		map[string]model.ModelAssignment{
			"sdd-init": {ProviderID: "anthropic", ModelID: "claude-sonnet-4-20250514", Effort: "high"},
		},
		"anthropic/claude-opus-4-20250514", nil)

	if got := agentModelField(t, agents, "sdd-init", "model"); got != "anthropic/claude-sonnet-4-20250514" {
		t.Errorf("sdd-init model = %q, want TUI assignment", got)
	}
	if got := agentModelField(t, agents, "sdd-init", "variant"); got != "high" {
		t.Errorf("sdd-init variant = %q, want high", got)
	}
}

func TestInjectModelAssignments_EmptyEffortSetsEmptyVariant(t *testing.T) {
	agents := injectAndParse(t, []byte(overlayFixture),
		map[string]model.ModelAssignment{
			"sdd-init": {ProviderID: "anthropic", ModelID: "claude-sonnet-4-20250514"},
		},
		"", nil)

	if got := agentModelField(t, agents, "sdd-init", "variant"); got != "" {
		t.Errorf("sdd-init variant = %q, want empty string", got)
	}
}

func TestInjectModelAssignments_ExistingAgentKeySkipped(t *testing.T) {
	agents := injectAndParse(t, []byte(overlayFixture),
		nil,
		"anthropic/claude-opus-4-20250514",
		[]string{"sdd-spec", "custom-thing"})

	// sdd-spec exists in user config: untouched (no model written).
	if got := agentModelField(t, agents, "sdd-spec", "model"); got != "" {
		t.Errorf("sdd-spec model = %q, want empty (user config preserved)", got)
	}
	// sdd-init does NOT exist in user config: gets root model.
	if got := agentModelField(t, agents, "sdd-init", "model"); got != "anthropic/claude-opus-4-20250514" {
		t.Errorf("sdd-init model = %q, want root model", got)
	}
}

func TestInjectModelAssignments_RootModelFallbackClearsVariant(t *testing.T) {
	agents := injectAndParse(t, []byte(overlayFixture),
		nil, "anthropic/claude-opus-4-20250514", nil)

	if got := agentModelField(t, agents, "sdd-init", "model"); got != "anthropic/claude-opus-4-20250514" {
		t.Errorf("sdd-init model = %q, want root model", got)
	}
	if got := agentModelField(t, agents, "sdd-init", "variant"); got != "" {
		t.Errorf("sdd-init variant = %q, want empty (no stale leakage)", got)
	}
}

func TestInjectModelAssignments_JDExcludedFromRootPropagation(t *testing.T) {
	agents := injectAndParse(t, []byte(overlayFixture),
		nil, "anthropic/claude-opus-4-20250514", nil)

	if got := agentModelField(t, agents, "jd-judge-a", "model"); got != "" {
		t.Errorf("jd-judge-a model = %q, want empty (JD excluded from root)", got)
	}
	if got := agentModelField(t, agents, "jd-judge-b", "model"); got != "" {
		t.Errorf("jd-judge-b model = %q, want empty (JD excluded from root)", got)
	}
}

func TestInjectModelAssignments_NoRootNoAssignmentsWritesNothing(t *testing.T) {
	agents := injectAndParse(t, []byte(overlayFixture), nil, "", nil)
	for _, name := range []string{"biggz-orchestrator", "sdd-init", "jd-judge-a"} {
		if got := agentModelField(t, agents, name, "model"); got != "" {
			t.Errorf("%s model = %q, want empty", name, got)
		}
	}
}

func TestInjectModelAssignments_LegacyOrchestratorAlias(t *testing.T) {
	agents := injectAndParse(t, []byte(overlayFixture),
		map[string]model.ModelAssignment{
			"sdd-orchestrator": {ProviderID: "anthropic", ModelID: "claude-haiku-4-20250514"},
		},
		"", nil)

	if got := agentModelField(t, agents, OrchestratorAgent, "model"); got != "anthropic/claude-haiku-4-20250514" {
		t.Errorf("orchestrator model = %q, want legacy alias assignment", got)
	}
}

func TestInjectModelAssignments_RealOrchestratorWinsOverAlias(t *testing.T) {
	agents := injectAndParse(t, []byte(overlayFixture),
		map[string]model.ModelAssignment{
			OrchestratorAgent:  {ProviderID: "anthropic", ModelID: "claude-opus-4-20250514"},
			"sdd-orchestrator": {ProviderID: "anthropic", ModelID: "claude-haiku-4-20250514"},
		},
		"", nil)

	if got := agentModelField(t, agents, OrchestratorAgent, "model"); got != "anthropic/claude-opus-4-20250514" {
		t.Errorf("orchestrator model = %q, want real key to win", got)
	}
}

func TestInjectModelAssignments_InvalidOverlay(t *testing.T) {
	_, err := InjectModelAssignments([]byte(`{ nope`), nil, "", nil)
	if err == nil {
		t.Error("InjectModelAssignments() with invalid overlay: expected error, got nil")
	}
}

func TestInjectModelAssignments_OverlayWithoutAgentsUnchanged(t *testing.T) {
	overlay := []byte("{\n  \"agent\": {}\n}")
	out, err := InjectModelAssignments(overlay, map[string]model.ModelAssignment{"sdd-init": {ProviderID: "a", ModelID: "b"}}, "", nil)
	if err != nil {
		t.Fatalf("InjectModelAssignments() error = %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(out, &root); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if strings.Contains(string(out), "\"model\"") {
		t.Error("no agent definitions to inject into — model must not appear")
	}
}
