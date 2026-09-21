package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type oddTaskCounts struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
}

type oddStatusEntry struct {
	Path         string        `json:"path"`
	TaskProgress oddTaskCounts `json:"taskProgress"`
	LastTouched  string        `json:"lastTouched"`
}

type oddStatusChange struct {
	Name            string   `json:"name"`
	NextRecommended string   `json:"nextRecommended"`
	BlockedReasons  []string `json:"blockedReasons"`
}

// runOddStatusJSON runs `sdd-status --json` in the workspace and returns the
// raw `odd` array, its decoded entries, and the active envelope, failing fast
// when the CLI errors or the key is absent/null.
func runOddStatusJSON(t *testing.T, workspace string) (json.RawMessage, []oddStatusEntry, []oddStatusChange) {
	t.Helper()
	code, stdout, stderr := runSDDStatusCLIArgs(t, "--cwd", workspace, "--json")
	if code != 0 {
		t.Fatalf("status exit code = %d (stderr: %q)", code, stderr)
	}
	var envelope struct {
		Active []oddStatusChange `json:"active"`
		Odd    json.RawMessage   `json:"odd"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("parse envelope: %v\n%s", err, stdout)
	}
	if envelope.Odd == nil {
		t.Fatalf("odd key missing from envelope: %s", stdout)
	}
	var entries []oddStatusEntry
	if err := json.Unmarshal(envelope.Odd, &entries); err != nil {
		t.Fatalf("parse odd array: %v\n%s", err, envelope.Odd)
	}
	return envelope.Odd, entries, envelope.Active
}

// seedOddWorkspace creates a workspace where the openspec/ guard is
// satisfiable but no SDD change exists.
func seedOddWorkspace(t *testing.T) string {
	t.Helper()
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "openspec", "changes"), 0o755); err != nil {
		t.Fatalf("mkdir openspec/changes: %v", err)
	}
	return workspace
}

// writeOddDoc writes odd/tasks/<name> and returns its expected UTC stamp.
func writeOddDoc(t *testing.T, workspace, name, content string, mod time.Time) string {
	t.Helper()
	dir := filepath.Join(workspace, "odd", "tasks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir odd/tasks: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatalf("chtimes %s: %v", name, err)
	}
	return mod.UTC().Format(time.RFC3339)
}

// assertNoSyntheticChanges proves the read-only run added no change entries.
func assertNoSyntheticChanges(t *testing.T, workspace string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(workspace, "openspec", "changes"))
	if err != nil {
		t.Fatalf("read openspec/changes: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("openspec/changes gained %d entries, want 0 (no synthetic change)", len(entries))
	}
}

// TestOddStatusSurfaces covers SS-ODD-001-S1 in both surfaces: JSON carries
// exactly {path, taskProgress{total,completed}, lastTouched}, and the human
// output renders `<path> — <completed>/<total> tasks — <lastTouched>`.
func TestOddStatusSurfaces(t *testing.T) {
	workspace := seedOddWorkspace(t)
	mod := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	stamp := writeOddDoc(t, workspace, "a.md", "- [x] 1\n- [X] 2\n- [x] 3\n- [ ] 4\n- [ ] 5\n", mod)
	writeOddDoc(t, workspace, "b.md", "- [ ] one\n- [ ] two\n", mod)

	raw, entries, _ := runOddStatusJSON(t, workspace)
	want := []oddStatusEntry{
		{Path: "odd/tasks/a.md", TaskProgress: oddTaskCounts{Total: 5, Completed: 3}, LastTouched: stamp},
		{Path: "odd/tasks/b.md", TaskProgress: oddTaskCounts{Total: 2, Completed: 0}, LastTouched: stamp},
	}
	if len(entries) != len(want) {
		t.Fatalf("odd entries = %d, want 2: %s", len(entries), raw)
	}
	for i, entry := range entries {
		if entry != want[i] {
			t.Errorf("odd[%d] = %#v, want %#v", i, entry, want[i])
		}
	}
	var keySets []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keySets); err != nil {
		t.Fatalf("parse odd key sets: %v", err)
	}
	for i, keys := range keySets {
		if len(keys) != 3 {
			t.Errorf("odd[%d] keys = %v, want exactly path, taskProgress, lastTouched", i, keys)
		}
	}

	code, stdout, stderr := runSDDStatusCLIArgs(t, "--cwd", workspace)
	if code != 0 {
		t.Fatalf("human exit code = %d (stderr: %q)", code, stderr)
	}
	for _, line := range []string{
		"odd/tasks/a.md — 3/5 tasks — " + stamp,
		"odd/tasks/b.md — 0/2 tasks — " + stamp,
	} {
		if !strings.Contains(stdout, line) {
			t.Errorf("human output missing %q:\n%s", line, stdout)
		}
	}
}

// TestOddStatusEmptyAndIsolation covers the array's always-present shape and
// its isolation from routing: `odd: []` (never null or omitted) with no
// synthetic directories; a change-less workspace stays `active: []` while
// listing its document; a malformed document (directory named *.md) is
// skipped and a proposal-done change keeps nextRecommended=spec with empty
// blockedReasons; pure-SDD human output with no odd/tasks gains no section.
func TestOddStatusEmptyAndIsolation(t *testing.T) {
	empty := seedOddWorkspace(t)
	raw, _, active := runOddStatusJSON(t, empty)
	if got := strings.TrimSpace(string(raw)); got != "[]" {
		t.Errorf("empty workspace odd = %s, want []", got)
	}
	if len(active) != 0 {
		t.Errorf("empty workspace active = %#v, want empty", active)
	}
	if _, err := os.Stat(filepath.Join(empty, "odd")); !os.IsNotExist(err) {
		t.Errorf("status run created odd/ (stat err = %v)", err)
	}
	assertNoSyntheticChanges(t, empty)
	code, stdout, stderr := runSDDStatusCLIArgs(t, "--cwd", empty)
	if code != 0 {
		t.Fatalf("pure-SDD human exit code = %d (stderr: %q)", code, stderr)
	}
	if strings.Contains(stdout, "ODD tasks") || strings.Contains(stdout, "odd/tasks") {
		t.Errorf("human output grew an ODD section for an empty array:\n%s", stdout)
	}

	organic := seedOddWorkspace(t)
	mod := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	stamp := writeOddDoc(t, organic, "organic.md", "- [x] only\n", mod)
	raw, entries, active := runOddStatusJSON(t, organic)
	if len(active) != 0 {
		t.Errorf("change-less active = %#v, want empty", active)
	}
	if len(entries) != 1 || entries[0].Path != "odd/tasks/organic.md" || entries[0].LastTouched != stamp {
		t.Fatalf("change-less odd = %s, want the organic document", raw)
	}
	assertNoSyntheticChanges(t, organic)

	malformed := seedOddWorkspace(t)
	changeRoot := filepath.Join(malformed, "openspec", "changes", "proposal-only")
	if err := os.MkdirAll(changeRoot, 0o755); err != nil {
		t.Fatalf("mkdir change: %v", err)
	}
	if err := os.WriteFile(filepath.Join(changeRoot, "proposal.md"), []byte("# Proposal\n"), 0o644); err != nil {
		t.Fatalf("write proposal: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(malformed, "odd", "tasks", "broken.md"), 0o755); err != nil {
		t.Fatalf("mkdir broken.md: %v", err)
	}
	raw, _, active = runOddStatusJSON(t, malformed)
	if len(active) != 1 {
		t.Fatalf("malformed workspace active = %d, want 1", len(active))
	}
	if active[0].NextRecommended != "spec" || len(active[0].BlockedReasons) != 0 {
		t.Errorf("proposal-done change = %#v, want nextRecommended spec with empty blockedReasons", active[0])
	}
	if got := strings.TrimSpace(string(raw)); got != "[]" {
		t.Errorf("malformed workspace odd = %s, want [] (skipped)", got)
	}
}
