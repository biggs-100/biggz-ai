package review

import (
	"context"
	"testing"

	"github.com/biggz-ai/biggz/model"
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
