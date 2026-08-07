package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/bigmem"
)

// --- helpers ---

func setupStore(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	var err error
	store, err = bigmem.Open(dir)
	if err != nil {
		t.Fatalf("bigmem.Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = orig
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("copy: %v", err)
	}
	return strings.TrimSpace(buf.String())
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	ID     any              `json:"id"`
	Result json.RawMessage  `json:"result,omitempty"`
	Error  *rpcError        `json:"error,omitempty"`
}

func parseRPC(t *testing.T, raw string) *rpcResponse {
	t.Helper()
	var r rpcResponse
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, raw)
	}
	return &r
}

func toolNames(tools []map[string]any) []string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t["name"].(string)
	}
	return names
}

// --- helper function tests ---

func TestGetStr(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		key  string
		want string
	}{
		{"present", map[string]any{"foo": "bar"}, "foo", "bar"},
		{"missing", map[string]any{"foo": "bar"}, "baz", ""},
		{"wrong type", map[string]any{"foo": 42}, "foo", ""},
		{"nil value", map[string]any{"foo": nil}, "foo", ""},
		{"empty string", map[string]any{"foo": ""}, "foo", ""},
		{"empty map", map[string]any{}, "key", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getStr(tt.m, tt.key); got != tt.want {
				t.Errorf("getStr(%v, %q) = %q, want %q", tt.m, tt.key, got, tt.want)
			}
		})
	}
}

func TestGetInt(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		key  string
		def  int
		want int
	}{
		{"present", map[string]any{"n": 42.0}, "n", 0, 42},
		{"missing", map[string]any{}, "n", 10, 10},
		{"wrong type", map[string]any{"n": "42"}, "n", 0, 0},
		{"nil", map[string]any{"n": nil}, "n", 5, 5},
		{"int-like float", map[string]any{"n": 3.14}, "n", 0, 3},
		{"zero", map[string]any{"n": 0.0}, "n", 99, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getInt(tt.m, tt.key, tt.def); got != tt.want {
				t.Errorf("getInt(%v, %q, %d) = %d, want %d", tt.m, tt.key, tt.def, got, tt.want)
			}
		})
	}
}

func TestGetFloat(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		key  string
		def  float64
		want float64
	}{
		{"present", map[string]any{"f": 3.14}, "f", 0, 3.14},
		{"missing", map[string]any{}, "f", 1.5, 1.5},
		{"wrong type", map[string]any{"f": "3.14"}, "f", 0, 0},
		{"zero", map[string]any{"f": 0.0}, "f", 99.9, 0},
		{"int as float", map[string]any{"f": 7.0}, "f", 0, 7.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getFloat(tt.m, tt.key, tt.def); got != tt.want {
				t.Errorf("getFloat(%v, %q, %f) = %f, want %f", tt.m, tt.key, tt.def, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{"short", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"long", "hello world", 5, "hello..."},
		{"empty", "", 5, ""},
		{"one char", "a", 1, "a"},
		{"truncate to zero", "abc", 0, "..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncate(tt.s, tt.max); got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.max, got, tt.want)
			}
		})
	}
}

// --- toolDef tests ---

func TestToolDef(t *testing.T) {
	t.Run("with props", func(t *testing.T) {
		props := map[string]any{"foo": map[string]any{"type": "string"}}
		def := toolDef("test_tool", "A test tool", props, []string{"foo"})
		if def["name"] != "test_tool" {
			t.Errorf("name = %v", def["name"])
		}
		if def["description"] != "A test tool" {
			t.Errorf("description = %v", def["description"])
		}
		schema, ok := def["inputSchema"].(map[string]any)
		if !ok {
			t.Fatal("missing inputSchema")
		}
		if schema["type"] != "object" {
			t.Errorf("schema type = %v", schema["type"])
		}
		if req, ok := schema["required"].([]string); !ok || len(req) != 1 || req[0] != "foo" {
			t.Errorf("required = %v", req)
		}
	})

	t.Run("nil props", func(t *testing.T) {
		def := toolDef("no_props", "No props", nil, nil)
		schema, ok := def["inputSchema"].(map[string]any)
		if !ok {
			t.Fatal("missing inputSchema")
		}
		props, ok := schema["properties"].(map[string]any)
		if !ok || len(props) != 0 {
			t.Errorf("expected empty properties, got %v", props)
		}
	})
}

// --- buildToolList tests ---

func TestBuildToolList_AllToolsRegistered(t *testing.T) {
	tools := buildToolList("agent")
	names := toolNames(tools)

	expected := []string{
		"mem_save", "mem_search", "mem_get_observation", "mem_update", "mem_delete",
		"mem_context", "mem_session_summary", "mem_session_start", "mem_session_end",
		"mem_save_prompt", "mem_current_project", "mem_suggest_topic_key", "mem_timeline",
		"mem_stats", "mem_pin", "mem_unpin", "mem_doctor", "mem_compare", "mem_judge",
		"mem_capture_passive", "mem_merge_projects", "mem_review",
	}

	if len(tools) != len(expected) {
		t.Errorf("got %d tools, want %d\n  got:  %v\n  want: %v", len(tools), len(expected), names, expected)
	}

	for _, want := range expected {
		found := false
		for _, n := range names {
			if n == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing tool: %s", want)
		}
	}
}

func TestBuildToolList_ToolHasDescription(t *testing.T) {
	tools := buildToolList("agent")
	for _, tool := range tools {
		name := tool["name"].(string)
		desc, ok := tool["description"].(string)
		if !ok || desc == "" {
			t.Errorf("tool %q missing description", name)
		}
	}
}

func TestBuildToolList_ToolsHaveInputSchema(t *testing.T) {
	tools := buildToolList("agent")
	for _, tool := range tools {
		name := tool["name"].(string)
		schema, ok := tool["inputSchema"].(map[string]any)
		if !ok {
			t.Errorf("tool %q missing inputSchema", name)
			continue
		}
		if _, ok := schema["properties"]; !ok {
			t.Errorf("tool %q inputSchema missing properties", name)
		}
		if _, ok := schema["required"]; !ok {
			t.Errorf("tool %q inputSchema missing required", name)
		}
	}
}

// --- handleToolCall tests ---

func TestHandleToolCall_UnknownTool(t *testing.T) {
	setupStore(t)
	raw := captureStdout(t, func() {
		handleToolCall("test-1", "nonexistent_tool", map[string]any{})
	})
	r := parseRPC(t, raw)
	if r.Error == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(r.Error.Message, "unknown tool") {
		t.Errorf("error message = %q", r.Error.Message)
	}
}

func TestHandleToolCall_mem_save(t *testing.T) {
	setupStore(t)

	t.Run("happy path", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("s1", "mem_save", map[string]any{
				"title":   "Test save",
				"content": "**What**: testing save",
				"type":    "decision",
				"project": "test",
			})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
		if !strings.Contains(string(r.Result), "Saved: Test save") {
			t.Errorf("result = %s", string(r.Result))
		}
	})

	t.Run("missing title", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("s2", "mem_save", map[string]any{"content": "no title"})
		})
		r := parseRPC(t, raw)
		if r.Error == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(r.Error.Message, "title is required") {
			t.Errorf("error = %q", r.Error.Message)
		}
	})

	t.Run("empty title", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("s3", "mem_save", map[string]any{"title": ""})
		})
		r := parseRPC(t, raw)
		if r.Error == nil {
			t.Fatal("expected error")
		}
	})
}

func TestHandleToolCall_mem_search(t *testing.T) {
	setupStore(t)

	store.Save(&bigmem.Observation{Title: "Search me", Content: "find this", Type: "decision", Project: "test"})

	t.Run("by query", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("q1", "mem_search", map[string]any{"query": "Search me"})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
		if r.Result == nil {
			t.Fatal("expected result")
		}
	})

	t.Run("empty results", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("q2", "mem_search", map[string]any{"query": "ZZZZNOTFOUND"})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
	})

	t.Run("with project filter", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("q3", "mem_search", map[string]any{"query": "Search me", "project": "test"})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
	})

	t.Run("no args", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("q4", "mem_search", map[string]any{})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
	})
}

func TestHandleToolCall_mem_get_observation(t *testing.T) {
	setupStore(t)

	obs := &bigmem.Observation{Title: "Get me", Type: "decision", Content: "content to get"}
	store.Save(obs)

	t.Run("happy path", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("g1", "mem_get_observation", map[string]any{"id": obs.ID})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
		if !strings.Contains(string(r.Result), obs.Title) {
			t.Errorf("result missing title: %s", string(r.Result))
		}
	})

	t.Run("missing id", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("g2", "mem_get_observation", map[string]any{})
		})
		r := parseRPC(t, raw)
		if r.Error == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("not found", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("g3", "mem_get_observation", map[string]any{"id": "nonexistent"})
		})
		r := parseRPC(t, raw)
		if r.Error == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("alias mem_get", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("g4", "mem_get", map[string]any{"id": obs.ID})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
	})
}

func TestHandleToolCall_mem_update(t *testing.T) {
	setupStore(t)

	obs := &bigmem.Observation{Title: "Original", Type: "decision", Content: "v1"}
	store.Save(obs)

	t.Run("happy path", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("u1", "mem_update", map[string]any{
				"id": obs.ID, "title": "Updated", "content": "v2",
			})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
		if !strings.Contains(string(r.Result), "Updated:") {
			t.Errorf("result = %s", string(r.Result))
		}
	})

	t.Run("missing id", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("u2", "mem_update", map[string]any{"title": "x"})
		})
		r := parseRPC(t, raw)
		if r.Error == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(r.Error.Message, "id is required") {
			t.Errorf("error = %q", r.Error.Message)
		}
	})
}

func TestHandleToolCall_mem_delete(t *testing.T) {
	setupStore(t)

	obs := &bigmem.Observation{Title: "Delete me", Type: "discovery"}
	store.Save(obs)

	t.Run("happy path", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("d1", "mem_delete", map[string]any{"id": obs.ID})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
		if !strings.Contains(string(r.Result), "Deleted") {
			t.Errorf("result = %s", string(r.Result))
		}
	})

	t.Run("missing id", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("d2", "mem_delete", map[string]any{})
		})
		r := parseRPC(t, raw)
		if r.Error == nil {
			t.Fatal("expected error")
		}
	})
}

func TestHandleToolCall_mem_context(t *testing.T) {
	setupStore(t)

	t.Run("no sessions", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("c1", "mem_context", map[string]any{})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
		if !strings.Contains(string(r.Result), "No session history") {
			t.Errorf("result = %s", string(r.Result))
		}
	})

	t.Run("with sessions", func(t *testing.T) {
		store.SessionStart("sess-ctx-1", "test")
		store.SessionStart("sess-ctx-2", "test")

		raw := captureStdout(t, func() {
			handleToolCall("c2", "mem_context", map[string]any{"limit": 5.0})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
		if !strings.Contains(string(r.Result), "sess-ctx") {
			t.Errorf("result = %s", string(r.Result))
		}
	})
}

func TestHandleToolCall_mem_session_summary(t *testing.T) {
	setupStore(t)
	store.SessionStart("sess-sum-1", "test")

	t.Run("happy path", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("ss1", "mem_session_summary", map[string]any{
				"session_id": "sess-sum-1",
				"content":    "Completed the feature",
			})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
	})

	t.Run("missing session_id", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("ss2", "mem_session_summary", map[string]any{"content": "x"})
		})
		r := parseRPC(t, raw)
		if r.Error == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(r.Error.Message, "required") {
			t.Errorf("error = %q", r.Error.Message)
		}
	})

	t.Run("missing content", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("ss3", "mem_session_summary", map[string]any{"session_id": "sess-sum-1"})
		})
		r := parseRPC(t, raw)
		if r.Error == nil {
			t.Fatal("expected error")
		}
	})
}

func TestHandleToolCall_mem_session_start(t *testing.T) {
	setupStore(t)

	t.Run("happy path", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("st1", "mem_session_start", map[string]any{
				"id": "sess-new", "project": "test",
			})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
		if !strings.Contains(string(r.Result), "sess-new") {
			t.Errorf("result = %s", string(r.Result))
		}
	})

	t.Run("missing id", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("st2", "mem_session_start", map[string]any{})
		})
		r := parseRPC(t, raw)
		if r.Error == nil {
			t.Fatal("expected error")
		}
	})
}

func TestHandleToolCall_mem_session_end(t *testing.T) {
	setupStore(t)
	store.SessionStart("sess-end-1", "test")

	t.Run("happy path", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("se1", "mem_session_end", map[string]any{
				"id": "sess-end-1", "summary": "Done",
			})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
	})

	t.Run("missing id", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("se2", "mem_session_end", map[string]any{})
		})
		r := parseRPC(t, raw)
		if r.Error == nil {
			t.Fatal("expected error")
		}
	})
}

func TestHandleToolCall_mem_save_prompt(t *testing.T) {
	setupStore(t)

	t.Run("happy path", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("sp1", "mem_save_prompt", map[string]any{
				"content": "User asked about tests", "session_id": "sess-sp-1",
			})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
		if !strings.Contains(string(r.Result), "Prompt saved") {
			t.Errorf("result = %s", string(r.Result))
		}
	})

	t.Run("missing content", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("sp2", "mem_save_prompt", map[string]any{})
		})
		r := parseRPC(t, raw)
		if r.Error == nil {
			t.Fatal("expected error")
		}
	})
}

func TestHandleToolCall_mem_current_project(t *testing.T) {
	setupStore(t)

	raw := captureStdout(t, func() {
		handleToolCall("cp1", "mem_current_project", map[string]any{})
	})
	r := parseRPC(t, raw)
	if r.Error != nil {
		t.Fatalf("unexpected error: %v", r.Error)
	}
	if !strings.Contains(string(r.Result), "project") {
		t.Errorf("result = %s", string(r.Result))
	}
}

func TestHandleToolCall_mem_suggest_topic_key(t *testing.T) {
	setupStore(t)

	t.Run("with title", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("sk1", "mem_suggest_topic_key", map[string]any{
				"title": "Fixed auth bug", "type": "bugfix",
			})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
		if !strings.Contains(string(r.Result), "bugfix") {
			t.Errorf("result = %s", string(r.Result))
		}
	})

	t.Run("with content fallback", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("sk2", "mem_suggest_topic_key", map[string]any{
				"content": "Refactored handler", "type": "architecture",
			})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
		if !strings.Contains(string(r.Result), "architecture") {
			t.Errorf("result = %s", string(r.Result))
		}
	})

	t.Run("no args", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("sk3", "mem_suggest_topic_key", map[string]any{})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
	})
}

func TestHandleToolCall_mem_timeline(t *testing.T) {
	setupStore(t)
	store.Save(&bigmem.Observation{Title: "Timeline entry", Type: "decision"})

	raw := captureStdout(t, func() {
		handleToolCall("tl1", "mem_timeline", map[string]any{"limit": 10.0})
	})
	r := parseRPC(t, raw)
	if r.Error != nil {
		t.Fatalf("unexpected error: %v", r.Error)
	}
	if !strings.Contains(string(r.Result), "Timeline entry") {
		t.Errorf("result = %s", string(r.Result))
	}
}

func TestHandleToolCall_mem_stats(t *testing.T) {
	setupStore(t)
	store.Save(&bigmem.Observation{Title: "Stats test", Type: "decision"})

	raw := captureStdout(t, func() {
		handleToolCall("stat1", "mem_stats", map[string]any{})
	})
	r := parseRPC(t, raw)
	if r.Error != nil {
		t.Fatalf("unexpected error: %v", r.Error)
	}
	if !strings.Contains(string(r.Result), "total_observations") {
		t.Errorf("result = %s", string(r.Result))
	}
}

func TestHandleToolCall_mem_pin_unpin(t *testing.T) {
	setupStore(t)
	obs := &bigmem.Observation{Title: "Pin test", Type: "decision"}
	store.Save(obs)

	t.Run("pin", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("p1", "mem_pin", map[string]any{"id": obs.ID})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
		if !strings.Contains(string(r.Result), "Pinned") {
			t.Errorf("result = %s", string(r.Result))
		}
	})

	t.Run("unpin", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("p2", "mem_unpin", map[string]any{"id": obs.ID})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
		if !strings.Contains(string(r.Result), "Unpinned") {
			t.Errorf("result = %s", string(r.Result))
		}
	})

	t.Run("pin missing id", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("p3", "mem_pin", map[string]any{})
		})
		r := parseRPC(t, raw)
		if r.Error == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("unpin missing id", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("p4", "mem_unpin", map[string]any{})
		})
		r := parseRPC(t, raw)
		if r.Error == nil {
			t.Fatal("expected error")
		}
	})
}

func TestHandleToolCall_mem_doctor(t *testing.T) {
	setupStore(t)

	raw := captureStdout(t, func() {
		handleToolCall("doc1", "mem_doctor", map[string]any{})
	})
	r := parseRPC(t, raw)
	if r.Error != nil {
		t.Fatalf("unexpected error: %v", r.Error)
	}
	if !strings.Contains(string(r.Result), "store_exists") {
		t.Errorf("result = %s", string(r.Result))
	}
}

func TestHandleToolCall_mem_compare(t *testing.T) {
	setupStore(t)
	a := &bigmem.Observation{Title: "Compare A", Type: "decision", TopicKey: "same/topic"}
	b := &bigmem.Observation{Title: "Compare B", Type: "decision", TopicKey: "same/topic"}
	store.Save(a)
	store.Save(b)

	t.Run("happy path", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("comp1", "mem_compare", map[string]any{
				"memory_id_a": a.ID, "memory_id_b": b.ID,
			})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
	})

	t.Run("missing ids", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("comp2", "mem_compare", map[string]any{})
		})
		r := parseRPC(t, raw)
		if r.Error == nil {
			t.Fatal("expected error")
		}
	})
}

func TestHandleToolCall_mem_judge(t *testing.T) {
	setupStore(t)
	a := &bigmem.Observation{Title: "Judge A", Type: "decision", Project: "test"}
	b := &bigmem.Observation{Title: "Judge B", Type: "decision", Project: "test"}
	store.Save(a)
	store.Save(b)

	judgmentID := "rel-" + a.ID + "-" + b.ID

	t.Run("happy path", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("j1", "mem_judge", map[string]any{
				"judgment_id": judgmentID,
				"relation":    "related",
				"reason":      "same topic",
				"confidence":  0.9,
			})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
	})

	t.Run("missing judgment_id", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("j2", "mem_judge", map[string]any{"relation": "related"})
		})
		r := parseRPC(t, raw)
		if r.Error == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing relation", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("j3", "mem_judge", map[string]any{"judgment_id": judgmentID})
		})
		r := parseRPC(t, raw)
		if r.Error == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("nonexistent judgment_id", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("j4", "mem_judge", map[string]any{
				"judgment_id": "rel-nonexistent",
				"relation":    "related",
			})
		})
		r := parseRPC(t, raw)
		if r.Error == nil {
			t.Fatal("expected error for nonexistent judgment")
		}
	})
}

func TestHandleToolCall_mem_capture_passive(t *testing.T) {
	setupStore(t)

	content := "## Key Learnings\n- Found a race condition\n- Fixed it with mutex"

	t.Run("happy path", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("cap1", "mem_capture_passive", map[string]any{
				"content": content, "project": "test",
			})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
		if !strings.Contains(string(r.Result), "Captured") {
			t.Errorf("result = %s", string(r.Result))
		}
	})

	t.Run("missing content", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("cap2", "mem_capture_passive", map[string]any{})
		})
		r := parseRPC(t, raw)
		if r.Error == nil {
			t.Fatal("expected error")
		}
	})
}

func TestHandleToolCall_mem_merge_projects(t *testing.T) {
	setupStore(t)
	store.Save(&bigmem.Observation{Title: "Merge src", Project: "src"})

	t.Run("happy path", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("m1", "mem_merge_projects", map[string]any{
				"source_project": "src", "target_project": "dst",
			})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
		if !strings.Contains(string(r.Result), "Merged") {
			t.Errorf("result = %s", string(r.Result))
		}
	})

	t.Run("missing source", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("m2", "mem_merge_projects", map[string]any{"target_project": "dst"})
		})
		r := parseRPC(t, raw)
		if r.Error == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing target", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("m3", "mem_merge_projects", map[string]any{"source_project": "src"})
		})
		r := parseRPC(t, raw)
		if r.Error == nil {
			t.Fatal("expected error")
		}
	})
}

func TestHandleToolCall_mem_review(t *testing.T) {
	setupStore(t)

	t.Run("list action", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("rv1", "mem_review", map[string]any{"action": "list"})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
		if !strings.Contains(string(r.Result), "need_review") {
			t.Errorf("result = %s", string(r.Result))
		}
	})

		t.Run("mark_reviewed", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("rv2", "mem_review", map[string]any{
				"action": "mark_reviewed", "observation_id": "1",
			})
		})
		r := parseRPC(t, raw)
		if r.Error != nil {
			t.Fatalf("unexpected error: %v", r.Error)
		}
		if !strings.Contains(string(r.Result), "Marked reviewed") {
			t.Errorf("result = %s", string(r.Result))
		}
	})

	t.Run("mark_reviewed missing id", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("rv3", "mem_review", map[string]any{"action": "mark_reviewed"})
		})
		r := parseRPC(t, raw)
		if r.Error == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("unknown action", func(t *testing.T) {
		raw := captureStdout(t, func() {
			handleToolCall("rv4", "mem_review", map[string]any{"action": "bad_action"})
		})
		r := parseRPC(t, raw)
		if r.Error == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(r.Error.Message, "unknown action") {
			t.Errorf("error = %q", r.Error.Message)
		}
	})
}

func TestWriteJSON(t *testing.T) {
	tests := []struct {
		name string
		v    any
		want string
	}{
		{"map", map[string]any{"key": "value"}, `{"key":"value"}`},
		{"string", "hello", `"hello"`},
		{"number", 42, `42`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := captureStdout(t, func() { writeJSON(tt.v) })
			if raw != tt.want {
				t.Errorf("writeJSON(%v) = %q, want %q", tt.v, raw, tt.want)
			}
		})
	}
}

func TestWriteError(t *testing.T) {
	raw := captureStdout(t, func() { writeError("err1", "something went wrong") })
	r := parseRPC(t, raw)
	if r.Error == nil {
		t.Fatal("expected error")
	}
	if r.Error.Code != -32603 {
		t.Errorf("code = %d, want -32603", r.Error.Code)
	}
	if r.Error.Message != "something went wrong" {
		t.Errorf("message = %q", r.Error.Message)
	}
}

func TestTextResult(t *testing.T) {
	raw := captureStdout(t, func() { textResult("t1", "hello world") })
	r := parseRPC(t, raw)
	if r.Error != nil {
		t.Fatalf("unexpected error: %v", r.Error)
	}
	if !strings.Contains(string(r.Result), "hello world") {
		t.Errorf("result = %s", string(r.Result))
	}
	if !strings.Contains(string(r.Result), "text") {
		t.Errorf("result missing content type: %s", string(r.Result))
	}
}

func TestJsonResult(t *testing.T) {
	data := map[string]any{"count": 3, "items": []string{"a", "b"}}
	raw := captureStdout(t, func() { jsonResult("j1", data) })
	r := parseRPC(t, raw)
	if r.Error != nil {
		t.Fatalf("unexpected error: %v", r.Error)
	}
	if !strings.Contains(string(r.Result), "count") {
		t.Errorf("result = %s", string(r.Result))
	}
}
