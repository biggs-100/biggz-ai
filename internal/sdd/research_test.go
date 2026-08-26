package sdd

import (
	"strings"
	"testing"
)

func TestResearchHybridDivergentBlocked(t *testing.T) {
	bytesA := []byte("schema: biggz-ai.sdd-research/v1\nrevision: 1\noutcome: done\ncontent: a")
	bytesB := []byte("schema: biggz-ai.sdd-research/v1\nrevision: 1\noutcome: done\ncontent: b")
	ready, reason := EvaluateResearchHybrid("hybrid", ResearchDone, 1, bytesA, 1, bytesB)
	if ready || reason == "" || (!strings.Contains(reason, "divergence") && !strings.Contains(reason, "differ")) {
		t.Fatalf("divergent hybrid should block, got ready=%v reason=%q", ready, reason)
	}
	// also different revisions
	ready2, _ := EvaluateResearchHybrid("hybrid", ResearchDone, 1, bytesA, 2, bytesA)
	if ready2 {
		t.Fatalf("different revisions should block")
	}
}

func TestResearchHybridOneSidedRecoveryWritesBoth(t *testing.T) {
	retainedRev := 5
	canonical := []byte("schema: biggz-ai.sdd-research/v1\nrevision: 6\noutcome: done\ncanonical")
	// one-sided: only OpenSpec had the write, Engram missing
	newRev, ready, reason := RecoverHybridResearch(retainedRev, canonical, 5, canonical, 0, nil)
	if !ready || newRev != retainedRev+1 {
		t.Fatalf("one-sided recovery should write both and be ready, got ready=%v newRev=%d reason=%q", ready, newRev, reason)
	}
	if newRev <= 0 || len(canonical) == 0 {
		t.Fatalf("recovery produced invalid revision/bytes")
	}
	// Verify that after recovery, hybrid evaluation with the new equal bytes would pass
	ready2, _ := EvaluateResearchHybrid("hybrid", ResearchDone, newRev, canonical, newRev, canonical)
	if !ready2 {
		t.Fatalf("post-recovery hybrid should be ready with equal bytes")
	}
}

func TestResearchHybridMissingBlocked(t *testing.T) {
	_, ready, reason := RecoverHybridResearch(0, nil, 0, nil, 0, nil)
	if ready {
		t.Fatalf("missing intent recovery should stay blocked, got ready")
	}
	if !strings.Contains(reason, "unavailable") && !strings.Contains(reason, "blocked") {
		t.Fatalf("missing intent reason should mention unavailable/blocked, got %q", reason)
	}
	_, ready2, _ := RecoverHybridResearch(5, []byte{}, 5, []byte("a"), 0, nil)
	if ready2 {
		t.Fatalf("empty canonical bytes should stay blocked")
	}
	// Also Evaluate with missing artifacts blocks
	ready3, reason3 := EvaluateResearchHybrid("hybrid", ResearchDone, 0, nil, 0, nil)
	if ready3 {
		t.Fatalf("missing hybrid artifacts should block, got ready")
	}
	if reason3 == "" {
		t.Fatalf("missing artifacts should produce reason")
	}
}
