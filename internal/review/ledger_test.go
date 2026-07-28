package review

import (
	"strings"
	"testing"
)

func TestLedgerAppend(t *testing.T) {
	l := NewLedger()
	l.AppendOp("review-1", "started", "Review started", "pending", "system")
	l.AppendOp("review-1", "completed", "All checks passed", "completed", "system")

	entries := l.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Operation != "started" {
		t.Errorf("expected started, got %s", entries[0].Operation)
	}
	if entries[1].Status != "completed" {
		t.Errorf("expected completed, got %s", entries[1].Status)
	}
}

func TestLedgerFilter(t *testing.T) {
	l := NewLedger()
	l.AppendOp("r1", "started", "", "pending", "system")
	l.AppendOp("r1", "lens_completed", "risk", "in_progress", "lens:risk")
	l.AppendOp("r1", "completed", "", "completed", "system")

	filtered := l.FilterByOperation("lens_completed")
	if len(filtered) != 1 {
		t.Errorf("expected 1 lens_completed, got %d", len(filtered))
	}
}

func TestLedgerSummary(t *testing.T) {
	l := NewLedger()
	summary := strings.TrimSpace(l.Summary())
	if summary != "No entries." {
		t.Errorf("expected 'No entries.', got %q", summary)
	}

	l.AppendOp("r1", "started", "", "pending", "system")
	l.AppendOp("r1", "completed", "", "completed", "system")
	summary = l.Summary()
	if len(summary) == 0 {
		t.Error("expected non-empty summary")
	}
}

func TestLedgerEmpty(t *testing.T) {
	l := NewLedger()
	if len(l.Entries()) != 0 {
		t.Error("expected 0 entries in new ledger")
	}
}
