package review

import (
	"context"
	"testing"

	"github.com/biggz-ai/biggz/model"
)

func TestGenerateReceipt(t *testing.T) {
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	r.Start(context.Background())
	r.Complete(context.Background())

	receipt := GenerateReceipt(r.State)
	if receipt.ReviewID != r.State.ID {
		t.Errorf("ReviewID mismatch: %s vs %s", receipt.ReviewID, r.State.ID)
	}
	if receipt.MerkleRoot == "" {
		t.Log("MerkleRoot empty (expected when no evidence)")
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

	// Tamper with the receipt
	receipt.MerkleRoot = "tampered"

	if VerifyReceipt(receipt, r.State) {
		t.Fatal("VerifyReceipt() returned true for tampered receipt")
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
