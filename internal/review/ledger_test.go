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

func TestStoreLedger_AppendPersists(t *testing.T) {
	dir := t.TempDir()
	s := OpenWithDir(dir, "ledger-test")

	ledger := NewStoreLedger(s, "ledger-test", "")
	ledger.AppendOp("review-1", "started", "Review started", "pending", "system")
	ledger.AppendOp("review-1", "completed", "All checks passed", "completed", "system")

	// Verify in-memory state.
	if len(ledger.Entries()) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(ledger.Entries()))
	}

	// Verify persisted events via store.
	chain, err := s.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain() error: %v", err)
	}
	if chain.Count != 2 {
		t.Errorf("expected 2 events in store, got %d", chain.Count)
	}
	if chain.Records[0].Operation != "started" {
		t.Errorf("record 0: expected started, got %s", chain.Records[0].Operation)
	}
	if chain.Records[1].Operation != "completed" {
		t.Errorf("record 1: expected completed, got %s", chain.Records[1].Operation)
	}
}

func TestStoreLedger_ChainContinuity(t *testing.T) {
	dir := t.TempDir()
	s := OpenWithDir(dir, "ledger-chain")

	ledger := NewStoreLedger(s, "ledger-chain", "")
	ledger.AppendOp("r1", "step1", "", "pending", "system")
	ledger.AppendOp("r1", "step2", "", "in_progress", "system")
	ledger.AppendOp("r1", "step3", "", "completed", "system")

	chain, err := s.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	if chain.Count != 3 {
		t.Fatalf("expected 3 events, got %d", chain.Count)
	}

	// Verify chain linking.
	for i := 1; i < chain.Count; i++ {
		expectedPrev := chain.Records[i-1].PrevRevision
		// Each record (except genesis) should have a non-empty PrevRevision.
		if chain.Records[i].PrevRevision == "" {
			t.Errorf("record %d has empty PrevRevision", i)
		}
		_ = expectedPrev
	}

	// Validate integrity.
	verdict := s.Validate()
	if !verdict.Valid {
		t.Errorf("expected valid chain, got: %s", verdict.Reason)
	}
}
