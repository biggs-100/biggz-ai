package review

import (
	"testing"
)

func TestSnapshotRecord(t *testing.T) {
	sm := NewSnapshotManager()
	s := sm.Record("review-1", "base123", "cand456", []string{"main.go", "test.go"}, 50)

	if s.ReviewID != "review-1" {
		t.Errorf("ReviewID = %q, want %q", s.ReviewID, "review-1")
	}
	if len(s.Paths) != 2 {
		t.Errorf("expected 2 paths, got %d", len(s.Paths))
	}
	if s.ChangedLines != 50 {
		t.Errorf("ChangedLines = %d, want 50", s.ChangedLines)
	}
}

func TestSnapshotLatest(t *testing.T) {
	sm := NewSnapshotManager()
	if sm.Latest() != nil {
		t.Error("expected nil for empty manager")
	}

	sm.Record("r1", "b1", "c1", nil, 10)
	sm.Record("r1", "b2", "c2", nil, 20)

	latest := sm.Latest()
	if latest == nil {
		t.Fatal("expected non-nil latest")
	}
	if latest.ChangedLines != 20 {
		t.Errorf("expected 20 changed lines, got %d", latest.ChangedLines)
	}
}

func TestSnapshotAll(t *testing.T) {
	sm := NewSnapshotManager()
	all := sm.All()
	if len(all) != 0 {
		t.Error("expected empty")
	}

	sm.Record("r1", "b1", "c1", nil, 10)
	all = sm.All()
	if len(all) != 1 {
		t.Errorf("expected 1, got %d", len(all))
	}
}
