package sdd

// subroute_test.go — OR-003: the orchestrator-declared organic subroute is
// read from state.yaml and surfaced only when route == organic. The CLI never
// infers, defaults, or synthesizes the value: undeclared, invalid, malformed,
// SDD, and change-less cases omit it and never error.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// isolatedSubrouteHome points HOME/USERPROFILE at a temp dir so status
// derivation never reads the real BigMem store.
func isolatedSubrouteHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

// seedSubrouteChange seeds a proposal-only (route=organic) change and writes
// stateYAML verbatim as state.yaml when non-empty. It returns the openspec root.
func seedSubrouteChange(t *testing.T, name, stateYAML string) string {
	t.Helper()
	workspace := t.TempDir()
	changeRoot := seedDeriveChange(t, workspace, name, map[string]string{
		"proposal.md": "# Proposal\n",
	})
	if stateYAML != "" {
		if err := os.WriteFile(filepath.Join(changeRoot, "state.yaml"), []byte(stateYAML), 0o644); err != nil {
			t.Fatalf("write state.yaml: %v", err)
		}
	}
	return filepath.Join(workspace, "openspec")
}

// deriveSubrouteChange derives status and returns the named active change.
func deriveSubrouteChange(t *testing.T, root, name string) ChangeStatus {
	t.Helper()
	active, _, err := StatusWithOptions(root, StatusOptions{})
	if err != nil {
		t.Fatalf("StatusWithOptions error = %v", err)
	}
	for _, cs := range active {
		if cs.Name == name {
			return cs
		}
	}
	t.Fatalf("change %q not active (active = %d)", name, len(active))
	return ChangeStatus{}
}

// assertSubrouteWire asserts the JSON contract on ChangeStatus: the subroute
// key carries the declared value or is absent entirely (never guessed).
func assertSubrouteWire(t *testing.T, cs ChangeStatus, want string) {
	t.Helper()
	raw, err := json.Marshal(cs)
	if err != nil {
		t.Fatalf("marshal ChangeStatus: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("unmarshal ChangeStatus: %v", err)
	}
	value, present := fields["subroute"]
	if want == "" {
		if present {
			t.Fatalf("subroute key present, want omitted: %s", raw)
		}
		return
	}
	if !present || string(value) != `"`+want+`"` {
		t.Fatalf("subroute = %s (present = %v), want %q", value, present, want)
	}
}

func TestSubrouteOrganicDeclared(t *testing.T) {
	for _, declared := range []string{"direct-inline", "delegated-direct"} {
		t.Run(declared, func(t *testing.T) {
			isolatedSubrouteHome(t)
			root := seedSubrouteChange(t, "declared-change",
				"phases:\n  propose: pending\nsubroute: "+declared+"\n")
			cs := deriveSubrouteChange(t, root, "declared-change")
			if cs.Route != "organic" {
				t.Fatalf("route = %q, want organic", cs.Route)
			}
			if cs.Subroute != declared {
				t.Fatalf("subroute = %q, want %q", cs.Subroute, declared)
			}
			assertSubrouteWire(t, cs, declared)
		})
	}
}

func TestSubrouteUndeclaredOmitted(t *testing.T) {
	tests := []struct{ name, state string }{
		{"no state.yaml", ""},
		{"state.yaml without subroute", "phases:\n  propose: done\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolatedSubrouteHome(t)
			root := seedSubrouteChange(t, "undeclared-change", tt.state)
			cs := deriveSubrouteChange(t, root, "undeclared-change")
			if cs.Route != "organic" {
				t.Fatalf("route = %q, want organic", cs.Route)
			}
			if cs.Subroute != "" {
				t.Fatalf("subroute = %q, want empty", cs.Subroute)
			}
			assertSubrouteWire(t, cs, "")
		})
	}
}

func TestSubrouteInvalidIgnored(t *testing.T) {
	tests := []struct{ name, state string }{
		{"sdd is not a subroute", "subroute: sdd\n"},
		{"direct is not declared vocabulary", "subroute: direct\n"},
		{"wrong case", "subroute: DIRECT-INLINE\n"},
		{"empty declaration", "subroute: \"\"\n"},
		{"mapping instead of scalar", "subroute:\n  nested: direct-inline\n"},
		{"malformed yaml", "subroute: [unclosed\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolatedSubrouteHome(t)
			root := seedSubrouteChange(t, "invalid-change", tt.state)
			cs := deriveSubrouteChange(t, root, "invalid-change")
			if cs.Route != "organic" {
				t.Fatalf("route = %q, want organic", cs.Route)
			}
			if cs.Subroute != "" {
				t.Fatalf("subroute = %q, want empty for invalid declaration", cs.Subroute)
			}
			assertSubrouteWire(t, cs, "")
		})
	}
}

func TestSubrouteSDDRouteSuppresses(t *testing.T) {
	isolatedSubrouteHome(t)
	workspace := t.TempDir()
	changeRoot := seedDeriveChange(t, workspace, "sdd-change", map[string]string{
		"proposal.md": "# Proposal\n",
		"tasks.md":    "- [ ] T1\n",
	})
	if err := os.WriteFile(filepath.Join(changeRoot, "state.yaml"), []byte("subroute: direct-inline\n"), 0o644); err != nil {
		t.Fatalf("write state.yaml: %v", err)
	}
	cs := deriveSubrouteChange(t, filepath.Join(workspace, "openspec"), "sdd-change")
	if cs.Route != "sdd" {
		t.Fatalf("route = %q, want sdd", cs.Route)
	}
	if cs.Subroute != "" {
		t.Fatalf("subroute = %q, want empty for SDD route", cs.Subroute)
	}
	assertSubrouteWire(t, cs, "")
}

func TestSubrouteChangeLessWorkspace(t *testing.T) {
	isolatedSubrouteHome(t)
	workspace := t.TempDir()
	changesDir := filepath.Join(workspace, "openspec", "changes")
	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatalf("mkdir changes: %v", err)
	}
	active, archived, err := StatusWithOptions(filepath.Join(workspace, "openspec"), StatusOptions{})
	if err != nil {
		t.Fatalf("StatusWithOptions error = %v", err)
	}
	if len(active) != 0 || len(archived) != 0 {
		t.Fatalf("active/archived = %d/%d, want 0/0 (no route, no subroute)", len(active), len(archived))
	}
	entries, err := os.ReadDir(changesDir)
	if err != nil {
		t.Fatalf("read changes dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("status synthesized %d entries in openspec/changes: %v", len(entries), entries)
	}
}
