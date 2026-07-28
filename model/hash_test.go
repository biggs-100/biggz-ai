package model

import (
	"crypto/sha256"
	"fmt"
	"testing"
	"time"
)

// TestAppendEvidence_EmptyChain verifies that appending to an empty chain
// produces PrevHash="" and a non-empty Hash.
func TestAppendEvidence_EmptyChain(t *testing.T) {
	chain := AppendEvidence(nil, "test", `{"msg":"hello"}`)
	if len(chain) != 1 {
		t.Fatalf("expected chain length 1, got %d", len(chain))
	}
	if chain[0].PrevHash != "" {
		t.Errorf("first entry PrevHash should be empty, got %q", chain[0].PrevHash)
	}
	if chain[0].Hash == "" {
		t.Errorf("first entry Hash should not be empty")
	}
	if chain[0].Position != 1 {
		t.Errorf("first entry Position should be 1, got %d", chain[0].Position)
	}
}

// TestAppendEvidence_ChainLinks verifies that each entry's PrevHash matches
// the previous entry's Hash.
func TestAppendEvidence_ChainLinks(t *testing.T) {
	chain := AppendEvidence(nil, "a", `{"n":1}`)
	chain = AppendEvidence(chain, "b", `{"n":2}`)
	chain = AppendEvidence(chain, "c", `{"n":3}`)

	if len(chain) != 3 {
		t.Fatalf("expected chain length 3, got %d", len(chain))
	}

	// First entry has no previous hash.
	if chain[0].PrevHash != "" {
		t.Errorf("entry 0 PrevHash should be empty, got %q", chain[0].PrevHash)
	}

	// Entry 1's PrevHash must equal entry 0's Hash.
	if chain[1].PrevHash != chain[0].Hash {
		t.Errorf("entry 1 PrevHash (%q) != entry 0 Hash (%q)", chain[1].PrevHash, chain[0].Hash)
	}

	// Entry 2's PrevHash must equal entry 1's Hash.
	if chain[2].PrevHash != chain[1].Hash {
		t.Errorf("entry 2 PrevHash (%q) != entry 1 Hash (%q)", chain[2].PrevHash, chain[1].Hash)
	}
}

// TestHashUniqueness verifies that entries with different content produce
// different hashes.
func TestHashUniqueness(t *testing.T) {
	chain := AppendEvidence(nil, "kind_a", `{"x":1}`)
	chain = AppendEvidence(chain, "kind_b", `{"y":2}`)

	if chain[0].Hash == chain[1].Hash {
		t.Error("entries with different content should have different hashes")
	}
}

// TestAppendEvidence_OriginalUnmodified verifies that AppendEvidence
// does not mutate the original slice.
func TestAppendEvidence_OriginalUnmodified(t *testing.T) {
	original := AppendEvidence(nil, "a", `{"n":1}`)
	origHash := original[0].Hash
	origLen := len(original)

	_ = AppendEvidence(original, "b", `{"n":2}`)

	if len(original) != origLen {
		t.Error("original slice length was modified")
	}
	if original[0].Hash != origHash {
		t.Error("original entry Hash was modified")
	}
}

// TestMerkleRoot_Empty verifies that an empty chain produces "".
func TestMerkleRoot_Empty(t *testing.T) {
	root := MerkleRoot(nil)
	if root != "" {
		t.Errorf("expected empty MerkleRoot for nil chain, got %q", root)
	}

	root = MerkleRoot([]Evidence{})
	if root != "" {
		t.Errorf("expected empty MerkleRoot for empty chain, got %q", root)
	}
}

// TestMerkleRoot_NonEmpty verifies that MerkleRoot for a non-empty chain
// equals SHA-256 of the last entry's Hash.
func TestMerkleRoot_NonEmpty(t *testing.T) {
	chain := AppendEvidence(nil, "a", `{"n":1}`)
	chain = AppendEvidence(chain, "b", `{"n":2}`)
	chain = AppendEvidence(chain, "c", `{"n":3}`)

	expected := sha256Hex(chain[2].Hash)
	root := MerkleRoot(chain)
	if root != expected {
		t.Errorf("MerkleRoot mismatch:\n  expected: %q\n  got:      %q", expected, root)
	}
}

// TestTamperDetection verifies that modifying an entry's Payload after
// append produces an inconsistent chain — the stored hash no longer
// matches a recomputed hash of the modified fields.
func TestTamperDetection(t *testing.T) {
	chain := AppendEvidence(nil, "a", `{"n":1}`)
	chain = AppendEvidence(chain, "b", `{"n":2}`)
	chain = AppendEvidence(chain, "c", `{"n":3}`)

	originalMerkleRoot := MerkleRoot(chain)

	// Tamper with the middle entry's Payload.
	chain[1].Payload = `{"n":999}`

	// MerkleRoot from stored hashes hasn't changed (the stored hashes are
	// stale), but confirm the original value was captured.
	_ = originalMerkleRoot

	// The key test: recompute the hash of the tampered entry and verify
	// it no longer matches the stored Hash.
	recomputedHash := evidenceHash(chain[1])
	if recomputedHash == chain[1].Hash {
		t.Errorf("recomputed hash should differ from stored hash after tamper:\n  stored:  %q\n  recomputed: %q",
			chain[1].Hash, recomputedHash)
	}

	// PrevHash link is also broken: entry 2's PrevHash still points to
	// entry 1's OLD hash, which no longer matches entry 1's recomputed hash.
	recomputedEntry1 := evidenceHash(chain[1])
	if chain[2].PrevHash == recomputedEntry1 {
		t.Errorf("after tamper, entry 2 PrevHash should NOT equal entry 1 rehash:\n  entry2.PrevHash: %q\n  entry1 rehash:   %q",
			chain[2].PrevHash, recomputedEntry1)
	}
}

// TestMerkleRoot_ChangesAfterTamper verifies that if we rebuild the chain
// with a modified payload, the MerkleRoot is different from the original
// chain before modification. This simulates a full re-hash scenario.
func TestMerkleRoot_ChangesAfterTamper(t *testing.T) {
	chain := AppendEvidence(nil, "a", `{"n":1}`)
	chain = AppendEvidence(chain, "b", `{"n":2}`)

	originalRoot := MerkleRoot(chain)

	// Build a new chain with different payload for the second entry.
	chain2 := AppendEvidence(nil, "a", `{"n":1}`)
	chain2 = AppendEvidence(chain2, "b", `{"n":999}`) // tampered payload

	tamperedRoot := MerkleRoot(chain2)
	if tamperedRoot == originalRoot {
		t.Error("tampered chain should produce a different MerkleRoot")
	}
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}

// Ensure time is used (imported for compilation).
var _ = time.Now
