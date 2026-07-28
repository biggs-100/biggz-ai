package filemerge

import (
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
