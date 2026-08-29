package sdd

import "testing"

func TestShouldEnforceScopedSurfaces(t *testing.T) {
	if ShouldEnforceScopedSurfaces(3) {
		t.Fatal("3 files should not enforce")
	}
	if !ShouldEnforceScopedSurfaces(4) {
		t.Fatal("4 files should enforce")
	}
	if !ShouldEnforceScopedSurfaces(5) {
		t.Fatal("5 files should enforce")
	}
}

func TestValidateBoundedWriterSurfaces(t *testing.T) {
	if r := ValidateBoundedWriterSurfaces(map[string]any{"agent": "worker", "task": "x"}, 3); r != nil {
		t.Fatalf("3 files should allow, got %v", r)
	}
	if r := ValidateBoundedWriterSurfaces(map[string]any{"agent": "worker", "task": "x"}, 4); r == nil {
		t.Fatal("4 files without surfaces should block")
	}
	if r := ValidateBoundedWriterSurfaces(map[string]any{"agent": "worker", "task": "## Allowed edit surfaces\n- `internal/orchestrator/surfaces.go`\n"}, 4); r != nil {
		t.Fatalf("4 files with surfaces should allow, got %v", r)
	}
	if r := ValidateBoundedWriterSurfaces(map[string]any{"agent": "researcher", "task": "x"}, 4); r != nil {
		t.Fatalf("non-writer should not block even at 4 files, got %v", r)
	}
}
