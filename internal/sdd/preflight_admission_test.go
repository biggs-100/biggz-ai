package sdd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// REQ-DG-3: HasExplicitPreflight distinguishes explicit prefs (cache hit or
// disk read success) from silent defaults. ResolvePreflightPrefs behavior
// is unchanged (still returns usable defaults).

func TestHasExplicitPreflight_ReqDG3_DefaultsOnlyFalse(t *testing.T) {
	cwd := "dg3-cwd-defaults-only"
	home := t.TempDir() // empty: no preflight file on disk
	ClearPreflightPrefs(cwd)
	if HasExplicitPreflight(cwd, home) {
		t.Fatal("HasExplicitPreflight should be false with only silent defaults (no cache, no disk file)")
	}
	prefs := ResolvePreflightPrefs(cwd, home)
	if prefs.ExecutionMode == "" || prefs.ArtifactStore == "" {
		t.Fatalf("ResolvePreflightPrefs silent defaults must stay usable, got %+v", prefs)
	}
}

func TestHasExplicitPreflight_ReqDG3_CacheAdmits(t *testing.T) {
	cwd := "dg3-cwd-cache"
	home := t.TempDir()
	ClearPreflightPrefs(cwd)
	defer ClearPreflightPrefs(cwd)
	SetPreflightPrefs(cwd, PreflightPrefs{ExecutionMode: "auto", ArtifactStore: "openspec", ChainedPrStrategy: "single-pr", ReviewBudgetLines: 400})
	if !HasExplicitPreflight(cwd, home) {
		t.Fatal("HasExplicitPreflight should be true on cache hit")
	}
}

func TestHasExplicitPreflight_ReqDG3_DiskAdmits(t *testing.T) {
	cwd := "dg3-cwd-disk"
	home := t.TempDir()
	ClearPreflightPrefs(cwd)
	defer ClearPreflightPrefs(cwd)
	if err := WriteSddPreflightToDisk(PreflightPrefs{ExecutionMode: "interactive", ArtifactStore: "openspec", ChainedPrStrategy: "stacked-to-main", ReviewBudgetLines: 400}, home); err != nil {
		t.Fatalf("WriteSddPreflightToDisk() error: %v", err)
	}
	if !HasExplicitPreflight(cwd, home) {
		t.Fatal("HasExplicitPreflight should be true on disk read success")
	}
}

func TestCheckPhaseEntryPreflight_ReqDG3(t *testing.T) {
	cwd := "dg3-cwd-admission"
	home := t.TempDir()
	ClearPreflightPrefs(cwd)
	defer ClearPreflightPrefs(cwd)

	blocked, reason, next := CheckPhaseEntryPreflight(cwd, home)
	if !blocked || reason != "blocked(preflight_missing)" || next != "resolve-blockers" {
		t.Fatalf("without explicit preflight: got blocked=%v reason=%q next=%q, want true blocked(preflight_missing) resolve-blockers", blocked, reason, next)
	}

	SetPreflightPrefs(cwd, PreflightPrefs{ExecutionMode: "interactive", ArtifactStore: "openspec", ChainedPrStrategy: "stacked-to-main", ReviewBudgetLines: 400})
	blocked, reason, next = CheckPhaseEntryPreflight(cwd, home)
	if blocked || reason != "" || next != "" {
		t.Fatalf("with explicit preflight: got blocked=%v reason=%q next=%q, want admitted", blocked, reason, next)
	}
}

// REQ-DG-3 newly-blocked-flows row: status reads without preflight still
// succeed; SDD phase entry without preflight blocks with
// blocked(preflight_missing) + resolve-blockers and launches nothing.

func TestPhaseEntryPreflight_ReqDG3_ReadVsEntry(t *testing.T) {
	root := t.TempDir()
	openspecRoot := filepath.Join(root, "openspec")
	changeDir := filepath.Join(openspecRoot, "changes", "dg3-change")
	if err := os.MkdirAll(changeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# P"), 0644); err != nil {
		t.Fatal(err)
	}

	cwd := "dg3-cwd-read-vs-entry"
	home := t.TempDir()
	ClearPreflightPrefs(cwd)
	defer ClearPreflightPrefs(cwd)

	// Status read without preflight succeeds (reads unaffected).
	active, _, err := StatusWithOptions(openspecRoot, StatusOptions{})
	if err != nil {
		t.Fatalf("StatusWithOptions() without preflight must succeed, got error: %v", err)
	}
	if len(active) != 1 || active[0].Name != "dg3-change" {
		t.Fatalf("StatusWithOptions() must still report the change, got %+v", active)
	}

	// Phase entry without preflight blocks: no launch, resolve-blockers.
	if _, err := NextPhaseChecked(openspecRoot, "dg3-change", cwd, home); err == nil {
		t.Fatal("NextPhaseChecked() without preflight must block, got nil error")
	} else if !strings.Contains(err.Error(), "blocked(preflight_missing)") {
		t.Fatalf("blocked phase entry must report blocked(preflight_missing), got: %v", err)
	} else if !strings.Contains(err.Error(), "resolve-blockers") {
		t.Fatalf("blocked phase entry must recommend resolve-blockers, got: %v", err)
	}

	// Explicit preflight admits dispatch to normal phase routing.
	SetPreflightPrefs(cwd, PreflightPrefs{ExecutionMode: "interactive", ArtifactStore: "openspec", ChainedPrStrategy: "stacked-to-main", ReviewBudgetLines: 400})
	phase, err := NextPhaseChecked(openspecRoot, "dg3-change", cwd, home)
	if err != nil {
		t.Fatalf("NextPhaseChecked() with explicit preflight must admit, got error: %v", err)
	}
	if phase != "spec" {
		t.Errorf("expected normal routing to spec, got %q", phase)
	}
}
