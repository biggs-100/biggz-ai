package review

import (
	"testing"
	"time"

	"pgregory.net/rapid"
)

// PropTest: Event hash chain integrity is never broken after any sequence
// of BeginTransaction, CommitTransaction, and FailTransaction.
func TestRapid_CompactStoreEventChainIntegrity(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		store := NewCompactStore()
		lineageID := rapid.StringMatching(`lineage-[a-z]{4}-\d{4}`).Draw(t, "lineageID")
		ops := rapid.SliceOf(rapid.IntRange(0, 2)).Draw(t, "operations")

		for _, op := range ops {
			switch op {
			case 0: // begin
				_, err := store.BeginTransaction(lineageID)
				if err != nil {
					// Budget exhausted — skip
					continue
				}
			case 1: // commit
				err := store.CommitTransaction(lineageID, "merkle-root", "receipt-hash", nil)
				if err != nil {
					// No active tx — skip
					continue
				}
			case 2: // fail
				err := store.FailTransaction(lineageID, "test-failure")
				if err != nil {
					continue
				}
			}
		}

		// Verify chain integrity for this lineage
		issues, err := store.Reconcile()
		if err != nil {
			t.Fatal(err)
		}
		if len(issues) > 0 {
			t.Fatalf("chain integrity broken: %v", issues)
		}
	})
}

// PropTest: Concurrent lineages do not interfere with each other's state.
func TestRapid_ConcurrentLineages(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		store := NewCompactStore()
		nLineages := rapid.IntRange(2, 10).Draw(t, "nLineages")
		opsPerLineage := rapid.IntRange(1, 8).Draw(t, "opsPerLineage")

		lineageIDs := make([]string, nLineages)
		for i := 0; i < nLineages; i++ {
			lineageIDs[i] = rapid.StringMatching(`lineage-\d{4}`).Draw(t, "lineageID")
		}

		// Interleave operations across lineages
		for step := 0; step < opsPerLineage; step++ {
			for _, lid := range lineageIDs {
				op := rapid.IntRange(0, 2).Draw(t, "op")
				switch op {
				case 0:
					store.BeginTransaction(lid)
				case 1:
					store.CommitTransaction(lid, "mr", "rh", nil)
				case 2:
					store.FailTransaction(lid, "concurrent-fail")
				}
			}
		}

		// Each lineage's terminal records must be valid; skip in-progress
		for _, lid := range lineageIDs {
			rec, ok := store.GetRecord(lid)
			if !ok {
				continue
			}
			if rec.State == "in_progress" || rec.State == "pending" {
				continue // non-terminal is fine during concurrent ops
			}
			if err := ValidateRecord(rec); err != nil {
				t.Fatalf("lineage %s: %v", lid, err)
			}
		}

		// Global reconciliation must pass
		issues, err := store.Reconcile()
		if err != nil {
			t.Fatal(err)
		}
		if len(issues) > 0 {
			t.Fatalf("global reconciliation failed: %v", issues)
		}
	})
}

// PropTest: Snapshot hash chain is consistent regardless of record order.
func TestRapid_SnapshotChain(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		sm := NewSnapshotManager(nil)
		n := rapid.IntRange(1, 20).Draw(t, "n")
		prevHash := ""

		for i := 0; i < n; i++ {
			reviewID := rapid.StringMatching(`review-\d+`).Draw(t, "reviewID")
			baseTree := rapid.StringMatching(`[a-f0-9]{8}`).Draw(t, "baseTree")
			candTree := rapid.StringMatching(`[a-f0-9]{8}`).Draw(t, "candTree")
			nPaths := rapid.IntRange(0, 5).Draw(t, "nPaths")
			paths := make([]string, nPaths)
			for j := 0; j < nPaths; j++ {
				paths[j] = rapid.StringMatching(`[a-z]+\.go`).Draw(t, "path")
			}
			changedLines := rapid.IntRange(0, 2000).Draw(t, "changedLines")

			s := sm.Record(reviewID, baseTree, candTree, paths, changedLines)

			// Verify hash chain: parent must match previous
			if s.ParentHash != prevHash {
				t.Fatalf("snapshot %d: parent hash mismatch: got %q, want %q", i, s.ParentHash, prevHash)
			}
			prevHash = s.Hash
		}

		// Full chain verification must pass
		issues, err := sm.VerifyChain()
		if err != nil {
			t.Fatal(err)
		}
		if len(issues) > 0 {
			t.Fatalf("chain issues: %v", issues)
		}
	})
}

// PropTest: Import/Export round-trip preserves snapshot chain integrity.
func TestRapid_SnapshotRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		sm := NewSnapshotManager(nil)
		n := rapid.IntRange(3, 15).Draw(t, "n")

		for i := 0; i < n; i++ {
			sm.Record(
				rapid.StringMatching(`r-\d+`).Draw(t, "reviewID"),
				rapid.StringMatching(`[a-f0-9]{8}`).Draw(t, "base"),
				rapid.StringMatching(`[a-f0-9]{8}`).Draw(t, "cand"),
				rapid.SliceOf(rapid.StringMatching(`[a-z]+\.go`)).Draw(t, "paths"),
				rapid.IntRange(0, 1000).Draw(t, "lines"),
			)
		}

		// Export
		data, err := sm.Export()
		if err != nil {
			t.Fatal(err)
		}

		// Import into fresh manager
		sm2 := NewSnapshotManager(nil)
		if err := sm2.Import(data); err != nil {
			t.Fatal(err)
		}

		// Verify chain in imported manager
		issues, err := sm2.VerifyChain()
		if err != nil {
			t.Fatal(err)
		}
		if len(issues) > 0 {
			t.Fatalf("imported chain issues: %v", issues)
		}

		// Same number of snapshots
		if len(sm.All()) != len(sm2.All()) {
			t.Fatalf("snapshot count mismatch: %d vs %d", len(sm.All()), len(sm2.All()))
		}
	})
}

// PropTest: Verification contract retries until pass or budget exhausted.
func TestRapid_VerificationRetry(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		engine := NewVerificationEngine(10)
		maxRetries := rapid.IntRange(1, 5).Draw(t, "maxRetries")

		contract := VerificationContract{
			ID:         "test-contract",
			Domain:     DomainFunctionalProof,
			MaxRetries: maxRetries,
			RetryDelay: 0,
			CreatedAt:  now(),
		}
		engine.RegisterContract(contract)

		shouldPassOn := rapid.IntRange(1, maxRetries+2).Draw(t, "passOnAttempt")
		attempts := 0

		result, err := engine.Execute("test-contract", func() (bool, string, []string, error) {
			attempts++
			if attempts >= shouldPassOn {
				return true, "passed", nil, nil
			}
			return false, "not yet", []string{"incomplete"}, nil
		})
		if err != nil {
			t.Fatal(err)
		}

		// If shouldPassOn <= maxRetries+1, must pass
		if shouldPassOn <= maxRetries+1 {
			if !result.Passed {
				t.Fatalf("expected pass on attempt %d, got fail after %d attempts", shouldPassOn, attempts)
			}
			if result.Attempt != shouldPassOn {
				t.Fatalf("expected pass on attempt %d, got %d", shouldPassOn, result.Attempt)
			}
		} else {
			if result.Passed {
				t.Fatalf("expected fail (passOn=%d > maxRetries+1=%d)", shouldPassOn, maxRetries+1)
			}
		}
	})
}

// PropTest: Quarantine prevents further operations on a lineage.
func TestRapid_QuarantineIsolation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		store := NewCompactStore()
		lineageID := rapid.StringMatching(`q-[a-z]{4}`).Draw(t, "lineageID")

		// Begin, then quarantine
		_, err := store.BeginTransaction(lineageID)
		if err != nil {
			t.Fatal(err)
		}
		err = store.QuarantineLineage(lineageID, "rapid-test")
		if err != nil {
			t.Fatal(err)
		}

		// All operations must fail after quarantine
		_, err = store.BeginTransaction(lineageID)
		if err == nil {
			t.Fatal("expected error on begin after quarantine")
		}
		err = store.CommitTransaction(lineageID, "mr", "rh", nil)
		if err == nil {
			t.Fatal("expected error on commit after quarantine")
		}

		// Recovery must succeed
		err = store.RecoverLineage(lineageID)
		if err != nil {
			t.Fatalf("recovery failed: %v", err)
		}
	})
}

func now() time.Time {
	return time.Now().UTC()
}
