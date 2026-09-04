package screens

import (
	"os"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/opencode"
)

// gentleTokenDenylist covers gentle-ai branding and palette references that
// MUST NOT appear in picker output (REQ-WIZ-002, biggz styling only).
var gentleTokenDenylist = []string{
	"gentle-ai",
	"gentle_ai",
	"gentleai",
	"gentle ai",
	"gentleman",
}

func TestPickerPrecedence_AgentsOverUserOverBuiltin(t *testing.T) {
	agents := opencode.AgentModelConfig{
		"sdd-design": {Model: "agent-model", Thinking: opencode.ThinkingHigh},
	}
	user := opencode.AgentModelConfig{
		"sdd-design": {Model: "user-model", Thinking: opencode.ThinkingLow},
		"sdd-spec":   {Model: "user-only", Thinking: opencode.ThinkingMedium},
	}
	builtin := opencode.AgentModelConfig{
		"sdd-design": {Model: "builtin-model"},
		"sdd-spec":   {Model: "builtin-spec"},
		"sdd-init":   {Model: "builtin-init"},
	}

	resolvers := map[string]func(user, builtin opencode.AgentModelConfig) opencode.AgentModelConfig{
		"claude":   ClaudePicker{Agents: agents}.ResolveEffective,
		"codex":    CodexPicker{Agents: agents}.ResolveEffective,
		"kiro":     KiroPicker{Agents: agents}.ResolveEffective,
		"opencode": OpenCodePicker{Agents: agents}.ResolveEffective,
	}

	for name, resolve := range resolvers {
		merged := resolve(user, builtin)
		if merged["sdd-design"].Model != "agent-model" {
			t.Errorf("%s: sdd-design = %q, want agent-model (agents wins)", name, merged["sdd-design"].Model)
		}
		if merged["sdd-spec"].Model != "user-only" {
			t.Errorf("%s: sdd-spec = %q, want user-only (user beats builtin)", name, merged["sdd-spec"].Model)
		}
		if merged["sdd-init"].Model != "builtin-init" {
			t.Errorf("%s: sdd-init = %q, want builtin-init (fallback)", name, merged["sdd-init"].Model)
		}
	}
}

func TestPickerPrecedence_EmptyAgentsDefersToUser(t *testing.T) {
	user := opencode.AgentModelConfig{
		"sdd-design": {Model: "user-model", Thinking: opencode.ThinkingLow},
	}
	builtin := opencode.AgentModelConfig{
		"sdd-design": {Model: "builtin-model"},
	}
	merged := OpenCodePicker{Agents: OpenCodeDefaultAgents()}.ResolveEffective(user, builtin)
	if merged["sdd-design"].Model != "user-model" {
		t.Errorf("opencode empty agents: sdd-design = %q, want user-model", merged["sdd-design"].Model)
	}
}

func TestPickerBackgrounds_Distinct(t *testing.T) {
	seen := map[string]bool{}
	for _, b := range []string{
		ClaudePicker{}.Background(),
		CodexPicker{}.Background(),
		KiroPicker{}.Background(),
		OpenCodePicker{}.Background(),
	} {
		if b == "" {
			t.Error("picker background is empty")
		}
		if seen[b] {
			t.Errorf("duplicate picker background %q", b)
		}
		seen[b] = true
	}
}

func TestPickerViews_ZeroGentleTokens(t *testing.T) {
	dir := t.TempDir()
	cache, variants, settings := pickerPaths(dir)
	writePickerCache(t, cache, variants)

	views := map[string]string{
		"claude":   NewClaudePickerWithPaths(cache, variants, settings).View(),
		"codex":    NewCodexPickerWithPaths(cache, variants, settings).View(),
		"kiro":     NewKiroPickerWithPaths(cache, variants, settings).View(),
		"opencode": NewOpenCodePickerWithPaths(cache, variants, settings).View(),
	}

	for name, view := range views {
		if view == "" {
			t.Errorf("%s: View() is empty", name)
			continue
		}
		lowered := strings.ToLower(view)
		for _, token := range gentleTokenDenylist {
			if strings.Contains(lowered, token) {
				t.Errorf("%s: View() contains gentle token %q", name, token)
			}
		}
	}
}

// TestPickerSources_NoGentleImports guards the import block of each picker:
// no gentle-ai palette packages and no catalog/model/planner ports. Prose
// comments are out of scope; only real imports can widen the dependency
// surface (task 3.2 constraint).
func TestPickerSources_NoGentleImports(t *testing.T) {
	for _, file := range []string{
		"claude_picker.go",
		"codex_picker.go",
		"kiro_picker.go",
		"opencode_picker.go",
	} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		imports := pickerImportBlock(string(data))
		lowered := strings.ToLower(imports)
		for _, token := range []string{"gentle", "catalog", "planner"} {
			if strings.Contains(lowered, token) {
				t.Errorf("%s imports forbidden package %q", file, token)
			}
		}
	}
}

// pickerImportBlock extracts the import (...) block for import-scope checks.
func pickerImportBlock(src string) string {
	start := strings.Index(src, "import (")
	if start < 0 {
		return src
	}
	rest := src[start:]
	end := strings.Index(rest, ")")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

// pickerPaths returns temp cache/variants/settings paths for picker tests.
func pickerPaths(dir string) (cache, variants, settings string) {
	return dir + "/models.json", dir + "/model-variants.json", dir + "/opencode.json"
}

// writePickerCache seeds a minimal model cache plus variants so the wrapped
// ModelPickerScreen has providers to navigate.
func writePickerCache(t *testing.T, cachePath, variantsPath string) {
	t.Helper()
	if err := os.WriteFile(cachePath, []byte(pickerCache), 0o644); err != nil {
		t.Fatalf("write cache: %v", err)
	}
	if err := os.WriteFile(variantsPath, []byte(pickerVariants), 0o644); err != nil {
		t.Fatalf("write variants: %v", err)
	}
}

func TestPickerDelegation_UpdateReachesProviderList(t *testing.T) {
	dir := t.TempDir()
	cache, variants, settings := pickerPaths(dir)
	writePickerCache(t, cache, variants)

	p := NewClaudePickerWithPaths(cache, variants, settings)
	updated, _ := p.Update(keyMsg("enter"))
	cp, ok := updated.(ClaudePicker)
	if !ok {
		t.Fatalf("Update returned %T, want ClaudePicker", updated)
	}
	if cp.Inner.view != mpProvider {
		t.Errorf("inner view = %v, want mpProvider after enter", cp.Inner.view)
	}
}
