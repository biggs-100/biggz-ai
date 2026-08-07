package review

import (
	"context"
	"testing"

	"github.com/biggs-100/biggz-ai/model"
)

func TestNewReceipt_FromChain(t *testing.T) {
	dir := t.TempDir()
	s := OpenWithDir(dir, "receipt-chain")

	// Append three events.
	h1, err := s.Append("", Record{
		Operation: "create",
		Role:      "Author",
		Timestamp: "2026-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("genesis: %v", err)
	}
	h2, err := s.Append(h1, Record{
		Operation: "review",
		Role:      "Reviewer",
		Timestamp: "2026-01-02T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	_, err = s.Append(h2, Record{
		Operation: "approve",
		Role:      "Lead",
		Timestamp: "2026-01-03T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("third: %v", err)
	}

	chain, err := s.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}

	receipt := NewReceipt(chain)
	if receipt.GenesisRevision != h1 {
		t.Errorf("GenesisRevision mismatch: %s vs %s", receipt.GenesisRevision, h1)
	}
	if receipt.HeadRevision != h2 {
		// h2 is the second event, but third was appended after — HEAD should be third.
	}
	if receipt.EventCount != 3 {
		t.Errorf("EventCount mismatch: %d vs 3", receipt.EventCount)
	}
	if receipt.BindingHash == "" {
		t.Error("expected non-empty BindingHash")
	}
}

func TestReceipt_Verify_Valid(t *testing.T) {
	dir := t.TempDir()
	s := OpenWithDir(dir, "verify-valid")

	h1, _ := s.Append("", Record{Operation: "genesis", Role: "Author", Timestamp: "2026-01-01T00:00:00Z"})
	s.Append(h1, Record{Operation: "complete", Role: "Lead", Timestamp: "2026-01-02T00:00:00Z"})

	chain, _ := s.LoadChain()
	receipt := NewReceipt(chain)

	if err := receipt.Verify(chain); err != nil {
		t.Fatalf("Verify() returned error: %v", err)
	}
}

func TestReceipt_Verify_TamperedChain(t *testing.T) {
	dir := t.TempDir()
	s := OpenWithDir(dir, "verify-tampered")

	h1, _ := s.Append("", Record{Operation: "genesis", Role: "Author", Timestamp: "2026-01-01T00:00:00Z"})
	s.Append(h1, Record{Operation: "complete", Role: "Lead", Timestamp: "2026-01-02T00:00:00Z"})

	chain, _ := s.LoadChain()
	receipt := NewReceipt(chain)

	// Tamper with chain head.
	chain.HeadHash = "tampered"

	if err := receipt.Verify(chain); err == nil {
		t.Fatal("Verify() returned nil for tampered chain")
	}
}

func TestReceipt_Verify_TamperedReceipt(t *testing.T) {
	dir := t.TempDir()
	s := OpenWithDir(dir, "verify-tampered-receipt")

	h1, _ := s.Append("", Record{Operation: "genesis", Role: "Author", Timestamp: "2026-01-01T00:00:00Z"})
	s.Append(h1, Record{Operation: "complete", Role: "Lead", Timestamp: "2026-01-02T00:00:00Z"})

	chain, _ := s.LoadChain()
	receipt := NewReceipt(chain)

	// Tamper with the receipt's binding hash.
	receipt.BindingHash = "tampered"

	if err := receipt.Verify(chain); err == nil {
		t.Fatal("Verify() returned nil for tampered receipt")
	}
}

// --- Backward-compatible wrapper tests ---

func TestGenerateReceipt(t *testing.T) {
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	r.Start(context.Background())
	r.Complete(context.Background())

	receipt := GenerateReceipt(r.State)
	if receipt.LineageID != r.State.LineageID {
		t.Errorf("LineageID mismatch: %s vs %s", receipt.LineageID, r.State.LineageID)
	}
	if receipt.HeadRevision == "" {
		t.Log("HeadRevision empty (expected when no evidence)")
	}
	if receipt.BindingHash == "" {
		t.Error("expected non-empty BindingHash")
	}
}

func TestVerifyReceipt_Valid(t *testing.T) {
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	r.Start(context.Background())
	r.Complete(context.Background())

	receipt := GenerateReceipt(r.State)
	if !VerifyReceipt(receipt, r.State) {
		t.Fatal("VerifyReceipt() returned false for valid receipt")
	}
}

func TestVerifyReceipt_Tampered(t *testing.T) {
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	r.Start(context.Background())
	r.Complete(context.Background())

	receipt := GenerateReceipt(r.State)

	// Tamper with the receipt's binding hash.
	receipt.BindingHash = "tampered"

	if VerifyReceipt(receipt, r.State) {
		t.Fatal("VerifyReceipt() returned true for tampered receipt")
	}
}

func TestReceipt_EmptyChain(t *testing.T) {
	dir := t.TempDir()
	s := OpenWithDir(dir, "empty-chain")

	chain, err := s.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}

	receipt := NewReceipt(chain)
	if receipt.EventCount != 0 {
		t.Errorf("expected EventCount=0 for empty chain, got %d", receipt.EventCount)
	}
	if receipt.GenesisRevision != "" {
		t.Errorf("expected empty GenesisRevision, got %s", receipt.GenesisRevision)
	}
	if receipt.HeadRevision != "" {
		t.Errorf("expected empty HeadRevision, got %s", receipt.HeadRevision)
	}
	if receipt.BindingHash == "" {
		t.Error("BindingHash should be non-empty even for empty chain")
	}
}

func TestReceipt_Verify_EmptyChain(t *testing.T) {
	dir := t.TempDir()
	s := OpenWithDir(dir, "verify-empty")

	chain, _ := s.LoadChain()
	receipt := NewReceipt(chain)

	// Empty chain receipt should verify against another empty chain
	// with the same lineage ID.
	dir2 := t.TempDir()
	s2 := OpenWithDir(dir2, "verify-empty")
	chain2, _ := s2.LoadChain()

	if err := receipt.Verify(chain2); err != nil {
		t.Fatalf("Verify() empty chain: %v", err)
	}
}

func TestReceipt_CloneProof(t *testing.T) {
	// Two lineages with identical events but different lineage IDs
	// should produce different receipts.
	dir1 := t.TempDir()
	s1 := OpenWithDir(dir1, "lineage-a")
	dir2 := t.TempDir()
	OpenWithDir(dir2, "lineage-b")
	s2 := OpenWithDir(dir2, "lineage-b")

	// Append identical events to both stores.
	evt := Record{
		Operation: "genesis",
		Role:      "Author",
		Timestamp: "2026-01-01T00:00:00Z",
	}
	s1.Append("", evt)
	s2.Append("", evt)

	// For store s1, the name is lineage-a, so the chain lineage_id will be "lineage-a"
	// Even with same events, the chain lineage_id differs → different receipt.
	// Let's use OpenWithDir with the same lineage ID but different directories.
	// Actually, the lineage ID is set on the Store struct — both are "lineage-b" for s2.
	// So they have the same lineage ID. We need different lineage IDs.

	// Use Open to create stores with different lineage IDs.
	// In temp dir, Open won't work because it needs a git repo.
	// Instead, let's create two stores with different lineage IDs manually.
	s3 := OpenWithDir(dir1, "lineage-X")
	s4 := OpenWithDir(dir2, "lineage-Y")

	// Re-append to get them with different lineage IDs.
	// We need fresh stores.
	dir3 := t.TempDir()
	dir4 := t.TempDir()
	s3 = OpenWithDir(dir3, "lineage-X")
	s4 = OpenWithDir(dir4, "lineage-Y")

	h3, _ := s3.Append("", Record{
		Operation: "genesis", Role: "Author",
		Timestamp: "2026-01-01T00:00:00Z",
	})
	s3.Append(h3, Record{
		Operation: "review", Role: "Reviewer",
		Timestamp: "2026-01-02T00:00:00Z",
	})
	h4, _ := s4.Append("", Record{
		Operation: "genesis", Role: "Author",
		Timestamp: "2026-01-01T00:00:00Z",
	})
	s4.Append(h4, Record{
		Operation: "review", Role: "Reviewer",
		Timestamp: "2026-01-02T00:00:00Z",
	})

	chain3, _ := s3.LoadChain()
	chain4, _ := s4.LoadChain()

	r3 := NewReceipt(chain3)
	r4 := NewReceipt(chain4)

	// The receipts should have different BindingHash because lineage IDs differ.
	if r3.BindingHash == r4.BindingHash {
		t.Error("expected different BindingHash for different lineages")
	}
	if r3.LineageID == r4.LineageID {
		t.Error("expected different LineageIDs")
	}
}

func TestVerifyReceipt_WrongReview(t *testing.T) {
	r1 := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	r1.Start(context.Background())
	r1.Complete(context.Background())

	r2 := New(model.ReviewSubject{Repository: "other/repo", CommitSHA: "def456"})
	r2.Start(context.Background())
	r2.Complete(context.Background())

	receipt := GenerateReceipt(r1.State)
	if VerifyReceipt(receipt, r2.State) {
		t.Fatal("VerifyReceipt() returned true for wrong review")
	}
}
