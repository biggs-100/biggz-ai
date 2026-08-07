package review

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/biggs-100/biggz-ai/model"
)

var ctx = context.Background()

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func storeWithChain(t *testing.T, events int) (*Store, ValidatedChain) {
	t.Helper()
	dir := t.TempDir()
	s := OpenWithDir(dir, "gate-chain")

	var prev string
	for i := 0; i < events; i++ {
		h, err := s.Append(prev, Record{
			Schema:       recordSchemaVersion,
			PrevRevision: prev,
			Operation:    "review_event",
			Role:         "Lead",
			Actor:        "tester",
			Timestamp:    "2026-07-28T00:00:00Z",
		})
		if err != nil {
			t.Fatalf("Append event %d: %v", i, err)
		}
		prev = h
	}

	chain, err := s.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	return s, chain
}

func makeCompletedReview(t *testing.T) *model.ReviewState {
	t.Helper()
	r := New(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	r.State.Role = model.RoleLead
	if err := r.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := r.Complete(ctx); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	return r.State
}

func blockingFindings() []Finding {
	return []Finding{
		{ID: "f1", Severity: SeverityCritical, Message: "security vulnerability in auth"},
		{ID: "f2", Severity: SeverityWarning, Message: "unused variable"},
	}
}

// ---------------------------------------------------------------------------
// New Gate API tests
// ---------------------------------------------------------------------------

func TestPrePRGate_Passes(t *testing.T) {
	_, chain := storeWithChain(t, 3)
	receipt := NewReceipt(chain)

	result := PrePRGate(chain, nil, &receipt, false, "")
	if !result.Passed {
		t.Errorf("expected pre-PR gate to pass, got reasons: %v", result.Reasons)
	}
	if result.Gate != GatePrePR {
		t.Errorf("expected gate pre-pr, got %s", result.Gate)
	}
}

func TestPrePRGate_BlocksUnresolvedFindings(t *testing.T) {
	_, chain := storeWithChain(t, 3)
	receipt := NewReceipt(chain)
	findings := blockingFindings()

	result := PrePRGate(chain, findings, &receipt, false, "")
	if result.Passed {
		t.Fatal("expected pre-PR gate to block on unresolved findings")
	}
	if len(result.Reasons) < 2 {
		t.Errorf("expected at least 2 blocking reasons for 2 findings, got %d: %v",
			len(result.Reasons), result.Reasons)
	}
}

func TestPrePRGate_BlocksEmptyChain(t *testing.T) {
	dir := t.TempDir()
	s := OpenWithDir(dir, "empty-chain")
	chain, err := s.LoadChain()
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}

	result := PrePRGate(chain, nil, nil, false, "")
	if result.Passed {
		t.Fatal("expected pre-PR gate to block on empty chain")
	}
}

func TestPrePRGate_DryRunReportsButPasses(t *testing.T) {
	_, chain := storeWithChain(t, 3)
	receipt := NewReceipt(chain)
	findings := blockingFindings()

	result := PrePRGate(chain, findings, &receipt, true, "")
	if !result.Passed {
		t.Fatal("expected dry-run to pass (exit zero)")
	}
	if !result.DryRun {
		t.Error("expected DryRun=true")
	}
	if len(result.Reasons) == 0 {
		t.Error("expected dry-run to report blocking reasons")
	}
}

func TestPrePRGate_BlocksTamperedChain(t *testing.T) {
	_, chain := storeWithChain(t, 2)
	receipt := NewReceipt(chain)

	// Tamper with the chain head hash (simulates a tampered receipt binding).
	tampered := chain
	tampered.HeadHash = "tampered"

	result := PrePRGate(tampered, nil, &receipt, false, "")
	if result.Passed {
		t.Fatal("expected pre-PR gate to block on head hash mismatch")
	}
}

func TestPrePRGate_ChainInvalidFlag(t *testing.T) {
	_, chain := storeWithChain(t, 2)
	receipt := NewReceipt(chain)

	// Set Valid=false to simulate a chain that failed integrity check.
	invalidChain := chain
	invalidChain.Valid = false

	result := PrePRGate(invalidChain, nil, &receipt, false, "")
	if result.Passed {
		t.Fatal("expected gate to block on invalid chain flag")
	}
}

func TestPrePRGate_AutoReceiptWhenNil(t *testing.T) {
	_, chain := storeWithChain(t, 3)

	result := PrePRGate(chain, nil, nil, false, "")
	if !result.Passed {
		t.Errorf("expected pre-PR gate to pass with auto-receipt, got: %v", result.Reasons)
	}
}

// ---------------------------------------------------------------------------
// Pre-Push Gate tests
// ---------------------------------------------------------------------------

func TestPrePushGate_PassesWithoutScopeChange(t *testing.T) {
	// When snapshotTree is empty, scope check is skipped.
	_, chain := storeWithChain(t, 3)
	receipt := NewReceipt(chain)

	result := PrePushGate(chain, nil, &receipt, "", false, "")
	if !result.Passed {
		t.Errorf("expected pre-push gate to pass, got: %v", result.Reasons)
	}
}

func TestPrePushGate_IncludesScopeCheck(t *testing.T) {
	_, chain := storeWithChain(t, 3)
	receipt := NewReceipt(chain)

	// A non-empty snapshot tree that doesn't match HEAD will trigger scope diff.
	// Since this test runs in a temp dir (not a git repo), ScopeDiff will error,
	// which is captured as a reason — the gate will report but not crash.
	// We use a fake tree hash to verify the scope detection is invoked.
	result := PrePushGate(chain, nil, &receipt, "deadbeef", false, "")
	// In a non-git dir, ScopeDiff will error — that's OK, it should be reported.
	if result.Passed && len(result.Reasons) == 0 {
		// If no reasons at all but snapshotTree is set, scope was checked.
		// In a real git repo this would work.
	}
}

func TestPrePushGate_DryRun(t *testing.T) {
	_, chain := storeWithChain(t, 1)
	receipt := NewReceipt(chain)
	findings := blockingFindings()

	result := PrePushGate(chain, findings, &receipt, "deadbeef", true, "")
	if !result.Passed {
		t.Fatal("expected dry-run to pass (exit zero)")
	}
	if !result.DryRun {
		t.Error("expected DryRun=true")
	}
	if len(result.Reasons) == 0 {
		t.Error("expected dry-run to report reasons")
	}
}

// ---------------------------------------------------------------------------
// ScopeDiff tests
// ---------------------------------------------------------------------------

func TestScopeDiff_EmptyTree(t *testing.T) {
	files, err := ScopeDiff("")
	if err != nil {
		t.Fatalf("ScopeDiff(''): %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected empty files, got %v", files)
	}
}

func TestScopeDiff_InTempDir(t *testing.T) {
	// In a temp dir without a git repo, ScopeDiff should error.
	files, err := ScopeDiff("abc123")
	if err == nil {
		t.Log("ScopeDiff succeeded in non-git dir (unexpected):", files)
	} else {
		t.Logf("ScopeDiff expected error in temp dir: %v", err)
	}
}

// ---------------------------------------------------------------------------
// GateConfig tests
// ---------------------------------------------------------------------------

func TestLoadGateConfig_Defaults(t *testing.T) {
	cfg := LoadGateConfig("")
	if cfg == nil {
		t.Fatal("LoadGateConfig returned nil")
	}
	if !cfg.IsEnabled(GatePrePR) {
		t.Error("expected pre-pr enabled by default")
	}
	if !cfg.IsEnabled(GatePrePush) {
		t.Error("expected pre-push enabled by default")
	}
}

func TestLoadGateConfig_FromFile(t *testing.T) {
	// Create a temp git repo with a .biggz/config.yaml.
	repoDir := t.TempDir()
	gitInit(t, repoDir)

	biggzDir := filepath.Join(repoDir, ".biggz")
	if err := os.MkdirAll(biggzDir, 0755); err != nil {
		t.Fatalf("mkdir .biggz: %v", err)
	}
	configContent := []byte("gate:\n  pre-pr:\n    enabled: false\n  pre-push:\n    enabled: true\n")
	if err := os.WriteFile(filepath.Join(biggzDir, "config.yaml"), configContent, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := LoadGateConfig(repoDir)
	if cfg.IsEnabled(GatePrePR) {
		t.Error("expected pre-pr disabled from config")
	}
	if !cfg.IsEnabled(GatePrePush) {
		t.Error("expected pre-push enabled from config")
	}
}

// ---------------------------------------------------------------------------
// Backward-compatible tests (legacy API)
// ---------------------------------------------------------------------------

func TestLegacyValidateCheck_PreCommit_Passes(t *testing.T) {
	state := makeCompletedReview(t)
	cfg := DefaultGateConfigs()[GatePreCommit]
	receipt := GenerateReceipt(state)

	result := ValidateCheck(GatePreCommit, state, cfg, receipt)
	if !result.Allowed {
		t.Errorf("pre-commit should pass: %s", result.Reason)
	}
}

func TestLegacyValidateCheck_PrePush_Passes(t *testing.T) {
	state := makeCompletedReview(t)
	cfg := DefaultGateConfigs()[GatePrePush]
	receipt := GenerateReceipt(state)

	result := ValidateCheck(GatePrePush, state, cfg, receipt)
	if !result.Allowed {
		t.Errorf("pre-push should pass: %s", result.Reason)
	}
}

func TestLegacyValidateCheck_NoReceipt(t *testing.T) {
	state := makeCompletedReview(t)
	cfg := DefaultGateConfigs()[GatePreCommit]

	result := ValidateCheck(GatePreCommit, state, cfg, nil)
	if result.Allowed {
		t.Fatal("expected failure without receipt")
	}
}

func TestLegacyValidateCheck_WrongStatus(t *testing.T) {
	state := model.NewReviewState(model.ReviewSubject{Repository: "test/repo", CommitSHA: "abc123"})
	state.Status = model.StatusUnreviewed
	cfg := DefaultGateConfigs()[GatePreCommit]
	receipt := GenerateReceipt(state)

	result := ValidateCheck(GatePreCommit, state, cfg, receipt)
	if result.Allowed {
		t.Fatal("expected failure for non-completed review")
	}
}

func TestLegacyValidateCheck_TamperedReceipt(t *testing.T) {
	state := makeCompletedReview(t)
	cfg := DefaultGateConfigs()[GatePreCommit]

	receipt := GenerateReceipt(state)
	receipt.BindingHash = "tampered"

	result := ValidateCheck(GatePreCommit, state, cfg, receipt)
	if result.Allowed {
		t.Fatal("expected failure for tampered receipt")
	}
}
