package review

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// startOnlyFixture starts a review with no lens captures.
func startOnlyFixture(t *testing.T, lineageID string) (string, string) {
	t.Helper()
	repo, _, head := finalizeFixtureRepo(t)
	finalizeStart(t, repo, head, lineageID, nil, "")
	return repo, head
}

func TestInvalidate_AppendsEventAndGatesFailWithReason(t *testing.T) {
	repo, head := startOnlyFixture(t, "inval-1")

	revision, err := Invalidate(repo, "inval-1", "scope changed beyond the review")
	if err != nil {
		t.Fatalf("Invalidate: %v", err)
	}
	if revision == "" {
		t.Fatal("invalidate returned an empty revision")
	}

	store, err := Open(repo, "inval-1")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	// start_review + in_review + invalidate.
	if chain.Count != 3 {
		t.Fatalf("event count = %d, want 3", chain.Count)
	}
	last := chain.Records[chain.Count-1]
	if last.Operation != InvalidateOperation {
		t.Fatalf("last operation = %q, want %q", last.Operation, InvalidateOperation)
	}
	if last.Role != "Admin" {
		t.Errorf("invalidate role = %q, want Admin (FSM guard)", last.Role)
	}
	state, reason := terminatedStateOf(chain)
	if state != "invalidated" || reason != "scope changed beyond the review" {
		t.Fatalf("terminatedStateOf = (%q, %q)", state, reason)
	}

	// Gates fail with the reason — a pass is never fabricated.
	result, err := EvaluateGate(GatePreCommit, repo, "inval-1", GateOptions{})
	if err != nil {
		t.Fatalf("EvaluateGate: %v", err)
	}
	if result.Allowed || result.Passed {
		t.Fatal("gate must not pass an invalidated lineage")
	}
	if !strings.Contains(result.Reason, "lineage is invalidated: scope changed beyond the review") {
		t.Fatalf("gate reason = %q, want the invalidate reason", result.Reason)
	}

	// Finalize refuses terminated lineages.
	if _, err := Finalize(repo, "inval-1"); err == nil || !strings.Contains(err.Error(), "invalidated") {
		t.Fatalf("Finalize on invalidated lineage = %v, want refusal", err)
	}
	_ = head
}

func TestInvalidate_RequiresReason(t *testing.T) {
	repo, _ := startOnlyFixture(t, "inval-2")
	if _, err := Invalidate(repo, "inval-2", "   "); err == nil {
		t.Fatal("invalidate with a blank reason must fail")
	}
}

func TestInvalidate_EmptyLineageRefused(t *testing.T) {
	repo := t.TempDir()
	gitInit(t, repo)
	if _, err := Invalidate(repo, "inval-empty", "reason"); err == nil || !strings.Contains(err.Error(), "no events") {
		t.Fatalf("invalidate on empty lineage = %v, want refusal", err)
	}
}

func TestInvalidate_BrokenChainRefused(t *testing.T) {
	repo, _ := startOnlyFixture(t, "inval-3")
	store, err := Open(repo, "inval-3")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	// Tamper with parseable content so LoadChain succeeds and the integrity
	// verdict (content address) catches it.
	genesis := chain.GenesisHash
	data, err := os.ReadFile(filepath.Join(store.Dir, genesis))
	if err != nil {
		t.Fatalf("read genesis: %v", err)
	}
	if err := os.WriteFile(filepath.Join(store.Dir, genesis), append(data, ' '), 0644); err != nil {
		t.Fatalf("tamper genesis: %v", err)
	}
	if _, err := Invalidate(repo, "inval-3", "reason"); err == nil || !strings.Contains(err.Error(), "chain integrity") {
		t.Fatalf("invalidate on broken chain = %v, want chain integrity refusal", err)
	}
}

func TestInvalidate_AlreadyTerminatedRefused(t *testing.T) {
	repo, _ := startOnlyFixture(t, "inval-4")
	if _, err := Invalidate(repo, "inval-4", "first"); err != nil {
		t.Fatalf("first Invalidate: %v", err)
	}
	if _, err := Invalidate(repo, "inval-4", "second"); err == nil || !strings.Contains(err.Error(), "already invalidated") {
		t.Fatalf("second Invalidate = %v, want already-invalidated refusal", err)
	}
}

func TestAbandon_WithdrawsAndKeepsChainReadable(t *testing.T) {
	repo, _ := startOnlyFixture(t, "aband-1")

	revision, err := Abandon(repo, "aband-1")
	if err != nil {
		t.Fatalf("Abandon: %v", err)
	}
	if revision == "" {
		t.Fatal("abandon returned an empty revision")
	}

	store, err := Open(repo, "aband-1")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	// start_review + in_review + withdraw.
	if chain.Count != 3 || chain.Records[2].Operation != WithdrawOperation {
		t.Fatalf("chain = %d events, last %q, want 3 with %q", chain.Count, chain.Records[chain.Count-1].Operation, WithdrawOperation)
	}
	state, _ := terminatedStateOf(chain)
	if state != "withdrawn" {
		t.Fatalf("state = %q, want withdrawn", state)
	}
	// The chain stays valid and exportable/importable (export replays the
	// same records; nothing here mutates beyond the withdraw event).
	if verdict := store.Validate(); !verdict.Valid {
		t.Fatalf("chain must stay valid after abandon: %s", verdict.Reason)
	}

	// Gates fail for a withdrawn lineage.
	result, err := EvaluateGate(GateRelease, repo, "aband-1", GateOptions{})
	if err != nil {
		t.Fatalf("EvaluateGate: %v", err)
	}
	if result.Allowed || result.Passed {
		t.Fatal("gate must not pass a withdrawn lineage")
	}
	if !strings.Contains(result.Reason, "lineage is withdrawn") {
		t.Fatalf("gate reason = %q, want withdrawn", result.Reason)
	}

	// Double abandon is refused.
	if _, err := Abandon(repo, "aband-1"); err == nil || !strings.Contains(err.Error(), "already withdrawn") {
		t.Fatalf("second Abandon = %v, want already-withdrawn refusal", err)
	}
}

func TestTerminalEvent_RecordsCarryRoles(t *testing.T) {
	repo, _ := startOnlyFixture(t, "inval-5")
	if _, err := Invalidate(repo, "inval-5", "admin call"); err != nil {
		t.Fatalf("Invalidate: %v", err)
	}
	store, err := Open(repo, "inval-5")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	chain, err := store.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	if chain.Records[chain.Count-1].Role != "Admin" {
		t.Errorf("invalidate role = %q, want Admin (FSM guard)", chain.Records[chain.Count-1].Role)
	}

	if _, err := Abandon(repo, "inval-5"); err == nil || !strings.Contains(err.Error(), "already invalidated") {
		t.Fatalf("abandon on invalidated = %v, want refusal", err)
	}
}
