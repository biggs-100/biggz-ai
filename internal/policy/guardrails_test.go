package policy

import "testing"

func TestIsDenied_GitSelectionRED(t *testing.T) {
	if !IsDenied("git -C /r push --force") {
		t.Fatalf("expected IsDenied true for git -C push --force")
	}
	if IsDenied("git push") {
		t.Fatalf("expected IsDenied false for git push without force")
	}
}

func TestClassifyGuardedCommand_PushStateRED(t *testing.T) {
	cfg := RuntimeGuardrailsConfig{AutonomousMode: true, GuardedCommands: map[string]string{GuardGitPush: "allow"}}
	if got := ClassifyGuardedCommand("git push --force", cfg); got != "block" {
		t.Fatalf("expected block, got %q", got)
	}
}
