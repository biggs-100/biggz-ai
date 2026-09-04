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

// TestInstallCommand_IncludesTodoOverlay ensures pi installs the rpiv-todo
// visual task-tracking overlay (gentle-pi parity).
func TestInstallCommand_IncludesTodoOverlay(t *testing.T) {
	a := NewAdapter()
	cmds, err := a.InstallCommand(nil)
	if err != nil {
		t.Fatalf("InstallCommand: %v", err)
	}
	for _, cmd := range cmds {
		if strings.Join(cmd, " ") == "pi install npm:rpiv-todo" {
			return
		}
	}
	t.Errorf("InstallCommand missing todo overlay install, got %v", cmds)
}

// TestInstallCommand_IncludesWebAndBtw ensures pi installs the web-access
// capabilities and the /btw side-conversation channel (gentle-pi parity).
func TestInstallCommand_IncludesWebAndBtw(t *testing.T) {
	a := NewAdapter()
	cmds, err := a.InstallCommand(nil)
	if err != nil {
		t.Fatalf("InstallCommand: %v", err)
	}
	joined := make([]string, 0, len(cmds))
	for _, cmd := range cmds {
		joined = append(joined, strings.Join(cmd, " "))
	}
	for _, want := range []string{"pi install npm:pi-web-access", "pi install npm:pi-btw"} {
		found := false
		for _, got := range joined {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("InstallCommand missing %q, got %v", want, joined)
		}
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
