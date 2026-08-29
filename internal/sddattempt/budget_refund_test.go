package sddattempt

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDualBudget(t *testing.T) {
	setStoreRoot(t)
	// Build cumulative 300 via one failed attempt with 300 lines.
	acq1, err := Acquire(AcquireParams{ChangeName: "ch-budget", RepoRoot: "r", RequestID: "req-budget-1", WorkUnit: "w", EvidenceGoal: "goal", MaxAttempts: 5, MaxLines: 400, ChangedLines: 300})
	if err != nil {
		t.Fatalf("Acquire 300: %v", err)
	}
	if _, err := Settle(SettleParams{ChangeName: "ch-budget", RepoRoot: "r", Token: acq1.Token, RequestID: "req-settle-1", Outcome: "failed", ChangedLines: 300, Diagnosis: "d"}); err != nil {
		t.Fatalf("Settle 300: %v", err)
	}
	store, _, err := loadStore("ch-budget", "r")
	if err != nil {
		t.Fatalf("loadStore: %v", err)
	}
	if store.CumulativeChangedLines != 300 {
		t.Fatalf("cumulative after 300 = %d want 300", store.CumulativeChangedLines)
	}
	// 300+150 >400 must be blocked
	_, err = Acquire(AcquireParams{ChangeName: "ch-budget", RepoRoot: "r", RequestID: "req-budget-2", WorkUnit: "w", EvidenceGoal: "goal", MaxAttempts: 5, MaxLines: 400, ChangedLines: 150})
	if err == nil {
		t.Fatal("Acquire 150 must be blocked")
	}
	var blocked *BlockedError
	if !errors.As(err, &blocked) || blocked.Reason != BlockedReasonBudgetExhausted {
		t.Fatalf("Acquire 150 blocked reason %v want budget_exhausted", err)
	}
	// 300+80=380 must succeed
	acq2, err := Acquire(AcquireParams{ChangeName: "ch-budget", RepoRoot: "r", RequestID: "req-budget-3", WorkUnit: "w", EvidenceGoal: "goal", MaxAttempts: 5, MaxLines: 400, ChangedLines: 80})
	if err != nil {
		t.Fatalf("Acquire 80: %v", err)
	}
	if _, err := Settle(SettleParams{ChangeName: "ch-budget", RepoRoot: "r", Token: acq2.Token, RequestID: "req-settle-2", Outcome: "failed", ChangedLines: 80, Diagnosis: "d"}); err != nil {
		t.Fatalf("Settle 80: %v", err)
	}
	store2, _, err := loadStore("ch-budget", "r")
	if err != nil {
		t.Fatalf("loadStore2: %v", err)
	}
	if store2.CumulativeChangedLines != 380 {
		t.Fatalf("cumulative after 80 = %d want 380", store2.CumulativeChangedLines)
	}
	// Single predicate ownership: verify helper is used (no duplicate inequality)
	// We rely on design that only runtimeChangedLineBudgetExceeded owns the check.
	if !runtimeChangedLineBudgetExceeded(store2, 30) {
		t.Fatal("predicate 380+30>400 must be true (380+30=410)")
	}
	if runtimeChangedLineBudgetExceeded(store2, 20) {
		t.Fatal("predicate 380+20>400 must be false")
	}
	// Also test Begin path for same budget
	_, err = Begin(BeginParams{ChangeName: "ch-budget", RepoRoot: "r", RequestID: "req-begin-budget", MaxAttempts: 5, MaxLines: 400, ChangedLines: 150, WorkUnit: "w"})
	if err == nil {
		t.Fatal("Begin 150 must be blocked")
	}
	if !errors.As(err, &blocked) || blocked.Reason != BlockedReasonBudgetExhausted {
		t.Fatalf("Begin 150 blocked reason %v want budget_exhausted", err)
	}
}

func TestRefund(t *testing.T) {
	setStoreRoot(t)
	// Test delivered increment helper directly
	if runtimeAttemptDeliveredIncrement(RuntimeAttempt{Outcome: "interrupted", ChangedLines: 20}) != 1 {
		t.Fatal("interrupted 20 must count as delivered (1)")
	}
	if runtimeAttemptDeliveredIncrement(RuntimeAttempt{Outcome: "interrupted", ChangedLines: 0}) != 0 {
		t.Fatal("interrupted 0 must be refund-eligible (0)")
	}
	if runtimeAttemptDeliveredIncrement(RuntimeAttempt{Outcome: "failed", ChangedLines: 0}) != 1 {
		t.Fatal("failed must count as delivered")
	}
	if runtimeAttemptDeliveredIncrement(RuntimeAttempt{Outcome: "passed", ChangedLines: 0}) != 1 {
		t.Fatal("passed must count as delivered")
	}
	// Build 3 refund-eligible interrupted 0 attempts with MaxAttempts=3
	for i := 1; i <= 3; i++ {
		acq, err := Acquire(AcquireParams{ChangeName: "ch-refund", RepoRoot: "r", RequestID: "req-refund-a" + string(rune('0'+i)), WorkUnit: "w", EvidenceGoal: "goal", MaxAttempts: 3, MaxLines: 1000, ChangedLines: 0})
		if err != nil {
			t.Fatalf("Acquire refund %d: %v", i, err)
		}
		if _, err := Settle(SettleParams{ChangeName: "ch-refund", RepoRoot: "r", Token: acq.Token, RequestID: "req-refund-s" + string(rune('0'+i)), Outcome: "interrupted", ChangedLines: 0, Diagnosis: "d"}); err != nil {
			t.Fatalf("Settle refund %d: %v", i, err)
		}
	}
	store, _, err := loadStore("ch-refund", "r")
	if err != nil {
		t.Fatalf("loadStore refund: %v", err)
	}
	if got := runtimeAttemptDeliveredIncrementSlice(store.Attempts); got != 0 {
		t.Fatalf("delivered after 3 interrupted 0 = %d want 0", got)
	}
	if got := runtimeRefundedAttempts(store); got != 3 {
		t.Fatalf("refunded after 3 interrupted 0 = %d want 3", got)
	}
	if len(store.Attempts) != 3 {
		t.Fatalf("len attempts = %d want 3", len(store.Attempts))
	}
	// Next 3 delivered attempts to reach 2x cap (total 6)
	for i := 4; i <= 6; i++ {
		acq, err := Acquire(AcquireParams{ChangeName: "ch-refund", RepoRoot: "r", RequestID: "req-refund-a" + string(rune('0'+i)), WorkUnit: "w", EvidenceGoal: "goal", MaxAttempts: 3, MaxLines: 1000, ChangedLines: 10})
		if err != nil {
			t.Fatalf("Acquire delivered %d: %v", i, err)
		}
		if _, err := Settle(SettleParams{ChangeName: "ch-refund", RepoRoot: "r", Token: acq.Token, RequestID: "req-refund-s" + string(rune('0'+i)), Outcome: "failed", ChangedLines: 10, Diagnosis: "d"}); err != nil {
			t.Fatalf("Settle delivered %d: %v", i, err)
		}
	}
	store2, _, err := loadStore("ch-refund", "r")
	if err != nil {
		t.Fatalf("loadStore2: %v", err)
	}
	if len(store2.Attempts) != 6 {
		t.Fatalf("len after 6 = %d want 6", len(store2.Attempts))
	}
	if got := runtimeRefundedAttempts(store2); got != 3 {
		t.Fatalf("refunded after 6 = %d want 3 (capped)", got)
	}
	// 7th acquire must be blocked at 2x cap
	_, err = Acquire(AcquireParams{ChangeName: "ch-refund", RepoRoot: "r", RequestID: "req-refund-a7", WorkUnit: "w", EvidenceGoal: "goal", MaxAttempts: 3, MaxLines: 1000, ChangedLines: 10})
	if err == nil {
		t.Fatal("Acquire at 2x cap must be blocked")
	}
	var blocked *BlockedError
	if !errors.As(err, &blocked) || blocked.Reason != BlockedReasonBudgetExhausted {
		t.Fatalf("2x cap blocked reason %v want budget_exhausted", err)
	}
	// Also test Begin path refund cap
	setStoreRoot(t)
	for i := 1; i <= 6; i++ {
		changed := 0
		outcome := "interrupted"
		if i > 3 {
			changed = 10
			outcome = "failed"
		}
		b, err := Begin(BeginParams{ChangeName: "ch-refund-b", RepoRoot: "r", RequestID: "req-b" + string(rune('0'+i)), MaxAttempts: 3, MaxLines: 1000, ChangedLines: changed, WorkUnit: "w"})
		if err != nil {
			t.Fatalf("Begin %d: %v", i, err)
		}
		if _, err := Finish(FinishParams{ChangeName: "ch-refund-b", RepoRoot: "r", ExpectedRev: b.Revision, Outcome: outcome, ChangedLines: changed, Diagnosis: "d", RequestID: "req-f" + string(rune('0'+i))}); err != nil {
			t.Fatalf("Finish %d: %v", i, err)
		}
	}
	_, err = Begin(BeginParams{ChangeName: "ch-refund-b", RepoRoot: "r", RequestID: "req-b7", MaxAttempts: 3, MaxLines: 1000, ChangedLines: 10, WorkUnit: "w"})
	if err == nil {
		t.Fatal("Begin at 2x cap must be blocked")
	}
	if !errors.As(err, &blocked) || blocked.Reason != BlockedReasonBudgetExhausted {
		t.Fatalf("Begin 2x cap blocked reason %v want budget_exhausted", err)
	}
}

func TestRecordRejected(t *testing.T) {
	setStoreRoot(t)
	if _, err := Begin(BeginParams{ChangeName: "ch-reject", RepoRoot: "r", WorkUnit: "w"}); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	storeDir := filepath.Join(storeRootOverride, RuntimeVersion, "ch-reject")
	headData, err := os.ReadFile(filepath.Join(storeDir, "HEAD"))
	if err != nil {
		t.Fatalf("read HEAD: %v", err)
	}
	head := strings.TrimSpace(string(headData))
	recordPath := filepath.Join(storeDir, "record-"+head+".json")
	data, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("read record: %v", err)
	}
	// Tamper hash: change a byte
	tampered := strings.Replace(string(data), `"change_name":"ch-reject"`, `"change_name":"other"`, 1)
	if err := os.WriteFile(recordPath, []byte(tampered), 0644); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	s, err := resolveStore("ch-reject", "r")
	if err != nil {
		t.Fatalf("resolveStore: %v", err)
	}
	_, err = s.loadRecord(head)
	if err == nil {
		t.Fatal("tampered record must fail")
	}
	var rejected *RuntimeRecordRejectedError
	if !errors.As(err, &rejected) {
		t.Fatalf("tampered must be RuntimeRecordRejectedError, got %T %v", err, err)
	}
	// Also test schema error: write invalid JSON
	if err := os.WriteFile(recordPath, []byte("{invalid json"), 0644); err != nil {
		t.Fatalf("tamper2: %v", err)
	}
	_, err = s.loadRecord(head)
	if !errors.As(err, &rejected) {
		t.Fatalf("schema error must be RuntimeRecordRejectedError, got %v", err)
	}
	// Test staleness via commit CAS conflict
	setStoreRoot(t)
	b1, err := Begin(BeginParams{ChangeName: "ch-stale2", RepoRoot: "r", WorkUnit: "w"})
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	f1, err := Finish(FinishParams{ChangeName: "ch-stale2", RepoRoot: "r", ExpectedRev: b1.Revision, Outcome: "failed", Diagnosis: "d"})
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	b2, err := Begin(BeginParams{ChangeName: "ch-stale2", RepoRoot: "r", ExpectedRev: f1.Revision, WorkUnit: "w2"})
	if err != nil {
		t.Fatalf("Begin b2: %v", err)
	}
	_ = b2
	s2, _ := resolveStore("ch-stale2", "r")
	stale := &RuntimeStore{ChangeName: "ch-stale2", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"}
	stale.Revision = b1.Revision
	err = s2.commit(stale)
	if err == nil {
		t.Fatal("stale commit must fail")
	}
	if !errors.As(err, &rejected) {
		t.Fatalf("stale CAS must be RuntimeRecordRejectedError, got %T %v", err, err)
	}
}
