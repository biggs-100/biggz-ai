package review

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBurn_PreventsReplay verifies that a second finalize on a burned lineage
// fails with ErrAlreadyBurned instead of returning the same receipt idempotently.
func TestBurn_PreventsReplay(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	_, _ = finalizeStart(t, repo, head, "burn-replay", []string{"risk"}, "")
	captureLens(t, repo, "burn-replay", head, "risk", 0)

	first, err := Finalize(repo, "burn-replay")
	if err != nil {
		t.Fatalf("first Finalize: %v", err)
	}
	if first.ReceiptPath == "" || first.ReceiptHash == "" {
		t.Fatalf("first finalize outcome incomplete: %+v", first)
	}

	_, err = Finalize(repo, "burn-replay")
	if err == nil {
		t.Fatal("expected second finalize to fail with burned error, got nil")
	}
	if !errors.Is(err, ErrAlreadyBurned) && !strings.Contains(strings.ToLower(err.Error()), "burned") {
		t.Fatalf("expected ErrAlreadyBurned or burned error, got: %v", err)
	}
}

// TestBurn_GateBecomesInformational verifies that after burn every gate
// becomes informational (non-deciding) with DeliveryBurned and does not block
// delivery via ordinary repository policy.
func TestBurn_GateBecomesInformational(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	_, _ = finalizeStart(t, repo, head, "burn-gate", []string{"risk"}, "")
	captureLens(t, repo, "burn-gate", head, "risk", 0)

	if _, err := Finalize(repo, "burn-gate"); err != nil {
		t.Fatalf("Finalize: %v", err)
	}

	for _, kind := range []GateKind{GatePostApply, GatePreCommit, GatePrePush, GatePrePR, GateRelease} {
		result, err := EvaluateGate(kind, repo, "burn-gate", GateOptions{})
		if err != nil {
			t.Fatalf("EvaluateGate %s: %v", kind, err)
		}
		if result.Delivery != DeliveryBurned {
			t.Errorf("gate %s: Delivery = %q, want %q", kind, result.Delivery, DeliveryBurned)
		}
		if result.Passed {
			t.Errorf("gate %s: Passed = true, want false (burned is informational, not a pass)", kind)
		}
		if !result.Allowed {
			t.Errorf("gate %s: Allowed = false, want true (burned gates are non-blocking via ordinary policy)", kind)
		}
		if !strings.Contains(strings.ToLower(result.Reason), "burned") {
			t.Errorf("gate %s: Reason = %q, want to mention burned", kind, result.Reason)
		}
	}
}

// TestBurn_ReceiptEphemeral verifies that after burn the receipt file is
// gone (ephemeral) and the burned marker exists.
func TestBurn_ReceiptEphemeral(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	store, _ := finalizeStart(t, repo, head, "burn-ephemeral", []string{"risk"}, "")
	captureLens(t, repo, "burn-ephemeral", head, "risk", 0)

	outcome, err := Finalize(repo, "burn-ephemeral")
	if err != nil {
		t.Fatalf("Finalize: %v", err)
	}

	// The outcome still carries the receipt path/hash, but the file must be gone.
	fullReceiptPath := filepath.Join(store.Dir, outcome.ReceiptPath)
	if _, err := os.Stat(fullReceiptPath); !os.IsNotExist(err) {
		t.Errorf("receipt file %q should be deleted after burn, stat err: %v", fullReceiptPath, err)
	}

	// The burned marker must exist.
	markerPath := filepath.Join(store.Dir, BurnedMarkerFile)
	if _, err := os.Stat(markerPath); err != nil {
		t.Fatalf("burned marker %q should exist after burn, got: %v", markerPath, err)
	}

	// The chain must contain a burn_review event and IsBurned must be true.
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	if !IsChainBurned(chain) {
		t.Error("IsChainBurned = false, want true after burn")
	}
	if !store.IsBurned() {
		t.Error("Store.IsBurned = false, want true after burn")
	}

	// Receipt file gone implies readReceiptFile fails, but the chain still
	// remembers the receipt reference via complete_review.
	foundComplete := false
	for _, rec := range chain.Records {
		if rec.Operation == CompleteReviewOperation {
			foundComplete = true
			break
		}
	}
	if !foundComplete {
		t.Error("chain should still contain complete_review event after burn")
	}
	foundBurn := false
	for _, rec := range chain.Records {
		if rec.Operation == BurnOperation {
			foundBurn = true
			break
		}
	}
	if !foundBurn {
		t.Error("chain should contain burn_review event after burn")
	}
}
