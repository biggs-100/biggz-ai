package review

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/biggs-100/biggz-ai/model"
)

func TestAuthorityVerify_Valid(t *testing.T) {
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	r.Start(context.Background())
	r.Complete(context.Background())
	receipt := GenerateReceipt(r.State)

	av := &AuthorityVerifier{}
	result := av.Verify(receipt, r.State)
	if !result.Valid {
		t.Errorf("expected valid, got: %s", result.Reason)
	}
	if result.ReviewID != r.State.ID {
		t.Errorf("ReviewID mismatch")
	}
}

func TestAuthorityVerify_NilReceipt(t *testing.T) {
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	av := &AuthorityVerifier{}
	result := av.Verify(nil, r.State)
	if result.Valid {
		t.Fatal("expected invalid for nil receipt")
	}
}

func TestAuthorityVerify_NilState(t *testing.T) {
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	receipt := GenerateReceipt(r.State)
	av := &AuthorityVerifier{}
	result := av.Verify(receipt, nil)
	if result.Valid {
		t.Fatal("expected invalid for nil state")
	}
}

func TestAuthorityVerify_Tampered(t *testing.T) {
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	r.Start(context.Background())
	r.Complete(context.Background())
	receipt := GenerateReceipt(r.State)

	// Tamper with state
	r.State.MerkleRoot = "tampered"

	av := &AuthorityVerifier{}
	result := av.Verify(receipt, r.State)
	if result.Valid {
		t.Fatal("expected invalid for tampered state")
	}
}

func TestAuthorityInventory(t *testing.T) {
	repoDir := t.TempDir()
	gitInit(t, repoDir)

	a := NewAuthority(repoDir)

	// Should be empty initially.
	inv, err := a.Inventory()
	if err != nil {
		t.Fatalf("Inventory() error: %v", err)
	}
	if len(inv) != 0 {
		t.Errorf("expected empty inventory, got %d items", len(inv))
	}

	// Append events to create a lineage.
	store, err := a.Open("test-lineage")
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	_, err = store.Append("", Record{
		Operation: "start",
		Role:      "Author",
		Actor:     "user",
		Timestamp: "2026-01-01T00:00:00Z",
		Payload:   json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("Append() error: %v", err)
	}

	// Now inventory should list the lineage.
	inv, err = a.Inventory()
	if err != nil {
		t.Fatalf("Inventory() error: %v", err)
	}
	if len(inv) != 1 {
		t.Fatalf("expected 1 lineage, got %d", len(inv))
	}
	if inv[0].LineageID != "test-lineage" {
		t.Errorf("expected test-lineage, got %s", inv[0].LineageID)
	}
}

func TestAuthorityStatus(t *testing.T) {
	repoDir := t.TempDir()
	gitInit(t, repoDir)

	a := NewAuthority(repoDir)

	store, err := a.Open("status-lineage")
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	h1, err := store.Append("", Record{
		Operation: "create",
		Role:      "Author",
		Actor:     "user",
		Timestamp: "2026-01-01T00:00:00Z",
		Payload:   json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("first Append: %v", err)
	}

	h2, err := store.Append(h1, Record{
		Operation: "review",
		Role:      "Reviewer",
		Actor:     "reviewer",
		Timestamp: "2026-01-02T00:00:00Z",
		Payload:   json.RawMessage(`{"finding":"typo"}`),
	})
	if err != nil {
		t.Fatalf("second Append: %v", err)
	}

	st, err := a.Status("status-lineage")
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if st.LineageID != "status-lineage" {
		t.Errorf("expected status-lineage, got %s", st.LineageID)
	}
	if st.HeadHash != h2 {
		t.Errorf("head hash mismatch: %s vs %s", st.HeadHash, h2)
	}
	if st.EventCount != 2 {
		t.Errorf("expected 2 events, got %d", st.EventCount)
	}
	if !st.ChainValid {
		t.Error("expected valid chain")
	}
	if st.Receipt == nil {
		t.Error("expected non-nil receipt for valid chain")
	}
}
