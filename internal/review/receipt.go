package review

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/biggs-100/biggz-ai/model"
)

// Compact receipts retired (v2.5 last-event closure): no compact receipt
// file is persisted or consumed. Chain-bound Receipt below is for the
// non-compact event-store only; compact delivery is burn-based without
// receipt, tombstone, or mirror.
// ---------------------------------------------------------------------------
// Chain-bound Receipt (PR 2)
// ---------------------------------------------------------------------------
//
// A receipt binds a complete review chain by its genesis revision, head
// revision, event count, and lineage ID. The binding hash is:
//
//	SHA-256(genesis_revision || head_revision || event_count || lineage_id)
//
// Verifying a receipt replays the chain and recomputes the binding hash to
// ensure no event has been tampered with.

// Receipt binds a complete review chain by its genesis, head, event count,
// and lineage ID. The binding hash is SHA-256(genesis || head || count || lineage).
type Receipt struct {
	LineageID       string `json:"lineage_id"`
	GenesisRevision string `json:"genesis_revision"`
	HeadRevision    string `json:"head_revision"`
	EventCount      int    `json:"event_count"`
	BindingHash     string `json:"binding_hash"`
}

// NewReceipt creates a chain-bound receipt from a validated chain.
func NewReceipt(chain ValidatedChain) Receipt {
	r := Receipt{
		LineageID:       chain.LineageID,
		GenesisRevision: chain.GenesisHash,
		HeadRevision:    chain.HeadHash,
		EventCount:      chain.Count,
	}
	r.BindingHash = computeReceiptHash(r.GenesisRevision, r.HeadRevision, r.EventCount, r.LineageID)
	return r
}

// Verify checks that the receipt matches the given chain by recomputing
// the binding hash and comparing all fields.
func (r Receipt) Verify(chain ValidatedChain) error {
	if r.GenesisRevision != chain.GenesisHash {
		return fmt.Errorf("receipt genesis mismatch: expected %s, got %s",
			r.GenesisRevision, chain.GenesisHash)
	}
	if r.HeadRevision != chain.HeadHash {
		return fmt.Errorf("receipt head mismatch: expected %s, got %s",
			r.HeadRevision, chain.HeadHash)
	}
	if r.EventCount != chain.Count {
		return fmt.Errorf("receipt count mismatch: expected %d, got %d",
			r.EventCount, chain.Count)
	}
	expected := computeReceiptHash(chain.GenesisHash, chain.HeadHash, chain.Count, chain.LineageID)
	if r.BindingHash != expected {
		return fmt.Errorf("receipt binding hash mismatch: expected %s, got %s",
			expected, r.BindingHash)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Backward-compatible wrappers (used by gate.go — PR 3)
// ---------------------------------------------------------------------------

// GenerateReceipt creates a chain-bound receipt from a ReviewState.
// This is a compatibility adapter for code that currently uses the old
// receipt format. Prefer NewReceipt(chain) with a ValidatedChain.
func GenerateReceipt(state *model.ReviewState) *Receipt {
	r := &Receipt{
		LineageID:       state.LineageID,
		GenesisRevision: state.ID,
		HeadRevision:    state.MerkleRoot,
		EventCount:      len(state.Evidence),
	}
	r.BindingHash = computeReceiptHash(r.GenesisRevision, r.HeadRevision, r.EventCount, r.LineageID)
	return r
}

// VerifyReceipt checks that a receipt matches the given ReviewState.
// Deprecated: use Receipt.Verify() with a ValidatedChain.
func VerifyReceipt(r *Receipt, state *model.ReviewState) bool {
	if r == nil || state == nil {
		return false
	}
	if r.GenesisRevision != state.ID {
		return false
	}
	if r.HeadRevision != state.MerkleRoot {
		return false
	}
	if r.EventCount != len(state.Evidence) {
		return false
	}
	expected := computeReceiptHash(state.ID, state.MerkleRoot, len(state.Evidence), state.LineageID)
	return r.BindingHash == expected
}

// computeReceiptHash computes SHA-256(genesis || head || count || lineage).
func computeReceiptHash(genesis, head string, count int, lineage string) string {
	h := sha256.New()
	h.Write([]byte(genesis))
	h.Write([]byte(head))
	h.Write([]byte(strconv.Itoa(count)))
	h.Write([]byte(lineage))
	return hex.EncodeToString(h.Sum(nil))
}
