package filemerge

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestMergeJSONC_Basic(t *testing.T) {
	existing := []byte(`{"name": "test", "version": 1}`)
	overlay := []byte(`{"enabled": true}`)

	result, err := MergeJSONC(existing, overlay)
	if err != nil {
		t.Fatalf("MergeJSONC() returned error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v\n%s", err, string(result))
	}

	if parsed["name"] != "test" {
		t.Errorf("name = %v, want %v", parsed["name"], "test")
	}
	if parsed["version"] != float64(1) {
		t.Errorf("version = %v, want %v", parsed["version"], float64(1))
	}
	if parsed["enabled"] != true {
		t.Errorf("enabled = %v, want true", parsed["enabled"])
	}
}

func TestMergeJSONC_OverlayReplaces(t *testing.T) {
	existing := []byte(`{"name": "original", "version": 1, "enabled": false}`)
	overlay := []byte(`{"name": "replaced", "enabled": true}`)

	result, err := MergeJSONC(existing, overlay)
	if err != nil {
		t.Fatalf("MergeJSONC() returned error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v\n%s", err, string(result))
	}

	if parsed["name"] != "replaced" {
		t.Errorf("name = %v, want %v", parsed["name"], "replaced")
	}
	if parsed["version"] != float64(1) {
		t.Errorf("version = %v, want %v", parsed["version"], float64(1))
	}
	if parsed["enabled"] != true {
		t.Errorf("enabled = %v, want true", parsed["enabled"])
	}
}

func TestMergeJSONC_StripComments(t *testing.T) {
	input := []byte(`{
		// This is a single-line comment
		"name": "test", /* inline comment */
		/*
		   Multi-line comment
		*/
		"version": 1
	}`)

	stripped := stripComments(input)

	if strings.Contains(string(stripped), "//") && strings.Contains(string(stripped), "This is a") {
		t.Error("stripComments() did not remove // single-line comment")
	}
	if strings.Contains(string(stripped), "/*") {
		t.Error("stripComments() did not remove /* */ comment")
	}
	if !strings.Contains(string(stripped), `"name"`) {
		t.Error("stripComments() removed actual content")
	}
	if !strings.Contains(string(stripped), `"version"`) {
		t.Error("stripComments() removed actual content")
	}

	// Should still be valid enough to parse after stripTrailingCommas
	cleaned := stripTrailingCommas(stripped)
	var parsed map[string]any
	if err := json.Unmarshal(cleaned, &parsed); err != nil {
		t.Fatalf("stripped + cleaned is not valid JSON: %v\n%s", err, string(cleaned))
	}
}

func TestMergeJSONC_StripTrailingCommas(t *testing.T) {
	input := []byte(`{
		"a": 1,
		"b": [1, 2,],
		"c": {
			"d": 3,
		},
	}`)

	// Strip comments first (no-op here)
	noComments := stripComments(input)
	result := stripTrailingCommas(noComments)

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("after stripTrailingCommas, output is not valid JSON: %v\n%s", err, string(result))
	}

	if parsed["a"] != float64(1) {
		t.Errorf("a = %v, want %v", parsed["a"], float64(1))
	}
}

func TestMergeJSONC_CommentsInStrings(t *testing.T) {
	input := []byte(`{
		"url": "http://example.com/api", // trailing comment
		"code": "return /* not a comment */;",
		"regex": "foo//bar"
	}`)

	stripped := stripComments(input)
	cleaned := stripTrailingCommas(stripped)

	var parsed map[string]any
	if err := json.Unmarshal(cleaned, &parsed); err != nil {
		t.Fatalf("stripped + cleaned is not valid JSON: %v\n%s", err, string(cleaned))
	}

	if parsed["url"] != "http://example.com/api" {
		t.Errorf("url = %v, want %v", parsed["url"], "http://example.com/api")
	}
	if parsed["code"] != "return /* not a comment */;" {
		t.Errorf("code = %v, want %v", parsed["code"], "return /* not a comment */;")
	}
	if parsed["regex"] != "foo//bar" {
		t.Errorf("regex = %v, want %v", parsed["regex"], "foo//bar")
	}
}

func TestMergeJSONC_EmptyOverlay(t *testing.T) {
	existing := []byte(`{"name": "test"}`)

	result, err := MergeJSONC(existing, nil)
	if err != nil {
		t.Fatalf("MergeJSONC() with nil overlay returned error: %v", err)
	}

	if string(result) != string(existing) {
		t.Errorf("with nil overlay, result = %s, want %s", string(result), string(existing))
	}

	// Also test with empty slice
	result, err = MergeJSONC(existing, []byte{})
	if err != nil {
		t.Fatalf("MergeJSONC() with empty overlay returned error: %v", err)
	}
	if string(result) != string(existing) {
		t.Errorf("with empty overlay, result = %s, want %s", string(result), string(existing))
	}
}

func TestMergeJSONC_InvalidJSON(t *testing.T) {
	tests := []struct {
		name     string
		existing []byte
		overlay  []byte
	}{
		{"invalid existing", []byte(`{not json}`), []byte(`{"a": 1}`)},
		{"invalid overlay", []byte(`{"a": 1}`), []byte(`{not json}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := MergeJSONC(tt.existing, tt.overlay)
			if err == nil {
				t.Error("MergeJSONC() expected error, got nil")
			}
		})
	}
}

func TestMergeJSONC_DeepMergeNested(t *testing.T) {
	existing := []byte(`{"settings": {"theme": "dark", "font": 12}}`)
	overlay := []byte(`{"settings": {"theme": "light"}}`)

	result, err := MergeJSONC(existing, overlay)
	if err != nil {
		t.Fatalf("MergeJSONC() returned error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v\n%s", err, string(result))
	}

	settings, ok := parsed["settings"].(map[string]any)
	if !ok {
		t.Fatal("settings must be a map")
	}
	if settings["theme"] != "light" {
		t.Errorf("settings.theme = %v, want %v", settings["theme"], "light")
	}
	if settings["font"] != float64(12) {
		t.Errorf("settings.font = %v, want %v", settings["font"], float64(12))
	}
}

func TestMergeJSONC_DeepReplaceSentinel_NewKey(t *testing.T) {
	existing := []byte(`{"existing": "keep"}`)
	overlay := []byte(`{"agent": {"biggz": {"tools": {"__replace__": true, "read": true, "write": true}}}}`)

	result, err := MergeJSONC(existing, overlay)
	if err != nil {
		t.Fatalf("MergeJSONC() returned error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v\n%s", err, string(result))
	}

	// existing key must survive
	if parsed["existing"] != "keep" {
		t.Errorf("existing key should be preserved")
	}

	// agent key must be a map
	agent, ok := parsed["agent"].(map[string]any)
	if !ok {
		t.Fatal("agent must be a map")
	}

	// __replace__ must NOT survive anywhere in the tree
	data, _ := json.Marshal(result)
	if bytes.Contains(data, []byte("__replace__")) {
		t.Errorf("__replace__ should not appear in output:\n%s", string(result))
	}

	// tools must have the overlay keys
	biggz, ok := agent["biggz"].(map[string]any)
	if !ok {
		t.Fatal("agent.biggz must be a map")
	}
	tools, ok := biggz["tools"].(map[string]any)
	if !ok {
		t.Fatal("agent.biggz.tools must be a map")
	}
	if tools["read"] != true {
		t.Error("agent.biggz.tools.read should be true")
	}
	if tools["write"] != true {
		t.Error("agent.biggz.tools.write should be true")
	}
}

func TestMergeJSONC_DeepReplaceSentinel(t *testing.T) {
	existing := []byte(`{"nested": {"a": 1, "b": 2}}`)
	overlay := []byte(`{"nested": {"__replace__": true, "c": 3}}`)

	result, err := MergeJSONC(existing, overlay)
	if err != nil {
		t.Fatalf("MergeJSONC() returned error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v\n%s", err, string(result))
	}

	nested, ok := parsed["nested"].(map[string]any)
	if !ok {
		t.Fatal("nested must be a map")
	}
	if nested["c"] != float64(3) {
		t.Errorf("nested.c = %v, want %v", nested["c"], float64(3))
	}
	if _, exists := nested["a"]; exists {
		t.Error("nested.a should not exist after __replace__")
	}
	if _, exists := nested["b"]; exists {
		t.Error("nested.b should not exist after __replace__")
	}
	if _, exists := nested["__replace__"]; exists {
		t.Error("__replace__ key should be stripped from output")
	}
}

func TestMergeJSONC_DeepArrayReplacement(t *testing.T) {
	existing := []byte(`{"items": [1, 2, 3], "tags": ["old"]}`)
	overlay := []byte(`{"items": [4, 5]}`)

	result, err := MergeJSONC(existing, overlay)
	if err != nil {
		t.Fatalf("MergeJSONC() returned error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v\n%s", err, string(result))
	}

	items, ok := parsed["items"].([]any)
	if !ok {
		t.Fatal("items must be an array")
	}
	if len(items) != 2 {
		t.Errorf("len(items) = %d, want 2", len(items))
	}
	if items[0] != float64(4) || items[1] != float64(5) {
		t.Errorf("items = %v, want [4, 5]", items)
	}

	// tags should be preserved from existing
	tags, ok := parsed["tags"].([]any)
	if !ok {
		t.Fatal("tags must be an array")
	}
	if len(tags) != 1 || tags[0] != "old" {
		t.Errorf("tags = %v, want [\"old\"]", tags)
	}
}

func TestMergeJSONC_DeepMultiLevel(t *testing.T) {
	existing := []byte(`{"a": {"b": {"c": 1, "d": 2}, "e": 3}, "f": 4}`)
	overlay := []byte(`{"a": {"b": {"c": 10}, "g": 5}}`)

	result, err := MergeJSONC(existing, overlay)
	if err != nil {
		t.Fatalf("MergeJSONC() returned error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v\n%s", err, string(result))
	}

	a, _ := parsed["a"].(map[string]any)
	b, _ := a["b"].(map[string]any)

	if b["c"] != float64(10) {
		t.Errorf("a.b.c = %v, want 10", b["c"])
	}
	if b["d"] != float64(2) {
		t.Errorf("a.b.d = %v, want 2 (preserved from existing)", b["d"])
	}
	if a["e"] != float64(3) {
		t.Errorf("a.e = %v, want 3 (preserved from existing)", a["e"])
	}
	if a["g"] != float64(5) {
		t.Errorf("a.g = %v, want 5 (added from overlay)", a["g"])
	}
	if parsed["f"] != float64(4) {
		t.Errorf("f = %v, want 4 (preserved from existing)", parsed["f"])
	}
}

func TestMergeJSONC_DeepFlatKeyReplace(t *testing.T) {
	existing := []byte(`{"name": "original", "color": "red"}`)
	overlay := []byte(`{"name": "replaced"}`)

	result, err := MergeJSONC(existing, overlay)
	if err != nil {
		t.Fatalf("MergeJSONC() returned error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v\n%s", err, string(result))
	}

	if parsed["name"] != "replaced" {
		t.Errorf("name = %v, want %v", parsed["name"], "replaced")
	}
	if parsed["color"] != "red" {
		t.Errorf("color = %v, want %v (preserved from existing)", parsed["color"], "red")
	}
}

func TestMergeJSONC_JSONCWithComments(t *testing.T) {
	existing := []byte(`{
		// Agent configuration
		"name": "opencode",
		"skills": ["sdd-init", "sdd-apply"],
	}`)
	overlay := []byte(`{
		/* Add sub_agents section */
		"sub_agents": ["coder", "reviewer"],
	}`)

	result, err := MergeJSONC(existing, overlay)
	if err != nil {
		t.Fatalf("MergeJSONC() returned error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v\n%s", err, string(result))
	}

	if parsed["name"] != "opencode" {
		t.Errorf("name = %v, want %v", parsed["name"], "opencode")
	}
	if parsed["sub_agents"] == nil {
		t.Error("sub_agents key missing from merged result")
	}
}

func TestUnmarshalJSONObject_JSONC(t *testing.T) {
	raw := []byte("{\n  // comment line\n  \"name\": \"test\",\n  \"nested\": { \"enabled\": true, },\n}")
	object, err := UnmarshalJSONObject(raw)
	if err != nil {
		t.Fatalf("UnmarshalJSONObject() error = %v", err)
	}
	if object["name"] != "test" {
		t.Errorf("name = %v, want test", object["name"])
	}
	if nested, ok := object["nested"].(map[string]any); !ok || nested["enabled"] != true {
		t.Errorf("nested = %v, want {enabled: true}", object["nested"])
	}
}

func TestUnmarshalJSONObject_Invalid(t *testing.T) {
	if _, err := UnmarshalJSONObject([]byte("not json")); err == nil {
		t.Error("UnmarshalJSONObject() invalid input: expected error, got nil")
	}
}
