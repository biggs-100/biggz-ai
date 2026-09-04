package sdd

import (
	"testing"
)

func TestLaunchDedup_New(t *testing.T) {
	dedup := NewLaunchDedup()
	if dedup.Count() != 0 {
		t.Errorf("expected 0 launches, got %d", dedup.Count())
	}
}

func TestLaunchDedup_RecordAndCheck(t *testing.T) {
	dedup := NewLaunchDedup()
	
	// First launch should not be blocked
	if dedup.ShouldBlock("spec", "Create spec for feature X") {
		t.Error("first launch should not be blocked")
	}
	
	// Record it
	dedup.Record("spec", "Create spec for feature X")
	
	// Same task should now be blocked
	if !dedup.ShouldBlock("spec", "Create spec for feature X") {
		t.Error("same task should be blocked after recording")
	}
	
	// Different task should not be blocked
	if dedup.ShouldBlock("spec", "Create spec for feature Y") {
		t.Error("different task should not be blocked")
	}
	
	// Same task, different phase should not be blocked
	if dedup.ShouldBlock("design", "Create spec for feature X") {
		t.Error("same task, different phase should not be blocked")
	}
}

func TestLaunchDedup_DifferentPhases(t *testing.T) {
	dedup := NewLaunchDedup()
	
	dedup.Record("explore", "Explore codebase for feature X")
	dedup.Record("spec", "Create spec for feature X")
	
	if dedup.Count() != 2 {
		t.Errorf("expected 2 launches, got %d", dedup.Count())
	}
	
	phases := dedup.Phases()
	if len(phases) != 2 {
		t.Errorf("expected 2 phases, got %d", len(phases))
	}
}

func TestLaunchDedup_FingerprintStability(t *testing.T) {
	// Same content, different formatting should produce same fingerprint
	desc1 := "Create spec for feature X"
	desc2 := "  Create   spec   for   feature   X  "
	desc3 := "CREATE SPEC FOR FEATURE X"
	
	fp1 := ComputeFingerprint(desc1)
	fp2 := ComputeFingerprint(desc2)
	fp3 := ComputeFingerprint(desc3)
	
	if fp1 != fp2 {
		t.Errorf("fingerprints should match for same content: %q vs %q", fp1, fp2)
	}
	if fp1 != fp3 {
		t.Errorf("fingerprints should match for case-insensitive content: %q vs %q", fp1, fp3)
	}
}

func TestLaunchDedup_FingerprintDifferentiation(t *testing.T) {
	// Different content should produce different fingerprints
	desc1 := "Create spec for feature X"
	desc2 := "Create spec for feature Y"
	
	fp1 := ComputeFingerprint(desc1)
	fp2 := ComputeFingerprint(desc2)
	
	if fp1 == fp2 {
		t.Errorf("fingerprints should differ for different content: %q", fp1)
	}
}

func TestLaunchDedup_ArtifactExtraction(t *testing.T) {
	// Should extract artifact references
	desc := "Update spec.md and design.md for feature X"
	refs := extractArtifactRefs(normalizeTaskDescription(desc))
	
	if len(refs) != 2 {
		t.Errorf("expected 2 artifact refs, got %d: %v", len(refs), refs)
	}
}

func TestLaunchDedup_Reset(t *testing.T) {
	dedup := NewLaunchDedup()
	dedup.Record("spec", "task 1")
	dedup.Record("design", "task 2")
	
	if dedup.Count() != 2 {
		t.Errorf("expected 2 launches, got %d", dedup.Count())
	}
	
	dedup.Reset()
	
	if dedup.Count() != 0 {
		t.Errorf("expected 0 launches after reset, got %d", dedup.Count())
	}
}

func TestLaunchDedup_CheckAndRecord(t *testing.T) {
	dedup := NewLaunchDedup()
	
	// First call should record and not block
	result := CheckAndRecord(dedup, "spec", "Create spec for feature X")
	if result.Blocked {
		t.Error("first CheckAndRecord should not block")
	}
	if result.Phase != "spec" {
		t.Errorf("expected phase 'spec', got %q", result.Phase)
	}
	
	// Second call should block
	result = CheckAndRecord(dedup, "spec", "Create spec for feature X")
	if !result.Blocked {
		t.Error("second CheckAndRecord should block")
	}
	if result.Message == "" {
		t.Error("blocked result should have a message")
	}
}

func TestLaunchDedup_HasLaunch(t *testing.T) {
	dedup := NewLaunchDedup()
	dedup.Record("spec", "task 1")
	
	fp := ComputeFingerprint("task 1")
	if !dedup.HasLaunch("spec", fp) {
		t.Error("HasLaunch should return true for recorded launch")
	}
	
	if dedup.HasLaunch("spec", "nonexistent") {
		t.Error("HasLaunch should return false for unrecorded fingerprint")
	}
	
	if dedup.HasLaunch("design", fp) {
		t.Error("HasLaunch should return false for different phase")
	}
}
