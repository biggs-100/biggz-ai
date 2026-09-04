package pi

import (
	"strings"
	"testing"
)

// TestInstallCommand_UsesJ0k3rFork ensures pi installs the maintained
// j0k3r fork as its subagent dispatcher, never the predecessor package.
func TestInstallCommand_UsesJ0k3rFork(t *testing.T) {
	a := NewAdapter()
	cmds, err := a.InstallCommand(nil)
	if err != nil {
		t.Fatalf("InstallCommand: %v", err)
	}
	foundFork := false
	for _, cmd := range cmds {
		joined := strings.Join(cmd, " ")
		if joined == "pi install npm:pi-subagents" {
			t.Errorf("InstallCommand installs predecessor package: %q", joined)
		}
		if joined == "pi install npm:pi-subagents-j0k3r" {
			foundFork = true
		}
	}
	if !foundFork {
		t.Errorf("InstallCommand missing fork install, got %v", cmds)
	}
}

// TestFilterPiPackages_DropsPredecessor ensures settings.json reconciliation
// drops the predecessor dispatcher entry while keeping the fork, so pi never
// loads both (duplicate subagent_* tool registrations).
func TestFilterPiPackages_DropsPredecessor(t *testing.T) {
	in := []any{
		"npm:pi-subagents",
		"npm:pi-subagents@0.65.0",
		"npm:pi-subagents-j0k3r",
		"npm:@heyhuynhgiabuu/pi-pretty",
	}
	got := filterPiPackages(in)
	for _, pkg := range got {
		if s, _ := pkg.(string); s == "npm:pi-subagents" || s == "npm:pi-subagents@0.65.0" {
			t.Errorf("predecessor entry survived filter: %q", s)
		}
	}
	joined := make([]string, 0, len(got))
	for _, pkg := range got {
		if s, ok := pkg.(string); ok {
			joined = append(joined, s)
		}
	}
	for _, want := range []string{"npm:pi-subagents-j0k3r", "npm:@heyhuynhgiabuu/pi-pretty"} {
		if !containsPiPackage(got, want) {
			t.Errorf("expected %q to survive filter, got %v", want, joined)
		}
	}
}
