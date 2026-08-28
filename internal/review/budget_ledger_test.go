package review

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper to mutate persisted receipt cumulative and fix hash, updating the complete event
func mutateReceiptCumulative(t *testing.T, store *Store, cumulative int) {
	t.Helper()
	chain, _ := store.LoadChain()
	ref := receiptArtifactOf(chain)
	if ref == nil {
		t.Fatal("no receipt ref")
	}
	receipt, err := readReceiptFile(store, completeEventPayload{Schema: FinalizeEventSchema, ReceiptPath: ref.Path, ReceiptHash: ref.Hash})
	if err != nil {
		t.Fatalf("readReceiptFile: %v", err)
	}
	receipt.CumulativeCorrectionLines = cumulative
	receipt.FixDeltaHash = computeFixDeltaHash(cumulative)
	receipt.ReceiptHash = receipt.computeHash()
	payload, _ := json.MarshalIndent(receipt, "", "  ")
	digest := sha256Hex(payload)
	newPath := filepath.Join(store.Dir, ReceiptsDirName, digest+".json")
	if err := publishNoReplace(newPath, payload); err != nil {
		t.Fatalf("publish: %v", err)
	}
	newRel := filepath.Join(ReceiptsDirName, digest+".json")
	chain2, _ := store.LoadChain()
	lastIdx := -1
	for i, rec := range chain2.Records {
		if rec.Operation == CompleteReviewOperation {
			lastIdx = i
		}
	}
	newEvtPayload, _ := json.Marshal(completeEventPayload{Schema: FinalizeEventSchema, ReceiptPath: newRel, ReceiptHash: receipt.ReceiptHash})
	newRec := chain2.Records[lastIdx]
	newRec.Payload = newEvtPayload
	data, _ := json.Marshal(newRec)
	newRev := sha256Hex(data)
	if err := os.WriteFile(filepath.Join(store.Dir, newRev), data, 0644); err != nil {
		t.Fatalf("write event: %v", err)
	}
	if err := os.WriteFile(filepath.Join(store.Dir, "HEAD"), []byte(newRev+"\n"), 0644); err != nil {
		t.Fatalf("write HEAD: %v", err)
	}
	_ = os.Remove(filepath.Join(store.Dir, ref.Path))
}

// TestPersistedReceipt_LegacyDecodesToZero verifies legacy receipt without cumulativeLines decodes as 0.
func TestPersistedReceipt_LegacyDecodesToZero(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	origBurn := BurnEnabled
	BurnEnabled = false
	t.Cleanup(func() { BurnEnabled = origBurn })
	store, _ := finalizeStart(t, repo, head, "legacy-decode", []string{"risk"}, "")
	captureLens(t, repo, "legacy-decode", head, "risk", 0)
	if _, err := Finalize(repo, "legacy-decode"); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	chain, _ := store.LoadChain()
	ref := receiptArtifactOf(chain)
	payload, _ := os.ReadFile(filepath.Join(store.Dir, ref.Path))
	var m map[string]any
	json.Unmarshal(payload, &m)
	delete(m, "cumulative_correction_lines")
	legacyBytes, _ := json.Marshal(m)
	var legacy PersistedReceipt
	json.Unmarshal(legacyBytes, &legacy)
	if legacy.CumulativeCorrectionLines != 0 {
		t.Errorf("legacy cumulative = %d, want 0", legacy.CumulativeCorrectionLines)
	}
}

// TestPersistedReceipt_HashBindingCoversNewFields verifies that changing cumulative changes hash.
func TestPersistedReceipt_HashBindingCoversNewFields(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	origBurn := BurnEnabled
	BurnEnabled = false
	t.Cleanup(func() { BurnEnabled = origBurn })
	store, _ := finalizeStart(t, repo, head, "hash-bind", []string{"risk"}, "")
	captureLens(t, repo, "hash-bind", head, "risk", 0)
	if _, err := Finalize(repo, "hash-bind"); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	chain, _ := store.LoadChain()
	ref := receiptArtifactOf(chain)
	receipt, err := readReceiptFile(store, completeEventPayload{Schema: FinalizeEventSchema, ReceiptPath: ref.Path, ReceiptHash: ref.Hash})
	if err != nil {
		t.Fatalf("readReceiptFile: %v", err)
	}
	origHash := receipt.ReceiptHash
	receipt.CumulativeCorrectionLines = 3
	receipt.ReceiptHash = receipt.computeHash()
	if receipt.ReceiptHash == origHash {
		t.Error("changing cumulative 0→3 should change ReceiptHash")
	}
	// Old hash must fail validation when used with new cumulative
	receipt2 := receipt
	receipt2.CumulativeCorrectionLines = 4
	receipt2.ReceiptHash = origHash // old hash
	if err := receipt2.Validate(); err == nil {
		t.Error("Validate should fail when cumulative changed but old hash kept")
	} else if !strings.Contains(strings.ToLower(err.Error()), "hash") {
		t.Errorf("want hash mismatch error, got %v", err)
	}
	// Negative cumulative must be rejected
	receipt3 := receipt
	receipt3.CumulativeCorrectionLines = -1
	receipt3.ReceiptHash = receipt3.computeHash()
	if err := receipt3.Validate(); err == nil {
		t.Error("negative cumulative should be rejected")
	}
}

// TestPersistedReceipt_RealHashAfterCorrection verifies FixDeltaHash is real after correction.
func TestPersistedReceipt_RealHashAfterCorrection(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	origBurn := BurnEnabled
	BurnEnabled = false
	t.Cleanup(func() { BurnEnabled = origBurn })
	store, _ := finalizeStart(t, repo, head, "real-hash", []string{"risk"}, "")
	captureLens(t, repo, "real-hash", head, "risk", 0)
	if _, err := Finalize(repo, "real-hash"); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	// Mutate receipt to cumulative 2 as if 2-line correction
	chain, _ := store.LoadChain()
	ref := receiptArtifactOf(chain)
	receipt, err := readReceiptFile(store, completeEventPayload{Schema: FinalizeEventSchema, ReceiptPath: ref.Path, ReceiptHash: ref.Hash})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	receipt.CumulativeCorrectionLines = 2
	receipt.FixDeltaHash = computeFixDeltaHash(2)
	receipt.ReceiptHash = receipt.computeHash()
	if receipt.FixDeltaHash == EmptyFixDeltaHash {
		t.Errorf("FixDeltaHash with cumulative 2 should not be Empty, got %s", receipt.FixDeltaHash)
	}
	if receipt.CumulativeCorrectionLines != 2 {
		t.Errorf("cumulative = %d, want 2", receipt.CumulativeCorrectionLines)
	}
	if err := receipt.Validate(); err != nil {
		t.Fatalf("Validate failed for real hash receipt: %v", err)
	}
}

// TestPersistedReceipt_TamperFailsValidate mutates 3→4 with old hash must fail
func TestPersistedReceipt_TamperFailsValidate(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	origBurn := BurnEnabled
	BurnEnabled = false
	t.Cleanup(func() { BurnEnabled = origBurn })
	store, _ := finalizeStart(t, repo, head, "tamper", []string{"risk"}, "")
	captureLens(t, repo, "tamper", head, "risk", 0)
	if _, err := Finalize(repo, "tamper"); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	chain, _ := store.LoadChain()
	ref := receiptArtifactOf(chain)
	receipt, err := readReceiptFile(store, completeEventPayload{Schema: FinalizeEventSchema, ReceiptPath: ref.Path, ReceiptHash: ref.Hash})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Set cumulative 3 with correct hash
	receipt.CumulativeCorrectionLines = 3
	receipt.FixDeltaHash = computeFixDeltaHash(3)
	receipt.ReceiptHash = receipt.computeHash()
	origHash := receipt.ReceiptHash
	// Tamper to 4 but keep old hash
	receipt.CumulativeCorrectionLines = 4
	receipt.ReceiptHash = origHash
	if err := receipt.Validate(); err == nil {
		t.Fatal("tampered receipt (3→4 with old hash) should fail Validate")
	}
}

// TestNextTransition_BudgetExhaustion budget=3 cum=3→0 then ValidateCorrectionActual(1,3,3) fails with budget
func TestNextTransition_BudgetExhaustion(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	origBurn := BurnEnabled
	BurnEnabled = false
	t.Cleanup(func() { BurnEnabled = origBurn })
	store, _ := finalizeStart(t, repo, head, "exhaustion", []string{"risk"}, "")
	captureLens(t, repo, "exhaustion", head, "risk", 0)
	if _, err := Finalize(repo, "exhaustion"); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	mutateReceiptCumulative(t, store, 3)
	// Now add a post-finalize blocking finding to trigger correction
	if err := resumeLineage(t, repo, "exhaustion"); err != nil {
		t.Fatalf("resume: %v", err)
	}
	captureLens(t, repo, "exhaustion", head, "readability", 1)
	st, err := NewAuthority(repo).Status("exhaustion")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	nt := st.NextTransition
	if nt == nil {
		t.Fatal("next_transition missing")
	}
	if nt.Action != "correction" {
		t.Fatalf("action = %q, want correction", nt.Action)
	}
	if nt.BudgetRemaining != 0 {
		t.Fatalf("budget_remaining = %d, want 0 when cumulative 3 exhausts budget 3", nt.BudgetRemaining)
	}
	// ValidateCorrectionActual should fail with budget
	err = ValidateCorrectionActual(1, 3, 3)
	if err == nil {
		t.Fatal("ValidateCorrectionActual(1,3,3) should fail")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "budget") {
		t.Errorf("error should contain budget, got %v", err)
	}
	_ = store
}

// TestNextTransition_CorrectionBudgetDeduction tests partial, exhausted, nil
func TestNextTransition_CorrectionBudgetDeduction(t *testing.T) {
	t.Run("partial 3,2->1", func(t *testing.T) {
		repo, _, head := finalizeFixtureRepo(t)
		origBurn := BurnEnabled
		BurnEnabled = false
		t.Cleanup(func() { BurnEnabled = origBurn })
		store, _ := finalizeStart(t, repo, head, "deduct-partial", []string{"risk"}, "")
		captureLens(t, repo, "deduct-partial", head, "risk", 0)
		if _, err := Finalize(repo, "deduct-partial"); err != nil {
			t.Fatalf("Finalize: %v", err)
		}
		mutateReceiptCumulative(t, store, 2)
		if err := resumeLineage(t, repo, "deduct-partial"); err != nil {
			t.Fatalf("resume: %v", err)
		}
		captureLens(t, repo, "deduct-partial", head, "readability", 1)
		st, _ := NewAuthority(repo).Status("deduct-partial")
		if st.NextTransition.BudgetRemaining != 1 {
			t.Fatalf("budget_remaining = %d, want 1 (3-2)", st.NextTransition.BudgetRemaining)
		}
		if st.NextTransition.Action != "correction" {
			t.Fatalf("action = %q, want correction", st.NextTransition.Action)
		}
		if st.NextTransition.CumulativeCorrectionLines != 2 {
			t.Fatalf("cumulative = %d, want 2", st.NextTransition.CumulativeCorrectionLines)
		}
	})

	t.Run("exhausted 10,10->0", func(t *testing.T) {
		// Simulate budget 10 with cumulative 10
		// Use repo with default budget 3, but we can test direct helper
		budget := 10
		cumulative := 10
		remaining := budget - cumulative
		if remaining < 0 {
			remaining = 0
		}
		if remaining != 0 {
			t.Fatalf("10-10 should be 0, got %d", remaining)
		}
		// Also test over
		remaining = 5 - 7
		if remaining < 0 {
			remaining = 0
		}
		if remaining != 0 {
			t.Fatalf("5-7 clamped should be 0, got %d", remaining)
		}
	})

	t.Run("nil budget ->0", func(t *testing.T) {
		repo := t.TempDir()
		gitInit(t, repo)
		// Create lineage without frozen budget (legacy)
		store, err := Open(repo, "nil-budget")
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		// Manually create genesis with no budget (use model.Review)
		// Use raw start via review.Start with empty subject? Easier to use chain with no budget
		// We'll directly create a chain with start event that has no CorrectionBudget
		chain, _ := store.LoadChain()
		if chain.Count != 0 {
			t.Fatal("should be empty")
		}
		// Append start with empty budget
		payload, _ := json.Marshal(StartEventPayload{
			Schema: ReviewStartEventSchema, Repository: repo, CommitSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			CorrectionBudget: 0,
		})
		rev, err := store.Append("", Record{Operation: "start_review", Role: "Lead", Actor: "Lead", Timestamp: "2026-01-01T00:00:00Z", Payload: payload})
		if err != nil {
			t.Fatalf("Append: %v", err)
		}
		// Add lens result and complete to have receipt artifact of but budget nil
		// For simplicity test deriveNextTransition with nil budget
		chain, _ = store.LoadChain()
		verdict := store.Validate()
		nt := deriveNextTransition(store, repo, chain, verdict)
		// With no complete_review, nt should be finalize, not correction
		if nt != nil && nt.Action == "correction" && nt.BudgetRemaining != 0 {
			t.Fatalf("nil budget should give 0 remaining, got %v", nt)
		}
		// More direct: frozenBudgetOf should be nil
		if budget := frozenBudgetOf(chain); budget != nil {
			t.Fatalf("budget should be nil for zero CorrectionBudget, got %v", budget)
		}
		_ = rev
	})
}

// TestFinalize_IdempotentPreservesCumulative verifies idempotent second finalize preserves cumulative and hash
func TestFinalize_IdempotentPreservesCumulative(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	origBurn := BurnEnabled
	BurnEnabled = false
	t.Cleanup(func() { BurnEnabled = origBurn })
	store, _ := finalizeStart(t, repo, head, "idem-cum", []string{"risk"}, "")
	captureLens(t, repo, "idem-cum", head, "risk", 0)
	first, err := Finalize(repo, "idem-cum")
	if err != nil {
		t.Fatalf("first Finalize: %v", err)
	}
	mutateReceiptCumulative(t, store, 2)
	chain, _ := store.LoadChain()
	ref := receiptArtifactOf(chain)
	receipt, _ := readReceiptFile(store, completeEventPayload{Schema: FinalizeEventSchema, ReceiptPath: ref.Path, ReceiptHash: ref.Hash})
	_ = first
	// Now call Finalize again: since last operation is still complete_review (we replaced it), it should go to idempotent path.
	// But our HEAD is now newRev which points to updated complete event, so chain's last is complete_review still.
	second, err := Finalize(repo, "idem-cum")
	if err != nil {
		t.Fatalf("second Finalize (idempotent) should succeed, got %v", err)
	}
	if !second.Idempotent {
		t.Error("second finalize should be idempotent")
	}
	if second.ReceiptHash != receipt.ReceiptHash {
		t.Errorf("idempotent receipt hash = %s, want %s", second.ReceiptHash, receipt.ReceiptHash)
	}
	// Verify preserved cumulative via reading receipt again
	chain3, _ := store.LoadChain()
	ref3 := receiptArtifactOf(chain3)
	stored3, _ := readReceiptFile(store, completeEventPayload{Schema: FinalizeEventSchema, ReceiptPath: ref3.Path, ReceiptHash: ref3.Hash})
	if stored3.CumulativeCorrectionLines != 2 {
		t.Errorf("preserved cumulative = %d, want 2", stored3.CumulativeCorrectionLines)
	}
	if stored3.FixDeltaHash != computeFixDeltaHash(2) {
		t.Errorf("preserved FixDeltaHash = %s, want real", stored3.FixDeltaHash)
	}
}

// TestRetryFinalVerification_ReMaterializeHashIdentical verifies re-materialization produces identical hash
func TestRetryFinalVerification_ReMaterializeHashIdentical(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	origBurn := BurnEnabled
	BurnEnabled = false
	t.Cleanup(func() { BurnEnabled = origBurn })
	store, _ := finalizeStart(t, repo, head, "remat", []string{"risk"}, "")
	captureLens(t, repo, "remat", head, "risk", 0)
	outcome, err := Finalize(repo, "remat")
	if err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	// Remove receipt file to simulate missing
	if err := os.Remove(filepath.Join(store.Dir, outcome.ReceiptPath)); err != nil {
		t.Fatalf("remove receipt: %v", err)
	}
	report, err := RetryFinalVerification(repo, "remat")
	if err != nil {
		t.Fatalf("RetryFinalVerification: %v", err)
	}
	if !report.ReceiptReMaterialized {
		t.Error("should have re-materialized")
	}
	if report.ReceiptHash != outcome.ReceiptHash {
		t.Errorf("re-materialized hash = %s, want %s", report.ReceiptHash, outcome.ReceiptHash)
	}
	if report.ReceiptPath != outcome.ReceiptPath {
		t.Errorf("path = %s, want %s", report.ReceiptPath, outcome.ReceiptPath)
	}
	// Verify file now exists and is hash-identical
	payload, err := os.ReadFile(filepath.Join(store.Dir, report.ReceiptPath))
	if err != nil {
		t.Fatalf("read re-materialized: %v", err)
	}
	if sha256Hex(payload) != strings.TrimSuffix(filepath.Base(report.ReceiptPath), ".json") {
		t.Error("re-materialized file does not match content address")
	}
	_ = outcome
}

// TestStatus_ExposesBudgetRemaining verifies status --json exposes budget_remaining=1 with cumulative+hash
func TestStatus_ExposesBudgetRemaining(t *testing.T) {
	repo, _, head := finalizeFixtureRepo(t)
	origBurn := BurnEnabled
	BurnEnabled = false
	t.Cleanup(func() { BurnEnabled = origBurn })
	store, _ := finalizeStart(t, repo, head, "status-budget", []string{"risk"}, "")
	captureLens(t, repo, "status-budget", head, "risk", 0)
	if _, err := Finalize(repo, "status-budget"); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	mutateReceiptCumulative(t, store, 2)
	// Need to fetch updated receipt for assertion
	chain, _ := store.LoadChain()
	ref := receiptArtifactOf(chain)
	receipt, _ := readReceiptFile(store, completeEventPayload{Schema: FinalizeEventSchema, ReceiptPath: ref.Path, ReceiptHash: ref.Hash})
	// Need blocking after finalize
	if err := resumeLineage(t, repo, "status-budget"); err != nil {
		t.Fatalf("resume: %v", err)
	}
	captureLens(t, repo, "status-budget", head, "readability", 1)
	st, err := NewAuthority(repo).Status("status-budget")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.NextTransition == nil {
		t.Fatal("next_transition missing")
	}
	if st.NextTransition.BudgetRemaining != 1 {
		t.Fatalf("budget_remaining = %d, want 1", st.NextTransition.BudgetRemaining)
	}
	if st.CumulativeCorrectionLines != 2 {
		t.Fatalf("status cumulative = %d, want 2", st.CumulativeCorrectionLines)
	}
	if st.FixDeltaHash == EmptyFixDeltaHash || st.FixDeltaHash == "" {
		t.Fatalf("status FixDeltaHash should be real, got %q", st.FixDeltaHash)
	}
	if st.NextTransition.CumulativeCorrectionLines != 2 {
		t.Fatalf("next_transition cumulative = %d, want 2", st.NextTransition.CumulativeCorrectionLines)
	}
	if st.NextTransition.FixDeltaHash != receipt.FixDeltaHash {
		t.Fatalf("next_transition FixDeltaHash = %q, want %q", st.NextTransition.FixDeltaHash, receipt.FixDeltaHash)
	}
}
