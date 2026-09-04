package main

// Unit tests for the mem_search limit validation (SDD fix-bigmem-mcp-nplus1,
// Phase 2): missing/non-numeric/<=0 defaults to 20, >50 clamps to 50 with an
// explicit requested value for the stderr signal.

import (
	"encoding/json"
	"testing"
)

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
