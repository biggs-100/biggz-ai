package model

import (
	"crypto/sha256"
	"fmt"
	"time"
)

// evidenceHash computes the SHA-256 hash of an evidence entry from its
// content fields. The input to the hash is the canonical string:
//
//	Position|Timestamp|Kind|Payload|PrevHash
func evidenceHash(e Evidence) string {
	input := fmt.Sprintf("%d|%s|%s|%s|%s",
		e.Position,
		e.Timestamp.Format(time.RFC3339Nano),
		e.Kind,
		e.Payload,
		e.PrevHash,
	)
	h := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", h)
}

// AppendEvidence appends a new evidence entry to the chain, computing
// its PrevHash from the previous tail entry and its own Hash from its
// content fields. It returns a new slice; the original is not modified.
//
// The Kind classifies the evidence (e.g. "lens_result", "policy_verdict",
// "provider_response"). Payload is an arbitrary JSON string.
func AppendEvidence(chain []Evidence, kind, payload string) []Evidence {
	n := len(chain)
	var prevHash string
	if n > 0 {
		prevHash = chain[n-1].Hash
	}

	entry := Evidence{
		Position:  n + 1,
		Timestamp: time.Now(),
		Kind:      kind,
		Payload:   payload,
		PrevHash:  prevHash,
	}
	entry.Hash = evidenceHash(entry)

	return append(chain, entry)
}

// MerkleRoot returns the Merkle root of the evidence chain.
//
// For a non-empty chain, the Merkle root is SHA-256 of the last entry's
// Hash. For an empty chain, it returns an empty string.
func MerkleRoot(chain []Evidence) string {
	n := len(chain)
	if n == 0 {
		return ""
	}
	h := sha256.Sum256([]byte(chain[n-1].Hash))
	return fmt.Sprintf("%x", h)
}
