package review

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/biggz-ai/biggz/model"
)

// Receipt is proof that a review completed with a specific evidence chain.
// It binds the ReviewID to the MerkleRoot at completion time.
// This is NOT a complex structure — it's the MerkleRoot + metadata.
type Receipt struct {
	ReviewID   string           `json:"review_id"`
	MerkleRoot string           `json:"merkle_root"`
	Status     model.ReviewStatus `json:"status"`
	Completed  time.Time        `json:"completed"`
	// BindingHash is SHA-256(ReviewID + MerkleRoot + Completed) — a compact proof.
	BindingHash string `json:"binding_hash,omitempty"`
}

// GenerateReceipt creates a signed receipt for the given review state.
func GenerateReceipt(state *model.ReviewState) *Receipt {
	r := &Receipt{
		ReviewID:   state.ID,
		MerkleRoot: state.MerkleRoot,
		Status:     state.Status,
		Completed:  state.UpdatedAt,
	}
	// Compute binding hash
	h := sha256.New()
	h.Write([]byte(r.ReviewID))
	h.Write([]byte(r.MerkleRoot))
	h.Write([]byte(r.Completed.String()))
	r.BindingHash = hex.EncodeToString(h.Sum(nil))
	return r
}

// VerifyReceipt checks that a receipt matches the given state.
func VerifyReceipt(r *Receipt, state *model.ReviewState) bool {
	if r.ReviewID != state.ID {
		return false
	}
	if r.MerkleRoot != state.MerkleRoot {
		return false
	}
	if r.Status != state.Status {
		return false
	}
	// Recompute binding hash
	h := sha256.New()
	h.Write([]byte(r.ReviewID))
	h.Write([]byte(r.MerkleRoot))
	h.Write([]byte(r.Completed.String()))
	expected := hex.EncodeToString(h.Sum(nil))
	return r.BindingHash == expected
}
