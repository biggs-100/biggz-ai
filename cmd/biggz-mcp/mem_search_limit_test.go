package main

// Unit tests for the mem_search limit validation (SDD fix-bigmem-mcp-nplus1,
// Phase 2): missing/non-numeric/<=0 defaults to 20, >50 clamps to 50 with an
// explicit requested value for the stderr signal.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestPrefixMemRefs(t *testing.T) {
	in := "Call mem_save, then mem_search. After biggz_mem_save judge with biggz_mem_judge. Deferred: mem_update, bigmem_branch_create. BigMem provides memory."
	got := prefixMemRefs(in, "biggz")
	for _, want := range []string{"biggz_mem_save", "biggz_mem_search", "biggz_mem_judge", "biggz_mem_update", "biggz_bigmem_branch_create"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in %q", want, got)
		}
	}
	for _, banned := range []string{"biggz_biggz_", "bigbiggz_"} {
		if strings.Contains(got, banned) {
			t.Errorf("double-prefix %q in %q", banned, got)
		}
	}
	if !strings.Contains(got, "BigMem provides") {
		t.Errorf("prose must be untouched: %q", got)
	}
	if out := prefixMemRefs(in, ""); out != in {
		t.Errorf("empty prefix must be identity, got %q", out)
	}
}

func TestParseSearchLimit(t *testing.T) {
	cases := []struct {
		name        string
		args        map[string]any
		wantEff     int
		wantReq     int
		wantClamped bool
	}{
		{name: "missing defaults to 20", args: map[string]any{}, wantEff: 20},
		{name: "nil defaults to 20", args: map[string]any{"limit": nil}, wantEff: 20},
		{name: "normal value passes through", args: map[string]any{"limit": float64(10)}, wantEff: 10},
		{name: "boundary 50 passes through", args: map[string]any{"limit": float64(50)}, wantEff: 50},
		{name: "zero defaults to 20", args: map[string]any{"limit": float64(0)}, wantEff: 20},
		{name: "negative defaults to 20", args: map[string]any{"limit": float64(-3)}, wantEff: 20},
		{name: "oversize clamps with signal", args: map[string]any{"limit": float64(100000)}, wantEff: 50, wantReq: 100000, wantClamped: true},
		{name: "just over clamps", args: map[string]any{"limit": float64(51)}, wantEff: 50, wantReq: 51, wantClamped: true},
		{name: "non-numeric string defaults", args: map[string]any{"limit": "abc"}, wantEff: 20},
		{name: "numeric string parses", args: map[string]any{"limit": "30"}, wantEff: 30},
		{name: "bool defaults", args: map[string]any{"limit": true}, wantEff: 20},
		{name: "int passes through", args: map[string]any{"limit": 15}, wantEff: 15},
		{name: "json number clamps", args: map[string]any{"limit": json.Number("75")}, wantEff: 50, wantReq: 75, wantClamped: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eff, req, clamped := parseSearchLimit(tc.args)
			if eff != tc.wantEff || req != tc.wantReq || clamped != tc.wantClamped {
				t.Fatalf("parseSearchLimit(%v) = (%d,%d,%v), want (%d,%d,%v)",
					tc.args["limit"], eff, req, clamped, tc.wantEff, tc.wantReq, tc.wantClamped)
			}
		})
	}
}

func TestResolveSearchProject(t *testing.T) {
	// all_projects forces "" even with explicit project.
	if got := resolveSearchProject("foo", true, "/repo"); got != "" {
		t.Errorf("all_projects: got %q want empty", got)
	}
	// Explicit project wins (normalized).
	if got := resolveSearchProject("MyProj", false, "/repo"); got == "" {
		t.Errorf("explicit project should not be empty")
	}
	// Empty + invalid cwd => "" (all).
	if got := resolveSearchProject("", false, ""); got != "" {
		t.Errorf("empty cwd: got %q want empty", got)
	}
	// Empty + nonexistent dir => "" (all, no error).
	dir := t.TempDir()
	if got := resolveSearchProject("", false, dir); got != "" {
		t.Logf("autodetect in empty tmp returned %q (allowed: empty or basename)", got)
	}
}

func TestSearchPreviewBudget120(t *testing.T) {
	// Snapshot: search previews must stay at 120 chars (payload trim).
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	s := string(src)
	if !strings.Contains(s, "truncate(r.Content, 120)") {
		t.Errorf("preview budget changed: want truncate(r.Content, 120)")
	}
	if strings.Contains(s, "truncate(r.Content, 300)") {
		t.Errorf("stale 300-char preview still present")
	}
	if !strings.Contains(s, "previews (120 chars)") {
		t.Errorf("stderr message must say previews (120 chars)")
	}
	if got := truncate(strings.Repeat("x", 200), 120); len(got) != 123 {
		t.Errorf("truncate(200,120) len = %d, want 123", len(got))
	}
}

func TestTrimmedPayloads(t *testing.T) {
	setupStore(t)
	// mem_current_project: no duplicated path/cwd keys.
	raw := captureStdout(t, func() {
		handleToolCall("trim-cp", "mem_current_project", map[string]any{})
	})
	r := parseRPC(t, raw)
	if r.Error != nil {
		t.Fatalf("current_project error: %v", r.Error)
	}
	var envelope struct {
		Content []struct {
			JSON map[string]any `json:"json"`
		} `json:"content"`
	}
	if err := json.Unmarshal(r.Result, &envelope); err != nil {
		t.Fatalf("unmarshal current_project: %v", err)
	}
	if len(envelope.Content) == 0 {
		t.Fatal("empty content")
	}
	payload := envelope.Content[0].JSON
	if _, ok := payload["path"]; ok {
		t.Errorf("payload still has duplicate key \"path\"")
	}
	if _, ok := payload["cwd"]; ok {
		t.Errorf("payload still has duplicate key \"cwd\"")
	}
	if _, ok := payload["project"]; !ok {
		t.Errorf("payload missing \"project\"")
	}
	// mem_stats: no by_type unless verbose.
	t.Setenv("BIGGZ_VERBOSE", "")
	raw = captureStdout(t, func() {
		handleToolCall("trim-st", "mem_stats", map[string]any{})
	})
	r = parseRPC(t, raw)
	if r.Error != nil {
		t.Fatalf("stats error: %v", r.Error)
	}
	if strings.Contains(string(r.Result), "by_type") {
		t.Errorf("stats should omit by_type without BIGGZ_VERBOSE=1: %s", string(r.Result))
	}
}
