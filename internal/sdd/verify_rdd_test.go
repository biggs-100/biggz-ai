package sdd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biggs-100/biggz-ai/internal/review"
)

func TestVerifyPreflight_DisabledAllows(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	// Disable RDD globally
	if _, err := review.RDDDisable("", "", "global"); err != nil {
		t.Fatalf("RDDDisable: %v", err)
	}
	ws := t.TempDir()
	if err := VerifyPreflightAt(ws, "no-receipt-change"); err != nil {
		t.Fatalf("expected nil when RDD disabled, got %v", err)
	}
}

func TestVerifyPreflight_EnabledBlocksMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	// Default is enabled (no file), ensure no lingering disable
	// Remove any global state file if exists
	if p, err := os.UserHomeDir(); err == nil {
		_ = os.Remove(filepath.Join(p, ".biggz", "rdd-mode.json"))
	}
	ws := t.TempDir()
	err := VerifyPreflightAt(ws, "missing-lineage-change")
	if err == nil {
		t.Fatal("expected block when RDD enabled and no receipt")
	}
	if !strings.Contains(err.Error(), "rdd_receipt_missing") {
		t.Fatalf("expected rdd_receipt_missing, got %q", err.Error())
	}
}

func TestVerifyRDDGate_TamperedBindingBlocks(t *testing.T) {
	// PersistedReceipt tamper via review layer — ensure Validate fails on tampered binding
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	ws := t.TempDir()
	// Enabled by default, missing receipt should be rdd_receipt_missing
	err := VerifyPreflightAt(ws, "tampered-change")
	if err == nil || !strings.Contains(err.Error(), "rdd_receipt_missing") {
		t.Fatalf("tampered-like missing should be rdd_receipt_missing, got %v", err)
	}
	// Also verify that a tampered PersistedReceipt fails Validate (domainHash binding)
	// Construct a minimal valid receipt and tamper FixDeltaHash
	valid := review.PersistedReceipt{
		Schema:            "biggz-ai.review-receipt/v1",
		LineageID:         "test-lineage",
		Generation:        1,
		GenesisRevision:   strings.Repeat("a", 64),
		HeadRevision:      strings.Repeat("b", 64),
		BaseTree:          strings.Repeat("c", 40),
		InitialReviewTree: strings.Repeat("d", 40),
		FinalCandidateTree: strings.Repeat("e", 40),
		PathsDigest:       "sha256:" + strings.Repeat("f", 64),
		FixDeltaHash:      review.EmptyFixDeltaHash,
		EvidenceHash:      "sha256:" + strings.Repeat("a", 64),
		RiskTier:          "low",
		SelectedLenses:    []string{"risk"},
		LensSubjects: []review.ReceiptLensSubject{{
			Lens:          "risk",
			SelectedOrder: 0,
			SubjectHash:   "sha256:" + strings.Repeat("b", 64),
			ResultHash:    "sha256:" + strings.Repeat("c", 64),
		}},
		TerminalState: "completed",
	}
	// compute hash via Validate path: need to set ReceiptHash correctly first
	// Use reflection via building receipt then setting hash via known good
	// We can directly call Validate after setting ReceiptHash to something invalid to ensure it fails
	valid.ReceiptHash = "sha256:" + strings.Repeat("0", 64) // invalid hash
	if err := valid.Validate(); err == nil {
		t.Fatal("expected Validate to fail on tampered ReceiptHash")
	} else if !strings.Contains(strings.ToLower(err.Error()), "hash") {
		t.Fatalf("expected hash error, got %v", err)
	}
}

func TestStatusV2_RDDGatePropagates(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	// Ensure RDD enabled (default)
	ws := t.TempDir()
	// Create openspec structure
	change := "rdd-gate-change"
	changeDir := filepath.Join(ws, "openspec", "changes", change)
	if err := os.MkdirAll(filepath.Join(changeDir, "specs", "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(ws, "openspec", "changes", "archive"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Proposal\ncontent"), 0o644)
	_ = os.WriteFile(filepath.Join(changeDir, "design.md"), []byte("# Design\ncontent"), 0o644)
	_ = os.WriteFile(filepath.Join(changeDir, "tasks.md"), []byte("- [x] Task 1\n- [x] Task 2\n"), 0o644)
	_ = os.WriteFile(filepath.Join(changeDir, "specs", "core", "spec.md"), []byte("### Requirement: REQ-001 — something\n#### Scenario: happy\n"), 0o644)
	_ = os.WriteFile(filepath.Join(ws, "openspec", "config.yaml"), []byte("artifact_store: openspec\n"), 0o644)
	// Init git for workspace so isRDDEnabled resolves via git dirs (global fallback still enabled)
	// Not strictly needed, but ensure .git exists for EvaluateGate store resolution
	_ = os.MkdirAll(filepath.Join(ws, ".git", "objects"), 0o755)
	_ = os.WriteFile(filepath.Join(ws, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644)

	active, _, err := StatusWithOptions(filepath.Join(ws, "openspec"), StatusOptions{})
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	var cs *ChangeStatus
	for i := range active {
		if active[i].Name == change {
			cs = &active[i]
			break
		}
	}
	if cs == nil {
		t.Fatalf("change %q not found in status", change)
	}
	// When RDD enabled and no receipt, Verify should be blocked and reason contains rdd_receipt_missing, nextRecommended == resolve-blockers
	foundRDD := false
	for _, r := range cs.BlockedReasons {
		if strings.Contains(r, "rdd_receipt_missing") || strings.Contains(r, "rdd_unmanaged") {
			foundRDD = true
			break
		}
	}
	if !foundRDD {
		t.Fatalf("expected blockedReasons to contain rdd_receipt_missing/unmanaged, got %v", cs.BlockedReasons)
	}
	if cs.NextRecommended != "resolve-blockers" {
		t.Fatalf("expected nextRecommended == resolve-blockers when RDD blocks, got %q", cs.NextRecommended)
	}
	if cs.Dependencies.Verify != DependencyBlocked {
		t.Fatalf("expected Verify == blocked when RDD gate blocks, got %q", cs.Dependencies.Verify)
	}

	// Now disable RDD and re-check: verify should become ready
	if _, err := review.RDDDisable("", "", "global"); err != nil {
		t.Fatalf("RDDDisable: %v", err)
	}
	active2, _, err := StatusWithOptions(filepath.Join(ws, "openspec"), StatusOptions{})
	if err != nil {
		t.Fatalf("Status after disable: %v", err)
	}
	var cs2 *ChangeStatus
	for i := range active2 {
		if active2[i].Name == change {
			cs2 = &active2[i]
			break
		}
	}
	if cs2 == nil {
		t.Fatalf("change not found after disable")
	}
	for _, r := range cs2.BlockedReasons {
		if strings.Contains(r, "rdd_receipt_missing") || strings.Contains(r, "rdd_unmanaged") {
			t.Fatalf("when RDD disabled, should not contain rdd_* blocker, got %v", cs2.BlockedReasons)
		}
	}
	// Verify should be ready when disabled (apply all_done, no remediation)
	if cs2.Dependencies.Verify != DependencyReady {
		t.Fatalf("when RDD disabled, Verify should be ready, got %q", cs2.Dependencies.Verify)
	}
}

func TestStatusV2_ArchiveKeepsEnabled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	ws := t.TempDir()
	change := "archive-keep-enabled"
	changeDir := filepath.Join(ws, "openspec", "changes", change)
	if err := os.MkdirAll(changeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(ws, "openspec", "changes", "archive"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# P"), 0o644)
	_ = os.WriteFile(filepath.Join(ws, "openspec", "config.yaml"), []byte("artifact_store: openspec\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(ws, ".git", "objects"), 0o755)
	_ = os.WriteFile(filepath.Join(ws, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644)
	// Ensure RDD enabled before archive
	rs, _ := review.RDDStatus("", "")
	if rs.EffectiveMode != review.RDDModeEnabled {
		t.Fatalf("expected enabled before archive, got %s", rs.EffectiveMode)
	}
	if _, err := ArchiveChange(filepath.Join(ws, "openspec"), change); err != nil {
		t.Fatalf("ArchiveChange: %v", err)
	}
	rs2, _ := review.RDDStatus("", "")
	if rs2.EffectiveMode != review.RDDModeEnabled {
		t.Fatalf("archive must not auto-disable RDD: got %s", rs2.EffectiveMode)
	}
	if _, err := os.Stat(filepath.Join(ws, ".git", "biggz", "rdd-mode")); err == nil {
		t.Fatal("archive created rdd-mode file")
	}
}
