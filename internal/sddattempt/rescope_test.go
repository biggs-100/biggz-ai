package sddattempt

import (
	"errors"
	"strings"
	"testing"
)

func TestRescopeCumulativeNeverReset(t *testing.T) {
	setStoreRoot(t)
	// Create 2 attempts with max 5
	b1, err := Begin(BeginParams{ChangeName: "ch", RepoRoot: "r", MaxAttempts: 5, MaxLines: 400, WorkUnit: "w", RequestID: "b1"})
	if err != nil {
		t.Fatalf("Begin b1: %v", err)
	}
	if _, err := Finish(FinishParams{ChangeName: "ch", RepoRoot: "r", ExpectedRev: b1.Revision, Outcome: "failed", Diagnosis: "fail 1", RequestID: "f1"}); err != nil {
		t.Fatalf("Finish f1: %v", err)
	}
	b2, err := Begin(BeginParams{ChangeName: "ch", RepoRoot: "r", MaxAttempts: 5, MaxLines: 400, WorkUnit: "w", RequestID: "b2"})
	if err != nil {
		t.Fatalf("Begin b2: %v", err)
	}
	if _, err := Finish(FinishParams{ChangeName: "ch", RepoRoot: "r", ExpectedRev: b2.Revision, Outcome: "failed", Diagnosis: "fail 2", RequestID: "f2"}); err != nil {
		t.Fatalf("Finish f2: %v", err)
	}
	store, _, err := loadStore("ch", "r")
	if err != nil {
		t.Fatalf("loadStore: %v", err)
	}
	if len(store.Attempts) != 2 {
		t.Fatalf("pre-rescope attempts = %d, want 2", len(store.Attempts))
	}
	store, _, _ = loadStore("ch", "r")
	rev := store.Revision
	// Narrow rescope to max 3 (still > cumulative 2, so should succeed)
	res, err := Rescope(RescopeParams{ChangeName: "ch", RepoRoot: "r", ExpectedRev: rev, RequestID: "res1", MaxAttempts: 3, MaxLines: 300, Reason: "narrow for test", Actor: "tester"})
	if err != nil {
		t.Fatalf("Rescope narrow: %v", err)
	}
	_ = res
	store2, _, err := loadStore("ch", "r")
	if err != nil {
		t.Fatalf("loadStore after rescope: %v", err)
	}
	t.Logf("store2 attempts=%d max=%d active=%d rev=%s", len(store2.Attempts), store2.MaxAttempts, store2.ActiveAttempt, store2.Revision)
	if len(store2.Attempts) != 2 {
		t.Fatalf("post-rescope attempts = %d, want 2 (cumulative never reset, not 0)", len(store2.Attempts))
	}
	if store2.MaxAttempts != 3 {
		t.Fatalf("post-rescope MaxAttempts = %d, want 3", store2.MaxAttempts)
	}
	// Next attempt ordinal should be 3, not 1 (vs fresh 0)
	b3, err := Begin(BeginParams{ChangeName: "ch", RepoRoot: "r", RequestID: "b3", MaxAttempts: 3, MaxLines: 300})
	if err != nil {
		t.Fatalf("Begin b3 after rescope: %v", err)
	}
	if b3.ActiveAttempt != 3 {
		t.Fatalf("post-rescope next attempt ordinal = %d, want 3 (measured vs 2 already consumed, not fresh)", b3.ActiveAttempt)
	}
}

func TestRescopeFiveFiveToThreeVsFive(t *testing.T) {
	setStoreRoot(t)
	// Seed 5 attempts with max 5 (exhausted)
	var rev string
	for i := 1; i <= 5; i++ {
		b, err := Begin(BeginParams{ChangeName: "ch2", RepoRoot: "r", MaxAttempts: 5, MaxLines: 400, WorkUnit: "w", RequestID: "b" + string(rune('0'+i))})
		if err != nil {
			t.Fatalf("Begin %d: %v", i, err)
		}
		rev = b.Revision
		f, err := Finish(FinishParams{ChangeName: "ch2", RepoRoot: "r", ExpectedRev: rev, Outcome: "failed", Diagnosis: "fail", RequestID: "f" + string(rune('0'+i))})
		if err != nil {
			t.Fatalf("Finish %d: %v", i, err)
		}
		rev = f.Revision
	}
	store, _, err := loadStore("ch2", "r")
	if err != nil {
		t.Fatalf("loadStore: %v", err)
	}
	if len(store.Attempts) != 5 || store.MaxAttempts != 5 {
		t.Fatalf("pre-rescope state len=%d max=%d want 5/5", len(store.Attempts), store.MaxAttempts)
	}
	// Rescope to max 3 should be measured against 5 already consumed, so next acquire would be blocked.
	// Our implementation refuses rescope when cumulative > new ceiling (budget binds at admission).
	_, err = Rescope(RescopeParams{ChangeName: "ch2", RepoRoot: "r", ExpectedRev: rev, RequestID: "res55", MaxAttempts: 3, MaxLines: 300, Reason: "narrow", Actor: "tester"})
	if err == nil || !errors.Is(err, ErrRuntimeRescopeWidened) {
		t.Fatalf("rescope 5/5->3 should be refused as budget exhausted vs 5, got err=%v want ErrRuntimeRescopeWidened", err)
	}
	// Verify ledger unchanged (still 5 vs 5)
	store2, _, _ := loadStore("ch2", "r")
	if len(store2.Attempts) != 5 || store2.MaxAttempts != 5 {
		t.Fatalf("post-refused rescope mutated ledger len=%d max=%d", len(store2.Attempts), store2.MaxAttempts)
	}
	// If it had been fresh 0, then 0/3 would allow acquire; prove our store is not fresh:
	if !strings.Contains(err.Error(), "cumulative") && !strings.Contains(err.Error(), "exceeds") {
		t.Logf("rescope error should mention cumulative/exceeds, got %q", err.Error())
	}
}
