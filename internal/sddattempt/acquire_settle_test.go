package sddattempt

import (
	"errors"
	"strings"
	"testing"
)

func TestAcquire_Settle_RoundTrip(t *testing.T) {
	setStoreRoot(t)

	acq, err := Acquire(AcquireParams{
		ChangeName:   "ch-acq-1",
		RepoRoot:     "r",
		RequestID:    "req-acq-1",
		WorkUnit:     "w",
		EvidenceGoal: "goal",
		MaxAttempts:  3,
		MaxLines:     400,
	})
	if err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}
	if acq.Token == "" {
		t.Fatal("Acquire() returned empty token")
	}
	if acq.Revision == "" {
		t.Fatal("Acquire() returned empty revision")
	}
	afterAcquire := storeFileBytes(t, "ch-acq-1")

	// Idempotent replay with same request ID returns same token.
	replay, err := Acquire(AcquireParams{
		ChangeName:   "ch-acq-1",
		RepoRoot:     "r",
		RequestID:    "req-acq-1",
		WorkUnit:     "w",
		EvidenceGoal: "goal",
		MaxAttempts:  3,
		MaxLines:     400,
	})
	if err != nil {
		t.Fatalf("Acquire(replay) error: %v", err)
	}
	if replay.Token != acq.Token {
		t.Fatalf("replay token %q != first %q", replay.Token, acq.Token)
	}
	afterReplay := storeFileBytes(t, "ch-acq-1")
	if string(afterReplay) != string(afterAcquire) {
		t.Fatal("store changed on idempotent replay")
	}

	// Settle by token.
	settle, err := Settle(SettleParams{
		ChangeName:         "ch-acq-1",
		RepoRoot:           "r",
		Token:              acq.Token,
		RequestID:          "req-settle-1",
		Outcome:            "passed",
		EvidenceRevision:   "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Diagnosis:          "ok",
		HarnessDisposition: "reused",
		CleanupEvidence:    "cleanup ok",
		ProcessEvidence:    "process ok",
	})
	if err != nil {
		t.Fatalf("Settle() error: %v", err)
	}
	if !settle.Complete {
		t.Fatalf("Settle() result %+v want Complete true", settle)
	}
	if settle.Revision == "" {
		t.Fatal("Settle() returned empty revision")
	}

	// Idempotent settle replay.
	replaySettle, err := Settle(SettleParams{
		ChangeName:         "ch-acq-1",
		RepoRoot:           "r",
		Token:              acq.Token,
		RequestID:          "req-settle-1",
		Outcome:            "passed",
		EvidenceRevision:   "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Diagnosis:          "ok",
		HarnessDisposition: "reused",
		CleanupEvidence:    "cleanup ok",
		ProcessEvidence:    "process ok",
	})
	if err != nil {
		t.Fatalf("Settle(replay) error: %v", err)
	}
	if replaySettle.Revision != settle.Revision {
		t.Fatalf("replay settle revision %q != first %q", replaySettle.Revision, settle.Revision)
	}
}

func TestAcquire_TokenContinuation(t *testing.T) {
	setStoreRoot(t)

	first, err := Acquire(AcquireParams{
		ChangeName:   "ch-acq-2",
		RepoRoot:     "r",
		RequestID:    "req-acq-2a",
		WorkUnit:     "w",
		EvidenceGoal: "goal",
	})
	if err != nil {
		t.Fatalf("first Acquire() error: %v", err)
	}

	// Second acquire while active, presenting the active token, should
	// succeed with the same token and not create a new attempt.
	second, err := Acquire(AcquireParams{
		ChangeName:   "ch-acq-2",
		RepoRoot:     "r",
		RequestID:    "req-acq-2b",
		WorkUnit:     "w",
		EvidenceGoal: "goal",
		Token:        first.Token,
	})
	if err != nil {
		t.Fatalf("second Acquire(token) error: %v", err)
	}
	if second.Token != first.Token {
		t.Fatalf("token continuation token %q != first %q", second.Token, first.Token)
	}
	status, err := StatusWithInstance("ch-acq-2", "r", "")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.AttemptCount != 1 {
		t.Fatalf("attempt count after token continuation = %d, want 1", status.AttemptCount)
	}
}

func TestAcquire_BlockedWhenActive(t *testing.T) {
	setStoreRoot(t)

	_, err := Acquire(AcquireParams{
		ChangeName:   "ch-acq-3",
		RepoRoot:     "r",
		RequestID:    "req-acq-3a",
		WorkUnit:     "w",
		EvidenceGoal: "goal",
	})
	if err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}

	// Second acquire without token while active should be blocked.
	_, err = Acquire(AcquireParams{
		ChangeName:   "ch-acq-3",
		RepoRoot:     "r",
		RequestID:    "req-acq-3b",
		WorkUnit:     "w",
		EvidenceGoal: "goal",
	})
	if err == nil {
		t.Fatal("expected blocked error for active attempt, got nil")
	}
	if !IsBlocked(err) {
		t.Fatalf("expected BlockedError, got %v", err)
	}
	var blocked *BlockedError
	if !errors.As(err, &blocked) || blocked.Reason != BlockedReasonActiveAttempt {
		t.Fatalf("expected active_attempt blocked reason, got %v", err)
	}
}

func TestAcquire_BlockedWhenComplete(t *testing.T) {
	setStoreRoot(t)

	acq, err := Acquire(AcquireParams{
		ChangeName:   "ch-acq-4",
		RepoRoot:     "r",
		RequestID:    "req-acq-4a",
		WorkUnit:     "w",
		EvidenceGoal: "goal",
	})
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if _, err := Settle(SettleParams{
		ChangeName:         "ch-acq-4",
		RepoRoot:           "r",
		Token:              acq.Token,
		RequestID:          "req-settle-4",
		Outcome:            "passed",
		EvidenceRevision:   "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Diagnosis:          "ok",
		HarnessDisposition: "reused",
		CleanupEvidence:    "c",
		ProcessEvidence:    "p",
	}); err != nil {
		t.Fatalf("Settle: %v", err)
	}

	// Now ledger is complete; next acquire should be blocked with corrupt_authority.
	_, err = Acquire(AcquireParams{
		ChangeName:   "ch-acq-4",
		RepoRoot:     "r",
		RequestID:    "req-acq-4b",
		WorkUnit:     "w",
		EvidenceGoal: "goal",
	})
	if err == nil {
		t.Fatal("expected blocked when complete")
	}
	var blocked *BlockedError
	if !errors.As(err, &blocked) || blocked.Reason != BlockedReasonCorruptAuthority {
		t.Fatalf("expected corrupt_authority, got %v", err)
	}

	// Status should project blocked reason.
	status, _ := StatusWithInstance("ch-acq-4", "r", "")
	if status.BlockedReason != BlockedReasonCorruptAuthority {
		t.Fatalf("status BlockedReason = %q, want %q", status.BlockedReason, BlockedReasonCorruptAuthority)
	}
}

func TestAcquire_BudgetExhaustedWithObligation(t *testing.T) {
	setStoreRoot(t)

	// MaxAttempts=1, so after one failed settle, next acquire is budget_exhausted.
	acq, err := Acquire(AcquireParams{
		ChangeName:   "ch-acq-5",
		RepoRoot:     "r",
		RequestID:    "req-acq-5a",
		WorkUnit:     "w",
		EvidenceGoal: "goal",
		MaxAttempts:  1,
		MaxLines:     400,
	})
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if _, err := Settle(SettleParams{
		ChangeName:         "ch-acq-5",
		RepoRoot:           "r",
		Token:              acq.Token,
		RequestID:          "req-settle-5",
		Outcome:            "failed",
		EvidenceRevision:   "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		Diagnosis:          "failed",
		HarnessDisposition: "reused",
		CleanupEvidence:    "c",
		ProcessEvidence:    "p",
	}); err != nil {
		t.Fatalf("Settle: %v", err)
	}

	// Next acquire should be budget_exhausted with settle obligation.
	_, err = Acquire(AcquireParams{
		ChangeName:   "ch-acq-5",
		RepoRoot:     "r",
		RequestID:    "req-acq-5b",
		WorkUnit:     "w",
		EvidenceGoal: "goal",
		MaxAttempts:  1,
		MaxLines:     400,
	})
	if err == nil {
		t.Fatal("expected budget_exhausted")
	}
	var blocked *BlockedError
	if !errors.As(err, &blocked) || blocked.Reason != BlockedReasonBudgetExhausted {
		t.Fatalf("expected budget_exhausted, got %v", err)
	}
	if blocked.SettleObligation == nil || blocked.SettleObligation.EvidenceRevision == "" {
		t.Fatalf("expected settle obligation, got %+v", blocked.SettleObligation)
	}
	status, _ := StatusWithInstance("ch-acq-5", "r", "")
	if status.BlockedReason != BlockedReasonBudgetExhausted {
		t.Fatalf("status blocked reason %q, want budget_exhausted", status.BlockedReason)
	}
	if status.SettleObligation == nil {
		t.Fatal("status settle obligation nil, want non-nil")
	}
}

func TestSettle_ProgressMultiUnitSingleFinalSettle(t *testing.T) {
	setStoreRoot(t)

	// One ledger scope ("units-multi") covers N=2 work units: an
	// intermediate settle(progress) must NOT complete the ledger, the next
	// acquire (same scope) must be admitted, and only the final
	// settle(passed) completes. RED: "progress" was not a valid outcome.
	acq, err := Acquire(AcquireParams{
		ChangeName: "ch-acq-multi", RepoRoot: "r", RequestID: "req-multi-1",
		WorkUnit: "units-multi", EvidenceGoal: "goal", MaxAttempts: 3, MaxLines: 400,
	})
	if err != nil {
		t.Fatalf("Acquire(unit1): %v", err)
	}
	mid, err := Settle(SettleParams{
		ChangeName: "ch-acq-multi", RepoRoot: "r", Token: acq.Token,
		RequestID: "req-multi-settle-1", Outcome: "progress",
		Diagnosis: "unit1 done", HarnessDisposition: "reused",
		CleanupEvidence: "c", ProcessEvidence: "p",
	})
	if err != nil {
		t.Fatalf("Settle(progress): %v", err)
	}
	if mid.Complete {
		t.Fatal("intermediate settle(progress) completed the ledger, want open")
	}
	status, _ := StatusWithInstance("ch-acq-multi", "r", "")
	if status.Complete || status.BlockedReason != "" || status.NextAction != "begin" {
		t.Fatalf("mid status = %+v, want open/begin/unblocked", status)
	}
	acq2, err := Acquire(AcquireParams{
		ChangeName: "ch-acq-multi", RepoRoot: "r", RequestID: "req-multi-2",
		WorkUnit: "units-multi", EvidenceGoal: "goal", MaxAttempts: 3, MaxLines: 400,
	})
	if err != nil {
		t.Fatalf("Acquire(unit2) after progress settle: %v", err)
	}
	final, err := Settle(SettleParams{
		ChangeName: "ch-acq-multi", RepoRoot: "r", Token: acq2.Token,
		RequestID: "req-multi-settle-2", Outcome: "passed",
		EvidenceRevision:   "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		Diagnosis:          "done", HarnessDisposition: "reused",
		CleanupEvidence: "c", ProcessEvidence: "p",
	})
	if err != nil {
		t.Fatalf("Settle(passed) final: %v", err)
	}
	if !final.Complete {
		t.Fatalf("final settle(passed) = %+v, want Complete true", final)
	}
}

func TestSettle_InvalidToken(t *testing.T) {
	setStoreRoot(t)

	acq, err := Acquire(AcquireParams{
		ChangeName:   "ch-acq-6",
		RepoRoot:     "r",
		RequestID:    "req-acq-6",
		WorkUnit:     "w",
		EvidenceGoal: "goal",
	})
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	_ = acq

	_, err = Settle(SettleParams{
		ChangeName:         "ch-acq-6",
		RepoRoot:           "r",
		Token:              "tok-invalid-token",
		RequestID:          "req-settle-6",
		Outcome:            "passed",
		EvidenceRevision:   "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		Diagnosis:          "ok",
		HarnessDisposition: "reused",
		CleanupEvidence:    "c",
		ProcessEvidence:    "p",
	})
	if err == nil || !strings.Contains(err.Error(), "does not continue") {
		t.Fatalf("expected invalid continuation error, got %v", err)
	}
}

func TestSettle_RequestIDReusedWithDifferentInputs(t *testing.T) {
	setStoreRoot(t)

	acq, err := Acquire(AcquireParams{
		ChangeName:   "ch-acq-7",
		RepoRoot:     "r",
		RequestID:    "req-acq-7",
		WorkUnit:     "w",
		EvidenceGoal: "goal",
	})
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	_, err = Settle(SettleParams{
		ChangeName:         "ch-acq-7",
		RepoRoot:           "r",
		Token:              acq.Token,
		RequestID:          "req-settle-7",
		Outcome:            "failed",
		EvidenceRevision:   "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		Diagnosis:          "d",
		HarnessDisposition: "reused",
		CleanupEvidence:    "c",
		ProcessEvidence:    "p",
	})
	if err != nil {
		t.Fatalf("first Settle: %v", err)
	}
	// Reuse same request ID with different outcome should fail.
	_, err = Settle(SettleParams{
		ChangeName:         "ch-acq-7",
		RepoRoot:           "r",
		Token:              acq.Token,
		RequestID:          "req-settle-7",
		Outcome:            "passed",
		EvidenceRevision:   "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		Diagnosis:          "d",
		HarnessDisposition: "reused",
		CleanupEvidence:    "c",
		ProcessEvidence:    "p",
	})
	if err == nil || !strings.Contains(err.Error(), "reused with different inputs") {
		t.Fatalf("expected reuse error, got %v", err)
	}
}
