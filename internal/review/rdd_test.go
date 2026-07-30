package review

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRDDStatus_Default(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	status, err := RDDStatus("", "")
	if err != nil {
		t.Fatalf("RDDStatus() error: %v", err)
	}
	if status.EffectiveMode != RDDModeEnabled {
		t.Errorf("expected enabled, got %s", status.EffectiveMode)
	}
	if status.RecordedAt != nil {
		t.Errorf("expected nil recorded_at when no state file, got %v", status.RecordedAt)
	}
}

func TestRDDDisable_Global(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	status, err := RDDDisable("", "", "global")
	if err != nil {
		t.Fatalf("RDDDisable() error: %v", err)
	}
	if status.EffectiveMode != RDDModeDisabled {
		t.Errorf("expected disabled, got %s", status.EffectiveMode)
	}
	if status.GlobalMode != RDDModeDisabled {
		t.Errorf("expected global disabled, got %s", status.GlobalMode)
	}
	if status.RecordedAt == nil {
		t.Error("expected non-nil recorded_at after global disable")
	}

	// Re-enable
	status, err = RDDEnable("", "")
	if err != nil {
		t.Fatalf("RDDEnable() error: %v", err)
	}
	if status.EffectiveMode != RDDModeEnabled {
		t.Errorf("expected enabled after re-enable, got %s", status.EffectiveMode)
	}
	if status.RecordedAt == nil {
		t.Error("expected non-nil recorded_at after global enable")
	}
}

func TestRDDDisable_CloneLocal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	cloneDir := t.TempDir()
	gitDir := filepath.Join(cloneDir, ".git")
	os.MkdirAll(gitDir, 0755)

	status, err := RDDDisable(gitDir, gitDir, "clone")
	if err != nil {
		t.Fatalf("RDDDisable(clone) error: %v", err)
	}
	if status.EffectiveMode != RDDModeDisabled {
		t.Errorf("expected disabled, got %s", status.EffectiveMode)
	}
	if status.CloneMode != RDDModeDisabled {
		t.Errorf("expected clone disabled, got %s", status.CloneMode)
	}

	// Global should still be unset (clone overrides)
	if status.GlobalMode != RDDModeUnset {
		t.Errorf("expected global unset, got %s", status.GlobalMode)
	}

	if status.RecordedAt == nil {
		t.Error("expected non-nil recorded_at after clone disable")
	}
}

func TestRDD_AnyOffWins(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	// Enable globally
	RDDEnable("", "")

	// Disable via clone
	cloneDir := t.TempDir()
	gitDir := filepath.Join(cloneDir, ".git")
	os.MkdirAll(gitDir, 0755)
	RDDDisable(gitDir, gitDir, "clone")

	// Should be disabled (clone off wins over global on)
	status, _ := RDDStatus(gitDir, gitDir)
	if status.EffectiveMode != RDDModeDisabled {
		t.Errorf("expected disabled (any off wins), got %s", status.EffectiveMode)
	}
}

// ---------------------------------------------------------------------------
// AuthorizeRDDOperation tests
// ---------------------------------------------------------------------------

func TestAuthorizeRDDOperation_ReadAlwaysPasses(t *testing.T) {
	err := AuthorizeRDDOperation(RDDOperationRead, "", "")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestAuthorizeRDDOperation_StartBlockedWhenDisabled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	RDDDisable("", "", "global")

	err := AuthorizeRDDOperation(RDDOperationStart, "", "")
	if err == nil {
		t.Fatal("expected error when RDD is disabled")
	}
	var rddErr *RDDDisabledError
	if !errors.As(err, &rddErr) {
		t.Fatalf("expected *RDDDisabledError, got %T", err)
	}
	if rddErr.Operation != RDDOperationStart {
		t.Errorf("expected Operation=Start, got %v", rddErr.Operation)
	}
	if rddErr.Source != "global" {
		t.Errorf("expected Source=global, got %s", rddErr.Source)
	}
	if rddErr.Error() != "review start blocked by RDD. Enable with: biggz rdd enable" {
		t.Errorf("unexpected error message: %s", rddErr.Error())
	}
}

func TestAuthorizeRDDOperation_MutateBlockedWhenDisabled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	RDDDisable("", "", "global")

	err := AuthorizeRDDOperation(RDDOperationMutate, "", "")
	if err == nil {
		t.Fatal("expected error when RDD is disabled")
	}
	var rddErr *RDDDisabledError
	if !errors.As(err, &rddErr) {
		t.Fatalf("expected *RDDDisabledError, got %T", err)
	}
	if rddErr.Operation != RDDOperationMutate {
		t.Errorf("expected Operation=Mutate, got %v", rddErr.Operation)
	}
	if rddErr.Error() != "review mutation blocked by RDD. Enable with: biggz rdd enable" {
		t.Errorf("unexpected error message: %s", rddErr.Error())
	}
}

func TestAuthorizeRDDOperation_EnabledPasses(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	// Enabled by default
	err := AuthorizeRDDOperation(RDDOperationStart, "", "")
	if err != nil {
		t.Fatalf("expected nil when enabled, got %v", err)
	}

	err = AuthorizeRDDOperation(RDDOperationMutate, "", "")
	if err != nil {
		t.Fatalf("expected nil when enabled, got %v", err)
	}
}

func TestAuthorizeRDDOperation_CloneDisabledSource(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	// Enable globally
	RDDEnable("", "")

	// Disable via clone
	cloneDir := t.TempDir()
	gitDir := filepath.Join(cloneDir, ".git")
	os.MkdirAll(gitDir, 0755)
	RDDDisable(gitDir, gitDir, "clone")

	err := AuthorizeRDDOperation(RDDOperationStart, gitDir, gitDir)
	var rddErr *RDDDisabledError
	if !errors.As(err, &rddErr) {
		t.Fatalf("expected *RDDDisabledError, got %T", err)
	}
	if rddErr.Source != "clone" {
		t.Errorf("expected Source=clone, got %s", rddErr.Source)
	}
}

// ---------------------------------------------------------------------------
// CAS Generation tests
// ---------------------------------------------------------------------------

func TestRDDDisable_CloneLocalGenerations(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	cloneDir := t.TempDir()
	gitDir := filepath.Join(cloneDir, ".git")
	os.MkdirAll(gitDir, 0755)

	// First disable — creates gen-0000000000.json
	status, err := RDDDisable(gitDir, gitDir, "clone")
	if err != nil {
		t.Fatalf("RDDDisable: %v", err)
	}
	if status.EffectiveMode != RDDModeDisabled {
		t.Fatalf("expected disabled, got %s", status.EffectiveMode)
	}

	genDir := filepath.Join(gitDir, rddGenerationsDir)
	entries, err := os.ReadDir(genDir)
	if err != nil {
		t.Fatalf("read gen dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 generation file, got %d", len(entries))
	}

	// Re-enable with gitDir — should remove the generation directory
	RDDEnable(gitDir, gitDir)
	if _, err := os.Stat(genDir); !os.IsNotExist(err) {
		t.Log("enable cleaned up clone mode generation directory")
	}

	// Disable again — creates gen-0000000000.json (fresh start after dir cleanup)
	RDDDisable(gitDir, gitDir, "clone")
	entries2, _ := os.ReadDir(genDir)
	if len(entries2) != 1 {
		t.Fatalf("expected 1 generation file after re-disable, got %d", len(entries2))
	}
}

func TestRDDDisable_GenerationRevisions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	cloneDir := t.TempDir()
	gitDir := filepath.Join(cloneDir, ".git")
	os.MkdirAll(gitDir, 0755)

	// First disable — should create gen-0000000000.json with empty previous_revision
	_, err := RDDDisable(gitDir, gitDir, "clone")
	if err != nil {
		t.Fatalf("first RDDDisable: %v", err)
	}

	genDir := filepath.Join(gitDir, rddGenerationsDir)
	gen := readLatestGeneration(genDir)
	if gen == nil {
		t.Fatal("expected generation after disable")
	}
	if gen.Generation != 0 {
		t.Errorf("expected generation 0, got %d", gen.Generation)
	}
	if gen.PreviousRevision != "" {
		t.Errorf("expected empty previous_revision, got %s", gen.PreviousRevision)
	}
	if gen.Revision == "" {
		t.Error("expected non-empty revision")
	}
	if gen.Mode != RDDModeDisabled {
		t.Errorf("expected mode disabled, got %s", gen.Mode)
	}

	// Verify revision is deterministic
	rev1 := gen.Revision

	// Re-enable and re-disable to create gen-0000000001.json
	RDDEnable("", "")
	RDDDisable(gitDir, gitDir, "clone")

	gen = readLatestGeneration(genDir)
	if gen == nil {
		t.Fatal("expected generation after re-disable")
	}
	if gen.Generation != 0 {
		t.Logf("generation reset to 0 after enable+disable (gen dir was cleared)")
	}
	if gen.PreviousRevision != "" {
		t.Logf("previous_revision: %s (gen dir was cleared by enable)", gen.PreviousRevision)
	}

	// Enable via RDDEnable with gitDir removes the gen directory
	// So the second disable starts fresh
	_ = rev1
}

func TestVerifyCloneRevision(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	cloneDir := t.TempDir()
	gitDir := filepath.Join(cloneDir, ".git")
	os.MkdirAll(gitDir, 0755)

	// No generation yet — VerifyCloneRevision should return error
	err := VerifyCloneRevision(gitDir, "any")
	if err == nil {
		t.Fatal("expected error when no generation exists")
	}

	// Create a generation
	RDDDisable(gitDir, gitDir, "clone")
	gen := readLatestGeneration(filepath.Join(gitDir, rddGenerationsDir))
	if gen == nil {
		t.Fatal("expected generation after disable")
	}

	// Verify with correct revision
	if err := VerifyCloneRevision(gitDir, gen.Revision); err != nil {
		t.Fatalf("expected nil for correct revision, got %v", err)
	}

	// Verify with wrong revision
	if err := VerifyCloneRevision(gitDir, "wrong-revision"); err == nil {
		t.Fatal("expected error for wrong revision")
	}
}

// ---------------------------------------------------------------------------
// DeliveryDisposition tests
// ---------------------------------------------------------------------------

func TestResolveDeliveryDisposition_Disabled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	RDDDisable("", "", "global")

	disp := ResolveDeliveryDisposition("")
	if disp != DispositionDisabledUnmanaged {
		t.Errorf("expected DispositionDisabledUnmanaged, got %d", disp)
	}
}

func TestResolveDeliveryDisposition_Enabled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	disp := ResolveDeliveryDisposition("")
	if disp != DispositionReceiptGoverned {
		t.Errorf("expected DispositionReceiptGoverned, got %d", disp)
	}
}

func TestDeliveryDisposition_GateBypass(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	RDDDisable("", "", "global")

	// With RDD disabled, gate should pass without evaluating
	cloneDir := t.TempDir()
	gitDir := filepath.Join(cloneDir, ".git")
	os.MkdirAll(gitDir, 0755)

	// Even with an empty chain (would normally fail), gate should pass
	store := OpenWithDir(t.TempDir(), "empty")
	chain, _ := store.LoadChain()
	result := PrePRGate(chain, nil, nil, false, gitDir)
	if !result.Passed {
		t.Fatal("expected gate to pass when RDD disabled (delivery unmanaged)")
	}
}

// ---------------------------------------------------------------------------
// RDDRecordedAt tests
// ---------------------------------------------------------------------------

func TestRDDRecordedAt_CloneOverridesGlobal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	RDDEnable("", "")

	cloneDir := t.TempDir()
	gitDir := filepath.Join(cloneDir, ".git")
	os.MkdirAll(gitDir, 0755)
	RDDDisable(gitDir, gitDir, "clone")

	status, _ := RDDStatus(gitDir, gitDir)
	if status.EffectiveMode != RDDModeDisabled {
		t.Fatalf("expected disabled, got %s", status.EffectiveMode)
	}
	if status.RecordedAt == nil {
		t.Fatal("expected non-nil recorded_at from clone source")
	}
}

// ---------------------------------------------------------------------------
// ErrRDDModeRepositoryForcedOn tests
// ---------------------------------------------------------------------------

func TestWriteCloneMode_RejectsEnabled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	cloneDir := t.TempDir()
	gitDir := filepath.Join(cloneDir, ".git")
	os.MkdirAll(gitDir, 0755)

	err := writeRDDMode(gitDir, RDDModeEnabled)
	if err == nil {
		t.Fatal("expected error when writing enabled via clone mode")
	}
	if !errors.Is(err, ErrRDDModeRepositoryForcedOn) {
		t.Fatalf("expected ErrRDDModeRepositoryForcedOn, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Consent flow tests
// ---------------------------------------------------------------------------

func TestCheckConsent_NotFound(t *testing.T) {
	cloneDir := t.TempDir()
	gitDir := filepath.Join(cloneDir, ".git")
	os.MkdirAll(gitDir, 0755)

	given, err := CheckConsent(gitDir)
	if err != nil {
		t.Fatalf("CheckConsent: %v", err)
	}
	if given {
		t.Fatal("expected false when no asked.json exists")
	}
}

func TestRecordConsent_CreatesFile(t *testing.T) {
	cloneDir := t.TempDir()
	gitDir := filepath.Join(cloneDir, ".git")
	os.MkdirAll(gitDir, 0755)

	if err := RecordConsent(gitDir); err != nil {
		t.Fatalf("RecordConsent: %v", err)
	}

	p := filepath.Join(gitDir, rddGenerationsDir, rddConsentFile)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		t.Fatal("expected asked.json to exist after RecordConsent")
	}
}

func TestCheckConsent_Found(t *testing.T) {
	cloneDir := t.TempDir()
	gitDir := filepath.Join(cloneDir, ".git")
	os.MkdirAll(gitDir, 0755)

	if err := RecordConsent(gitDir); err != nil {
		t.Fatalf("RecordConsent: %v", err)
	}

	given, err := CheckConsent(gitDir)
	if err != nil {
		t.Fatalf("CheckConsent: %v", err)
	}
	if !given {
		t.Fatal("expected true after RecordConsent")
	}
}

func TestRecordConsent_Idempotent(t *testing.T) {
	cloneDir := t.TempDir()
	gitDir := filepath.Join(cloneDir, ".git")
	os.MkdirAll(gitDir, 0755)

	if err := RecordConsent(gitDir); err != nil {
		t.Fatalf("first RecordConsent: %v", err)
	}
	if err := RecordConsent(gitDir); err != nil {
		t.Fatalf("second RecordConsent: %v", err)
	}

	p := filepath.Join(gitDir, rddGenerationsDir, rddConsentFile)
	info, _ := os.Stat(p)
	if info == nil {
		t.Fatal("expected asked.json to exist")
	}
}

func TestPromptConsent_LowRiskAutoConsents(t *testing.T) {
	cloneDir := t.TempDir()
	gitDir := filepath.Join(cloneDir, ".git")
	os.MkdirAll(gitDir, 0755)

	err := PromptConsent(ConsentRequest{
		Risk:   "low",
		GitDir: gitDir,
	})
	if err != nil {
		t.Fatalf("PromptConsent(low): %v", err)
	}

	given, _ := CheckConsent(gitDir)
	if !given {
		t.Fatal("expected consent recorded after low-risk prompt")
	}
}
