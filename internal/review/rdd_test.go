package review

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRDDStatus_Default(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	status, err := RDDStatus("")
	if err != nil {
		t.Fatalf("RDDStatus() error: %v", err)
	}
	if status.EffectiveMode != RDDModeEnabled {
		t.Errorf("expected enabled, got %s", status.EffectiveMode)
	}
}

func TestRDDDisable_Global(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	status, err := RDDDisable("")
	if err != nil {
		t.Fatalf("RDDDisable() error: %v", err)
	}
	if status.EffectiveMode != RDDModeDisabled {
		t.Errorf("expected disabled, got %s", status.EffectiveMode)
	}
	if status.GlobalMode != RDDModeDisabled {
		t.Errorf("expected global disabled, got %s", status.GlobalMode)
	}

	// Re-enable
	status, err = RDDEnable("")
	if err != nil {
		t.Fatalf("RDDEnable() error: %v", err)
	}
	if status.EffectiveMode != RDDModeEnabled {
		t.Errorf("expected enabled after re-enable, got %s", status.EffectiveMode)
	}
}

func TestRDDDisable_CloneLocal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	cloneDir := t.TempDir()
	gitDir := filepath.Join(cloneDir, ".git")
	os.MkdirAll(gitDir, 0755)

	status, err := RDDDisable(gitDir)
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
}

func TestRDD_AnyOffWins(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	// Enable globally
	RDDEnable("")

	// Disable via clone
	cloneDir := t.TempDir()
	gitDir := filepath.Join(cloneDir, ".git")
	os.MkdirAll(gitDir, 0755)
	RDDDisable(gitDir)

	// Should be disabled (clone off wins over global on)
	status, _ := RDDStatus(gitDir)
	if status.EffectiveMode != RDDModeDisabled {
		t.Errorf("expected disabled (any off wins), got %s", status.EffectiveMode)
	}
}
