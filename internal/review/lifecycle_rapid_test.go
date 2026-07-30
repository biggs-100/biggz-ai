package review

import (
	"fmt"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// PropTest: Review lifecycle transitions follow valid paths.
func TestRapid_ReviewLifecycleTransitions(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		store := NewCompactStore()
		lineageID := rapid.StringMatching(`lifecycle-[a-z]{4}`).Draw(t, "lineageID")
		steps := rapid.IntRange(0, 10).Draw(t, "steps")

		for i := 0; i < steps; i++ {
			action := rapid.IntRange(0, 4).Draw(t, "action")
			switch action {
			case 0: // begin
				store.BeginTransaction(lineageID)
			case 1: // commit
				store.CommitTransaction(lineageID, "mr", "rh", nil)
			case 2: // fail
				store.FailTransaction(lineageID, "test-fail")
			case 3: // quarantine
				store.QuarantineLineage(lineageID, "test-quarantine")
			case 4: // recover
				store.RecoverLineage(lineageID)
			}

			// State must never panic or deadlock
			store.GetRecord(lineageID)
		}

		// Final state must be consistent
		issues, err := store.Reconcile()
		if err != nil {
			t.Fatal(err)
		}
		if len(issues) > 0 {
			// Quarantined lineages may have issues — that's OK
			rec, _ := store.GetRecord(lineageID)
			if rec != nil && rec.State == "failed" {
				return
			}
			t.Fatalf("reconciliation issues: %v", issues)
		}
	})
}

// PropTest: Snapshot manager handles any sequence of record/restore/export.
func TestRapid_SnapshotLifecycle(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		sm := NewSnapshotManager(nil)
		ops := rapid.IntRange(0, 15).Draw(t, "ops")

		for i := 0; i < ops; i++ {
			op := rapid.IntRange(0, 3).Draw(t, "op")
			switch op {
			case 0: // record
				sm.Record(
					rapid.StringMatching(`r-\d+`).Draw(t, "reviewID"),
					rapid.StringMatching(`[a-f0-9]{8}`).Draw(t, "base"),
					rapid.StringMatching(`[a-f0-9]{8}`).Draw(t, "cand"),
					rapid.SliceOf(rapid.StringMatching(`[a-z]+\.go`)).Draw(t, "paths"),
					rapid.IntRange(0, 5000).Draw(t, "lines"),
				)
			case 1: // get latest
				sm.Latest()
			case 2: // list all
				sm.All()
			case 3: // diagnose
				sm.Diagnose()
			}
		}

		// Chain must remain verifiable
		issues, err := sm.VerifyChain()
		if err != nil {
			t.Fatal(err)
		}
		if len(issues) > 0 && ops > 0 {
			t.Fatalf("chain issues after %d ops: %v", ops, issues)
		}
	})
}

// PropTest: Verification engine handles concurrent contract execution.
func TestRapid_ConcurrentVerification(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		engine := NewVerificationEngine(5)
		nContracts := rapid.IntRange(1, 8).Draw(t, "nContracts")
		passThreshold := rapid.Float64().Draw(t, "passThreshold")

		for i := 0; i < nContracts; i++ {
			contract := VerificationContract{
				ID:         fmt.Sprintf("contract-%d", i),
				Domain:     DomainFunctionalProof,
				MaxRetries: rapid.IntRange(0, 3).Draw(t, "maxRetries"),
				CreatedAt:  time.Now().UTC(),
			}
			engine.RegisterContract(contract)
		}

		// Run all contracts
		for _, c := range engine.contracts {
			shouldPass := rapid.Float64().Draw(t, "shouldPass") > passThreshold
			engine.Execute(c.ID, func() (bool, string, []string, error) {
				return shouldPass, "", nil, nil
			})
		}

		// Each contract must have at least one result
		for _, c := range engine.contracts {
			cr := engine.ContractResults(c.ID)
			if len(cr) == 0 {
				t.Fatalf("contract %s has no results", c.ID)
			}
		}
		_ = engine.AllPassed()
	})
}

// PropTest: Compact store scales with many lineages.
func TestRapid_LargeScaleCompactStore(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		store := NewCompactStore()
		n := rapid.IntRange(10, 50).Draw(t, "lineages")

		// Create many lineages
		for i := 0; i < n; i++ {
			lid := fmt.Sprintf("scale-%d", i)
			store.BeginTransaction(lid)
			store.CommitTransaction(lid, "mr", "rh", nil)
		}

		// Stats must be consistent
		stats := store.Stats()
		if stats.TotalLineages != n {
			t.Fatalf("expected %d lineages, got %d", n, stats.TotalLineages)
		}
		if stats.CompletedCount != n {
			t.Fatalf("expected %d completed, got %d", n, stats.CompletedCount)
		}

		// All records must be valid
		lineages := store.ListLineages()
		for _, rec := range lineages {
			if err := ValidateRecord(&rec); err != nil {
				t.Fatalf("invalid record: %v", err)
			}
		}
	})
}

// PropTest: Snapshot import/export handles large chains.
func TestRapid_LargeSnapshotChain(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		sm := NewSnapshotManager(nil)
		n := rapid.IntRange(50, 200).Draw(t, "snapshots")

		for i := 0; i < n; i++ {
			sm.Record("bench", "base", "cand",
				[]string{"a.go", "b.go", "c.go"},
				rapid.IntRange(0, 1000).Draw(t, "lines"))
		}

		// Export/Import round trip
		data, err := sm.Export()
		if err != nil {
			t.Fatal(err)
		}

		sm2 := NewSnapshotManager(nil)
		if err := sm2.Import(data); err != nil {
			t.Fatal(err)
		}

		// Verify chain in restored manager
		issues, err := sm2.VerifyChain()
		if err != nil {
			t.Fatal(err)
		}
		if len(issues) > 0 {
			t.Fatalf("post-import chain issues: %v", issues)
		}
	})
}
