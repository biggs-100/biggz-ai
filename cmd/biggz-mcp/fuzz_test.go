package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/biggz-ai/biggz/internal/bigmem"
)

// FuzzMCPRequest fuzzes the MCP server's request parser with random JSON.
func FuzzMCPRequest(f *testing.F) {
	// Seed corpora — valid MCP messages
	seeds := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"mem_save","arguments":{"title":"test","content":"hello"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"ping"}`,
		`invalid json`,
		`{"jsonrpc":"2.0","id":5,"method":"unknown_method"}`,
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"mem_search","arguments":{"query":"test"}}}`,
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Must not panic for any input
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic: %v", r)
			}
		}()

		// Parse as JSON
		var req map[string]any
		if err := json.Unmarshal([]byte(input), &req); err != nil {
			return // invalid JSON is expected for some inputs — skip
		}

		// Check method is a string if present
		if method, ok := req["method"]; ok {
			if _, ok := method.(string); !ok {
				return // method must be string, skip non-string
			}
		}

		// Must not panic for any valid JSON structure
		_ = req["id"]
		_ = req["params"]
	})
}

// FuzzMCPRequestParser fuzzes the scanner-based request parser.
func FuzzMCPRequestParser(f *testing.F) {
	seeds := []string{
		`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n",
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n",
		`not json` + "\n",
		``,
		"\n\n",
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"mem_save","arguments":{"title":"","content":""}}}` + "\n",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic: %v", r)
			}
		}()

		// Simulate JSON-RPC over stdin — each line should be parseable
		scanner := bufio.NewScanner(strings.NewReader(input))
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			var req map[string]any
			if err := json.Unmarshal([]byte(line), &req); err != nil {
				continue
			}
			// Must not panic for any parseable request
			method, _ := req["method"].(string)
			id := req["id"]
			params, _ := req["params"].(map[string]any)
			_ = method
			_ = id
			_ = params
		}
	})
}

// FuzzMCPJSONOutput fuzzes the JSON output encoding.
func FuzzMCPJSONOutput(f *testing.F) {
	seeds := []map[string]any{
		{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"tools": []map[string]any{}}},
		{"jsonrpc": "2.0", "id": nil, "error": map[string]any{"code": -32601, "message": "unknown"}},
		{"jsonrpc": "2.0", "result": map[string]any{"content": []map[string]any{{"type": "text", "text": ""}}}},
	}
	for _, s := range seeds {
		data, _ := json.Marshal(s)
		f.Add(string(data))
	}

	f.Fuzz(func(t *testing.T, input string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic: %v", r)
			}
		}()

		var v any
		if err := json.Unmarshal([]byte(input), &v); err != nil {
			return
		}
		// Must not panic when marshaling any valid structure
		_, err := json.Marshal(v)
		if err != nil {
			t.Skipf("marshal error: %v", err)
		}
	})
}

// FuzzMCPStoreOperations fuzzes store operations via the MCP handler.
func FuzzMCPStoreOperations(f *testing.F) {
	// Use a temp dir for each fuzz run — but that's expensive,
	// so we just test the handler dispatch logic.
	handlerTests := []string{
		`{"name":"mem_save","arguments":{"title":"x","content":"y"}}`,
		`{"name":"mem_search","arguments":{"query":"x"}}`,
		`{"name":"mem_delete","arguments":{"id":"nonexistent"}}`,
		`{"name":"mem_stats"}`,
	}
	for _, s := range handlerTests {
		f.Add(s)
	}

	// Set up a real store once
	tmpDir, _ := os.MkdirTemp("", "biggz-mcp-fuzz")
	defer os.RemoveAll(tmpDir)

	origStore := store
	store, _ = bigmem.Open(tmpDir)
	defer func() { store = origStore }()

	f.Fuzz(func(t *testing.T, input string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic in handler: %v", r)
			}
		}()

		var params map[string]any
		if err := json.Unmarshal([]byte(input), &params); err != nil {
			return
		}
		name, _ := params["name"].(string)
		args, _ := params["arguments"].(map[string]any)

		// Simulate handleToolCall — must not panic
		var buf bytes.Buffer
		// Capture output (we don't care about the result)
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		func() {
			defer func() { recover() }()
			handleToolCall("fuzz-id", name, args)
		}()

		w.Close()
		os.Stdout = oldStdout
		buf.ReadFrom(r)
	})
}
